package entitymanager

import "context"

// CascadeHostDelete runs the delete an automation's if_exists: replace
// performs, through the cascade host m would hand the runner. Test-only: it
// lets the external tests drive that path on a faced family, which the
// runner itself cannot reach because a cascade cannot create a faced entity.
func CascadeHostDelete(ctx context.Context, m *Manager, id string, cascade bool) error {
	return (&cascadeHost{deps: m.deps}).DeleteEntity(ctx, "", id, cascade)
}
