package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Query budgets (TKT-1U8XYN). Each test drives one SPA-shaped request over a
// counting store at two sizes and asserts the store-call count does not
// grow with the number of rows — the property an N+1 violates — and then
// pins the measured constant so a regression names itself. The pre-change
// counts in the comments come from the same fixtures before batching.

// budgetMeta: tickets implement features, block other tickets and are
// assigned to people; people belong to teams, which is what the ACL walks.
func budgetMeta() *metamodel.Metamodel {
	str := map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true}, "status": {Type: "string"},
			}, PropertyOrder: []string{"title", "status"}},
			"feature": {Label: "Feature", Properties: str, PropertyOrder: []string{"title"}},
			"person":  {Label: "Person", Properties: str, PropertyOrder: []string{"title"}},
			"team":    {Label: "Team", Properties: str, PropertyOrder: []string{"title"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements":  {Label: "implements", From: []string{"ticket"}, To: []string{"feature"}},
			"blocks":      {Label: "blocks", From: []string{"ticket"}, To: []string{"ticket"}},
			"assigned-to": {Label: "assigned to", From: []string{"ticket"}, To: []string{"person"}},
			"member-of":   {Label: "member of", From: []string{"person"}, To: []string{"team"}},
		},
	}
}

func budgetConfig() *Config {
	return &Config{
		App: dataentryconfig.AppConfig{Name: "budget"},
		Lists: map[string]dataentryconfig.List{
			"tickets": {EntityType: "ticket", Columns: []dataentryconfig.ListColumn{
				{Property: "title"}, {Property: "status"},
				{Relation: "implements", Label: "Feature"},
				{Relation: "assigned-to", Label: "Assignee"},
			}},
		},
		Views: map[string]dataentryconfig.ViewConfig{
			"ticket": {
				Title: "Ticket",
				Entry: dataentryconfig.ViewEntry{Type: "ticket"},
				Traverse: []dataentryconfig.ViewTraverse{
					{From: "entry", FollowIncoming: "blocks", CollectAs: "blockers"},
				},
				Sections: []dataentryconfig.ViewSection{
					{Heading: "Blocked by", Source: "blockers", Display: "table", Columns: []dataentryconfig.ListColumn{
						{Property: "title"},
						{Relation: "implements", Label: "Feature"},
						{Relation: "assigned-to", Label: "Assignee"},
					}},
				},
			},
		},
		Forms:      map[string]dataentryconfig.Form{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
}

// newBudgetApp seeds n tickets (each implementing a feature, assigned to a
// person, and blocking TKT-0001) into a counting memstore BEFORE the app is
// assembled, so the search index backfills from it, then wires an ACL whose
// role is reached through a member-of walk (person P1 → team T1).
func newBudgetApp(t *testing.T, n int) (*App, *storetest.Counting, *acl.Declarative, context.Context) {
	t.Helper()
	counting := storetest.NewCounting(memstore.New())
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mk := func(id, typ, title string) {
		e := entity.New(id, typ)
		e.SetString("title", title)
		must(counting.CreateEntity(ctx, e))
	}
	mk("T1", "team", "Team one")
	for i := 1; i <= 3; i++ {
		mk(fmt.Sprintf("P%d", i), "person", fmt.Sprintf("Person %d", i))
		_, err := counting.CreateRelation(ctx, fmt.Sprintf("P%d", i), "member-of", "T1", nil)
		must(err)
	}
	for i := 1; i <= 5; i++ {
		mk(fmt.Sprintf("F%d", i), "feature", fmt.Sprintf("Feature %d", i))
	}
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("TKT-%04d", i)
		e := entity.New(id, "ticket")
		e.SetString("title", "Ticket "+id)
		e.SetString("status", []string{"open", "done"}[i%2])
		must(counting.CreateEntity(ctx, e))
		_, err := counting.CreateRelation(ctx, id, "implements", fmt.Sprintf("F%d", i%5+1), nil)
		must(err)
		_, err = counting.CreateRelation(ctx, id, "assigned-to", fmt.Sprintf("P%d", i%3+1), nil)
		must(err)
		if i > 1 {
			_, err = counting.CreateRelation(ctx, id, "blocks", "TKT-0001", nil)
			must(err)
		}
	}

	app := newAppFromParts(budgetConfig(), budgetMeta(), newFixture(), appbuildtest.WithStore(counting))
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {
			Read: []string{"*"}, Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"},
		}},
		Assignments: map[string]string{"T1": "editor"},
	}, app.store)
	app.acl = d
	counting.Reset()
	return app, counting, d, principalCtx("P1")
}

// readsFor runs op at both sizes and returns the read counts.
func readsFor(t *testing.T, op func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context)) (small, large int, detail string) {
	t.Helper()
	counts := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		app, counting, d, ctx := newBudgetApp(t, n)
		op(t, app, d, ctx)
		counts = append(counts, counting.Reads())
		detail = counting.String()
	}
	return counts[0], counts[1], detail
}

func assertBudget(t *testing.T, name string, small, large, pinned int, detail string) {
	t.Helper()
	if small != large {
		t.Errorf("%s: store reads grow with page size: %d at 10 rows, %d at 50 rows (%s)", name, small, large, detail)
	}
	if large != pinned {
		t.Errorf("%s: store reads = %d, pinned budget %d (%s) — update the pin if the change is deliberate",
			name, large, pinned, detail)
	}
}

// A list page with two relation columns. Before batching: 7 + 8×rows reads
// (per row: outgoing + incoming edges, a GetEntity per neighbor, and two
// membership walks per affordance verb).
func TestQueryBudget_ListPageIsSizeIndependent(t *testing.T) {
	small, large, detail := readsFor(t, func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
		t.Helper()
		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100")
		if rec.Code != http.StatusOK {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		if len(resp.Data) < 10 {
			t.Fatalf("list returned %d rows", len(resp.Data))
		}
	})
	assertBudget(t, "list page", small, large, listPageBudget, detail)
}

// A view whose table section has two relation columns over the blockers of
// TKT-0001 (n-1 rows). Before batching: ~4 + 4×rows reads.
func TestQueryBudget_ViewTableSectionIsSizeIndependent(t *testing.T) {
	small, large, detail := readsFor(t, func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
		t.Helper()
		rec := viewsAs(ctx, t, app, d, "ticket", "TKT-0001")
		if rec.Code != http.StatusOK {
			t.Fatalf("view: %d %s", rec.Code, rec.Body)
		}
	})
	assertBudget(t, "view table section", small, large, viewSectionBudget, detail)
}

// A structured search (the dashboard card shape). Before batching: 2 +
// 6×hits reads from the per-hit affordance membership walks.
func TestQueryBudget_SearchIsSizeIndependent(t *testing.T) {
	small, large, detail := readsFor(t, func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=type%3Aticket", http.NoBody)
		req = req.WithContext(gateCtxFor(ctx, t, d))
		rec := httptest.NewRecorder()
		app.handleV1Search(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("search: %d %s", rec.Code, rec.Body)
		}
	})
	assertBudget(t, "search", small, large, searchBudget, detail)
}

// Pinned budgets: the measured store-call count per request shape after
// TKT-1U8XYN. Raise one only with a reason in the commit.
const (
	// list page: scoped count + one bounded page read (listpushdown.go),
	// page edges, neighbor headers, membership walk.
	listPageBudget = 6
	// view: entry, two traverse passes, collection load, section columns +
	// target headers, entry edges, membership walk.
	viewSectionBudget = 11
	// search: whole-type read, membership walk.
	searchBudget = 3
)
