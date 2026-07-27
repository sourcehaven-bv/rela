package affordances

import "context"

// historicalSubjectKey marks a context as resolving a HISTORICAL entity
// snapshot (a version read of a possibly-deleted or drifted entity) rather
// than the live entity. See [WithHistoricalSubject].
type historicalSubjectKey struct{}

// WithHistoricalSubject marks ctx as resolving field visibility for a
// historical snapshot (TKT-73C6B2). Under this marker the binding context
// treats subject-world graph lookups (has_relation / count_relations, which
// funnel through outgoingCounts) as UNRESOLVABLE — the live store no longer
// holds the entity's as-of-version edges, so trusting it would let a
// conditional `visible:` grant flip OPEN for a deleted/drifted entity and leak
// a field hidden at write time. With no edges to affirm the grant, the
// predicate evaluates false and the field FAILS CLOSED (hidden).
//
// Reader-side inputs are deliberately NOT touched: current_user stays live,
// and has_role degrades on its own (a deleted entity confers no local roles,
// so the principal keeps only global roles) — that is a safe, intended
// degradation, not a leak. Only the subject's own outgoing-edge state, which
// the live store cannot answer as-of-version, is neutered here.
//
// The history read path sets this before serializing a snapshot; a reader
// holding acl.PermHistoryReadRedacted bypasses the resulting redaction at the
// handler (OVERRIDE reveal), so this marker only governs the fail-closed
// default for ordinary readers.
func WithHistoricalSubject(ctx context.Context) context.Context {
	return context.WithValue(ctx, historicalSubjectKey{}, true)
}

// isHistoricalSubject reports whether ctx was marked by [WithHistoricalSubject].
func isHistoricalSubject(ctx context.Context) bool {
	v, _ := ctx.Value(historicalSubjectKey{}).(bool)
	return v
}
