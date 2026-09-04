package dataentry

import (
	"context"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// ctxRowGate adapts the dataentry per-request read gate to
// [visibility.RowGate]. Each call resolves the gate FROM CTX
// (readGateFromContext), so the wrapper inherits exactly the per-request
// nop/acl selection attachACLRequest sets up: under no ACL policy the nop
// gate permits everything (byte-parity), and under a policy the
// ctx-attached acl.Request provides the amortized per-request scope
// (RR-JJYW / RR-MXKD2O — dataentry already binds one Request per request,
// so no separate Bind call is needed here).
type ctxRowGate struct{}

// PermitsRead implements [visibility.RowGate].
func (ctxRowGate) PermitsRead(ctx context.Context, entityType, id string) (bool, error) {
	return readGateFromContext(ctx).PermitsRead(ctx, entityType, id)
}

// PermitsReadMany implements [visibility.RowGate].
func (ctxRowGate) PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error) {
	return readGateFromContext(ctx).PermitsReadMany(ctx, entityType, ids)
}

// PermittedFaces implements [visibility.FaceGate], from the same ctx-resolved
// gate the two row methods use.
//
// Without this the wrapper would row-gate but not face-gate, which was a
// cross-principal content leak: the view surface served a draft body to a
// principal granted only `policy@published` (TKT-O7R2A1).
func (ctxRowGate) PermittedFaces(ctx context.Context, entityType string) ([]entityPkg.Face, error) {
	return readGateFromContext(ctx).ReadQuery(ctx, entityType).Faces, nil
}

// affRedactor adapts the dataentry affordance service to
// [visibility.FieldRedactor]. The aff closure (not a captured value)
// keeps test builders free to swap App.affordances after construction.
// Under the Nop resolver hiddenProperties returns nil — nothing hidden —
// which is the correct no-policy parity, not a fail-open (the resolver's
// own failure modes resolve fail-closed inside affordances).
type affRedactor struct {
	aff func() affordanceService
}

// HiddenProperties implements [visibility.FieldRedactor].
func (a affRedactor) HiddenProperties(ctx context.Context, e *entityPkg.Entity) map[string]struct{} {
	return a.aff().hiddenProperties(ctx, e)
}
