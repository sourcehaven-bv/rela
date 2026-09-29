package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Exporting a faced address (BUG-PLZDPR). Every assertion here is on WHICH
// face's content came back, never on a status alone: memstore resolves a
// suffixed id by accident, so a 200 can come from the wrong row (the
// BUG-VFHUWO lesson).

// facedExportApp is facedApp (policy declares draft+published, default world
// published, neighbors wired) plus the identity transform `copy`.
func facedExportApp(t *testing.T, d func(st store.Store) *acl.Declarative) (*App, *acl.Declarative) {
	t.Helper()
	requireCp(t)
	app, decl := facedApp(t, d)
	s := app.State()
	meta := *s.Meta
	meta.Transforms = map[string]metamodel.TransformDef{
		"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
	}
	app.schema.Publish(&Schema{Cfg: s.Cfg, Meta: &meta})
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	return app, decl
}

// exportRouted exports addr (plural/id, optionally with a query) through the
// real router, so attachWorld binds the configured default world, or the
// `?world=` in the query, exactly as it does for a browser.
func exportRouted(t *testing.T, app *App, addr string) *httptest.ResponseRecorder {
	t.Helper()
	path, query, _ := strings.Cut(addr, "?")
	if query != "" {
		query = "&" + query
	}
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/"+path+"/_export?transform=copy"+query, http.NoBody))
	return rec
}

// assertExport fails unless addr exports with 200, contains every want and
// none of notWant.
func assertExport(t *testing.T, app *App, addr string, want, notWant []string) {
	t.Helper()
	rec := exportRouted(t, app, addr)
	if rec.Code != http.StatusOK {
		t.Fatalf("export %s: status = %d, want 200; body=%s", addr, rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("export %s = %q, want it to contain %q", addr, body, w)
		}
	}
	for _, w := range notWant {
		if strings.Contains(body, w) {
			t.Errorf("export %s = %q, must not contain %q", addr, body, w)
		}
	}
}

func TestExport_FacedAddressExportsThatFace(t *testing.T) {
	app, _ := facedExportApp(t, nil)

	tests := []struct {
		name, addr, want, notWant string
	}{
		{"explicit draft", "policys/POL-1@draft", "DRAFT TEXT", "PUBLISHED TEXT"},
		{"explicit published", "policys/POL-1@published", "PUBLISHED TEXT", "DRAFT TEXT"},
		// Export resolves a bare id through the world like the entity GET,
		// so the configured default world (published) picks the face.
		{"bare id in the default world", "policys/POL-1", "PUBLISHED TEXT", "DRAFT TEXT"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExport(t, app, tc.addr, []string{tc.want}, []string{tc.notWant})
		})
	}
}

// seedExportGraph adds entities and relations to a facedExportApp store.
func seedExportGraph(t *testing.T, app *App, ents []*entity.Entity, rels [][3]string, tails []entity.Face) {
	t.Helper()
	ctx := context.Background()
	for _, e := range ents {
		if err := app.store.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s@%q: %v", e.ID, e.Face, err)
		}
	}
	for i, r := range rels {
		if _, err := app.store.CreateRelation(ctx, r[0], r[1], r[2],
			&store.RelationData{FromFace: tails[i]}); err != nil {
			t.Fatalf("seed %v@%q: %v", r, tails[i], err)
		}
	}
}

// TestExport_FacedAddressCarriesOnlyItsOwnEdges: a content-scoped edge
// belongs to one face, so the published export must not list the draft's
// link. The bare-id relation read returned both.
func TestExport_FacedAddressCarriesOnlyItsOwnEdges(t *testing.T) {
	app, _ := facedExportApp(t, nil)
	seedExportGraph(t, app,
		[]*entity.Entity{
			{ID: "FEAT-DRAFT", Type: "feature", Properties: map[string]any{"title": "FEAT-DRAFT title"}},
			{ID: "FEAT-PUB", Type: "feature", Properties: map[string]any{"title": "FEAT-PUB title"}},
		},
		[][3]string{{"POL-1", "cites", "FEAT-DRAFT"}, {"POL-1", "cites", "FEAT-PUB"}},
		[]entity.Face{"draft", "published"})

	tests := []struct{ addr, has, hasNot string }{
		{"policys/POL-1@draft", "FEAT-DRAFT title", "FEAT-PUB title"},
		{"policys/POL-1@published", "FEAT-PUB title", "FEAT-DRAFT title"},
	}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			assertExport(t, app, tc.addr, []string{tc.has}, []string{tc.hasNot})
		})
	}
}

