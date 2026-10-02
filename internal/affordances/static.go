package affordances

import "context"

// staticGrantsKey marks a context as asking what a principal COULD see, with
// no entity in hand. See [WithStaticGrants].
type staticGrantsKey struct{}

// WithStaticGrants marks ctx so every `when:` on a field, visible or relation
// grant counts as passing. [PolicyResolver.FieldVerdicts] and
// [PolicyResolver.RelationFieldVerdicts] then answer the worst case for the
// principal: the most a row of the type could ever show them.
//
// It exists for offline analysis (`rela acl audit` cross-checking
// classification.yaml, TKT-8UCV32), which has a policy and a principal but no
// entity. Answering through the same resolver, rather than re-deriving the
// fold, keeps the analysis from drifting from the runtime: closed-world
// opt-in, the union across roles and the client ceiling all apply unchanged.
// Only the per-row predicate is replaced, and only in the direction that
// shows more.
//
// Never set this on a request path. It widens every grant PolicyResolver.passes
// decides: redaction, editable fields, options and relation grants alike.
// staticguard_test.go fails on a caller outside the allowlist.
func WithStaticGrants(ctx context.Context) context.Context {
	return context.WithValue(ctx, staticGrantsKey{}, true)
}

func isStaticGrants(ctx context.Context) bool {
	v, _ := ctx.Value(staticGrantsKey{}).(bool)
	return v
}
