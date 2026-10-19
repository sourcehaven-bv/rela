package audit

import "context"

// triggeredByKey is the unexported context.WithValue key so no other
// package can collide with it or read/write the value outside this
// package's API.
type triggeredByKey struct{}

// WithTriggeredBy returns a derived context carrying label. The
// autocascade runner stamps "automation:<name>"; the scheduler
// stamps "schedule:<task-name>"; cascade-deleted relations from a
// DeleteEntity cascade stamp "cascade:delete-entity:<id>".
func WithTriggeredBy(ctx context.Context, label string) context.Context {
	return context.WithValue(ctx, triggeredByKey{}, label)
}

// TriggeredByFrom returns the triggered-by label carried by ctx, or
// "" if none was stamped (the normal case for direct user actions).
func TriggeredByFrom(ctx context.Context) string {
	if v, ok := ctx.Value(triggeredByKey{}).(string); ok {
		return v
	}
	return ""
}

// assertedByKey is the context key of the asserting OS user.
type assertedByKey struct{}

// WithAssertedBy returns a derived context carrying the OS user who
// asserted the principal on ctx. The local CLI and stdio `rela mcp` stamp
// it when `--run-as` names the identity: that name is self-asserted, not
// verified, so the audit records who claimed it. It is attribution only
// and never reaches an ACL decision or a reader.
func WithAssertedBy(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, assertedByKey{}, user)
}

// AssertedByFrom returns the asserting OS user carried by ctx, or "" when
// the principal was not self-asserted (the normal case).
func AssertedByFrom(ctx context.Context) string {
	if v, ok := ctx.Value(assertedByKey{}).(string); ok {
		return v
	}
	return ""
}