// TestExport_FacedNeighborsResolveInTheWorld: a neighbor that declares
// faces stores no bare row, so its title comes from the face the request's
// world resolves, and a neighbor the world excludes is not listed. This is
// what dropped every faced link while export ignored the world.
func TestExport_FacedNeighborsResolveInTheWorld(t *testing.T) {
	app, _ := facedExportApp(t, nil)
	seedExportGraph(t, app,
		[]*entity.Entity{
			{ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "POL2 DRAFT"}},
			{ID: "POL-2", Type: "policy", Face: "published", Properties: map[string]any{"title": "POL2 PUB"}},
			{ID: "POL-3", Type: "policy", Face: "draft", Properties: map[string]any{"title": "POL3 DRAFT"}},
			{ID: "FEAT-X", Type: "feature", Properties: map[string]any{"title": "FEAT-X title"}},
		},
		[][3]string{
			{"POL-1", "relates-to", "POL-2"},
			{"POL-1", "relates-to", "POL-3"},
			// Identity-scoped, so FEAT-X lists it; POL-1 has no bare row.
			{"POL-1", "implements", "FEAT-X"},
		},
		[]entity.Face{"published", "published", ""})

	t.Run("faced to faced", func(t *testing.T) {
		// POL-3 has no published face, and the published world excludes it.
		assertExport(t, app, "policys/POL-1@published",
			[]string{"POL2 PUB", "FEAT-X title"}, []string{"POL2 DRAFT", "POL3 DRAFT"})
	})
	t.Run("unfaced to faced", func(t *testing.T) {
		assertExport(t, app, "features/FEAT-X",
			[]string{"PUBLISHED TEXT"}, []string{"DRAFT TEXT"})
	})
}

// TestExport_DeniedWorldIsNotFound: export is world-capable, so a principal
// who may not select the world reads nothing in it, whatever the address.
func TestExport_DeniedWorldIsNotFound(t *testing.T) {
	app, _ := facedExportApp(t, func(st store.Store) *acl.Declarative {
		// Reads everything, but holds no `world:published` grant.
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"*"}}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})

	assertExport(t, app, "policys/POL-1@published?world=default", []string{"PUBLISHED TEXT"}, nil)
	for _, addr := range []string{"policys/POL-1?world=published", "policys/POL-1@published?world=published"} {
		rec := exportRouted(t, app, addr)
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "TEXT") {
			t.Errorf("export %s under a denied world = %d %s, want 404", addr, rec.Code, rec.Body)
		}
	}
}

// TestExport_NeighborFaceGateHidesTheLink: the neighbor's resolved face
// passes the same face gate as a direct read, so a reader without that face
// sees neither its title nor the link.
func TestExport_NeighborFaceGateHidesTheLink(t *testing.T) {
	app, _ := facedExportApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"viewer": {
				Read: []string{"policy@draft", "feature"}, Worlds: []string{"published"},
			}},
			Assignments: map[string]string{"alice": "viewer"},
		}, st)
	})
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})
	seedExportGraph(t, app,
		[]*entity.Entity{{ID: "FEAT-X", Type: "feature", Properties: map[string]any{"title": "FEAT-X title"}}},
		[][3]string{{"POL-1", "implements", "FEAT-X"}},
		[]entity.Face{""})

	assertExport(t, app, "features/FEAT-X?world=published", []string{"FEAT-X title"},
		[]string{"PUBLISHED TEXT", "POL-1"})
}

