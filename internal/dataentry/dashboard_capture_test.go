package dataentry

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Card figures for capture tasks (TKT-6OYQO79).

// boundCtxFor binds the ctx principal's Request as the scheduler's request
// edge does, and attaches no read gate: CardFigure attaches it itself.
func boundCtxFor(ctx context.Context, t *testing.T, d *acl.Declarative) context.Context {
	t.Helper()
	req, err := d.ForIdentity(acltest.IdentityFor(t.Context(), t, d, principal.From(ctx)))
	require.NoError(t, err)
	return acl.WithRequest(ctx, req)
}

func cardFigures(t *testing.T, app *App) CardFigureSource {
	t.Helper()
	src, err := CardFigures(app)
	require.NoError(t, err)
	return src
}

func TestCardFigures_RejectsNilApp(t *testing.T) {
	_, err := CardFigures(nil)
	require.Error(t, err)
}

func TestCardFigure_Displays(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	wireRealQueryScopes(t, app, map[string]string{"is-open": "entity.status == 'open'"})
	seedStatusTickets(app) // status open, open, done, unset; points 2, 2, 4, unset
	installDashboardCards(app,
		map[string]dataentryconfig.List{
			"open": {EntityType: "ticket", Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}}},
			"all":  {EntityType: "ticket"},
		},
		dataentryconfig.DashboardCard{Title: "Open", List: "open", Display: "count"},
		dataentryconfig.DashboardCard{Title: "All", List: "all", Display: "count"},
		dataentryconfig.DashboardCard{Title: "Open share", List: "all", Display: "ratio", Match: "is-open"},
		dataentryconfig.DashboardCard{Title: "Points", List: "all", Display: "sum", Property: "points"},
		dataentryconfig.DashboardCard{Title: "Mean", List: "all", Display: "avg", Property: "points"},
		dataentryconfig.DashboardCard{Title: "Least", List: "all", Display: "min", Property: "points"},
		dataentryconfig.DashboardCard{Title: "Most", List: "all", Display: "max", Property: "points"},
	)
	src := cardFigures(t, app)
	ctx := principalCtx("system:metrics")

	tests := []struct {
		card string
		want *big.Rat
	}{
		{"Open", big.NewRat(2, 1)},
		{"All", big.NewRat(4, 1)},
		{"Open share", big.NewRat(50, 1)},
		{"Points", big.NewRat(8, 1)},
		{"Mean", big.NewRat(8, 3)}, // exact, not the display's "2.67"
		{"Least", big.NewRat(2, 1)},
		{"Most", big.NewRat(4, 1)},
	}
	for _, tc := range tests {
		t.Run(tc.card, func(t *testing.T) {
			got, err := src.CardFigure(ctx, "", tc.card)
			require.NoError(t, err)
			require.Zero(t, tc.want.Cmp(got), "got %s, want %s", got.RatString(), tc.want.RatString())
		})
	}
	t.Run("default names the default dashboard", func(t *testing.T) {
		got, err := src.CardFigure(ctx, dataentryconfig.DefaultDashboardName, "All")
		require.NoError(t, err)
		require.Zero(t, big.NewRat(4, 1).Cmp(got))
	})
}

func TestCardFigure_Refusals(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedStatusTickets(app)
	installDashboardCards(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}, "done": {EntityType: "ticket",
			Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "nothing"}}}},
		dataentryconfig.DashboardCard{Title: "Twice", List: "all", Display: "count"},
		dataentryconfig.DashboardCard{Title: "Twice", List: "all", Display: "count"},
		dataentryconfig.DashboardCard{Title: "By status", List: "all", Display: "breakdown", GroupBy: "status"},
		dataentryconfig.DashboardCard{Title: "Search", Query: "type:ticket", Display: "count"},
		dataentryconfig.DashboardCard{Title: "Latest status", List: "all", Display: "max", Property: "status"},
		dataentryconfig.DashboardCard{Title: "Nothing", List: "done", Display: "ratio", Match: "all"},
		dataentryconfig.DashboardCard{Title: "No points", List: "done", Display: "sum", Property: "points"},
	)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{
		"per-project": {Title: "Project", EntityType: "project", Cards: []dataentryconfig.DashboardCard{
			{Title: "Tickets", List: "all", Display: "count"},
		}},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)

	src := cardFigures(t, app)
	ctx := principalCtx("system:metrics")
	tests := []struct {
		name, dashboard, card, want string
	}{
		{"unknown dashboard", "nope", "All", "no dashboard named"},
		{"entity dashboard", "per-project", "Tickets", "entity dashboard"},
		{"unknown card", "", "Missing", "no card titled"},
		{"ambiguous title", "", "Twice", "2 cards are titled"},
		{"no single figure", "", "By status", "has no single figure"},
		{"query card", "", "Search", "only a card with list:"},
		{"non-numeric max", "", "Latest status", "not a number"},
		{"ratio over no rows", "", "Nothing", "has no rows"},
		{"sum with no value", "", "No points", "no row has a value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := src.CardFigure(ctx, tc.dashboard, tc.card)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
			require.ErrorIs(t, err, errCardNotCapturable)
		})
	}
}

