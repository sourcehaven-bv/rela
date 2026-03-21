package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestV1SchemaRoutes(t *testing.T) {
	app := newTestAppV1(t)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"list types", "/api/v1/_schema/types", http.StatusOK},
		{"get ticket type", "/api/v1/_schema/types/ticket", http.StatusOK},
		{"get feature type", "/api/v1/_schema/types/feature", http.StatusOK},
		{"get unknown type", "/api/v1/_schema/types/nonexistent", http.StatusNotFound},
		{"list relations", "/api/v1/_schema/relations", http.StatusOK},
		{"unknown route", "/api/v1/_schema/unknown", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			app.handleV1SchemaRoutes(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rec.Code)
			}
		})
	}
}

func TestV1SchemaTypesListReturnsNames(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema/types", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1SchemaRoutes(rec, req)

	var names []string
	if err := json.NewDecoder(rec.Body).Decode(&names); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(names) != 2 {
		t.Errorf("expected 2 types, got %d", len(names))
	}
}

func TestV1SearchEmptyQuery(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1Search(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(resp.Data))
	}
}

func TestV1SearchWithQuery(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entity
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Search Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=Search", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1Search(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1SearchWithTypeFilter(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=Test&type=ticket", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1Search(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should only return tickets, not features
	for _, e := range resp.Data {
		if e.Type != "ticket" {
			t.Errorf("expected all results to be tickets, got %s", e.Type)
		}
	}
}

func TestV1Analyze(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1AnalyzeMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_analyze", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1SchemaMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_schema", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Schema(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1ConfigMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_config", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Config(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1GetEntityWithIncludesAll(t *testing.T) {
	app := newTestAppV1(t)

	// Add entities with relations
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	app.g.AddEdge(&model.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=*", http.NoBody)
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

	if len(entity.Included) == 0 {
		t.Error("expected included entities for include=*")
	}
}

func TestV1GetEntityWithIncludesSpecific(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	app.g.AddEdge(&model.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=implements", http.NoBody)
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

	if _, ok := entity.Included["FEA-001"]; !ok {
		t.Error("expected FEA-001 in included entities")
	}
}

func TestV1GetEntityIfNoneMatch(t *testing.T) {
	app := newTestAppV1(t)

	entity := &model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	}
	app.g.AddNode(entity)

	// First request to get ETag
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	// Second request with If-None-Match
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	if rec.Code != http.StatusNotModified {
		t.Errorf("expected status 304, got %d", rec.Code)
	}
}

func TestV1GetEntityWithActions(t *testing.T) {
	app := newTestAppV1(t)

	// Set up status property with values
	app.meta.Entities["ticket"] = metamodel.EntityDef{
		Label: "Ticket",
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "string", Values: []string{"open", "in_progress", "closed"}},
		},
	}

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=_actions", http.NoBody)
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

	if entity.Actions == nil {
		t.Error("expected actions in response")
	}

	if entity.Actions != nil && entity.Actions.Delete == nil {
		t.Error("expected delete action status")
	}
}

func TestV1SingleEntityOptions(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1SingleEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}

	allow := rec.Header().Get("Allow")
	if allow == "" {
		t.Error("expected Allow header")
	}
}

func TestV1SingleEntityMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1SingleEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1ListEntitiesEmpty(t *testing.T) {
	app := newTestAppV1(t)

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

	if resp.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Meta.Total)
	}
}

func TestV1ListEntitiesDescendingSort(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "A Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "B Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=-title", http.NoBody)
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

	// Should be sorted descending by title (B first)
	if resp.Data[0].ID != "TKT-002" {
		t.Errorf("expected first entity 'TKT-002' (B Ticket), got %q", resp.Data[0].ID)
	}
}

func TestV1FilteringContains(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Bug Fix Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Feature Request",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[title][contains]=Bug", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 filtered entity, got %d", len(resp.Data))
	}

	if len(resp.Data) > 0 && resp.Data[0].ID != "TKT-001" {
		t.Errorf("expected TKT-001, got %s", resp.Data[0].ID)
	}
}

func TestV1FilteringIn(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "in_progress",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "closed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[status][in]=open,in_progress", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 filtered entities, got %d", len(resp.Data))
	}
}

