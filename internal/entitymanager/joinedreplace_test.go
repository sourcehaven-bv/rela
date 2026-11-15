package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// joinedPolicy: linker may link plans to risks, nolink may update risks but
// not link plans.
const joinedPolicy = `
role_relations:
  heeft_behandelplan:
    requires_permission: link-plan
roles:
  linker:
    read: ["*"]
    create: [risico, behandelplan]
    update: [risico, behandelplan]
    delete: [risico, behandelplan]
    permissions: [link-plan]
  nolink:
    read: ["*"]
    create: [risico, behandelplan]
    update: [risico, behandelplan]
    delete: [risico, behandelplan]
assignments:
  linker: linker
  nolink: nolink
`

const closedRuleMessage = `validation rule "gesloten-zonder-plan" is not satisfied: ` +
	`Een gesloten risico heeft geen behandelplan.`

func newJoinedFixture(
	t *testing.T, st store.Store, status string, edges []entity.RelationKey,
) *entitymanager.Manager {
	t.Helper()
	seedHidden(t, st, []*entity.Entity{
		riskWith(status), plan("BP-1", "concept"), plan("BP-3", "vastgesteld"),
	}, edges)
	p, err := acl.LoadPolicyBytes([]byte(joinedPolicy))
	if err != nil {
		t.Fatal(err)
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	return newRelationRuleManager(t, st, d)
}

// TestPatchWithRelations pins that an update and the edges it counts on are
// written together or not at all: a refusal of either part, by a rule, a far
// end's rule or the ACL, leaves the entity and its edges as they were.
func TestPatchWithRelations(t *testing.T) {
	bp3 := entity.RelationKey{From: "RIS-1", Type: "heeft_behandelplan", To: "BP-3"}
	tests := []struct {
		name    string
		user    string
		status  string // stored before the patch
		edges   []entity.RelationKey
		patch   entity.Patch
		creates []entity.RelationKey
		removes []entity.RelationKey
		// wantErr checks the error; nil means the write is accepted.
		wantErr    func(t *testing.T, err error)
		wantStatus string
		wantEdges  map[entity.RelationKey]bool
	}{
		{
			name: "the status and the plan it needs", user: "linker", status: "nieuw",
			patch:      entity.Patch{Properties: map[string]any{"status": "in_behandeling"}},
			creates:    []entity.RelationKey{planEdge},
			wantStatus: "in_behandeling", wantEdges: map[entity.RelationKey]bool{planEdge: true},
		},
		{
			name: "closing and unlinking the plan together", user: "linker", status: "in_behandeling",
			edges:      []entity.RelationKey{planEdge},
			patch:      entity.Patch{Properties: map[string]any{"status": "gesloten"}},
			removes:    []entity.RelationKey{planEdge},
			wantStatus: "gesloten", wantEdges: map[entity.RelationKey]bool{planEdge: false},
		},
		{
			name: "a change beside unlinking the last plan", user: "linker", status: "in_behandeling",
			edges:   []entity.RelationKey{planEdge},
			patch:   entity.Patch{Content: new("Notitie.")},
			removes: []entity.RelationKey{planEdge},
			wantErr: func(t *testing.T, err error) {
				t.Helper()
				assertRuleOnly(t, err, ruleMessage)
			},
			wantStatus: "in_behandeling", wantEdges: map[entity.RelationKey]bool{planEdge: true},
		},
		{
			name: "a far end's rule refuses the unlink", user: "linker", status: "nieuw",
			edges:   []entity.RelationKey{bp3},
			patch:   entity.Patch{Properties: map[string]any{"status": "beoordeeld"}},
			removes: []entity.RelationKey{bp3},
			wantErr: func(t *testing.T, err error) {
				t.Helper()
				if _, ok := errors.AsType[*entitymanager.JoinedRelationError](err); !ok {
					t.Errorf("err = %v, want a JoinedRelationError", err)
				}
				var invalid *entitymanager.ValidationError
				if !errors.As(err, &invalid) {
					t.Fatalf("err = %v, want a validation refusal", err)
				}
			},
			wantStatus: "nieuw", wantEdges: map[entity.RelationKey]bool{bp3: true},
		},
		{
			name: "the ACL denies the plan the status counts on", user: "nolink", status: "nieuw",
			patch:   entity.Patch{Properties: map[string]any{"status": "in_behandeling"}},
			creates: []entity.RelationKey{planEdge},
			wantErr: func(t *testing.T, err error) {
				t.Helper()
				if _, ok := errors.AsType[*entitymanager.JoinedRelationError](err); !ok {
					t.Errorf("err = %v, want a JoinedRelationError", err)
				}
				if _, ok := errors.AsType[*acl.ForbiddenError](err); !ok {
					t.Errorf("err = %v, want an ACL denial", err)
				}
			},
			wantStatus: "nieuw", wantEdges: map[entity.RelationKey]bool{planEdge: false},
		},
	}
	for backend, open := range relationRuleBackends() {
		for _, tc := range tests {
			t.Run(backend+"/"+tc.name, func(t *testing.T) {
				st := open(t)
				m := newJoinedFixture(t, st, tc.status, tc.edges)
				ctx := entitymanager.BindAs(asUser(tc.user), t, m)
				row, err := st.GetEntity(ctx, entity.Ref{ID: "RIS-1", Face: entity.ImplicitFace})
				if err != nil {
					t.Fatal(err)
				}
				grant, err := m.AuthorizeUpdate(ctx, row)
				if err != nil {
					t.Fatal(err)
				}
				creates := make([]entitymanager.RelationCreate, len(tc.creates))
				for i, k := range tc.creates {
					creates[i] = entitymanager.RelationCreate{Key: k}
				}
				_, _, err = entitymanager.PatchWithRelations(ctx, grant, "RIS-1", tc.patch, creates, tc.removes)
				switch {
				case tc.wantErr == nil && err != nil:
					t.Fatalf("err = %v, want the write accepted", err)
				case tc.wantErr != nil && err == nil:
					t.Fatal("write accepted, want it refused")
				case tc.wantErr != nil:
					tc.wantErr(t, err)
				}
				bg := context.Background()
				if got := statusOf(bg, t, st, "RIS-1"); got != tc.wantStatus {
					t.Errorf("status = %v, want %q", got, tc.wantStatus)
				}
				for k, want := range tc.wantEdges {
					if got := hasEdge(bg, t, st, k); got != want {
						t.Errorf("edge %s stored = %v, want %v", k.To, got, want)
					}
				}
			})
		}
	}
}

// TestRelationRules_RelationWriteRefusalNamesOnlyTheRule pins, under a real
// policy, that a relation write a rule refuses answers with the rule's
// message and names no entity, on every relation write path.
func TestRelationRules_RelationWriteRefusalNamesOnlyTheRule(t *testing.T) {
	tests := []struct {
		name   string
		status string
		edges  []entity.RelationKey
		write  func(ctx context.Context, m *entitymanager.Manager) error
		want   string
	}{
		{
			name: "create", status: "gesloten",
			write: func(ctx context.Context, m *entitymanager.Manager) error {
				_, err := m.CreateRelation(ctx, planEdge, entity.RelationOptions{})
				return err
			},
			want: closedRuleMessage,
		},
		{
			name: "delete", status: "in_behandeling", edges: []entity.RelationKey{planEdge},
			write: func(ctx context.Context, m *entitymanager.Manager) error {
				return m.DeleteRelation(ctx, planEdge)
			},
			want: ruleMessage,
		},
		{
			name: "replace", status: "in_behandeling", edges: []entity.RelationKey{planEdge},
			write: func(ctx context.Context, m *entitymanager.Manager) error {
				_, err := m.ReplaceRelations(ctx, nil, []entity.RelationKey{planEdge})
				return err
			},
			want: ruleMessage,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := memstore.New()
			m := newJoinedFixture(t, st, tc.status, tc.edges)
			ctx := entitymanager.BindAs(asUser("linker"), t, m)
			assertRuleOnly(t, tc.write(ctx, m), tc.want)
		})
	}
}

// assertRuleOnly checks that err is a validation refusal carrying exactly
// msg, and that it names neither end of the edge.
func assertRuleOnly(t *testing.T, err error, msg string) {
	t.Helper()
	var invalid *entitymanager.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want a validation refusal", err)
	}
	if len(invalid.Errors) != 1 || invalid.Errors[0].Message != msg {
		t.Fatalf("refusal = %+v, want only %q", invalid.Errors, msg)
	}
	assertNoTrace(t, err, "RIS-1", "BP-1", "BP-3")
}
