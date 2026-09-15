package dataentry

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// newDocExportApp builds an app that can both render documents through a fake
// script engine and export them through a real transform ("copy" = cp{in}{out},
// the identity transform newExportApp uses). The two halves are wired together
// here because document export is exactly their intersection: a fake renderer
// producing markdown, fed to a genuine transform + download response.
//
// Returns the app and the fake engine so a test can assert whether the renderer
// ran at all — "the renderer never executed" is the property that matters on a
// deny path, where a correct status code alone would still pass if the
// expensive Lua aggregation had already happened.
func newDocExportApp(t *testing.T, docs map[string]dataentryconfig.DocumentConfig,
	entities ...*entity.Entity) (*App, *fakeScriptEngine) {
	t.Helper()
	app := newTestAppV1(t)

	// Register the identity transform on the live metamodel so the export
	// machinery has a format to resolve.
	s := app.State()
	meta := *s.Meta
	meta.Transforms = map[string]metamodel.TransformDef{
		"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
	}
	cfg := *s.Cfg
	cfg.Documents = docs
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: &meta,
		StyleMap: s.StyleMap, StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})

	fake := withFakeDocRenderer(t, app, entities...)
	fake.stdout = func(c fakeScriptCall) string {
		// Echo the entry id so a test can prove which engine method ran: the
		// standalone path leaves it empty by construction.
		return "# doc " + c.documentID + " entry=[" + c.entryID + "]"
	}

	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	return app, fake
}

// docExportReq drives the real router (handleV1Documents), not the export
// handler directly, so every test here exercises the actual path split.
func docExportReq(t *testing.T, app *App, url string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, r)
	return rec
}

// anchoredDoc / standaloneDoc build the two document kinds. The script path is
// arbitrary — every test here renders through the fake engine, which records
// the call rather than reading the file.
func anchoredDoc() dataentryconfig.DocumentConfig {
	return dataentryconfig.DocumentConfig{EntityType: "ticket", Script: "docs/r.lua"}
}

func standaloneDoc(script string) dataentryconfig.DocumentConfig {
	return dataentryconfig.DocumentConfig{Script: script}
}

// TestExportDocument_Anchored is AC 1: an entity-anchored document exports to
// the chosen transform, with the hardened download headers.
func TestExportDocument_Anchored(t *testing.T) {
	requireCp(t)
	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
	app, fake := newDocExportApp(t,
		map[string]dataentryconfig.DocumentConfig{"report": anchoredDoc()}, ticket)
	seedEntity(app, ticket)

	rec := docExportReq(t, app, "/api/v1/_documents/report/TKT-001/_export?transform=copy")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	// The identity transform means the body IS the rendered markdown.
	if got := rec.Body.String(); !strings.Contains(got, "entry=[TKT-001]") {
		t.Errorf("body = %q, want the anchored render carrying its entry id", got)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, "report-TKT-001") {
		t.Errorf("Content-Disposition = %q, want an attachment named for doc+entity", cd)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff on an export download")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("export bytes are per-request; want Cache-Control: no-store")
	}
	if len(fake.calls) != 1 || fake.calls[0].entryID != "TKT-001" {
		t.Errorf("calls = %+v, want one anchored render of TKT-001", fake.calls)
	}
}

// TestExportDocument_Standalone is AC 2. The entry id must be ABSENT, not
// empty-string: routing a standalone render through the anchored engine call
// would set rela.document.entry_id to "" and a script branching on it would
// render the wrong document.
func TestExportDocument_Standalone(t *testing.T) {
	requireCp(t)
	app, fake := newDocExportApp(t,
		map[string]dataentryconfig.DocumentConfig{"sales": standaloneDoc("docs/s.lua")})

	rec := docExportReq(t, app, "/api/v1/_documents/sales/_export?transform=copy")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, "entry=[]") {
		t.Errorf("body = %q, want a standalone render with no entry id", got)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "sales") {
		t.Errorf("Content-Disposition = %q, want an attachment named for the document", cd)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %+v, want exactly one render", fake.calls)
	}
	if fake.calls[0].entryID != "" {
		t.Errorf("entryID = %q, want empty — a standalone document has no entry entity",
			fake.calls[0].entryID)
	}
}

