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
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// sidebarShape performs a sidebar request and returns the groups as
// (groupLabel, itemLabels) pairs.
//
// It records every item's presence — the property every test here turns on —
// and surfaces group structure, needed to assert that an emptied group is
// dropped rather than rendered as a bare heading.
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

// TestNavPermission_ReadOnlyShowsEverything pins the read-only case, which is
// the one place the obvious answer is wrong in two directions at once.
//
// acl.ReadOnlyACL denies WRITES; it restricts no reads at all. Nav entries are
// overwhelmingly read surfaces, so hiding them would remove entries an
// observe-only principal can use — and, since ReadOnlyACL carries no identity,
// would hide them from EVERYONE rather than from non-holders. A `permission:`
// would then mean something different depending on a process-wide flag about
// writes. Losing the audit-log entry in post-incident forensic mode (a
// documented ReadOnlyACL use case) is the concrete cost.
//
// So read-only behaves like NopACL: no policy, no permission model, nothing
// hidden. Copying authorizeCommand's deny arm here would be wrong — commands
// shell out, which is write-shaped; a menu link is not.
func TestNavPermission_ReadOnlyShowsEverything(t *testing.T) {
	for _, tc := range []struct {
		name string
		impl acl.ACL
	}{
		// Both forms: AuthorizeWrite has a value receiver, so &ReadOnlyACL{}
		// also satisfies acl.ACL, and matching only the value form has
		// previously let a pointer slip into a default arm in this package.
		{"value form", acl.ReadOnlyACL{}},
		{"pointer form", &acl.ReadOnlyACL{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			installGatedNavConfig(app)
			app.acl = tc.impl

			labels := sidebarLabels(t.Context(), t, app)
			for _, want := range []string{"Open", "Audit", "Secrets", "Keys", "Public", "Private"} {
				if !slices.Contains(labels, want) {
					t.Errorf("--read-only restricts writes, not reads; %q must stay visible, got %v",
						want, labels)
				}
			}
		})
	}
}

// TestNavPermission_ReadOnlyArmIsExplicit is the RR-CWWJGW canary.
//
// Under ReadOnlyACL no middleware attaches a read gate, so readGateFromContext
// returns nopReadGate, whose HoldsPermission returns true unconditionally. A
// predicate that fell THROUGH to the read gate would therefore also show every
// gated entry — the same answer this feature wants, reached by accident.
//
// This test pins that the answer comes from the explicit arm instead: with a
// gate on the context that denies everything, read-only must STILL show the
// entries. If someone deletes the ReadOnlyACL arm, the *Declarative path is
// not taken (ReadOnlyACL is not *Declarative) and the default arm hides them —
// this fails. It is the difference between "correct" and "accidentally
// correct", which matters because this predicate is a copy target.
func TestNavPermission_ReadOnlyArmIsExplicit(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	app.acl = acl.ReadOnlyACL{}

	// A gate that denies every permission. If the arm were removed and the
	// predicate consulted the gate, this would hide the entries.
	ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{}})

	labels := sidebarLabels(ctx, t, app)
	if !slices.Contains(labels, "Audit") {
		t.Errorf("the ReadOnlyACL arm must decide before any read-gate call, got %v", labels)
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

// TestNavPermission_FilterIsPresentationOnly covers AC8, and is the assertion
// the whole "UX, not security" claim rests on. It has to fail if anyone ever
// wires the nav filter into enforcement, so it tests the two cases where the
// menu and the data could diverge — in BOTH directions:
//
//   - bob is denied the *permission* but permitted the *data*: the entry is
//     hidden and the list still returns its rows. Hiding gates nothing.
//   - carol holds the *permission* but is denied the *data*: the entry is
//     shown and the list comes back empty. Data-gating gates no menu.
//
// The rows assertion is the load-bearing half. Asserting only a 200 would pass
// vacuously — handleV1ListEntities has no knowledge of navigation config, so
// "the status is unchanged" is true by construction and survives any mutation
// of permitsNavEntry.
func TestNavPermission_FilterIsPresentationOnly(t *testing.T) {
	app := newTestAppV1(t)
	installGatedNavConfig(app)
	seedEntity(app, &entity.Entity{
		ID: "TKT-501", Type: "ticket", Properties: map[string]any{"title": "visible"},
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

	rowsFor := func(t *testing.T, ctx context.Context) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody).WithContext(ctx)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		if rec.Code != http.StatusOK {
			t.Fatalf("list: got %d, want 200; body=%s", rec.Code, rec.Body)
		}
		var resp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode list: %v\nbody: %s", err, rec.Body)
		}
		return len(resp.Data)
	}

	t.Run("entry hidden, data still readable", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("bob"), t, d)
		if slices.Contains(sidebarLabels(ctx, t, app), "Audit") {
			t.Fatalf("precondition: the Audit entry should be hidden for bob")
		}
		if n := rowsFor(t, ctx); n == 0 {
			t.Errorf("hiding a nav entry must not gate its target's data; got %d rows, want >0", n)
		}
	})

	t.Run("entry shown, data still gated", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("carol"), t, d)
		if !slices.Contains(sidebarLabels(ctx, t, app), "Audit") {
			t.Fatalf("precondition: carol holds admin:read, so the Audit entry should be shown")
		}
		if n := rowsFor(t, ctx); n != 0 {
			t.Errorf("showing a nav entry must not grant its target's data; got %d rows, want 0", n)
		}
	})
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
