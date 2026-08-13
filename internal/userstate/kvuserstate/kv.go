// Package kvuserstate is the durable [userstate.Store] backend, persisting
// next-action state through a [state.KV].
//
// # Why KV rather than a bespoke file
//
// state.KV is already the project's home for "state that persists between
// runs but isn't part of the project's tracked source" — the scheduler's
// last-run timestamps and the document render cache live there. Snoozes,
// mutes and cooldowns are exactly that, so they get the same seam (and the
// same `.rela/` gitignore, the same swap boundary if an operator ever plugs
// in Redis) rather than a second persistence mechanism beside it.
//
// # One document, read-modify-write under a mutex
//
// All of a deployment's next-action state lives in a single JSON value. That
// is deliberate for the data's shape: it is small (a handful of rows per
// user), read on every page render, and written only on explicit user
// feedback. Per-key entries would turn one render into N KV reads.
//
// The cost is that concurrent writers must serialize, which the mutex does
// WITHIN a process. Across processes this backend is last-writer-wins: two
// servers sharing a project directory can lose a snooze. That is acceptable
// for disposable state and is why the postgres backend exists for the
// multi-process deployment — it is stated here so nobody discovers it as a
// surprise.
package kvuserstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// StateKey is where the document lives inside the KV.
const StateKey = "next-action-state.json"

// Store is a durable [userstate.Store] over a [state.KV].
type Store struct {
	kv state.KV

	mu     sync.Mutex
	closed bool
	// doc is the in-memory copy, loaded lazily on first use. Held so a page
	// render costs no KV read once warm; writes update it and flush.
	doc *document
}

// document is the serialized shape. Maps are keyed by the composite key
// string so the JSON stays flat and diffable.
type document struct {
	// Snoozes maps a suggestion key to its deadline.
	Snoozes map[string]time.Time `json:"snoozes,omitempty"`
	// Shown maps a suggestion key to when it was last surfaced.
	Shown map[string]time.Time `json:"shown,omitempty"`
	// Muted maps a user to their muted source ids.
	Muted map[string][]string `json:"muted,omitempty"`
}

func newDocument() *document {
	return &document{
		Snoozes: map[string]time.Time{},
		Shown:   map[string]time.Time{},
		Muted:   map[string][]string{},
	}
}

// New returns a durable store backed by kv. Rejects a nil KV rather than
// silently degrading to in-memory: a store that forgets snoozes on restart
// while claiming to persist them is the deferred failure the project's
// constructor rule exists to prevent.
func New(kv state.KV) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvuserstate: state KV must be non-nil")
	}
	return &Store{kv: kv}, nil
}

// compile-time check
var _ userstate.Store = (*Store)(nil)

// keyString renders a suggestion key. The user is part of it, so one
// document holds every user's state without their entries colliding.
//
// NUL-separated because none of the components may contain it, so two
// different keys can never render to the same string (a source id containing
// a colon would otherwise collide with a variant containing one).
func keyString(k userstate.Key) string {
	return k.User + "\x00" + k.Source + "\x00" + k.EntityID + "\x00" + k.Variant
}

// load returns the document, reading it from the KV on first use.
// Caller must hold s.mu.
func (s *Store) load(ctx context.Context) (*document, error) {
	if s.doc != nil {
		return s.doc, nil
	}
	data, err := s.kv.Get(ctx, StateKey)
	switch {
	case err == nil:
		var d document
		if jsonErr := json.Unmarshal(data, &d); jsonErr != nil {
			// A corrupt document is treated as empty, matching the
			// scheduler's handling of its own state file. This state is
			// disposable: refusing to start because a snooze log is
			// unreadable would be a worse failure than forgetting it.
			d = *newDocument()
		}
		normalize(&d)
		s.doc = &d
	case os.IsNotExist(err):
		s.doc = newDocument()
	default:
		return nil, fmt.Errorf("kvuserstate: read state: %w", err)
	}
	return s.doc, nil
}

