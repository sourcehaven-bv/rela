package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// A count is the length of the list it summarizes (TKT-QZTROQ). This pins it
// on the fallback path: a ScriptReader with no ReadQueryProvider counts the
// gated list rather than the raw store.
func TestScriptReader_CountsEqualGatedLists(t *testing.T) {
	st := seedScriptWorld(t)
	sr := newTicketOnlyScriptReader(t, st)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		q    store.EntityQuery
		want int
	}{
		{"readable type", store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())}, 2},
		{"hidden type", store.EntityQuery{Type: "secret", Faces: store.InWorld(store.TrivialScope())}, 0},
		{"all types", store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listed := 0
			for _, err := range sr.ListEntities(ctx, tc.q) {
				if err != nil {
					t.Fatalf("ListEntities: %v", err)
				}
				listed++
			}
			n, err := sr.CountEntities(ctx, tc.q)
			if err != nil {
				t.Fatalf("CountEntities: %v", err)
			}
			if n != listed || n != tc.want {
				t.Errorf("count = %d, list = %d, want %d", n, listed, tc.want)
			}
		})
	}

	// TKT-1 -> SEC-1 has a hidden endpoint; only TKT-1 -> TKT-2 is readable.
	n, err := sr.CountRelations(ctx, store.RelationQuery{Type: "relates"})
	if err != nil {
		t.Fatalf("CountRelations: %v", err)
	}
	if n != 1 {
		t.Errorf("relation count = %d, want 1: an edge to a hidden entity must not be counted", n)
	}
}

func TestUnrestrictedAndDenyReaderCounts(t *testing.T) {
	st := seedScriptWorld(t)
	ctx := context.Background()
	q := store.EntityQuery{Type: "secret", Faces: store.InWorld(store.TrivialScope())}

	if n, err := visibility.Unrestricted(st).CountEntities(ctx, q); err != nil || n != 1 {
		t.Errorf("Unrestricted CountEntities = (%d, %v), want (1, nil)", n, err)
	}
	if n, err := visibility.Unrestricted(st).CountRelations(ctx, store.RelationQuery{Type: "relates"}); err != nil || n != 2 {
		t.Errorf("Unrestricted CountRelations = (%d, %v), want (2, nil)", n, err)
	}
	if _, err := (visibility.DenyReader{}).CountEntities(ctx, q); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("DenyReader CountEntities err = %v, want ErrReaderUnavailable", err)
	}
	if _, err := (visibility.DenyReader{}).CountRelations(ctx, store.RelationQuery{}); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("DenyReader CountRelations err = %v, want ErrReaderUnavailable", err)
	}
}

// The production wiring: a ScriptReader whose binder is a DeclarativeGate
// counts in the store over the pushed-down scope, and never lists the rows.
func TestScriptReader_CountEntitiesPushesDown(t *testing.T) {
	ctx := context.Background()
	base := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy"},
		{ID: "POL-2", Type: "policy", Face: "published"},
		{ID: "POL-2", Type: "policy", Face: "draft"},
		{ID: "POL-3", Type: "policy", Face: "published"},
		{ID: "bob", Type: "user"},
		{ID: "carol", Type: "user"},
		{ID: "dave", Type: "user"},
	} {
		if err := base.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"readers":          {Read: []string{"policy"}},
			"publishedreaders": {Read: []string{"policy@published"}},
		},
		Assignments: map[string]string{"bob": "readers", "carol": "publishedreaders"},
	}, acl.NewStoreGraph(base), base)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := visibility.NewDeclarativeGate(d, store.TrivialScope())
	if err != nil {
		t.Fatal(err)
	}
	st := storetest.NewCounting(base)
	reader, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(reader, st, gate)
	if err != nil {
		t.Fatal(err)
	}

	q := store.EntityQuery{Type: "policy", Faces: store.AllFaces()}
	for user, want := range map[string]int{"bob": 4, "carol": 2, "dave": 0} {
		t.Run(user, func(t *testing.T) {
			as := principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolDataEntry})
			st.Reset()
			n, err := sr.CountEntities(as, q)
			if err != nil {
				t.Fatalf("CountEntities: %v", err)
			}
			if n != want {
				t.Errorf("count = %d, want %d", n, want)
			}
			if calls := st.Calls(); calls["ListEntities"] != 0 || calls["GraphQuery"] != 0 {
				t.Errorf("count listed rows: %s", st)
			}
		})
	}
}

// TestScriptReader_CountRelationsBudget: a relation count reads the edges in
// one query and gates their endpoints in one batch, at 10 edges and at 50.
// Relations have no store pushdown, so the count is a scan; this pins that
// the scan costs a fixed number of store calls.
func TestScriptReader_CountRelationsBudget(t *testing.T) {
	reads := map[int]int{}
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			for i := range n + 1 {
				id := fmt.Sprintf("TKT-%03d", i)
				if err := base.CreateEntity(ctx, &entity.Entity{ID: id, Type: "ticket"}); err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					continue
				}
				key := entity.RelationKey{From: fmt.Sprintf("TKT-%03d", i-1), Type: "relates", To: id}
				if _, err := base.CreateRelation(ctx, key, nil); err != nil {
					t.Fatal(err)
				}
			}
			st := storetest.NewCounting(base)
			sr := newTicketOnlyScriptReader(t, st)
			got, err := sr.CountRelations(ctx, store.RelationQuery{Type: "relates"})
			if err != nil {
				t.Fatalf("CountRelations: %v", err)
			}
			if got != n {
				t.Errorf("count = %d, want %d", got, n)
			}
			reads[n] = st.Reads()
		})
	}
	if reads[10] != reads[50] {
		t.Errorf("store reads = %d at 10 edges, %d at 50; want equal", reads[10], reads[50])
	}
}
