package dataentry

import (
	"context"
	"encoding/json"
	"maps"
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

// Entity dashboards: a dashboard with entity_type whose cards count only the
// rows the anchor reaches over each card's scope relation.

// implementsScope is the card scope from a feature to its tickets.
var implementsScope = &dataentryconfig.PageTabScope{
	Relation: "implements", Direction: dataentryconfig.DirectionIncoming,
}

// installEntityDashboard publishes a `feat` entity dashboard over features
// with the given cards, and a plain `plain` dashboard with one unscoped card.
func installEntityDashboard(app *App, cards ...dataentryconfig.DashboardCard) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = make(map[string]dataentryconfig.List, len(cur.Cfg.Lists)+1)
	maps.Copy(cfg.Lists, cur.Cfg.Lists)
	cfg.Lists["tickets"] = dataentryconfig.List{EntityType: "ticket"}
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{
		"feat":  {Title: "Feature", EntityType: "feature", Cards: cards},
		"plain": {Title: "Plain", Cards: []dataentryconfig.DashboardCard{{Title: "All", List: "tickets", Display: "count"}}},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// entityCardAs calls GET /_dashboard/card with the given query.
func entityCardAs(ctx context.Context, t *testing.T, app *App, q url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_dashboard/card?"+q.Encode(), http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1DashboardCard(app, rec, req)
	return rec
}

func entityCardValue(ctx context.Context, t *testing.T, app *App, card, anchor string) v1.DashboardCardValue {
	t.Helper()
	rec := entityCardAs(ctx, t, app, url.Values{"name": {"feat"}, "card": {card}, "anchor": {anchor}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store",
		"a card value is per principal, so no shared cache may hold it")
	var out v1.DashboardCardValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func TestEntityDashboard_CountsForTheAnchor(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedPageScopeGraph(app)
	installEntityDashboard(app,
		dataentryconfig.DashboardCard{Title: "Tickets", List: "tickets", Display: "count", Scope: implementsScope},
		dataentryconfig.DashboardCard{Title: "By status", List: "tickets", Display: "breakdown",
			GroupBy: "status", Scope: implementsScope},
		dataentryconfig.DashboardCard{Title: "Rows", List: "tickets", Display: "table", Limit: 2,
			Columns: []dataentryconfig.ListColumn{{Property: "title"}},
			Sort:    []dataentryconfig.SortSpec{{Property: "title", Direction: "desc"}}, Scope: implementsScope},
	)
	ctx := context.Background()

	require.Equal(t, 3, entityCardValue(ctx, t, app, "0", "FEAT-1").Total)
	require.Equal(t, 1, entityCardValue(ctx, t, app, "0", "FEAT-2").Total)

	groups := map[string]int{}
	for _, g := range entityCardValue(ctx, t, app, "1", "FEAT-1").Groups {
		groups[g.Value] = g.Count
	}
	require.Equal(t, map[string]int{"open": 2, "done": 1}, groups)

	table := entityCardValue(ctx, t, app, "2", "FEAT-1")
	require.Equal(t, 3, table.Total, "the total describes the narrowed set, before the limit")
	require.Len(t, table.Rows, 2)
	require.Equal(t, "TKT-003", table.Rows[0].ID, "rows come in the card's sort")
	require.Equal(t, "TKT-002", table.Rows[1].ID)

	table = entityCardValue(ctx, t, app, "2", "FEAT-2")
	require.Equal(t, 1, table.Total)
	require.Equal(t, "TKT-004", table.Rows[0].ID)
}

// A row the anchor reaches but the principal may not read is neither counted
// nor folded into a group: the gate runs before the narrowing and the fold.
func TestEntityDashboard_GatesBeforeFolding(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedPageScopeGraph(app)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-1", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-1"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-1"))
	seedRelation(app, entity.NewRelation("FEAT-1", "belongs-to", "PRJ-1"))
	setPoints(t, app, map[string]int{"TKT-001": 2, "TKT-002": 5, "TKT-003": 4})
	installEntityDashboard(app,
		dataentryconfig.DashboardCard{Title: "Tickets", List: "tickets", Display: "count", Scope: implementsScope},
		dataentryconfig.DashboardCard{Title: "By status", List: "tickets", Display: "breakdown",
			GroupBy: "status", Scope: implementsScope},
		dataentryconfig.DashboardCard{Title: "Rows", List: "tickets", Display: "table",
			Columns: []dataentryconfig.ListColumn{{Property: "title"}}, Scope: implementsScope},
		dataentryconfig.DashboardCard{Title: "Points", List: "tickets", Display: "sum",
			Property: "points", Scope: implementsScope},
	)
	d := mustNewACL(t, &acl.Policy{
		Roles:               map[string]acl.RoleDef{"editor": {Read: []string{"ticket", "feature"}}},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	setTestACL(t, app, d)
	ctx := gateCtxFor(aliceCtx(), t, d)

	require.Equal(t, 1, entityCardValue(ctx, t, app, "0", "FEAT-1").Total, "TKT-002 and TKT-003 are hidden")
	groups := map[string]int{}
	for _, g := range entityCardValue(ctx, t, app, "1", "FEAT-1").Groups {
		groups[g.Value] = g.Count
	}
	require.Equal(t, map[string]int{"open": 1}, groups, "the hidden done ticket folds into no group")
	table := entityCardValue(ctx, t, app, "2", "FEAT-1")
	require.Equal(t, 1, table.Total)
	require.Len(t, table.Rows, 1)
	sum := entityCardValue(ctx, t, app, "3", "FEAT-1")
	require.Equal(t, 1, sum.Total)
	require.InDelta(t, 2, sum.Value, 0, "the hidden tickets' points are not summed")
}

// setPoints gives each ticket its points.
func setPoints(t *testing.T, app *App, points map[string]int) {
	t.Helper()
	for id, n := range points {
		e, err := app.store.GetEntity(context.Background(), entity.Ref{ID: id})
		require.NoError(t, err)
		e.Properties["points"] = n
		require.NoError(t, app.store.UpdateEntity(context.Background(), e))
	}
}

// An outgoing scope counts the targets of the anchor's edges, and an anchor
// that reaches nothing counts zero rather than the whole list.
func TestEntityDashboard_OutgoingScopeAndEmptyReach(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedPageScopeGraph(app)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = map[string]dataentryconfig.List{"features": {EntityType: "feature"}}
	outgoing := &dataentryconfig.PageTabScope{Relation: "implements", Direction: dataentryconfig.DirectionOutgoing}
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{"feat": {
		Title: "Ticket", EntityType: "ticket", Cards: []dataentryconfig.DashboardCard{
			{Title: "Features", List: "features", Display: "count", Scope: outgoing},
			{Title: "Rows", List: "features", Display: "table",
				Columns: []dataentryconfig.ListColumn{{Property: "title"}}, Scope: outgoing},
		},
	}}
	next.Cfg = &cfg
	app.schema.Publish(&next)
	ctx := context.Background()

	require.Equal(t, 1, entityCardValue(ctx, t, app, "0", "TKT-001").Total)
	table := entityCardValue(ctx, t, app, "1", "TKT-004")
	require.Equal(t, 1, table.Total)
	require.Equal(t, "FEAT-2", table.Rows[0].ID)

	// TKT-005 implements nothing.
	require.Equal(t, 0, entityCardValue(ctx, t, app, "0", "TKT-005").Total)
	empty := entityCardValue(ctx, t, app, "1", "TKT-005")
	require.Equal(t, 0, empty.Total)
	require.Empty(t, empty.Rows)
}

// A hidden anchor, a missing one and one of another type answer the same
// 404, before any row is read.
func TestEntityDashboard_HiddenAnchorIsIndistinguishable(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedPageScopeGraph(app)
	installEntityDashboard(app,
		dataentryconfig.DashboardCard{Title: "Tickets", List: "tickets", Display: "count", Scope: implementsScope})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	ctx := gateCtxFor(aliceCtx(), t, d)

	bodies := map[string]map[string]any{}
	for _, anchor := range []string{"FEAT-1", "FEAT-NOPE", "TKT-001"} {
		rec := entityCardAs(ctx, t, app, url.Values{"name": {"feat"}, "card": {"0"}, "anchor": {anchor}})
		require.Equal(t, http.StatusNotFound, rec.Code, "anchor %s: %s", anchor, rec.Body)
		bodies[anchor] = withoutInstance(t, rec.Body.String())
	}
	require.Equal(t, bodies["FEAT-NOPE"], bodies["FEAT-1"], "hidden and missing anchors differ")
	require.Equal(t, bodies["FEAT-NOPE"], bodies["TKT-001"], "wrong-type and missing anchors differ")

	rec := getEntityAs(aliceCtx(), t, app, d, "feature", "features", "FEAT-NOPE", "")
	require.Equal(t, withoutInstance(t, rec.Body.String()), bodies["FEAT-NOPE"],
		"the card 404 differs from an entity GET 404")
}

func TestEntityDashboard_BadRequests(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedPageScopeGraph(app)
	installEntityDashboard(app,
		dataentryconfig.DashboardCard{Title: "Tickets", List: "tickets", Display: "count", Scope: implementsScope})

	cases := []struct {
		name string
		q    url.Values
		code string
	}{
		{"entity dashboard without anchor", url.Values{"name": {"feat"}, "card": {"0"}}, "anchor_required"},
		{"empty anchor", url.Values{"name": {"feat"}, "card": {"0"}, "anchor": {""}}, "anchor_required"},
		{"repeated anchor", url.Values{"name": {"feat"}, "card": {"0"}, "anchor": {"FEAT-1", "FEAT-2"}},
			"anchor_required"},
		{"anchor on a plain dashboard", url.Values{"name": {"plain"}, "card": {"0"}, "anchor": {"FEAT-1"}},
			"anchor_not_allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := entityCardAs(context.Background(), t, app, tc.q)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), tc.code)
		})
	}
}

// /_dashboard serves the dashboard's entity_type, so the SPA knows to pass
// the anchor.
func TestEntityDashboard_ServesEntityType(t *testing.T) {
	app := newTestAppV1(t)
	installEntityDashboard(app,
		dataentryconfig.DashboardCard{Title: "Tickets", List: "tickets", Display: "count", Scope: implementsScope})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_dashboard?name=feat", http.NoBody)
	rec := httptest.NewRecorder()
	app.views.handleV1Dashboard(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp v1.DashboardResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "feature", resp.EntityType)
	require.Equal(t, "implements", resp.Cards[0].Scope.Relation)
}