// TestExportDocument_TransformResolution is AC 4.
func TestExportDocument_TransformResolution(t *testing.T) {
	requireCp(t)
	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
	docs := map[string]dataentryconfig.DocumentConfig{
		"report": anchoredDoc(),
		"sales":  standaloneDoc("docs/s.lua"),
	}

	tests := []struct {
		name string
		url  string
		want int
	}{
		{"anchored missing transform", "/api/v1/_documents/report/TKT-001/_export", http.StatusBadRequest},
		{"anchored unknown transform", "/api/v1/_documents/report/TKT-001/_export?transform=nope", http.StatusNotFound},
		{"standalone missing transform", "/api/v1/_documents/sales/_export", http.StatusBadRequest},
		{"standalone unknown transform", "/api/v1/_documents/sales/_export?transform=nope", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, fake := newDocExportApp(t, docs, ticket)
			seedEntity(app, ticket)

			rec := docExportReq(t, app, tc.url)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			// Bad caller input must be rejected before any render runs.
			if len(fake.calls) != 0 {
				t.Errorf("renderer ran %d time(s) on an unresolvable transform", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_CommandRendererRefused is AC 5. A command: document is
// refused with a clear 400 rather than rendered entity-less or half-rendered.
func TestExportDocument_CommandRendererRefused(t *testing.T) {
	requireCp(t)
	tests := []struct {
		name string
		docs map[string]dataentryconfig.DocumentConfig
		url  string
	}{
		{
			name: "standalone",
			docs: map[string]dataentryconfig.DocumentConfig{
				"sales": {Command: []string{"echo", "hi"}},
			},
			url: "/api/v1/_documents/sales/_export?transform=copy",
		},
		{
			name: "anchored",
			docs: map[string]dataentryconfig.DocumentConfig{
				"report": {EntityType: "ticket", Command: []string{"echo", "hi"}},
			},
			url: "/api/v1/_documents/report/TKT-001/_export?transform=copy",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
			app, fake := newDocExportApp(t, tc.docs, ticket)
			seedEntity(app, ticket)

			rec := docExportReq(t, app, tc.url)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); !strings.Contains(body, "script") {
				t.Errorf("body = %q, want an error naming the script-only limitation", body)
			}
			if len(fake.calls) != 0 {
				t.Errorf("renderer ran %d time(s) for a refused command: document", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_Gates walks the whole gate chain on the EXPORT route. A
// new route is exactly where a gate gets forgotten, so each one is asserted
// here independently of the render route's own tests. Every case also asserts
// the renderer never ran.
func TestExportDocument_Gates(t *testing.T) {
	requireCp(t)
	docs := map[string]dataentryconfig.DocumentConfig{
		"report": anchoredDoc(),
		"sales":  standaloneDoc("docs/s.lua"),
	}

	tests := []struct {
		name string
		url  string
		want int
	}{
		{"unsafe doc segment", "/api/v1/_documents/..%2Fetc/_export?transform=copy", http.StatusBadRequest},
		{"unknown document, standalone", "/api/v1/_documents/nope/_export?transform=copy", http.StatusNotFound},
		{"unknown document, anchored", "/api/v1/_documents/nope/TKT-001/_export?transform=copy", http.StatusNotFound},
		{
			"anchored document at the standalone export shape",
			"/api/v1/_documents/report/_export?transform=copy", http.StatusBadRequest,
		},
		{
			"standalone document at the anchored export shape",
			"/api/v1/_documents/sales/TKT-001/_export?transform=copy", http.StatusBadRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
			app, fake := newDocExportApp(t, docs, ticket)
			seedEntity(app, ticket)

			rec := docExportReq(t, app, tc.url)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			if len(fake.calls) != 0 {
				t.Errorf("renderer ran %d time(s) behind a gate that should have denied", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_PermissionGate pins that a document's `permission:`
// applies to the export route exactly as it does to the render route — on both
// document kinds.
func TestExportDocument_PermissionGate(t *testing.T) {
	requireCp(t)
	tests := []struct {
		name string
		docs map[string]dataentryconfig.DocumentConfig
		url  string
	}{
		{
			name: "standalone",
			docs: map[string]dataentryconfig.DocumentConfig{
				"sales": {Script: "docs/s.lua", Permission: "view-sales"},
			},
			url: "/api/v1/_documents/sales/_export?transform=copy",
		},
		{
			name: "anchored",
			docs: map[string]dataentryconfig.DocumentConfig{
				"report": {EntityType: "ticket", Script: "docs/r.lua", Permission: "view-sales"},
			},
			url: "/api/v1/_documents/report/TKT-001/_export?transform=copy",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
			app, fake := newDocExportApp(t, tc.docs, ticket)
			seedEntity(app, ticket)

			// A principal holding no permissions, but allowed to READ tickets,
			// so the only thing that can deny is the document permission.
			d := mustNewACL(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"ticket"}}},
				Assignments: map[string]string{"bob": "reader"},
			}, app.store)
			app.acl = d

			r := httptest.NewRequest(http.MethodGet, tc.url, http.NoBody).
				WithContext(gateCtxFor(principalCtx("bob"), t, d))
			rec := httptest.NewRecorder()
			app.handleV1Documents(rec, r)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (body %s)", rec.Code, rec.Body.String())
			}
			if len(fake.calls) != 0 {
				t.Errorf("renderer ran %d time(s) for a principal lacking the permission", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_ElevatedGateAppliesToExport is the highest-value test in
// this file. An elevated document reads through a RAW handle, so
// gateElevatedDocument is the only thing between a principal and everything the
// script reads. It denies under NopACL and ReadOnlyACL precisely because the
// read gate FAILS OPEN there (RR-CWWJGW) — so an export route that consulted
// the read gate alone, or skipped this gate entirely, would turn a
// permission-gated report into an ungated download.
func TestExportDocument_ElevatedGateAppliesToExport(t *testing.T) {
	requireCp(t)
	elevated := map[string]dataentryconfig.DocumentConfig{
		"sales": {
			Script:         "docs/s.lua",
			Permission:     "view-sales",
			AllowACLBypass: metamodel.ACLBypassRead,
		},
	}

	tests := []struct {
		name    string
		aclImpl acl.ACL
	}{
		{"NopACL denies", acl.NopACL{}},
		{"NopACL pointer denies", &acl.NopACL{}},
		{"ReadOnlyACL denies", acl.ReadOnlyACL{}},
		{"ReadOnlyACL pointer denies", &acl.ReadOnlyACL{}},
		{"nil ACL denies", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, fake := newDocExportApp(t, elevated)
			app.acl = tc.aclImpl

			rec := docExportReq(t, app, "/api/v1/_documents/sales/_export?transform=copy")

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 — an elevated export must fail closed (body %s)",
					rec.Code, rec.Body.String())
			}
			if len(fake.calls) != 0 {
				t.Errorf("elevated renderer ran %d time(s) behind a denying gate", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_NoExistenceOracle is AC 8 and the RR-XIYP3I regression
// test. A principal who may not read the type must not be able to tell a
// HIDDEN entity from an ABSENT one, on either route.
//
// Before this ticket the anchored handler read the store BEFORE the ACL gate
// and 404'd with a different error code, a different title, and the id echoed
// back — so the two probes were trivially distinguishable.
func TestExportDocument_NoExistenceOracle(t *testing.T) {
	requireCp(t)
	docs := map[string]dataentryconfig.DocumentConfig{"report": anchoredDoc()}

	routes := []struct {
		name string
		url  func(id string) string
	}{
		{"render", func(id string) string { return "/api/v1/_documents/report/" + id }},
		{"export", func(id string) string {
			return "/api/v1/_documents/report/" + id + "/_export?transform=copy"
		}},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
			app, fake := newDocExportApp(t, docs, ticket)
			seedEntity(app, ticket)

			// bob may read nothing, so the gate denies him either way.
			d := mustNewACL(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"outsider": {}},
				Assignments: map[string]string{"bob": "outsider"},
			}, app.store)
			app.acl = d

			probe := func(id string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodGet, route.url(id), http.NoBody).
					WithContext(gateCtxFor(principalCtx("bob"), t, d))
				rec := httptest.NewRecorder()
				app.handleV1Documents(rec, r)
				return rec
			}

			hidden := probe("TKT-001") // exists, denied
			absent := probe("TKT-999") // does not exist

			if hidden.Code != http.StatusNotFound || absent.Code != http.StatusNotFound {
				t.Fatalf("want 404 for both, got hidden=%d absent=%d", hidden.Code, absent.Code)
			}

			h, a := decodeProblem(t, hidden), decodeProblem(t, absent)
			// `instance` echoes the requested URL, which is information the
			// caller supplied, not information about the entity.
			delete(h, "instance")
			delete(a, "instance")
			if !maps.Equal(h, a) {
				t.Errorf("hidden entity distinguishable from absent one (existence oracle):"+
					"\n hidden=%v\n absent=%v", h, a)
			}
			if len(fake.calls) != 0 {
				t.Errorf("renderer ran %d time(s) for a denied principal", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_ScriptErrorCarriesNoDetail is AC 9. Export keeps the flat
// 500 both shipped export routes produce. This is deliberate (RR-74LHU1): a
// lua.ScriptError carries Source, Stack and CapturedOutput, and
// AttachCapturedOutput puts the script's PARTIAL STDOUT in the last of those —
// which for an elevated document is content rendered under a bypassed ACL.
// Keep it out of an export error body entirely.
func TestExportDocument_ScriptErrorCarriesNoDetail(t *testing.T) {
	requireCp(t)
	app, fake := newDocExportApp(t,
		map[string]dataentryconfig.DocumentConfig{"sales": standaloneDoc("docs/secret_path.lua")})
	fake.err = &lua.ScriptError{
		Path:           "docs/secret_path.lua",
		LuaMessage:     "boom",
		Source:         []lua.SourceLine{{Text: "secret source line"}},
		Stack:          []lua.StackFrame{{Path: "secret stack frame"}},
		CapturedOutput: "secret partial output",
	}

	rec := docExportReq(t, app, "/api/v1/_documents/sales/_export?transform=copy")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	for _, leak := range []string{"secret source line", "secret stack frame", "secret partial output", "secret_path.lua"} {
		if strings.Contains(body, leak) {
			t.Errorf("export error body leaked %q:\n%s", leak, body)
		}
	}
}

// TestExportDocument_UsesSharedEngine pins that document export runs on the
// SAME transform.Engine as entity/list export. The engine owns the bounded
// worker pool capping concurrent converter processes, so a second engine for
// this path would give it its own pool and the cap would bound nothing.
func TestExportDocument_UsesSharedEngine(t *testing.T) {
	app, _ := newDocExportApp(t,
		map[string]dataentryconfig.DocumentConfig{"sales": standaloneDoc("docs/s.lua")})

	if app.export.engine == nil {
		t.Fatal("export handler has no engine")
	}
	// Document export routes through a.export.convertAndWrite, so it is the
	// same engine by construction; assert the handler is the shared one rather
	// than a per-request build.
	before := app.export.engine
	_ = docExportReq(t, app, "/api/v1/_documents/sales/_export?transform=copy")
	if app.export.engine != before {
		t.Error("export engine was replaced during a request; the concurrency cap would bound nothing")
	}
}

// TestExportBaseName pins the download filename stem for both document kinds,
// including a dotted document name — isSafePathSegment permits '.', and
// safeAttachmentFilename sanitizes stem and extension separately downstream.
func TestExportBaseName(t *testing.T) {
	tests := []struct {
		name, doc, entity, want string
	}{
		{"standalone", "sales", "", "sales"},
		{"anchored", "report", "TKT-001", "report-TKT-001"},
		{"dotted standalone", "report.tar", "", "report.tar"},
		{"dotted anchored", "report.tar", "TKT-001", "report.tar-TKT-001"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := exportBaseName(tc.doc, tc.entity); got != tc.want {
				t.Errorf("exportBaseName(%q, %q) = %q, want %q", tc.doc, tc.entity, got, tc.want)
			}
		})
	}
}

// TestExportSegmentPremise guards the routing premise: an entity id can never
// collide with the reserved `_export` segment, because entity.ValidateID
// rejects a leading underscore. That rule's own godoc warns it is a
// well-formedness rule rather than a security control, so the dependency is
// pinned here — if the grammar is ever relaxed, this fails loudly instead of
// silently shadowing the export route.
func TestExportSegmentPremise(t *testing.T) {
	if err := entity.ValidateID(exportSegment); err == nil {
		t.Fatalf("entity.ValidateID(%q) accepted the reserved export segment; "+
			"an entity with that id would shadow the document export route", exportSegment)
	}
	if exportSegment != dataentryconfig.ReservedExportSegment {
		t.Errorf("route segment %q and the reserved config name %q have drifted",
			exportSegment, dataentryconfig.ReservedExportSegment)
	}
}

// TestRenderDocumentMarkdown_RefusesCommandRenderer pins the fail-closed
// backstop BELOW the handler. The handler's own check produces the user-facing
// 400, but it is not the only guard: RenderMarkdown legitimately dispatches to
// renderCommand (entity export supports command: renderers), so without a
// refusal here the anchored branch would silently run a command renderer if a
// future caller reached RenderDocumentMarkdown directly.
func TestRenderDocumentMarkdown_RefusesCommandRenderer(t *testing.T) {
	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
	svc, fake := newTestService(t, ticket)

	tests := []struct {
		name    string
		entryID string
	}{
		{"standalone", ""},
		{"anchored", "TKT-001"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := documentRenderConfig{ConfigID: "doc", Command: []string{"echo", "hi"}}

			_, err := svc.RenderDocumentMarkdown(t.Context(), tc.entryID, cfg)

			if err == nil {
				t.Fatal("expected a command: renderer to be refused")
			}
			if !strings.Contains(err.Error(), "script") {
				t.Errorf("error should name the script-only requirement, got: %v", err)
			}
			if len(fake.calls) != 0 {
				t.Errorf("renderer ran %d time(s) for a refused command: document", len(fake.calls))
			}
		})
	}
}

// TestExportDocument_RouteShapes pins how every neighboring path shape
// resolves, so a future change to the segment split cannot quietly move one of
// them. The dispatch comment draws a four-shape table; this walks it, plus the
// shapes just outside it.
//
// The empty-interior-segment cases are the ones that matter most. Without an
// explicit rejection, "/{doc}//_export" splits to ["doc", "", "_export"],
// matches the 3-segment export case, and reaches the export handler with an
// empty entity id — which dispatches to the STANDALONE resolver and serves a
// standalone document at the anchored URL shape. That is what this package's
// CLAUDE.md forbids. net/http's ServeMux redirects the "//" spelling before a
// handler sees it, but a percent-encoded %2F survives its cleaning and tests
// call the router directly, so the guarantee has to live in the router.
func TestExportDocument_RouteShapes(t *testing.T) {
	requireCp(t)
	docs := map[string]dataentryconfig.DocumentConfig{
		"report": anchoredDoc(),
		"sales":  standaloneDoc("docs/s.lua"),
	}

	tests := []struct {
		name       string
		path       string
		want       int
		wantRender bool
	}{
		{"bare prefix", "/api/v1/_documents/", http.StatusBadRequest, false},
		{"standalone render", "/api/v1/_documents/sales", http.StatusOK, true},
		{"standalone render, trailing slash", "/api/v1/_documents/sales/", http.StatusOK, true},
		{"anchored render", "/api/v1/_documents/report/TKT-001", http.StatusOK, true},
		{"anchored render, trailing slash", "/api/v1/_documents/report/TKT-001/", http.StatusOK, true},
		{"standalone export, trailing slash", "/api/v1/_documents/sales/_export/", http.StatusOK, true},
		{"anchored export, trailing slash", "/api/v1/_documents/report/TKT-001/_export/", http.StatusOK, true},

		{"empty interior segment, standalone doc", "/api/v1/_documents/sales//_export", http.StatusBadRequest, false},
		{"empty interior segment, anchored doc", "/api/v1/_documents/report//_export", http.StatusBadRequest, false},
		{"encoded empty interior segment", "/api/v1/_documents/sales/%2F_export", http.StatusBadRequest, false},
		{"empty document name", "/api/v1/_documents//_export", http.StatusBadRequest, false},
		{"double empty segment", "/api/v1/_documents/sales//", http.StatusBadRequest, false},

		{"suffix past the reserved segment", "/api/v1/_documents/sales/_export/extra", http.StatusNotFound, false},
		{"anchored suffix past reserved", "/api/v1/_documents/report/TKT-001/_export/x", http.StatusNotFound, false},
		// Exact match only: "_EXPORT" is an entity id, so a standalone document
		// rejects it as a kind mismatch rather than treating it as an export.
		{"reserved segment is case-sensitive", "/api/v1/_documents/sales/_EXPORT", http.StatusBadRequest, false},
		{"reserved segment in entity position", "/api/v1/_documents/report/_export/TKT-001", http.StatusNotFound, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
			app, fake := newDocExportApp(t, docs, ticket)
			seedEntity(app, ticket)

			rec := docExportReq(t, app, tc.path+"?transform=copy")

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			if ran := len(fake.calls) > 0; ran != tc.wantRender {
				t.Errorf("renderer ran = %v, want %v", ran, tc.wantRender)
			}
		})
	}
}

// TestExportDocument_ElevationAndCapabilitiesReachTheRender is the POSITIVE
// counterpart to TestExportDocument_ElevatedGateAppliesToExport, and it exists
// because every other elevated assertion on this route is a denial.
//
// The failure it guards against is silent and fails CLOSED: if the export path
// reached the script engine without going through elevatedDeps, an elevated
// report would simply render as unelevated — no error, no denial, just a
// smaller number in a PDF. A deny-only suite cannot see that. The same applies
// to the document's declared `capabilities:` (TKT-YH52OM), which ride the same
// call.
func TestExportDocument_ElevationAndCapabilitiesReachTheRender(t *testing.T) {
	st := memstore.New()
	fake := &fakeScriptEngine{stdout: func(fakeScriptCall) string { return "# ok" }}
	caps := lua.Capabilities{HTTP: true}

	svc := newDocumentService(st, nil, "/p", fake,
		func() lua.WriteDeps { return lua.WriteDeps{} },
		func() documentElevation {
			return documentElevation{Reader: visibility.Unrestricted(st)}
		})

	cfg := documentRenderConfig{
		ConfigID:     "sales",
		Script:       "docs/s.lua",
		Elevated:     true,
		Capabilities: caps,
	}

	if _, err := svc.RenderDocumentMarkdown(t.Context(), "", cfg); err != nil {
		t.Fatalf("RenderDocumentMarkdown: %v", err)
	}

	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(fake.calls))
	}
	got := fake.calls[0].deps
	if got.ElevatedReader == nil {
		t.Error("elevated standalone export reached the engine WITHOUT the bypass reader; " +
			"the render would silently produce an unelevated report")
	}
	if !reflect.DeepEqual(got.Capabilities, caps) {
		t.Errorf("Capabilities = %+v, want %+v — the document's declared grant was dropped",
			got.Capabilities, caps)
	}
}
