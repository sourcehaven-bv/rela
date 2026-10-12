// Package lease holds advisory entity leases: a short, renewable claim that
// says "this holder is working on this entity until this time".
//
// A lease is advisory. The token it carries is issued but not enforced on
// writes; it tells other people and agents that someone is busy, so they can
// pick other work.
//
// # What the store sees
//
// A claim returns an opaque lease id to its caller once. The store never sees
// that id: it stores and matches a hash of it (the lease hash), so a copy of
// the store's data cannot be used to renew or release a lease. Renew, lookup
// and release need the lease hash AND the identity that claimed it.
//
// # Keys and time
//
// A lease is keyed by entity id: the whole face family, never one face. Every
// method that decides whether a lease is still live takes the evaluation
// instant `now` from its caller, so a fake clock drives every backend the
// same way. A lease is live while now is before its expiry. Expired leases are
// filtered out of every read and removed by later writes; there is no
// separate reaper.
//
// # Tokens
//
// Every claim gets a fencing token from one store-wide counter:
// max(counter+1, now in microseconds). The counter makes tokens strictly
// increasing within the store's life, across release and reopen. The
// microsecond floor keeps them increasing after the counter itself is lost
// (a deleted cache document), as long as the clock has moved on.
//
// # Input checks
//
// [CheckClaim], [CheckRenew], [CheckHash], [CheckStatus] and [CheckLabel] are
// exported so the service in front of a store refuses the same input the
// store does, before it reaches the store.
//
// # Backends
//
// Implementations live in subpackages and are selected at wiring time
// (internal/appbuild). Every implementation must pass leasetest.RunAll.
package lease

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxPerIdentity is the number of live leases one identity may hold.
	// Local agents usually share one principal, so this cap is shared by
	// every agent running as that user.
	MaxPerIdentity = 32
	// MaxPerStore is the number of live leases the whole store holds.
	MaxPerStore = 2048
	// MaxStatusRunes is the longest status a holder may set.
	MaxStatusRunes = 140
	// hashLen is the length of a lease hash: a SHA-256 digest in hex.
	hashLen = 64
)

var (
	// ErrLost reports a renew or lookup that cannot proceed: the lease
	// expired, was released or dropped, or belongs to another identity. The
	// causes are one error on purpose. The caller's answer is the same for
	// each (claim again), and telling them apart would confirm that a guessed
	// lease hash exists.
	ErrLost = errors.New("lease: lease lost")
	// ErrLimit reports an identity that already holds [MaxPerIdentity] live
	// leases.
	ErrLimit = errors.New("lease: too many leases for this holder")
	// ErrFull reports a store that already holds [MaxPerStore] live leases.
	// The message names no holder and no count.
	ErrFull = errors.New("lease: lease store is full")
	// ErrInvalid reports invalid input; the wrapped message says which.
	ErrInvalid = errors.New("lease: invalid input")
)

// HeldError reports a claim on an entity that already has a live lease.
type HeldError struct {
	// Current is the live lease. Its LeaseHash is always empty, as from
	// [Store.Get].
	Current Lease
}

func (e *HeldError) Error() string {
	return fmt.Sprintf("lease: %s is held until %s", e.Current.ID, e.Current.Expires.UTC().Format(time.RFC3339))
}

// Holder says who holds a lease.
type Holder struct {
	// Identity is the key renew, release and the per-identity cap match on.
	Identity string
	// User and Tool are shown to other people.
	User string
	Tool string
	// Label tells apart agents that share one identity. See [CheckLabel].
	Label string
}

// Lease is one live claim on an entity.
type Lease struct {
	// ID is the entity id the lease covers.
	ID     string
	Holder Holder
	// LeaseHash is the hash of the lease id returned to the claimant. It is
	// set only on answers to the holder: [Store.Claim], [Store.Renew],
	// [Store.Lookup] and [Store.ListByHolder]. [Store.Get] and [HeldError]
	// serve other people and leave it empty, because the hash is what renew
	// and release match on.
	LeaseHash string
	// Token is the fencing token issued at claim. Renew keeps it.
	Token   uint64
	Since   time.Time
	Expires time.Time
	// Status is the holder's free-text note on what they are doing. See
	// [CheckStatus].
	Status string
}

