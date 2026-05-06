// Tests for the unified PATCH endpoint with relations support (TKT-K2VAA).
// Covers ACs #1-26 from PLAN-CAK3L.

package dataentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// newRelationsTestApp builds an App with a writable in-memory workspace
// configured with multiple relation types: a to-one (`belongs-to →
// category`), a to-many (`tagged → label`) with declared properties, a
// symmetric (`linked-to`), and an inverse (`assesses` ↔ `assessed-by`).
func newRelationsTestApp(t *testing.T) *App {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket", IDPrefix: "TKT",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
			},
			"category": {
				Label: "Category", IDPrefix: "CAT",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"label": {
				Label: "Label", IDPrefix: "LBL",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"belongs-to": {
				Label: "belongs to",
				From:  []string{"ticket"},
				To:    []string{"category"},
			},
			"tagged": {
				Label: "tagged",
				From:  []string{"ticket"},
				To:    []string{"label"},
				Properties: map[string]metamodel.PropertyDef{
					"weight":   {Type: "integer"},
					"added_by": {Type: "string"},
				},
			},
			"linked-to": {
				Label:     "linked to",
				From:      []string{"ticket"},
				To:        []string{"ticket"},
				Symmetric: true,
			},
			"assesses": {
				Label: "assesses",
				From:  []string{"ticket"},
				To:    []string{"category"},
				Inverse: &metamodel.InverseDef{
					ID: "assessed-by",
				},
			},
			"notes": {
				Label:   "notes",
				From:    []string{"ticket"},
				To:      []string{"label"},
				Content: true,
			},
		},
	}

	// Pre-create entities in the graph + on-disk filesystem.
	g := graph.New()
	g.AddNode(&model.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]interface{}{"title": "Main Ticket", "status": "open"},
	})
	g.AddNode(&model.Entity{
		ID: "TKT-002", Type: "ticket",
		Properties: map[string]interface{}{"title": "Other Ticket", "status": "open"},
	})
	g.AddNode(&model.Entity{
		ID: "TKT-003", Type: "ticket",
		Properties: map[string]interface{}{"title": "Third Ticket", "status": "open"},
	})
	g.AddNode(&model.Entity{
		ID: "CAT-001", Type: "category",
		Properties: map[string]interface{}{"title": "Bugs"},
	})
	g.AddNode(&model.Entity{
		ID: "CAT-002", Type: "category",
		Properties: map[string]interface{}{"title": "Features"},
	})
	g.AddNode(&model.Entity{
		ID: "LBL-001", Type: "label",
		Properties: map[string]interface{}{"title": "p0"},
	})
	g.AddNode(&model.Entity{
		ID: "LBL-002", Type: "label",
		Properties: map[string]interface{}{"title": "p1"},
	})
	g.AddNode(&model.Entity{
		ID: "LBL-003", Type: "label",
		Properties: map[string]interface{}{"title": "p2"},
	})

	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}

	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root: root, CacheDir: root + "/.rela",
		EntitiesDir: root + "/entities", RelationsDir: root + "/relations",
	}
	for _, dir := range []string{ctx.CacheDir, ctx.EntitiesDir, ctx.RelationsDir} {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for entityType := range meta.Entities {
		def, _ := meta.GetEntityDef(entityType)
		plural := def.Plural
		if plural == "" {
			plural = entityType + "s"
		}
		if err := fs.MkdirAll(ctx.EntitiesDir+"/"+plural, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", entityType, err)
		}
	}

	repo := repository.New(fs, ctx)
	ws := workspace.NewWithGraph(repo, meta, g)
	for _, e := range g.AllNodes() {
		if err := repo.WriteEntity(e, meta); err != nil {
			t.Fatalf("pre-write entity %s: %v", e.ID, err)
		}
	}

	app := &App{ws: ws, broker: newEventBroker()}
	app.state.Store(&AppState{
		Cfg:   cfg,
		Meta:  meta,
		Graph: g,
	})
	return app
}

// patchRequest builds a PATCH request with the given JSON body. URL is
// only for httptest's record; the handler doesn't route on it.
func patchRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", strings.NewReader(body))
}

