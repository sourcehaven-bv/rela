package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// sidebarShape performs a sidebar request and returns the groups as
// (groupLabel, itemLabels) pairs.
//
// A sibling of sidebarCountsByLabel, which records only items carrying a
// non-nil Count and so cannot express "this entry is absent" — the property
// every test here turns on. It also surfaces group structure, needed to assert
// that an emptied group is dropped rather than rendered as a bare heading.
func sidebarShape(ctx context.Context, t *testing.T, app *App) []struct {
	Group string
	Items []string
} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v\nbody: %s", err, rec.Body)
	}
	out := make([]struct {
		Group string
		Items []string
	}, 0, len(resp.Navigation))
	for _, g := range resp.Navigation {
		labels := make([]string, 0, len(g.Items))
		for _, it := range g.Items {
			labels = append(labels, it.Label)
		}
		out = append(out, struct {
			Group string
			Items []string
		}{Group: g.Group, Items: labels})
	}
	return out
}

// sidebarLabels flattens sidebarShape to the set of visible item labels.
func sidebarLabels(ctx context.Context, t *testing.T, app *App) []string {
	t.Helper()
	var labels []string
	for _, g := range sidebarShape(ctx, t, app) {
		labels = append(labels, g.Items...)
	}
	return labels
}

// installGatedNavConfig publishes a navigation tree mixing gated and ungated
// entries, including a group that is entirely gated (so the empty-group drop
// is observable) and one that is only partly gated.
func installGatedNavConfig(app *App) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = map[string]dataentryconfig.List{
		"all-tickets": {EntityType: "ticket", Title: "All"},
	}
	cfg.Navigation = []dataentryconfig.NavigationEntry{
		{Label: "Open", List: "all-tickets"},
		{Label: "Audit", List: "all-tickets", Permission: "admin:read"},
		{Group: "Admin", Items: []dataentryconfig.NavigationEntry{
			{Label: "Secrets", List: "all-tickets", Permission: "admin:read"},
			{Label: "Keys", List: "all-tickets", Permission: "admin:read"},
		}},
		{Group: "Mixed", Items: []dataentryconfig.NavigationEntry{
			{Label: "Public", List: "all-tickets"},
			{Label: "Private", List: "all-tickets", Permission: "admin:read"},
		}},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// gatedNavPolicy grants admin:read to alice only.
func gatedNavPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"admin":  {Read: []string{"ticket"}, Permissions: []string{"admin:read"}},
			"viewer": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "admin", "bob": "viewer"},
	}
}

// TestNavPermission_NopACLShowsEverything pins the no-policy case (AC2): with
// no acl.yaml, an entry carrying a `permission:` is still shown.
//
// No policy configured means no restrictions — the same posture nopReadGate
// takes for reads. This is what keeps a single-user instance's menu whole even
// if its config carries permissions.
func TestNavPermission_NopACLShowsEverything(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	app.acl = acl.NopACL{}

	labels := sidebarLabels(t.Context(), t, app)
	for _, want := range []string{"Open", "Audit", "Secrets", "Keys", "Public", "Private"} {
		if !slices.Contains(labels, want) {
			t.Errorf("under NopACL every entry must be shown; %q missing from %v", want, labels)
		}
	}
}

// TestNavPermission_HolderSeesEverything covers AC3: a principal holding the
// permission sees the gated entries.
func TestNavPermission_HolderSeesEverything(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	labels := sidebarLabels(gateCtxFor(aliceCtx(), t, d), t, app)
	for _, want := range []string{"Open", "Audit", "Secrets", "Keys", "Public", "Private"} {
		if !slices.Contains(labels, want) {
			t.Errorf("alice holds admin:read; %q missing from %v", want, labels)
		}
	}
}

// TestNavPermission_NonHolderFiltered covers AC4, AC6 and AC7 together, since
// they are three properties of one response: gated entries are absent, the
// fully-gated group is dropped entirely, and the partly-gated group survives
// with only its permitted item.
func TestNavPermission_NonHolderFiltered(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	shape := sidebarShape(gateCtxFor(principalCtx("bob"), t, d), t, app)

	var groups []string
	var labels []string
	for _, g := range shape {
		groups = append(groups, g.Group)
		labels = append(labels, g.Items...)
		if len(g.Items) == 0 {
			t.Errorf("group %q rendered with no items; emptied groups must be dropped", g.Group)
		}
	}

	// AC4: gated entries absent.
	for _, unwanted := range []string{"Audit", "Secrets", "Keys", "Private"} {
		if slices.Contains(labels, unwanted) {
			t.Errorf("bob lacks admin:read but %q is present in %v", unwanted, labels)
		}
	}
	// Ungated entries untouched.
	for _, want := range []string{"Open", "Public"} {
		if !slices.Contains(labels, want) {
			t.Errorf("ungated entry %q must stay visible, got %v", want, labels)
		}
	}
	// AC6: the wholly-gated group is gone.
	if slices.Contains(groups, "Admin") {
		t.Errorf("group %q had all items filtered and must be dropped, got groups %v", "Admin", groups)
	}
	// AC7: the partly-gated group survives with its permitted item.
	if !slices.Contains(groups, "Mixed") {
		t.Errorf("group %q keeps a permitted item and must survive, got groups %v", "Mixed", groups)
	}
}

