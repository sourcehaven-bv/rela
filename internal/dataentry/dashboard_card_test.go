package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Dashboard cards with a `list:` source (TKT-2SGPJZ).

// installDashboardCards publishes the default dashboard with cards and adds
// lists to the config.
func installDashboardCards(app *App, lists map[string]dataentryconfig.List, cards ...dataentryconfig.DashboardCard) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = make(map[string]dataentryconfig.List, len(cur.Cfg.Lists)+len(lists))
	maps.Copy(cfg.Lists, cur.Cfg.Lists)
	maps.Copy(cfg.Lists, lists)
	cfg.Dashboard = &dataentryconfig.DashboardConfig{Title: "Overview", Cards: cards}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// dashboardCardAs calls GET /_dashboard/card for the default dashboard.
func dashboardCardAs(ctx context.Context, t *testing.T, app *App, index string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_dashboard/card?card="+index, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1DashboardCard(app, rec, req)
	return rec
}

// dashboardCardValueAs decodes a 200 card value.
func dashboardCardValueAs(
	ctx context.Context, t *testing.T, app *App, index int,
) (value v1.DashboardCardValue, body string) {
	t.Helper()
	rec := dashboardCardAs(ctx, t, app, strconv.Itoa(index))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store",
		"a card value is per principal, so no shared cache may hold it")
	var out v1.DashboardCardValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out, rec.Body.String()
}

// wireRealQueryScopes declares scopes on the ticket type and wires the
// production scope resolver, as cmd/rela-server does.
func wireRealQueryScopes(t *testing.T, app *App, scopes map[string]string) {
	t.Helper()
	meta := app.Meta()
	def := meta.Entities["ticket"]
	def.QueryScopes = scopes
	meta.Entities["ticket"] = def
	require.NoError(t, app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)))
}

func seedStatusTickets(app *App) {
	points := []int{2, 2, 4, 0} // 0: unset
	for i, status := range []string{"open", "open", "done", ""} {
		props := map[string]any{"title": "T" + strconv.Itoa(i)}
		if status != "" {
			props["status"] = status
		}
		if points[i] != 0 {
			props["points"] = points[i]
		}
		seedEntity(app, &entity.Entity{ID: "TKT-00" + strconv.Itoa(i+1), Type: "ticket", Properties: props})
	}
}