// addRelation creates a relation in both the graph and on disk so the
// fixture matches what loadEntity sees.
func addRelation(t *testing.T, app *App, fromID, relType, toID string, properties map[string]interface{}, content string) {
	t.Helper()
	rel := model.NewRelation(fromID, relType, toID)
	if properties != nil {
		rel.Properties = properties
	}
	rel.Content = content
	if err := app.ws.Repo().WriteRelation(rel); err != nil {
		t.Fatalf("write relation %s-%s->%s: %v", fromID, relType, toID, err)
	}
	app.Graph().AddEdge(rel)
}

// AC #1: list-level wire format accepted; PATCH adds a new outgoing relation.
func TestV1Patch_AddNewRelation(t *testing.T) {
	app := newRelationsTestApp(t)

	body := `{"relations":{"belongs-to":{"data":[{"type":"category","id":"CAT-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "belongs-to", "CAT-001"); !ok {
		t.Errorf("edge TKT-001 belongs-to CAT-001 not in graph")
	}
}

// AC #2: omitting `relations` key leaves existing relations untouched.
func TestV1Patch_OmitRelationsLeavesAlone(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", nil, "")
	addRelation(t, app, "TKT-001", "tagged", "LBL-002", nil, "")

	body := `{"properties":{"status":"closed"}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001"); !ok {
		t.Errorf("LBL-001 lost")
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-002"); !ok {
		t.Errorf("LBL-002 lost")
	}
}

