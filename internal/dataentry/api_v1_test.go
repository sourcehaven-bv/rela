package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestV1SchemaEndpoint(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Schema(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var schema V1Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(schema.Entities) != 2 {
		t.Errorf("expected 2 entity types, got %d", len(schema.Entities))
	}

	if _, ok := schema.Entities["ticket"]; !ok {
		t.Error("expected 'ticket' entity type in schema")
	}
}

func TestV1ConfigEndpoint(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Config(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var config V1Config
	if err := json.NewDecoder(rec.Body).Decode(&config); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if config.App.Name != "Test App" {
		t.Errorf("expected app name 'Test App', got %q", config.App.Name)
	}
}

func TestV1ListEntities(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entity
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 entity, got %d", len(resp.Data))
	}

	if resp.Data[0].ID != "TKT-001" {
		t.Errorf("expected entity ID 'TKT-001', got %q", resp.Data[0].ID)
	}

	if resp.Meta.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Meta.Total)
	}

	// Check pagination headers
	if rec.Header().Get("X-Total-Count") != "1" {
		t.Errorf("expected X-Total-Count '1', got %q", rec.Header().Get("X-Total-Count"))
	}
}

func TestV1GetEntity(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entity
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
		Content: "Test content",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if entity.ID != "TKT-001" {
		t.Errorf("expected ID 'TKT-001', got %q", entity.ID)
	}

	if entity.Properties["title"] != "Test Ticket" {
		t.Errorf("expected title 'Test Ticket', got %v", entity.Properties["title"])
	}

	// Check ETag header
	if rec.Header().Get("ETag") == "" {
		t.Error("expected ETag header to be set")
	}
}

func TestV1GetEntityNotFound(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/NONEXISTENT", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "NONEXISTENT")
	app.mu.RUnlock()

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	// Should be RFC 7807 Problem Details
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q",
			rec.Header().Get("Content-Type"))
	}
}

func TestV1DynamicRouting(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entity
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	tests := []struct {
		path           string
		expectedStatus int
	}{
		{"/api/v1/tickets", http.StatusOK},
		{"/api/v1/tickets/TKT-001", http.StatusOK},
		{"/api/v1/unknown", http.StatusNotFound},
		{"/api/v1/_unknown", http.StatusNotFound}, // System endpoint doesn't exist
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			app.mu.RLock()
			app.handleV1DynamicRoutes(rec, req)
			app.mu.RUnlock()

			if rec.Code != tc.expectedStatus {
				t.Errorf("path %s: expected status %d, got %d", tc.path, tc.expectedStatus, rec.Code)
			}
		})
	}
}

func TestV1Filtering(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entities
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Open Ticket",
			"status": "open",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Closed Ticket",
			"status": "closed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[status]=open", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 filtered entity, got %d", len(resp.Data))
	}

	if resp.Data[0].ID != "TKT-001" {
		t.Errorf("expected filtered entity 'TKT-001', got %q", resp.Data[0].ID)
	}
}

func TestV1FilteringNEMultipleValues(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entities with various statuses
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Open Ticket",
			"status": "open",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Completed Ticket",
			"status": "completed",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Superseded Ticket",
			"status": "superseded",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-004",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "In Progress Ticket",
			"status": "in_progress",
		},
	})

	// Test filtering with ne operator and comma-separated values (NOT IN semantics)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[status][ne]=completed,superseded", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return only TKT-001 (open) and TKT-004 (in_progress), excluding completed and superseded
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 filtered entities, got %d", len(resp.Data))
	}

	ids := make(map[string]bool)
	for _, e := range resp.Data {
		ids[e.ID] = true
	}

	if !ids["TKT-001"] {
		t.Errorf("expected TKT-001 (open) to be in results")
	}
	if !ids["TKT-004"] {
		t.Errorf("expected TKT-004 (in_progress) to be in results")
	}
	if ids["TKT-002"] {
		t.Errorf("TKT-002 (completed) should have been filtered out")
	}
	if ids["TKT-003"] {
		t.Errorf("TKT-003 (superseded) should have been filtered out")
	}
}

func TestV1Sorting(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entities
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "B Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "A Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=title", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(resp.Data))
	}

	// Should be sorted ascending by title
	if resp.Data[0].ID != "TKT-002" {
		t.Errorf("expected first entity 'TKT-002' (A Ticket), got %q", resp.Data[0].ID)
	}
}

func TestV1Pagination(t *testing.T) {
	app := newTestAppV1(t)

	// Add multiple entities
	for i := 1; i <= 30; i++ {
		app.g.AddNode(&model.Entity{
			ID:   "TKT-" + padInt(i),
			Type: "ticket",
			Properties: map[string]interface{}{
				"title": "Ticket " + padInt(i),
			},
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?page=2&per_page=10", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.Total != 30 {
		t.Errorf("expected total 30, got %d", resp.Meta.Total)
	}

	if resp.Meta.Page != 2 {
		t.Errorf("expected page 2, got %d", resp.Meta.Page)
	}

	if len(resp.Data) != 10 {
		t.Errorf("expected 10 entities on page, got %d", len(resp.Data))
	}

	if resp.Meta.HasMore != true {
		t.Error("expected HasMore to be true")
	}

	// Check Link header
	link := rec.Header().Get("Link")
	if link == "" {
		t.Error("expected Link header to be set")
	}
}

func padInt(i int) string {
	if i < 10 {
		return "00" + string(rune('0'+i))
	}
	if i < 100 {
		return "0" + string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	return string(rune('0'+i/100)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
}

func newTestAppV1(t *testing.T) *App {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
			},
			"feature": {
				Label: "Feature",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "implements",
				From:  []string{"ticket"},
				To:    []string{"feature"},
			},
		},
	}

	g := graph.New()

	cfg := &dataentryconfig.Config{
		App: dataentryconfig.AppConfig{
			Name:        "Test App",
			Description: "Test Description",
		},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}

	return &App{
		meta: meta,
		g:    g,
		Cfg:  cfg,
	}
}