// Live reports whether the lease is live at now.
func (l Lease) Live(now time.Time) bool { return now.Before(l.Expires) }

// NextToken returns the token after last for a claim at now:
// max(last+1, now in microseconds). A now before the Unix epoch counts as 0.
func NextToken(last uint64, now time.Time) uint64 {
	floor := uint64(0)
	if us := now.UnixMicro(); us > 0 {
		floor = uint64(us)
	}
	return max(last+1, floor)
}

// Instant normalizes a time a caller hands to a [Store]: UTC, truncated to
// the microsecond, without a monotonic clock reading. PostgreSQL keeps
// microseconds, so every backend applies Instant to each now and until at
// its boundary. Then all backends store, compare and return equal values.
func Instant(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// labelPattern is the shape of a holder label: short and plain, because it
// is shown next to other people's work.
var labelPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{0,32}$`)

// CheckStatus validates a holder's status note. It must be valid UTF-8 of at
// most [MaxStatusRunes] runes, without control characters (CR, LF and tab
// included) and without the Unicode bidi controls, which could make a status
// shown next to an entity read as something else.
func CheckStatus(status string) error {
	if !utf8.ValidString(status) {
		return fmt.Errorf("%w: status must be valid UTF-8", ErrInvalid)
	}
	if utf8.RuneCountInString(status) > MaxStatusRunes {
		return fmt.Errorf("%w: status must be at most %d characters", ErrInvalid, MaxStatusRunes)
	}
	for _, r := range status {
		if unicode.IsControl(r) || isBidiControl(r) {
			return fmt.Errorf("%w: status must not contain control characters", ErrInvalid)
		}
	}
	return nil
}

// isBidiControl reports whether r is a Unicode bidi embedding, override,
// isolate or mark.
func isBidiControl(r rune) bool {
	return (r >= '\u202A' && r <= '\u202E') || (r >= '\u2066' && r <= '\u2069') || r == '\u200E' || r == '\u200F'
}

// CheckLabel validates a holder label: at most 32 of A-Z, a-z, 0-9, '_',
// '.' and '-'. The empty label is valid.
func CheckLabel(label string) error {
	if !labelPattern.MatchString(label) {
		return fmt.Errorf("%w: label must be at most 32 letters, digits, '_', '.' or '-'", ErrInvalid)
	}
	return nil
}

// CheckHash validates a lease hash: exactly 64 lowercase hex digits, the
// form of a SHA-256 digest.
func CheckHash(leaseHash string) error {
	if len(leaseHash) != hashLen {
		return fmt.Errorf("%w: lease hash must be %d lowercase hex digits", ErrInvalid, hashLen)
	}
	for i := range len(leaseHash) {
		if c := leaseHash[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("%w: lease hash must be %d lowercase hex digits", ErrInvalid, hashLen)
		}
	}
	return nil
}

// CheckClaim validates the arguments of [Store.Claim]. Backends call it first,
// so they refuse the same input.
func CheckClaim(id string, h Holder, leaseHash string, now, until time.Time, status string) error {
	switch {
	case id == "":
		return fmt.Errorf("%w: entity id must be non-empty", ErrInvalid)
	case h.Identity == "":
		return fmt.Errorf("%w: holder identity must be non-empty", ErrInvalid)
	}
	if err := CheckLabel(h.Label); err != nil {
		return err
	}
	if err := CheckHash(leaseHash); err != nil {
		return err
	}
	if err := CheckStatus(status); err != nil {
		return err
	}
	return CheckUntil(now, until)
}

// CheckRenew validates the arguments of [Store.Renew]. A nil status is valid:
// it keeps the old one.
func CheckRenew(leaseHash string, now, until time.Time, status *string) error {
	if err := CheckHash(leaseHash); err != nil {
		return err
	}
	if status != nil {
		if err := CheckStatus(*status); err != nil {
			return err
		}
	}
	return CheckUntil(now, until)
}

// CheckUntil validates the expiry of a claim or renew: it must be after now,
// compared at [Instant] precision.
func CheckUntil(now, until time.Time) error {
	if !Instant(until).After(Instant(now)) {
		return fmt.Errorf("%w: expiry must be after now", ErrInvalid)
	}
	return nil
}

// Store is the backend contract for leases.
//
// Implementations must be safe for concurrent use and must apply each method
// atomically. Of any number of concurrent claims on one entity, exactly one
// wins, and the caps hold under concurrency. On the single-process tiers this
// includes other processes over the same cache directory.
//
// Every time a method takes is normalized with [Instant], and every time it
// returns is in that form.
//
// # Version
//
// [Store.Version] changes exactly when the set of live leases, or a field of
// one that readers see, changes: a claim, a renew, the release of a live
// lease, and a rekey or drop that moves or removes a live lease. Removing
// expired leases alone does not change it, and neither does a refused write,
// because no reader could see the difference. A lease that expires changes
// what readers see without any write; watchers learn of that from
// [Store.NextExpiry].
type Store interface {
	// Claim takes the lease on entity id for h until until, with leaseHash
	// as its key and status as its note. It may first remove leases that are
	// expired at now. It returns *[HeldError] when id has a live lease
	// (whoever holds it, the same identity included: a second agent of one
	// user needs its own claim to fail), [ErrLimit] or [ErrFull] past a cap
	// (counting live leases only), and [ErrInvalid] for bad input (see
	// [CheckClaim]) or a lease hash already in use.
	Claim(
		ctx context.Context, id string, h Holder, leaseHash string, now, until time.Time, status string,
	) (Lease, error)
	// Renew moves the expiry of the live lease with leaseHash to until, and
	// replaces its status when status is non-nil. The token is kept. It
	// returns [ErrLost] when no lease with that hash is live at now for
	// identity, and [ErrInvalid] for bad input (see [CheckRenew]).
	Renew(ctx context.Context, leaseHash, identity string, now, until time.Time, status *string) (Lease, error)
	// Lookup returns the lease with leaseHash that is live at now for
	// identity, or [ErrLost] for the same causes as Renew. A caller uses it
	// to check its access to the entity again before it renews. A malformed
	// hash is [ErrInvalid].
	Lookup(ctx context.Context, leaseHash, identity string, now time.Time) (Lease, error)
	// Release removes the lease with leaseHash when identity holds it. It
	// returns nil whether or not such a lease existed, so the answer reveals
	// nothing about other people's leases; only a malformed hash is
	// [ErrInvalid]. now decides whether the removed lease was live, for the
	// version.
	Release(ctx context.Context, leaseHash, identity string, now time.Time) error
	// Get returns the leases on ids that are live at now, keyed by entity id.
	// An id without a live lease has no entry. Every LeaseHash is empty.
	Get(ctx context.Context, ids []string, now time.Time) (map[string]Lease, error)
	// ListByHolder returns identity's leases that are live at now, oldest
	// claim first.
	ListByHolder(ctx context.Context, identity string, now time.Time) ([]Lease, error)
	// NextExpiry returns the earliest expiry after now across the store, and
	// false when no lease is live at now. A watcher sleeps until then to
	// notice a lease expiring, which no write announces.
	NextExpiry(ctx context.Context, now time.Time) (time.Time, bool, error)
	// Rekey moves the lease on entity from to entity to, after a rename.
	//
	// A live lease on from moves when to has no live lease, and takes a fresh
	// token: its old token may be lower than one a lease on to had before,
	// and tokens must increase per entity. When to has a live lease, that
	// lease stays and the one on from is removed. An expired lease on from is
	// removed; a missing one is not an error.
	Rekey(ctx context.Context, from, to string, now time.Time) error
	// Drop removes the lease on entity id, whoever holds it, after a delete.
	// A missing lease is not an error. The entity delete hook calls it
	// synchronously in the delete's write path, so it stays one keyed
	// removal. now decides whether the lease was live, for the version.
	Drop(ctx context.Context, id string, now time.Time) error
	// Version returns a number that changes as the Version section above
	// says, through any process. Watchers poll it to learn about writes
	// made elsewhere. It is cheap: one small read.
	Version(ctx context.Context) (uint64, error)
	// Close releases the backend's own resources. A shared handle, such as
	// the store's connection pool, stays open.
	Close() error
}