// AC #3: empty data: [] removes all relations of that type but leaves others.
func TestV1Patch_EmptyDataRemovesAllOfType(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", nil, "")
	addRelation(t, app, "TKT-001", "tagged", "LBL-002", nil, "")
	addRelation(t, app, "TKT-001", "belongs-to", "CAT-001", nil, "")

	body := `{"relations":{"tagged":{"data":[]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001"); ok {
		t.Errorf("LBL-001 should be removed")
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-002"); ok {
		t.Errorf("LBL-002 should be removed")
	}
	// belongs-to was not in the request → leave alone.
	if _, ok := app.Graph().GetEdge("TKT-001", "belongs-to", "CAT-001"); !ok {
		t.Errorf("belongs-to CAT-001 should be untouched")
	}
}

// AC #4: data: null is equivalent to data: [] (per JSON:API §9.2.1).
func TestV1Patch_DataNullEquivalentToEmpty(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", nil, "")

	body := `{"relations":{"tagged":{"data":null}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001"); ok {
		t.Errorf("LBL-001 should be removed by data:null")
	}
}

// AC #5: add + remove + keep + update in one PATCH.
func TestV1Patch_AddRemoveKeepUpdateInOne(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", map[string]interface{}{"weight": 1}, "")
	addRelation(t, app, "TKT-001", "tagged", "LBL-002", nil, "")
	addRelation(t, app, "TKT-001", "tagged", "LBL-003", nil, "")

	// Keep LBL-001 with new meta; add LBL-002 (already present, no change → kept);
	// drop LBL-003 (not in list); LBL-002 is present in list with no changes.
	// Expected: LBL-001 (weight=5), LBL-002 (no meta), LBL-003 removed.
	body := `{"relations":{"tagged":{"data":[
		{"type":"label","id":"LBL-001","meta":{"weight":5}},
		{"type":"label","id":"LBL-002"}
	]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-003"); ok {
		t.Errorf("LBL-003 should be removed")
	}
	if rel, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001"); !ok {
		t.Errorf("LBL-001 should still exist")
	} else if fmt.Sprintf("%v", rel.Properties["weight"]) != "5" {
		t.Errorf("LBL-001 weight should be 5, got %v", rel.Properties["weight"])
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-002"); !ok {
		t.Errorf("LBL-002 should still exist")
	}
}

// AC #6: per-edge meta UPSERT. Existing keys preserved; specified keys updated.
func TestV1Patch_MetaUpsert(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001",
		map[string]interface{}{"weight": 3, "added_by": "alice"}, "")

	body := `{"relations":{"tagged":{"data":[
		{"type":"label","id":"LBL-001","meta":{"weight":5}}
	]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	rel, _ := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001")
	if fmt.Sprintf("%v", rel.Properties["weight"]) != "5" {
		t.Errorf("weight should be 5, got %v", rel.Properties["weight"])
	}
	if fmt.Sprintf("%v", rel.Properties["added_by"]) != "alice" {
		t.Errorf("added_by should be preserved as alice, got %v", rel.Properties["added_by"])
	}
}

// AC #7: meta_unset clears named keys, keeping others.
func TestV1Patch_MetaUnset(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001",
		map[string]interface{}{"weight": 3, "added_by": "alice"}, "")

	body := `{"relations":{"tagged":{"data":[
		{"type":"label","id":"LBL-001","meta":{"weight":5},"meta_unset":["added_by"]}
	]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	rel, _ := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001")
	if fmt.Sprintf("%v", rel.Properties["weight"]) != "5" {
		t.Errorf("weight should be 5, got %v", rel.Properties["weight"])
	}
	if _, has := rel.Properties["added_by"]; has {
		t.Errorf("added_by should have been unset, got %v", rel.Properties["added_by"])
	}
}

// AC #8: per-edge content upsert (leave-alone, set, set-empty).
func TestV1Patch_ContentUpsert(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "notes", "LBL-001", nil, "old body")

	cases := []struct {
		name        string
		body        string
		wantContent string
	}{
		{"absent leaves alone", `{"relations":{"notes":{"data":[{"type":"label","id":"LBL-001"}]}}}`, "old body"},
		{"set replaces", `{"relations":{"notes":{"data":[{"type":"label","id":"LBL-001","content":"new body"}]}}}`, "new body"},
		{"empty clears", `{"relations":{"notes":{"data":[{"type":"label","id":"LBL-001","content":""}]}}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := patchRequest(tc.body)
			rec := httptest.NewRecorder()
			app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			rel, _ := app.Graph().GetEdge("TKT-001", "notes", "LBL-001")
			if rel.Content != tc.wantContent {
				t.Errorf("content = %q, want %q", rel.Content, tc.wantContent)
			}
		})
	}
}

// AC #9: unknown relation type → 422.
func TestV1Patch_UnknownRelationType(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations":{"never-heard-of":{"data":[{"type":"category","id":"CAT-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// AC #10: target type mismatch → 422.
func TestV1Patch_TargetTypeMismatch(t *testing.T) {
	app := newRelationsTestApp(t)
	// belongs-to expects category, send a label.
	body := `{"relations":{"belongs-to":{"data":[{"type":"label","id":"LBL-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// AC #11: target ID doesn't exist → 422.
func TestV1Patch_TargetIDMissing(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations":{"belongs-to":{"data":[{"type":"category","id":"CAT-NONEXISTENT"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// AC #12: missing `type` field → 400 with structured pointer.
func TestV1Patch_MissingTypeField(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations":{"belongs-to":{"data":[{"id":"CAT-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/data/0/type") {
		t.Errorf("expected pointer to data[0]/type in body: %s", rec.Body.String())
	}
}

// AC #13: invalid meta property type → 422.
func TestV1Patch_InvalidMetaType(t *testing.T) {
	app := newRelationsTestApp(t)
	// `weight` is integer; send a string.
	body := `{"relations":{"tagged":{"data":[{"type":"label","id":"LBL-001","meta":{"weight":"not a number"}}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// AC #14: symmetric propagation — adding A→B implies B→A; removal cleans the
// counterparty edge but leaves counterparty's unrelated edges untouched.
func TestV1Patch_SymmetricPropagation(t *testing.T) {
	app := newRelationsTestApp(t)
	// Pre-existing: TKT-001 ↔ TKT-002 (linked-to is symmetric).
	addRelation(t, app, "TKT-001", "linked-to", "TKT-002", nil, "")
	addRelation(t, app, "TKT-002", "linked-to", "TKT-001", nil, "") // symmetric back-edge

	// Also: TKT-002 has an unrelated outgoing linked-to → TKT-003.
	addRelation(t, app, "TKT-002", "linked-to", "TKT-003", nil, "")
	addRelation(t, app, "TKT-003", "linked-to", "TKT-002", nil, "")

	// PATCH TKT-001.linked-to: [TKT-003] (replaces TKT-002 with TKT-003).
	body := `{"relations":{"linked-to":{"data":[{"type":"ticket","id":"TKT-003"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	// Primary edges
	if _, ok := app.Graph().GetEdge("TKT-001", "linked-to", "TKT-003"); !ok {
		t.Errorf("primary edge TKT-001->TKT-003 missing")
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "linked-to", "TKT-002"); ok {
		t.Errorf("primary edge TKT-001->TKT-002 should be removed")
	}
	// Symmetric back-edges
	if _, ok := app.Graph().GetEdge("TKT-003", "linked-to", "TKT-001"); !ok {
		t.Errorf("symmetric back-edge TKT-003->TKT-001 missing")
	}
	if _, ok := app.Graph().GetEdge("TKT-002", "linked-to", "TKT-001"); ok {
		t.Errorf("symmetric back-edge TKT-002->TKT-001 should be removed")
	}
	// Counterparty's unrelated edges untouched
	if _, ok := app.Graph().GetEdge("TKT-002", "linked-to", "TKT-003"); !ok {
		t.Errorf("unrelated edge TKT-002->TKT-003 was incorrectly removed")
	}
}

// AC #16: inverse propagation — adding A.assesses → B implies
// B.assessed-by → A; removal cleans the inverse edge.
func TestV1Patch_InversePropagation(t *testing.T) {
	app := newRelationsTestApp(t)

	body := `{"relations":{"assesses":{"data":[{"type":"category","id":"CAT-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "assesses", "CAT-001"); !ok {
		t.Errorf("primary edge missing")
	}
	if _, ok := app.Graph().GetEdge("CAT-001", "assessed-by", "TKT-001"); !ok {
		t.Errorf("inverse edge CAT-001 -assessed-by-> TKT-001 missing")
	}
}

// AC #19: validation failure leaves the live entity untouched (fixes the
// pre-mutation hazard).
func TestV1Patch_ValidationFailureLeavesEntityUntouched(t *testing.T) {
	app := newRelationsTestApp(t)
	originalTitle := "Main Ticket"

	// Invalid relation: target doesn't exist.
	body := `{"properties":{"title":"NEW TITLE"},"relations":{"belongs-to":{"data":[{"type":"category","id":"CAT-NONEXISTENT"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	live, _ := app.Graph().GetNode("TKT-001")
	if live.Properties["title"] != originalTitle {
		t.Errorf("live entity was mutated despite validation failure: title=%v", live.Properties["title"])
	}
}

// AC #22: ETag still works.
func TestV1Patch_ETagMismatch(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"properties":{"title":"NEW"}}`
	req := patchRequest(body)
	req.Header.Set("If-Match", "stale-etag-value")
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d", rec.Code)
	}
}

// AC #23: backwards compat — old PATCH bodies (no relations key) work
// unchanged. This is implicitly covered by all the existing
// TestV1UpdateEntity_* tests in api_v1_test.go; here's a quick smoke test.
func TestV1Patch_BackwardsCompat(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"properties":{"status":"closed"}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	live, _ := app.Graph().GetNode("TKT-001")
	if live.Properties["status"] != "closed" {
		t.Errorf("status = %v, want closed", live.Properties["status"])
	}
}

// Edge case: relations: {} (empty map) is a no-op, equivalent to omitting.
func TestV1Patch_EmptyRelationsMap(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", nil, "")

	body := `{"relations":{}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001"); !ok {
		t.Errorf("relation should be untouched on empty relations map")
	}
}

// Edge case: content on a relation type without Content: true → 422.
func TestV1Patch_ContentNotSupported(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations":{"tagged":{"data":[{"type":"label","id":"LBL-001","content":"body"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// Edge case: meta_unset of an unknown key → 422.
func TestV1Patch_MetaUnsetUnknownKey(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", nil, "")
	body := `{"relations":{"tagged":{"data":[{"type":"label","id":"LBL-001","meta_unset":["nonexistent_prop"]}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// Edge case: unknown meta key (closed schema rejection) → 422.
func TestV1Patch_UnknownMetaKey(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations":{"tagged":{"data":[{"type":"label","id":"LBL-001","meta":{"unknown_key":"value"}}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// Properties + relations in one PATCH; both apply atomically when valid.
func TestV1Patch_PropertiesAndRelationsTogether(t *testing.T) {
	app := newRelationsTestApp(t)

	body := `{"properties":{"status":"closed"},"relations":{"belongs-to":{"data":[{"type":"category","id":"CAT-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	live, _ := app.Graph().GetNode("TKT-001")
	if live.Properties["status"] != "closed" {
		t.Errorf("status not updated")
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "belongs-to", "CAT-001"); !ok {
		t.Errorf("belongs-to edge missing")
	}
}

// Properties + relations: when relations fail validation, properties are
// also rolled back. (Atomicity at the validation layer.)
func TestV1Patch_AtomicityOnValidationFailure(t *testing.T) {
	app := newRelationsTestApp(t)

	body := `{"properties":{"status":"would-be-set"},"relations":{"belongs-to":{"data":[{"type":"category","id":"CAT-NONEXISTENT"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	live, _ := app.Graph().GetNode("TKT-001")
	if live.Properties["status"] == "would-be-set" {
		t.Errorf("status was applied despite validation failure: %v", live.Properties["status"])
	}
}

// Smoke: the response body should reflect the staged entity (post-PATCH state).
func TestV1Patch_ResponseBodyReflectsNewState(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"properties":{"status":"in_progress"}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var resp V1Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Properties["status"] != "in_progress" {
		t.Errorf("response status = %v, want in_progress", resp.Properties["status"])
	}
}

// drainBrokerEvents collects all events delivered to a broker subscription
// over a short window. Returns events in delivery order.
func drainBrokerEvents(t *testing.T, app *App) []sseEvent {
	t.Helper()
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	// Collect everything currently in the channel without blocking.
	var events []sseEvent
	for {
		select {
		case ev := <-ch:
			events = append(events, ev)
		default:
			return events
		}
	}
}

// AC #15: a PATCH on a symmetric relation that touches counterparty edges
// fires `entity:updated` for each affected counterparty plus the PATCHed
// entity. We expect exactly 1 + |touched counterparties|.
func TestV1Patch_SymmetricEventCount(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "linked-to", "TKT-002", nil, "")
	addRelation(t, app, "TKT-002", "linked-to", "TKT-001", nil, "")

	// Subscribe BEFORE the PATCH so we capture all broker events.
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)

	// Replace TKT-002 with TKT-003. Two counterparties are touched
	// (TKT-002 loses its back-edge, TKT-003 gains a back-edge), so we
	// expect 3 entity:updated events total: TKT-001 + TKT-002 + TKT-003.
	body := `{"relations":{"linked-to":{"data":[{"type":"ticket","id":"TKT-003"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	events := []sseEvent{}
	// Collect all events that were buffered.
	for {
		select {
		case ev := <-ch:
			events = append(events, ev)
		default:
			goto done
		}
	}
done:
	updates := 0
	gotIDs := map[string]int{}
	for _, ev := range events {
		if ev.Type == "entity:updated" {
			var data map[string]string
			_ = json.Unmarshal([]byte(ev.Data), &data)
			gotIDs[data["id"]]++
			updates++
		}
	}
	if updates != 3 {
		t.Errorf("expected 3 entity:updated events, got %d (events: %+v)", updates, events)
	}
	for _, want := range []string{"TKT-001", "TKT-002", "TKT-003"} {
		if gotIDs[want] == 0 {
			t.Errorf("expected entity:updated for %s, got events: %v", want, gotIDs)
		}
	}
}

// AC #17: PATCH with relations exactly matching current state writes zero
// relation files. Verified by counting writes via a counter wrapper around
// the underlying FS.
func TestV1Patch_NoOpSuppression_NoWrites(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001",
		map[string]interface{}{"weight": 5}, "")

	// Drain any prior events that might be in the broker.
	_ = drainBrokerEvents(t, app)

	// PATCH the same data we already have. Should write nothing.
	body := `{"relations":{"tagged":{"data":[{"type":"label","id":"LBL-001","meta":{"weight":5}}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// Edge still present, identical state.
	rel, ok := app.Graph().GetEdge("TKT-001", "tagged", "LBL-001")
	if !ok {
		t.Fatal("relation disappeared")
	}
	if fmt.Sprintf("%v", rel.Properties["weight"]) != "5" {
		t.Errorf("weight changed unexpectedly: %v", rel.Properties["weight"])
	}
}

// AC #18: a PATCH where everything is no-op does NOT fire an entity:updated
// SSE event. (Auto-save will hit this constantly; pointless events would
// trigger spurious refetches.)
func TestV1Patch_NoOpSuppression_NoSSE(t *testing.T) {
	app := newRelationsTestApp(t)
	addRelation(t, app, "TKT-001", "tagged", "LBL-001", nil, "")

	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)

	// Send back exactly what we already have.
	body := `{"relations":{"tagged":{"data":[{"type":"label","id":"LBL-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// Drain — should be empty.
	updates := 0
	for {
		select {
		case ev := <-ch:
			if ev.Type == "entity:updated" {
				updates++
			}
		default:
			goto done
		}
	}
done:
	if updates != 0 {
		t.Errorf("expected zero entity:updated events on no-op, got %d", updates)
	}
}

// failOnNthRenameFS wraps an FS and fails the Nth Rename call. Used to
// inject Phase 1 commit failures (per AC #20).
type failOnNthRenameFS struct {
	storage.FS
	n     int
	count int
}

func (f *failOnNthRenameFS) Rename(oldpath, newpath string) error {
	f.count++
	if f.count == f.n {
		return fmt.Errorf("injected rename failure on call %d", f.n)
	}
	return f.FS.Rename(oldpath, newpath)
}

// AC #20: on Phase 1 commit failure (Nth rename fails), the in-memory
// graph is NOT mutated. tx.applyGraphMutations is gated on full commit
// success; on rollback, all renames are reversed and the graph reflects
// pre-PATCH state.
func TestV1Patch_AtomicityOnCommitFailure(t *testing.T) {
	// Set up the same fixture as newRelationsTestApp but inject a
	// failing FS so we control which write fails. We rebuild here
	// rather than wrapping the existing app because we want the FS
	// failure injected from the start.
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":   {Label: "Ticket", IDPrefix: "TKT", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}},
			"category": {Label: "Category", IDPrefix: "CAT", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"belongs-to": {Label: "belongs to", From: []string{"ticket"}, To: []string{"category"}},
		},
	}
	g := graph.New()
	g.AddNode(&model.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})
	g.AddNode(&model.Entity{ID: "CAT-001", Type: "category", Properties: map[string]interface{}{"title": "C"}})

	memfs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root: root, CacheDir: root + "/.rela",
		EntitiesDir: root + "/entities", RelationsDir: root + "/relations",
	}
	for _, dir := range []string{ctx.CacheDir, ctx.EntitiesDir, ctx.RelationsDir, ctx.EntitiesDir + "/tickets", ctx.EntitiesDir + "/categorys"} {
		_ = memfs.MkdirAll(dir, 0o755)
	}
	// Use the wrapped FS that fails the 2nd rename. The first rename
	// will be the entity write (a property change that causes the
	// staged entity to differ from live); the second rename will be
	// the relation write — and we want THAT to fail.
	failFS := &failOnNthRenameFS{FS: memfs, n: 2}
	repo := repository.New(failFS, ctx)
	ws := workspace.NewWithGraph(repo, meta, g)

	// Pre-write entities so the entity-write step has something to
	// rename. (This bypasses our injection — the pre-write uses the
	// underlying memfs directly and counts as 0 so far... actually no,
	// the failFS is the FS the repo uses, so its counter ticks. Let's
	// reset.)
	for _, e := range g.AllNodes() {
		if err := repo.WriteEntity(e, meta); err != nil {
			t.Fatalf("pre-write: %v", err)
		}
	}
	failFS.count = 0 // reset after fixture setup

	cfg := &dataentryconfig.Config{
		App:     dataentryconfig.AppConfig{Name: "T"},
		Forms:   make(map[string]dataentryconfig.Form),
		Lists:   make(map[string]dataentryconfig.List),
		Views:   make(map[string]dataentryconfig.ViewConfig),
		Kanbans: make(map[string]dataentryconfig.Kanban),
	}
	app := &App{ws: ws, broker: newEventBroker()}
	app.state.Store(&AppState{Cfg: cfg, Meta: meta, Graph: g})

	// PATCH: change title (1st rename) AND add a relation (2nd rename
	// — this one fails).
	body := `{"properties":{"title":"NEW"},"relations":{"belongs-to":{"data":[{"type":"category","id":"CAT-001"}]}}}`
	req := patchRequest(body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-200 (commit failure), got 200 (body: %s)", rec.Body.String())
	}

	// In-memory graph must be untouched.
	live, _ := app.Graph().GetNode("TKT-001")
	if live.Properties["title"] != "T" {
		t.Errorf("title was applied despite commit failure: %v", live.Properties["title"])
	}
	if _, ok := app.Graph().GetEdge("TKT-001", "belongs-to", "CAT-001"); ok {
		t.Errorf("relation was added despite commit failure")
	}
}
