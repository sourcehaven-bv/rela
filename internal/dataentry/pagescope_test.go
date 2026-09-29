package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Entity pages (pagescope.go): a tab of an entity page narrows its collection
// read to the rows the anchor reaches over the tab's scope relation.

// installEntityPage publishes a `feat` entity page over features whose
// `tickets` tab lists the tickets implementing the anchor.
func installEntityPage(app *App) {
	installPagesConfig(app, map[string]dataentryconfig.List{"tickets": {EntityType: "ticket"}},
		map[string]dataentryconfig.Page{
			"feat": {Label: "Feature", EntityType: "feature", Badge: "title", Tabs: []dataentryconfig.PageTab{
				{ID: "tickets", Label: "Tickets", List: "tickets", Scope: &dataentryconfig.PageTabScope{
					Relation: "implements", Direction: dataentryconfig.DirectionIncoming,
				}},
			}},
		}, nil)
}

// seedPageScopeGraph seeds FEAT-1 implemented by TKT-001..003 and FEAT-2
// implemented by TKT-004, with TKT-005 implementing nothing.
func seedPageScopeGraph(app *App) {
	for _, id := range []string{"FEAT-1", "FEAT-2"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "feature", Properties: map[string]any{"title": id}})
	}
	for i, feat := range []string{"FEAT-1", "FEAT-1", "FEAT-1", "FEAT-2", ""} {
		id := []string{"TKT-001", "TKT-002", "TKT-003", "TKT-004", "TKT-005"}[i]
		status := "open"
		if i == 1 {
			status = "done"
		}
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id, "status": status}})
		if feat != "" {
			seedRelation(app, entity.NewRelation(id, "implements", feat))
		}
	}
}

// pageScopeQuery is the query string an entity-page tab sends.
func pageScopeQuery(anchor string, extra ...string) string {
	q := url.Values{"scope_page": {"feat"}, "scope_tab": {"tickets"}, "anchor": {anchor}}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Add(extra[i], extra[i+1])
	}
	return q.Encode()
}

func scopedIDs(resp v1.ListResponse) []string {
	ids := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids
}

// listUngated calls the list handler with no ACL on the context, which is the
// shape under which the store pushdown may serve the request.
func listUngated(t *testing.T, app *App, plural, typeName, rawQuery string) (v1.ListResponse, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+plural+"?"+rawQuery, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, typeName, plural)
	var resp v1.ListResponse
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	}
	return resp, rec
}

// withoutInstance decodes a problem body and drops its request path.
func withoutInstance(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &m))
	delete(m, "instance")
	return m
}

