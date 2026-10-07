package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// owningMetamodel has a task type that owns tasks (the recursive model) and a
// plain relation beside it, so a test can tell owning edges from ordinary ones.
const owningMetamodel = `version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
relations:
  subtask:
    label: subtask
    from: [task]
    to: [task]
    owning: true
  relates-to:
    label: relates to
    from: [task]
    to: [task]
`

func newOwningManager(t *testing.T) (*entitymanager.Manager, store.Store) {
	t.Helper()
	m, err := metamodel.Parse([]byte(owningMetamodel))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        m,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

func mkTask(t *testing.T, mgr *entitymanager.Manager, title string) string {
	t.Helper()
	e := entity.New("", "task")
	e.SetString("title", title)
	res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return res.Entity.ID
}

func link(mgr *entitymanager.Manager, from, relType, to string) error {
	_, err := mgr.CreateRelation(context.Background(),
		entity.RelationKey{From: from, Type: relType, To: to}, entity.RelationOptions{})
	return err
}

// TestCreateRelation_OwningRules pins the write-time ownership rules
// (TKT-QO14GB): one owner, one level, never oneself. Plain relations between
// the same entities are unaffected.
func TestCreateRelation_OwningRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// setup builds the graph and returns the edge to try.
		setup   func(t *testing.T, mgr *entitymanager.Manager) (from, relType, to string)
		wantErr bool
	}{
		{
			name: "first owner is accepted",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				return mkTask(t, mgr, "parent"), "subtask", mkTask(t, mgr, "child")
			},
		},
		{
			name: "an owner may own several children",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				p := mkTask(t, mgr, "parent")
				mustLink(t, mgr, p, "subtask", mkTask(t, mgr, "first"))
				return p, "subtask", mkTask(t, mgr, "second")
			},
		},
		{
			name: "self ownership is refused",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				p := mkTask(t, mgr, "self")
				return p, "subtask", p
			},
			wantErr: true,
		},
		{
			name: "a second owner is refused",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				c := mkTask(t, mgr, "child")
				mustLink(t, mgr, mkTask(t, mgr, "first parent"), "subtask", c)
				return mkTask(t, mgr, "second parent"), "subtask", c
			},
			wantErr: true,
		},
		{
			name: "an owned entity cannot own",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				c := mkTask(t, mgr, "child")
				mustLink(t, mgr, mkTask(t, mgr, "parent"), "subtask", c)
				return c, "subtask", mkTask(t, mgr, "grandchild")
			},
			wantErr: true,
		},
		{
			name: "an owner cannot be owned",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				p := mkTask(t, mgr, "parent")
				mustLink(t, mgr, p, "subtask", mkTask(t, mgr, "child"))
				return mkTask(t, mgr, "grandparent"), "subtask", p
			},
			wantErr: true,
		},
		{
			name: "plain relations ignore ownership",
			setup: func(t *testing.T, mgr *entitymanager.Manager) (string, string, string) {
				t.Helper()
				c := mkTask(t, mgr, "child")
				mustLink(t, mgr, mkTask(t, mgr, "parent"), "subtask", c)
				return c, "relates-to", c
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr, st := newOwningManager(t)
			from, relType, to := tt.setup(t, mgr)
			err := link(mgr, from, relType, to)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, entitymanager.ErrOwningRule) {
				t.Fatalf("err = %v, want ErrOwningRule", err)
			}
			key := entity.RelationKey{From: from, Type: relType, To: to}
			if _, gErr := st.GetRelation(context.Background(), key); !errors.Is(gErr, store.ErrNotFound) {
				t.Errorf("refused edge was stored (GetRelation err = %v)", gErr)
			}
		})
	}
}

func mustLink(t *testing.T, mgr *entitymanager.Manager, from, relType, to string) {
	t.Helper()
	if err := link(mgr, from, relType, to); err != nil {
		t.Fatalf("link %s --%s--> %s: %v", from, relType, to, err)
	}
}

