// Package kvlease is the [lease.Store] backend for the single-process tiers
// (fs, memory, sqlite, desktop), keeping every lease in one document in a
// [state.KV].
//
// # One document, read-modify-write under a lock
//
// All leases live in one JSON value under [StateKey], with the token counter
// in its header. Every method takes the store's one-slot semaphore and then
// its [Locker], re-reads the document, applies its change and writes it back
// through the KV's own write path (an atomic replace on disk). That makes
// each method atomic: one claim wins, and the caps hold. The semaphore is a
// channel rather than a mutex so a caller waiting for it stops when its
// context ends.
//
// # Several processes, one machine
//
// The desktop app, `rela mcp`, CLI calls and `rela scheduler` can all open
// one project, and each builds its own Store over the same cache-directory
// document. The semaphore only excludes callers of one instance, so the
// Locker must exclude every other process too: over an on-disk KV that is a
// filelock.Locker on a lock file next to the document. Several machines are
// out of scope; the postgres build uses pglease for that.
//
// # The version key
//
// Watchers poll [Store.Version] to learn about writes made by other
// processes. It reads only the small [VersionKey] value, written under the
// lock after the document, so a poll never parses the lease document. The
// version key also stays above every token issued, so tokens keep
// increasing after the document is deleted or reset. A version key that
// does not parse counts as missing, with one warning per Store; tokens then
// rest on the document's counter and the microsecond floor.
//
// On unix the poll reads without the lock: the KV replaces the value whole,
// so a reader sees the old or the new number. Windows refuses to rename a
// file over one another process has open, so there an unlocked poll could
// make a writer's version write fail. The Store reads the version under the
// lock on windows instead.
//
// # A damaged document is reset
//
// Leases expire within the hour and their holders renew them, so a document
// that does not parse is replaced by an empty one. The first load that finds
// it damaged writes the empty document back under the lock, with one warning
// that names the key but not the contents. Unlike piles, nothing a person
// collected by hand is lost.
package kvlease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
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
// until the lock is held or ctx ends, and returns the release func. A KV no
// other process can open takes filelock.ProcessPrivate.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// Store is a [lease.Store] over a [state.KV].
type Store struct {
	kv     state.KV
	locker Locker
	// sem is a one-slot semaphore over this instance's callers.
	sem chan struct{}
	// lockVersion makes Version read under the lock (windows; see the
	// package doc).
	lockVersion bool
	// warnedVersion keeps the warning about an unparsable version key to one.
	warnedVersion atomic.Bool
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
	// versionKey is the [VersionKey] value read at load, so one write reads
	// it once.
	versionKey uint64
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
		Since:     s.Since.UTC(),
		Expires:   s.Expires.UTC(),
		Status:    s.Status,
	}
}

func (s storedLease) live(now time.Time) bool { return now.Before(s.Expires) }

// change says what a write did to the document.
type change int

const (
	// unchanged writes nothing.
	unchanged change = iota
	// quiet writes the document without moving the version: only expired
	// leases went, which no reader sees.
	quiet
	// visible writes the document and moves the version.
	visible
)

// New returns a store backed by kv, with locker excluding other processes.
//
// Nil: both rejected. A nil KV would defer the failure to the first claim;
// a nil locker would hide the choice between filelock.Locker and
// filelock.ProcessPrivate.
func New(kv state.KV, locker Locker) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvlease: state KV must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("kvlease: locker must be non-nil")
	}
	return &Store{kv: kv, locker: locker, sem: make(chan struct{}, 1), lockVersion: runtime.GOOS == "windows"}, nil
}

// lock takes the semaphore and then the locker, and returns the release of
// both. It stops waiting when ctx ends.
func (s *Store) lock(ctx context.Context) (func(), error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("kvlease: lock %s: %w", StateKey, ctx.Err())
	}
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		<-s.sem
		return nil, fmt.Errorf("kvlease: lock %s: %w", StateKey, err)
	}
	return func() {
		unlock()
		<-s.sem
	}, nil
}

