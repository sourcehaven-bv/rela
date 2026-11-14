package entitymanager_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// relationRuleYAML holds the GRC example of TKT-EBARYBI: a risk in
// treatment needs a treatment plan.
const relationRuleYAML = `
version: "1.0"
entities:
  risico:
    label: Risico
    id_prefix: "RIS-"
    id_type: sequential
    properties:
      status: {type: string}
  behandelplan:
    label: Behandelplan
    id_prefix: "BP-"
    id_type: sequential
    properties:
      titel: {type: string}
relations:
  heeft_behandelplan:
    from: [risico]
    to: [behandelplan]
    inverse: behandelt
validations:
  - name: behandelplan-nodig
    description: Een risico in behandeling heeft een behandelplan.
    entity_type: risico
    enforce: write
    when: ["status=in_behandeling"]
    then_condition: "related(entity, 'heeft_behandelplan')"
  - name: plan-behandelt-iets
    description: Een vastgesteld plan behandelt een risico.
    entity_type: behandelplan
    enforce: write
    when: ["titel=vastgesteld"]
    then_condition: "related(entity, 'behandelt')"
  - name: gesloten-zonder-plan
    description: Een gesloten risico heeft geen behandelplan.
    entity_type: risico
    enforce: write
    when: ["status=gesloten"]
    then_condition: "not related(entity, 'heeft_behandelplan')"
`

const ruleMessage = `validation rule "behandelplan-nodig" is not satisfied: ` +
	`Een risico in behandeling heeft een behandelplan.`

