// Package kvpiles is the [piles.Store] backend for the single-process tiers
// (fs, memory, sqlite, desktop), persisting every owner's piles through a
// [state.KV].
//
// # One document, read-modify-write under a lock
//
// All piles live in one JSON value under [StateKey]. Owner strings are inside
// the value and never in a key, so a login name or person id cannot reach a
// file name. Every method takes the store's mutex and then its [Locker],
// re-reads the document, applies its change and writes it back. That makes
// each method atomic: concurrent adds lose nothing and the limits hold.
//
// # Several processes, one machine
//
// The desktop app, `rela mcp`, CLI Lua and `rela scheduler` can all open one
// project, and each builds its own Store over the same cache-directory
// document. The mutex only excludes callers of one instance, so the Locker
// must exclude every other process too: over an on-disk KV that is a
// [FileLocker] on a lock file next to the document. Several MACHINES are out
// of scope; the postgres build uses pgpiles for that.
//
// # Node-local, not in the project database
//
// On the sqlite build the KV handed in is the .rela cache directory, not the
// database's state_kv table. A pile is personal, and rela.db is a file that is
// shipped to other people.
package kvpiles

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// StateKey is where the document lives inside the KV.
const StateKey = "piles.json"

// Store is a [piles.Store] over a [state.KV].
type Store struct {
	kv     state.KV
	locker Locker
	mu     sync.Mutex
}

// Locker excludes every other writer of the document for the span of one
// read-modify-write: other processes over the same KV included. Lock blocks
// until the lock is held or ctx ends, and returns the release func.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// ProcessPrivate is the [Locker] for a KV no other process can open, such as
// an in-memory one. It locks nothing beyond the Store's own mutex; choosing it
// over an on-disk KV loses writes between processes.
type ProcessPrivate struct{}

// Lock returns at once: the Store's mutex already excludes this process.
func (ProcessPrivate) Lock(context.Context) (func(), error) { return func() {}, nil }

var _ piles.Store = (*Store)(nil)

// document is the serialized shape.
type document struct {
	Piles []storedPile `json:"piles"`
}

