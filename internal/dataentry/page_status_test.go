package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// done_when per step (TKT-MWZF0UO): /api/v1/_page_status judges each step
// of a steps page over the rows its list shows the principal.

// pageStatusReq calls the endpoint with ctx and the given query.
func pageStatusReq(ctx context.Context, app *App, method, rawQuery string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/_page_status?"+rawQuery, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1PageStatus(app, rec, req)
	return rec
}

// pageStatusAs calls the endpoint for a page and decodes its steps.
func pageStatusAs(ctx context.Context, t *testing.T, app *App, page, anchor string) map[string]v1.PageStepStatus {
	t.Helper()
	q := url.Values{"page": {page}}
	if anchor != "" {
		q.Set("entity", anchor)
	}
	rec := pageStatusReq(ctx, app, http.MethodGet, q.Encode())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store",
		"the answer is per principal, so no shared cache may hold it")
	var resp v1.PageStatusResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	return resp.Steps
}

// stepsPage is a steps page over the list "all" whose tabs carry the given
// done_when expressions, in order, with ids s0, s1, ...
func stepsPage(doneWhen ...string) map[string]dataentryconfig.Page {
	tabs := make([]dataentryconfig.PageTab, 0, len(doneWhen))
	for i, cond := range doneWhen {
		id := "s" + string(rune('0'+i))
		tabs = append(tabs, dataentryconfig.PageTab{ID: id, Label: id, List: "all", DoneWhen: cond})
	}
	return map[string]dataentryconfig.Page{"workshop": {
		PageHeader: dataentryconfig.PageHeader{Label: "Workshop"}, Mode: dataentryconfig.PageModeSteps, Tabs: tabs,
	}}
}

// A step is done when every row of its list matches and there is at least
// one; a tab without done_when is absent.
func TestPageStatus_EveryRowMustMatch(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "A", "status": "done"}})
	seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "B", "status": "done"}})
	seedEntity(app, &entity.Entity{ID: "TKT-3", Type: "ticket", Properties: map[string]any{"title": "C", "status": "open"}})

	pages := stepsPage("entity.status == 'done' or entity.status == 'open'", "entity.status == 'done'",
		"entity.title == 'A'", "true", "")
	tabs := pages["workshop"].Tabs
	tabs[2].List = "done"
	tabs[3].ID, tabs[3].List = "empty", "none"
	tabs[4].ID = "plain"
	installPagesConfig(app, map[string]dataentryconfig.List{
		"all":  {EntityType: "ticket"},
		"done": {EntityType: "ticket", Condition: "entity.status == 'done'"},
		"none": {EntityType: "ticket", Condition: "entity.status == 'nope'"},
	}, pages, nil)

	require.Equal(t, map[string]v1.PageStepStatus{
		"s0":    {Done: true},
		"s1":    {Done: false},
		"s2":    {Done: false},
		"empty": {Done: false},
	}, pageStatusAs(principalCtx("alice"), t, app, "workshop", ""))
}

// The list's own narrowing applies before done_when: a step over a list
// whose condition keeps only matching rows is done even though the type
// holds rows that do not match.
func TestPageStatus_JudgesTheListsRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "A", "status": "done"}})
	seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "B", "status": "open"}})
	installPagesConfig(app, map[string]dataentryconfig.List{
		"all": {EntityType: "ticket", Condition: "entity.status == 'done'"},
	}, stepsPage("entity.status == 'done'"), nil)
	require.True(t, pageStatusAs(principalCtx("alice"), t, app, "workshop", "")["s0"].Done)
}

