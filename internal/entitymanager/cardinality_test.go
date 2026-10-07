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
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// cardinalityManager is a manager over a task/status schema (TKT-65LVAK):
// has_status allows one status per task, and owns allows one task per owner
// on the incoming side.
func cardinalityManager(t *testing.T) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`
entities:
  task:
    label: Task
    id_prefix: T
    properties:
      title: {type: string}
  status:
    label: Status
    id_prefix: S
    properties:
      title: {type: string}
relations:
  has_status:
    from: [task]
    to: [status]
    max_outgoing: 1
  exclusive_status:
    from: [task]
    to: [status]
    max_outgoing: 1
    max_incoming: 1
  tagged:
    from: [task]
    to: [status]
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "task"}, {ID: "T-2", Type: "task"},
		{ID: "S-1", Type: "status"}, {ID: "S-2", Type: "status"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	return mgr, st
}

func edgesFrom(t *testing.T, st store.Store, from, relType string) []string {
	t.Helper()
	var out []string
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{From: from, Type: relType}) {
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		out = append(out, r.To)
	}
	return out
}

func key(from, relType, to string) entity.RelationKey {
	return entity.RelationKey{From: from, Type: relType, To: to}
}

func TestCreateRelation_RejectsSecondEdgeOverMaxOutgoing(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-2"), entity.RelationOptions{})
	var ce *entitymanager.CardinalityError
	if !errors.As(err, &ce) || !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("second create err = %v, want a CardinalityError", err)
	}
	if ce.Relation != "has_status" || ce.Constraint != "max_outgoing" || ce.Limit != 1 || ce.Entity != "T-1" {
		t.Errorf("error = %+v", ce)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}

func TestCreateRelation_RejectsOverMaxIncoming(t *testing.T) {
	mgr, _ := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "exclusive_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := mgr.CreateRelation(ctx, key("T-2", "exclusive_status", "S-1"), entity.RelationOptions{})
	var ce *entitymanager.CardinalityError
	if !errors.As(err, &ce) || ce.Constraint != "max_incoming" || ce.Entity != "S-1" {
		t.Fatalf("err = %v, want max_incoming on S-1", err)
	}
}

func TestCreateRelation_UnboundedRelationAllowsMany(t *testing.T) {
	mgr, _ := cardinalityManager(t)
	ctx := context.Background()
	for _, to := range []string{"S-1", "S-2"} {
		if _, err := mgr.CreateRelation(ctx, key("T-1", "tagged", to), entity.RelationOptions{}); err != nil {
			t.Fatalf("create tagged %s: %v", to, err)
		}
	}
}

func TestReplaceOutgoing_RepointsToExactlyOneEdge(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	if _, err := mgr.ReplaceOutgoing(ctx, key("T-1", "has_status", "S-2"), entity.RelationOptions{}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-2" {
		t.Errorf("edges = %v, want [S-2]", got)
	}
}

func TestReplaceOutgoing_CreatesWhenNoEdge(t *testing.T) {
	mgr, st := cardinalityManager(t)
	if _, err := mgr.ReplaceOutgoing(context.Background(), key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}

func TestReplaceOutgoing_KeepsExistingTargetAndDropsOthers(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	// Data over the bound (written around the manager, as a loaded file can
	// be) is repaired by a replace rather than refused.
	for _, to := range []string{"S-1", "S-2"} {
		if _, err := st.CreateRelation(ctx, key("T-1", "has_status", to), nil); err != nil {
			t.Fatalf("seed %s: %v", to, err)
		}
	}
	if _, err := mgr.ReplaceOutgoing(ctx, key("T-1", "has_status", "S-2"), entity.RelationOptions{}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-2" {
		t.Errorf("edges = %v, want [S-2]", got)
	}
}

func TestReplaceOutgoing_FailedCreateKeepsOriginalEdge(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	for _, k := range []entity.RelationKey{
		key("T-1", "exclusive_status", "S-1"), key("T-2", "exclusive_status", "S-2"),
	} {
		if _, err := mgr.CreateRelation(ctx, k, entity.RelationOptions{}); err != nil {
			t.Fatalf("seed %v: %v", k, err)
		}
	}
	// S-2 already holds its one incoming edge, so the create half is refused.
	_, err := mgr.ReplaceOutgoing(ctx, key("T-1", "exclusive_status", "S-2"), entity.RelationOptions{})
	if !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("err = %v, want ErrCardinalityExceeded", err)
	}
	if got := edgesFrom(t, st, "T-1", "exclusive_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want the original [S-1]", got)
	}
}

func TestReplaceOutgoing_MissingTargetChangesNothing(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	_, err := mgr.ReplaceOutgoing(ctx, key("T-1", "has_status", "S-9"), entity.RelationOptions{})
	if !errors.Is(err, entitymanager.ErrEntityNotFound) {
		t.Fatalf("err = %v, want ErrEntityNotFound", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}
