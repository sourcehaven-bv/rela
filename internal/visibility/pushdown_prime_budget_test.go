package visibility_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// edgeLookup is affordances.RelationLookup over a store.
type edgeLookup struct{ st store.Store }

func (l edgeLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{From: fromID, Direction: store.DirectionOutgoing}) {
		if err == nil && rel != nil {
			counts[rel.Type]++
		}
	}
	return counts
}

func (l edgeLookup) HasEdge(ctx context.Context, fromID, relType, toID string) bool {
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Type: relType, To: toID, Direction: store.DirectionOutgoing,
	}) {
		if err == nil && rel != nil {
			return true
		}
	}
	return false
}

// The appbuild wiring (MCP, CLI, scheduler) redacts with PolicyRedactor,
// whose PrimeTraversals primes traversals only. The pushed-down list must
// prime the page's local roles itself, or every row's field verdict probes
// the role relations one edge at a time (TKT-QM833U). The read count of a
// whole-type list is the same at 10 rows and at 50.
func TestScriptReader_PushdownPrimesLocalRolesUnderPolicyRedactor(t *testing.T) {
	reads := map[int]int{}
	var detail string
	for _, n := range []int{10, 50} {
		ctx := context.Background()
		base := memstore.New()
		mk := func(e *entity.Entity) {
			if err := base.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		mk(&entity.Entity{ID: "alice", Type: "person"})
		for i := range n {
			id := fmt.Sprintf("TKT-%03d", i)
			mk(&entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id, "secret": "s"}})
			if _, err := base.CreateRelation(ctx, entity.RelationKey{From: "alice", Type: "assigned-to", To: id}, nil); err != nil {
				t.Fatal(err)
			}
		}
		meta := &metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{
				"ticket": {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}, "secret": {Type: "string"}}},
				"person": {},
			},
			Relations: map[string]metamodel.RelationDef{
				"assigned-to": {From: []string{"person"}, To: []string{"ticket"}},
			},
		}
		st := storetest.NewCounting(base)
		d, err := acl.NewDeclarative(&acl.Policy{
			Roles: map[string]acl.RoleDef{
				"reader": {
					Read:    []string{"*"},
					Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}},
				},
				"assignee": {Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "secret"}}}},
			},
			Assignments:   map[string]string{"alice": "reader"},
			RoleRelations: map[string]acl.RoleRelationDef{"assigned-to": {Confers: "assignee"}},
		}, aclstore.MustNewGraph(st), st)
		if err != nil {
			t.Fatal(err)
		}
		resolver, err := affordances.New(meta, edgeLookup{st}, d)
		if err != nil {
			t.Fatal(err)
		}
		gate, err := visibility.NewDeclarativeGate(d, store.TrivialScope())
		if err != nil {
			t.Fatal(err)
		}
		redact, err := visibility.NewPolicyRedactor(resolver)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := visibility.NewPolicyReader(gate, redact, st)
		if err != nil {
			t.Fatal(err)
		}
		sr, err := visibility.NewScriptReader(reader, st, gate)
		if err != nil {
			t.Fatal(err)
		}
		as := acltest.Bind(principal.With(ctx, principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), t, d)
		st.Reset()
		got := 0
		for e, err := range sr.ListEntities(as, store.EntityQuery{Type: "ticket", Faces: store.AllFaces()}) {
			if err != nil {
				t.Fatal(err)
			}
			got++
			if e.Properties["secret"] == nil {
				t.Fatalf("%s: the assignee's visible field was redacted", e.ID)
			}
		}
		if got != n {
			t.Fatalf("listed %d tickets, want %d", got, n)
		}
		reads[n], detail = st.Reads(), st.String()
	}
	if reads[10] != reads[50] {
		t.Errorf("reads grow with the rows: %d at 10, %d at 50 (%s)", reads[10], reads[50], detail)
	}
}