func TestDashboardCard_Displays(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedStatusTickets(app)
	installDashboardCards(app,
		map[string]dataentryconfig.List{
			"open": {EntityType: "ticket", Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}}},
			"all":  {EntityType: "ticket"},
		},
		dataentryconfig.DashboardCard{Title: "Open", List: "open", Display: "count"},
		dataentryconfig.DashboardCard{Title: "By status", List: "all", Display: "breakdown", GroupBy: "status"},
		dataentryconfig.DashboardCard{Title: "Latest", List: "all", Display: "table", Limit: 2,
			Columns: []dataentryconfig.ListColumn{{Property: "title"}},
			Sort:    []dataentryconfig.SortSpec{{Property: "title", Direction: "desc"}}},
		dataentryconfig.DashboardCard{Title: "Grid", List: "all", Display: "matrix", Rows: "points", MatrixColumns: "points",
			RowsLabel: "Effort", RowsOrder: dataentryconfig.MatrixOrderDesc},
		dataentryconfig.DashboardCard{Title: "Query", Query: "type:ticket", Display: "count"},
	)
	ctx := principalCtx("alice")

	t.Run("count is the list's filtered total", func(t *testing.T) {
		got, _ := dashboardCardValueAs(ctx, t, app, 0)
		require.Equal(t, "Open", got.Title)
		require.Equal(t, "open", got.List)
		require.Equal(t, 2, got.Total)
	})
	t.Run("breakdown counts each value, unset last", func(t *testing.T) {
		got, _ := dashboardCardValueAs(ctx, t, app, 1)
		require.Equal(t, 4, got.Total)
		require.Equal(t, []v1.DashboardGroup{
			{Value: "open", Label: "open", Count: 2},
			{Value: "done", Label: "done", Count: 1},
			{Value: "", Label: "", Count: 1},
		}, got.Groups)
		require.False(t, got.Partial)
	})
	t.Run("table is sorted by the card and limited", func(t *testing.T) {
		got, _ := dashboardCardValueAs(ctx, t, app, 2)
		require.Equal(t, 4, got.Total)
		require.Len(t, got.Rows, 2)
		require.Equal(t, "T3", got.Rows[0].Properties["title"])
		require.Equal(t, "T2", got.Rows[1].Properties["title"])
	})
	t.Run("matrix places set values and counts the rest as unplaced", func(t *testing.T) {
		got, _ := dashboardCardValueAs(ctx, t, app, 3)
		require.NotNil(t, got.Matrix)
		require.Equal(t, 1, got.Matrix.Unplaced)
		require.Equal(t, []v1.DashboardCell{
			{Row: "4", Column: "4", Count: 1},
			{Row: "2", Column: "2", Count: 2},
		}, got.Matrix.Cells)
	})
	t.Run("matrix shows the declared range, titled and ordered", func(t *testing.T) {
		got, _ := dashboardCardValueAs(ctx, t, app, 3)
		axis := func(values []v1.DashboardAxisValue) (out []string) {
			for _, v := range values {
				out = append(out, v.Value)
			}
			return out
		}
		require.Equal(t, []string{"5", "4", "3", "2", "1"}, axis(got.Matrix.Rows))
		require.Equal(t, []string{"1", "2", "3", "4", "5"}, axis(got.Matrix.Columns))
		require.Equal(t, "Effort", got.Matrix.RowTitle)
		require.Equal(t, "points", got.Matrix.ColumnTitle)
	})
	t.Run("a query card is not computed", func(t *testing.T) {
		rec := dashboardCardAs(ctx, t, app, "4")
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "card_not_computed")
	})
	for _, bad := range []string{"5", "-1", "x", ""} {
		t.Run("unknown index "+bad, func(t *testing.T) {
			rec := dashboardCardAs(ctx, t, app, bad)
			require.Equal(t, http.StatusNotFound, rec.Code)
			require.Contains(t, rec.Body.String(), "card_not_found")
		})
	}
}

// A card hidden by permission: answers like an unknown index. The list it
// reads still answers the same rows on its own page: the filter is
// presentation only.
func TestDashboardCard_PermissionHiddenCardIsNotFound(t *testing.T) {
	app := newTestAppV1(t)
	seedStatusTickets(app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "Gated", List: "all", Display: "count", Permission: "admin:read"})

	rec := dashboardCardAs(gateCtxFor(aliceCtx(), t, d), t, app, "0")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "card_not_found")
}

// A row the principal may not read is not counted, and a card's count equals
// the list's own total for the same principal.
func TestDashboardCard_CountsOnlyReadableRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedGatedTickets(t, app)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "All", List: "all", Display: "count"},
		dataentryconfig.DashboardCard{Title: "Rows", List: "all", Display: "table",
			Columns: []dataentryconfig.ListColumn{{Property: "title"}}})
	ctx := gateCtxFor(aliceCtx(), t, d)

	got, _ := dashboardCardValueAs(ctx, t, app, 0)
	require.Equal(t, 2, got.Total)

	table, body := dashboardCardValueAs(ctx, t, app, 1)
	require.Equal(t, 2, table.Total)
	require.Len(t, table.Rows, 2)
	require.NotContains(t, body, "SECRET-ROW")
}