// ownedDeleteMetamodel reuses the concurrency fixture's note type, so the
// cascade tests run on every backend in concBackends.
const ownedDeleteMetamodel = `version: "1.0"
entities:
  note:
    label: Note
    plural: notes
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title:
        type: string
relations:
  part:
    label: part
    from: [note]
    to: [note]
    owning: true
  links:
    label: links
    from: [note]
    to: [note]
`

// denyIDsACL allows every write except a delete of the listed entity ids.
type denyIDsACL struct{ ids map[string]bool }

func (a denyIDsACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if es, ok := req.Subject.(acl.EntitySubject); ok && req.Op == acl.OpDelete && a.ids[es.ID()] {
		return acl.Decision{RuleKind: "role-grant", RuleID: "-", Reason: `no role grants delete on "note"`}
	}
	return acl.Decision{Allow: true}
}

type ownedDeleteFixture struct {
	st                 store.Store
	mgr                *entitymanager.Manager
	aud                *audit.Memory
	owner, c1, c2, far string
}

// newOwnedDeleteFixture seeds owner --part--> c1, c2 and c1 --links--> far.
func newOwnedDeleteFixture(t *testing.T, b concBackend, deny ...string) ownedDeleteFixture {
	t.Helper()
	st := b.open(t)
	meta, err := metamodel.Parse([]byte(ownedDeleteMetamodel))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	// Seed with an allow-all manager; the one under test denies.
	seed, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	mk := func(title string) string {
		e := entity.New("", "note")
		e.SetString("title", title)
		res, cErr := seed.CreateEntity(context.Background(), e, entity.CreateOptions{})
		if cErr != nil {
			t.Fatalf("create note: %v", cErr)
		}
		return res.Entity.ID
	}
	f := ownedDeleteFixture{st: st, aud: audit.NewMemory()}
	f.owner, f.c1, f.c2, f.far = mk("owner"), mk("first"), mk("second"), mk("far")
	mustLink(t, seed, f.owner, "part", f.c1)
	mustLink(t, seed, f.owner, "part", f.c2)
	mustLink(t, seed, f.c1, "links", f.far)

	ids := map[string]bool{}
	for _, d := range deny {
		ids[map[string]string{"owner": f.owner, "c1": f.c1, "c2": f.c2}[d]] = true
	}
	f.mgr, err = entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: f.aud,
		ACL: denyIDsACL{ids: ids}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return f
}

func (f ownedDeleteFixture) exists(t *testing.T, id string) bool {
	t.Helper()
	fam, err := store.Family(context.Background(), f.st, id)
	if err != nil {
		t.Fatalf("read %s: %v", id, err)
	}
	return len(fam) > 0
}

func (f ownedDeleteFixture) hasEdge(t *testing.T, from, relType, to string) bool {
	t.Helper()
	_, err := f.st.GetRelation(context.Background(), entity.RelationKey{From: from, Type: relType, To: to})
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("read edge: %v", err)
	}
	return err == nil
}

// TestDeleteEntity_TakesOwnedEntities pins that a cascade delete of an owner
// removes what it owns, with their relations, and records them as part of
// the owner's delete (TKT-QO14GB).
func TestDeleteEntity_TakesOwnedEntities(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			f := newOwnedDeleteFixture(t, b)
			res, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			for _, id := range []string{f.owner, f.c1, f.c2} {
				if f.exists(t, id) {
					t.Errorf("%s survived its owner's delete", id)
				}
			}
			if !f.exists(t, f.far) {
				t.Errorf("unowned neighbor %s was deleted", f.far)
			}
			if f.hasEdge(t, f.c1, "links", f.far) {
				t.Errorf("owned entity's own edge survived")
			}
			if got := len(res.DeletedEntities); got != 3 {
				t.Errorf("result lists %d entities, want 3", got)
			}

			deleted := map[string]string{}
			for _, r := range f.aud.Records() {
				if r.Op == audit.OpDeleteEntity && r.Subject != nil {
					deleted[r.Subject.ID] = r.TriggeredBy
				}
			}
			want := "cascade:owner-delete:" + f.owner
			for _, id := range []string{f.c1, f.c2} {
				if tb, ok := deleted[id]; !ok || tb != want {
					t.Errorf("audit for %s: triggered_by = %q (recorded %v), want %q", id, tb, ok, want)
				}
			}
			if tb := deleted[f.owner]; tb != "" {
				t.Errorf("owner's own record carries triggered_by %q", tb)
			}
		})
	}
}