func relationRuleBackends() map[string]func(t *testing.T) store.Store {
	return map[string]func(t *testing.T) store.Store{
		"mem": func(*testing.T) store.Store { return memstore.New() },
		"fs": func(t *testing.T) store.Store {
			t.Helper()
			fs := storage.NewMemFS()
			rooted, err := storage.NewRootedFS(fs, "/")
			if err != nil {
				t.Fatal(err)
			}
			st, err := fsstore.New(fsstore.Config{
				FS: fs, Rooted: rooted,
				EntitiesKey: "entities", RelationsKey: "relations",
				AttachmentsKey: "attachments", CacheKey: ".rela",
				Schemas: map[string]store.EntityTypeSchema{
					"risico":       {Plural: "risicos"},
					"behandelplan": {Plural: "behandelplannen"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.Close() })
			return st
		},
	}
}

func newRelationRuleManager(t *testing.T, st store.Store, a acl.ACL) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(relationRuleYAML))
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: a, Transitions: statemachine.EmptySet(),
		FieldGate:      entitymanager.AllowAllFieldGate{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
		AnchorGuard:    entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

var (
	planEdge  = entity.RelationKey{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-1"}
	planEdge2 = entity.RelationKey{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-2"}
)

func riskWith(status string) *entity.Entity {
	return &entity.Entity{ID: "RIS-1", Type: "risico", Properties: map[string]any{"status": status}}
}

func plan(id, titel string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "behandelplan", Properties: map[string]any{"titel": titel}}
}

func statusOf(ctx context.Context, t *testing.T, st store.Store, id string) any {
	t.Helper()
	e, err := st.GetEntity(ctx, entity.Ref{ID: id, Face: entity.ImplicitFace})
	if err != nil {
		t.Fatal(err)
	}
	return e.Properties["status"]
}

func hasEdge(ctx context.Context, t *testing.T, st store.Store, k entity.RelationKey) bool {
	t.Helper()
	_, err := st.GetRelation(ctx, k)
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

// TestRelationRules pins which writes a relation rule refuses, on each
// backend that does not roll a Tx back: a refused write must leave the store
// as it was.
func TestRelationRules(t *testing.T) {
	type write func(ctx context.Context, m *entitymanager.Manager, st store.Store) error
	setStatus := func(s string) write {
		return func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
			_, err := m.UpdateEntity(ctx, riskWith(s))
			return err
		}
	}
	type relationRuleCase struct {
		name    string
		status  string
		edges   []entity.RelationKey
		write   write
		refused bool
		// check runs after the write, whether it was refused or not.
		check func(ctx context.Context, t *testing.T, st store.Store)
	}
	tests := []relationRuleCase{
		{
			name: "update into the rule without a plan is refused", status: "nieuw",
			write: setStatus("in_behandeling"), refused: true,
			check: func(ctx context.Context, t *testing.T, st store.Store) {
				t.Helper()
				if got := statusOf(ctx, t, st, "RIS-1"); got != "nieuw" {
					t.Errorf("status = %v, want the stored row unchanged", got)
				}
			},
		},
		{
			name: "update into the rule with a plan is accepted", status: "nieuw",
			edges: []entity.RelationKey{planEdge}, write: setStatus("in_behandeling"),
		},
		{
			name: "patch into the rule without a plan is refused", status: "nieuw",
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				_, err := m.PatchEntity(ctx, "RIS-1",
					entity.Patch{Properties: map[string]any{"status": "in_behandeling"}})
				return err
			},
			refused: true,
		},
		{
			name: "a row that already breaks the rule stays editable", status: "in_behandeling",
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				e := riskWith("in_behandeling")
				e.Content = "Notitie."
				_, err := m.UpdateEntity(ctx, e)
				return err
			},
		},
		{
			name: "deleting the last plan is refused", status: "in_behandeling",
			edges: []entity.RelationKey{planEdge},
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				return m.DeleteRelation(ctx, planEdge)
			},
			refused: true,
			check: func(ctx context.Context, t *testing.T, st store.Store) {
				t.Helper()
				if !hasEdge(ctx, t, st, planEdge) {
					t.Error("the refused delete removed the edge")
				}
			},
		},
		{
			name: "deleting one of two plans is accepted", status: "in_behandeling",
			edges: []entity.RelationKey{planEdge, planEdge2},
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				return m.DeleteRelation(ctx, planEdge)
			},
			check: func(ctx context.Context, t *testing.T, st store.Store) {
				t.Helper()
				if hasEdge(ctx, t, st, planEdge) {
					t.Error("the accepted delete left the edge")
				}
			},
		},
		{
			name: "an If-Match delete of the last plan is refused", status: "in_behandeling",
			edges: []entity.RelationKey{planEdge},
			write: func(ctx context.Context, m *entitymanager.Manager, st store.Store) error {
				r, err := st.GetRelation(ctx, planEdge)
				if err != nil {
					return err
				}
				return entitymanager.IfMatchDeletes{M: m}.DeleteRelation(ctx, planEdge, store.RelationVersionOf(r))
			},
			refused: true,
			check: func(ctx context.Context, t *testing.T, st store.Store) {
				t.Helper()
				if !hasEdge(ctx, t, st, planEdge) {
					t.Error("the refused delete removed the edge")
				}
			},
		},
		{
			name: "replacing the last plan with another is accepted", status: "in_behandeling",
			edges: []entity.RelationKey{planEdge},
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				_, err := m.ReplaceRelations(ctx,
					[]entitymanager.RelationCreate{{Key: planEdge2}}, []entity.RelationKey{planEdge})
				return err
			},
		},
		{
			name: "removing the last plan in a replace is refused", status: "in_behandeling",
			edges: []entity.RelationKey{planEdge},
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				_, err := m.ReplaceRelations(ctx, nil, []entity.RelationKey{planEdge})
				return err
			},
			refused: true,
			check: func(ctx context.Context, t *testing.T, st store.Store) {
				t.Helper()
				if !hasEdge(ctx, t, st, planEdge) {
					t.Error("the refused replace removed the edge")
				}
			},
		},
		{
			name: "the rule of the far end is checked", status: "nieuw",
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				// BP-3 is vastgesteld and behandelt nothing: it already breaks
				// its rule, so creating an edge to it is accepted, and deleting
				// that edge again is refused.
				k := entity.RelationKey{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-3"}
				if _, err := m.CreateRelation(ctx, k, entity.RelationOptions{}); err != nil {
					return err
				}
				return m.DeleteRelation(ctx, k)
			},
			refused: true,
		},
		{
			name: "both ends refusing are reported together", status: "in_behandeling",
			edges: []entity.RelationKey{{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-3"}},
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				err := m.DeleteRelation(ctx, entity.RelationKey{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-3"})
				var invalid *entitymanager.ValidationError
				if errors.As(err, &invalid) && len(invalid.Errors) != 2 {
					return fmt.Errorf("want one error per rule, got %v", invalid.Errors)
				}
				return err
			},
			refused: true,
		},
		{
			name: "a negated rule refuses a relation create at the near end", status: "gesloten",
			write: func(ctx context.Context, m *entitymanager.Manager, _ store.Store) error {
				_, err := m.CreateRelation(ctx, planEdge, entity.RelationOptions{})
				return err
			},
			refused: true,
			check: func(ctx context.Context, t *testing.T, st store.Store) {
				t.Helper()
				if hasEdge(ctx, t, st, planEdge) {
					t.Error("the refused create stored the edge")
				}
			},
		},
		{
			name: "an edge to an entity that is gone does not count", status: "nieuw",
			edges: []entity.RelationKey{{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-GONE"}},
			write: setStatus("in_behandeling"), refused: true,
		},
	}
	for backend, open := range relationRuleBackends() {
		for _, tc := range tests {
			t.Run(backend+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				st := open(t)
				seedHidden(t, st, []*entity.Entity{
					riskWith(tc.status), plan("BP-1", "concept"), plan("BP-2", "concept"),
					plan("BP-3", "vastgesteld"),
				}, tc.edges)
				m := newRelationRuleManager(t, st, acl.NopACL{})
				err := tc.write(ctx, m, st)
				var invalid *entitymanager.ValidationError
				switch {
				case tc.refused && !errors.As(err, &invalid):
					t.Fatalf("err = %v, want a validation refusal", err)
				case !tc.refused && err != nil:
					t.Fatalf("err = %v, want the write accepted", err)
				}
				if tc.check != nil {
					tc.check(ctx, t, st)
				}
			})
		}
	}
}

// TestRelationRules_CreateThenLink pins that a create is not checked: the
// request that creates a risk in treatment links its plan next.
func TestRelationRules_CreateThenLink(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	seedHidden(t, st, []*entity.Entity{plan("BP-1", "concept")}, nil)
	m := newRelationRuleManager(t, st, acl.NopACL{})
	res, err := m.CreateEntity(ctx, &entity.Entity{Type: "risico",
		Properties: map[string]any{"status": "in_behandeling"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	k := entity.RelationKey{From: res.Entity.ID, Type: "heeft_behandelplan", To: "BP-1"}
	if _, err := m.CreateRelation(ctx, k, entity.RelationOptions{}); err != nil {
		t.Fatalf("link: %v", err)
	}
	// Now the rule holds, so unlinking breaks it.
	var invalid *entitymanager.ValidationError
	if err := m.DeleteRelation(ctx, k); !errors.As(err, &invalid) {
		t.Fatalf("unlink err = %v, want a validation refusal", err)
	}
}

// relationRulePolicy: blind reads and writes risks but cannot read plans.
const relationRulePolicy = `
roles:
  blind:
    read: [risico]
    create: [risico]
    update: [risico]
    delete: [risico]
assignments:
  blind: blind
`

// TestRelationRules_HiddenPlanCounts pins the ACL design of TKT-EBARYBI. An
// edge to a plan the writer cannot read counts, so the verdict does not
// depend on who writes. The refusal is the rule's message and nothing else:
// it names no entity, visible or hidden.
func TestRelationRules_HiddenPlanCounts(t *testing.T) {
	st := memstore.New()
	seedHidden(t, st, []*entity.Entity{
		riskWith("nieuw"),
		{ID: "RIS-2", Type: "risico", Properties: map[string]any{"status": "nieuw"}},
		plan("BP-SECRET", "geheim plan"),
	}, []entity.RelationKey{{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-SECRET"}})
	p, err := acl.LoadPolicyBytes([]byte(relationRulePolicy))
	if err != nil {
		t.Fatal(err)
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	m := newRelationRuleManager(t, st, d)
	ctx := entitymanager.BindAs(asUser("blind"), t, m)

	if _, err = m.UpdateEntity(ctx, riskWith("in_behandeling")); err != nil {
		t.Fatalf("RIS-1 has a hidden plan; update err = %v, want accepted", err)
	}
	_, err = m.UpdateEntity(ctx, &entity.Entity{ID: "RIS-2", Type: "risico",
		Properties: map[string]any{"status": "in_behandeling"}})
	var invalid *entitymanager.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("RIS-2 err = %v, want a validation refusal", err)
	}
	if len(invalid.Errors) != 1 || invalid.Errors[0].Message != ruleMessage {
		t.Fatalf("refusal = %+v, want only %q", invalid.Errors, ruleMessage)
	}
	assertNoTrace(t, err, "BP-SECRET", "geheim", "behandelplan\"", "RIS-1")
}
