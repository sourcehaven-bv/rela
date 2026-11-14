package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// searchPlanMeta has a property of every shape planSearch judges: plain
// strings, an inline enum, a custom enum type, the legacy `status` type, a
// date, an integer and a list.
func searchPlanMeta() *metamodel.Metamodel {
	str := metamodel.PropertyDef{Type: "string"}
	workflow := metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum, Values: []string{"todo", "doing", "done"}}
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{"severity": {Values: []string{"low", "high"}}},
		Entities: map[string]metamodel.EntityDef{
			"task": {Label: "Task", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true}, "owner": str, "state": workflow,
				"level": {Type: "severity"}, "legacy": {Type: "status"},
				"due": {Type: metamodel.PropertyTypeDate}, "points": {Type: metamodel.PropertyTypeInteger},
				"tags": {Type: "string", List: true},
			}, PropertyOrder: []string{"title", "owner", "state", "level", "legacy", "due", "points", "tags"}},
			"bug": {Label: "Bug", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true}, "owner": str, "state": workflow,
			}, PropertyOrder: []string{"title", "owner", "state"}},
			"person": {Label: "Person", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}, PropertyOrder: []string{"title"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"editor-of": {Label: "editor of", From: []string{"person"}, To: []string{"task"}},
		},
	}
}

// newSearchPlanApp seeds tasks and bugs whose properties are present,
// empty or missing in turn, so the plan meets every emptiness case.
func newSearchPlanApp(t *testing.T, opts ...appbuildtest.Option) *App {
	t.Helper()
	app := newAppFromParts(&Config{
		Lists: map[string]dataentryconfig.List{}, Views: map[string]dataentryconfig.ViewConfig{},
	}, searchPlanMeta(), newFixture(), opts...)
	owners := []any{"ann", "bob", "", nil}
	states := []string{"todo", "doing", "done"}
	for i := 1; i <= 12; i++ {
		props := map[string]any{
			"title":  fmt.Sprintf("Task %c", 'a'+(i*7)%12),
			"state":  states[i%3],
			"level":  []string{"low", "high"}[i%2],
			"due":    "2026-01-01",
			"points": i % 4,
		}
		if o := owners[i%4]; o != nil {
			props["owner"] = o
		}
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TASK-%02d", i), Type: "task", Properties: props})
	}
	for i := 1; i <= 5; i++ {
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("BUG-%02d", i), Type: "bug", Properties: map[string]any{
			"title": fmt.Sprintf("Bug %d", 6-i), "state": states[i%3], "owner": owners[i%2],
		}})
	}
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	return app
}

// restrictSearchPlanApp installs an ACL under which alice reads the tasks
// she is an editor of (a compiled read query) and every bug (allow-all), and
// returns her gated context and the policy.
func restrictSearchPlanApp(t *testing.T, app *App) (context.Context, *acl.Declarative) {
	t.Helper()
	for _, id := range []string{"TASK-02", "TASK-03", "TASK-05", "TASK-08", "TASK-11"} {
		seedRelation(app, entity.NewRelation("alice", "editor-of", id))
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"task"}},
			"viewer": {Read: []string{"bug"}},
		},
		Assignments:   map[string]string{"alice": "viewer"},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
	}, app.store)
	setTestACL(t, app, d)
	return gateCtxFor(aliceCtx(), t, d), d
}

