package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
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
			// epic exists solely to give the recursive-budget view its own
			// entry type (see budgetConfig).
			"epic": {Label: "Epic", Properties: str, PropertyOrder: []string{"title"}},
			// program is the nested-budget view's entry type, for the same
			// reason epic is the recursive one's: findViewByEntityType returns
			// the FIRST view matching a type, so two views may not share one.
			"program": {Label: "Program", Properties: str, PropertyOrder: []string{"title"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements":  {Label: "implements", From: []string{"ticket"}, To: []string{"feature"}},
			"blocks":      {Label: "blocks", From: []string{"ticket"}, To: []string{"ticket"}},
			"assigned-to": {Label: "assigned to", From: []string{"ticket"}, To: []string{"person"}},
			"member-of":   {Label: "member of", From: []string{"person"}, To: []string{"team"}},
			"tracked-by":  {Label: "tracked by", From: []string{"ticket"}, To: []string{"epic", "ticket"}},
			// The nested view's two levels. Single-target on purpose: a nested
			// section validates child_columns against the child type, and a
			// multi-`to:` relation leaves that type statically unknown.
			"in-program": {Label: "in program", From: []string{"epic"}, To: []string{"program"}},
			"in-epic":    {Label: "in epic", From: []string{"ticket"}, To: []string{"epic"}},
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
			// A RECURSIVE traversal, which is what exercises the BFS frontier
			// source gate (BUG-9Z20WH). The gate costs one header scan plus one
			// probe per distinct type PER LEVEL, so its cost must scale with
			// DEPTH, never with the number of rows at a level.
			//
			// Entry type is `epic`, NOT `ticket`: findViewByEntityType iterates
			// the views MAP and returns the first entry matching the type, so a
			// second ticket-entry view would make BOTH this test and the
			// non-recursive one pick a random view per run. That is a
			// nondeterminism `-shuffle=on` finds and a local run does not.
			"recursive": {
				Title: "Recursive",
				Entry: dataentryconfig.ViewEntry{Type: "epic"},
				Traverse: []dataentryconfig.ViewTraverse{
					{From: "entry", FollowIncoming: "tracked-by", CollectAs: "chain", Recursive: true},
				},
				Sections: []dataentryconfig.ViewSection{
					{Heading: "Chain", Source: "chain", Display: "list"},
				},
			},
			// The NESTED shape: a program's epics, each with its tickets
			// nested under it. Entry type `program` for the reason given on
			// the recursive view above.
			//
			// Both levels carry columns and the child level a RELATION column,
			// so the measured budget covers resolveRelationColumns — one query
			// per (column, row type), plus the gated batch that turns neighbor
			// ids into titles. A property-only section would leave that
			// unmeasured, which is where the cost actually lives.
			"nested": {
				Title: "Program",
				Entry: dataentryconfig.ViewEntry{Type: "program"},
				Traverse: []dataentryconfig.ViewTraverse{
					{From: "entry", FollowIncoming: "in-program", CollectAs: "epics"},
					{From: "epics", FollowIncoming: "in-epic", CollectAs: "epicTickets"},
				},
				Sections: []dataentryconfig.ViewSection{{
					Heading: "Epics", Source: "epics", Display: dataentryconfig.DisplayNested,
					Children: "epicTickets",
					ParentColumns: map[string][]dataentryconfig.ListColumn{
						"epic": {{Property: "title"}},
					},
					ChildColumns: map[string][]dataentryconfig.ListColumn{
						"ticket": {
							{Property: "status"},
							{Relation: "assigned-to", Label: "Assignee"},
						},
					},
				}},
			},
		},
		Forms:      map[string]dataentryconfig.Form{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
}

// budgetTicketsPerEpic fixes the fan-out of the nested fixture, so the parent
// level grows with n rather than staying pinned at one row.
const budgetTicketsPerEpic = 5

// budgetEpics is how many epics n tickets occupy, rounded up.
//
// readsFor drives {10, 50}, giving 2 and 10 — so the parent level grows with
// the child level, which is what makes the nested pin meaningful. A size below
// budgetTicketsPerEpic would collapse it to a single parent (and 0 would leave
// the program childless), so a third size added there must stay above it or
// the parent dimension stops being measured.
func budgetEpics(n int) int { return (n + budgetTicketsPerEpic - 1) / budgetTicketsPerEpic }

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
	mk("E1", "epic", "Epic one")
	// The nested view's root. BOTH of its levels scale with n: one epic per 5
	// tickets sits at the parent level, the n tickets at the child level.
	//
	// Varying both is the point. relationColumnTargets issues one query per
	// (column, ROW TYPE), so a fixture with a single fixed parent holds the
	// parent level constant and a per-parent N+1 — the likeliest regression in
	// a two-level builder, and the one this ticket was written about
	// (buildNestedEntityData runs per node) — would not move the count.
	mk("PRG1", "program", "Program one")
	for i := 1; i <= budgetEpics(n); i++ {
		mk(fmt.Sprintf("EP%d", i), "epic", fmt.Sprintf("Epic %d", i))
		_, err := counting.CreateRelation(ctx, fmt.Sprintf("EP%d", i), "in-program", "PRG1", nil)
		must(err)
	}
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
		// Every ticket is tracked by E1, so the recursive view's first level
		// holds n rows — the size the budget must be independent of.
		_, err = counting.CreateRelation(ctx, id, "tracked-by", "E1", nil)
		must(err)
		// Spread across the nested view's epics, so both levels grow with n.
		_, err = counting.CreateRelation(ctx, id, "in-epic", fmt.Sprintf("EP%d", (i-1)/budgetTicketsPerEpic+1), nil)
		must(err)
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

// A RECURSIVE view traversal, the shape that exercises the BFS frontier source
// gate (BUG-9Z20WH). This is the budget the gate's own doc comment claims: one
// header scan + one probe per distinct type per LEVEL, so the cost is a
// function of depth, not of how many rows sit at a level. The fixture puts
// n-1 blockers one level below TKT-0001, so if the gate were per-node this
// count would grow from 10 to 50 rows.
func TestQueryBudget_RecursiveViewTraversalIsSizeIndependent(t *testing.T) {
	small, large, detail := readsFor(t, func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
		t.Helper()
		rec := viewsAs(ctx, t, app, d, "epic", "E1")
		if rec.Code != http.StatusOK {
			t.Fatalf("view: %d %s", rec.Code, rec.Body)
		}
	})
	assertBudget(t, "recursive view traversal", small, large, recursiveViewBudget, detail)
}

