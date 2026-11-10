package affordances

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// FlowEditCeiling narrows the writable properties of a row while a flow run
// holds it: a flow's `outside_edits:` in flows.yaml (TKT-XI7IDBS). It is a
// ceiling like the client one: [PolicyResolver.FieldVerdicts] applies it
// after the role union, and it only removes. Defined at the consumer; the
// wiring site supplies the flowedits implementation.
type FlowEditCeiling interface {
	// Governs reports whether some flow on entityType declares
	// `outside_edits:`. A row of any other type is never narrowed and costs
	// no lookup.
	Governs(entityType string) bool
	// PrimeRuns looks up the runs on the subject ids in one batch and
	// returns a ctx that answers [FlowEditCeiling.Withheld] for them
	// without another lookup. Read paths only: the answer is a snapshot.
	PrimeRuns(ctx context.Context, ids []string) (context.Context, error)
	// Withheld returns the properties of e that the runs on e keep
	// read-only for the caller, each with the rule that withholds it
	// (audit only). holds reports whether the caller holds a permission
	// on e. An error means the runs could not be read.
	Withheld(ctx context.Context, e *entity.Entity, holds func(permission string) bool) (map[string]string, error)
}

// WithFlowEdits wires the flow edit ceiling. Without it, or with nil, no
// row is narrowed.
func WithFlowEdits(c FlowEditCeiling) Option {
	return func(r *PolicyResolver) { r.flowEdits = c }
}

// Attribution rules of the flow edit ceiling that do not name a run.
const (
	// ruleFlowEditsUnavailable withholds every property of a governed row
	// when its runs could not be read: the ceiling fails closed, and the
	// audit row says why rather than blaming a role.
	ruleFlowEditsUnavailable = "outside_edits:unavailable"
	// ruleFlowEditsNotComputed withholds every property of a governed row
	// in verdicts computed under [ForVisibility], which skips the run
	// lookup. A caller that wrongly reads Writable from them fails closed.
	ruleFlowEditsNotComputed = "outside_edits:not-computed"
)

// kindReadOnly is the writable dimension's rule kind ("affordance:read-only").
const kindReadOnly = "read-only"

type forVisibilityKey struct{}

// ForVisibility marks ctx for a caller that reads only the Visible
// dimension of [PolicyResolver.FieldVerdicts]: redaction of a read. The
// flow edit ceiling then skips its run lookup, so redacting a page costs
// no lookup per row. Writable is not a write check under it: every
// property of a row the ceiling governs is read-only.
func ForVisibility(ctx context.Context) context.Context {
	return context.WithValue(ctx, forVisibilityKey{}, true)
}

func isForVisibility(ctx context.Context) bool {
	v, _ := ctx.Value(forVisibilityKey{}).(bool)
	return v
}

// PrimeFlowRuns primes r's flow edit ceiling for a page of rows, so the
// field verdicts of the page cost one run lookup, not one per row. Call it
// where a page's `_fields` are built; a read that only redacts does not
// need it. Unprimed verdicts are still correct, only slower. A lookup
// error leaves ctx unprimed: each row then looks up on its own and fails
// closed on its own error.
//
// Nil: r is accepted (no ceiling).
func PrimeFlowRuns(ctx context.Context, r *PolicyResolver, rows []*entity.Entity) context.Context {
	if r == nil || r.flowEdits == nil {
		return ctx
	}
	ids := make([]string, 0, len(rows))
	for _, e := range rows {
		if e != nil && e.ID != "" && r.flowEdits.Governs(e.Type) {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return ctx
	}
	primed, err := r.flowEdits.PrimeRuns(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "affordances: priming flow runs failed; rows look up their own", "err", err)
		return ctx
	}
	return primed
}

// applyFlowEdits removes from writable the properties the flow edit ceiling
// withholds from the caller on e. Update verdicts of a live row only: a
// new row carries no run, and a snapshot is not written.
func applyFlowEdits(
	ctx context.Context, c FlowEditCeiling, req *acl.Request, e *entity.Entity, universe []string,
	writable *dimension,
) {
	if c == nil || e.ID == "" || !c.Governs(e.Type) {
		return
	}
	if isForVisibility(ctx) {
		writable.withhold(allRule(universe, ruleFlowEditsNotComputed), universe)
		return
	}
	held := map[string]bool{}
	holds := func(permission string) bool {
		v, ok := held[permission]
		if !ok {
			v = req.HoldsPermissionForEntity(ctx, e.ID, permission)
			held[permission] = v
		}
		return v
	}
	withheld, err := c.Withheld(ctx, e, holds)
	if err != nil {
		slog.WarnContext(ctx, "affordances: flow runs unreadable; every field read-only",
			"type", e.Type, "id", e.ID, "err", err)
		withheld = allRule(universe, ruleFlowEditsUnavailable)
	}
	writable.withhold(withheld, universe)
}

// allRule maps every field of universe to rule.
func allRule(universe []string, rule string) map[string]string {
	out := make(map[string]string, len(universe))
	for _, f := range universe {
		out[f] = rule
	}
	return out
}