// searchPlanCases are the queries the differential test runs, with whether
// planSearch must take them. A case the plan declines still runs, so the
// fallback is pinned to the same answer.
var searchPlanCases = []struct {
	query, exactType string
	plans            bool
}{
	{"type:task", "", true},
	{"type:task prop:state=doing", "", true},
	{"type:task prop:owner=ann", "", true},
	{"type:task prop:owner!=ann", "", true},
	{"type:task prop:owner=", "", true},
	{"type:task prop:level=high", "", true},
	{"type:task sort:title", "", true},
	{"type:task sort:state:desc", "", true},
	{"type:task sort:owner sort:title:desc", "", true},
	{"type:task prop:state!=done sort:owner:desc", "", true},
	{"type:task,bug prop:state=todo", "", true},
	{"type:bug,task", "", true},
	{"type:task,bug", "task", true},
	{"type:bug", "task", true},
	{"prop:state=todo", "bug", true},
	{"type:task,task", "", true},
	// The query /_search builds for `type=task` and `q=type:task ...`.
	{"type:task type:task prop:state=doing", "task", true},
	{"type:task type:task sort:modified:desc", "task", false},
	// Declined: the store would answer differently, or cannot answer.
	{"type:task,bug sort:title", "", false},
	{"type:task prop:due=2026-01-01", "", false},
	{"type:task prop:points=1", "", false},
	{"type:task prop:state=nope", "", false},
	{"type:task prop:legacy=open", "", false},
	{"type:task prop:owner!=", "", false},
	{"type:task prop:owner=a*", "", false},
	{"type:task prop:tags=x", "", false},
	{"type:task sort:modified:desc", "", false},
	{"type:task sort:points", "", false},
	{"prop:state=todo", "", false},
}

// TestSearchPlan_MatchesGoPath is the differential test the plan's
// eligibility rests on: for every case, searchPage returns the same rows,
// in the same order, with the same total, as the Go path (executeQueryWith,
// then the type filter and the cut), for an allow-all reader and for one
// whose task reads go through a compiled query.
func TestSearchPlan_MatchesGoPath(t *testing.T) {
	assertSearchPlanMatchesGoPath(t, func(t *testing.T) *App { t.Helper(); return newSearchPlanApp(t) })
}

// assertSearchPlanMatchesGoPath runs the differential test on the apps
// newApp builds, one per principal, so a backend test can run it on its
// own store.
func assertSearchPlanMatchesGoPath(t *testing.T, newApp func(*testing.T) *App) {
	t.Helper()
	for _, principal := range []string{"nop", "alice"} {
		app := newApp(t)
		ctx := principalCtx("anyone")
		if principal == "alice" {
			ctx, _ = restrictSearchPlanApp(t, app)
		}
		for _, tc := range searchPlanCases {
			for _, limit := range []int{0, 1, 3} {
				name := fmt.Sprintf("%s/%s/%s/limit=%d", principal, tc.query, tc.exactType, limit)
				t.Run(name, func(t *testing.T) {
					_, ok := planSearch(app.State().Meta, searchparser.ParseQuery(tc.query), tc.exactType)
					if ok != tc.plans {
						t.Fatalf("planSearch ok = %v, want %v", ok, tc.plans)
					}
					opts := queryOptions{limit: limit, exactType: tc.exactType}
					got, gotTotal, err := app.queries.searchPage(ctx, tc.query, opts)
					if err != nil {
						t.Fatal(err)
					}
					want, err := app.queries.executeQueryWith(ctx, tc.query, opts)
					if err != nil {
						t.Fatal(err)
					}
					if tc.exactType != "" {
						want = slices.DeleteFunc(want, func(e *entity.Entity) bool { return e.Type != tc.exactType })
					}
					wantTotal := len(want)
					if limit > 0 && len(want) > limit {
						want = want[:limit]
					}
					if g, w := entityIDs(got), entityIDs(want); !slices.Equal(g, w) {
						t.Errorf("rows\n got %v\nwant %v", g, w)
					}
					if gotTotal != wantTotal {
						t.Errorf("total = %d, want %d", gotTotal, wantTotal)
					}
				})
			}
		}
	}
}

// TestSearchPlan_PositionMatchesGoPath pins the search scope's store
// position to the Go path's: the index of each row in the full result, its
// neighbors, the total, and not-found for a row outside the result.
func TestSearchPlan_PositionMatchesGoPath(t *testing.T) {
	assertSearchPositionMatchesGoPath(t, func(t *testing.T) *App { t.Helper(); return newSearchPlanApp(t) })
}

