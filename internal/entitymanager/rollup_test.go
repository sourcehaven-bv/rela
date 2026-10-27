package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const rollupMetaYAML = `
version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "R-"
    id_type: sequential
    properties:
      name: {type: string}
      plans:
        type: integer
        rollup: {via: treatedBy, count: true}
  plan:
    label: Plan
    id_prefix: "P-"
    id_type: sequential
    properties:
      name: {type: string}
relations:
  treats:
    from: [plan]
    to: [risk]
    inverse: treatedBy
types: {}
`

func newRollupManager(t *testing.T, st *memstore.MemStore) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(rollupMetaYAML))
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

// TestRollup_WritesRefused pins that a roll-up (TKT-6I2OPG) is never
// authored: a create or a patch naming it is refused, and a whole-entity
// update that round-trips a read keeps what is stored instead of storing
// the computed value.
func TestRollup_WritesRefused(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	mgr := newRollupManager(t, st)

	e := entity.New("", "risk")
	e.Properties["name"] = "r"
	e.Properties["plans"] = 3
	if _, err := mgr.CreateEntity(ctx, e, entity.CreateOptions{}); !errors.As(err, new(*entitymanager.ComputedWriteError)) {
		t.Fatalf("CreateEntity roll-up error = %T %v", err, err)
	}
	delete(e.Properties, "plans")
	created, err := mgr.CreateEntity(ctx, e, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	id := created.Entity.ID

	_, err = mgr.PatchEntity(ctx, id, entity.Patch{Properties: map[string]any{"plans": 4}})
	if !errors.As(err, new(*entitymanager.ComputedWriteError)) {
		t.Fatalf("PatchEntity roll-up error = %T %v", err, err)
	}
	_, err = mgr.PatchEntity(ctx, id, entity.Patch{MetaUnset: []string{"plans"}})
	if !errors.As(err, new(*entitymanager.ComputedWriteError)) {
		t.Fatalf("PatchEntity unset roll-up error = %T %v", err, err)
	}

	whole := created.Entity.Clone()
	whole.Properties["plans"] = 2 // as a read served it
	whole.Properties["name"] = "renamed"
	if _, err = mgr.UpdateEntity(ctx, whole); err != nil {
		t.Fatalf("UpdateEntity with a round-tripped roll-up: %v", err)
	}
	stored, err := st.GetEntity(ctx, created.Entity.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if _, has := stored.Properties["plans"]; has {
		t.Errorf("the roll-up value was stored: %v", stored.Properties)
	}
	if stored.Properties["name"] != "renamed" {
		t.Errorf("the rest of the update was lost: %v", stored.Properties)
	}
}
