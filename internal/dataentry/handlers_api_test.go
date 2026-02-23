package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

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
	app := testAppInstance()
	e := &model.Entity{
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
	app := testAppInstance()

	// Add a relation to test graph
	app.g.AddEdge(model.NewRelation("TKT-001", "depends_on", "TKT-002"))

	entity, found := app.g.GetNode("TKT-001")
	if !found {
		t.Fatal("TKT-001 not found in test graph")
	}

	result := app.entityToAPI(entity, true)

	if len(result.Relations) == 0 {
		t.Fatal("expected relations, got none")
	}

	hasOutgoing := false
	for _, r := range result.Relations {
		if r.Direction == "outgoing" && r.Type == "depends_on" && r.To == "TKT-002" {
			hasOutgoing = true
		}
	}
	if !hasOutgoing {
		t.Error("expected outgoing depends_on relation to TKT-002")
	}

	// Check incoming from perspective of TKT-002
	entity2, _ := app.g.GetNode("TKT-002")
	result2 := app.entityToAPI(entity2, true)
	hasIncoming := false
	for _, r := range result2.Relations {
		if r.Direction == "incoming" && r.Type == "depends_on" && r.From == "TKT-001" {
			hasIncoming = true
		}
	}
	if !hasIncoming {
		t.Error("expected incoming depends_on relation from TKT-001")
	}
}

func TestHandleAPIEntityTypes(t *testing.T) {
	app := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/entity-types", nil)

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
	app := testAppInstance()

	t.Run("all entities", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
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
		r := httptest.NewRequest(http.MethodGet, "/api/entities?type=ticket", nil)
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
	app := testAppInstance()

	t.Run("found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities/TKT-001", nil)
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
		r := httptest.NewRequest(http.MethodGet, "/api/entities/NONEXISTENT", nil)
		app.handleAPIEntity(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/entities/", nil)
		app.handleAPIEntity(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleAPIMetamodel(t *testing.T) {
	app := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/metamodel", nil)

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
	app := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/entities", nil)

	app.handleAPIEntitiesCRUD(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleAPIEntityCRUD_MethodNotAllowed(t *testing.T) {
	app := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/entities/TKT-001", nil)

	app.handleAPIEntityCRUD(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleAPIRelationsCRUD_MethodNotAllowed(t *testing.T) {
	app := testAppInstance()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/api/relations", nil)

	app.handleAPIRelationsCRUD(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