// normalize fills nil maps so callers never write to one.
func normalize(d *document) {
	if d.Snoozes == nil {
		d.Snoozes = map[string]time.Time{}
	}
	if d.Shown == nil {
		d.Shown = map[string]time.Time{}
	}
	if d.Muted == nil {
		d.Muted = map[string][]string{}
	}
}

// flush writes the in-memory document back. Caller must hold s.mu.
func (s *Store) flush(ctx context.Context) error {
	data, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return fmt.Errorf("kvuserstate: encode state: %w", err)
	}
	if err := s.kv.Put(ctx, StateKey, data); err != nil {
		return fmt.Errorf("kvuserstate: write state: %w", err)
	}
	return nil
}

func (s *Store) SnoozedUntil(
	ctx context.Context, key userstate.Key, now time.Time,
) (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return time.Time{}, false, userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	until, ok := doc.Snoozes[keyString(key)]
	// Expiry is judged at READ time so correctness never depends on Prune
	// having run; `now` is exclusive-of-equal, matching how a deadline reads.
	if !ok || !now.Before(until) {
		return time.Time{}, false, nil
	}
	return until, true, nil
}

func (s *Store) SetSnooze(ctx context.Context, key userstate.Key, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return err
	}
	// Replace, never stack: a later "not for a week" must win over an
	// earlier "not for an hour", and vice versa.
	doc.Snoozes[keyString(key)] = until
	return s.flush(ctx)
}

func (s *Store) Muted(ctx context.Context, user, source string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return false, err
	}
	return slices.Contains(doc.Muted[user], source), nil
}

func (s *Store) SetMuted(ctx context.Context, user, source string, muted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return err
	}

	cur := doc.Muted[user]
	idx := slices.Index(cur, source)
	switch {
	case muted && idx < 0:
		doc.Muted[user] = append(cur, source)
	case !muted && idx >= 0:
		doc.Muted[user] = slices.Delete(cur, idx, idx+1)
		if len(doc.Muted[user]) == 0 {
			delete(doc.Muted, user)
		}
	default:
		return nil // already in the requested state
	}
	return s.flush(ctx)
}

func (s *Store) MutedSources(ctx context.Context, user string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(doc.Muted[user])
	slices.Sort(out)
	return out, nil
}

func (s *Store) LastShown(ctx context.Context, key userstate.Key) (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return time.Time{}, false, userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	at, ok := doc.Shown[keyString(key)]
	return at, ok, nil
}

func (s *Store) MarkShown(ctx context.Context, key userstate.Key, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return err
	}
	doc.Shown[keyString(key)] = at
	return s.flush(ctx)
}

func (s *Store) Prune(ctx context.Context, now time.Time, keepShown time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, userstate.ErrClosed
	}
	doc, err := s.load(ctx)
	if err != nil {
		return 0, err
	}

	removed := 0
	for k, until := range doc.Snoozes {
		if !now.Before(until) {
			delete(doc.Snoozes, k)
			removed++
		}
	}
	cutoff := now.Add(-keepShown)
	for k, at := range doc.Shown {
		if at.Before(cutoff) {
			delete(doc.Shown, k)
			removed++
		}
	}
	if removed == 0 {
		return 0, nil
	}
	// Mutes are untouched: they are a standing choice with no expiry, and a
	// housekeeping pass that silently un-muted a source would surface as
	// suggestions the user switched off coming back.
	return removed, s.flush(ctx)
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.doc = nil
	return nil
}

// snapshot returns a copy of the in-memory document. Test-only helper kept
// unexported; the conformance suite exercises behavior through the interface.
func (s *Store) snapshot() document {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return *newDocument()
	}
	out := newDocument()
	maps.Copy(out.Snoozes, s.doc.Snoozes)
	maps.Copy(out.Shown, s.doc.Shown)
	maps.Copy(out.Muted, s.doc.Muted)
	return *out
}
