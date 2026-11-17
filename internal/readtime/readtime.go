// Package readtime carries the read time of one operation on its context.
//
// A condition that reads `today()`, `now()`, `$today` or `$now` must see
// one value for the whole operation: the Go evaluation of every row, the
// store pushdown of the same condition and every list on the page. The
// request edge captures the time once: the data-entry router with [With]
// for every request, and appbuild.IdentityBinder (the CLI, MCP, rela-docs,
// the scheduler) with [Ensure]. Everything below reads it with [Or].
//
// The time is always UTC with whole seconds. `today()` is the UTC calendar
// day of it, so the day boundary is the same on every server, whatever its
// local zone. Whole seconds make `now()` and `$now` (written as RFC 3339,
// which has no fraction) the same instant.
//
// The package is a leaf with no dependencies, so every layer may import it.
package readtime

import (
	"context"
	"time"
)

type ctxKey struct{}

// With returns ctx carrying t, in UTC and truncated to the second, as the
// operation's read time. It replaces a time already on ctx: an edge starts a new operation,
// and a long-lived reader that rebinds per call (the MCP watch) must see the
// clock advance rather than keep the time of its first bind.
func With(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, ctxKey{}, Normalize(t))
}

// Ensure returns ctx with a read time: the one already on it, else clock()
// at this moment. A binder that may run below the edge of an operation
// (appbuild.IdentityBinder: a job rebinding identity, a CLI helper binding
// twice) uses it so it never moves the time within one operation.
//
// Nil: ctx is rejected (panic in context.WithValue); clock is rejected when
// ctx has no read time (panic), as in [Or].
func Ensure(ctx context.Context, clock func() time.Time) context.Context {
	if _, ok := From(ctx); ok {
		return ctx
	}
	return With(ctx, clock())
}

// From returns the read time on ctx, and whether there is one.
func From(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	t, ok := ctx.Value(ctxKey{}).(time.Time)
	return t, ok
}

// Or returns the read time on ctx, else fallback() in UTC. The fallback is
// for work that has no request edge: a validation run, an automation, a
// test. Such work reads its own clock once per call, as before.
//
// Nil: fallback is rejected — a nil fallback with no read time on ctx
// panics, because a zero time would silently make `today()` year 1.
func Or(ctx context.Context, fallback func() time.Time) time.Time {
	if t, ok := From(ctx); ok {
		return t
	}
	return Normalize(fallback())
}

// Normalize returns t as a read time: UTC, truncated to the second.
func Normalize(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}
