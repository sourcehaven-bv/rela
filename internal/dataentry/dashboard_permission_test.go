package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// dashboardBody performs a /_dashboard request and returns the raw body, so
// tests can assert on the wire form (`[]` vs `null`) and not only on the
// decoded Go value — a nil slice and an empty slice are indistinguishable
// after unmarshalling, but only one of them is acceptable on the wire.
func dashboardBody(ctx context.Context, t *testing.T, app *App) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_dashboard", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Dashboard(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _dashboard: %d %s", rec.Code, rec.Body)
	}
	return rec.Body.String()
}

// dashboardTitles returns the titles of the cards visible to this principal.
func dashboardTitles(ctx context.Context, t *testing.T, app *App) []string {
	t.Helper()
	var resp v1.DashboardResponse
	body := dashboardBody(ctx, t, app)
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode dashboard: %v\nbody: %s", err, body)
	}
	titles := make([]string, 0, len(resp.Cards))
	for _, c := range resp.Cards {
		titles = append(titles, c.Title)
	}
	return titles
}

// installGatedDashboardConfig publishes a dashboard mixing gated and ungated
// cards. Card order is deliberate (ungated, gated, ungated) so a filter that
// reorders or off-by-ones is visible.
func installGatedDashboardConfig(app *App) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboard = &dataentryconfig.DashboardConfig{
		Title:       "Overview",
		Description: "Cards",
		Cards: []dataentryconfig.DashboardCard{
			{Title: "Open", Query: "type:ticket", Display: "count"},
			{Title: "Audit", Query: "type:ticket", Display: "count", Permission: "admin:read"},
			{Title: "Recent", Query: "type:ticket", Display: "count"},
		},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// installDashboardConfig publishes an arbitrary dashboard (or none, for nil).
func installDashboardConfig(app *App, dash *dataentryconfig.DashboardConfig) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboard = dash
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// TestDashboardPermission_UngatedCardsAlwaysShown covers AC1: a card with no
// `permission:` is shown to everyone, whatever the ACL.
func TestDashboardPermission_UngatedCardsAlwaysShown(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)

	cases := []struct {
		name string
		set  func()
		ctx  func() context.Context
	}{
		{"NopACL", func() { app.acl = acl.NopACL{} }, t.Context},
		{"holder", func() { app.acl = d }, func() context.Context { return gateCtxFor(aliceCtx(), t, d) }},
		{"non-holder", func() { app.acl = d }, func() context.Context {
			return gateCtxFor(principalCtx("bob"), t, d)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.set()
			titles := dashboardTitles(tc.ctx(), t, app)
			for _, want := range []string{"Open", "Recent"} {
				if !slices.Contains(titles, want) {
					t.Errorf("ungated card %q must always be shown, got %v", want, titles)
				}
			}
		})
	}
}

// TestDashboardPermission_HolderSeesGatedCard covers AC2 (positive half).
func TestDashboardPermission_HolderSeesGatedCard(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	titles := dashboardTitles(gateCtxFor(aliceCtx(), t, d), t, app)
	if !slices.Contains(titles, "Audit") {
		t.Errorf("alice holds admin:read and must see the gated card, got %v", titles)
	}
}

// TestDashboardPermission_NonHolderFiltered covers AC2 (negative half) and the
// order-preservation requirement: the surviving cards keep their configured
// order, because the SPA keys per-card data by index.
func TestDashboardPermission_NonHolderFiltered(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	titles := dashboardTitles(gateCtxFor(principalCtx("bob"), t, d), t, app)
	if slices.Contains(titles, "Audit") {
		t.Errorf("bob holds no admin:read and must not see the gated card, got %v", titles)
	}
	if want := []string{"Open", "Recent"}; !slices.Equal(titles, want) {
		t.Errorf("surviving cards must keep configured order: got %v, want %v", titles, want)
	}
}

// TestDashboardPermission_NopACLShowsEverything covers AC3 for the no-policy
// case: with no acl.yaml there is no permission model to consult, so a gated
// card is still shown.
func TestDashboardPermission_NopACLShowsEverything(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)

	for _, tc := range []struct {
		name string
		impl acl.ACL
	}{
		{"value form", acl.NopACL{}},
		{"pointer form", &acl.NopACL{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.acl = tc.impl
			if titles := dashboardTitles(t.Context(), t, app); !slices.Contains(titles, "Audit") {
				t.Errorf("under NopACL a gated card must be shown, got %v", titles)
			}
		})
	}
}