func TestPageScope_NarrowsToAnchor(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)

	resp, rec := listUngated(t, app, "tickets", "ticket", pageScopeQuery("FEAT-1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []string{"TKT-001", "TKT-002", "TKT-003"}, scopedIDs(resp))
	require.Equal(t, 3, resp.Meta.Total)

	// Filters apply within the scope, never beside it.
	resp, rec = listUngated(t, app, "tickets", "ticket", pageScopeQuery("FEAT-1", "filter[status]", "open"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []string{"TKT-001", "TKT-003"}, scopedIDs(resp))
}

// A paged request with a scope must not be served by the store pushdown,
// which knows nothing of the scope and would page and count the whole type.
func TestPageScope_DeclinesPushdown(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)

	resp, rec := listUngated(t, app, "tickets", "ticket", pageScopeQuery("FEAT-1", "per_page", "2", "page", "2"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 3, resp.Meta.Total, "the count describes the scoped set, not the type")
	require.Len(t, resp.Data, 1)
}

// A hidden anchor, a missing one and one of another type answer with the
// same 404 an entity GET gives, byte for byte.
func TestPageScope_HiddenAnchorIsIndistinguishable(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	bodies := map[string]string{}
	for _, anchor := range []string{"FEAT-1", "FEAT-NOPE", "TKT-001"} {
		_, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", pageScopeQuery(anchor))
		require.Equal(t, http.StatusNotFound, rec.Code, "anchor %s: %s", anchor, rec.Body)
		bodies[anchor] = rec.Body.String()
	}
	require.Equal(t, bodies["FEAT-NOPE"], bodies["FEAT-1"], "hidden and missing anchors differ")
	require.Equal(t, bodies["FEAT-NOPE"], bodies["TKT-001"], "wrong-type and missing anchors differ")

	// The entity GET's problem body differs only in `instance`, the request path.
	rec := getEntityAs(aliceCtx(), t, app, d, "feature", "features", "FEAT-NOPE", "")
	require.Equal(t, withoutInstance(t, rec.Body.String()), withoutInstance(t, bodies["FEAT-NOPE"]),
		"the scope 404 differs from an entity GET 404")
}

// An anchor of another type is refused even when the principal may read it.
func TestPageScope_OtherTypeAnchor(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)

	_, rec := listUngated(t, app, "tickets", "ticket", pageScopeQuery("TKT-001"))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// The scope is an intersection: a linked row the principal may not read stays
// out, and the anchor adds nothing the plain list would not show.
func TestPageScope_NeverWidens(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-1", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-1"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-1"))
	seedRelation(app, entity.NewRelation("FEAT-1", "belongs-to", "PRJ-1"))

	d := mustNewACL(t, &acl.Policy{
		Roles:               map[string]acl.RoleDef{"editor": {Read: []string{"ticket", "feature"}}},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", pageScopeQuery("FEAT-1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []string{"TKT-001"}, scopedIDs(resp), "TKT-002 and TKT-003 are linked but hidden")
	require.Equal(t, 1, resp.Meta.Total)
}

func TestPageScope_BadRequests(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)

	cases := []struct {
		name, plural, typeName, query string
	}{
		{"unknown tab", "tickets", "ticket", "scope_page=feat&scope_tab=nope&anchor=FEAT-1"},
		{"unknown page", "tickets", "ticket", "scope_page=nope&scope_tab=tickets&anchor=FEAT-1"},
		{"missing anchor", "tickets", "ticket", "scope_page=feat&scope_tab=tickets"},
		{"repeated anchor", "tickets", "ticket", "scope_page=feat&scope_tab=tickets&anchor=FEAT-1&anchor=FEAT-2"},
		{"tab of another type", "features", "feature", pageScopeQuery("FEAT-1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, rec := listUngated(t, app, tc.plural, tc.typeName, tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

// Prev/next in a scoped tab walks the tab's rows.
func TestPageScope_Position(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	seedPageScopeGraph(app)

	scope, err := json.Marshal(ScopeDescriptor{
		Source: "list", Type: "ticket", Sort: "title", ScopePage: "feat", ScopeTab: "tickets", Anchor: "FEAT-2",
	})
	require.NoError(t, err)
	q := url.Values{"id": {"TKT-004"}, "scope": {string(scope)}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_position?"+q.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var pos v1.Position
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pos))
	require.Equal(t, 1, pos.Total)
	require.Nil(t, pos.Next)
}

// The search pipeline cannot narrow to a tab, so a search descriptor that
// names one is refused rather than walking the whole type.
func TestPageScope_PositionRefusesSearchSource(t *testing.T) {
	_, ok, reason := scopeFromParam(
		`{"source":"search","q":"x","scope_page":"feat","scope_tab":"tickets","anchor":"FEAT-1"}`,
		newTestAppV1(t).Meta())
	require.False(t, ok)
	require.Contains(t, reason, "source list")
}

func TestPageScope_SidebarWire(t *testing.T) {
	app := newTestAppV1(t)
	installEntityPage(app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"*"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	resp, _ := sidebarFor(gateCtxFor(principalCtx("alice"), t, d), t, app, "")
	require.Equal(t, v1.SidebarPage{Label: "Feature", EntityType: "feature", Badge: "title", Tabs: []v1.SidebarPageTab{{
		ID: "tickets", Label: "Tickets", View: "list", Target: "tickets",
		Scope: "relation", Relation: "implements", Direction: "incoming",
	}}}, resp.Pages["feat"])
}
