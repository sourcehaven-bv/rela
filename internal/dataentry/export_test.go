package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/transform"
)

// newExportApp builds an app whose metamodel registers an identity transform
// ("copy" = cp {in} {out}) so export tests need no external converter. It mirrors
// newTestAppV1's entity/relation shape and adds the transforms registry.
func newExportApp(t *testing.T) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
				PropertyOrder: []string{"title", "status"},
			},
			"feature": {
				Label:         "Feature",
				IDPrefix:      "FEAT-",
				Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
				PropertyOrder: []string{"title"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {Label: "implements", From: []string{"ticket"}, To: []string{"feature"}},
		},
		Transforms: map[string]metamodel.TransformDef{
			"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
		},
	}
	cfg := &dataentryconfig.Config{
		App:   dataentryconfig.AppConfig{Name: "Export Test"},
		Forms: make(map[string]dataentryconfig.Form),
		Lists: map[string]dataentryconfig.List{
			"tickets": {
				EntityType: "ticket",
				Columns: []dataentryconfig.ListColumn{
					{Property: "title", Label: "Title"},
					{Property: "status", Label: "Status"},
					{Relation: "implements", Label: "Implements"},
				},
			},
		},
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{{List: "tickets"}},
	}
	return newAppFromParts(cfg, meta, newFixture())
}

// withRenderOverride configures a per-type export_render override for typeName
// and swaps app.documents for a fake-script-engine documentService that emits
// `output` for the exported entry — so the override tests exercise the render
// path without a real Lua runtime. Republishes the schema and rebuilds the
// export handler so both pick up the change.
func withRenderOverride(t *testing.T, app *App, typeName string, output func(entryID string) string) {
	t.Helper()
	s := app.State()
	cfg := *s.Cfg
	cfg.Views = map[string]dataentryconfig.ViewConfig{
		typeName: {
			Entry:        dataentryconfig.ViewEntry{Type: typeName},
			ExportRender: "docs/fancy.lua",
		},
	}
	app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap, StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen})

	fake := &fakeScriptEngine{stdout: func(c fakeScriptCall) string { return output(c.entryID) }}
	deps := func() lua.WriteDeps { return lua.WriteDeps{} }
	app.documents = newDocumentService(app.store, app.kv, "/", fake, deps)
	app.export = newExportHandler(app)
}

func requireCp(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX cp fixture")
	}
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
}

func TestExport_TransformsList(t *testing.T) {
	app := newExportApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_transforms", http.NoBody)
	rec := httptest.NewRecorder()
	app.export.handleV1Transforms(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var got []transformInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	if len(got) != 1 || got[0].Name != "copy" || got[0].Produces != "text/plain" {
		t.Fatalf("transforms list = %+v, want one 'copy'/'text/plain'", got)
	}
}

func TestExport_Entity_VisibleReturnsBytesAndHeaders(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Do the thing", "status": "open", "priority": "high"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "The Feature"}})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})

	// Allow-all ACL for alice.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportEntity(ctx, app, "ticket", "TKT-001", "copy")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	// Hardened download headers.
	h := rec.Header()
	if got := h.Get("Content-Type"); got != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := h.Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
		t.Errorf("CSP = %q, want sandbox", got)
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if cd := h.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "TKT-001") {
		t.Errorf("Content-Disposition = %q, want attachment + TKT-001", cd)
	}
	// The identity transform returns the rendered markdown verbatim.
	body := rec.Body.String()
	for _, want := range []string{"# Do the thing", "**priority:** high", "## implements", "- The Feature"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s", want, body)
		}
	}
	if strings.Contains(body, "**status:**") {
		t.Error("status should be omitted from the rendered document")
	}
}

func TestExport_Entity_DeniedReturns404NoBytes(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Secret"}})

	// alice may read features, NOT tickets → ticket export must 404.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportEntity(ctx, app, "ticket", "TKT-001", "copy")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Secret") {
		t.Errorf("denied export leaked entity content: %s", rec.Body)
	}
}

func TestExport_Entity_UnknownTransform(t *testing.T) {
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "T"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportEntity(ctx, app, "ticket", "TKT-001", "nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for unknown transform", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unknown_transform") {
		t.Errorf("body missing unknown_transform: %s", rec.Body)
	}
}

func TestExport_Entity_MissingTransformParam(t *testing.T) {
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "T"}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001/_export", http.NoBody)
	rec := httptest.NewRecorder()
	app.export.handleV1ExportEntity(rec, req, "ticket", "TKT-001")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing transform", rec.Code)
	}
}

func exportEntity(ctx context.Context, app *App, typeName, id, transformName string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/"+typeName+"s/"+id+"/_export?transform="+transformName, http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.export.handleV1ExportEntity(rec, req, typeName, id)
	return rec
}

// TestExport_EngineIsSharedAcrossRequests pins that the transform engine — and
// therefore its bounded worker pool — is reused rather than rebuilt per request.
// A per-request engine gives every request a private pool, so N concurrent
// exports spawn N×poolSize converter processes and the concurrency limit bounds
// nothing. This was a real bug: the handlers originally called NewEngine inline.
func TestExport_EngineIsSharedAcrossRequests(t *testing.T) {
	app := newExportApp(t)
	reg := transform.RegistryFromMetamodel(app.Meta())

	first, err := app.export.transformEngine(reg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.export.transformEngine(reg)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("transform engine must be shared across requests; a per-request " +
			"engine gives each request its own pool and defeats the concurrency bound")
	}
}

