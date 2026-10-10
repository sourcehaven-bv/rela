package entitymanager

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// CascadeHostDelete runs the delete an automation's if_exists: replace
// performs, through the cascade host m would hand the runner. Test-only: it
// lets the external tests drive that path on a faced family, which the
// runner itself cannot reach because a cascade cannot create a faced entity.
func CascadeHostDelete(ctx context.Context, m *Manager, id string, cascade bool) error {
	return (&cascadeHost{deps: m.deps}).DeleteEntity(ctx, "", id, cascade)
}

// CascadeHostWriteRelation writes r through the cascade host m would hand
// the runner, as an automation's create_relation does.
func CascadeHostWriteRelation(ctx context.Context, m *Manager, r *entity.Relation) error {
	return (&cascadeHost{deps: m.deps}).WriteRelation(ctx, r)
}

// CascadeHostCreate runs a cascade's create_entity through the cascade host.
// Test-only: the metamodel refuses an automation whose create_entity names
// an external ref, so the write-time backstop is reachable only from here.
func CascadeHostCreate(
	ctx context.Context, m *Manager, entityType, id string, props map[string]any,
) (*entity.Entity, error) {
	return (&cascadeHost{deps: m.deps}).CreateEntity(ctx, entityType,
		autocascade.CreateEntityOptions{ID: id, Properties: props})
}

// WriteAutomationProperties persists an automation `set:` onto created.
// Test-only, for the same reason as [CascadeHostCreate].
func WriteAutomationProperties(
	ctx context.Context, m *Manager, created *entity.Entity, set map[string]string,
) (*entity.Entity, error) {
	return writeAutomationProperties(ctx, m.deps, created, set, nil)
}

// BindAs runs the request edge for ctx against m's ACL: it resolves the
// stamped principal and attaches its request scope, as a transport does
// before it calls the manager. Test-only.
func BindAs(ctx context.Context, tb testing.TB, m *Manager) context.Context {
	tb.Helper()
	a := m.deps.ACL
	// A test decorator around the real policy names it, so the identity is
	// bound against the Declarative the decorator forwards to.
	if w, ok := a.(interface{ Declarative() *acl.Declarative }); ok {
		a = w.Declarative()
	}
	out, err := acl.BindRequest(ctx, a)
	if err != nil {
		tb.Fatalf("acl.BindRequest: %v", err)
	}
	return out
}
