package dataentry

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// withListRenderOverride configures a per-LIST export_render override on the
// fixture's "tickets" list and swaps app.documents for a fake-script-engine
// documentService, so the override tests exercise the render path without a
// real Lua runtime. Mirrors withRenderOverride (the per-type equivalent),
// including the step that is easy to forget: rebuilding app.export so the
// handler picks up both the republished config and the swapped document
// service.
func withListRenderOverride(
	t *testing.T, app *App, output func(c fakeScriptCall) string,
) *fakeScriptEngine {
	t.Helper()
	const listID = "tickets"
	s := app.State()
	cfg := *s.Cfg
	lists := make(map[string]dataentryconfig.List, len(cfg.Lists))
	maps.Copy(lists, cfg.Lists)
	l := lists[listID]
	l.ExportRender = "docs/fancy_list.lua"
	lists[listID] = l
	cfg.Lists = lists
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})

	fake := &fakeScriptEngine{stdout: output}
	deps := func() lua.WriteDeps { return lua.WriteDeps{} }
	app.documents = newDocumentService(app.store, app.kv, "/", fake, deps)
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	return fake
}

// exportListURL drives a list export against an arbitrary query string, so
// tests can exercise the no-`list=` fallback and filter/sort context.
func exportListURL(ctx context.Context, app *App, rawQuery string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/_export?"+rawQuery, http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.export.handleV1ExportList(rec, req, "ticket")
	return rec
}

// adminListCtx seeds an admin ACL that can read everything and returns a
// gated context for alice.
func adminListCtx(t *testing.T, app *App) context.Context {
	t.Helper()
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	return gateCtxFor(aliceCtx(), t, d)
}

func seedExportTickets(app *App, ids ...string) {
	for _, id := range ids {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": "Title " + id, "status": "open"}})
	}
}

// TestExport_List_RenderOverride is the core contract: the configured script
// replaces the built-in column table entirely.
func TestExport_List_RenderOverride(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1", "TKT-2")
	fake := withListRenderOverride(t, app, func(c fakeScriptCall) string {
		return "# Fancy " + c.listID + " (" + strings.Join(c.rowIDs, ",") + ")\n"
	})
	ctx := adminListCtx(t, app)

	rec := exportListURL(ctx, app, "transform=copy&list=tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "# Fancy tickets (TKT-1,TKT-2)") {
		t.Errorf("override output missing:\n%s", body)
	}
	// The negative half: the built-in table must be fully replaced, not
	// prepended or appended to.
	if strings.Contains(body, "| Title | Status |") {
		t.Errorf("built-in column table leaked into an overridden export:\n%s", body)
	}
	if fake.callCount() != 1 {
		t.Errorf("want exactly 1 script call, got %d", fake.callCount())
	}
}

// TestExport_List_NoOverrideUsesBuiltin guards the regression direction: a
// list without export_render keeps the column table, and no script runs.
func TestExport_List_NoOverrideUsesBuiltin(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")

	// Override configured on a DIFFERENT list, so the tickets export must
	// not pick it up.
	s := app.State()
	cfg := *s.Cfg
	lists := map[string]dataentryconfig.List{}
	maps.Copy(lists, cfg.Lists)
	lists["other"] = dataentryconfig.List{
		EntityType:   "feature",
		Columns:      []dataentryconfig.ListColumn{{Property: "title"}},
		ExportRender: "docs/other.lua",
	}
	cfg.Lists = lists
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})
	fake := &fakeScriptEngine{stdout: func(fakeScriptCall) string { return "SHOULD NOT RUN" }}
	app.documents = newDocumentService(app.store, app.kv, "/", fake,
		func() lua.WriteDeps { return lua.WriteDeps{} })
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	ctx := adminListCtx(t, app)

	body := exportListURL(ctx, app, "transform=copy&list=tickets").Body.String()
	if !strings.Contains(body, "| Title | Status | Implements |") {
		t.Errorf("built-in header row missing:\n%s", body)
	}
	if strings.Contains(body, "SHOULD NOT RUN") {
		t.Errorf("unrelated list's override ran:\n%s", body)
	}
	if fake.callCount() != 0 {
		t.Errorf("no script should have run, got %d calls", fake.callCount())
	}
}

// TestExport_List_RenderOverride_EffectiveListFallback is the regression test
// for resolving the override independently of the columns: a request with no
// ?list= must still find the type's default list AND its override.
func TestExport_List_RenderOverride_EffectiveListFallback(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")
	withListRenderOverride(t, app, func(c fakeScriptCall) string {
		return "# Fallback " + c.listID + "\n"
	})
	ctx := adminListCtx(t, app)

	// No list= param at all — the type's default list comes from navigation.
	body := exportListURL(ctx, app, "transform=copy").Body.String()
	if !strings.Contains(body, "# Fallback tickets") {
		t.Errorf("override did not apply without an explicit list= param:\n%s", body)
	}
	if strings.Contains(body, "| Title | Status |") {
		t.Errorf("built-in table rendered instead of the override:\n%s", body)
	}
}