// The budget fixtures pin numbers for a config an operator could actually
// load. newAppFromParts publishes the schema DIRECTLY (test_helpers_test.go),
// never calling ValidateConfig, so nothing else in this file would notice a
// fixture that validation refuses — and a budget pinned for a section shape
// that cannot exist measures nothing.
//
// This asserts only that validation ACCEPTS these fixtures. The nested
// section's own rules (children differs from source, the children rule
// traverses from the section's source, no recursive:, no columns:) are pinned
// by TestValidateConfig_NestedSection in internal/dataentryconfig, which is
// where they can actually be exercised against invalid input.
func TestQueryBudget_FixtureConfigIsValid(t *testing.T) {
	if err := dataentryconfig.ValidateConfig(nil, budgetConfig(), budgetMeta()); err != nil {
		t.Fatalf("budget fixture config is not loadable: %v", err)
	}
}

// A `display: nested` section: a program's epics, each with its tickets
// nested under it (TKT-M0WMEE).
//
// BOTH levels grow with n — epics 2→10 as tickets 10→50 — so this pins the
// per-parent dimension as well as the per-child one. A fixture with a single
// fixed parent would hold the parent level constant, and a per-parent N+1
// (buildNestedEntityData runs once per node) would not move the count at all.
//
// Two levels and a relation column make this the widest of the view budgets:
// resolveRelationColumns issues one ListRelations per (column, ROW TYPE), and
// a nested section has two row types where a table section has one.
func TestQueryBudget_NestedSectionIsSizeIndependent(t *testing.T) {
	small, large, detail := readsFor(t, func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
		t.Helper()
		rec := viewsAs(ctx, t, app, d, "program", "PRG1")
		if rec.Code != http.StatusOK {
			t.Fatalf("view: %d %s", rec.Code, rec.Body)
		}
	})
	assertBudget(t, "nested section", small, large, nestedSectionBudget, detail)
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
	// recursive view: entry, the fixpoint's relation queries, the collection
	// load, and the BFS frontier source gate's ONE header scan per level
	// walked (BUG-9Z20WH). Measured, not derived; the point of the pin is that
	// it does not move with row count — the fixture puts every ticket one
	// level below the epic, so a per-node gate would make this grow.
	recursiveViewBudget = 12
	// nested view: entry, the two traverse passes, the collection loads, the
	// entry edges and the membership walk, plus the relation column's own
	// reads — one ListRelations per ROW TYPE (epic and ticket) and the gated
	// batch that turns neighbor ids into titles.
	//
	// Measured, not derived. No committed test pins the per-leg split, so
	// treat the breakdown above as orientation rather than an assertion.
	//
	// None of the three scales with row count, which is the point — but a
	// COUNT cannot see the whole property. A regression that resolved over
	// every visible child would still issue exactly these reads and merely
	// hand them a far larger id set, leaving this pin unmoved. That is why
	// this is necessary but not sufficient, and why
	// TestQueryBudget_NestedRelationColumnsResolveOverEmittedRowsOnly measures
	// the breadth instead.
	nestedSectionBudget = 15
	// nestedRelationIDsPerRow is how many relation-column ids a nested section
	// may spend PER EMITTED ROW, as a numerator over
	// nestedRelationIDsPerRowDiv.
	//
	// Per row, not a flat total, because that is the actual property: the
	// section's relation queries are issued over the rows it renders, so their
	// width must track the emitted tree and not the visible one. Only the
	// visible tree is unbounded — it grows with the project, while the emitted
	// one cannot exceed nestedNodeBudget — and turning the first into the
	// second is the RR-HKHPYG defect.
	//
	// 5/4 = 1.25 against a measured 1.086. Mutation-tested sensitivity: an
	// over-fetch of +600 rows is caught, +300 is not. A flat "2 per row"
	// allowance missed +1200, which is what a loosely fitted threshold buys
	// you — room for the next regression to live inside the slack.
	//
	// The residual ~13% is deliberate and is NOT a claim that a smaller
	// over-fetch is acceptable. It is the margin that keeps the test from
	// failing on incidental fixture drift; closing it entirely would mean
	// pinning the measured total, which then moves whenever the fixture does.
	//
	// It is a ratio and not a pinned total so it does not move when the
	// fixture does. Raise it only with a measurement showing the legitimate
	// per-row cost genuinely grew.
	nestedRelationIDsPerRow    = 5
	nestedRelationIDsPerRowDiv = 4
)