// A value hidden by visible: is left out of every group and marks the card
// partial; the row still counts.
func TestDashboardCard_HiddenValueIsLeftOutAndPartial(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"status": false}}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "A", "status": "SECRET-STATUS"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "By status", List: "all", Display: "breakdown", GroupBy: "status"},
		dataentryconfig.DashboardCard{Title: "Grid", List: "all", Display: "matrix", Rows: "status", MatrixColumns: "title"},
		dataentryconfig.DashboardCard{Title: "Rows", List: "all", Display: "table",
			Columns: []dataentryconfig.ListColumn{{Property: "title"}, {Property: "status"}}},
	)
	ctx := gateCtxFor(aliceCtx(), t, d)

	for i := range 3 {
		got, body := dashboardCardValueAs(ctx, t, app, i)
		require.Equal(t, 1, got.Total, "card %d still counts the row", i)
		require.True(t, got.Partial, "card %d", i)
		require.NotContains(t, body, "SECRET", "card %d", i)
	}
	breakdown, _ := dashboardCardValueAs(ctx, t, app, 0)
	require.Empty(t, breakdown.Groups)
	table, _ := dashboardCardValueAs(ctx, t, app, 2)
	require.Equal(t, []string{"status"}, table.Rows[0].Redacted)
}

// A ratio counts the rows its match: scope admits, overall and per group.
func TestDashboardCard_Ratio(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	wireRealQueryScopes(t, app, map[string]string{"is-open": "entity.status == 'open'"})
	seedStatusTickets(app)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "Open share", List: "all", Display: "ratio", Match: "is-open", GroupBy: "status"})

	got, _ := dashboardCardValueAs(principalCtx("alice"), t, app, 0)
	require.Equal(t, 4, got.Total)
	require.Equal(t, 2, got.Matched)
	require.Equal(t, []v1.DashboardGroup{
		{Value: "open", Label: "open", Count: 2, Matched: 2},
		{Value: "done", Label: "done", Count: 1},
		{Value: "", Label: "", Count: 1},
	}, got.Groups)
}

// /_dashboard serves each card's config index, skipping hidden cards
// without renumbering the rest.
func TestDashboard_ServesConfigIndex(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)
	rec := namedDashboard(principalCtx("alice"), t, app, "name=crm")
	resp := decodeDashboard(t, rec)
	require.Len(t, resp.Cards, 3)
	require.Equal(t, []int{0, 1, 2}, []int{resp.Cards[0].Index, resp.Cards[1].Index, resp.Cards[2].Index})
}

func TestFoldMatrix_SeverityColorAndHiddenTone(t *testing.T) {
	meta := &metamodel.Metamodel{Types: map[string]metamodel.CustomType{
		"level": {Values: []string{"low", "medium", "high"}},
	}}
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"impact": {Type: "level"}, "likelihood": {Type: "integer"}, "severity": {Type: "level"},
	}}
	row := func(id string, impact string, likelihood int, severity string) *entity.Entity {
		props := map[string]any{"impact": impact, "likelihood": likelihood}
		if severity != "" {
			props["severity"] = severity
		}
		return &entity.Entity{ID: id, Type: "risk", Properties: props}
	}
	hidden := row("R5", "high", 2, "")
	hidden.Redacted = []string{"severity"}
	rows := []*entity.Entity{
		row("R1", "low", 3, "low"),
		row("R2", "low", 3, "high"),
		row("R3", "medium", 1, "medium"),
		row("R4", "high", 2, "low"),
		hidden,
	}
	tone := newAxisProperty(meta, def, "severity")
	in := matrixInput{
		rows:     newAxisProperty(meta, def, "impact"),
		columns:  newAxisProperty(meta, def, "likelihood"),
		toneBy:   &tone,
		colorFor: func(v string) string { return "c-" + v },
	}
	m, partial := foldMatrix(rows, in)
	require.True(t, partial, "a hidden tone_by value marks the card partial")
	require.Equal(t, []v1.DashboardAxisValue{{Value: "low", Label: "low"}, {Value: "medium", Label: "medium"}, {Value: "high", Label: "high"}}, m.Rows)
	require.Equal(t, []v1.DashboardAxisValue{{Value: "1", Label: "1"}, {Value: "2", Label: "2"}, {Value: "3", Label: "3"}}, m.Columns)
	require.Equal(t, []v1.DashboardCell{
		{Row: "low", Column: "3", Count: 2, Color: "c-high"},
		{Row: "medium", Column: "1", Count: 1, Color: "c-medium"},
		{Row: "high", Column: "2", Count: 2, Color: "c-low"},
	}, m.Cells, "the most severe tone present colors a cell; the hidden one counts but does not color")
}

