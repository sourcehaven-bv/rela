package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// listBudgetMeta declares the two types the list budget fixture stores.
func listBudgetMeta() *metamodel.Metamodel {
	props := map[string]metamodel.PropertyDef{"title": {Type: "string"}, "status": {Type: "string"}}
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"requirement": {DisplayProperty: "title", Properties: props},
		"decision":    {DisplayProperty: "title", Properties: props},
	}}
}

// newListBudgetServer returns a server over a counting store of n
// requirements, every other one accepted, and three decisions.
func newListBudgetServer(t *testing.T, n int) (*Server, *storetest.Counting) {
	t.Helper()
	ctx := context.Background()
	base := memstore.New()
	mk := func(id, typ, status string) {
		e := &entity.Entity{ID: id, Type: typ, Content: "body",
			Properties: map[string]any{"title": id, "status": status}}
		if err := base.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for i := range n {
		status := "accepted"
		if i%2 == 1 {
			status = "draft"
		}
		mk(fmt.Sprintf("REQ-%04d", i), "requirement", status)
	}
	for i := range 3 {
		mk(fmt.Sprintf("DEC-%04d", i), "decision", "accepted")
	}
	st := storetest.NewCounting(base)
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, newTestDeps(t, listBudgetMeta(), st))
	return srv, st
}

func listIDs(t *testing.T, srv *Server, args map[string]any) (ids []string, total int, hasMore bool) {
	t.Helper()
	result, err := srv.handleListEntities(context.Background(), makeToolRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	text := getResultText(t, result)
	if isErrorResult(result) {
		t.Fatalf("list_entities %v: %s", args, text)
	}
	page := decodeEntityPage(t, text)
	for _, e := range page.Entities {
		ids = append(ids, e.ID)
	}
	return ids, page.Total, page.HasMore
}

// list_entities pages in the store: its reads are the same over 100
// requirements as over 1,000, with no filter, with a filter that lowers to a
// property equality, and with no type (TKT-D9UO9Y4).
func TestHandleListEntities_ScaleFree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args map[string]any
		want []string
	}{
		{
			name: "type",
			args: map[string]any{"type": "requirement", "limit": float64(3), "offset": float64(2)},
			want: []string{"REQ-0002", "REQ-0003", "REQ-0004"},
		},
		{
			name: "lowered filter",
			args: map[string]any{
				"type": "requirement", "filter": "entity.status == 'accepted'",
				"limit": float64(3), "offset": float64(1),
			},
			want: []string{"REQ-0002", "REQ-0004", "REQ-0006"},
		},
		{
			name: "every type",
			args: map[string]any{"limit": float64(4), "offset": float64(1)},
			want: []string{"DEC-0001", "DEC-0002", "REQ-0000", "REQ-0001"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				srv, st := newListBudgetServer(t, n)
				st.Reset()
				ids, _, hasMore := listIDs(t, srv, tc.args)
				if !slices.Equal(ids, tc.want) || !hasMore {
					t.Fatalf("ids = %v (has_more %v), want %v and more", ids, hasMore, tc.want)
				}
				return st.Cost(), st.String()
			})
		})
	}
}

// The total counts every matching row, whichever path serves the page.
func TestHandleListEntities_PagedTotals(t *testing.T) {
	t.Parallel()
	srv, _ := newListBudgetServer(t, 10)
	for _, tc := range []struct {
		name      string
		args      map[string]any
		wantTotal int
		wantMore  bool
	}{
		{"type", map[string]any{"type": "requirement", "limit": float64(4)}, 10, true},
		{"last page", map[string]any{"type": "requirement", "limit": float64(4), "offset": float64(8)}, 10, false},
		{"lowered filter", map[string]any{"type": "requirement", "filter": "entity.status == 'draft'"}, 5, false},
		{"every type", map[string]any{"limit": float64(50)}, 13, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, total, more := listIDs(t, srv, tc.args)
			if total != tc.wantTotal || more != tc.wantMore {
				t.Errorf("total = %d, has_more = %v, want %d, %v", total, more, tc.wantTotal, tc.wantMore)
			}
		})
	}
}

// A filter the store answers and the same filter answered in Go serve the
// same page and the same total.
func TestHandleListEntities_PushedFilterMatchesGoFilter(t *testing.T) {
	t.Parallel()
	srv, _ := newListBudgetServer(t, 9)
	d := snapshotFor(context.Background(), srv).deps
	prog, err := compileListFilter(d.Meta, "requirement", "entity.status == 'accepted'", true)
	if err != nil {
		t.Fatal(err)
	}
	page := listPage{faces: store.InWorld(store.TrivialScope()), offset: 1, limit: 3}
	pushed, pushedTotal, err := page.ofType(context.Background(), d, "requirement", prog)
	if err != nil {
		t.Fatal(err)
	}
	inGo, goTotal, err := page.filterInGo(context.Background(), d, "requirement", prog)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(pushed, inGo, func(a, b *entity.Entity) bool { return a.ID == b.ID }) ||
		pushedTotal != goTotal {

		t.Errorf("pushed %d rows of %d, Go %d rows of %d", len(pushed), pushedTotal, len(inGo), goTotal)
	}
}

// The pushed filter is answered by the store: one count and one page, no
// read of the whole type.
func TestHandleListEntities_LoweredFilterIsPushed(t *testing.T) {
	t.Parallel()
	srv, st := newListBudgetServer(t, 9)
	st.Reset()
	listIDs(t, srv, map[string]any{"type": "requirement", "filter": "entity.status == 'accepted'", "limit": float64(2)})
	if calls := st.String(); !strings.Contains(calls, "CountMatched") || strings.Contains(calls, "ListEntities=") {
		t.Errorf("store calls %s: want a pushed count and no whole-type list", calls)
	}
}

// A limit past the cap serves at most the cap, and never sizes a buffer
// from the caller's number.
func TestHandleListEntities_LimitIsCapped(t *testing.T) {
	t.Parallel()
	srv, _ := newListBudgetServer(t, maxPageLimit+5)
	for _, args := range []map[string]any{
		{"type": "requirement", "limit": float64(1e12)},
		{"limit": float64(1e12), "offset": float64(1e12)},
	} {
		ids, total, _ := listIDs(t, srv, args)
		if len(ids) > maxPageLimit || total != maxPageLimit+5+3 && total != maxPageLimit+5 {
			t.Errorf("%v: %d rows of %d", args, len(ids), total)
		}
	}
}

// A filter that does not lower still answers, in Go.
func TestHandleListEntities_UnloweredFilterKeepsGoPath(t *testing.T) {
	t.Parallel()
	srv, _ := newListBudgetServer(t, 6)
	ids, total, _ := listIDs(t, srv, map[string]any{
		"type": "requirement", "filter": "not (entity.status == 'accepted')", "limit": float64(2),
	})
	if !slices.Equal(ids, []string{"REQ-0001", "REQ-0003"}) || total != 3 {
		t.Errorf("ids = %v of %d, want [REQ-0001 REQ-0003] of 3", ids, total)
	}
}
