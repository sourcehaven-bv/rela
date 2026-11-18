package store

import (
	"context"
	"errors"
	"time"
)

// Point-in-time reads (TKT-6KLUT3U).
//
// A context bound to a read instant ([BindReadInstant]) makes every read of a
// supporting backend answer from the state the graph had at that instant, and
// makes every write fail with [ErrReadOnlyInstant]. The instant travels on ctx
// rather than as a second store value so every consumer built over the store
// at assembly (the ACL graph, the visibility readers, the tracer, Lua) reads
// the same state without being rewired.
//
// The state at instant T, per entity face and per relation:
//
//   - The live row, when it was last written at or before T and its version
//     history holds nothing edited after T.
//   - Otherwise the latest captured version edited at or before T, unless that
//     version is a delete or a purge, or a later rename or face conversion at
//     or before T moved the entity away from that address.
//
// Relations follow the same rule per relation record. A version's edit
// time is when its content became current, not when the sweep captured it
// (see the content_at columns), so a capture delayed by the sweep's idle
// window, or by a period with no sweep running, does not move the content in
// time. Edits that one sweep capture merged are one state.
//
// Supported by pgstore and sqlitestore, which keep the version history.
// fsstore and memstore keep none; [BindReadInstant] refuses them with
// [ErrInstantUnsupported], so an instant can never reach a backend that would
// answer with live data.

// ErrInstantUnsupported means the store keeps no version history to answer a
// point-in-time read from.
var ErrInstantUnsupported = errors.New(
	"store: point-in-time reads need version history (PostgreSQL or SQLite backend)")

// ErrReadOnlyInstant is the error of a write attempted under a read instant.
var ErrReadOnlyInstant = errors.New("store: a point-in-time read cannot write")

// InstantReader is the marker capability of a backend that answers reads
// under a [BindReadInstant] context from its version history.
type InstantReader interface {
	// ReadsAtInstant reports whether this store answers point-in-time reads.
	ReadsAtInstant() bool
}

// readInstantKey carries the read instant on ctx.
type readInstantKey struct{}

// BindReadInstant returns ctx bound to the instant at, for reads of st.
//
// It refuses a zero instant with [ErrInvalidQuery], and a store that keeps no
// version history with [ErrInstantUnsupported]. Decorators are looked
// through with [Innermost].
//
// Nil: st is rejected with [ErrInstantUnsupported].
func BindReadInstant(ctx context.Context, st Store, at time.Time) (context.Context, error) {
	if at.IsZero() {
		return ctx, ErrInvalidQuery
	}
	if !SupportsReadInstant(st) {
		return ctx, ErrInstantUnsupported
	}
	return context.WithValue(ctx, readInstantKey{}, at.UTC()), nil
}

// SupportsReadInstant reports whether st answers point-in-time reads.
//
// Nil: accepted; a nil st does not.
func SupportsReadInstant(st Store) bool {
	r, ok := Innermost(st).(InstantReader)
	return ok && r.ReadsAtInstant()
}

// ReadInstant returns the instant ctx is bound to, in UTC.
func ReadInstant(ctx context.Context) (time.Time, bool) {
	at, ok := ctx.Value(readInstantKey{}).(time.Time)
	return at, ok
}

// readTodayKey marks a ctx whose reads see today's state although an instant
// is bound.
type readTodayKey struct{}

// ReadToday returns ctx whose reads answer from today's state while the read
// instant stays bound, so every write is still refused with
// [ErrReadOnlyInstant]. It is for an ACL read gate a backend evaluates with
// separate store reads (graphquerynaive) rather than in SQL, which must read
// today's edges as the SQL gate does (DEC-TDEF3GD). Without an instant on ctx
// it returns ctx unchanged.
func ReadToday(ctx context.Context) context.Context {
	if _, ok := ReadInstant(ctx); !ok {
		return ctx
	}
	return context.WithValue(ctx, readTodayKey{}, true)
}

// ReadsToday reports whether ctx was marked by [ReadToday]. A backend's read
// seam answers such a read from its live tables.
func ReadsToday(ctx context.Context) bool {
	v, _ := ctx.Value(readTodayKey{}).(bool)
	return v
}