// A hand-edited value outside the enum has no row or column, so the row is
// reported as unplaced rather than silently dropped.
func TestFoldMatrix_UndeclaredEnumValueIsUnplaced(t *testing.T) {
	meta := &metamodel.Metamodel{Types: map[string]metamodel.CustomType{
		"level": {Values: []string{"low", "high"}},
	}}
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"impact": {Type: "level"}, "likelihood": {Type: "integer"},
	}}
	rows := []*entity.Entity{
		{ID: "R1", Properties: map[string]any{"impact": "low", "likelihood": 1}},
		{ID: "R2", Properties: map[string]any{"impact": "extreme", "likelihood": 1}},
	}
	m, partial := foldMatrix(rows, matrixInput{
		rows:    newAxisProperty(meta, def, "impact"),
		columns: newAxisProperty(meta, def, "likelihood"),
	})
	require.False(t, partial)
	require.Equal(t, 1, m.Unplaced)
	require.Equal(t, []v1.DashboardCell{{Row: "low", Column: "1", Count: 1}}, m.Cells)
}

// Integer matrix axes (TKT-BRAM1A): a declared range is the whole axis, an
// undeclared one fills from the lowest to the highest value, and a value the
// axis has no place for is unplaced.
func TestFoldMatrix_IntegerAxisRange(t *testing.T) {
	meta := &metamodel.Metamodel{Types: map[string]metamodel.CustomType{
		"level": {Values: []string{"low", "high"}},
	}}
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"impact":  {Type: "integer", Label: "Impact", Min: new(1), Max: new(5)},
		"floor":   {Type: "integer", Min: new(1)},
		"ceiling": {Type: "integer", Max: new(0)},
		"count":   {Type: "integer"},
		"level":   {Type: "level"},
	}}
	tests := []struct {
		name         string
		prop         string
		order        string
		values       []any // one row per value, all in column "low"
		wantRows     []string
		wantUnplaced int
	}{
		{"declared range shows every value", "impact", "", []any{2, 4}, []string{"1", "2", "3", "4", "5"}, 0},
		{"declared range with no rows", "impact", "", nil, []string{"1", "2", "3", "4", "5"}, 0},
		{"out of range is unplaced", "impact", "", []any{3, 9, 0}, []string{"1", "2", "3", "4", "5"}, 2},
		{"desc reverses", "impact", "desc", []any{3}, []string{"5", "4", "3", "2", "1"}, 0},
		{"undeclared fills the gap", "count", "", []any{1, 4}, []string{"1", "2", "3", "4"}, 0},
		{"undeclared with no rows is empty", "count", "", nil, nil, 0},
		{"one value read three ways is one row", "count", "", []any{3, 3.0, "03"}, []string{"3"}, 0},
		{"not a whole number is unplaced", "count", "", []any{2, 2.5, "x"}, []string{"2"}, 2},
		{"min only extends to the highest", "floor", "", []any{0, 3}, []string{"1", "2", "3"}, 1},
		{"min only with no rows is the bound", "floor", "", nil, []string{"1"}, 0},
		{"max only starts at the lowest", "ceiling", "", []any{-2, 1}, []string{"-2", "-1", "0"}, 1},
		{"enum desc reverses declared order", "level", "desc", []any{"low"}, []string{"high", "low"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rows []*entity.Entity
			for i, v := range tc.values {
				rows = append(rows, &entity.Entity{
					ID: "R" + strconv.Itoa(i), Properties: map[string]any{tc.prop: v, "level": "low"},
				})
			}
			m, partial := foldMatrix(rows, matrixInput{
				rows:    newMatrixAxis(meta, def, tc.prop, "", tc.order),
				columns: newAxisProperty(meta, def, "level"),
			})
			require.False(t, partial)
			var got []string
			for _, r := range m.Rows {
				got = append(got, r.Value)
			}
			require.Equal(t, tc.wantRows, got)
			require.Equal(t, tc.wantUnplaced, m.Unplaced)
			require.False(t, m.Truncated)
		})
	}
}