// assertSearchPositionMatchesGoPath is [assertSearchPlanMatchesGoPath] for
// the position.
func assertSearchPositionMatchesGoPath(t *testing.T, newApp func(*testing.T) *App) {
	t.Helper()
	for _, principal := range []string{"nop", "alice"} {
		app := newApp(t)
		ctx := principalCtx("anyone")
		if principal == "alice" {
			ctx, _ = restrictSearchPlanApp(t, app)
		}
		for _, tc := range searchPlanCases {
			t.Run(principal+"/"+tc.query+"/"+tc.exactType, func(t *testing.T) {
				want, err := app.queries.executeQueryWith(ctx, tc.query, queryOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if tc.exactType != "" {
					want = slices.DeleteFunc(want, func(e *entity.Entity) bool { return e.Type != tc.exactType })
				}
				ids := append(entityIDs(want), "TASK-99", "alice")
				for _, id := range ids {
					pos, found, handled, err := app.queries.searchPosition(ctx, tc.query, tc.exactType, id)
					if err != nil {
						t.Fatal(err)
					}
					if !handled {
						// Several types, or a query the plan declines:
						// the Go path answers, as before.
						return
					}
					idx := slices.IndexFunc(want, func(e *entity.Entity) bool { return e.ID == id })
					if found != (idx >= 0) {
						t.Fatalf("%s: found = %v, want %v", id, found, idx >= 0)
					}
					if !found {
						continue
					}
					if pos.Index != idx+1 || pos.Total != len(want) {
						t.Errorf("%s: position %d of %d, want %d of %d", id, pos.Index, pos.Total, idx+1, len(want))
					}
					if wantPrev := neighborID(want, idx-1); refID(pos.Prev) != wantPrev {
						t.Errorf("%s: prev %q, want %q", id, refID(pos.Prev), wantPrev)
					}
					if wantNext := neighborID(want, idx+1); refID(pos.Next) != wantNext {
						t.Errorf("%s: next %q, want %q", id, refID(pos.Next), wantNext)
					}
				}
			})
		}
	}
}

// TestSearchPlan_HidesUnreadableRows pins the negative cases of the store
// path directly, not only by agreement with the Go path: a task alice may not
// read is neither served, counted nor positioned, a principal with no role
// reads nothing, and a request with no gate attached fails closed.
func TestSearchPlan_HidesUnreadableRows(t *testing.T) {
	app := newSearchPlanApp(t)
	alice, d := restrictSearchPlanApp(t, app)
	readable := []string{"TASK-02", "TASK-03", "TASK-05", "TASK-08", "TASK-11"}
	const query = "type:task sort:title"

	t.Run("page and total", func(t *testing.T) {
		for _, limit := range []int{0, 1} {
			rows, total, err := app.queries.searchPage(alice, query, queryOptions{limit: limit})
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range entityIDs(rows) {
				if !slices.Contains(readable, id) {
					t.Errorf("limit %d: served hidden %s", limit, id)
				}
			}
			if total != len(readable) {
				t.Errorf("limit %d: total = %d, want %d", limit, total, len(readable))
			}
		}
	})

	t.Run("position of a hidden row", func(t *testing.T) {
		_, found, handled, err := app.queries.searchPosition(alice, query, "", "TASK-01")
		if err != nil || !handled || found {
			t.Fatalf("searchPosition(TASK-01) = found %v, handled %v, err %v; want handled and not found",
				found, handled, err)
		}
		req := httptest.NewRequest(http.MethodGet,
			positionURL(t, "TASK-01", ScopeDescriptor{Source: "search", Q: query}), http.NoBody).WithContext(alice)
		rec := httptest.NewRecorder()
		app.handleV1EntityPosition(rec, req)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not_in_scope") {
			t.Errorf("position of a hidden row: %d %s, want 404 not_in_scope", rec.Code, rec.Body)
		}
	})

	for name, ctx := range map[string]context.Context{
		"no role": gateCtxFor(principalCtx("mallory"), t, d),
		// Enforced, as attachACLRequest marks every request, but no gate.
		"no gate": withACLEnforced(aliceCtx()),
	} {
		t.Run(name, func(t *testing.T) {
			rows, total, err := app.queries.searchPage(ctx, query, queryOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 0 || total != 0 {
				t.Errorf("rows %v, total %d; want none", entityIDs(rows), total)
			}
		})
	}
}

func entityIDs(es []*entity.Entity) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.ID)
	}
	return out
}

func neighborID(es []*entity.Entity, i int) string {
	if i < 0 || i >= len(es) {
		return ""
	}
	return es[i].ID
}

func refID(r *store.EntityRef) string {
	if r == nil {
		return ""
	}
	return r.ID
}