// TestExport_UnreadableFaceIsIndistinguishableFromMissing is AC 2: a caller
// who may read one face but not the requested one gets the not-found a
// missing entity gets, and no content.
func TestExport_UnreadableFaceIsIndistinguishableFromMissing(t *testing.T) {
	app, d := facedExportApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"policy@draft", "feature"}}},
			Assignments: map[string]string{"alice": "viewer"},
		}, st)
	})
	ctx := gateCtxFor(aliceCtx(), t, d)

	// Positive control: the granted face exports.
	if rec := exportEntity(ctx, app, "policy", "POL-1@draft", "copy"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "DRAFT TEXT") {

		t.Fatalf("precondition: alice exports the draft face; got %d %s", rec.Code, rec.Body)
	}

	denied := exportEntity(ctx, app, "policy", "POL-1@published", "copy")
	missing := exportEntity(ctx, app, "policy", "POL-404@published", "copy")
	if denied.Code != http.StatusNotFound {
		t.Fatalf("denied face: status = %d, want 404; body=%s", denied.Code, denied.Body)
	}
	if strings.Contains(denied.Body.String(), "PUBLISHED TEXT") {
		t.Errorf("denied face leaked its content: %s", denied.Body)
	}
	// The problem body echoes the request path as `instance`; everything
	// else must match.
	deniedBody := strings.ReplaceAll(denied.Body.String(), "POL-1@", "POL-404@")
	if denied.Code != missing.Code || deniedBody != missing.Body.String() {
		t.Errorf("denied face distinguishable from a missing entity:\ndenied:  %d %s\nmissing: %d %s",
			denied.Code, denied.Body, missing.Code, missing.Body)
	}
}

// TestExport_NeighborFaultFailsTheExport: a failed neighbor read must fail
// the export, never ship a file that silently omits the links it could not
// resolve.
func TestExport_NeighborFaultFailsTheExport(t *testing.T) {
	app, _ := facedExportApp(t, nil)
	app.export.faceNeighbors = func(context.Context, *entity.Entity) (
		outgoing, incoming []*entity.Relation, neighbors map[string]*entity.Entity, err error,
	) {
		return nil, nil, nil, errors.New("store is down")
	}
	rec := exportRouted(t, app, "policys/POL-1@published")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "PUBLISHED TEXT") {
		t.Errorf("export with a failing neighbor read = %d %s, want 500 and no content", rec.Code, rec.Body)
	}
}

// TestExport_FacedRenderOverrideReceivesTheAddress: `export_render` renders
// the face being exported, so its entry id is the address, face included.
func TestExport_FacedRenderOverrideReceivesTheAddress(t *testing.T) {
	app, _ := facedExportApp(t, nil)
	withRenderOverride(t, app, "policy", func(entryID string) string { return "# Fancy " + entryID + "\n" })

	tests := []struct{ addr, want string }{
		{"policys/POL-1@draft", "# Fancy POL-1@draft"},
		{"policys/POL-1@published", "# Fancy POL-1@published"},
		{"policys/POL-1", "# Fancy POL-1@published"},
	}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			rec := exportRouted(t, app, tc.addr)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
			}
			if body := rec.Body.String(); strings.TrimSpace(body) != tc.want {
				t.Errorf("override rendered %q, want %q", body, tc.want)
			}
		})
	}
}

// TestDocument_FacedAnchorRendersThatFace: the anchored document route
// accepts an address (BUG-VFHUWO), and the document service must then read
// the entry by that address rather than as a bare id.
func TestDocument_FacedAnchorRendersThatFace(t *testing.T) {
	app, _ := facedExportApp(t, nil)
	withFacedReportDoc(t, app)

	for _, path := range []string{
		"/api/v1/_documents/report/POL-1@published",
		"/api/v1/_documents/report/POL-1@published/_export?transform=copy",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "report POL-1@published") {
				t.Errorf("body = %s, want the published face's render", rec.Body)
			}
		})
	}
}

// withFacedReportDoc declares the anchored document `report` on policy and
// renders it through a fake engine that echoes the entry address, so a test
// can see which face was rendered and whether the renderer ran at all.
func withFacedReportDoc(t *testing.T, app *App) *fakeScriptEngine {
	t.Helper()
	s := app.State()
	cfg := *s.Cfg
	cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"report": {EntityType: "policy", Script: "docs/r.lua"},
	}
	app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta})
	fake := &fakeScriptEngine{stdout: func(c fakeScriptCall) string { return "# report " + c.entryID }}
	app.documents = newDocumentService(app.store, app.kv, "/", fake,
		func() lua.WriteDeps { return lua.WriteDeps{} }, nil)
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	return fake
}