// TestExport_List_RenderOverride_ColumnlessList pins that a list carrying an
// override but no columns still gets its override. The columns predicate is a
// columns concern; folding it into list identity would silently skip this.
func TestExport_List_RenderOverride_ColumnlessList(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")

	s := app.State()
	cfg := *s.Cfg
	cfg.Lists = map[string]dataentryconfig.List{
		"tickets": {EntityType: "ticket", ExportRender: "docs/fancy_list.lua"},
	}
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})
	fake := &fakeScriptEngine{stdout: func(c fakeScriptCall) string { return "# Columnless " + c.listID + "\n" }}
	app.documents = newDocumentService(app.store, app.kv, "/", fake,
		func() lua.WriteDeps { return lua.WriteDeps{} })
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	ctx := adminListCtx(t, app)

	body := exportListURL(ctx, app, "transform=copy&list=tickets").Body.String()
	if !strings.Contains(body, "# Columnless tickets") {
		t.Errorf("override on a columns-less list did not apply:\n%s", body)
	}
}

// TestExport_List_RenderOverride_DeniedSeesNoRows asserts the override sits
// downstream of the ACL read. A denied caller yields an EMPTY row set and a
// 200 — deliberately not a 404, which is what the built-in table does too;
// diverging would leak "no access" vs "no rows".
func TestExport_List_RenderOverride_DeniedSeesNoRows(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "Top Secret"}})
	fake := withListRenderOverride(t, app, func(c fakeScriptCall) string {
		return "rows=" + strings.Join(c.rowIDs, ",") + "\n"
	})

	// alice may read features, not tickets.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportListURL(ctx, app, "transform=copy&list=tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (empty set, not 404)", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Top Secret") {
		t.Errorf("denied export leaked entity content: %s", rec.Body)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	if got := fake.calls[0].rowIDs; len(got) != 0 {
		t.Errorf("denied caller's script saw rows %v, want none", got)
	}
	if got := fake.calls[0].query.Total; got != 0 {
		t.Errorf("Total = %d, want 0 for a denied caller", got)
	}
}