// load reads the document. A missing document is empty. A damaged one is
// reset to empty and written back. Caller holds the lock.
//
// The token counter starts no lower than the version key, which flush keeps
// above every token issued.
func (s *Store) load(ctx context.Context) (*document, error) {
	vk, err := s.readVersionKey(ctx)
	if err != nil {
		// The document read below reports a broken KV; a version key that
		// alone cannot be read leaves the token on the counter and the
		// microsecond floor.
		vk = 0
	}
	d, damaged, err := s.parse(ctx)
	if err != nil {
		return nil, err
	}
	d.versionKey = vk
	d.Token = max(d.Token, vk)
	if damaged {
		// The decode error may quote the document, so only the key is logged.
		slog.Warn("kvlease: lease document is damaged; resetting it to an empty one", "key", StateKey)
		if err := s.flush(ctx, d, visible); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// parse reads and decodes the document. damaged reports a document that did
// not decode; d is then empty.
func (s *Store) parse(ctx context.Context) (d *document, damaged bool, err error) {
	d = &document{Leases: map[string]storedLease{}}
	data, err := s.kv.Get(ctx, StateKey)
	if os.IsNotExist(err) {
		return d, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("kvlease: read %s: %w", StateKey, err)
	}
	if err := json.Unmarshal(data, d); err != nil {
		return &document{Leases: map[string]storedLease{}}, true, nil //nolint:nilerr // reset by design; see load
	}
	if d.Leases == nil {
		d.Leases = map[string]storedLease{}
	}
	return d, false, nil
}

// flush writes the document and, for a visible change, the version key.
// Caller holds the lock.
//
// A visible change sets the version one past the largest of the document's
// version, the version key and the token counter. So it moves on after the
// document was deleted, and it stays above every token issued, which load
// relies on. A failed version write is logged, not returned: the change
// itself is stored, and reporting it as failed would make a caller retry a
// claim it already holds. Watchers then see the change at the next write.
func (s *Store) flush(ctx context.Context, d *document, c change) error {
	if c == visible {
		d.Version = max(d.Version, d.Token, d.versionKey) + 1
	}
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("kvlease: encode %s: %w", StateKey, err)
	}
	if err := s.kv.Put(ctx, StateKey, data); err != nil {
		return fmt.Errorf("kvlease: write %s: %w", StateKey, err)
	}
	if c != visible {
		return nil
	}
	if err := s.kv.Put(ctx, VersionKey, []byte(strconv.FormatUint(d.Version, 10))); err != nil {
		slog.Warn("kvlease: could not write the lease version", "key", VersionKey, "error", err)
		return nil
	}
	d.versionKey = d.Version
	return nil
}

// readVersionKey returns the version key's value. A missing key is 0, and so
// is one that does not parse, after one warning per Store.
func (s *Store) readVersionKey(ctx context.Context) (uint64, error) {
	data, err := s.kv.Get(ctx, VersionKey)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("kvlease: read %s: %w", VersionKey, err)
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		if !s.warnedVersion.Swap(true) {
			slog.Warn("kvlease: lease version is not a number; treating it as missing", "key", VersionKey)
		}
		return 0, nil
	}
	return v, nil
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

// write runs fn over the document under the lock and writes back what fn
// reports, also when fn returns an error (a refused claim still keeps its
// pruning, quietly).
func (s *Store) write(ctx context.Context, fn func(d *document) (change, error)) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.load(ctx)
	if err != nil {
		return err
	}
	c, fnErr := fn(d)
	if c != unchanged {
		if err := s.flush(ctx, d, c); err != nil {
			return err
		}
	}
	return fnErr
}