// An observed spread wider than the cap is cut from the low end and
// flagged. It never builds an axis as wide as the values are far apart.
func TestFoldMatrix_IntegerAxisIsCapped(t *testing.T) {
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"n": {Type: "integer"}, "m": {Type: "integer"},
	}}
	meta := &metamodel.Metamodel{}
	fold := func(t *testing.T, order string, ns ...int64) v1.DashboardMatrix {
		t.Helper()
		rows := make([]*entity.Entity, len(ns))
		for i, n := range ns {
			rows[i] = &entity.Entity{ID: fmt.Sprintf("R%d", i), Properties: map[string]any{"n": n, "m": 1}}
		}
		m, _ := foldMatrix(rows, matrixInput{
			rows: newMatrixAxis(meta, def, "n", "", order), columns: newAxisProperty(meta, def, "m"),
		})
		return m
	}
	values := func(m v1.DashboardMatrix) []string {
		out := make([]string, len(m.Rows))
		for i, r := range m.Rows {
			out[i] = r.Value
		}
		return out
	}

	// One outlier must not push the real values off the axis: a span wider
	// than the cap lists only the values that occur.
	t.Run("outlier lists the values that occur", func(t *testing.T) {
		m := fold(t, "", 1, 2, 3, 5, 100, math.MinInt64, math.MaxInt64)
		require.False(t, m.Truncated)
		require.Equal(t, []string{
			strconv.FormatInt(math.MinInt64, 10), "1", "2", "3", "5", "100", strconv.FormatInt(math.MaxInt64, 10),
		}, values(m))
		require.Zero(t, m.Unplaced)
	})

	many := make([]int64, 0, 2*dataentryconfig.MaxMatrixAxisValues)
	for i := range 2 * dataentryconfig.MaxMatrixAxisValues {
		many = append(many, int64(i*10))
	}
	t.Run("more distinct values than the cap keeps the first shown", func(t *testing.T) {
		m := fold(t, "", many...)
		require.True(t, m.Truncated)
		require.Len(t, m.Rows, dataentryconfig.MaxMatrixAxisValues)
		require.Equal(t, "0", m.Rows[0].Value)
	})
	t.Run("desc keeps the highest", func(t *testing.T) {
		m := fold(t, dataentryconfig.MatrixOrderDesc, many...)
		require.True(t, m.Truncated)
		require.Len(t, m.Rows, dataentryconfig.MaxMatrixAxisValues)
		require.Equal(t, strconv.FormatInt(many[len(many)-1], 10), m.Rows[0].Value)
	})
}

// A declared range is the whole axis even when every row's value is hidden;
// the card is then partial and has no cells.
func TestFoldMatrix_DeclaredRangeWithEveryValueHidden(t *testing.T) {
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"n": {Type: "integer", Min: new(1), Max: new(3)}, "m": {Type: "integer"},
	}}
	rows := []*entity.Entity{{ID: "R1", Properties: map[string]any{"m": 1}, Redacted: []string{"n"}}}
	meta := &metamodel.Metamodel{}
	m, partial := foldMatrix(rows, matrixInput{
		rows: newAxisProperty(meta, def, "n"), columns: newAxisProperty(meta, def, "m"),
	})
	require.True(t, partial)
	require.Len(t, m.Rows, 3)
	require.Empty(t, m.Columns)
	require.Empty(t, m.Cells)
}

// A hidden axis value marks the card partial and does not widen the axis,
// whose extent would otherwise disclose it.
func TestFoldMatrix_HiddenValueDoesNotWidenAxis(t *testing.T) {
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"n": {Type: "integer"}, "m": {Type: "integer"},
	}}
	hidden := &entity.Entity{ID: "R2", Properties: map[string]any{"m": 1}, Redacted: []string{"n"}}
	rows := []*entity.Entity{{ID: "R1", Properties: map[string]any{"n": 2, "m": 1}}, hidden}
	meta := &metamodel.Metamodel{}
	m, partial := foldMatrix(rows, matrixInput{
		rows: newAxisProperty(meta, def, "n"), columns: newAxisProperty(meta, def, "m"),
	})
	require.True(t, partial)
	require.Equal(t, []v1.DashboardAxisValue{{Value: "2", Label: "2"}}, m.Rows)
}