// The figure counts only what the bound principal reads: the gate is
// attached from the Request the scheduler bound, before anything folds.
func TestCardFigure_CountsOnlyReadableRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedGatedTickets(t, app) // alice reads 2 of 3 tickets
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "All", List: "all", Display: "count"})
	src := cardFigures(t, app)

	got, err := src.CardFigure(boundCtxFor(aliceCtx(), t, d), "", "All")
	require.NoError(t, err)
	require.Zero(t, big.NewRat(2, 1).Cmp(got), "got %s", got.RatString())

	_, err = src.CardFigure(aliceCtx(), "", "All")
	require.Error(t, err, "under a policy, a figure without a bound Request is refused")
}

// A fold over a property hidden on some row is refused rather than stored
// as a figure that covers fewer rows. A ratio grouped by a hidden property
// still counts every row, so it is not refused.
func TestCardFigure_HiddenValueRefusesFoldOnly(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	wireRealQueryScopes(t, app, map[string]string{"is-open": "entity.status == 'open'"})
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"points": false, "status": false}}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "A", "status": "open", "points": 3}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "Points", List: "all", Display: "sum", Property: "points"},
		dataentryconfig.DashboardCard{Title: "Share", List: "all", Display: "ratio", Match: "is-open", GroupBy: "status"})
	src := cardFigures(t, app)
	ctx := boundCtxFor(aliceCtx(), t, d)

	_, err := src.CardFigure(ctx, "", "Points")
	require.ErrorContains(t, err, "hidden on some rows")

	got, err := src.CardFigure(ctx, "", "Share")
	require.NoError(t, err)
	require.Zero(t, big.NewRat(100, 1).Cmp(got), "got %s", got.RatString())
}

// The fold refusal reads the verdicts of the Request the scheduler bound,
// through the real policy resolver: alice's role hides points, bob's shows
// them, and only bob gets a figure.
func TestCardFigure_HiddenValueFollowsTheBoundPrincipal(t *testing.T) {
	app, d := buildFieldPolicyApp(t, `
roles:
  limited:
    read: [ticket]
    visible:
      ticket:
        - field: title
  full:
    read: [ticket]
assignments:
  alice: limited
  bob: full
`)
	wireRealViewConditions(t, app)
	seedStatusTickets(app)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "Points", List: "all", Display: "sum", Property: "points"})
	src := cardFigures(t, app)

	_, err := src.CardFigure(boundCtxFor(aliceCtx(), t, d), "", "Points")
	require.ErrorContains(t, err, "hidden on some rows")

	got, err := src.CardFigure(boundCtxFor(bobCtx(), t, d), "", "Points")
	require.NoError(t, err)
	require.Zero(t, big.NewRat(8, 1).Cmp(got), "got %s", got.RatString())
}

func TestCardFigure_DecimalValues(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	for i, v := range []any{"1.5", "2.25", nil} {
		props := map[string]any{"title": "T"}
		if v != nil {
			props["points"] = v
		}
		seedEntity(app, &entity.Entity{ID: "TKT-00" + string(rune('1'+i)), Type: "ticket", Properties: props})
	}
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "Sum", List: "all", Display: "sum", Property: "points"},
		dataentryconfig.DashboardCard{Title: "Avg", List: "all", Display: "avg", Property: "points"},
		dataentryconfig.DashboardCard{Title: "Min", List: "all", Display: "min", Property: "points"})
	src := cardFigures(t, app)
	ctx := principalCtx("system:metrics")
	for card, want := range map[string]*big.Rat{
		"Sum": big.NewRat(15, 4), "Avg": big.NewRat(15, 8), "Min": big.NewRat(3, 2),
	} {
		got, err := src.CardFigure(ctx, "", card)
		require.NoError(t, err, card)
		require.Zero(t, want.Cmp(got), "%s: got %s, want %s", card, got.RatString(), want.RatString())
	}
}

// A hand-edited value that is not a number refuses the fold rather than
// being left out of it, and the error names neither the row nor the value.
func TestCardFigure_NonNumericValueRefusesFold(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "A", "points": 4}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "B", "points": "n/a"}})
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "Most", List: "all", Display: "max", Property: "points"})

	_, err := cardFigures(t, app).CardFigure(principalCtx("system:metrics"), "", "Most")
	require.ErrorIs(t, err, errCardNotCapturable)
	require.ErrorContains(t, err, "not a number")
	require.NotContains(t, err.Error(), "TKT-002")
	require.NotContains(t, err.Error(), "n/a")
}

type noWorlds struct{}

func (noWorlds) Lookup(string) (store.WorldScope, bool) { return store.WorldScope{}, false }

// A default world the lookup does not know is an error, not the trivial
// scope under which a faced type would look empty.
func TestCardFigure_UnknownDefaultWorld(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedStatusTickets(app)
	installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		dataentryconfig.DashboardCard{Title: "All", List: "all", Display: "count"})
	app.worlds = noWorlds{}

	_, err := cardFigures(t, app).CardFigure(principalCtx("system:metrics"), "", "All")
	require.ErrorContains(t, err, "is not known")
}

func TestCardFigure_NoDefaultDashboard(t *testing.T) {
	app := newTestAppV1(t)
	_, err := cardFigures(t, app).CardFigure(principalCtx("system:metrics"), "", "All")
	require.ErrorContains(t, err, "no dashboard: block")
}
