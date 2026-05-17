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