// TestNavPermission_ReadOnlyHides covers AC5, and is the canary for the
// RR-CWWJGW hazard.
//
// readGateFromContext returns nopReadGate under ReadOnlyACL exactly as it does
// under NopACL, and nopReadGate.HoldsPermission returns true unconditionally.
// A predicate written against the read gate alone therefore SHOWS every gated
// entry under --read-only. permitsNavEntry has an explicit ReadOnlyACL arm
// ahead of the gate; this test fails if someone removes it.
func TestNavPermission_ReadOnlyHides(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	app.acl = acl.ReadOnlyACL{}

	labels := sidebarLabels(t.Context(), t, app)
	for _, unwanted := range []string{"Audit", "Secrets", "Keys", "Private"} {
		if slices.Contains(labels, unwanted) {
			t.Errorf("under --read-only a permission-gated entry must be hidden; %q present in %v",
				unwanted, labels)
		}
	}
	for _, want := range []string{"Open", "Public"} {
		if !slices.Contains(labels, want) {
			t.Errorf("ungated entry %q must stay visible under --read-only, got %v", want, labels)
		}
	}
}

// TestNavPermission_ReadOnlyHides_PointerForm pins the &-reachable variant of
// the same arm. acl.ReadOnlyACL's AuthorizeWrite has a value receiver, so
// &acl.ReadOnlyACL{} also satisfies acl.ACL; matching only the value form once
// let a pointer fall through to the default arm elsewhere in this package.
func TestNavPermission_ReadOnlyHides_PointerForm(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	app.acl = &acl.ReadOnlyACL{}

	labels := sidebarLabels(t.Context(), t, app)
	if slices.Contains(labels, "Audit") {
		t.Errorf("&acl.ReadOnlyACL{} must hide gated entries too, got %v", labels)
	}
}

// TestNavPermission_NilACLHides pins the wiring-bug case: a handler
// constructed without an ACL closure must hide gated entries, not show them.
func TestNavPermission_NilACLHides(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	app.views.aclImpl = nil

	labels := sidebarLabels(t.Context(), t, app)
	if slices.Contains(labels, "Audit") {
		t.Errorf("a missing ACL wiring must fail closed for gated entries, got %v", labels)
	}
	if !slices.Contains(labels, "Open") {
		t.Errorf("ungated entries are unaffected by ACL wiring, got %v", labels)
	}
}

// TestNavPermission_FilterIsPresentationOnly covers AC8: hiding an entry
// changes NO enforcement. The list behind a hidden entry answers a direct
// request exactly as it did before — for this principal, with its normal
// ACL-scoped rows.
//
// This is the assertion that keeps the feature honest. If someone ever makes
// the sidebar filter load-bearing, this test is what should stop them.
func TestNavPermission_FilterIsPresentationOnly(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	ctx := gateCtxFor(principalCtx("bob"), t, d)

	// The entry is hidden…
	if slices.Contains(sidebarLabels(ctx, t, app), "Audit") {
		t.Fatalf("precondition: the Audit entry should be hidden for bob")
	}

	// …and its target is untouched: same status as the ungated list that
	// points at the identical config, because the filter enforces nothing.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Errorf("hiding a nav entry must not change its target's behavior: got %d, want 200; body=%s",
			rec.Code, rec.Body)
	}
}

// TestNavPermission_ConfigUnfiltered covers AC9: /_config keeps serving the
// whole navigation tree to every principal.
//
// Deliberate. Which entries are configured is not a secret — data-entry.yaml
// is an operator-authored file in the repo (root CLAUDE.md, "The configuration
// is not a secret; the data is"). Only the *menu* is filtered, and only so a
// user isn't offered something they cannot act on.
func TestNavPermission_ConfigUnfiltered(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
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
}
