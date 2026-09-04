// Package userstate holds per-user, per-suggestion state for the
// next-action layer: snoozes, muted sources, and last-shown timestamps.
//
// # Why this is not in the graph
//
// A snooze is not a fact about an entity; it is a fact about one person's
// relationship to a suggestion at a moment. Storing it as an entity would
// make it visible to everyone, audited forever in the append-only log, and
// (on postgres) fed through the version-capture sweep. Last-shown is worse
// still: cooldown means writing on RENDER, so a suggestion surface would
// generate graph writes merely by being looked at.
//
// So this is a separate service with its own backends, deliberately outside
// store.Store. It is disposable state — losing it costs a user a repeated
// suggestion, not data.
//
// # Backends
//
// Implementations live in subpackages and are selected at wiring time the
// same way store backends are (see internal/appbuild). Every implementation
// must pass [userstatetest.RunAll] — three backends without a shared contract
// test is three subtly different behaviors, and the one that diverges will be
// the one nobody runs locally.
//
// # Time is injected, never read
//
// No method reads the wall clock. Expiry is evaluated against a caller-
// supplied `now`, which is what lets the conformance suite pin TTL semantics
// deterministically across backends. This mirrors predicatefns.Bind's
// treatment of today() and exists for the same reason: a local-vs-UTC skew
// of one day was a real bug there (RR-YPYTP), and snooze windows are exactly
// as vulnerable.
package userstate

import (
	"context"
	"errors"
	"time"
)

// ErrClosed is returned by every method once Close has been called.
var ErrClosed = errors.New("userstate: store is closed")

// Key identifies one suggestion for one user.
//
// The optional Variant carries the values of the source's configured
// key_props, which is what lets a condition that RESETS be recognized as new:
// a proposal going draft -> sent -> draft yields a different Variant, so an
// older snooze no longer matches and the new stall surfaces. Without it the
// key is stable across a reset and the suggestion stays silently suppressed.
//
// EntityID is empty for a count-based (entity-less) source such as first-run,
// where the key degenerates to the source alone.
type Key struct {
	User     string
	Source   string
	EntityID string
	Variant  string
}

// Snooze suppresses one suggestion until a deadline.
type Snooze struct {
	Key   Key
	Until time.Time
}

// Store is the per-user state the next-action engine needs.
//
// Consumer-side interface: the engine declares the minimum it uses, and the
// wiring site supplies a backend. Implementations must be safe for concurrent
// use — a single user can have several tabs open, and the disk and postgres
// backends are reachable from multiple goroutines and processes respectively.
type Store interface {
	// SnoozedUntil reports the deadline for key, and whether a live snooze
	// exists at all. An expired snooze reports ok=false: expiry is a read-time
	// judgement against `now`, so no backend depends on a sweeper having run.
	SnoozedUntil(ctx context.Context, key Key, now time.Time) (until time.Time, ok bool, err error)

	// SetSnooze records a snooze until `until`. A second call for the same key
	// REPLACES the first rather than stacking: "not now, try tomorrow" after
	// "not for an hour" should mean tomorrow, not an hour plus a day.
	SetSnooze(ctx context.Context, key Key, until time.Time) error

	// Muted reports whether the user has muted this source entirely.
	Muted(ctx context.Context, user, source string) (bool, error)

	// SetMuted mutes or unmutes a source for a user. Muting is per-SOURCE,
	// never per-entity: a per-entity mute is invisible state that nobody can
	// find later, whereas the handful of configured sources make "what have I
	// turned off?" a short, reversible list.
	SetMuted(ctx context.Context, user, source string, muted bool) error

	// MutedSources lists a user's muted source ids, sorted. Backs the
	// settings screen that makes muting discoverable — the property that
	// justifies choosing per-source mute over per-entity in the first place.
	MutedSources(ctx context.Context, user string) ([]string, error)

	// LastShown reports when this suggestion was last surfaced, and whether
	// it ever was. Drives cooldown.
	LastShown(ctx context.Context, key Key) (at time.Time, ok bool, err error)

	// SnoozedUntilMany is SnoozedUntil for a batch: the live snooze deadline
	// per key that has one, in one round-trip (TKT-1U8XYN — the next-action
	// engine judges every candidate of a source and paid two lookups each).
	// A key with no live snooze is absent from the result.
	SnoozedUntilMany(ctx context.Context, keys []Key, now time.Time) (map[Key]time.Time, error)

	// LastShownMany is LastShown for a batch; keys never shown are absent.
	LastShownMany(ctx context.Context, keys []Key) (map[Key]time.Time, error)

	// MarkShown records that the suggestion was surfaced at `at`.
	MarkShown(ctx context.Context, key Key, at time.Time) error

	// Prune drops state that can no longer affect a decision: snoozes whose
	// deadline has passed, and last-shown records older than `keepShown`.
	// Returns how many records were removed.
	//
	// Purely a housekeeping optimisation — reads already ignore expired
	// snoozes, so a backend that never prunes stays CORRECT while growing
	// unboundedly. That split is deliberate: correctness must not depend on
	// a sweeper the operator might disable.
	Prune(ctx context.Context, now time.Time, keepShown time.Duration) (removed int, err error)

	// Close releases resources. Subsequent calls return ErrClosed.
	Close() error
}