type storedPile struct {
	ID      string    `json:"id"`
	Owner   string    `json:"owner"`
	Name    string    `json:"name"`
	Icon    string    `json:"icon"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	// Items are newest first.
	Items []storedItem `json:"items"`
}

// storedItem keeps id and face apart rather than storing the "ID@face" text
// form, so a document never fails to load over a ref that does not
// re-serialize.
type storedItem struct {
	ID    string    `json:"id"`
	Face  string    `json:"face,omitempty"`
	Added time.Time `json:"added"`
}

func (it storedItem) ref() entity.Ref { return entity.Ref{ID: it.ID, Face: entity.Face(it.Face)} }

// New returns a store backed by kv, with locker excluding other processes.
//
// Nil: both rejected. A nil KV would defer the failure to the first pile
// anyone creates, far from the wiring mistake that caused it; a nil locker
// would hide the choice between [FileLocker] and [ProcessPrivate].
func New(kv state.KV, locker Locker) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvpiles: state KV must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("kvpiles: locker must be non-nil")
	}
	return &Store{kv: kv, locker: locker}, nil
}

// lock takes the mutex and then the locker, and returns the release of both.
func (s *Store) lock(ctx context.Context) (func(), error) {
	s.mu.Lock()
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("kvpiles: lock %s: %w", StateKey, err)
	}
	return func() {
		unlock()
		s.mu.Unlock()
	}, nil
}

// load reads the document. It re-reads on every call, so a change made
// through another handle over the same KV is observed. Caller holds the lock.
//
// A corrupt document is an error, not an empty one. Unlike next-action state,
// piles are things a user collected by hand, and treating a damaged file as
// empty would overwrite it with the next write.
func (s *Store) load(ctx context.Context) (*document, error) {
	data, err := s.kv.Get(ctx, StateKey)
	if os.IsNotExist(err) {
		return &document{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kvpiles: read %s: %w", StateKey, err)
	}
	var d document
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("kvpiles: %s is not valid JSON (move it aside to start over): %w", StateKey, err)
	}
	return &d, nil
}

// flush writes the document back. Caller holds the lock.
func (s *Store) flush(ctx context.Context, d *document) error {
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("kvpiles: encode %s: %w", StateKey, err)
	}
	if err := s.kv.Put(ctx, StateKey, data); err != nil {
		return fmt.Errorf("kvpiles: write %s: %w", StateKey, err)
	}
	return nil
}

// read runs fn over the document under the lock. Reads lock too: the KV may
// replace the file while another process writes it.
func (s *Store) read(ctx context.Context, fn func(d *document) error) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.load(ctx)
	if err != nil {
		return err
	}
	return fn(d)
}

// write runs fn over the document under the lock and writes it back when fn
// reports a change.
func (s *Store) write(ctx context.Context, fn func(d *document) (changed bool, err error)) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.load(ctx)
	if err != nil {
		return err
	}
	changed, err := fn(d)
	if err != nil || !changed {
		return err
	}
	return s.flush(ctx, d)
}

// find returns the index of the owner's pile, or -1. The owner filter is part
// of the lookup, so a foreign pile is indistinguishable from a missing one.
func (d *document) find(owner, id string) int {
	return slices.IndexFunc(d.Piles, func(p storedPile) bool { return p.ID == id && p.Owner == owner })
}

// nameTaken reports whether owner has a pile other than exceptID named name,
// compared case-insensitively.
func (d *document) nameTaken(owner, name, exceptID string) bool {
	return slices.ContainsFunc(d.Piles, func(p storedPile) bool {
		return p.Owner == owner && p.ID != exceptID && strings.EqualFold(p.Name, name)
	})
}

func (p storedPile) export() piles.Pile {
	items := make([]piles.Item, len(p.Items))
	for i, it := range p.Items {
		items[i] = piles.Item{Ref: it.ref(), Added: it.Added}
	}
	return piles.Pile{
		ID: p.ID, Owner: p.Owner, Name: p.Name, Icon: p.Icon,
		Created: p.Created, Updated: p.Updated, Items: items,
	}
}

// ListPiles returns the owner's piles, oldest first.
func (s *Store) ListPiles(ctx context.Context, owner string) ([]piles.Pile, error) {
	out := []piles.Pile{}
	err := s.read(ctx, func(d *document) error {
		for _, p := range d.Piles {
			if p.Owner == owner {
				out = append(out, p.export())
			}
		}
		return nil
	})
	slices.SortStableFunc(out, func(a, b piles.Pile) int {
		return cmp.Or(a.Created.Compare(b.Created), strings.Compare(a.ID, b.ID))
	})
	return out, err
}

// GetPile returns one of the owner's piles.
func (s *Store) GetPile(ctx context.Context, owner, id string) (piles.Pile, error) {
	var out piles.Pile
	err := s.read(ctx, func(d *document) error {
		i := d.find(owner, id)
		if i < 0 {
			return piles.ErrNotFound
		}
		out = d.Piles[i].export()
		return nil
	})
	return out, err
}

// PileByName finds the owner's pile by case-insensitive name.
func (s *Store) PileByName(ctx context.Context, owner, name string) (piles.Pile, error) {
	var out piles.Pile
	err := s.read(ctx, func(d *document) error {
		i := slices.IndexFunc(d.Piles, func(p storedPile) bool {
			return p.Owner == owner && strings.EqualFold(p.Name, name)
		})
		if i < 0 {
			return piles.ErrNotFound
		}
		out = d.Piles[i].export()
		return nil
	})
	return out, err
}

// CreatePile stores p with refs as its first items.
func (s *Store) CreatePile(ctx context.Context, p piles.Pile, refs []entity.Ref, maxPiles, maxItems int) error {
	return s.write(ctx, func(d *document) (bool, error) {
		if slices.ContainsFunc(d.Piles, func(q storedPile) bool { return q.ID == p.ID }) {
			return false, piles.ErrIDTaken
		}
		if d.nameTaken(p.Owner, p.Name, "") {
			return false, piles.ErrNameTaken
		}
		count := 0
		for _, q := range d.Piles {
			if q.Owner == p.Owner {
				count++
			}
		}
		if count >= maxPiles {
			return false, piles.ErrLimit
		}
		refs = firstUnique(refs, maxItems)
		items := make([]storedItem, len(refs))
		for i, r := range refs {
			items[i] = storedItem{ID: r.ID, Face: string(r.Face), Added: p.Created}
		}
		d.Piles = append(d.Piles, storedPile{
			ID: p.ID, Owner: p.Owner, Name: p.Name, Icon: p.Icon,
			Created: p.Created, Updated: p.Updated, Items: items,
		})
		return true, nil
	})
}

// UpdatePile renames or re-icons a pile.
func (s *Store) UpdatePile(ctx context.Context, owner, id, name, icon string, now time.Time) error {
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		if d.nameTaken(owner, name, id) {
			return false, piles.ErrNameTaken
		}
		d.Piles[i].Name, d.Piles[i].Icon, d.Piles[i].Updated = name, icon, now
		return true, nil
	})
}

// DeletePile removes a pile and its items.
func (s *Store) DeletePile(ctx context.Context, owner, id string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		d.Piles = slices.Delete(d.Piles, i, i+1)
		return true, nil
	})
}

// AddItems puts the new refs on top; past maxItems it evicts the oldest or,
// with [piles.KeepExisting], adds only what fits.
func (s *Store) AddItems(
	ctx context.Context, owner, id string, refs []entity.Ref, now time.Time, maxItems int, overflow piles.Overflow,
) (int, error) {
	added := 0
	err := s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		p := &d.Piles[i]
		present := make(map[entity.Ref]bool, len(p.Items))
		for _, it := range p.Items {
			present[it.ref()] = true
		}
		var fresh []storedItem
		for _, r := range firstUnique(refs, maxItems) {
			if !present[r] {
				fresh = append(fresh, storedItem{ID: r.ID, Face: string(r.Face), Added: now})
			}
		}
		if overflow != piles.EvictOldest {
			fresh = fresh[:min(len(fresh), max(maxItems-len(p.Items), 0))]
		}
		if len(fresh) == 0 {
			return false, nil
		}
		added = len(fresh)
		p.Items = append(fresh, p.Items...)
		if len(p.Items) > maxItems {
			p.Items = p.Items[:maxItems]
		}
		return true, nil
	})
	return added, err
}

// RemoveItems drops refs from the pile, ignoring refs not on it.
func (s *Store) RemoveItems(ctx context.Context, owner, id string, refs []entity.Ref) error {
	drop := make(map[entity.Ref]bool, len(refs))
	for _, r := range refs {
		drop[r] = true
	}
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		before := len(d.Piles[i].Items)
		d.Piles[i].Items = slices.DeleteFunc(d.Piles[i].Items, func(it storedItem) bool { return drop[it.ref()] })
		return len(d.Piles[i].Items) != before, nil
	})
}

// RenameEntity rewrites items naming oldID. Where the new ref is already on
// a pile, the existing item keeps its place and the renamed one is dropped.
func (s *Store) RenameEntity(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	return s.write(ctx, func(d *document) (bool, error) {
		changed := false
		for pi := range d.Piles {
			if renameItems(&d.Piles[pi], oldID, newID) {
				changed = true
			}
		}
		return changed, nil
	})
}

// RenameOwner moves oldOwner's piles to newOwner.
//
// It runs for a renamed person, whose new id is fresh, so newOwner normally
// holds no piles. If it does, they can only be a leftover the delete hook
// missed: where one clashes by name with a moved pile, the moved pile wins and
// the leftover is dropped, so the one-name-per-owner rule still holds.
func (s *Store) RenameOwner(ctx context.Context, oldOwner, newOwner string) error {
	if oldOwner == newOwner {
		return nil
	}
	return s.write(ctx, func(d *document) (bool, error) {
		movedNames := map[string]bool{}
		for _, p := range d.Piles {
			if p.Owner == oldOwner {
				movedNames[strings.ToLower(p.Name)] = true
			}
		}
		if len(movedNames) == 0 {
			return false, nil
		}
		d.Piles = slices.DeleteFunc(d.Piles, func(p storedPile) bool {
			return p.Owner == newOwner && movedNames[strings.ToLower(p.Name)]
		})
		for pi := range d.Piles {
			if d.Piles[pi].Owner == oldOwner {
				d.Piles[pi].Owner = newOwner
			}
		}
		return true, nil
	})
}

// DeleteOwner drops every pile owned by owner.
func (s *Store) DeleteOwner(ctx context.Context, owner string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		before := len(d.Piles)
		d.Piles = slices.DeleteFunc(d.Piles, func(p storedPile) bool { return p.Owner == owner })
		return len(d.Piles) != before, nil
	})
}

// renameItems rewrites one pile's items and reports whether any changed.
func renameItems(p *storedPile, oldID, newID string) bool {
	if !slices.ContainsFunc(p.Items, func(it storedItem) bool { return it.ID == oldID }) {
		return false
	}
	taken := map[string]bool{}
	for _, it := range p.Items {
		if it.ID == newID {
			taken[it.Face] = true
		}
	}
	out := p.Items[:0]
	for _, it := range p.Items {
		if it.ID == oldID {
			if taken[it.Face] {
				continue
			}
			it.ID = newID
		}
		out = append(out, it)
	}
	p.Items = out
	return true
}

// DeleteEntity drops items naming id.
func (s *Store) DeleteEntity(ctx context.Context, id string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		changed := false
		for pi := range d.Piles {
			n := len(d.Piles[pi].Items)
			d.Piles[pi].Items = slices.DeleteFunc(d.Piles[pi].Items, func(it storedItem) bool { return it.ID == id })
			changed = changed || len(d.Piles[pi].Items) != n
		}
		return changed, nil
	})
}

// DeleteFace drops items naming exactly that face of id.
func (s *Store) DeleteFace(ctx context.Context, id string, face entity.Face) error {
	return s.write(ctx, func(d *document) (bool, error) {
		changed := false
		for pi := range d.Piles {
			n := len(d.Piles[pi].Items)
			d.Piles[pi].Items = slices.DeleteFunc(d.Piles[pi].Items, func(it storedItem) bool {
				return it.ID == id && it.Face == string(face)
			})
			changed = changed || len(d.Piles[pi].Items) != n
		}
		return changed, nil
	})
}

// firstUnique collapses duplicate refs, keeping the first, and keeps at most
// limit of them.
func firstUnique(refs []entity.Ref, limit int) []entity.Ref {
	seen := make(map[entity.Ref]bool, len(refs))
	out := make([]entity.Ref, 0, min(len(refs), limit))
	for _, r := range refs {
		if len(out) == limit {
			break
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
