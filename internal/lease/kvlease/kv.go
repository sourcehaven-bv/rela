// Package kvlease is the [lease.Store] backend for the single-process tiers
// (fs, memory, sqlite, desktop), keeping every lease in one document in a
// [state.KV].
//
// # One document, read-modify-write under a lock
//
// All leases live in one JSON value under [StateKey], with the token counter
// in its header. Every method takes the store's mutex and then its [Locker],
// re-reads the document, applies its change and writes it back through the
// KV's own write path (an atomic replace on disk). That makes each method
// atomic: one claim wins, and the caps hold.
//
// # Several processes, one machine
//
// The desktop app, `rela mcp`, CLI calls and `rela scheduler` can all open
// one project, and each builds its own Store over the same cache-directory
// document. The mutex only excludes callers of one instance, so the Locker
// must exclude every other process too: over an on-disk KV that is a
// filelock.Locker on a lock file next to the document. Several machines are
// out of scope; the postgres build uses pglease for that.
//
// # The version key
//
// Watchers poll [Store.Version] to learn about writes made by other
// processes. It reads only the small [VersionKey] value, written under the
// lock after the document, so a poll never parses the lease document.
//
// # A damaged document is reset
//
// Leases expire within the hour and their holders renew them, so a document
// that does not parse is replaced by an empty one, under the lock, with a
// warning that names the key but not the contents. Unlike piles, nothing a
// person collected by hand is lost.
package kvlease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

const (
	// StateKey is where the lease document lives inside the KV.
	StateKey = "realtime-leases.json"
	// VersionKey holds the document's version as decimal text.
	VersionKey = "realtime-leases.version"
)

// Locker excludes every other writer of the document for the span of one
// read-modify-write: other processes over the same KV included. Lock blocks
// until the lock is held or ctx ends, and returns the release func.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// ProcessPrivate is the [Locker] for a KV no other process can open, such as
// an in-memory one. It locks nothing beyond the Store's own mutex; choosing it
// over an on-disk KV lets two processes both win one claim.
type ProcessPrivate struct{}

// Lock returns at once: the Store's mutex already excludes this process.
func (ProcessPrivate) Lock(context.Context) (func(), error) { return func() {}, nil }

// Store is a [lease.Store] over a [state.KV].
type Store struct {
	kv     state.KV
	locker Locker
	mu     sync.Mutex
}

var _ lease.Store = (*Store)(nil)

// document is the serialized shape.
type document struct {
	// Token is the last token issued.
	Token uint64 `json:"token"`
	// Version is the last version written; [VersionKey] repeats it.
	Version uint64 `json:"version"`
	// Leases is keyed by entity id.
	Leases map[string]storedLease `json:"leases"`
}

type storedLease struct {
	Identity  string    `json:"identity"`
	User      string    `json:"user"`
	Tool      string    `json:"tool"`
	Label     string    `json:"label,omitempty"`
	LeaseHash string    `json:"lease_hash"`
	Token     uint64    `json:"token"`
	Since     time.Time `json:"since"`
	Expires   time.Time `json:"expires"`
	Status    string    `json:"status,omitempty"`
}

func (s storedLease) lease(id string) lease.Lease {
	return lease.Lease{
		ID:        id,
		Holder:    lease.Holder{Identity: s.Identity, User: s.User, Tool: s.Tool, Label: s.Label},
		LeaseHash: s.LeaseHash,
		Token:     s.Token,
		Since:     s.Since,
		Expires:   s.Expires,
		Status:    s.Status,
	}
}

func (s storedLease) live(now time.Time) bool { return now.Before(s.Expires) }

// New returns a store backed by kv, with locker excluding other processes.
//
// Nil: both rejected. A nil KV would defer the failure to the first claim;
// a nil locker would hide the choice between a file lock and
// [ProcessPrivate].
func New(kv state.KV, locker Locker) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvlease: state KV must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("kvlease: locker must be non-nil")
	}
	return &Store{kv: kv, locker: locker}, nil
}

// lock takes the mutex and then the locker, and returns the release of both.
func (s *Store) lock(ctx context.Context) (func(), error) {
	s.mu.Lock()
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("kvlease: lock %s: %w", StateKey, err)
	}
	return func() {
		unlock()
		s.mu.Unlock()
	}, nil
}

