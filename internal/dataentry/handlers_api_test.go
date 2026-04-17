package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestHandleAPIPaletteCRUD(t *testing.T) {
	app := newHandlerTestApp(t)
	app.State().Palette = ResolvePalette(nil, nil)

	t.Run("GET returns empty palette when none set", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_palette", http.NoBody)
		w := httptest.NewRecorder()
		app.handleAPIPaletteCRUD(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("PUT saves valid palette", func(t *testing.T) {
		body := `{"accent":"#e11d48","badges":{"blue":"#1e40af"}}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/_palette", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Need to hold RLock to simulate reloadLockMiddleware
		app.handleAPISavePalette(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if app.State().UserPalette == nil || app.State().UserPalette.Accent != "#e11d48" {
			t.Error("palette not saved")
		}
	})

	t.Run("PUT rejects invalid palette", func(t *testing.T) {
		body := `{"accent":"not-hex"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/_palette", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		app.handleAPISavePalette(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/_palette", http.NoBody)
		w := httptest.NewRecorder()
		app.handleAPIPaletteCRUD(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result)
	}
}

func TestLoadUserPalette(t *testing.T) {
	t.Run("missing file returns nil with no error", func(t *testing.T) {
		app := newHandlerTestApp(t)
		p, err := app.loadUserPalette()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p != nil {
			t.Errorf("expected nil palette for missing file, got %+v", p)
		}
	})

	t.Run("malformed YAML returns error (legacy dark: auto)", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Write a legacy palette.yaml that the new two-state DarkMode
		// can't parse. Without the error path, the loader would
		// return nil and a subsequent save would silently overwrite
		// the user's palette with framework defaults.
		err := app.ws.WriteCacheFile(userPaletteFile, []byte("accent: '#e11d48'\ndark: auto\n"))
		if err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		p, perr := app.loadUserPalette()
		if perr == nil {
			t.Fatal("expected error for legacy `dark: auto`, got nil")
		}
		if p != nil {
			t.Errorf("expected nil palette on parse error, got %+v", p)
		}
		if !strings.Contains(perr.Error(), "dark: auto") {
			t.Errorf("error should mention legacy dark: auto migration, got: %v", perr)
		}
	})

	t.Run("valid file parses successfully", func(t *testing.T) {
		app := newHandlerTestApp(t)
		err := app.ws.WriteCacheFile(userPaletteFile, []byte("accent: '#e11d48'\ndark: false\n"))
		if err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		p, perr := app.loadUserPalette()
		if perr != nil {
			t.Fatalf("unexpected error: %v", perr)
		}
		if p == nil || p.Accent != "#e11d48" {
			t.Errorf("expected accent #e11d48, got %+v", p)
		}
		if !p.Dark.IsDisabled() {
			t.Errorf("expected dark disabled, got %+v", p.Dark)
		}
	})
}

func TestWriteJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONError(w, http.StatusNotFound, "not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "not found" {
		t.Errorf("expected error=not found, got %v", result)
	}
}

func TestEntityToAPI_WithoutRelations(t *testing.T) {
	app, _ := testAppInstance()
	e := &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test ticket",
			"status": "open",
		},
	}

	result := app.entityToAPI(e, false)

	if result.ID != "TKT-001" {
		t.Errorf("expected ID TKT-001, got %q", result.ID)
	}
	if result.Type != "ticket" {
		t.Errorf("expected type ticket, got %q", result.Type)
	}
	if result.Properties["title"] != "Test ticket" {
		t.Errorf("expected title 'Test ticket', got %v", result.Properties["title"])
	}
	if result.Relations != nil {
		t.Errorf("expected nil relations, got %v", result.Relations)
	}
}

func TestEntityToAPI_WithRelations(t *testing.T) {
	app, entities := testAppInstance()

	// Add a relation to test graph
	graphForTest(app).AddEdge(model.NewRelation(entities.ticket1.ID, "depends_on", entities.ticket2.ID))

	result := app.entityToAPI(model.EntityToDomain(entities.ticket1), true)

	if len(result.Relations) == 0 {
		t.Fatal("expected relations, got none")
	}

	hasOutgoing := false
	for _, r := range result.Relations {
		if r.Direction == DirectionOutgoing && r.Type == "depends_on" && r.To == entities.ticket2.ID {
			hasOutgoing = true
		}
	}
	if !hasOutgoing {
		t.Errorf("expected outgoing depends_on relation to %s", entities.ticket2.ID)
	}

	// Check incoming from perspective of TKT-002
	result2 := app.entityToAPI(model.EntityToDomain(entities.ticket2), true)
	hasIncoming := false
	for _, r := range result2.Relations {
		if r.Direction == DirectionIncoming && r.Type == "depends_on" && r.From == entities.ticket1.ID {
			hasIncoming = true
		}
	}
	if !hasIncoming {
		t.Errorf("expected incoming depends_on relation from %s", entities.ticket1.ID)
	}
}