// prune removes every lease that is expired at now: a quiet change when it
// removed any.
func (d *document) prune(now time.Time) change {
	n := len(d.Leases)
	for id, l := range d.Leases {
		if !l.live(now) {
			delete(d.Leases, id)
		}
	}
	if len(d.Leases) != n {
		return quiet
	}
	return unchanged
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

// liveFor returns the entity id of the lease with leaseHash when it is live
// at now and identity holds it.
func (d *document) liveFor(leaseHash, identity string, now time.Time) (string, bool) {
	id, ok := d.byHash(leaseHash)
	if !ok || d.Leases[id].Identity != identity || !d.Leases[id].live(now) {
		return "", false
	}
	return id, true
}

// Claim implements [lease.Store.Claim].
func (s *Store) Claim(
	ctx context.Context, id string, h lease.Holder, leaseHash string, now, until time.Time, status string,
) (lease.Lease, error) {
	if err := lease.CheckClaim(id, h, leaseHash, now, until, status); err != nil {
		return lease.Lease{}, err
	}
	now, until = lease.Instant(now), lease.Instant(until)
	var out lease.Lease
	err := s.write(ctx, func(d *document) (change, error) {
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
		return visible, nil
	})
	return out, err
}

// Renew implements [lease.Store.Renew].
func (s *Store) Renew(
	ctx context.Context, leaseHash, identity string, now, until time.Time, status *string,
) (lease.Lease, error) {
	if err := lease.CheckRenew(leaseHash, now, until, status); err != nil {
		return lease.Lease{}, err
	}
	now, until = lease.Instant(now), lease.Instant(until)
	var out lease.Lease
	err := s.write(ctx, func(d *document) (change, error) {
		pruned := d.prune(now)
		id, ok := d.liveFor(leaseHash, identity, now)
		if !ok {
			return pruned, lease.ErrLost
		}
		l := d.Leases[id]
		seen := until.Before(l.Expires) || (status != nil && *status != l.Status)
		l.Expires = until
		if status != nil {
			l.Status = *status
		}
		d.Leases[id] = l
		out = l.lease(id)
		if !seen {
			return quiet, nil // a plain extension; see the Version section of lease.Store
		}
		return visible, nil
	})
	return out, err
}

// Lookup implements [lease.Store.Lookup].
func (s *Store) Lookup(ctx context.Context, leaseHash, identity string, now time.Time) (lease.Lease, error) {
	if err := lease.CheckHash(leaseHash); err != nil {
		return lease.Lease{}, err
	}
	now = lease.Instant(now)
	var (
		out   lease.Lease
		found bool
	)
	err := s.read(ctx, func(d *document) {
		var id string
		if id, found = d.liveFor(leaseHash, identity, now); found {
			out = d.Leases[id].lease(id)
		}
	})
	if err != nil {
		return lease.Lease{}, err
	}
	if !found {
		return lease.Lease{}, lease.ErrLost
	}
	return out, nil
}

// Release implements [lease.Store.Release].
func (s *Store) Release(ctx context.Context, leaseHash, identity string, now time.Time) error {
	if err := lease.CheckHash(leaseHash); err != nil {
		return err
	}
	now = lease.Instant(now)
	return s.write(ctx, func(d *document) (change, error) {
		c := d.prune(now)
		if id, ok := d.liveFor(leaseHash, identity, now); ok {
			delete(d.Leases, id)
			c = visible
		}
		return c, nil
	})
}

// Get implements [lease.Store.Get].
func (s *Store) Get(ctx context.Context, ids []string, now time.Time) (map[string]lease.Lease, error) {
	out := make(map[string]lease.Lease)
	if len(ids) == 0 {
		return out, nil
	}
	now = lease.Instant(now)
	err := s.read(ctx, func(d *document) {
		for _, id := range ids {
			if l, ok := d.Leases[id]; ok && l.live(now) {
				got := l.lease(id)
				got.LeaseHash = ""
				out[id] = got
			}
		}
	})
	return out, err
}

// ListByHolder implements [lease.Store.ListByHolder].
func (s *Store) ListByHolder(ctx context.Context, identity string, now time.Time) ([]lease.Lease, error) {
	now = lease.Instant(now)
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

// NextExpiry implements [lease.Store.NextExpiry] with a scan: the document
// holds at most [lease.MaxPerStore] leases.
func (s *Store) NextExpiry(ctx context.Context, now time.Time) (time.Time, bool, error) {
	now = lease.Instant(now)
	var (
		next  time.Time
		found bool
	)
	err := s.read(ctx, func(d *document) {
		for _, l := range d.Leases {
			if l.live(now) && (!found || l.Expires.Before(next)) {
				next, found = l.Expires.UTC(), true
			}
		}
	})
	if err != nil {
		return time.Time{}, false, err
	}
	return next, found, nil
}

// Rekey implements [lease.Store.Rekey].
func (s *Store) Rekey(ctx context.Context, from, to string, now time.Time) error {
	if from == "" || to == "" {
		return fmt.Errorf("%w: entity id must be non-empty", lease.ErrInvalid)
	}
	if from == to {
		return nil
	}
	now = lease.Instant(now)
	return s.write(ctx, func(d *document) (change, error) {
		l, ok := d.Leases[from]
		if !ok {
			return unchanged, nil
		}
		delete(d.Leases, from)
		if !l.live(now) {
			return quiet, nil
		}
		if cur, ok := d.Leases[to]; ok && cur.live(now) {
			return visible, nil
		}
		d.Token = lease.NextToken(d.Token, now)
		l.Token = d.Token
		d.Leases[to] = l
		return visible, nil
	})
}

// Drop implements [lease.Store.Drop].
func (s *Store) Drop(ctx context.Context, id string, now time.Time) error {
	now = lease.Instant(now)
	return s.write(ctx, func(d *document) (change, error) {
		l, ok := d.Leases[id]
		if !ok {
			return unchanged, nil
		}
		delete(d.Leases, id)
		if !l.live(now) {
			return quiet, nil
		}
		return visible, nil
	})
}

// Version implements [lease.Store.Version]. It reads only [VersionKey]; a
// missing or unparsable key is version 0.
func (s *Store) Version(ctx context.Context) (uint64, error) {
	if s.lockVersion {
		unlock, err := s.lock(ctx)
		if err != nil {
			return 0, err
		}
		defer unlock()
	}
	return s.readVersionKey(ctx)
}

// Close implements [lease.Store.Close]. The store holds no resources of its
// own.
func (s *Store) Close() error { return nil }