func TestV1FilteringEmptyValue(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Has Title",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			// No status property
			"title": "No Status",
		},
	})

	// Filter for entities without status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[status]=", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1MultipleSort(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
			"title":  "B Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
			"title":  "A Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "closed",
			"title":  "C Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=status,title", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(resp.Data))
	}
}

func TestV1GetEntityWithNestedIncludes(t *testing.T) {
	app := newTestAppV1(t)

	// Add entities with relations
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-002",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Another Feature",
		},
	})
	app.g.AddEdge(&model.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})
	// Create another relation type for nested includes
	app.meta.Relations["requires"] = metamodel.RelationDef{
		Label: "requires",
		From:  []string{"feature"},
		To:    []string{"feature"},
	}
	app.g.AddEdge(&model.Relation{
		From: "FEA-001",
		To:   "FEA-002",
		Type: "requires",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=implements.requires", http.NoBody)
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

	// Should include both FEA-001 and FEA-002
	if _, ok := entity.Included["FEA-001"]; !ok {
		t.Error("expected FEA-001 in included entities")
	}
	if _, ok := entity.Included["FEA-002"]; !ok {
		t.Error("expected FEA-002 in nested included entities")
	}
}

func TestV1ComputeEntityActionsWithIncomingRelations(t *testing.T) {
	app := newTestAppV1(t)

	// Add entities with incoming relation
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	app.g.AddEdge(&model.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/FEA-001?include=_actions", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "feature", "features", "FEA-001")
	app.mu.RUnlock()

	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should not allow delete because FEA-001 has incoming relations
	if entity.Actions == nil || entity.Actions.Delete == nil {
		t.Fatal("expected delete action")
	}
	if entity.Actions.Delete.Allowed {
		t.Error("expected delete to be disallowed due to incoming relations")
	}
}

func TestV1DynamicRoutesPostToCollection(t *testing.T) {
	app := newTestAppV1(t)

	// POST without workspace should fail gracefully
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1DynamicRoutes(rec, req)
	app.mu.RUnlock()

	// Should return 400 or 422 because body is empty/invalid
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 400 or 422, got %d", rec.Code)
	}
}

func TestV1DynamicRoutesOptionsCollection(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1DynamicRoutes(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}

	allow := rec.Header().Get("Allow")
	if allow == "" {
		t.Error("expected Allow header")
	}
}

func TestV1SearchMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_search", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1Search(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1SidePanelMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_sidepanel/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1SidePanel(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1SidePanelInvalidPath(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidepanel/invalid", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1SidePanel(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestV1SidePanelFormNotFound(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidepanel/nonexistent/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1SidePanel(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestV1SidePanelNoConfig(t *testing.T) {
	app := newTestAppV1(t)
	app.Cfg.Forms["ticket"] = dataentryconfig.Form{
		EntityType: "ticket",
		SidePanel:  nil, // No side panel config
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidepanel/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1SidePanel(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1SchemaWithCustomTypes(t *testing.T) {
	app := newTestAppV1(t)

	// Add custom type
	app.meta.Types = map[string]metamodel.CustomType{
		"status_type": {
			Values:  []string{"open", "in_progress", "closed"},
			Default: "open",
		},
	}
	// Update property to use custom type
	app.meta.Entities["ticket"] = metamodel.EntityDef{
		Label: "Ticket",
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "status_type"},
		},
	}

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

	// Check that custom types are included
	if _, ok := schema.Types["status_type"]; !ok {
		t.Error("expected custom type 'status_type' in schema")
	}

	// Check that property has values from custom type
	ticketType := schema.Entities["ticket"]
	if ticketType.Properties["status"].Values == nil {
		t.Error("expected status property to have values from custom type")
	}
}

func TestV1PaginationLinkHeaders(t *testing.T) {
	app := newTestAppV1(t)

	// Add 30 entities
	for i := 1; i <= 30; i++ {
		app.g.AddNode(&model.Entity{
			ID:   "TKT-" + padInt(i),
			Type: "ticket",
			Properties: map[string]interface{}{
				"title": "Ticket " + padInt(i),
			},
		})
	}

	// Get first page
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?page=1&per_page=10", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	link := rec.Header().Get("Link")
	if !strings.Contains(link, "rel=\"first\"") {
		t.Error("expected 'first' link in Link header")
	}
	if !strings.Contains(link, "rel=\"next\"") {
		t.Error("expected 'next' link in Link header")
	}
	if !strings.Contains(link, "rel=\"last\"") {
		t.Error("expected 'last' link in Link header")
	}

	// Get middle page (should have prev)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tickets?page=2&per_page=10", http.NoBody)
	rec = httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	link = rec.Header().Get("Link")
	if !strings.Contains(link, "rel=\"prev\"") {
		t.Error("expected 'prev' link in Link header for page 2")
	}
}

func TestV1DynamicRoutesEmptyPath(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1DynamicRoutes(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestV1SidebarEndpoint(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Sidebar(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1SidebarMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Sidebar(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1SidebarWithNavigation(t *testing.T) {
	app := newTestAppV1(t)

	// Add entities to get counts
	app.g.AddNode(&model.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "Test"}})
	app.g.AddNode(&model.Entity{ID: "FEA-001", Type: "feature", Properties: map[string]interface{}{"title": "Test Feature"}})

	// Set up navigation with groups using actual struct fields
	app.Cfg.Navigation = []dataentryconfig.NavigationEntry{
		{
			Group: "Main",
			Items: []dataentryconfig.NavigationEntry{
				{Label: "Tickets", List: "tickets"},
				{Label: "Kanban", Kanban: "board"},
				{Label: "Dashboard", Dashboard: true},
				{Label: "Graph", Graph: true},
				{Label: "Search", Search: true},
				{Label: "Settings", Settings: true},
			},
		},
		// Top-level item without group
		{Label: "Features", List: "features"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Sidebar(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp V1SidebarResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Navigation) != 2 {
		t.Errorf("expected 2 navigation groups, got %d", len(resp.Navigation))
	}
}

func TestV1ComputeEntityActionsWithCustomType(t *testing.T) {
	app := newTestAppV1(t)

	// Set up status property with custom type
	app.meta.Types = map[string]metamodel.CustomType{
		"ticket_status": {
			Values:  []string{"open", "in_progress", "closed"},
			Default: "open",
		},
	}
	app.meta.Entities["ticket"] = metamodel.EntityDef{
		Label: "Ticket",
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "ticket_status"},
		},
	}

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=_actions", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have transitions from custom type
	if entity.Actions == nil || len(entity.Actions.Transitions) == 0 {
		t.Error("expected transitions in actions")
	}

	// Current status should be filtered out
	for _, tr := range entity.Actions.Transitions {
		if tr == "open" {
			t.Error("current status 'open' should be filtered out of transitions")
		}
	}
}

func TestV1FilterUnknownOperator(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	// Unknown operator should include entity (fallback)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[title][unknown]=test", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Entity should still be included with unknown operator
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 entity with unknown filter operator, got %d", len(resp.Data))
	}
}

func TestV1SchemaTypesSpecific(t *testing.T) {
	app := newTestAppV1(t)

	// Add custom type that should be reflected in property
	app.meta.Types = map[string]metamodel.CustomType{
		"priority_type": {
			Values:  []string{"low", "medium", "high"},
			Default: "medium",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema/types/ticket", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1SchemaRoutes(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var entityType V1EntityType
	if err := json.NewDecoder(rec.Body).Decode(&entityType); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if entityType.Label != "Ticket" {
		t.Errorf("expected label 'Ticket', got %q", entityType.Label)
	}
}

func TestV1GetEntityIncludeIncoming(t *testing.T) {
	app := newTestAppV1(t)

	// Add entities with relations
	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	app.g.AddEdge(&model.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	// Get the feature entity with include=* to get incoming relations
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/FEA-001?include=*", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "feature", "features", "FEA-001")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should include the incoming relation (ticket)
	if _, ok := entity.Included["TKT-001"]; !ok {
		t.Error("expected TKT-001 in included entities from incoming relation")
	}
}

func TestV1DynamicRoutesMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	// CONNECT method is not allowed
	req := httptest.NewRequest(http.MethodConnect, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1DynamicRoutes(rec, req)
	app.mu.RUnlock()

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1PaginationEdgeCases(t *testing.T) {
	app := newTestAppV1(t)

	// Add some entities
	for i := 1; i <= 5; i++ {
		app.g.AddNode(&model.Entity{
			ID:   "TKT-" + padInt(i),
			Type: "ticket",
			Properties: map[string]interface{}{
				"title": "Ticket " + padInt(i),
			},
		})
	}

	// Test page beyond total (should return empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?page=100&per_page=10", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Page beyond total should return empty data
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 entities for page beyond total, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Meta.Total)
	}
}

func TestV1AnalyzeWithIssues(t *testing.T) {
	app := newTestAppV1(t)

	// Add entity without required property
	app.g.AddNode(&model.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{
			// Missing required "title" property
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var result APIAnalysisResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return a valid result - we just verify it doesn't error
	_ = result
}

func TestV1SortMultipleSpecsWithSameValue(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
			"title":  "A Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open", // Same status as TKT-001
			"title":  "B Ticket",
		},
	})
	app.g.AddNode(&model.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",     // Same status
			"title":  "A Ticket", // Same title as TKT-001
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=status,title", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1ResolveIncludesEmptyPart(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	// Include with empty parts (trailing comma)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=implements,,_actions", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	app.mu.RUnlock()

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1SchemaWithRelationCardinality(t *testing.T) {
	app := newTestAppV1(t)

	// Add relation with cardinality constraints
	minOut := 1
	maxOut := 5
	app.meta.Relations["requires"] = metamodel.RelationDef{
		Label:       "requires",
		From:        []string{"ticket"},
		To:          []string{"feature"},
		MinOutgoing: &minOut,
		MaxOutgoing: &maxOut,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Schema(rec, req)

	var schema V1Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	rel := schema.Relations["requires"]
	if rel.MinOutgoing == nil || *rel.MinOutgoing != 1 {
		t.Error("expected min_outgoing to be 1")
	}
	if rel.MaxOutgoing == nil || *rel.MaxOutgoing != 5 {
		t.Error("expected max_outgoing to be 5")
	}
}

func TestV1EntityToV1WithoutRelations(t *testing.T) {
	app := newTestAppV1(t)

	app.g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
		Content: "Some markdown content",
	})

	// Call without relations (first list endpoint)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()

	app.mu.RLock()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	app.mu.RUnlock()

	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// List response shouldn't have relations by default
	if resp.Data[0].Relations != nil {
		t.Error("list response should not include relations by default")
	}

	// But should have properties and content
	if resp.Data[0].Properties["title"] != "Test Ticket" {
		t.Error("expected title property")
	}
}

func TestV1CommandsEndpoint(t *testing.T) {
	app := newTestAppV1(t)

	tests := []struct {
		name           string
		pageType       string
		qualifier      string
		entityType     string
		expectedStatus int
	}{
		{"no params", "", "", "", http.StatusOK},
		{"entity page type", "entity", "", "ticket", http.StatusOK},
		{"list page type", "list", "open-tickets", "ticket", http.StatusOK},
		{"dashboard page type", "dashboard", "", "", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/_commands"
			if tc.pageType != "" || tc.qualifier != "" || tc.entityType != "" {
				url += "?"
				parts := []string{}
				if tc.pageType != "" {
					parts = append(parts, "page_type="+tc.pageType)
				}
				if tc.qualifier != "" {
					parts = append(parts, "qualifier="+tc.qualifier)
				}
				if tc.entityType != "" {
					parts = append(parts, "entity_type="+tc.entityType)
				}
				url += strings.Join(parts, "&")
			}

			req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1Commands(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rec.Code)
			}
		})
	}
}

func TestV1CommandsMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_commands", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Commands(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1TemplatesEndpointErrors(t *testing.T) {
	app := newTestAppV1(t)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"missing entity type", "/api/v1/_templates/", http.StatusBadRequest},
		{"unknown entity type", "/api/v1/_templates/unknown", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1Templates(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rec.Code)
			}
		})
	}
}

func TestV1TemplatesMethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_templates/ticket", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Templates(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
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