func TestHandleAPIEntityTypes(t *testing.T) {
	app, _ := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/entity-types", http.NoBody)

	app.handleAPIEntityTypes(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var types []APIEntityType
	if err := json.Unmarshal(w.Body.Bytes(), &types); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(types) == 0 {
		t.Error("expected entity types, got none")
	}

	found := false
	for _, typ := range types {
		if typ.Name == "ticket" {
			found = true
			if typ.Properties["title"].Type != "string" {
				t.Errorf("expected title type string, got %q", typ.Properties["title"].Type)
			}
		}
	}
	if !found {
		t.Error("expected to find ticket entity type")
	}
}

func TestHandleAPIEntities(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("all entities", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities", http.NoBody)
		app.handleAPIEntities(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var entities []APIEntity
		if err := json.Unmarshal(w.Body.Bytes(), &entities); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(entities) == 0 {
			t.Error("expected entities, got none")
		}
	})

	t.Run("filtered by type", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities?type=ticket", http.NoBody)
		app.handleAPIEntities(w, r)

		var entities []APIEntity
		if err := json.Unmarshal(w.Body.Bytes(), &entities); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		for _, e := range entities {
			if e.Type != "ticket" {
				t.Errorf("expected type ticket, got %q", e.Type)
			}
		}
	})
}

func TestHandleAPIEntity(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities/TKT-001", http.NoBody)
		app.handleAPIEntity(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var entity APIEntity
		if err := json.Unmarshal(w.Body.Bytes(), &entity); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if entity.ID != "TKT-001" {
			t.Errorf("expected TKT-001, got %q", entity.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities/NONEXISTENT", http.NoBody)
		app.handleAPIEntity(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities/", http.NoBody)
		app.handleAPIEntity(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleAPIMetamodel(t *testing.T) {
	app, _ := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/metamodel", http.NoBody)

	app.handleAPIMetamodel(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var mm APIMetamodel
	if err := json.Unmarshal(w.Body.Bytes(), &mm); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(mm.EntityTypes) == 0 {
		t.Error("expected entity types")
	}
	if len(mm.RelationTypes) == 0 {
		t.Error("expected relation types")
	}
}

func TestHandleAPIEntitiesCRUD_MethodNotAllowed(t *testing.T) {
	app, _ := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/entities", http.NoBody)

	app.handleAPIEntitiesCRUD(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleAPIEntityCRUD_MethodNotAllowed(t *testing.T) {
	app, _ := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/entities/TKT-001", http.NoBody)

	app.handleAPIEntityCRUD(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleAPIRelationsCRUD_MethodNotAllowed(t *testing.T) {
	app, _ := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/api/relations", http.NoBody)

	app.handleAPIRelationsCRUD(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleEntityHelp(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("valid entity type", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/help/ticket", http.NoBody)
		app.handleEntityHelp(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "help-content") {
			t.Error("expected help-content class in response")
		}
		// Should contain properties section
		if !strings.Contains(body, "Properties") {
			t.Error("expected Properties section in response")
		}
		// Should contain title property (required)
		if !strings.Contains(body, "title") {
			t.Error("expected title property in response")
		}
		// Should contain outgoing relations
		if !strings.Contains(body, "Outgoing Relations") {
			t.Error("expected Outgoing Relations section in response")
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/help/nonexistent", http.NoBody)
		app.handleEntityHelp(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("missing entity type", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/help/", http.NoBody)
		app.handleEntityHelp(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestFormatCardinality(t *testing.T) {
	tests := []struct {
		name     string
		min, max *int
		want     string
	}{
		{"nil both", nil, nil, ""},
		{"min only zero", intPtr(0), nil, ""},
		{"min only nonzero", intPtr(1), nil, "min 1"},
		{"max only", nil, intPtr(5), "max 5"},
		{"exact", intPtr(3), intPtr(3), "exactly 3"},
		{"range", intPtr(1), intPtr(5), "1-5"},
		{"zero to max", intPtr(0), intPtr(3), "max 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCardinality(tt.min, tt.max)
			if got != tt.want {
				t.Errorf("formatCardinality(%v, %v) = %q, want %q", tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int { return &i }