// load reads the document. A missing document is empty; so is one that does
// not parse, after a warning. Caller holds the lock.
//
// The token counter starts no lower than the version key, which flush keeps
// above every token issued. So tokens keep increasing after the
// document is deleted or reset, even within one microsecond.
func (s *Store) load(ctx context.Context) (*document, error) {
	d, err := s.parse(ctx)
	if err != nil {
		return nil, err
	}
	d.Token = max(d.Token, s.readVersion(ctx))
	return d, nil
}

// parse reads and decodes the document.
func (s *Store) parse(ctx context.Context) (*document, error) {
	d := &document{Leases: map[string]storedLease{}}
	data, err := s.kv.Get(ctx, StateKey)
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kvlease: read %s: %w", StateKey, err)
	}
	if err := json.Unmarshal(data, d); err != nil {
		// The error text may quote the document, so only the key is logged.
		slog.Warn("kvlease: lease document is damaged; starting from an empty one", "key", StateKey)
		return &document{Leases: map[string]storedLease{}}, nil //nolint:nilerr // reset by design
	}
	if d.Leases == nil {
		d.Leases = map[string]storedLease{}
	}
	return d, nil
}

// flush writes the document and then the version key. Caller holds the lock.
//
// The version is one past the largest of the document's version, the version
// key and the token counter. So it moves on after the document was deleted,
// and it stays above every token issued, which load relies on. A
// failed version write is logged, not returned: the change itself is stored,
// and reporting it as failed would make a caller retry a claim it already
// holds. Watchers then see the change at the next write.
func (s *Store) flush(ctx context.Context, d *document) error {
	d.Version = max(d.Version, d.Token, s.readVersion(ctx)) + 1
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("kvlease: encode %s: %w", StateKey, err)
	}
	if err := s.kv.Put(ctx, StateKey, data); err != nil {
		return fmt.Errorf("kvlease: write %s: %w", StateKey, err)
	}
	if err := s.kv.Put(ctx, VersionKey, []byte(strconv.FormatUint(d.Version, 10))); err != nil {
		slog.Warn("kvlease: could not write the lease version", "key", VersionKey, "error", err)
	}
	return nil
}

// readVersion returns the version key's value, or 0 when it is missing or
// unreadable.
func (s *Store) readVersion(ctx context.Context) uint64 {
	v, err := s.Version(ctx)
	if err != nil {
		return 0
	}
	return v
}

// read runs fn over the document under the lock. Reads lock too: the KV may
// replace the file while another process writes it.
func (s *Store) read(ctx context.Context, fn func(d *document)) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.load(ctx)
	if err != nil {
		return err
	}
	fn(d)
	return nil
}

// write runs fn over the document under the lock and writes it back when fn
// reports a change, also when fn returns an error (a claim refused by a cap
// still keeps its pruning).
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
	changed, fnErr := fn(d)
	if changed {
		if err := s.flush(ctx, d); err != nil {
			return err
		}
	}
	return fnErr
}

// prune removes every lease that is expired at now and reports whether it
// removed any.
func (d *document) prune(now time.Time) bool {
	n := len(d.Leases)
	for id, l := range d.Leases {
		if !l.live(now) {
			delete(d.Leases, id)
		}
	}
	return len(d.Leases) != n
}

// byHash returns the entity id of the lease with leaseHash.
func (d *document) byHash(leaseHash string) (string, bool) {
	for id, l := range d.Leases {
		if l.LeaseHash == leaseHash {
			return id, true
		}
	}
	return "", false
}