// TestExport_EngineRebuiltWhenRegistryChanges pins the other half: a metamodel
// live-reload that changes the transforms must produce a fresh engine, so a
// config change is actually picked up.
func TestExport_EngineRebuiltWhenRegistryChanges(t *testing.T) {
	app := newExportApp(t)
	reg := transform.RegistryFromMetamodel(app.Meta())
	first, err := app.export.transformEngine(reg)
	if err != nil {
		t.Fatal(err)
	}

	changed := transform.Registry{
		"copy":  {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
		"extra": {From: "markdown", Command: []string{"cat"}, Produces: "text/plain"},
	}
	second, err := app.export.transformEngine(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("a changed registry must rebuild the engine")
	}
}

func TestExport_Entity_RenderOverride(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Plain"}})
	// Ticket type configures export_render — exporting a ticket must route through
	// it, NOT the built-in property renderer.
	withRenderOverride(t, app, "ticket", func(entryID string) string { return "# Fancy " + entryID + "\n" })

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportEntity(ctx, app, "ticket", "TKT-001", "copy")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "# Fancy TKT-001") {
		t.Errorf("override output missing '# Fancy TKT-001':\n%s", body)
	}
	// The built-in renderer would have emitted a "**...:**" property line; the
	// override replaces it entirely.
	if strings.Contains(body, "**") {
		t.Errorf("built-in property lines leaked; override should fully replace them:\n%s", body)
	}
}

func TestExport_Entity_NoOverrideUsesBuiltin(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "The Feature"}})
	// Override configured for TICKET only; feature has none → built-in renderer.
	withRenderOverride(t, app, "ticket", func(string) string { return "SHOULD NOT RUN" })

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"feature"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportEntity(ctx, app, "feature", "FEAT-001", "copy")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "# The Feature") {
		t.Errorf("built-in renderer should produce the H1 title:\n%s", body)
	}
	if strings.Contains(body, "SHOULD NOT RUN") {
		t.Error("feature has no override; the render script must not run")
	}
}

func TestExport_Entity_RenderOverride_DeniedIs404(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Secret"}})
	withRenderOverride(t, app, "ticket", func(entryID string) string { return "# Fancy " + entryID + "\n" })

	// alice cannot read tickets → override render must never run; 404.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportEntity(ctx, app, "ticket", "TKT-001", "copy")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for denied override export", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Fancy") {
		t.Errorf("denied override leaked render output: %s", rec.Body)
	}
}

func exportList(ctx context.Context, app *App) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/_export?transform=copy&list=tickets", http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.export.handleV1ExportList(rec, req, "ticket")
	return rec
}

func TestExport_List_WholeScopedSetAsTable(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	// Seed more tickets than a page (default per_page) to prove the export
	// covers the whole filtered set, not just a page.
	for i := range 30 {
		id := "TKT-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": "T" + id, "status": "open"}})
	}
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "Feat One"}})
	seedRelation(app, &entity.Relation{From: "TKT-A0", Type: "implements", To: "FEAT-1"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportList(ctx, app)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	// Header from the configured columns.
	if !strings.Contains(body, "| Title | Status | Implements |") {
		t.Errorf("missing configured header row:\n%s", firstLines(body, 3))
	}
	// One body row per seeded ticket (30) plus the header + separator = 32 lines
	// minimum; assert a couple of specific rows present.
	if !strings.Contains(body, "Feat One") {
		t.Errorf("relation column should show visible neighbor title 'Feat One':\n%s", body)
	}
	// Count table body rows (lines starting with "| T" for title cells).
	rows := strings.Count(body, "\n| TTKT-")
	if rows != 30 {
		t.Errorf("want 30 body rows, got %d\n%s", rows, body)
	}
}

func TestExport_List_SharedNeighborResolvedOnce(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	// Two tickets implement the SAME feature. Batched resolution must render the
	// shared neighbor's title in both rows (memoized, one gate for the export).
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "T2"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "Shared Feature"}})
	seedRelation(app, &entity.Relation{From: "TKT-1", Type: "implements", To: "FEAT-1"})
	seedRelation(app, &entity.Relation{From: "TKT-2", Type: "implements", To: "FEAT-1"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	body := exportList(ctx, app).Body.String()
	if got := strings.Count(body, "Shared Feature"); got != 2 {
		t.Errorf("shared neighbor should appear in both rows (2), got %d\n%s", got, body)
	}
}

func TestExport_List_HiddenNeighborExcluded(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T1", "status": "open"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-HIDDEN", Type: "feature", Properties: map[string]any{"title": "Secret Feature"}})
	seedRelation(app, &entity.Relation{From: "TKT-1", Type: "implements", To: "FEAT-HIDDEN"})

	// alice may read tickets but NOT features → the relation cell must be empty,
	// never leaking the hidden feature's title.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	rec := exportList(ctx, app)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "Secret Feature") {
		t.Errorf("hidden neighbor title leaked into list export:\n%s", rec.Body)
	}
}

func TestExport_List_TruncationNotice(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "admin"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	for i := range 3 {
		id := "TKT-" + string(rune('0'+i))
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
	}

	// Exactly at the cap → no notice.
	orig := listExportCap
	t.Cleanup(func() { listExportCap = orig })

	listExportCap = 3
	if rec := exportList(ctx, app); strings.Contains(rec.Body.String(), "truncated") {
		t.Errorf("set == cap should not truncate:\n%s", rec.Body)
	}

	// One over the cap → truncated + visible notice showing N of M.
	listExportCap = 2
	rec := exportList(ctx, app)
	body := rec.Body.String()
	if !strings.Contains(body, "Showing 2 of 3 rows (truncated).") {
		t.Errorf("want truncation notice 'Showing 2 of 3 rows (truncated).':\n%s", body)
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
