package entitymanager_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const triggerUnsetSchema = `
version: "1.0"
entities:
  todo:
    label: To-do
    id_prefix: "TODO-"
    properties:
      title: {type: string}
      status: {type: enum, values: [open, done]}
      completed_on: {type: date}
      note: {type: string}
  check:
    label: Check
    id_prefix: "CHK-"
    properties:
      title: {type: string}
      note: {type: string}
relations:
  has-check: {from: [todo], to: [check]}
`

// The reopen trigger removes completed_on; the create trigger removes a
// note the template or the caller supplied.
const triggerUnsetFlows = `
triggers:
  reopen:
    on: {entity: todo, property: status, becomes: open}
    outputs:
      - set: {unset: [completed_on]}
  scrub-note-on-create:
    on: {entity: todo, created: true}
    outputs:
      - set: {unset: [note]}
      - create:
          type: check
          properties: {title: "Check {{new.title}}", note: "drop me too"}
          relate: {type: has-check, from: subject}
  scrub-check-note:
    on: {entity: check, created: true}
    outputs:
      - set: {unset: [note]}
`

func newUnsetManager(t *testing.T) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(triggerUnsetSchema))
	if err != nil {
		t.Fatal(err)
	}
	set, err := transitions.CompileTransitions(meta)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := triggers.Parse([]byte(triggerUnsetFlows), meta)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := triggers.New(meta, rules)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: set,
		Automations: engine, Cascade: runner,
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return mgr, st
}

func createDoneTodo(t *testing.T, mgr *entitymanager.Manager) *entity.Entity {
	t.Helper()
	res, err := mgr.CreateEntity(context.Background(), &entity.Entity{Type: "todo", Properties: map[string]any{
		"title": "Buy milk", "status": "done", "completed_on": "2026-11-20", "note": "drop me",
	}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Entity
}

// TestTriggerUnset_ReopenClearsCompletedOn runs the reopen through every
// update path: the trigger's removal lands on the stored row.
func TestTriggerUnset_ReopenClearsCompletedOn(t *testing.T) {
	tests := []struct {
		name  string
		write func(ctx context.Context, m *entitymanager.Manager, e *entity.Entity) error
	}{
		{"patch", func(ctx context.Context, m *entitymanager.Manager, e *entity.Entity) error {
			_, err := m.PatchEntity(ctx, e.ID, entity.Patch{Properties: map[string]any{"status": "open"}})
			return err
		}},
		{"patch also setting completed_on", func(ctx context.Context, m *entitymanager.Manager, e *entity.Entity) error {
			// The trigger runs after the caller's changes and wins.
			_, err := m.PatchEntity(ctx, e.ID, entity.Patch{Properties: map[string]any{
				"status": "open", "completed_on": "2026-11-21",
			}})
			return err
		}},
		{"update", func(ctx context.Context, m *entitymanager.Manager, e *entity.Entity) error {
			next := e.Clone()
			next.Properties["status"] = "open"
			_, err := m.UpdateEntity(ctx, next)
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mgr, st := newUnsetManager(t)
			created := createDoneTodo(t, mgr)
			if err := tc.write(ctx, mgr, created); err != nil {
				t.Fatal(err)
			}
			stored, err := st.GetEntity(ctx, created.Ref())
			if err != nil {
				t.Fatal(err)
			}
			if v, ok := stored.Properties["completed_on"]; ok {
				t.Errorf("completed_on = %v after reopen, want it removed", v)
			}
			if stored.GetString("status") != "open" || stored.GetString("title") != "Buy milk" {
				t.Errorf("reopen changed other properties: %v", stored.Properties)
			}
		})
	}
}

// TestTriggerUnset_OnCreate pins the create path: the removal is applied
// to the stored row in the post-create compare-and-swap.
func TestTriggerUnset_OnCreate(t *testing.T) {
	mgr, st := newUnsetManager(t)
	created := createDoneTodo(t, mgr)
	if _, ok := created.Properties["note"]; ok {
		t.Errorf("create result still carries note: %v", created.Properties)
	}
	stored, err := st.GetEntity(context.Background(), created.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stored.Properties["note"]; ok {
		t.Errorf("stored row still carries note: %v", stored.Properties)
	}
	if stored.GetString("completed_on") == "" {
		t.Error("the create trigger removed a property it does not name")
	}
}

// TestTriggerUnset_OnCascadeCreate pins the cascade path: an entity a
// trigger creates gets its own on-create removal written to the store.
func TestTriggerUnset_OnCascadeCreate(t *testing.T) {
	ctx := context.Background()
	mgr, st := newUnsetManager(t)
	createDoneTodo(t, mgr)
	var checks []*entity.Entity
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: "check", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks, e)
	}
	if len(checks) != 1 {
		t.Fatalf("got %d checks, want 1", len(checks))
	}
	if v, ok := checks[0].Properties["note"]; ok {
		t.Errorf("cascade-created check kept note = %v", v)
	}
}