// TestDeleteEntity_OwnedWithoutCascade pins that a delete without cascade
// still refuses an owner, as it refuses any entity with relations.
func TestDeleteEntity_OwnedWithoutCascade(t *testing.T) {
	t.Parallel()
	f := newOwnedDeleteFixture(t, concBackends[0])
	if _, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, false); !errors.Is(err, entitymanager.ErrHasRelations) {
		t.Fatalf("err = %v, want ErrHasRelations", err)
	}
	for _, id := range []string{f.owner, f.c1, f.c2} {
		if !f.exists(t, id) {
			t.Errorf("%s was deleted", id)
		}
	}
}

// TestDeleteEntity_OwnedDenialDeletesNothing pins that the owner's delete is
// refused, before any write, when one owned entity may not be deleted, and
// that the refusal does not name that entity.
func TestDeleteEntity_OwnedDenialDeletesNothing(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			f := newOwnedDeleteFixture(t, b, "c2")
			_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			if !errors.Is(err, acl.ErrForbidden) {
				t.Fatalf("err = %v, want a forbidden error", err)
			}
			if strings.Contains(err.Error(), f.c2) {
				t.Errorf("refusal names the owned entity: %v", err)
			}
			for _, id := range []string{f.owner, f.c1, f.c2} {
				if !f.exists(t, id) {
					t.Errorf("%s was deleted despite the refusal", id)
				}
			}
			if !f.hasEdge(t, f.owner, "part", f.c1) {
				t.Errorf("owning edge was deleted despite the refusal")
			}
		})
	}
}

// TestDeleteEntity_OwnedEntityAlone pins that deleting an owned entity
// leaves its owner in place.
func TestDeleteEntity_OwnedEntityAlone(t *testing.T) {
	t.Parallel()
	f := newOwnedDeleteFixture(t, concBackends[0])
	if _, err := f.mgr.DeleteEntity(aliceCtx(), f.c1, true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !f.exists(t, f.owner) || !f.exists(t, f.c2) {
		t.Errorf("deleting an owned entity removed its owner or sibling")
	}
}

// TestSoftDelete_OwnedEntitiesRoundTrip pins that a soft delete hides what
// the owner owns and a restore brings all of it back, owning edges included.
func TestSoftDelete_OwnedEntitiesRoundTrip(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			f := newOwnedDeleteFixture(t, b)
			if !entitymanager.SupportsSoftDelete(f.mgr) {
				t.Skip("backend has no soft delete")
			}
			ctx := aliceCtx()
			if _, err := entitymanager.SoftDeleteEntity(ctx, f.mgr, f.owner); err != nil {
				t.Fatalf("soft delete: %v", err)
			}
			for _, id := range []string{f.owner, f.c1, f.c2} {
				if f.exists(t, id) {
					t.Errorf("%s still visible after its owner's soft delete", id)
				}
			}
			if _, err := entitymanager.RestoreEntity(ctx, f.mgr, f.owner); err != nil {
				t.Fatalf("restore: %v", err)
			}
			for _, id := range []string{f.owner, f.c1, f.c2, f.far} {
				if !f.exists(t, id) {
					t.Errorf("%s not restored", id)
				}
			}
			for _, e := range [][3]string{{f.owner, "part", f.c1}, {f.owner, "part", f.c2}, {f.c1, "links", f.far}} {
				if !f.hasEdge(t, e[0], e[1], e[2]) {
					t.Errorf("edge %v not restored", e)
				}
			}
		})
	}
}