// Axis titles default to the property label, then the name; the card's
// rows_label and columns_label override them.
func TestFoldMatrix_AxisTitles(t *testing.T) {
	def := &metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"impact": {Type: "integer", Label: "Impact"}, "kans": {Type: "integer"},
	}}
	meta := &metamodel.Metamodel{}
	m, _ := foldMatrix(nil, matrixInput{
		rows: newMatrixAxis(meta, def, "impact", "", ""), columns: newMatrixAxis(meta, def, "kans", "", ""),
	})
	require.Equal(t, "Impact", m.RowTitle)
	require.Equal(t, "kans", m.ColumnTitle)
	m, _ = foldMatrix(nil, matrixInput{
		rows:    newMatrixAxis(meta, def, "impact", "Gevolg", ""),
		columns: newMatrixAxis(meta, def, "kans", "Kans", ""),
	})
	require.Equal(t, "Gevolg", m.RowTitle)
	require.Equal(t, "Kans", m.ColumnTitle)
}

func TestMatrixColorFor(t *testing.T) {
	cfg := &dataentryconfig.Config{}
	card := dataentryconfig.DashboardCard{Colors: map[string]string{"high": "error", "medium": "orange/300"}}
	color := matrixColorFor(card, cfg, map[string]string{"low": "badge-green", "very_low": "badge-teal"})
	require.Equal(t, "badge-red", color("high"))
	require.Equal(t, "kit-orange-300", color("medium"))
	require.Equal(t, "badge-green", color("low"), "falls back to the app style")
	require.Equal(t, "badge-teal", color("Very Low"), "matches the SPA's normalized style key")
	require.Empty(t, color("other"))
}

// Each display reads the card's list once, then folds in memory: the read
// count is the same at 10 and 50 rows. The ratio's match scope adds one batch
// (feature-one walks a relation), and the table is one bounded page.
func TestQueryBudget_DashboardCardsAreSizeIndependent(t *testing.T) {
	cards := []dataentryconfig.DashboardCard{
		{Title: "Count", List: "tickets", Display: "count"},
		{Title: "Breakdown", List: "tickets", Display: "breakdown", GroupBy: "status"},
		{Title: "Matrix", List: "tickets", Display: "matrix", Rows: "status", MatrixColumns: "status"},
		// tone_by runs a third value read per row and the color resolver.
		{Title: "Toned", List: "tickets", Display: "matrix", Rows: "status", MatrixColumns: "status", ToneBy: "status"},
		{Title: "Ratio", List: "tickets", Display: "ratio", Match: "feature-one", GroupBy: "status"},
		{Title: "Table", List: "tickets", Display: "table", Limit: 5,
			Columns: []dataentryconfig.ListColumn{{Property: "title"}}},
	}
	for i, card := range cards {
		t.Run(card.Title, func(t *testing.T) {
			small, large, detail := readsFor(t, func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
				t.Helper()
				installDashboardCards(app, nil, cards...)
				require.NoError(t, app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)))
				got, _ := dashboardCardValueAs(gateCtxFor(ctx, t, d), t, app, i)
				if got.Total < 10 {
					t.Fatalf("card counted %d rows", got.Total)
				}
				if card.Display == "ratio" && (got.Matched == 0 || got.Matched == got.Total) {
					t.Fatalf("ratio matched %d of %d; the scope did not run", got.Matched, got.Total)
				}
			})
			assertBudget(t, "dashboard "+card.Title, small, large, dashboardCardBudgets[card.Title], detail)
		})
	}
}

// dashboardCardBudgets are the measured reads per display. Raise one only
// with a reason in the commit.
//
// A folded card: the type's headers and the membership walk. A ratio adds
// ONE MatchingFaces batch for its related() scope. A table: the scoped
// count, one bounded page and the membership walk, as a nav items group.
var dashboardCardBudgets = map[string]int{
	"Count": 3, "Breakdown": 3, "Matrix": 3, "Toned": 3, "Ratio": 4, "Table": 4,
}
