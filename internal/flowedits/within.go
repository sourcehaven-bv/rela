package flowedits

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

type withinKey struct{}

// WithinRun marks ctx for a write that run makes itself: a completed step
// or an output. That run does not narrow the write; every other run on the
// subject still does. It is a capability, not provenance: only the flow
// runtime sets it, at the two writes that are the run's own, and
// TestFlowEditsWithinRunCallers in internal/archguard pins the callers.
// Nothing a request carries reaches it.
func WithinRun(ctx context.Context, id flowrun.RunID) context.Context {
	return context.WithValue(ctx, withinKey{}, id)
}