// TestSoftDelete_OwnedDenialMarksNothing mirrors the hard-delete denial.
func TestSoftDelete_OwnedDenialMarksNothing(t *testing.T) {
	t.Parallel()
	f := newOwnedDeleteFixture(t, concBackends[0], "c1")
	_, err := entitymanager.SoftDeleteEntity(aliceCtx(), f.mgr, f.owner)
	if !errors.Is(err, acl.ErrForbidden) {
		t.Fatalf("err = %v, want a forbidden error", err)
	}
	if strings.Contains(err.Error(), f.c1) {
		t.Errorf("refusal names the owned entity: %v", err)
	}
	for _, id := range []string{f.owner, f.c1, f.c2} {
		if !f.exists(t, id) {
			t.Errorf("%s was hidden despite the refusal", id)
		}
	}
}

// TestAutomationCreateRelation_OwningRules pins that an automation's
// create_relation, which writes the store below CreateRelation, is held to the
// same ownership rules.
func TestAutomationCreateRelation_OwningRules(t *testing.T) {
	t.Parallel()
	m, err := metamodel.Parse([]byte(owningMetamodel))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	st := memstore.New()
	engine := automation.NewEngine([]automation.Automation{{
		Name: "adopt",
		On:   automation.Trigger{Entity: []string{"task"}, Created: true},
		Do:   []automation.Action{{CreateRelation: &automation.CreateRelationAction{Relation: "subtask", To: "TASK-CHILD"}}},
	}})
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: m, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		Automations: engine, Cascade: runner,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	// Seed the child and its first owner straight into the store, so the
	// automation does not fire for them.
	for _, id := range []string{"TASK-CHILD", "TASK-OWNER"} {
		e := entity.New(id, "task")
		e.SetString("title", id)
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	first := entity.RelationKey{From: "TASK-OWNER", Type: "subtask", To: "TASK-CHILD"}
	if _, err := st.CreateRelation(ctx, first, &store.RelationData{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}

	res, _ := mgr.CreateEntity(ctx, func() *entity.Entity {
		e := entity.New("", "task")
		e.SetString("title", "second owner")
		return e
	}(), entity.CreateOptions{})
	if res == nil {
		return // the create itself failed, which also leaves no second owner
	}
	second := entity.RelationKey{From: res.Entity.ID, Type: "subtask", To: "TASK-CHILD"}
	if _, err := st.GetRelation(ctx, second); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("automation gave TASK-CHILD a second owner (GetRelation err = %v)", err)
	}
}

// Moving an owned entity to another owner in one ReplaceRelations call does
// not count the edge it leaves as a second owner (TKT-65LVAK). A plain
// create of the same edge is still refused.
func TestReplaceRelations_MovesOwnedEntity(t *testing.T) {
	t.Parallel()
	mgr, st := newOwningManager(t)
	ctx := context.Background()
	a, b, child := mkTask(t, mgr, "a"), mkTask(t, mgr, "b"), mkTask(t, mgr, "child")
	mustLink(t, mgr, a, "subtask", child)
	if err := link(mgr, b, "subtask", child); !errors.Is(err, entitymanager.ErrOwningRule) {
		t.Fatalf("plain second owner: err = %v, want ErrOwningRule", err)
	}
	old := entity.RelationKey{From: a, Type: "subtask", To: child}
	moved := entity.RelationKey{From: b, Type: "subtask", To: child}
	if _, err := mgr.ReplaceRelations(ctx,
		[]entitymanager.RelationCreate{{Key: moved}}, []entity.RelationKey{old}); err != nil {
		t.Fatalf("move owned entity: %v", err)
	}
	if _, err := st.GetRelation(ctx, moved); err != nil {
		t.Errorf("new owning edge missing: %v", err)
	}
	if _, err := st.GetRelation(ctx, old); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("old owning edge: err = %v, want ErrNotFound", err)
	}
}
