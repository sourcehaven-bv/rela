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
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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
		App:        dataentryconfig.AppConfig{Name: "Export Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	return newAppFromParts(cfg, meta, newFixture())
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
	app.handleV1Transforms(rec, req)

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
		Properties: map[string]any{"title": "Do the thing", "status": "open"}})
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
	for _, want := range []string{"# Do the thing", "| status | open |", "## implements", "- The Feature"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s", want, body)
		}
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
	app.handleV1ExportEntity(rec, req, "ticket", "TKT-001")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing transform", rec.Code)
	}
}

func exportEntity(ctx context.Context, app *App, typeName, id, transformName string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/"+typeName+"s/"+id+"/_export?transform="+transformName, http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1ExportEntity(rec, req, typeName, id)
	return rec
}