// A row the principal cannot read never changes the answer: a hidden row
// that fails done_when does not undo the step, and a hidden row that
// matches does not complete it.
func TestPageStatus_HiddenRowsDoNotCount(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Visible", "status": "done"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket",
		Properties: map[string]any{"title": "Hidden", "status": "open"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-002", "belongs-to", "PRJ-9"))
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		stepsPage("entity.status == 'done'", "entity.status == 'open'"), nil)

	// Without a policy both rows count, so the visible answer below differs.
	require.Equal(t, map[string]v1.PageStepStatus{"s0": {Done: false}, "s1": {Done: false}},
		pageStatusAs(principalCtx("alice"), t, app, "workshop", ""))

	d := mustNewACL(t, &acl.Policy{
		Roles:               map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	setTestACL(t, app, d)
	require.Equal(t, map[string]v1.PageStepStatus{"s0": {Done: true}, "s1": {Done: false}},
		pageStatusAs(gateCtxFor(aliceCtx(), t, d), t, app, "workshop", ""),
		"only the ticket alice may read is judged")
}

// On an entity page each step is judged over the rows its tab keeps for the
// anchor, and a hidden anchor is the uniform 404.
func TestPageStatus_EntityPage(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedPageScopeGraph(app)
	installEntityPage(app)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	page := cfg.Pages["feat"]
	page.Mode = dataentryconfig.PageModeSteps
	page.Tabs = []dataentryconfig.PageTab{page.Tabs[0]}
	page.Tabs[0].DoneWhen = "entity.status == 'open'"
	cfg.Pages = map[string]dataentryconfig.Page{"feat": page}
	next.Cfg = &cfg
	app.schema.Publish(&next)

	require.False(t, pageStatusAs(principalCtx("alice"), t, app, "feat", "FEAT-1")["tickets"].Done,
		"FEAT-1 has a done ticket")
	require.True(t, pageStatusAs(principalCtx("alice"), t, app, "feat", "FEAT-2")["tickets"].Done)

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	hidden := pageStatusReq(gateCtxFor(aliceCtx(), t, d), app, http.MethodGet, "page=feat&entity=FEAT-1")
	missing := pageStatusReq(gateCtxFor(aliceCtx(), t, d), app, http.MethodGet, "page=feat&entity=FEAT-NOPE")
	require.Equal(t, http.StatusNotFound, hidden.Code, hidden.Body.String())
	require.Equal(t, withoutInstance(t, missing.Body.String()), withoutInstance(t, hidden.Body.String()),
		"a hidden anchor and a missing one differ")
}

// A tab the principal does not see gets no status, and one whose done_when
// is not compiled is left out rather than reported done.
func TestPageStatus_GatedAndUncompiledStepsAreOmitted(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T", "status": "done"}})
	pages := stepsPage("entity.status == 'done'", "entity.status == 'done'")
	pages["workshop"].Tabs[1].Permission = "admin:read"
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, pages, nil)

	require.Empty(t, pageStatusAs(principalCtx("bob"), t, app, "workshop", ""),
		"without compiled conditions no step is judged")

	wireRealViewConditions(t, app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	setTestACL(t, app, d)
	require.Equal(t, map[string]v1.PageStepStatus{"s0": {Done: true}},
		pageStatusAs(gateCtxFor(principalCtx("bob"), t, d), t, app, "workshop", ""))
	require.Len(t, pageStatusAs(gateCtxFor(principalCtx("alice"), t, d), t, app, "workshop", ""), 2)
}

func TestPageStatus_BadRequests(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Pages["plain"] = stepsPage("true")["workshop"]
	next.Cfg = &cfg
	app.schema.Publish(&next)

	for _, tc := range []struct {
		name, method, query string
		want                int
	}{
		{"unknown page", http.MethodGet, "page=nope", http.StatusNotFound},
		{"entity page without entity", http.MethodGet, "page=feat", http.StatusBadRequest},
		{"entity on a plain page", http.MethodGet, "page=plain&entity=FEAT-1", http.StatusBadRequest},
		{"post", http.MethodPost, "page=plain", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := pageStatusReq(principalCtx("alice"), app, tc.method, tc.query)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}
}

// The sidebar says which steps have a check and which walk, never whether a
// step is done: that is per principal.
func TestPageStatus_SidebarCarriesOnlyTheFlag(t *testing.T) {
	app := newTestAppV1(t)
	pages := stepsPage("true", "")
	pages["workshop"].Tabs[1].Walk = true
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, pages, nil)
	resp, _ := sidebarFor(principalCtx("alice"), t, app, "")
	tabs := resp.Pages["workshop"].Tabs
	require.Equal(t, v1.SidebarPageTab{ID: "s0", Label: "s0", View: "list", Target: "all", DoneCheck: true}, tabs[0])
	require.Equal(t, v1.SidebarPageTab{ID: "s1", Label: "s1", View: "list", Target: "all", Walk: true}, tabs[1])
}

// A world-bound page judges each step over the faces its list shows in that
// world, not the default world's.
func TestPageStatus_JudgesTheWorldsRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedFacedTickets(t, app)
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		stepsPage("entity.title == 'walrus onboarding'"), nil)

	for _, tc := range []struct {
		name, query string
		done        bool
	}{
		{"default world shows the draft faces", "page=workshop", false},
		{"published world shows only the published face", "page=workshop&world=published", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := viewRecord(t, app, "/api/v1/_page_status?"+tc.query)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var resp v1.PageStatusResponse
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			require.Equal(t, map[string]v1.PageStepStatus{"s0": {Done: tc.done}}, resp.Steps)
		})
	}
}
