package flowedits

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

type withinKey struct{}

// within is what [WithinRun] marks: the run and the state it writes from.
type within struct {
	run   flowrun.RunID
	state string
}

// WithinRun marks ctx for a write that run makes itself from state: a
// completed step or an output. The rule of that state does not narrow the
// write; every other run on the subject still does, and so do the run's
// other states while it is in parallel branches (TKT-ZO9IB6V). It is a capability, not provenance: only the flow
// runtime sets it, at the two writes that are the run's own, and
// TestFlowEditsWithinRunCallers in internal/archguard pins the callers.
// Nothing a request carries reaches it.
func WithinRun(ctx context.Context, id flowrun.RunID, state string) context.Context {
	return context.WithValue(ctx, withinKey{}, within{run: id, state: state})
}
