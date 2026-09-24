package cli

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func relatedFilterMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  ticket:
    label: Ticket
    id_prefix: T
    properties:
      title: { type: string }
  person:
    label: Person
    id_prefix: P
    properties:
      name: { type: string }
relations:
  owned-by:
    from: [ticket]
    to: [person]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return m
}

// seedOwned stores n tickets; every even one is owned by alice.
func seedOwned(t *testing.T, n int) (*storetest.Counting, []*entity.Entity) {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	alice := entity.New("P-alice", "person")
	alice.Properties["name"] = "alice"
	if err := st.CreateEntity(ctx, alice); err != nil {
		t.Fatal(err)
	}
	var tickets []*entity.Entity
	for i := range n {
		e := entity.New(fmt.Sprintf("T-%03d", i), "ticket")
		e.Properties["title"] = "t"
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 {
			if _, err := st.CreateRelation(ctx, e.ID, "owned-by", alice.ID, nil); err != nil {
				t.Fatal(err)
			}
		}
		tickets = append(tickets, e)
	}
	return storetest.NewCounting(st), tickets
}

func TestListFilter_Related(t *testing.T) {
	meta := relatedFilterMeta(t)
	st, all := seedOwned(t, 4)
	for _, tc := range []struct {
		filter string
		want   []string
	}{
		{"related(entity, 'owned-by', {name='alice'})", []string{"T-000", "T-002"}},
		{"not related(entity, 'owned-by')", []string{"T-001", "T-003"}},
		{"related(entity, 'owned-by', {name='bob'})", nil},
	} {
		t.Run(tc.filter, func(t *testing.T) {
			got, err := applyListFilters(context.Background(), all, nil, tc.filter, "ticket", meta, st.MatchingIDs)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, e := range got {
				ids = append(ids, e.ID)
			}
			if !slices.Equal(ids, tc.want) {
				t.Fatalf("got %v, want %v", ids, tc.want)
			}
		})
	}
}

func TestListFilter_RelatedInvalidPath(t *testing.T) {
	meta := relatedFilterMeta(t)
	st, all := seedOwned(t, 1)
	for _, f := range []string{"related(entity, 'nope')", "related(entity, 'owned-by', {nope='x'})"} {
		if _, err := applyListFilters(context.Background(), all, nil, f, "ticket", meta, st.MatchingIDs); err == nil {
			t.Errorf("%s: want an error", f)
		}
	}
}

// One MatchingIDs per distinct traversal, whatever the number of rows; a
// filter without related() makes none.
func TestListFilter_RelatedBudget(t *testing.T) {
	meta := relatedFilterMeta(t)
	calls := func(n int, filter string) int {
		st, all := seedOwned(t, n)
		if _, err := applyListFilters(context.Background(), all, nil, filter, "ticket", meta, st.MatchingIDs); err != nil {
			t.Fatal(err)
		}
		return st.Calls()["MatchingIDs"]
	}
	const f = "related(entity, 'owned-by') and not related(entity, 'owned-by', {name='bob'})"
	if a, b := calls(10, f), calls(50, f); a != 2 || b != 2 {
		t.Fatalf("MatchingIDs calls at 10/50 rows = %d/%d, want 2/2", a, b)
	}
	if got := calls(10, "entity.title == 't'"); got != 0 {
		t.Fatalf("MatchingIDs calls without related() = %d, want 0", got)
	}
}
