package audit

import "context"

// principalKey / triggeredByKey are unexported context.WithValue keys
// so no other package can collide with them or read/write the values
// outside this package's API.
type principalKey struct{}
type triggeredByKey struct{}

// WithPrincipal returns a derived context carrying p. Entry points
// stamp Principal once at startup (or per-request for data-entry,
// later); Manager reads it via [PrincipalFrom] on each write.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the Principal carried by ctx, or
// Principal{User:"unknown", Tool:"unknown"} if none was stamped.
// Returning a default rather than panicking keeps audit best-effort
// even when a new call site forgets to thread principal through —
// the misattribution is visible in the audit log, not silent.
func PrincipalFrom(ctx context.Context) Principal {
	if v, ok := ctx.Value(principalKey{}).(Principal); ok {
		return v
	}
	return Principal{User: "unknown", Tool: "unknown"}
}

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
