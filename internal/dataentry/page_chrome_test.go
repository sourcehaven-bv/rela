package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// Page chrome toggles (TKT-31822PN): the dashboard's validation card and the
// entity page's History and Add to pile buttons, resolved per principal.
// UX filters only, like a card's `permission:`.

// installPageChrome publishes a default dashboard with the given validation
// toggle and an header_buttons block.
func installPageChrome(app *App, validation dataentryconfig.ChromeToggle, header dataentryconfig.HeaderButtonsConfig) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboard = &dataentryconfig.DashboardConfig{Title: "Overview", Validation: validation}
	cfg.HeaderButtons = header
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

func dashboardValidation(ctx context.Context, t *testing.T, app *App) bool {
	t.Helper()
	var resp v1.DashboardResponse
	body := dashboardBody(ctx, t, app)
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode dashboard: %v\nbody: %s", err, body)
	}
	return resp.Validation
}

func sidebarHeaderButtons(ctx context.Context, t *testing.T, app *App) v1.SidebarHeaderButtons {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v", err)
	}
	return resp.HeaderButtons
}

func TestPageChrome_ResolvedPerPrincipal(t *testing.T) {
	gated := dataentryconfig.ChromeToggle{Permission: "admin:read"}
	off := dataentryconfig.ChromeToggle{Off: true}

	cases := []struct {
		name       string
		validation dataentryconfig.ChromeToggle
		header     dataentryconfig.HeaderButtonsConfig
		policy     bool   // a configured acl.yaml (gatedNavPolicy)
		principal  string // with policy: alice holds admin:read, bob does not
		want       v1.SidebarHeaderButtons
		wantValid  bool
	}{
		{name: "default shows everything", want: v1.SidebarHeaderButtons{History: true, AddToPile: true}, wantValid: true},
		{
			name:       "off hides from everyone",
			validation: off,
			header:     dataentryconfig.HeaderButtonsConfig{History: off, AddToPile: off},
			policy:     true, principal: "alice",
			want: v1.SidebarHeaderButtons{}, wantValid: false,
		},
		{
			name:       "holder sees gated",
			validation: gated,
			header:     dataentryconfig.HeaderButtonsConfig{History: gated, AddToPile: gated},
			policy:     true, principal: "alice",
			want: v1.SidebarHeaderButtons{History: true, AddToPile: true}, wantValid: true,
		},
		{
			name:       "non-holder does not",
			validation: gated,
			header:     dataentryconfig.HeaderButtonsConfig{History: gated},
			policy:     true, principal: "bob",
			want: v1.SidebarHeaderButtons{History: false, AddToPile: true}, wantValid: false,
		},
		{
			name:       "no acl.yaml shows gated",
			validation: gated,
			header:     dataentryconfig.HeaderButtonsConfig{History: gated, AddToPile: gated},
			want:       v1.SidebarHeaderButtons{History: true, AddToPile: true}, wantValid: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			installPageChrome(app, tc.validation, tc.header)
			ctx := t.Context()
			if tc.policy {
				d := mustNewACL(t, gatedNavPolicy(), app.store)
				setTestACL(t, app, d)
				ctx = gateCtxFor(principalCtx(tc.principal), t, d)
			} else {
				setTestACL(t, app, acl.NopACL{})
			}
			if got := sidebarHeaderButtons(ctx, t, app); got != tc.want {
				t.Errorf("header_buttons = %+v, want %+v", got, tc.want)
			}
			if got := dashboardValidation(ctx, t, app); got != tc.wantValid {
				t.Errorf("validation = %v, want %v", got, tc.wantValid)
			}
		})
	}
}

// Without a `dashboard:` block the validation card shows, as before.
func TestPageChrome_NoDashboardShowsValidation(t *testing.T) {
	app := newTestAppV1(t)
	installDashboardConfig(app, nil)
	if !dashboardValidation(t.Context(), t, app) {
		t.Error("validation must be true when no dashboard is configured")
	}
}

// The validation card covers the whole project, so an entity dashboard never
// shows it, whatever its toggle says.
func TestPageChrome_EntityDashboardHasNoValidation(t *testing.T) {
	app := newTestAppV1(t)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{
		"cat": {Title: "Category", EntityType: "ticket"},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_dashboard?name=cat", http.NoBody).WithContext(t.Context())
	rec := httptest.NewRecorder()
	app.views.handleV1Dashboard(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _dashboard: %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"validation":false`) {
		t.Errorf("entity dashboard must serve validation false, got %s", rec.Body)
	}
}

// A named dashboard resolves its own validation toggle per principal, so a
// portal dashboard can hide the card from customers and keep it for staff.
func TestPageChrome_NamedDashboardGatedPerPrincipal(t *testing.T) {
	app := newTestAppV1(t)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{
		"portal": {Title: "Portal", Validation: dataentryconfig.ChromeToggle{Permission: "admin:read"}},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	setTestACL(t, app, d)

	for principal, want := range map[string]bool{"alice": true, "bob": false} {
		t.Run(principal, func(t *testing.T) {
			ctx := gateCtxFor(principalCtx(principal), t, d)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_dashboard?name=portal", http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			app.views.handleV1Dashboard(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET _dashboard: %d %s", rec.Code, rec.Body)
			}
			var resp v1.DashboardResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode dashboard: %v", err)
			}
			if resp.Validation != want {
				t.Errorf("validation = %v, want %v", resp.Validation, want)
			}
		})
	}
}

func sidebarListButtons(ctx context.Context, t *testing.T, app *App) v1.SidebarListButtons {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v", err)
	}
	return resp.ListButtons
}

// `list_buttons:` resolves per principal like `header_buttons:`.
func TestListButtons_ResolvedPerPrincipal(t *testing.T) {
	gated := dataentryconfig.ChromeToggle{Permission: "admin:read"}
	off := dataentryconfig.ChromeToggle{Off: true}

	cases := []struct {
		name      string
		toggle    dataentryconfig.ChromeToggle
		policy    bool
		principal string
		want      bool
	}{
		{name: "default shows", want: true},
		{name: "off hides from everyone", toggle: off, policy: true, principal: "alice", want: false},
		{name: "holder sees gated", toggle: gated, policy: true, principal: "alice", want: true},
		{name: "non-holder does not", toggle: gated, policy: true, principal: "bob", want: false},
		{name: "no acl.yaml shows gated", toggle: gated, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			cur := app.State()
			next := *cur
			cfg := *cur.Cfg
			cfg.ListButtons = dataentryconfig.ListButtonsConfig{AddToPile: tc.toggle}
			next.Cfg = &cfg
			app.schema.Publish(&next)
			ctx := t.Context()
			if tc.policy {
				d := mustNewACL(t, gatedNavPolicy(), app.store)
				setTestACL(t, app, d)
				ctx = gateCtxFor(principalCtx(tc.principal), t, d)
			} else {
				setTestACL(t, app, acl.NopACL{})
			}
			if got := sidebarListButtons(ctx, t, app).AddToPile; got != tc.want {
				t.Errorf("list_buttons.add_to_pile = %v, want %v", got, tc.want)
			}
		})
	}
}