// The breadth companion to the pin above (RR-HKHPYG, TKT-M0WMEE AC3).
//
// A nested section must resolve its relation columns over the rows its budget
// will EMIT, not over every child the caller can see. Those two sets differ
// only when the visible tree overflows nestedNodeBudget, so this fixture is
// deliberately built to overflow it: far more visible children exist than any
// response can carry.
//
// This cannot be a read-COUNT assertion, which is why it needs its own
// instrument. The section's relation queries are issued per (column, row
// type), not per row, so widening them from the emitted rows to every visible
// child changes how many ids each call carries and not how many calls there
// are: the defect was byte-identical under storetest.Counting, before and
// after the fix.
//
// The measured leg is ListRelations. ListEntityHeaders is NOT a useful signal
// here even though resolveRelationColumns ends in it, because
// relationColumnTargets dedupes its target ids: the header batch is bounded by
// distinct NEIGHBORS rather than by row count, and this fixture assigns every
// ticket to one person, so it carries a single id whatever the section does.
// A deployment with many assignees would see it grow — but bounded by the
// assignee count, which is not the quantity this test is about.
func TestQueryBudget_NestedRelationColumnsResolveOverEmittedRowsOnly(t *testing.T) {
	breadth := storetest.NewBreadth(memstore.New())
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
		must(breadth.CreateEntity(ctx, e))
	}

	mk("T1", "team", "Team one")
	mk("P1", "person", "Person one")
	_, err := breadth.CreateRelation(ctx, "P1", "member-of", "T1", nil)
	must(err)
	mk("PRG1", "program", "Program one")

	// Sized so the VISIBLE universe far exceeds what the emitted tree can
	// carry — the gap the assertion reads. Derived from the budget constants
	// rather than hardcoded: a fixed parent count could quietly stop
	// straddling if nestedNodeBudget or nestedChildPreview moved, and the
	// guard would pass for the wrong reason (which is how two earlier
	// attempts at this test failed).
	//
	// Fan-out is high and the parent count low on purpose. Each parent emits
	// at most nestedChildPreview children whatever its true size, so wide
	// parents overflow the visible/emitted gap far more cheaply than many
	// narrow ones: the seeding cost is one relation per child either way, but
	// fewer parents means fewer epics and a smaller parent level.
	const childrenPerParent = nestedChildPreview * 3
	parents := nestedNodeBudget/nestedChildPreview + 5
	visibleChildren := 0
	for p := 1; p <= parents; p++ {
		epic := fmt.Sprintf("EP%d", p)
		mk(epic, "epic", epic)
		_, err := breadth.CreateRelation(ctx, epic, "in-program", "PRG1", nil)
		must(err)
		for c := range childrenPerParent {
			id := fmt.Sprintf("TKT-%d-%d", p, c)
			e := entity.New(id, "ticket")
			e.SetString("title", id)
			e.SetString("status", "open")
			must(breadth.CreateEntity(ctx, e))
			_, err := breadth.CreateRelation(ctx, id, "in-epic", epic, nil)
			must(err)
			_, err = breadth.CreateRelation(ctx, id, "assigned-to", "P1", nil)
			must(err)
			visibleChildren++
		}
	}

	app := newAppFromParts(budgetConfig(), budgetMeta(), newFixture(), appbuildtest.WithStore(breadth))
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"*"}}},
		Assignments: map[string]string{"T1": "editor"},
	}, app.store)
	app.acl = d
	breadth.Reset()

	rec := viewsAs(principalCtx("P1"), t, app, d, "program", "PRG1")
	if rec.Code != http.StatusOK {
		t.Fatalf("view: %d %s", rec.Code, rec.Body)
	}

	// Count what was actually RENDERED, from the response itself. The bound
	// below is a function of this rather than a constant, because the property
	// under test is "proportional to the emitted tree, not to the visible
	// one" — and a fixed threshold expresses that only for over-fetches large
	// enough to clear it. A constant of 2x nestedNodeBudget, measured against
	// ~2.5k ids of legitimate traffic, left ~1.4k ids of slack: a regression
	// that over-fetched by a thousand rows passed it.
	var resp v1.ViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode view response: %v", err)
	}
	emitted := 0
	for _, sec := range resp.Sections {
		for _, node := range sec.Tree {
			emitted += 1 + len(node.Children)
		}
	}
	if emitted == 0 {
		t.Fatal("no nested rows emitted; the fixture is not exercising the section")
	}
	// The emitted tree must be CAPPED, not merely non-empty. If it grew to
	// meet the visible universe the two sets would coincide and the assertion
	// below could not tell them apart — the straddle needs both halves, and
	// asserting only the upper one would leave this passing vacuously after a
	// budget change.
	if emitted >= visibleChildren {
		t.Fatalf("fixture does not overflow: %d rows emitted from %d visible children, "+
			"so emitted and visible cannot be distinguished", emitted, visibleChildren)
	}

	// The fixture must be able to TRIP the assertion, or it proves nothing:
	// the defect resolves over every visible child, so the visible universe
	// has to exceed what the emitted tree permits. Both halves are asserted —
	// checking only the upper one would still pass if a change let the emitted
	// tree grow to meet it.
	if allowance := emitted * nestedRelationIDsPerRow / nestedRelationIDsPerRowDiv; visibleChildren <= allowance {
		t.Fatalf("fixture cannot trip the guard: %d visible children does not exceed "+
			"the %d-id allowance for %d emitted rows",
			visibleChildren, allowance, emitted)
	}

	// ListRelations is the signal, and ONLY it.
	//
	// relationColumnTargets issues its queries as RelationQuery.EntityIDs over
	// the rows being rendered, so this is the read the defect widened.
	//
	// Deliberately NOT asserted on ListEntities. That read is byte-identical
	// with and without the defect, because the view pipeline loads each
	// collection in full before any section builder runs — so it cannot serve
	// as the guard, and folding it in made the assertion untrippable on the
	// first attempt. It is also large (~2x the visible universe across its
	// calls), which is a real cost question but a different one; this test
	// makes no claim about it.
	//
	// ListEntityHeaders is likewise not the signal HERE: relationColumnTargets
	// dedupes its target ids, so the header batch is bounded by distinct
	// neighbors rather than by row count. This fixture assigns every ticket
	// to one person, so that call carries a single id and would be silent
	// whatever the section did.
	limit := emitted * nestedRelationIDsPerRow / nestedRelationIDsPerRowDiv
	if got := breadth.IDs("ListRelations"); got > limit {
		t.Errorf("relation columns resolved over %d ids to render %d rows "+
			"(allowance %d = %d/%d per row; %d children visible) — selection must precede "+
			"resolution (RR-HKHPYG); breadth: %s",
			got, emitted, limit, nestedRelationIDsPerRow, nestedRelationIDsPerRowDiv, visibleChildren, breadth)
	}
}