// Claim implements [lease.Store.Claim].
func (s *Store) Claim(
	ctx context.Context, id string, h lease.Holder, leaseHash string, now, until time.Time, status string,
) (lease.Lease, error) {
	if err := lease.CheckClaim(id, h, leaseHash, now, until); err != nil {
		return lease.Lease{}, err
	}
	var out lease.Lease
	err := s.write(ctx, func(d *document) (bool, error) {
		pruned := d.prune(now)
		if cur, ok := d.Leases[id]; ok {
			held := cur.lease(id)
			held.LeaseHash = ""
			return pruned, &lease.HeldError{Current: held}
		}
		if _, taken := d.byHash(leaseHash); taken {
			return pruned, fmt.Errorf("%w: lease hash already in use", lease.ErrInvalid)
		}
		if len(d.Leases) >= lease.MaxPerStore {
			return pruned, lease.ErrFull
		}
		mine := 0
		for _, l := range d.Leases {
			if l.Identity == h.Identity {
				mine++
			}
		}
		if mine >= lease.MaxPerIdentity {
			return pruned, lease.ErrLimit
		}
		d.Token = lease.NextToken(d.Token, now)
		sl := storedLease{
			Identity: h.Identity, User: h.User, Tool: h.Tool, Label: h.Label,
			LeaseHash: leaseHash, Token: d.Token, Since: now, Expires: until, Status: status,
		}
		d.Leases[id] = sl
		out = sl.lease(id)
		return true, nil
	})
	return out, err
}

// Renew implements [lease.Store.Renew].
func (s *Store) Renew(
	ctx context.Context, leaseHash, identity string, now, until time.Time, status *string,
) (lease.Lease, error) {
	if err := lease.CheckUntil(now, until); err != nil {
		return lease.Lease{}, err
	}
	var out lease.Lease
	err := s.write(ctx, func(d *document) (bool, error) {
		pruned := d.prune(now)
		id, ok := d.byHash(leaseHash)
		if !ok || leaseHash == "" || d.Leases[id].Identity != identity {
			return pruned, lease.ErrLost
		}
		l := d.Leases[id]
		l.Expires = until
		if status != nil {
			l.Status = *status
		}
		d.Leases[id] = l
		out = l.lease(id)
		return true, nil
	})
	return out, err
}

// Release implements [lease.Store.Release].
func (s *Store) Release(ctx context.Context, leaseHash, identity string, now time.Time) error {
	return s.write(ctx, func(d *document) (bool, error) {
		changed := d.prune(now)
		if id, ok := d.byHash(leaseHash); ok && leaseHash != "" && d.Leases[id].Identity == identity {
			delete(d.Leases, id)
			changed = true
		}
		return changed, nil
	})
}

// Get implements [lease.Store.Get].
func (s *Store) Get(ctx context.Context, ids []string, now time.Time) (map[string]lease.Lease, error) {
	out := make(map[string]lease.Lease)
	if len(ids) == 0 {
		return out, nil
	}
	err := s.read(ctx, func(d *document) {
		for _, id := range ids {
			if l, ok := d.Leases[id]; ok && l.live(now) {
				out[id] = l.lease(id)
			}
		}
	})
	return out, err
}

// ListByHolder implements [lease.Store.ListByHolder].
func (s *Store) ListByHolder(ctx context.Context, identity string, now time.Time) ([]lease.Lease, error) {
	out := []lease.Lease{}
	err := s.read(ctx, func(d *document) {
		for id, l := range d.Leases {
			if l.Identity == identity && l.live(now) {
				out = append(out, l.lease(id))
			}
		}
	})
	slices.SortFunc(out, func(a, b lease.Lease) int {
		if c := a.Since.Compare(b.Since); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, err
}

// Rekey implements [lease.Store.Rekey].
func (s *Store) Rekey(ctx context.Context, from, to string) error {
	if from == to {
		return nil
	}
	return s.write(ctx, func(d *document) (bool, error) {
		l, ok := d.Leases[from]
		if !ok {
			return false, nil
		}
		delete(d.Leases, from)
		d.Leases[to] = l
		return true, nil
	})
}

// Drop implements [lease.Store.Drop].
func (s *Store) Drop(ctx context.Context, id string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		if _, ok := d.Leases[id]; !ok {
			return false, nil
		}
		delete(d.Leases, id)
		return true, nil
	})
}

// Version implements [lease.Store.Version]. It reads only [VersionKey],
// without the lock: the KV replaces the value whole, so a reader sees the old
// or the new number, never a mix. A missing key is version 0.
func (s *Store) Version(ctx context.Context) (uint64, error) {
	data, err := s.kv.Get(ctx, VersionKey)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("kvlease: read %s: %w", VersionKey, err)
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("kvlease: %s is not a number", VersionKey)
	}
	return v, nil
}

// Close implements [lease.Store.Close]. The store holds no resources of its
// own.
func (s *Store) Close() error { return nil }