// TestDashboardPermission_ReadOnlyShowsEverything covers AC3 for --read-only.
//
// [acl.ReadOnlyACL] restricts WRITES only; it gates no reads and carries no
// identity. Denying here would hide cards from EVERYONE based on a process-wide
// flag about writes — the RR-XYO03L mistake, which this pins against for the
// dashboard surface as nav_permission_test.go does for the sidebar.
func TestDashboardPermission_ReadOnlyShowsEverything(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)

	for _, tc := range []struct {
		name string
		impl acl.ACL
	}{
		{"value form", acl.ReadOnlyACL{}},
		{"pointer form", &acl.ReadOnlyACL{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.acl = tc.impl
			if titles := dashboardTitles(t.Context(), t, app); !slices.Contains(titles, "Audit") {
				t.Errorf("under ReadOnlyACL a gated card must be shown, got %v", titles)
			}
		})
	}
}

// TestDashboardPermission_ReadOnlyArmIsExplicit is the RR-CWWJGW canary for
// this surface.
//
// readGateFromContext returns nopReadGate under ReadOnlyACL, and its
// HoldsPermission returns true unconditionally — so a predicate that fell
// THROUGH to the gate would show gated cards while looking like it had checked
// something. Attaching a gate that denies every permission distinguishes the
// two: if the explicit ReadOnlyACL arm is removed, ReadOnlyACL is not
// *Declarative, the default arm hides the card, and this fails.
func TestDashboardPermission_ReadOnlyArmIsExplicit(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	app.acl = acl.ReadOnlyACL{}

	ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{}})

	if titles := dashboardTitles(ctx, t, app); !slices.Contains(titles, "Audit") {
		t.Errorf("the ReadOnlyACL arm must decide before any read-gate call, got %v", titles)
	}
}

// TestDashboardPermission_NilACLHides covers AC4: a handler wired without an
// ACL closure fails closed for gated cards, and leaves ungated ones alone.
func TestDashboardPermission_NilACLHides(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	app.views.aclImpl = nil

	titles := dashboardTitles(t.Context(), t, app)
	if slices.Contains(titles, "Audit") {
		t.Errorf("a missing ACL wiring must fail closed for gated cards, got %v", titles)
	}
	if !slices.Contains(titles, "Open") {
		t.Errorf("ungated cards are unaffected by ACL wiring, got %v", titles)
	}
}

// unknownACL is an acl.ACL implementation the predicate was never taught
// about, pinning the closed switch (AC4).
type unknownACL struct{ acl.ACL }

// TestDashboardPermission_UnknownACLHides covers the closed-switch half of AC4:
// forgetting an arm must produce a missing card, never an unintended one.
func TestDashboardPermission_UnknownACLHides(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	app.acl = unknownACL{}

	titles := dashboardTitles(t.Context(), t, app)
	if slices.Contains(titles, "Audit") {
		t.Errorf("an unknown ACL implementation must hide gated cards, got %v", titles)
	}
}

// TestDashboardPermission_FilterIsPresentationOnly covers AC5, and is the
// assertion the whole "UX, not security" claim rests on for this surface.
//
// It tests both directions of divergence between the card list and the data:
//
//   - bob is denied the *permission* but permitted the *data*: the card is
//     hidden and the card's query still returns rows. Hiding gates nothing.
//   - carol holds the *permission* but is denied the *data*: the card is shown
//     and the same query comes back empty. Data-gating gates no card.
//
// The row counts are the load-bearing assertion. Asserting only a 200 would
// pass vacuously — handleV1Search has no knowledge of dashboard config, so
// "the status is unchanged" is true by construction and would survive any
// mutation of the filter.
func TestDashboardPermission_FilterIsPresentationOnly(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	seedEntity(app, &entity.Entity{
		ID: "TKT-601", Type: "ticket", Properties: map[string]any{"title": "visible"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			// bob: may read tickets, holds no permission.
			"viewer": {Read: []string{"ticket"}},
			// carol: holds the permission, may read NOTHING.
			"auditor": {Permissions: []string{"admin:read"}},
		},
		Assignments: map[string]string{"bob": "viewer", "carol": "auditor"},
	}, app.store)
	app.acl = d

	// The query the hidden card carries, run through the real search endpoint.
	rowsFor := func(t *testing.T, ctx context.Context) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=visible", http.NoBody).WithContext(ctx)
		rec := httptest.NewRecorder()
		app.handleV1Search(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("search: got %d, want 200; body=%s", rec.Code, rec.Body)
		}
		var resp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode search: %v\nbody: %s", err, rec.Body)
		}
		return len(resp.Data)
	}

	t.Run("card hidden, data still readable", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("bob"), t, d)
		if slices.Contains(dashboardTitles(ctx, t, app), "Audit") {
			t.Fatalf("precondition: the Audit card should be hidden for bob")
		}
		if n := rowsFor(t, ctx); n == 0 {
			t.Errorf("hiding a card must not gate its query's data; got %d rows, want >0", n)
		}
	})

	t.Run("card shown, data still gated", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("carol"), t, d)
		if !slices.Contains(dashboardTitles(ctx, t, app), "Audit") {
			t.Fatalf("precondition: carol holds admin:read, so the Audit card should be shown")
		}
		if n := rowsFor(t, ctx); n != 0 {
			t.Errorf("showing a card must not grant its query's data; got %d rows, want 0", n)
		}
	})
}

