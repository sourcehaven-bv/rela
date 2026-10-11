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
// the store's data cannot be used to renew or release a lease. Renew and
// release need the lease hash AND the identity that claimed it.
//
// # Keys and time
//
// A lease is keyed by entity id: the whole face family, never one face. Every
// method that decides whether a lease is still live takes the evaluation
// instant `now` from its caller, so a fake clock drives every backend the
// same way. A lease is live while now is before its expiry. Expired leases are
// filtered out of every read and removed by the next claim; there is no
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
// # Backends
//
// Implementations live in subpackages and are selected at wiring time
// (internal/appbuild). Every implementation must pass leasetest.RunAll.
package lease

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// MaxPerIdentity is the number of live leases one identity may hold.
	// Local agents usually share one principal, so this cap is shared by
	// every agent running as that user.
	MaxPerIdentity = 32
	// MaxPerStore is the number of live leases the whole store holds.
	MaxPerStore = 2048
)

var (
	// ErrLost reports a renew that cannot proceed: the lease expired, was
	// released or dropped, or belongs to another identity. The causes are one
	// error on purpose. The caller's answer is the same for each (claim
	// again), and telling them apart would confirm that a guessed lease hash
	// exists.
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
	// Current is the live lease. Its LeaseHash is always empty: the hash
	// is matched on renew and release, so it is never handed to a competing
	// claimant.
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
	// Label tells apart agents that share one identity.
	Label string
}

// Lease is one live claim on an entity.
type Lease struct {
	// ID is the entity id the lease covers.
	ID     string
	Holder Holder
	// LeaseHash is the hash of the lease id returned to the claimant.
	LeaseHash string
	// Token is the fencing token issued at claim. Renew keeps it.
	Token   uint64
	Since   time.Time
	Expires time.Time
	// Status is the holder's free-text note on what they are doing.
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

// CheckClaim validates the arguments of [Store.Claim]. Backends call it first,
// so they refuse the same input.
func CheckClaim(id string, h Holder, leaseHash string, now, until time.Time) error {
	switch {
	case id == "":
		return fmt.Errorf("%w: entity id must be non-empty", ErrInvalid)
	case h.Identity == "":
		return fmt.Errorf("%w: holder identity must be non-empty", ErrInvalid)
	case leaseHash == "":
		return fmt.Errorf("%w: lease hash must be non-empty", ErrInvalid)
	}
	return CheckUntil(now, until)
}

// CheckUntil validates the expiry of a claim or renew: it must be after now.
func CheckUntil(now, until time.Time) error {
	if !until.After(now) {
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
type Store interface {
	// Claim takes the lease on entity id for h until until, with leaseHash
	// as its key and status as its note. It first removes every lease that
	// is expired at now. It returns *[HeldError] when id has a live lease
	// (whoever holds it, the same identity included: a second agent of one
	// user needs its own claim to fail), [ErrLimit] or [ErrFull] past a cap
	// (counting live leases only), and [ErrInvalid] for bad input or a
	// lease hash already in use.
	Claim(
		ctx context.Context, id string, h Holder, leaseHash string, now, until time.Time, status string,
	) (Lease, error)
	// Renew moves the expiry of the live lease with leaseHash to until, and
	// replaces its status when status is non-nil. The token is kept. It
	// returns [ErrLost] when no lease with that hash is live at now for
	// identity.
	Renew(ctx context.Context, leaseHash, identity string, now, until time.Time, status *string) (Lease, error)
	// Release removes the lease with leaseHash when identity holds it. It
	// returns nil whether or not such a lease existed, so the answer reveals
	// nothing about other people's leases. now is used to prune expired
	// leases where the backend rewrites its data anyway.
	Release(ctx context.Context, leaseHash, identity string, now time.Time) error
	// Get returns the leases on ids that are live at now, keyed by entity id.
	// An id without a live lease has no entry.
	Get(ctx context.Context, ids []string, now time.Time) (map[string]Lease, error)
	// ListByHolder returns identity's leases that are live at now, oldest
	// claim first.
	ListByHolder(ctx context.Context, identity string, now time.Time) ([]Lease, error)
	// Rekey moves the lease on entity from to entity to, after a rename.
	// A lease already on to is replaced: to names the renamed entity now.
	// A missing lease on from is not an error.
	Rekey(ctx context.Context, from, to string) error
	// Drop removes the lease on entity id, after a delete. A missing lease
	// is not an error.
	Drop(ctx context.Context, id string) error
	// Version returns a number that changes whenever the store's leases
	// change, through any process. Watchers poll it to learn about writes
	// made elsewhere. It is cheap: one small read.
	Version(ctx context.Context) (uint64, error)
	// Close releases the backend's own resources. A shared handle, such as
	// the store's connection pool, stays open.
	Close() error
}