// TestExport_List_RenderOverride_TruncationReachesScript asserts the cap
// bookkeeping reaches the script so an override can render its own notice.
func TestExport_List_RenderOverride_TruncationReachesScript(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1", "TKT-2", "TKT-3", "TKT-4", "TKT-5")

	orig := listExportCap
	t.Cleanup(func() { listExportCap = orig })
	listExportCap = 2

	fake := withListRenderOverride(t, app, func(fakeScriptCall) string {
		return "rendered\n"
	})
	ctx := adminListCtx(t, app)

	rec := exportListURL(ctx, app, "transform=copy&list=tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	call := fake.calls[0]
	if len(call.rowIDs) != 2 {
		t.Errorf("script saw %d rows, want 2 (the cap)", len(call.rowIDs))
	}
	if call.query.Total != 5 {
		t.Errorf("Total = %d, want 5 (pre-cap count)", call.query.Total)
	}
	// Truncation is derived at render time from Total vs the row count, so
	// assert the two inputs the script's `truncated` is computed from.
	if got := call.query.Total > len(call.rowIDs); !got {
		t.Error("Total <= rows, so the script would see truncated=false")
	}
	// The built-in truncation notice must NOT appear — the script owns its
	// own notice when it overrides.
	if strings.Contains(rec.Body.String(), "truncated") {
		t.Errorf("built-in truncation notice leaked into an override:\n%s", rec.Body)
	}
}

// TestExport_List_RenderOverride_RowsWalkableTwice pins the iterator contract
// at the provider level: a script that walks once to total and again to emit
// must see the full set both times.
func TestExport_List_RenderOverride_RowsWalkableTwice(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1", "TKT-2", "TKT-3")
	fake := withListRenderOverride(t, app, func(fakeScriptCall) string { return "ok\n" })
	ctx := adminListCtx(t, app)

	exportListURL(ctx, app, "transform=copy&list=tickets")

	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	call := fake.calls[0]
	want := "TKT-1,TKT-2,TKT-3"
	if got := strings.Join(call.rowIDs, ","); got != want {
		t.Errorf("provider yielded %q, want %q (in list order)", got, want)
	}
	// The provider is re-readable — a precondition for the Lua cursor's
	// walk-twice contract, which is itself pinned against a real runtime by
	// TestListDocumentMode_IteratorWalkableTwice in internal/lua.
	if got := strings.Join(call.rowIDs2, ","); got != want {
		t.Errorf("second read yielded %q, want %q", got, want)
	}
}

// TestExport_List_RenderOverride_QueryContext asserts the resolved filters and
// sort reach the script, so an override can title and annotate the export.
func TestExport_List_RenderOverride_QueryContext(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")
	fake := withListRenderOverride(t, app, func(fakeScriptCall) string { return "ok\n" })
	ctx := adminListCtx(t, app)

	exportListURL(ctx, app, "transform=copy&list=tickets&filter[status]=open&sort=-title&q=urgent")

	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	q := fake.calls[0].query
	if fake.calls[0].listID != "tickets" || q.EntityType != "ticket" {
		t.Errorf("list identity wrong: listID=%q %+v", fake.calls[0].listID, q)
	}
	if q.Q != "urgent" {
		t.Errorf("Q = %q, want %q", q.Q, "urgent")
	}
	if q.Filters["status"] != "open" {
		t.Errorf("Filters = %v, want status=open", q.Filters)
	}
	if len(q.Sort) != 1 || q.Sort[0].Property != "title" || q.Sort[0].Direction != "desc" {
		t.Errorf("Sort = %+v, want one {title desc}", q.Sort)
	}
}

// TestExport_List_RenderOverride_FilterOperatorKey pins that an operator
// segment is parsed, not swallowed into the property name (the RR-6RF60V bug
// class). A naive TrimPrefix/TrimSuffix keys this table on "status][ne", so a
// script would report a filter name that never matches what was filtered on.
func TestExport_List_RenderOverride_FilterOperatorKey(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")
	fake := withListRenderOverride(t, app, func(fakeScriptCall) string { return "ok\n" })
	ctx := adminListCtx(t, app)

	exportListURL(ctx, app, "transform=copy&list=tickets&filter[status][ne]=done")

	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	filters := fake.calls[0].query.Filters
	if got, ok := filters["status"]; !ok || got != "done" {
		t.Errorf("Filters = %v, want the operator segment parsed off to key %q", filters, "status")
	}
	if _, bad := filters["status][ne"]; bad {
		t.Errorf("operator segment swallowed into the key: %v", filters)
	}
}

// TestExport_List_RenderOverride_ConfigIDDistinctFromEntityPath pins the
// PROPERTY the "list:" infix exists for: a list whose id equals an entity type
// name must not produce the same config identity as that type's entity-export
// override. Asserting the literal string alone would pass even if the infix
// were dropped.
func TestExport_List_RenderOverride_ConfigIDDistinctFromEntityPath(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")

	// A list literally named "ticket" — the same string as the entity type,
	// whose entity-export ConfigID is "export:ticket".
	s := app.State()
	cfg := *s.Cfg
	cfg.Lists = map[string]dataentryconfig.List{
		"ticket": {EntityType: "ticket", ExportRender: "docs/fancy_list.lua"},
	}
	cfg.Navigation = []dataentryconfig.NavigationEntry{{List: "ticket"}}
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})
	fake := &fakeScriptEngine{stdout: func(fakeScriptCall) string { return "ok\n" }}
	app.documents = newDocumentService(app.store, app.kv, "/", fake,
		func() lua.WriteDeps { return lua.WriteDeps{} })
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	ctx := adminListCtx(t, app)

	exportListURL(ctx, app, "transform=copy&list=ticket")

	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	got := fake.calls[0].documentID
	if entityPathID := "export:" + "ticket"; got == entityPathID {
		t.Errorf("list ConfigID %q collides with the entity path's %q", got, entityPathID)
	}
}

// TestExport_List_RenderOverride_ConfigIDNamespaced pins the synthetic config
// identity, whose "list:" infix keeps a list id from colliding with an entity
// type name on the entity export path.
func TestExport_List_RenderOverride_ConfigIDNamespaced(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedExportTickets(app, "TKT-1")
	fake := withListRenderOverride(t, app, func(fakeScriptCall) string { return "ok\n" })
	ctx := adminListCtx(t, app)

	exportListURL(ctx, app, "transform=copy&list=tickets")

	if len(fake.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(fake.calls))
	}
	if got, want := fake.calls[0].documentID, "export:list:tickets"; got != want {
		t.Errorf("documentID = %q, want %q", got, want)
	}
	if got, want := fake.calls[0].path, "docs/fancy_list.lua"; got != want {
		t.Errorf("script path = %q, want %q", got, want)
	}
}