// TestDashboardPermission_ConfigUnfiltered covers AC6: /_config keeps serving
// the whole `dashboard:` block, `permission:` values included, to everyone.
//
// This is the invariant that made a separate endpoint necessary in the first
// place. Which cards are configured is not a secret — data-entry.yaml is an
// operator-authored file in the repo. Only /_dashboard is filtered.
func TestDashboardPermission_ConfigUnfiltered(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	bodyFor := func(t *testing.T, ctx context.Context) string {
		t.Helper()
		rec := httptest.NewRecorder()
		app.handleV1Config(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody).
			WithContext(ctx))
		if rec.Code != http.StatusOK {
			t.Fatalf("_config: got %d, want 200; body=%s", rec.Code, rec.Body)
		}
		return rec.Body.String()
	}

	alice := bodyFor(t, gateCtxFor(aliceCtx(), t, d))
	bob := bodyFor(t, gateCtxFor(principalCtx("bob"), t, d))
	if alice != bob {
		t.Errorf("_config must be principal-independent:\n alice=%s\n bob=%s", alice, bob)
	}
	if !strings.Contains(alice, "Audit") {
		t.Errorf("_config must still carry the gated card, got %s", alice)
	}
}

// TestDashboardPermission_EmptyCardsIsAlways200 covers AC7: every "nothing to
// show" case is one behavior — 200 with a JSON `[]`.
//
// Asserting on the raw body rather than the decoded value is deliberate: a nil
// slice unmarshals identically to an empty one, so only the wire form can
// catch a regression to `null`, which the SPA would have to special-case.
func TestDashboardPermission_EmptyCardsIsAlways200(t *testing.T) {
	cases := []struct {
		name string
		dash *dataentryconfig.DashboardConfig
	}{
		{"no dashboard configured", nil},
		{"empty cards", &dataentryconfig.DashboardConfig{Title: "Overview"}},
		{"every card filtered", &dataentryconfig.DashboardConfig{
			Title: "Overview",
			Cards: []dataentryconfig.DashboardCard{
				{Title: "Audit", Query: "type:ticket", Display: "count", Permission: "admin:read"},
			},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			installDashboardConfig(app, tc.dash)
			// One Declarative, used for BOTH the handler's ACL and the gate on
			// the context. Building the gate from a second policy would make
			// the "every card filtered" case pass by coincidence rather than
			// because this principal genuinely lacks the permission.
			d := mustNewACL(t, gatedNavPolicy(), app.store)
			app.acl = d

			body := dashboardBody(gateCtxFor(principalCtx("bob"), t, d), t, app)
			if !strings.Contains(body, `"cards":[]`) {
				t.Errorf("cards must serialize as [] (never null): got %s", body)
			}
		})
	}
}

// TestDashboardPermission_MethodNotAllowed pins the non-GET path, matching
// handleV1Sidebar.
func TestDashboardPermission_MethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)
	installGatedDashboardConfig(app)

	rec := httptest.NewRecorder()
	app.views.handleV1Dashboard(rec, httptest.NewRequest(http.MethodPost, "/api/v1/_dashboard", http.NoBody))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST _dashboard: got %d, want 405", rec.Code)
	}
}
