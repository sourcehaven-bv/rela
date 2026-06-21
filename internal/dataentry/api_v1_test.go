package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	url2 "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

	ticket, ok := schema.Entities["ticket"]
	if !ok {
		t.Fatal("expected 'ticket' entity type in schema")
	}
	// The `docs` file property has max:3 — the schema must surface it so the
	// SPA's file widget knows to use multi-file (add) mode, not replace mode.
	if got := ticket.Properties["docs"].Max; got != 3 {
		t.Errorf("docs.max = %d, want 3 (multi-file widget depends on this)", got)
	}
	if got := ticket.Properties["screenshot"].Max; got != 0 {
		t.Errorf("screenshot.max = %d, want 0 (single-file, omitted)", got)
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

func TestV1ConfigEndpoint_IncludesActions(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Test App"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
		Actions: map[string]dataentryconfig.Action{
			"mark-done": {
				Label: "Done",
				Key:   "d",
				Set:   map[string]string{"status": "closed"},
			},
		},
	}
	app := newAppFromParts(cfg, meta, newFixture())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Config(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var config V1Config
	if err := json.NewDecoder(rec.Body).Decode(&config); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	action, ok := config.Actions["mark-done"]
	if !ok {
		t.Fatal("expected 'mark-done' action in config response")
	}
	if action.Label != "Done" {
		t.Errorf("expected label 'Done', got %q", action.Label)
	}
	if action.Key != "d" {
		t.Errorf("expected key 'd', got %q", action.Key)
	}
	if action.Set["status"] != "closed" {
		t.Errorf("expected set status 'closed', got %q", action.Set["status"])
	}
}

func TestV1ListEntities(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entity
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
	seedEntity(app, &entity.Entity{
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
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
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
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "NONEXISTENT")
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
	seedEntity(app, &entity.Entity{
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
			app.handleV1DynamicRoutes(rec, req)
			if rec.Code != tc.expectedStatus {
				t.Errorf("path %s: expected status %d, got %d", tc.path, tc.expectedStatus, rec.Code)
			}
		})
	}
}

func TestV1Filtering(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entities
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Open Ticket",
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Closed Ticket",
			"status": "closed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[status]=open", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Open Ticket",
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Completed Ticket",
			"status": "completed",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Superseded Ticket",
			"status": "superseded",
		},
	})
	seedEntity(app, &entity.Entity{
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
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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

// fakeSearcher returns the configured ids for any non-empty Text query.
// `gotTypes` records the q.Types it received so tests can assert that
// freeTextIDsForType pins the search to the list's type rather than letting
// a stray `type:` token in the query escape that scope.
//
// `err`, when non-nil, is yielded once instead of hits — used to test the
// handler's error-surface path.
type fakeSearcher struct {
	hits     []search.Hit
	err      error
	gotTypes []string
}

func (f *fakeSearcher) Search(_ context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	f.gotTypes = q.Types
	return func(yield func(search.Hit, error) bool) {
		if q.Text == "" {
			return
		}
		if f.err != nil {
			yield(search.Hit{}, f.err)
			return
		}
		for _, h := range f.hits {
			if !yield(h, nil) {
				return
			}
		}
	}
}

func TestV1ListEntitiesSearchQuery(t *testing.T) {
	t.Run("empty q is a no-op", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
		seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "B"}})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d", rec.Code)
		}
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 2 {
			t.Fatalf("expected 2 entities (q empty = no filter), got %d", len(resp.Data))
		}
	})

	t.Run("q intersects with the typed list and preserves list sort", func(t *testing.T) {
		app := newTestAppV1(t)
		// B-titled ticket is hit by search; A-titled ticket is not. With sort=title
		// ascending the result is just the B ticket — and absence of A confirms the
		// intersection happens before sort/paginate.
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "B Ticket"}})
		seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "A Ticket"}})
		seedEntity(app, &entity.Entity{ID: "TKT-003", Type: "ticket", Properties: map[string]interface{}{"title": "C Ticket"}})
		// Searcher returns TKT-001 and TKT-003 only. List sort must reorder them
		// (C → 003) — proving Bleve ranking is discarded.
		app.searcher = &fakeSearcher{hits: []search.Hit{
			{ID: "TKT-001", Type: "ticket"},
			{ID: "TKT-003", Type: "ticket"},
		}}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=ticket&sort=title", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d", rec.Code)
		}
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 2 {
			t.Fatalf("expected 2 hits, got %d", len(resp.Data))
		}
		// TKT-001 (B Ticket) must come before TKT-003 (C Ticket).
		if resp.Data[0].ID != "TKT-001" || resp.Data[1].ID != "TKT-003" {
			t.Errorf("expected list-sorted [TKT-001, TKT-003], got [%s, %s]",
				resp.Data[0].ID, resp.Data[1].ID)
		}
	})

	t.Run("q with no matching ids returns empty page", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
		app.searcher = &fakeSearcher{hits: nil}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=needle", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 0 {
			t.Fatalf("expected empty result, got %d", len(resp.Data))
		}
		if resp.Meta.Total != 0 {
			t.Errorf("expected total 0, got %d", resp.Meta.Total)
		}
	})

	t.Run("q AND-combines with property filter", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A", "status": "open"}})
		seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "B", "status": "closed"}})
		// Searcher hits both; the filter must still narrow to "open".
		app.searcher = &fakeSearcher{hits: []search.Hit{
			{ID: "TKT-001", Type: "ticket"},
			{ID: "TKT-002", Type: "ticket"},
		}}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=ticket&filter[status]=open", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
			t.Fatalf("expected only TKT-001, got %v", responseIDs(resp))
		}
	})

	t.Run("whitespace-only q is treated as empty", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
		// Force a fakeSearcher that would return zero hits if invoked, so the
		// test can prove the searcher was never called.
		fake := &fakeSearcher{hits: nil}
		app.searcher = fake

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=%20%20", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 1 {
			t.Fatalf("expected 1 entity (whitespace q is no-op), got %d", len(resp.Data))
		}
		// Confirm we never reached the searcher — gotTypes is set on every
		// Search call, so its zero value proves we short-circuited.
		if fake.gotTypes != nil {
			t.Errorf("searcher should not be called for whitespace q, got types %v", fake.gotTypes)
		}
	})

	t.Run("prop-only q without free-text words is ignored on the list endpoint", func(t *testing.T) {
		// `q=type:foo` and `q=prop:status=done` parse to a SearchQuery with
		// no free-text words. Per the helper's contract, that's treated as
		// "no free-text filter" — the list still uses the typed list and
		// any URL filter[*] params, but the searcher is not invoked. This
		// test pins that behavior so a future change to also intersect on
		// prop-only queries doesn't slip through silently.
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
		seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "B"}})
		fake := &fakeSearcher{hits: []search.Hit{{ID: "TKT-999", Type: "ticket"}}}
		app.searcher = fake

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=type%3Afoo", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 2 {
			t.Errorf("expected 2 (full list, search not invoked), got %d", len(resp.Data))
		}
		if fake.gotTypes != nil {
			t.Errorf("searcher should not be called for prop-only q, got types %v", fake.gotTypes)
		}
	})

	t.Run("searcher pins type to the list type, ignoring stray type: in q", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "Hit"}})
		fake := &fakeSearcher{hits: []search.Hit{{ID: "TKT-001", Type: "ticket"}}}
		app.searcher = fake

		// Query says `type:feature` but we're listing tickets — the helper
		// must overwrite the type to the list's type.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=type%3Afeature+hit", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")

		if len(fake.gotTypes) != 1 || fake.gotTypes[0] != "ticket" {
			t.Errorf("expected searcher to receive Types=[ticket], got %v", fake.gotTypes)
		}
	})

	t.Run("searcher error surfaces as 500", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
		app.searcher = &fakeSearcher{err: errors.New("index unavailable")}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=anything", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500 on searcher error, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("q + pagination: total reflects post-q count, page slice is from filtered set", func(t *testing.T) {
		app := newTestAppV1(t)
		// 12 tickets; searcher matches 7 of them (TKT-001..TKT-007).
		hits := make([]search.Hit, 0)
		for i := 1; i <= 12; i++ {
			id := "TKT-" + padInt(i)
			seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]interface{}{"title": "T " + padInt(i)}})
			if i <= 7 {
				hits = append(hits, search.Hit{ID: id, Type: "ticket"})
			}
		}
		app.searcher = &fakeSearcher{hits: hits}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q=hit&page=2&per_page=3", http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		var resp V1ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Meta.Total != 7 {
			t.Errorf("expected total=7 (post-q count), got %d", resp.Meta.Total)
		}
		if resp.Meta.Page != 2 || resp.Meta.PerPage != 3 {
			t.Errorf("expected page=2 per_page=3, got page=%d per_page=%d", resp.Meta.Page, resp.Meta.PerPage)
		}
		// Page 2 of 7 hits with per_page=3 → indices [3..5] → TKT-004, 005, 006.
		if got := responseIDs(resp); len(got) != 3 || got[0] != "TKT-004" || got[2] != "TKT-006" {
			t.Errorf("expected page-2 slice [TKT-004, TKT-005, TKT-006], got %v", got)
		}
	})

	t.Run("quoted phrase in q is forwarded to the searcher", func(t *testing.T) {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "Hit"}})
		fake := &fakeSearcher{hits: []search.Hit{{ID: "TKT-001", Type: "ticket"}}}
		app.searcher = fake

		req := httptest.NewRequest(http.MethodGet, `/api/v1/tickets?q=%22exact+phrase%22`, http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1ListEntities(rec, req, "ticket", "tickets")
		// We don't assert the precise q.Text shape (parser-dependent), only
		// that the searcher was reached with a non-empty types pin.
		if fake.gotTypes == nil {
			t.Errorf("searcher should be called for quoted phrase q")
		}
	})
}

func responseIDs(r V1ListResponse) []string {
	out := make([]string, 0, len(r.Data))
	for _, e := range r.Data {
		out = append(out, e.ID)
	}
	return out
}

func TestV1Sorting(t *testing.T) {
	app := newTestAppV1(t)

	// Add test entities
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "B Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "A Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=title", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
		seedEntity(app, &entity.Entity{
			ID:   "TKT-" + padInt(i),
			Type: "ticket",
			Properties: map[string]interface{}{
				"title": "Ticket " + padInt(i),
			},
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?page=2&per_page=10", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
	app.handleV1Search(rec, req)
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
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Search Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=Search", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1SearchWithTypeFilter(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=Test&type=ticket", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)
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
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	seedRelation(app, &entity.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=*", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
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

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	seedRelation(app, &entity.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=implements", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
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

	entity := &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	}
	seedEntity(app, entity)

	// First request to get ETag
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	// Second request with If-None-Match
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusNotModified {
		t.Errorf("expected status 304, got %d", rec.Code)
	}
}

func TestV1GetEntityWithActions(t *testing.T) {
	app := newTestAppV1(t)

	// Set up status property with values
	app.Meta().Entities["ticket"] = metamodel.EntityDef{
		Label: "Ticket",
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "string", Values: []string{"open", "in_progress", "closed"}},
		},
	}

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if entity.Actions == nil {
		t.Fatal("expected actions in response")
	}
	if _, ok := entity.Actions["delete"]; !ok {
		t.Error("expected 'delete' key in actions map")
	}
}

func TestV1SingleEntityOptions(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1SingleEntity(rec, req, "ticket", "tickets", "TKT-001")
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

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1SingleEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1ListEntitiesEmpty(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "A Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "B Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=-title", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Bug Fix Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Feature Request",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[title][contains]=Bug", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "in_progress",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "closed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?filter[status][in]=open,in_progress", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 filtered entities, got %d", len(resp.Data))
	}
}

// TestV1FilteringPercentEncodedBrackets verifies the parser accepts the
// percent-encoded form Vue Router emits (`filter%5Bstatus%5D=open`). Without
// this, the FE→BE round-trip silently no-ops because the key prefix check
// looks for the literal `filter[`.
func TestV1FilteringPercentEncodedBrackets(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "closed",
		},
	})

	// Plain percent-encoded form
	got := runListFilter(t, app, "filter%5Bstatus%5D=open")
	if len(got) != 1 || got[0] != "TKT-001" {
		t.Errorf("plain encoded brackets: expected [TKT-001], got %v", got)
	}

	// Percent-encoded with operator
	seedEntity(app, &entity.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"due_date": "2026-01-01",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-004",
		Type: "ticket",
		Properties: map[string]interface{}{
			"due_date": "2027-01-01",
		},
	})
	got = runListFilter(t, app, "filter%5Bdue_date%5D%5Blte%5D=2026-06-01")
	if len(got) != 1 || got[0] != "TKT-003" {
		t.Errorf("encoded operator: expected [TKT-003], got %v", got)
	}
}

// TestV1FilteringMultiValueRepeatedParams verifies that the `in` operator
// honors repeated query params (`filter[tags][in][]=a&filter[tags][in][]=b`),
// matching the array form Vue Router emits for multi-select widgets. Before
// the fix, only the first value survived because the handler took values[0].
func TestV1FilteringMultiValueRepeatedParams(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "in_progress",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "closed",
		},
	})

	// Repeated params (array form): should match BOTH values, not just the first
	got := runListFilter(t, app, "filter%5Bstatus%5D%5Bin%5D%5B%5D=open&filter%5Bstatus%5D%5Bin%5D%5B%5D=in_progress")
	if len(got) != 2 {
		t.Errorf("repeated params: expected 2 results, got %d (%v)", len(got), got)
	}
}

// runListFilter is a tiny helper for filter tests: builds the request,
// invokes the handler under the read lock, and returns the IDs in the
// response in document order.
func runListFilter(t *testing.T, app *App, query string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?"+query, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	var resp V1ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	ids := make([]string, len(resp.Data))
	for i, e := range resp.Data {
		ids[i] = e.ID
	}
	return ids
}

func TestV1FilteringLte(t *testing.T) {
	app := newTestAppV1(t)

	earlier := "2025-12-31"
	threshold := "2026-04-07"
	later := "2026-12-31"

	earlierID := "TKT-earlier"
	thresholdID := "TKT-threshold"
	laterID := "TKT-later"

	seedEntity(app, &entity.Entity{ID: earlierID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": earlier}})
	seedEntity(app, &entity.Entity{ID: thresholdID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": threshold}})
	seedEntity(app, &entity.Entity{ID: laterID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": later}})

	got := runListFilter(t, app, "filter[due_date][lte]="+threshold)
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	if len(got) != 2 || !gotSet[earlierID] || !gotSet[thresholdID] {
		t.Errorf("expected %v, got %v", []string{earlierID, thresholdID}, got)
	}
}

func TestV1FilteringGte(t *testing.T) {
	app := newTestAppV1(t)

	earlier := "2025-12-31"
	later := "2026-12-31"
	earlierID := "TKT-earlier"
	laterID := "TKT-later"

	seedEntity(app, &entity.Entity{ID: earlierID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": earlier}})
	seedEntity(app, &entity.Entity{ID: laterID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": later}})

	got := runListFilter(t, app, "filter[due_date][gte]=2026-01-01")
	if len(got) != 1 || got[0] != laterID {
		t.Errorf("expected [%s], got %v", laterID, got)
	}
}

func TestV1FilteringTodaySubstitution(t *testing.T) {
	// Pin the clock for deterministic test behavior.
	pinned := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	prev := nowFunc
	nowFunc = func() time.Time { return pinned }
	defer func() { nowFunc = prev }()

	app := newTestAppV1(t)

	overdueID := "TKT-overdue"
	todayID := "TKT-today"
	futureID := "TKT-future"

	seedEntity(app, &entity.Entity{ID: overdueID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-06"}})
	seedEntity(app, &entity.Entity{ID: todayID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-07"}})
	seedEntity(app, &entity.Entity{ID: futureID, Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-08"}})

	got := runListFilter(t, app, "filter[due_date][lte]=$today")
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	if len(got) != 2 || !gotSet[overdueID] || !gotSet[todayID] {
		t.Errorf("expected [%s, %s], got %v", overdueID, todayID, got)
	}
}

// TestV1FilteringTypeMismatch verifies that comparing a date property against
// a non-date filter value excludes the entity rather than silently lying.
func TestV1FilteringTypeMismatch(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-07"}})

	// "tomorrow" is not a date and not a known variable; should NOT silently
	// match via lexicographic comparison.
	got := runListFilter(t, app, "filter[due_date][lt]=tomorrow")
	if len(got) != 0 {
		t.Errorf("expected 0 entities (type mismatch), got %v", got)
	}
}

// TestV1FilteringMissingProperty verifies that lt/gte against a property that
// the entity doesn't have excludes the entity (no panic, no inclusion).
func TestV1FilteringMissingProperty(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-no-due", Type: "ticket",
		Properties: map[string]interface{}{"title": "no due date"}})
	seedEntity(app, &entity.Entity{ID: "TKT-with-due", Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-07"}})

	got := runListFilter(t, app, "filter[due_date][lte]=2026-12-31")
	if len(got) != 1 || got[0] != "TKT-with-due" {
		t.Errorf("expected [TKT-with-due], got %v", got)
	}
}

// TestV1FilteringInWithVariableTokens verifies $today substitution works
// for individual tokens in a comma-separated `in` filter.
func TestV1FilteringInWithVariableTokens(t *testing.T) {
	pinned := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	prev := nowFunc
	nowFunc = func() time.Time { return pinned }
	defer func() { nowFunc = prev }()

	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-yesterday", Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-06"}})
	seedEntity(app, &entity.Entity{ID: "TKT-today", Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-07"}})
	seedEntity(app, &entity.Entity{ID: "TKT-other", Type: "ticket",
		Properties: map[string]interface{}{"due_date": "2026-04-09"}})

	got := runListFilter(t, app, "filter[due_date][in]=$yesterday,$today")
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	if len(got) != 2 || !gotSet["TKT-yesterday"] || !gotSet["TKT-today"] {
		t.Errorf("expected [TKT-yesterday, TKT-today], got %v", got)
	}
}

func TestV1FilteringEmptyValue(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Has Title",
		},
	})
	seedEntity(app, &entity.Entity{
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
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1MultipleSort(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
			"title":  "B Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
			"title":  "A Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "closed",
			"title":  "C Ticket",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=status,title", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-002",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Another Feature",
		},
	})
	seedRelation(app, &entity.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})
	// Create another relation type for nested includes
	app.Meta().Relations["requires"] = metamodel.RelationDef{
		Label: "requires",
		From:  []string{"feature"},
		To:    []string{"feature"},
	}
	seedRelation(app, &entity.Relation{
		From: "FEA-001",
		To:   "FEA-002",
		Type: "requires",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=implements.requires", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
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
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	seedRelation(app, &entity.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/FEA-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "feature", "features", "FEA-001")
	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Delete should be allowed even with incoming relations (cascade handles cleanup).
	// Under acl.NopACL (the test-app default), every per-item verb is true.
	if entity.Actions == nil {
		t.Fatal("expected _actions on response")
	}
	if !entity.Actions["delete"] {
		t.Error("expected _actions.delete=true under NopACL (cascade removes relations)")
	}
}

func TestV1DynamicRoutesPostToCollection(t *testing.T) {
	app := newTestAppV1(t)

	// POST without workspace should fail gracefully
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DynamicRoutes(rec, req)
	// Should return 400 or 422 because body is empty/invalid
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 400 or 422, got %d", rec.Code)
	}
}

func TestV1DynamicRoutesOptionsCollection(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DynamicRoutes(rec, req)
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
	app.handleV1Search(rec, req)
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
	app.Cfg().Forms["ticket"] = dataentryconfig.Form{
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
	app.Meta().Types = map[string]metamodel.CustomType{
		"status_type": {
			Values:  []string{"open", "in_progress", "closed"},
			Default: "open",
		},
	}
	// Update property to use custom type
	app.Meta().Entities["ticket"] = metamodel.EntityDef{
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
		seedEntity(app, &entity.Entity{
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
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	link = rec.Header().Get("Link")
	if !strings.Contains(link, "rel=\"prev\"") {
		t.Error("expected 'prev' link in Link header for page 2")
	}
}

func TestV1DynamicRoutesEmptyPath(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DynamicRoutes(rec, req)
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
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "Test"}})
	seedEntity(app, &entity.Entity{ID: "FEA-001", Type: "feature", Properties: map[string]interface{}{"title": "Test Feature"}})

	// Set up navigation with groups using actual struct fields
	app.Cfg().Navigation = []dataentryconfig.NavigationEntry{
		{
			Group: "Main",
			Items: []dataentryconfig.NavigationEntry{
				{Label: "Tickets", List: "tickets"},
				{Label: "Kanban", Kanban: "board"},
				{Label: "Dashboard", Dashboard: true},
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

// TestV1SidebarAppliesListFilters verifies that sidebar counts for a list
// respect the list's configured filters, not just the entity-type total.
// Regression guard for the bug where "Open Tickets" (filter status=open)
// showed the same count as "All Tickets".
func TestV1SidebarAppliesListFilters(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]interface{}{"title": "Open A", "status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-002", Type: "ticket",
		Properties: map[string]interface{}{"title": "Open B", "status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-003", Type: "ticket",
		Properties: map[string]interface{}{"title": "Closed", "status": "closed"},
	})

	app.Cfg().Lists = map[string]dataentryconfig.List{
		"all_tickets": {
			EntityType: "ticket",
			Title:      "All Tickets",
		},
		"open_tickets": {
			EntityType: "ticket",
			Title:      "Open Tickets",
			Filters: []dataentryconfig.FilterConfig{
				{Property: "status", Operator: "=", Value: "open"},
			},
		},
	}
	app.Cfg().Navigation = []dataentryconfig.NavigationEntry{
		{Label: "All Tickets", List: "all_tickets"},
		{Label: "Open Tickets", List: "open_tickets"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Sidebar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp V1SidebarResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	counts := map[string]int{}
	for _, group := range resp.Navigation {
		for _, item := range group.Items {
			if item.Count != nil {
				counts[item.Label] = *item.Count
			}
		}
	}

	if counts["All Tickets"] != 3 {
		t.Errorf("All Tickets count = %d, want 3", counts["All Tickets"])
	}
	if counts["Open Tickets"] != 2 {
		t.Errorf("Open Tickets count = %d, want 2 (status=open); filter not applied",
			counts["Open Tickets"])
	}
}

// TestV1SidebarAppliesKanbanFilters is the kanban counterpart to
// TestV1SidebarAppliesListFilters.
func TestV1SidebarAppliesKanbanFilters(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]interface{}{"title": "P0 open", "status": "open", "priority": "high"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-002", Type: "ticket",
		Properties: map[string]interface{}{"title": "P0 closed", "status": "closed", "priority": "high"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-003", Type: "ticket",
		Properties: map[string]interface{}{"title": "P1 open", "status": "open", "priority": "low"},
	})

	app.Cfg().Kanbans = map[string]dataentryconfig.Kanban{
		"open_board": {
			EntityType: "ticket",
			Filters: []dataentryconfig.FilterConfig{
				{Property: "status", Operator: "=", Value: "open"},
			},
		},
	}
	app.Cfg().Navigation = []dataentryconfig.NavigationEntry{
		{Label: "Open Board", Kanban: "open_board"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Sidebar(rec, req)

	var resp V1SidebarResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var openCount int
	for _, group := range resp.Navigation {
		for _, item := range group.Items {
			if item.Label == "Open Board" && item.Count != nil {
				openCount = *item.Count
			}
		}
	}
	if openCount != 2 {
		t.Errorf("Open Board count = %d, want 2 (filter not applied)", openCount)
	}
}

// TestV1ComputeEntityActions_VerbVocabulary asserts the phase-1 verb
// vocabulary (update / delete / rename) is present on every entity
// response under NopACL. Workflow-transition verbs are out of phase 1;
// they will return when ACL v0.5 lands transition Op support.
func TestV1ComputeEntityActions_VerbVocabulary(t *testing.T) {
	app := newTestAppV1(t)

	app.Meta().Entities["ticket"] = metamodel.EntityDef{
		Label: "Ticket",
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "string", Values: []string{"open", "closed"}},
		},
	}

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	var entity V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&entity); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if entity.Actions == nil {
		t.Fatal("expected _actions on response")
	}
	for _, verb := range []string{"update", "delete", "rename"} {
		if _, ok := entity.Actions[verb]; !ok {
			t.Errorf("expected verb %q in _actions; got %v", verb, entity.Actions)
		}
	}
}

// TestV1FilterUnknownOperator verifies that an unknown operator (e.g. a
// typo) is SKIPPED entirely rather than falling through to a pass-all
// default. The previous fail-open behavior would have silently bypassed any
// configured scope filter whenever the URL carried a malformed operator.
func TestV1FilterUnknownOperator(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Another Ticket",
		},
	})

	// Unknown operator: the filter is dropped entirely (fail-closed), so
	// all entities are returned because no filter was actually applied.
	// Importantly, this is NOT "unknown operator matches everything" — it's
	// "unknown operator is logged and skipped, so the remaining filter set
	// is empty, so nothing constrains the list".
	got := runListFilter(t, app, "filter[title][unknown]=test")
	if len(got) != 2 {
		t.Errorf("expected 2 entities when unknown operator is skipped, got %d", len(got))
	}
}

// TestV1FilterMalformedKeySkipped verifies that malformed filter keys
// (empty property, empty operator, too many segments) are skipped with a
// log warning rather than silently passing every entity.
func TestV1FilterMalformedKeySkipped(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID:         "TKT-002",
		Type:       "ticket",
		Properties: map[string]interface{}{"status": "closed"},
	})

	// Malformed keys: should be dropped, so another valid filter on the
	// same request still applies cleanly. Here we combine a bogus key with
	// a legit status=open filter and assert the legit one still works.
	got := runListFilter(t, app, "filter[status][][weird]=nope&filter[status]=open")
	if len(got) != 1 || got[0] != "TKT-001" {
		t.Errorf("malformed key + valid filter: expected [TKT-001], got %v", got)
	}

	// Empty property: dropped.
	got = runListFilter(t, app, "filter[][eq]=anything&filter[status]=closed")
	if len(got) != 1 || got[0] != "TKT-002" {
		t.Errorf("empty property + valid filter: expected [TKT-002], got %v", got)
	}
}

func TestV1SchemaTypesSpecific(t *testing.T) {
	app := newTestAppV1(t)

	// Add custom type that should be reflected in property
	app.Meta().Types = map[string]metamodel.CustomType{
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
	// This endpoint shares toV1PropertyDef with /_schema, so it must surface
	// the same fields — in particular `max` for file properties (the multi-
	// file widget depends on it) so the two schema endpoints can't drift.
	if got := entityType.Properties["docs"].Max; got != 3 {
		t.Errorf("docs.max = %d, want 3 (file widget multi-file mode)", got)
	}
	if got := entityType.Properties["screenshot"].Max; got != 0 {
		t.Errorf("screenshot.max = %d, want 0 (single-file, omitted)", got)
	}
}

func TestV1GetEntityIncludeIncoming(t *testing.T) {
	app := newTestAppV1(t)

	// Add entities with relations
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "FEA-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	seedRelation(app, &entity.Relation{
		From: "TKT-001",
		To:   "FEA-001",
		Type: "implements",
	})

	// Get the feature entity with include=* to get incoming relations
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/FEA-001?include=*", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "feature", "features", "FEA-001")
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

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	// CONNECT method is not allowed
	req := httptest.NewRequest(http.MethodConnect, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DynamicRoutes(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestV1PaginationEdgeCases(t *testing.T) {
	app := newTestAppV1(t)

	// Add some entities
	for i := 1; i <= 5; i++ {
		seedEntity(app, &entity.Entity{
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
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
	seedEntity(app, &entity.Entity{
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

// newAnalyzeScriptErrorApp builds an App with one inline-Lua validation
// rule that fails to compile, so handleV1Analyze produces a script-error
// issue. Wires SecurityConfig so allowFullScriptDetail can branch on
// req.RemoteAddr (loopback vs. non-loopback) — same shape used by
// the action-surface tests.
func newAnalyzeScriptErrorApp(t *testing.T) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "broken-rule",
				EntityType: "ticket",
				Lua:        `if oops invalid`,
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App: dataentryconfig.AppConfig{Name: "Test"},
	}
	app := newAppFromParts(cfg, meta, newFixture())
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x"},
	})
	if err := app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}); err != nil {
		t.Fatalf("SetSecurityConfig: %v", err)
	}
	return app
}

// findIssueWithScriptError returns the first APIIssue carrying a
// non-nil ScriptError envelope; nil if none.
func findIssueWithScriptError(issues []APIIssue) *APIIssue {
	for i := range issues {
		if issues[i].ScriptError != nil {
			return &issues[i]
		}
	}
	return nil
}

// TestV1Analyze_ScriptErrorEnvelope_NonLoopback verifies that a broken
// Lua validation rule produces an issue with a populated ScriptError
// envelope on the wire, but with the gated detail (Source, Stack,
// CapturedOutput) absent for non-loopback callers — same shape as
// writeV1ScriptError.
func TestV1Analyze_ScriptErrorEnvelope_NonLoopback(t *testing.T) {
	app := newAnalyzeScriptErrorApp(t)

	// Default httptest RemoteAddr is 192.0.2.1 (TEST-NET-1, non-loopback).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	rec := httptest.NewRecorder()

	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result APIAnalysisResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	hit := findIssueWithScriptError(result.Issues)
	if hit == nil {
		t.Fatalf("expected at least one issue with ScriptError envelope; got %+v", result.Issues)
	}
	if hit.ScriptError.Error != "script_error" {
		t.Errorf("ScriptError.Error=%q, want script_error", hit.ScriptError.Error)
	}
	if hit.ScriptError.Lua.Message == "" {
		t.Errorf("ScriptError.Lua.Message is empty; want non-empty failure message")
	}
	// Degraded shape for non-loopback callers.
	if len(hit.ScriptError.Source) != 0 {
		t.Errorf("non-loopback caller got source slice: %+v", hit.ScriptError.Source)
	}
	if len(hit.ScriptError.Stack) != 0 {
		t.Errorf("non-loopback caller got stack: %+v", hit.ScriptError.Stack)
	}
	if hit.ScriptError.CapturedOutput != "" {
		t.Errorf("non-loopback caller got captured output: %q", hit.ScriptError.CapturedOutput)
	}
}

// TestV1Analyze_ScriptErrorEnvelope_Loopback verifies that loopback
// callers receive the full envelope (source slice present when the
// failure has a parsable line). Inline-Lua compile failures don't
// produce a source slice (no file to read), but Lua.Line and
// Lua.Message are always populated, so we assert on those.
func TestV1Analyze_ScriptErrorEnvelope_Loopback(t *testing.T) {
	app := newAnalyzeScriptErrorApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result APIAnalysisResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	hit := findIssueWithScriptError(result.Issues)
	if hit == nil {
		t.Fatalf("expected at least one issue with ScriptError envelope; got %+v", result.Issues)
	}
	if hit.ScriptError.Lua.Message == "" {
		t.Error("ScriptError.Lua.Message is empty; want non-empty")
	}
	// Other issues (e.g., orphan warnings) must NOT carry an envelope.
	for _, issue := range result.Issues {
		if issue.CheckType != "Validations" && issue.ScriptError != nil {
			t.Errorf("non-validation issue %q has unexpected ScriptError envelope", issue.CheckType)
		}
	}
}

func TestV1SortMultipleSpecsWithSameValue(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",
			"title":  "A Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open", // Same status as TKT-001
			"title":  "B Ticket",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:   "TKT-003",
		Type: "ticket",
		Properties: map[string]interface{}{
			"status": "open",     // Same status
			"title":  "A Ticket", // Same title as TKT-001
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?sort=status,title", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1ResolveIncludesEmptyPart(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "Test Ticket",
		},
	})

	// Include with empty parts (trailing comma)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=implements,,_actions", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestV1SchemaWithRelationCardinality(t *testing.T) {
	app := newTestAppV1(t)

	// Add relation with cardinality constraints
	minOut := 1
	maxOut := 5
	app.Meta().Relations["requires"] = metamodel.RelationDef{
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

	seedEntity(app, &entity.Entity{
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
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
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
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
					// The mechanics tests upload arbitrary text content, so the
					// fixture file properties accept text/plain; a dedicated
					// test exercises the default-safe allowlist rejection path.
					"screenshot": {Type: "file", Accept: []string{"text/plain"}},
					"docs":       {Type: "file", Max: 3, Accept: []string{"text/plain"}},
				},
				// PropertyOrder is populated at YAML-load time in
				// production; set it explicitly here so tests exercise
				// the same code paths the runtime hits.
				PropertyOrder: []string{"title", "status", "screenshot", "docs"},
			},
			"feature": {
				Label:    "Feature",
				IDPrefix: "FEAT-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
				PropertyOrder: []string{"title"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "implements",
				From:  []string{"ticket"},
				To:    []string{"feature"},
			},
			"blocks": {
				Label: "blocks",
				From:  []string{"ticket"},
				To:    []string{"ticket"},
			},
		},
	}

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

	app := newAppFromParts(cfg, meta, newFixture())
	return app
}

func TestV1EntityRelationsNotFound(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/NONEXISTENT/relations", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, "ticket", "NONEXISTENT")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q",
			rec.Header().Get("Content-Type"))
	}
}

func TestV1EntityRelationsWrongType(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/TKT-001/relations", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, "feature", "TKT-001")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

// newReverseRelationsTestApp builds an app whose `blocks` relation has both
// an `Inverse` (so grouped responses key incoming edges under "blockedBy")
// and a `reason` property (so the per-edge response includes meta).
// newTestAppV1's `blocks` is intentionally bare; it is shared by many tests
// and we don't want to perturb their assertions.
func newReverseRelationsTestApp(t *testing.T) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}},
			"feature": {Label: "Feature", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"blocks": {
				Label:   "blocks",
				From:    []string{"feature"},
				To:      []string{"feature"},
				Inverse: &metamodel.InverseDef{ID: "blockedBy"},
				Properties: map[string]metamodel.PropertyDef{
					"reason": {Type: "string"},
				},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Reverse Test", Description: "x"},
		Forms:      map[string]dataentryconfig.Form{},
		Lists:      map[string]dataentryconfig.List{},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	return newAppFromParts(cfg, meta, newFixture())
}

// seedBlocksReverseFixture seeds FEAT-001 --blocks--> FEAT-003 with a
// `reason` property, the canonical reverse-relation regression scenario.
func seedBlocksReverseFixture(t *testing.T, app *App) (sourceID, targetID string) {
	t.Helper()
	sourceID, targetID = "FEAT-001", "FEAT-003"
	seedEntity(app, &entity.Entity{ID: sourceID, Type: "feature", Properties: map[string]interface{}{"title": "source"}})
	seedEntity(app, &entity.Entity{ID: targetID, Type: "feature", Properties: map[string]interface{}{"title": "target"}})
	if _, err := app.store.CreateRelation(
		t.Context(),
		sourceID, "blocks", targetID,
		&store.RelationData{Properties: map[string]interface{}{"reason": "test block"}},
	); err != nil {
		t.Fatalf("seed blocks relation: %v", err)
	}
	return sourceID, targetID
}

// TestV1GetRelationType_IncomingReturnsEdgeWithMeta covers the
// `GET /api/v1/{plural}/{id}/relations/{relType}?direction=incoming`
// contract that the data-entry SPA's reverse widgets depend on. Was an
// e2e-level test; moved to Go because the assertion is purely on the
// JSON shape produced by handleV1GetRelationType.
func TestV1GetRelationType_IncomingReturnsEdgeWithMeta(t *testing.T) {
	app := newReverseRelationsTestApp(t)
	sourceID, targetID := seedBlocksReverseFixture(t, app)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/features/"+targetID+"/relations/blocks?direction=incoming", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetRelationType(rec, req, "feature", targetID, "blocks")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var edges []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &edges); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 incoming edge, got %d: %s", len(edges), rec.Body.String())
	}
	if got := edges[0]["id"]; got != sourceID {
		t.Errorf("incoming edge peer = %v, want %s", got, sourceID)
	}
	if got := edges[0]["type"]; got != "feature" {
		t.Errorf("incoming edge peer type = %v, want %q", got, "feature")
	}
	meta, ok := edges[0]["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta object on edge, got %T: %v", edges[0]["meta"], edges[0])
	}
	if meta["reason"] != "test block" {
		t.Errorf("meta.reason = %v, want %q", meta["reason"], "test block")
	}
}

// TestV1EntityRelations_GroupsIncomingUnderInverseName covers the contract
// that the grouped relations endpoint surfaces incoming edges under the
// relation's configured `inverse:` name (e.g. `blocks` → `blockedBy`).
// Was an e2e-level test; moved to Go because the SPA only consumes this
// JSON shape, it doesn't render it directly.
func TestV1EntityRelations_GroupsIncomingUnderInverseName(t *testing.T) {
	app := newReverseRelationsTestApp(t)
	sourceID, targetID := seedBlocksReverseFixture(t, app)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/features/"+targetID+"/relations", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, "feature", targetID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var grouped map[string][]map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &grouped); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	blockedBy, ok := grouped["blockedBy"]
	if !ok {
		t.Fatalf("expected key %q in response, got keys: %v", "blockedBy", keysOf(grouped))
	}
	if len(blockedBy) != 1 {
		t.Fatalf("expected 1 blockedBy entry, got %d", len(blockedBy))
	}
	if got := blockedBy[0]["id"]; got != sourceID {
		t.Errorf("blockedBy[0].id = %v, want %s", got, sourceID)
	}
	if got := blockedBy[0]["type"]; got != "feature" {
		t.Errorf("blockedBy[0].type = %v, want %q", got, "feature")
	}
	if got := blockedBy[0]["direction"]; got != "incoming" {
		t.Errorf("blockedBy[0].direction = %v, want %q", got, "incoming")
	}
}

// TestV1GetRelationType_OutgoingIncludesPeerType verifies the outgoing
// path emits `type` per resource identifier — required by the SPA to
// build JSON:API §9 resource identifiers without consulting the
// schema or guessing from ID prefix. (TKT-ZEKO4 Step 0.)
func TestV1GetRelationType_OutgoingIncludesPeerType(t *testing.T) {
	app := newReverseRelationsTestApp(t)
	sourceID, targetID := seedBlocksReverseFixture(t, app)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/features/"+sourceID+"/relations/blocks", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetRelationType(rec, req, "feature", sourceID, "blocks")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var edges []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &edges); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 outgoing edge, got %d", len(edges))
	}
	if got := edges[0]["id"]; got != targetID {
		t.Errorf("outgoing edge peer id = %v, want %s", got, targetID)
	}
	if got := edges[0]["type"]; got != "feature" {
		t.Errorf("outgoing edge peer type = %v, want %q", got, "feature")
	}
}

func keysOf(m map[string][]map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestV1DeleteEntityNotFound(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tickets/NONEXISTENT", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DeleteEntity(rec, req, "ticket", "tickets", "NONEXISTENT")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q",
			rec.Header().Get("Content-Type"))
	}
}

func TestV1UpdateEntityNotFound(t *testing.T) {
	app := newTestAppV1(t)

	body := strings.NewReader(`{"properties":{"title":"Updated"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/NONEXISTENT", body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "NONEXISTENT")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestV1UpdateEntityInvalidJSON(t *testing.T) {
	app := newTestAppV1(t)

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	body := strings.NewReader(`{invalid json`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", body)
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// implementsTargets returns the set of target IDs on outgoing `implements`
// edges for entityID, used to keep the relation-save tests concise.
func implementsTargets(app *App, entityID string) map[string]bool {
	out := map[string]bool{}
	for _, r := range app.reader.outgoingRelations(context.Background(), entityID) {
		if r.Type == "implements" {
			out[r.To] = true
		}
	}
	return out
}

// TestV1CreateEntity_SavesRelations covers the default chip-picker create
// path. Like TestV1UpdateEntity_SavesRelations this was red before the
// BUG-UNEBR fix — POST /api/v1/{plural} decoded only id/properties/content
// and silently dropped the relations payload that the frontend sends.
func TestV1CreateEntity_SavesRelations(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{
		ID:         "FEAT-001",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "Feature One"},
	})
	seedEntity(app, &entity.Entity{
		ID:         "FEAT-002",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "Feature Two"},
	})

	body := `{
		"properties": {"title":"New","status":"open"},
		"relations": {"implements": {"data": [
			{"type":"feature","id":"FEAT-001"},
			{"type":"feature","id":"FEAT-002"}
		]}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1CreateEntity(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST returned %d: %s", rec.Code, rec.Body.String())
	}

	// The ticket was auto-assigned a short ID; read it from the response body.
	var created V1Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, rec.Body.String())
	}
	if created.ID == "" {
		t.Fatalf("response body missing id: %s", rec.Body.String())
	}

	got := implementsTargets(app, created.ID)
	if !got["FEAT-001"] || !got["FEAT-002"] || len(got) != 2 {
		t.Fatalf("after create: outgoing implements edges = %v, want FEAT-001+FEAT-002", got)
	}
}

// TestV1UpdateEntity_SavesRelations covers the default chip-picker save path:
// the frontend PATCHes the entity with a `relations` key, expecting outgoing
// edges for each provided relation type to be reconciled (adds + removes).
// Before BUG-UNEBR was fixed this test was red — the handler decoded only
// properties and content, silently dropping the relations payload.
func TestV1UpdateEntity_SavesRelations(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	// Bind to a temp filesystem so the workspace's relation writer has a
	// real FS and Paths context to persist to.
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:         "FEAT-001",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "Feature One"},
	})
	seedEntity(app, &entity.Entity{
		ID:         "FEAT-002",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "Feature Two"},
	})

	patch := func(t *testing.T, body string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", strings.NewReader(body))
		rec := httptest.NewRecorder()
		app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH returned %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Add an edge via PATCH.
	patch(t, `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`)
	if got := implementsTargets(app, "TKT-001"); !got["FEAT-001"] || len(got) != 1 {
		t.Fatalf("after add: outgoing implements edges = %v, want only FEAT-001", got)
	}

	// Adding a second target leaves the first in place.
	patch(t, `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"},{"type":"feature","id":"FEAT-002"}]}}}`)
	got := implementsTargets(app, "TKT-001")
	if !got["FEAT-001"] || !got["FEAT-002"] || len(got) != 2 {
		t.Fatalf("after second add: outgoing implements edges = %v, want FEAT-001+FEAT-002", got)
	}

	// Shrinking the list removes the dropped target.
	patch(t, `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`)
	got = implementsTargets(app, "TKT-001")
	if !got["FEAT-001"] || got["FEAT-002"] || len(got) != 1 {
		t.Fatalf("after remove: outgoing implements edges = %v, want only FEAT-001", got)
	}

	// An empty list for a relation type removes all of its edges.
	patch(t, `{"relations":{"implements":{"data":[]}}}`)
	if got := implementsTargets(app, "TKT-001"); len(got) != 0 {
		t.Fatalf("after empty list: outgoing implements edges = %v, want none", got)
	}

	// A PATCH that omits the relations key must leave existing edges alone.
	patch(t, `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-002"}]}}}`)
	patch(t, `{"properties":{"title":"Renamed"}}`)
	if got := implementsTargets(app, "TKT-001"); !got["FEAT-002"] || len(got) != 1 {
		t.Fatalf("after properties-only PATCH: edges = %v, want FEAT-002 preserved", got)
	}

	// Duplicate ids in the caller-supplied list collapse to the same edge.
	patch(t, `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"},{"type":"feature","id":"FEAT-001"}]}}}`)
	if got := implementsTargets(app, "TKT-001"); !got["FEAT-001"] || len(got) != 1 {
		t.Fatalf("after duplicate-id list: edges = %v, want single FEAT-001", got)
	}
}

// TestV1UpdateEntity_Relations_ScopedToTypesInPayload is the explicit guard
// for the "scoped" semantic that the rest of the tests only cover
// indirectly: reconciling one relation type must not touch another type's
// existing edges, even when both appear on the same entity.
func TestV1UpdateEntity_Relations_ScopedToTypesInPayload(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "U"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]interface{}{"title": "F1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-002", Type: "feature", Properties: map[string]interface{}{"title": "F2"}})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "blocks", To: "TKT-002"})

	// PATCH implements only — blocks must be untouched.
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-002"}]}}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH returned %d: %s", rec.Code, rec.Body.String())
	}

	impls := map[string]bool{}
	blocks := map[string]bool{}
	for _, r := range app.reader.outgoingRelations(context.Background(), "TKT-001") {
		switch r.Type {
		case "implements":
			impls[r.To] = true
		case "blocks":
			blocks[r.To] = true
		}
	}
	if !impls["FEAT-002"] || impls["FEAT-001"] || len(impls) != 1 {
		t.Fatalf("implements edges = %v, want only FEAT-002", impls)
	}
	if !blocks["TKT-002"] || len(blocks) != 1 {
		t.Fatalf("blocks edges = %v, want TKT-002 untouched", blocks)
	}
}

// TestV1UpdateEntity_Relations_MultiType drives two relation types in a
// single PATCH and asserts each is reconciled independently.
func TestV1UpdateEntity_Relations_MultiType(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"title": "U"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]interface{}{"title": "F"}})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]},"blocks":{"data":[{"type":"ticket","id":"TKT-002"}]}}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH returned %d: %s", rec.Code, rec.Body.String())
	}

	types := map[string]bool{}
	for _, r := range app.reader.outgoingRelations(context.Background(), "TKT-001") {
		types[r.Type+"->"+r.To] = true
	}
	if !types["implements->FEAT-001"] || !types["blocks->TKT-002"] || len(types) != 2 {
		t.Fatalf("edges = %v, want implements->FEAT-001 + blocks->TKT-002", types)
	}
}

// TestV1UpdateEntity_Relations_UnknownType asserts that an unknown
// relation type surfaces as a 422 with the type name in the detail, and
// no writes happen.
func TestV1UpdateEntity_Relations_UnknownType(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"bogus":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unknown_relation_type") || !strings.Contains(rec.Body.String(), "bogus") {
		t.Fatalf("detail missing structured reason/type, got: %s", rec.Body.String())
	}
	if len(app.reader.outgoingRelations(context.Background(), "TKT-001")) != 0 {
		t.Fatalf("no edges should have been written on a rejected type")
	}
}

// TestV1UpdateEntity_Relations_UnknownTarget asserts that a missing
// target id surfaces as a warning (DEC-HWZHA: soft condition, 200 with
// structured warning) rather than a hard rejection. The edge is
// written referencing a missing peer; analyze_orphans surfaces it on
// the next run.
func TestV1UpdateEntity_Relations_UnknownTarget(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-999"}]}}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (soft condition), got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "target_not_found") || !strings.Contains(rec.Body.String(), "FEAT-999") {
		t.Fatalf("response missing warning code/target, got: %s", rec.Body.String())
	}
}

// TestV1UpdateEntity_Relations_SourceTypeMismatch asserts that using a
// relation whose `from` list doesn't include the source type surfaces
// as a warning (DEC-HWZHA: soft condition, 200 with structured warning).
// The edge is written so analyze_* can surface the inconsistency on the
// next run; the storage layer is intentionally permissive.
func TestV1UpdateEntity_Relations_SourceTypeMismatch(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	// `implements` is ticket -> feature. Call the update on a feature and
	// try to add an `implements` edge from it.
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]interface{}{"title": "F"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-002", Type: "feature", Properties: map[string]interface{}{"title": "F2"}})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/features/FEAT-001",
		strings.NewReader(`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-002"}]}}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "feature", "features", "FEAT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (soft condition), got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "source_type_not_allowed") {
		t.Fatalf("response missing source_type_not_allowed warning, got: %s", rec.Body.String())
	}
}

// TestV1UpdateEntity_Relations_OnlyPATCH_NoEntityRewrite asserts that a
// PATCH that only changes relations does not round-trip UpdateEntity
// (which would rewrite the file and emit a misleading SSE event).
// Verified indirectly: the entity's mtime-free identity holds via the
// ETag — relations-only PATCH should change the ETag because relations
// are folded in, but the entity properties/content hash stays stable.
func TestV1UpdateEntity_Relations_OnlyPATCH_ETagChangesButEntityStable(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]interface{}{"title": "F"}})

	entityBefore, _ := app.reader.getEntity(context.Background(), "TKT-001")
	etagBefore := app.computeEntityETag(context.Background(), entityBefore)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH returned %d: %s", rec.Code, rec.Body.String())
	}

	entityAfter, _ := app.reader.getEntity(context.Background(), "TKT-001")
	// Entity fields (id/type/props/content) should be byte-identical.
	if entityAfter.Content != entityBefore.Content ||
		len(entityAfter.Properties) != len(entityBefore.Properties) {

		t.Fatalf("relations-only PATCH mutated entity fields: before=%+v after=%+v", entityBefore, entityAfter)
	}
	// ETag must change because it now folds in relations.
	etagAfter := app.computeEntityETag(context.Background(), entityAfter)
	if etagAfter == etagBefore {
		t.Fatalf("ETag did not change after relations-only PATCH: %s", etagAfter)
	}
}

func TestExtractEntityIDs(t *testing.T) {
	entities := []*entity.Entity{
		{ID: "REQ-001"},
		{ID: "REQ-002"},
		{ID: "DEC-001"},
	}

	got := extractEntityIDs(entities)
	want := []string{"REQ-001", "REQ-002", "DEC-001"}

	if len(got) != len(want) {
		t.Fatalf("extractEntityIDs() returned %d IDs, want %d", len(got), len(want))
	}

	for i, id := range got {
		if id != want[i] {
			t.Errorf("extractEntityIDs()[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestExtractEntityIDs_Empty(t *testing.T) {
	got := extractEntityIDs(nil)
	if len(got) != 0 {
		t.Errorf("extractEntityIDs(nil) returned %d IDs, want 0", len(got))
	}
}

// TestHandleV1Documents_EntityTypeMismatch verifies AC9 / RR-FLCXC:
// the handler rejects a request whose entity.Type doesn't match the
// document's configured entity_type, and does NOT run the renderer.
func TestHandleV1Documents_EntityTypeMismatch(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "a ticket",
		},
	})
	// Configure a document that applies only to features. Renderer is
	// a shell command, but the handler must reject before it runs.
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"feature-notes": {
			EntityType: "feature",
			Command:    "echo hello",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_documents/feature-notes/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for type mismatch, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "entity_type") {
		t.Errorf("expected body to mention entity_type, got: %s", rec.Body.String())
	}
}

// TestHandleV1Documents_EntityTypeMatch verifies the positive case:
// when the types line up, the handler proceeds past the type check.
// We use a command doc so the test doesn't need Lua machinery; the
// render may or may not succeed depending on the shell, but the
// response must not be our explicit 400 mismatch error.
func TestHandleV1Documents_EntityTypeMatch(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title": "a ticket",
		},
	})
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"ticket-summary": {
			EntityType: "ticket",
			Command:    "echo hello",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_documents/ticket-summary/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, req)

	// The renderer itself may succeed (echo in PATH) or fail; what we
	// care about here is that the handler got past the type check.
	if rec.Code == http.StatusBadRequest && strings.Contains(rec.Body.String(), "entity_type") {
		t.Errorf("unexpected type-mismatch 400 for matching types: %s", rec.Body.String())
	}
}

// TestHandleV1Documents_CacheInvariance verifies TKT-JIEKC AC9: the
// on-disk command-mode cache never contains `return_to=` tokens, and
// different callers supplying different return_to values each get their
// own value rewritten in. The rewriter runs post-cache in
// handleV1Documents (not inside doRender) — this test pins that
// invariant so a future "push rewriter into doRender" refactor fails
// loudly instead of poisoning cache files shared across users.
func TestHandleV1Documents_CacheInvariance(t *testing.T) {
	app := newTestAppV1(t)
	// bindRepo rewires the app to a real filesystem-backed workspace so
	// documentService's cache writes actually land on disk (the default
	// nopState used by newTestAppV1 silently drops writes, which would
	// make this test vacuously pass).
	root := t.TempDir()
	cacheDir := filepath.Join(root, project.CacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "entities", "tickets"), 0o755); err != nil {
		t.Fatalf("mkdir entities: %v", err)
	}
	bindRepoWithFS(
		app,
		storage.NewSafeFS(storage.NewOsFS()),
		&project.Context{Root: root, CacheDir: cacheDir},
	)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "cache test"},
	})
	// Command emits a minimal markdown link that the rewriter will
	// append return_to to. Anchor shape matches what goldmark produces.
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"cache-test": {
			EntityType: "ticket",
			Command:    `echo '[Detail](/entity/ticket/TKT-001)'`,
		},
	}

	render := func(returnTo string) string {
		url := "/api/v1/_documents/cache-test/TKT-001"
		if returnTo != "" {
			url += "?return_to=" + url2.QueryEscape(returnTo)
		}
		req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
		rec := httptest.NewRecorder()
		app.handleV1Documents(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("render returnTo=%q: status %d body %s", returnTo, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	// First render with return_to=/A. Response must contain the
	// URL-encoded /A token.
	respA := render("/A")
	if !strings.Contains(respA, "return_to=%2FA") {
		t.Errorf("first render missing return_to=/A token: %s", respA)
	}

	// Second render with return_to=/B. Must contain /B, must NOT
	// contain /A — the cache did not bake in the first caller's value.
	respB := render("/B")
	if !strings.Contains(respB, "return_to=%2FB") {
		t.Errorf("second render missing return_to=/B token: %s", respB)
	}
	if strings.Contains(respB, "return_to=%2FA") {
		t.Errorf("second render leaked first caller's return_to=/A: %s", respB)
	}

	// Disk cache invariant: the cached HTML for this entry must contain
	// no `return_to=` token. Read it via documentService.GetCached,
	// which reconstructs the cache file path from the entry's content
	// hash and reads the underlying bytes verbatim.
	cached := app.documents.GetCached(context.Background(), "TKT-001")
	if cached == nil {
		t.Fatalf("expected cache file for TKT-001 to exist after two renders")
	}
	if strings.Contains(cached.HTML, "return_to") {
		t.Errorf("cache file contains return_to token — rewriter leaked into cache:\n%s",
			cached.HTML)
	}
}

// TestHandleV1Documents_EntityNotFound verifies the 404 path for a
// missing entry before any renderer runs.
func TestHandleV1Documents_EntityNotFound(t *testing.T) {
	app := newTestAppV1(t)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"notes": {
			EntityType: "ticket",
			Command:    "echo hi",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_documents/notes/TKT-MISSING", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing entity, got %d: %s", rec.Code, rec.Body.String())
	}
}

// newPrefixTestApp builds an app whose schema includes a multi-prefix type, a
// manual-ID type, and a single-prefix short-ID type for TKT-E7NNM tests.
// Wires in an in-memory FS so the create path can load (absent) entity templates.
func newPrefixTestApp(t *testing.T) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDType:   "short",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"decision": {
				Label:      "Decision",
				IDType:     "short",
				IDPrefixes: []string{"DEC-", "ADR-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"category": {
				Label:  "Category",
				IDType: "manual",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Test App"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture())
	fs := storage.NewMemFS()
	ctx := &project.Context{
		Root:                 "/project",
		CacheDir:             "/project/.rela",
		EntitiesDir:          "/project/entities",
		RelationsDir:         "/project/relations",
		TemplatesDir:         "/project/templates",
		EntityTemplatesDir:   "/project/templates/entities",
		RelationTemplatesDir: "/project/templates/relations",
	}
	_ = fs.MkdirAll(ctx.EntitiesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)
	bindRepoWithFS(app, fs, ctx)
	app.broker = newEventBroker()
	return app
}

// newManualPrefixedTestApp builds an app whose schema includes a manual-ID type
// that declares a required prefix (TAG-), used to test prefix enforcement on
// manual IDs.
func newManualPrefixedTestApp(t *testing.T) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"tag": {
				Label:    "Tag",
				IDType:   "manual",
				IDPrefix: "TAG-",
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Test App"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture())
	fs := storage.NewMemFS()
	ctx := &project.Context{
		Root:                 "/project",
		CacheDir:             "/project/.rela",
		EntitiesDir:          "/project/entities",
		RelationsDir:         "/project/relations",
		TemplatesDir:         "/project/templates",
		EntityTemplatesDir:   "/project/templates/entities",
		RelationTemplatesDir: "/project/templates/relations",
	}
	_ = fs.MkdirAll(ctx.EntitiesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)
	bindRepoWithFS(app, fs, ctx)
	app.broker = newEventBroker()
	return app
}

func TestV1Schema_MultiPrefix(t *testing.T) {
	app := newPrefixTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Schema(rec, req)

	var schema V1Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatalf("decode: %v", err)
	}
	dec, ok := schema.Entities["decision"]
	if !ok {
		t.Fatalf("decision entity missing from schema")
	}
	want := []string{"DEC-", "ADR-"}
	if len(dec.IDPrefixes) != len(want) {
		t.Fatalf("IDPrefixes = %v, want %v", dec.IDPrefixes, want)
	}
	for i, p := range want {
		if dec.IDPrefixes[i] != p {
			t.Errorf("IDPrefixes[%d] = %q, want %q", i, dec.IDPrefixes[i], p)
		}
	}
}

func TestV1Schema_SinglePrefix_Compat(t *testing.T) {
	app := newPrefixTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Schema(rec, req)

	var schema V1Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tkt, ok := schema.Entities["ticket"]
	if !ok {
		t.Fatalf("ticket entity missing from schema")
	}
	if tkt.IDPrefix != "TKT-" {
		t.Errorf("IDPrefix = %q, want %q", tkt.IDPrefix, "TKT-")
	}
	if len(tkt.IDPrefixes) != 1 || tkt.IDPrefixes[0] != "TKT-" {
		t.Errorf("IDPrefixes = %v, want [TKT-]", tkt.IDPrefixes)
	}
}

func postCreate(t *testing.T, app *App, plural, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/"+plural, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	typeName := strings.TrimSuffix(plural, "s")
	app.handleV1CreateEntity(rec, req, typeName, plural)
	return rec
}

func TestV1CreateEntity_PrefixOverride(t *testing.T) {
	app := newPrefixTestApp(t)

	rec := postCreate(t, app, "decisions", `{"prefix":"ADR-","properties":{"title":"use Redis"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(got.ID, "ADR-") {
		t.Errorf("ID = %q, want prefix ADR-", got.ID)
	}
}

func TestV1CreateEntity_EmptyPrefixUsesFirst(t *testing.T) {
	app := newPrefixTestApp(t)

	rec := postCreate(t, app, "decisions", `{"properties":{"title":"use Postgres"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(got.ID, "DEC-") {
		t.Errorf("ID = %q, want first prefix DEC-", got.ID)
	}
}

func TestV1CreateEntity_UnknownPrefix(t *testing.T) {
	app := newPrefixTestApp(t)

	rec := postCreate(t, app, "decisions", `{"prefix":"XXX-","properties":{"title":"x"}}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "XXX-") {
		t.Errorf("body does not mention rejected prefix: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "DEC-") {
		t.Errorf("body does not list allowed prefixes: %s", rec.Body.String())
	}
}

func TestV1CreateEntity_IDRejectedForNonManual(t *testing.T) {
	app := newPrefixTestApp(t)

	rec := postCreate(t, app, "tickets", `{"id":"TKT-HACKED","properties":{"title":"x"}}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "non-manual") {
		t.Errorf("expected 'non-manual' in error, got: %s", rec.Body.String())
	}
}

func TestV1CreateEntity_ManualTypeRejectsPrefix(t *testing.T) {
	app := newPrefixTestApp(t)

	rec := postCreate(t, app, "categorys", `{"id":"books","prefix":"CAT-","properties":{"title":"Books"}}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "prefix not applicable") {
		t.Errorf("expected 'prefix not applicable' in error, got: %s", rec.Body.String())
	}
}

func TestV1CreateEntity_ManualAcceptsID(t *testing.T) {
	app := newPrefixTestApp(t)

	rec := postCreate(t, app, "categorys", `{"id":"books","properties":{"title":"Books"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "books" {
		t.Errorf("ID = %q, want books", got.ID)
	}
}

// Manual-ID types with declared prefixes must reject IDs outside the allowlist.
func TestV1CreateEntity_ManualWithPrefix_RejectsBareID(t *testing.T) {
	app := newManualPrefixedTestApp(t)

	rec := postCreate(t, app, "tags", `{"id":"books","properties":{"name":"Books"}}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "must start with") {
		t.Errorf("expected 'must start with' in error, got: %s", rec.Body.String())
	}
}

func TestV1CreateEntity_ManualWithPrefix_AcceptsPrefixedID(t *testing.T) {
	app := newManualPrefixedTestApp(t)

	rec := postCreate(t, app, "tags", `{"id":"TAG-books","properties":{"name":"Books"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestValidateCreateIDOpts(t *testing.T) {
	short := &metamodel.EntityDef{IDType: "short", IDPrefixes: []string{"DEC-", "ADR-"}}
	manual := &metamodel.EntityDef{IDType: "manual"}
	manualPrefixed := &metamodel.EntityDef{IDType: "manual", IDPrefix: "TAG-"}
	manualMulti := &metamodel.EntityDef{IDType: "manual", IDPrefixes: []string{"A-", "B-"}}

	tests := []struct {
		name    string
		def     *metamodel.EntityDef
		id      string
		prefix  string
		wantErr string
	}{
		{"short, no id/prefix", short, "", "", ""},
		{"short, valid prefix", short, "", "ADR-", ""},
		{"short, id set", short, "DEC-X", "", "id not accepted"},
		{"short, unknown prefix", short, "", "X-", "not valid"},
		{"manual, id only", manual, "custom", "", ""},
		{"manual, prefix set", manual, "custom", "X-", "prefix not applicable"},
		{"manual prefixed, matching id", manualPrefixed, "TAG-books", "", ""},
		{"manual prefixed, missing prefix", manualPrefixed, "books", "", "must start with"},
		{"manual prefixed, wrong prefix", manualPrefixed, "CAT-books", "", "must start with"},
		{"manual multi, matches A-", manualMulti, "A-foo", "", ""},
		{"manual multi, matches B-", manualMulti, "B-foo", "", ""},
		{"manual multi, no match", manualMulti, "C-foo", "", "must start with"},
		// Coverage gaps closed per code-review #10.
		{"short, both id and prefix set — id rejection wins", short, "DEC-X", "DEC-", "id not accepted"},
		{"manual prefixed, id matches AND prefix also set", manualPrefixed, "TAG-x", "TAG-", "prefix not applicable"},
		{"manual prefixed, bare prefix as id", manualPrefixed, "TAG-", "", "must start with"},
		{"manual prefixed, whitespace-only id treated as empty", manualPrefixed, "   ", "", ""},
		{"short, whitespace prefix treated as empty", short, "", "  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateCreateIDOpts(tt.def, tt.id, tt.prefix)
			if tt.wantErr == "" && got != "" {
				t.Errorf("got error %q, want none", got)
			}
			if tt.wantErr != "" && !strings.Contains(got, tt.wantErr) {
				t.Errorf("got error %q, want containing %q", got, tt.wantErr)
			}
		})
	}
}

// --- handleV1Views (entity-type-keyed) ---

func TestV1Views_DefaultViewForUnconfiguredType(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var resp V1ViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Entry.ID != "TKT-001" || resp.Entry.Type != "ticket" {
		t.Errorf("entry: want TKT-001/ticket, got %+v", resp.Entry)
	}
	if len(resp.Sections) == 0 {
		t.Fatal("expected at least one section in default view")
	}
	// First section must be properties (the synthesizer always emits it
	// when the type has any properties — ticket has title and status).
	if resp.Sections[0].Display != "properties" {
		t.Errorf("section[0].display: want properties, got %q", resp.Sections[0].Display)
	}
}

func TestV1Views_ConfiguredViewForType(t *testing.T) {
	app := newTestAppV1(t)
	// Register an explicit view for ticket — replaces the default.
	state := app.State()
	state.Cfg.Views["ticket_detail"] = ViewConfig{
		Title: "Ticket",
		Entry: ViewEntry{Type: "ticket"},
		Sections: []ViewSection{
			{Heading: "Just Title", Source: "entry", Display: "properties",
				Fields: []ViewSectionField{{Property: "title"}}},
		},
	}
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var resp V1ViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Configured view has a single section with heading "Just Title" —
	// distinguishes it from the synthesized default.
	if len(resp.Sections) != 1 {
		t.Fatalf("expected 1 section from explicit config, got %d: %+v", len(resp.Sections), resp.Sections)
	}
	if resp.Sections[0].Heading != "Just Title" {
		t.Errorf("heading: want Just Title, got %q", resp.Sections[0].Heading)
	}
}

// assertViewSectionsLackKeys decodes a view response body and asserts that
// none of its sections contain any of the named JSON keys. Decoding via
// map[string]json.RawMessage so a future re-introduction that renamed the
// Go field would still get caught.
func assertViewSectionsLackKeys(t *testing.T, body []byte, keys ...string) {
	t.Helper()
	var raw struct {
		Sections []map[string]json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i, sec := range raw.Sections {
		for _, k := range keys {
			if _, ok := sec[k]; ok {
				t.Errorf("section[%d]: %q must not be present in view responses", i, k)
			}
		}
	}
}

// View responses must not carry add/link affordances. The view path is
// strictly read-only; mutations live on the form/side-panel path. This guards
// against re-introducing addInfo / linkInfo on V1ViewSection across every
// shape that historically emitted them: outgoing/incoming traversals,
// cards/list/table displays, and the variant where the target type has no
// create-form configured (which previously emitted only linkInfo).
func TestV1Views_NoAddOrLinkInfoOnSections(t *testing.T) {
	type variant struct {
		name     string
		traverse ViewTraverse
		display  string
		// withCreateForm seeds a Forms entry so createFormForType resolves
		// — exercising the path that pre-change emitted both addInfo and
		// linkInfo. Without it, only linkInfo was emitted historically.
		withCreateForm bool
	}
	variants := []variant{
		{"outgoing-cards-with-form", ViewTraverse{From: "entry", Follow: "implements", CollectAs: "features"}, "cards", true},
		{"outgoing-list-with-form", ViewTraverse{From: "entry", Follow: "implements", CollectAs: "features"}, "list", true},
		{"outgoing-table-with-form", ViewTraverse{From: "entry", Follow: "implements", CollectAs: "features"}, "table", true},
		{"incoming-cards-with-form", ViewTraverse{From: "entry", FollowIncoming: "implements", CollectAs: "tickets"}, "cards", true},
		{"outgoing-cards-no-form", ViewTraverse{From: "entry", Follow: "implements", CollectAs: "features"}, "cards", false},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			app := newTestAppV1(t)
			state := app.State()
			if v.withCreateForm {
				// Wire forms for both ends so the variant works regardless
				// of traversal direction (the resolver picks createFormForType
				// against `relDef.From` for incoming, `relDef.To` for outgoing).
				state.Cfg.Forms["create_feature"] = dataentryconfig.Form{EntityType: "feature"}
				state.Cfg.Forms["create_ticket"] = dataentryconfig.Form{EntityType: "ticket"}
			}

			// The view's entry type depends on traversal direction so the
			// FollowIncoming variant has a sensible from-side entity to
			// land on.
			entryType := "ticket"
			entryID := "TKT-001"
			otherType := "feature"
			otherID := "FEAT-001"
			if v.traverse.FollowIncoming != "" {
				entryType, otherType = otherType, entryType
				entryID, otherID = otherID, entryID
			}

			columns := []ListColumn{}
			if v.display == "table" {
				columns = []ListColumn{{Property: "title", Label: "Title"}}
			}
			state.Cfg.Views["v"] = ViewConfig{
				Entry:    ViewEntry{Type: entryType},
				Traverse: []ViewTraverse{v.traverse},
				Sections: []ViewSection{{
					Heading: "Section", Source: v.traverse.CollectAs, Display: v.display, Columns: columns,
				}},
			}

			seedEntity(app, &entity.Entity{
				ID: entryID, Type: entryType,
				Properties: map[string]interface{}{"title": "entry"},
			})
			seedEntity(app, &entity.Entity{
				ID: otherID, Type: otherType,
				Properties: map[string]interface{}{"title": "other"},
			})
			// Edge always points TKT → FEAT regardless of which is entry.
			seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/"+entryType+"/"+entryID, http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1Views(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			assertViewSectionsLackKeys(t, rec.Body.Bytes(), "addInfo", "linkInfo")
		})
	}
}

func TestV1Views_UnknownEntityType(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/nonexistent/X-1", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestV1Views_UnknownEntityID(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/MISSING", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: want 422 for missing entity, got %d", rec.Code)
	}
}

func TestV1Views_BadPath(t *testing.T) {
	app := newTestAppV1(t)

	tests := []struct {
		name string
		path string
	}{
		{"missing entity id", "/api/v1/_views/ticket"},
		{"empty entity type", "/api/v1/_views//TKT-001"},
		{"trailing slash only", "/api/v1/_views/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1Views(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status: want 400, got %d", rec.Code)
			}
		})
	}
}

func TestV1Views_MethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: want 405, got %d", rec.Code)
	}
}

func TestV1Views_MentionsPopulated(t *testing.T) {
	app := newTestAppV1(t)
	target := &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Target Ticket",
			"status": "open",
		},
	}
	seedEntity(app, target)
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Origin Ticket",
			"status": "open",
		},
		Content: "see `TKT-002` for the dependency; `TKT-NOPE` is unknown",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var resp V1ViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := resp.Mentions["TKT-002"]; got.Type != target.Type || got.Title != target.Title() {
		t.Errorf("mentions[TKT-002]: want {%q,%q}, got %+v", target.Type, target.Title(), got)
	}
	if _, ok := resp.Mentions["TKT-NOPE"]; ok {
		t.Errorf("mentions must not include unknown ID TKT-NOPE; got %+v", resp.Mentions)
	}
}

func TestV1Views_MentionsAbsentWhenNoRefs(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Plain Ticket",
			"status": "open",
		},
		Content: "no entity references here",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Views(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	// Assert the JSON omits the mentions key entirely; the SPA treats a
	// missing key the same as an empty map, but `omitempty` is the
	// documented wire contract.
	if strings.Contains(rec.Body.String(), `"mentions"`) {
		t.Errorf("response must omit mentions when none collected; body: %s", rec.Body.String())
	}
}

// TKT-G7N5 wire-shape tests (AC1, AC2).

// TestV1Affordance_PerEntityGet_NoneProfile asserts AC1: under the nop
// resolver, per-entity GET responses contain `_fields: {}` and
// `_relations: {}` (present-but-empty closed-world signal), and no
// properties are omitted.
func TestV1Affordance_PerEntityGet_NoneProfile(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = NopFieldVerdictResolver{}

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	raw := rec.Body.String()

	var got V1Entity
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.FieldAffordances == nil {
		t.Fatal("FieldAffordances pointer should be non-nil (closed-world signal)")
	}
	if len(*got.FieldAffordances) != 0 {
		t.Errorf("FieldAffordances: got %v, want empty (sparse, no deviations)", *got.FieldAffordances)
	}
	if got.RelationAffordances == nil {
		t.Fatal("RelationAffordances pointer should be non-nil")
	}
	if len(*got.RelationAffordances) != 0 {
		t.Errorf("RelationAffordances: got %v, want empty", *got.RelationAffordances)
	}

	if _, ok := got.Properties["title"]; !ok {
		t.Errorf("properties.title missing under nop")
	}
	if _, ok := got.Properties["status"]; !ok {
		t.Errorf("properties.status missing under nop")
	}

	// Wire format: both keys must appear in the raw JSON as `{}` so
	// the SPA distinguishes "anonymous fallback" (absent) from
	// "evaluated with no deviations" (present-empty). Round-tripping
	// V1Entity through omitempty proves this for pointer fields.
	if !strings.Contains(raw, `"_fields":{}`) {
		t.Errorf("raw body should contain \"_fields\":{}; got: %s", raw)
	}
	if !strings.Contains(raw, `"_relations":{}`) {
		t.Errorf("raw body should contain \"_relations\":{}; got: %s", raw)
	}
}

// TestV1Affordance_PerEntityGet_DemoFixture asserts AC2: the demo
// resolver populates the expected sparse fixture verdicts. Uses a
// hand-rolled fixture resolver to keep the test independent of the
// project's real metamodel (which doesn't have all the demo fields).
func TestV1Affordance_PerEntityGet_DemoFixture(t *testing.T) {
	app := newTestAppV1(t)
	falseVal := false

	// Inline fixture: read-only on `status`, hidden on `priority`,
	// option-filter on `status`. The newTestAppV1 metamodel declares
	// `title` and `status` on ticket; we add `priority` to the entity
	// via the seed.
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Writable: map[string]bool{"status": false},
			Visible:  map[string]bool{"priority": false},
			Options:  map[string]map[string]bool{"status": {"done": false}},
		},
		rv: RelationVerdicts{
			Types: map[string]RelationVerdict{
				"implements": {Creatable: true, Removable: false},
				"blocks":     {Creatable: false, Removable: true},
			},
		},
	}

	seedEntity(app, &entity.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":    "Demo Ticket",
			"status":   "open",
			"priority": "high",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-002", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-002")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var got V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Hidden property is omitted.
	if _, ok := got.Properties["priority"]; ok {
		t.Errorf("properties.priority must be omitted (hidden); got %v", got.Properties)
	}
	if _, ok := got.Properties["title"]; !ok {
		t.Errorf("properties.title should still be present")
	}

	// FieldAffordances: sparse — status is read-only AND has option filter,
	// merged into one entry.
	if got.FieldAffordances == nil {
		t.Fatal("FieldAffordances nil")
	}
	fa := *got.FieldAffordances
	if _, ok := fa["priority"]; ok {
		t.Errorf("priority must be absent from _fields (hidden): %v", fa)
	}
	if _, ok := fa["title"]; ok {
		t.Errorf("title must be absent from _fields (default writable): %v", fa)
	}
	status, ok := fa["status"]
	if !ok {
		t.Fatalf("status missing from _fields: %v", fa)
	}
	if status.Writable == nil || *status.Writable != falseVal {
		t.Errorf("status.writable: got %v, want false-pointer", status.Writable)
	}
	if allowed, present := status.Options["done"]; !present || allowed {
		t.Errorf("status.options.done: got %v present=%v, want false", allowed, present)
	}

	// RelationAffordances: sparse — implements (removable=false),
	// blocks (creatable=false).
	if got.RelationAffordances == nil {
		t.Fatal("RelationAffordances nil")
	}
	ra := *got.RelationAffordances
	impl, ok := ra["implements"]
	if !ok {
		t.Fatalf("implements missing from _relations: %v", ra)
	}
	if impl.Creatable != nil {
		t.Errorf("implements.creatable: got %v, want nil (default)", impl.Creatable)
	}
	if impl.Removable == nil || *impl.Removable {
		t.Errorf("implements.removable: got %v, want false-pointer", impl.Removable)
	}
	blocks, ok := ra["blocks"]
	if !ok {
		t.Fatalf("blocks missing from _relations: %v", ra)
	}
	if blocks.Creatable == nil || *blocks.Creatable {
		t.Errorf("blocks.creatable: got %v, want false-pointer", blocks.Creatable)
	}
}

// TestV1Affordance_PatchEcho_StripsHidden proves the C2 fix: PATCH
// responses must run through the same strip-hidden invariant as GET.
// Before the fix, the PATCH success response echoed the full entity
// including hidden properties, so a stale client could observe the
// hidden value via its own write response.
func TestV1Affordance_PatchEcho_StripsHidden(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Visible: map[string]bool{"title": false}, // hide the title field
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "secret", "status": "open"},
	})

	// PATCH an unrelated property (status) and inspect the echo.
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"properties":{"status":"in-progress"}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"title"`) || strings.Contains(body, "secret") {
		t.Errorf("PATCH echo must NOT contain hidden field; got: %s", body)
	}
}

// TestV1Affordance_Includes_StripHidden proves the C3 fix: ?include=*
// (and named includes) must strip hidden properties from related
// entities. Before the fix, a related entity's hidden field could
// leak through the `included` map.
func TestV1Affordance_Includes_StripHidden(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Visible: map[string]bool{"title": false}, // hide title on every entity
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "ticket-secret"},
	})
	seedEntity(app, &entity.Entity{
		ID:         "FEAT-001",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "feature-secret"},
	})
	bindEdge(app, "TKT-001", "implements", "FEAT-001")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001?include=*", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "ticket-secret") {
		t.Errorf("body must NOT contain ticket's hidden title; got: %s", body)
	}
	if strings.Contains(body, "feature-secret") {
		t.Errorf("body must NOT contain included feature's hidden title; got: %s", body)
	}
}

// TestV1Affordance_CollectionList_StripHidden asserts that list rows
// (which use serializeRelatedEntityForWire) strip hidden properties
// too — the wire invariant holds regardless of which response shape
// the entity rides in.
func TestV1Affordance_CollectionList_StripHidden(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Visible: map[string]bool{"title": false},
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "list-secret"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityCollection(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "list-secret") {
		t.Errorf("list row must NOT contain hidden title; got: %s", body)
	}
}

// seedDemoTicketForPatch wires a fakeResolver fixture appropriate for
// the AC3-AC5 PATCH tests and seeds TKT-PATCH. Returns the
// patchTicket-style raw helper bound to this entity.
func seedDemoTicketForPatch(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = newVerdicts().
		ReadOnly("status").
		EnumDeny("title", "forbidden-title").
		Build()
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Original",
			"status": "open",
		},
	})
	return app
}

// AC3: hidden + unknown fields produce structurally identical 403
// responses (F8 side channel closure).
func TestV1Affordance_PatchHiddenAndUnknownField_SameShape(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Visible: map[string]bool{"title": false}, // hide a declared field
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})

	hiddenCode, hiddenBody := patchTicketRaw(t, app, `{"properties":{"title":"new"}}`)
	if hiddenCode != http.StatusForbidden {
		t.Fatalf("hidden field: got %d, want 403; body=%s", hiddenCode, hiddenBody)
	}
	unknownCode, unknownBody := patchTicketRaw(t, app, `{"properties":{"bogus_field":"v"}}`)
	if unknownCode != http.StatusForbidden {
		t.Fatalf("unknown field: got %d, want 403; body=%s", unknownCode, unknownBody)
	}

	// Both must use the same rule_kind and rule_id prefix; only the
	// field name in the path varies. This is the F8 side-channel
	// closure assertion.
	for _, body := range []string{hiddenBody, unknownBody} {
		if !strings.Contains(body, `"rule_kind":"affordance"`) {
			t.Errorf("body must name rule_kind=affordance: %s", body)
		}
		if !strings.Contains(body, `"rule_id":"field-affordance:hidden:`) {
			t.Errorf("body must use field-affordance:hidden prefix: %s", body)
		}
	}
}

// AC4: read-only fields reject writes regardless of value.
func TestV1Affordance_PatchReadOnlyField_Forbidden(t *testing.T) {
	t.Run("different value", func(t *testing.T) {
		app := seedDemoTicketForPatch(t)
		code, body := patchTicketRaw(t, app, `{"properties":{"status":"closed"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"rule_id":"field-affordance:read-only:status"`) {
			t.Errorf("body must name read-only rule: %s", body)
		}
	})

	t.Run("same value", func(t *testing.T) {
		// F12: read-only is strict — same-value writes still 403.
		// useAutoSave suppresses no-op PATCHes client-side, so no real
		// SPA path exercises this; the rejection exists for defense
		// against hand-crafted clients.
		app := seedDemoTicketForPatch(t)
		code, body := patchTicketRaw(t, app, `{"properties":{"status":"open"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
	})

	t.Run("mixed with writable; whole PATCH rejected", func(t *testing.T) {
		app := seedDemoTicketForPatch(t)
		code, body := patchTicketRaw(t, app, `{"properties":{"status":"closed","title":"New"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		// Verify title was NOT updated.
		e, _ := app.reader.getEntity(context.Background(), "TKT-001")
		if e.Properties["title"] != "Original" {
			t.Errorf("title must not be applied when status fails: got %v", e.Properties["title"])
		}
	})

	t.Run("unset is a write", func(t *testing.T) {
		// F16: properties_unset on a read-only field is also a write.
		app := seedDemoTicketForPatch(t)
		code, body := patchTicketRaw(t, app, `{"properties_unset":["status"]}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"field-affordance:read-only:status"`) {
			t.Errorf("body must name read-only rule: %s", body)
		}
	})
}

// TestV1Affordance_AffordanceDenial_EmitsAudit proves the C5 fix: every
// affordance-gate rejection produces a `denied-write` audit row
// attributed to the request principal. The wire stream is uniform
// with ACL denials (which the entitymanager emits the same op for),
// so log readers see one stream for every denied write regardless of
// whether the gate fired in the manager or in the dataentry handler.
func TestV1Affordance_AffordanceDenial_EmitsAudit(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Writable: map[string]bool{"status": false},
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})

	// Trigger a read-only denial via PATCH.
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"properties":{"status":"closed"}}`))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("PATCH: got %d, want 403; body=%s", rec.Code, rec.Body.String())
	}

	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("audit records: got %d, want 1; records=%+v", len(records), records)
	}
	rec0 := records[0]
	if rec0.Op != audit.OpDeniedWrite {
		t.Errorf("audit op: got %q, want %q", rec0.Op, audit.OpDeniedWrite)
	}
	if rec0.Subject == nil || rec0.Subject.ID != "TKT-001" || rec0.Subject.Type != "ticket" {
		t.Errorf("audit subject: got %+v, want entity:ticket:TKT-001", rec0.Subject)
	}
	if !strings.Contains(rec0.Summary, "rule_kind=affordance") {
		t.Errorf("audit summary missing rule_kind=affordance: %q", rec0.Summary)
	}
	if !strings.Contains(rec0.Summary, "rule_id=field-affordance:read-only:status") {
		t.Errorf("audit summary missing rule_id: %q", rec0.Summary)
	}
}

// AC5: filtered enum options reject writes.
func TestV1Affordance_PatchFilteredOption_Forbidden(t *testing.T) {
	app := seedDemoTicketForPatch(t)

	t.Run("filtered value rejected", func(t *testing.T) {
		code, body := patchTicketRaw(t, app, `{"properties":{"title":"forbidden-title"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"field-affordance:enum-filtered:title=forbidden-title"`) {
			t.Errorf("body must name enum-filtered rule with value: %s", body)
		}
	})

	t.Run("allowed value succeeds", func(t *testing.T) {
		code, body := patchTicketRaw(t, app, `{"properties":{"title":"any-other-value"}}`)
		if code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", code, body)
		}
	})
}

// TestV1Affordance_PatchFilteredListEnum_Forbidden proves the
// list-typed enum option-filter now rejects rather than silently
// allowing. Before this change, a `tags: [allowed, denied]` PATCH
// would slip through because the validator only knew how to inspect
// scalar string values. This is the security-soft-spot the cranky
// reviewer flagged as S5.
func TestV1Affordance_PatchFilteredListEnum_Forbidden(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{
		fv: FieldVerdicts{
			Options: map[string]map[string]bool{
				"tags": {"forbidden-tag": false},
			},
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})

	t.Run("forbidden element rejects the whole list", func(t *testing.T) {
		code, body := patchTicketRaw(t, app,
			`{"properties":{"tags":["ok-tag","forbidden-tag"]}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"field-affordance:enum-filtered:tags=forbidden-tag"`) {
			t.Errorf("body must name the forbidden element: %s", body)
		}
	})

	t.Run("all-allowed list succeeds", func(t *testing.T) {
		code, body := patchTicketRaw(t, app,
			`{"properties":{"tags":["a","b","c"]}}`)
		if code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", code, body)
		}
	})
}

// createTicketRaw POSTs a raw body to the v1 create handler and returns
// the status code and response body. Mirrors patchTicketRaw for the
// create path.
func createTicketRaw(t *testing.T, app *App, body string) (code int, respBody string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1CreateEntity(rec, req, "ticket", "tickets")
	return rec.Code, rec.Body.String()
}

// TestHandleV1CreateEntity_FieldAffordances proves the BUG-Q60V fix: a
// `fields:` policy that hides, freezes, or filters a field gates it on
// CREATE, not just PATCH. Before the fix, POST went straight to
// entityManager.CreateEntity, so a denied field could be smuggled in at
// create time. The create gate must produce the same 403 + rule_id as
// the PATCH gate (TestV1Affordance_Patch* above).
func TestHandleV1CreateEntity_FieldAffordances(t *testing.T) {
	t.Run("hidden field rejected", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().Hidden("title").Build()

		code, body := createTicketRaw(t, app, `{"properties":{"title":"new"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"rule_id":"field-affordance:hidden:title"`) {
			t.Errorf("body must name hidden rule: %s", body)
		}
	})

	t.Run("unknown field rejected with same shape (F8)", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = NopFieldVerdictResolver{}

		code, body := createTicketRaw(t, app, `{"properties":{"bogus_field":"v"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"rule_id":"field-affordance:hidden:`) {
			t.Errorf("unknown field must use hidden-shape rule: %s", body)
		}
	})

	t.Run("read-only field rejected", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().ReadOnly("status").Build()

		code, body := createTicketRaw(t, app, `{"properties":{"title":"ok","status":"open"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"rule_id":"field-affordance:read-only:status"`) {
			t.Errorf("body must name read-only rule: %s", body)
		}
	})

	t.Run("filtered enum option rejected", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().EnumDeny("title", "forbidden-title").Build()

		code, body := createTicketRaw(t, app, `{"properties":{"title":"forbidden-title"}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"field-affordance:enum-filtered:title=forbidden-title"`) {
			t.Errorf("body must name enum-filtered rule: %s", body)
		}
	})

	t.Run("allowed fields create succeeds", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().ReadOnly("status").Build()

		// `title` is writable; `status` is omitted, so the read-only gate
		// never fires. The create must succeed end-to-end.
		code, body := createTicketRaw(t, app, `{"properties":{"title":"ok"}}`)
		if code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", code, body)
		}
	})
}

// TestHandleV1CreateEntity_AffordanceDenial_EmitsAudit proves the create
// gate audits its denials on the same `denied-write` op as the PATCH
// gate, so the audit stream is uniform across write paths. The audit
// subject carries the type but an empty ID (the entity was never
// created).
func TestHandleV1CreateEntity_AffordanceDenial_EmitsAudit(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)
	app.fieldResolver = newVerdicts().ReadOnly("status").Build()

	code, body := createTicketRaw(t, app, `{"properties":{"title":"ok","status":"open"}}`)
	if code != http.StatusForbidden {
		t.Fatalf("create: got %d, want 403; body=%s", code, body)
	}

	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("audit records: got %d, want 1; records=%+v", len(records), records)
	}
	rec0 := records[0]
	if rec0.Op != audit.OpDeniedWrite {
		t.Errorf("audit op: got %q, want %q", rec0.Op, audit.OpDeniedWrite)
	}
	if rec0.Subject == nil || rec0.Subject.Type != "ticket" {
		t.Errorf("audit subject: got %+v, want entity:ticket:<empty-id>", rec0.Subject)
	}
	if !strings.Contains(rec0.Summary, "rule_id=field-affordance:read-only:status") {
		t.Errorf("audit summary missing rule_id: %q", rec0.Summary)
	}
}

// dryRunCreateRaw POSTs a raw body to the dry-run create handler
// (?dry_run=true) and returns status + response body. Mirrors
// createTicketRaw for the dry-run path (TKT-3I5U).
func dryRunCreateRaw(t *testing.T, app *App, body string) (code int, resp *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets?dry_run=true", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1EntityCollection(rec, req, "ticket", "tickets")
	return rec.Code, rec
}

// countStoreEntities returns the number of entities currently in the
// app's store — used to assert the dry-run persists nothing.
func countStoreEntities(t *testing.T, app *App) int {
	t.Helper()
	n := 0
	for _, err := range app.store.ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			continue
		}
		n++
	}
	return n
}

// TestHandleV1DryRunCreate_Affordances proves the dry-run create
// endpoint (TKT-3I5U) returns the same `_fields` verdicts the create
// form needs to gate inputs — read-only, hidden, enum-filtered — WITHOUT
// persisting, and returns 200 (advisory) rather than 403.
func TestHandleV1DryRunCreate_Affordances(t *testing.T) {
	t.Run("read-only field surfaces in _fields", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().ReadOnly("status").Build()

		code, rec := dryRunCreateRaw(t, app, `{"properties":{"title":"x"}}`)
		if code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", code, rec.Body.String())
		}
		var v V1Entity
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
		}
		if v.FieldAffordances == nil {
			t.Fatalf("response missing _fields; body=%s", rec.Body.String())
		}
		fa := *v.FieldAffordances
		if entry, ok := fa["status"]; !ok || entry.Writable == nil || *entry.Writable {
			t.Errorf("_fields.status must be writable=false; got %+v", fa["status"])
		}
	})

	t.Run("hidden field omitted from _fields and properties", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().Hidden("status").Build()

		code, rec := dryRunCreateRaw(t, app, `{"properties":{"title":"x"}}`)
		if code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", code, rec.Body.String())
		}
		body := rec.Body.String()
		var v V1Entity
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("decode: %v; body=%s", err, body)
		}
		if v.FieldAffordances != nil {
			if _, present := (*v.FieldAffordances)["status"]; present {
				t.Errorf("hidden field must be absent from _fields; got %s", body)
			}
		}
		if _, present := v.Properties["status"]; present {
			t.Errorf("hidden field must be stripped from properties; got %s", body)
		}
	})

	t.Run("enum-filtered options surface in _fields", func(t *testing.T) {
		app := newTestAppV1(t)
		app.broker = newEventBroker()
		bindRepo(app, t.TempDir())
		app.fieldResolver = newVerdicts().EnumDeny("status", "done").Build()

		code, rec := dryRunCreateRaw(t, app, `{"properties":{"title":"x"}}`)
		if code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", code, rec.Body.String())
		}
		var v V1Entity
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
		if v.FieldAffordances == nil || (*v.FieldAffordances)["status"].Options["done"] {
			t.Errorf("_fields.status.options.done must be false; body=%s", rec.Body.String())
		}
	})
}

// TestHandleV1DryRunCreate_PersistsNothing proves the dry-run is
// read-shaped: no entity is written and (RR-4O6E) no audit row is
// emitted, even when a field would be denied.
func TestHandleV1DryRunCreate_PersistsNothing(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)
	app.fieldResolver = newVerdicts().ReadOnly("status").Build()

	before := countStoreEntities(t, app)

	// A body that sets a read-only field: the real create would 403 +
	// audit; the dry-run returns 200 with the verdict and writes nothing.
	code, rec := dryRunCreateRaw(t, app, `{"properties":{"title":"x","status":"open"}}`)
	if code != http.StatusOK {
		t.Fatalf("dry-run got %d, want 200; body=%s", code, rec.Body.String())
	}

	if after := countStoreEntities(t, app); after != before {
		t.Errorf("dry-run must not persist: entity count %d -> %d", before, after)
	}
	if recs := sink.Records(); len(recs) != 0 {
		t.Errorf("dry-run must emit no audit rows; got %d: %+v", len(recs), recs)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("dry-run Cache-Control = %q, want it to include no-store", cc)
	}
	if etag := rec.Header().Get("ETag"); etag != "" {
		t.Errorf("dry-run must not set an ETag; got %q", etag)
	}
}

// TestHandleV1DryRunCreate_SoftWarnings proves the dry-run surfaces the
// same DEC-HWZHA soft warnings the real create returns (e.g. a
// required-but-unset field), so the create form can show as-you-type
// validation feedback (option B parity).
func TestHandleV1DryRunCreate_SoftWarnings(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	// `title` is required on the ticket type; omit it → required-unset
	// soft warning, not a hard error.
	code, rec := dryRunCreateRaw(t, app, `{"properties":{"status":"open"}}`)
	if code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", code, rec.Body.String())
	}
	var v V1Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	found := false
	for _, w := range v.Warnings {
		if w.Code == "required_property_unset" && strings.Contains(w.Path, "title") {
			found = true
		}
	}
	if !found {
		t.Errorf("dry-run must surface required_property_unset for title; warnings=%+v", v.Warnings)
	}
}

// TestHandleV1DryRunCreate_ResponseIncludesAllVisibleDeclaredProps
// pins the contract the SPA's create-form filter relies on: every
// declared, visible property appears in the response's `properties`
// (with nil for keys the user hasn't set), so the form's
// `_fields OR properties` filter can identify visible-by-default
// fields. Without this, a visible-by-default field with no value
// (e.g. `title` before the user types it) would be filtered out of
// the create form entirely.
func TestHandleV1DryRunCreate_ResponseIncludesAllVisibleDeclaredProps(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	// Hide one field; the rest should all appear in `properties`.
	app.fieldResolver = newVerdicts().Hidden("status").Build()

	code, rec := dryRunCreateRaw(t, app, `{"properties":{}}`)
	if code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", code, rec.Body.String())
	}
	var v V1Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	// Every declared property on the ticket type EXCEPT the hidden one
	// must appear in properties (with nil values).
	def := app.State().Meta.Entities["ticket"]
	for name := range def.Properties {
		_, present := v.Properties[name]
		if name == "status" {
			if present {
				t.Errorf("hidden field %q must NOT appear in response properties; got %v", name, v.Properties)
			}
			continue
		}
		if !present {
			t.Errorf("visible declared field %q must appear in response properties (with nil); got %v", name, v.Properties)
		}
	}
}

// TestHandleV1DryRunCreate_UnknownType returns 404, matching the real
// create handler.
func TestHandleV1DryRunCreate_UnknownType(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nonexistents?dry_run=true",
		strings.NewReader(`{"properties":{}}`))
	rec := httptest.NewRecorder()
	app.handleV1EntityCollection(rec, req, "nonexistent", "nonexistents")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

// bindEdge inserts an edge directly through the store. Used by tests
// that need a pre-existing edge to remove or update via the API.
// relType is parameterized intentionally; today every caller passes
// "implements" but the helper exists to be used with any relation
// type as new tests need them.
//
//nolint:unparam // see preceding doc comment
func bindEdge(app *App, from, relType, to string) {
	if _, err := app.store.CreateRelation(context.Background(), from, relType, to, nil); err != nil {
		panic(err)
	}
}

// seedTicketWithRelationVerdicts wires fake relation verdicts and
// seeds a ticket plus a peer feature so create/delete can target a
// real edge.
func seedTicketWithRelationVerdicts(t *testing.T, rv RelationVerdicts) *App {
	t.Helper()
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{rv: rv}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID:         "FEAT-001",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "Feature"},
	})
	return app
}

// AC6: per-relation POST is gated by creatable; per-relation DELETE
// is gated by removable. Per-relation-type uniform: the 403 response
// names the relation type but no link identifier (F5).
func TestV1Affordance_PerRelationCreate_ForbiddenWhenNotCreatable(t *testing.T) {
	app := seedTicketWithRelationVerdicts(t, RelationVerdicts{
		Types: map[string]RelationVerdict{
			"implements": {Creatable: false, Removable: true},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/TKT-001/relations/implements",
		strings.NewReader(`{"id":"FEAT-001"}`))
	rec := httptest.NewRecorder()
	app.handleV1CreateRelation(rec, req, "ticket", "TKT-001", "implements")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"rule_id":"relation-affordance:not-creatable:implements"`) {
		t.Errorf("body must name not-creatable rule with type; got: %s", body)
	}
	if strings.Contains(body, "FEAT-001") {
		t.Errorf("body must NOT contain the link identifier (per-type uniform); got: %s", body)
	}
}

func TestV1Affordance_PerRelationDelete_ForbiddenWhenNotRemovable(t *testing.T) {
	app := seedTicketWithRelationVerdicts(t, RelationVerdicts{
		Types: map[string]RelationVerdict{
			"implements": {Creatable: true, Removable: false},
		},
	})
	// Pre-link the edge so DELETE has something to remove.
	bindEdge(app, "TKT-001", "implements", "FEAT-001")

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/tickets/TKT-001/relations/implements/FEAT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DeleteRelation(rec, req, "ticket", "TKT-001", "implements", "FEAT-001")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"rule_id":"relation-affordance:not-removable:implements"`) {
		t.Errorf("body must name not-removable rule with type; got: %s", body)
	}
}

func TestV1Affordance_PerRelationCreate_AllowedWhenCreatable(t *testing.T) {
	app := seedTicketWithRelationVerdicts(t, RelationVerdicts{
		Types: map[string]RelationVerdict{
			"implements": {Creatable: true, Removable: true},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/TKT-001/relations/implements",
		strings.NewReader(`{"id":"FEAT-001"}`))
	rec := httptest.NewRecorder()
	app.handleV1CreateRelation(rec, req, "ticket", "TKT-001", "implements")
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
}

// TestV1Affordance_PerRelationCreate_IncomingResolvesAgainstSource proves
// the C4 fix: an incoming-direction POST creates an edge whose SOURCE
// is the peer, not the path entity. The affordance verdict must be
// evaluated against the source's type.
//
// Setup: path entity is `concept`, peer is `ticket`. The ticket type's
// verdict denies create of `affects`; the concept's verdict permits
// it. A direction=incoming POST to /concepts/<id>/relations/affects
// creates a ticket --affects--> concept edge — the source is the
// ticket, so the deny must fire.
//
// Before the C4 fix this returned 201 (verdict evaluated against the
// concept, which permitted it). After the fix it returns 403.
func TestV1Affordance_PerRelationCreate_IncomingResolvesAgainstSource(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = byTypeResolver{
		rvByType: map[string]RelationVerdicts{
			"ticket": {
				Types: map[string]RelationVerdict{
					"affects": {Creatable: false, Removable: true},
				},
			},
			// concept verdict: implicit defaults (everything permitted).
		},
	}
	// The test metamodel doesn't declare `affects`, but the demo
	// metamodel does. We seed both ticket + concept entities; the
	// relation create path doesn't validate against the metamodel
	// here — it's the resolver's job that we're testing.
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "CONC-001", Type: "concept", Properties: map[string]interface{}{"title": "C"}})

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/concepts/CONC-001/relations/affects",
		strings.NewReader(`{"id":"TKT-001","direction":"incoming"}`))
	rec := httptest.NewRecorder()
	app.handleV1CreateRelation(rec, req, "concept", "CONC-001", "affects")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403 (incoming-direction must resolve verdict against source); body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"rule_id":"relation-affordance:not-creatable:affects"`) {
		t.Errorf("body must name not-creatable rule: %s", rec.Body.String())
	}
}

// AC6: unified PATCH path that ADDS or REMOVES relations is gated by
// the same verdicts.
func TestV1Affordance_UnifiedPatchAddRelation_ForbiddenWhenNotCreatable(t *testing.T) {
	app := seedTicketWithRelationVerdicts(t, RelationVerdicts{
		Types: map[string]RelationVerdict{
			"implements": {Creatable: false, Removable: true},
		},
	})
	code, body := patchTicketRaw(t, app,
		`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`)
	if code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", code, body)
	}
	if !strings.Contains(body, `"rule_id":"relation-affordance:not-creatable:implements"`) {
		t.Errorf("body must name not-creatable rule: %s", body)
	}
}

func TestV1Affordance_UnifiedPatchRemoveRelation_ForbiddenWhenNotRemovable(t *testing.T) {
	app := seedTicketWithRelationVerdicts(t, RelationVerdicts{
		Types: map[string]RelationVerdict{
			"implements": {Creatable: true, Removable: false},
		},
	})
	// Pre-link the edge so the empty desired set is a true removal.
	bindEdge(app, "TKT-001", "implements", "FEAT-001")

	code, body := patchTicketRaw(t, app,
		`{"relations":{"implements":{"data":[]}}}`)
	if code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", code, body)
	}
	if !strings.Contains(body, `"rule_id":"relation-affordance:not-removable:implements"`) {
		t.Errorf("body must name not-removable rule: %s", body)
	}
}

// AC7: relation-meta gate. Same shape across the three meta-write
// paths: POST creates with meta, PATCH /relations/<t>/<id> updates
// meta, unified PATCH upserts meta.
func TestV1Affordance_RelationMeta_ForbiddenWhenNotWritable(t *testing.T) {
	rv := RelationVerdicts{
		Types: map[string]RelationVerdict{
			"implements": {
				Creatable: true,
				Removable: true,
				Fields:    map[string]bool{"note": false},
			},
		},
	}

	t.Run("POST /relations/<type> with non-writable meta", func(t *testing.T) {
		app := seedTicketWithRelationVerdicts(t, rv)
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/tickets/TKT-001/relations/implements",
			strings.NewReader(`{"id":"FEAT-001","meta":{"note":"hi"}}`))
		rec := httptest.NewRecorder()
		app.handleV1CreateRelation(rec, req, "ticket", "TKT-001", "implements")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"rule_id":"relation-affordance:meta-read-only:implements.note"`) {
			t.Errorf("body must name meta-read-only rule: %s", rec.Body.String())
		}
	})

	t.Run("PATCH /relations/<type>/<id> with non-writable meta", func(t *testing.T) {
		app := seedTicketWithRelationVerdicts(t, rv)
		bindEdge(app, "TKT-001", "implements", "FEAT-001")
		req := httptest.NewRequest(http.MethodPatch,
			"/api/v1/tickets/TKT-001/relations/implements/FEAT-001",
			strings.NewReader(`{"meta":{"note":"hi"}}`))
		rec := httptest.NewRecorder()
		app.handleV1UpdateRelation(rec, req, "ticket", "TKT-001", "implements", "FEAT-001")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"rule_id":"relation-affordance:meta-read-only:implements.note"`) {
			t.Errorf("body must name meta-read-only rule: %s", rec.Body.String())
		}
	})

	t.Run("unified PATCH upserting meta", func(t *testing.T) {
		app := seedTicketWithRelationVerdicts(t, rv)
		code, body := patchTicketRaw(t, app,
			`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001","meta":{"note":"hi"}}]}}}`)
		if code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", code, body)
		}
		if !strings.Contains(body, `"rule_id":"relation-affordance:meta-read-only:implements.note"`) {
			t.Errorf("body must name meta-read-only rule: %s", body)
		}
	})

	t.Run("writable meta succeeds", func(t *testing.T) {
		// "role" not in Fields → default writable.
		app := seedTicketWithRelationVerdicts(t, rv)
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/tickets/TKT-001/relations/implements",
			strings.NewReader(`{"id":"FEAT-001","meta":{"role":"primary"}}`))
		rec := httptest.NewRecorder()
		app.handleV1CreateRelation(rec, req, "ticket", "TKT-001", "implements")
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", rec.Code, rec.Body.String())
		}
	})
}

// AC11: collection-level GET does NOT emit _fields or _relations (the
// new wire keys are per-entity only in v1). The collection-level
// response retains its existing shape; per-entity affordances ride on
// per-entity responses.
func TestV1Affordance_CollectionGet_NoFieldVerdicts(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	app.fieldResolver = DemoFieldVerdictResolver{}

	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityCollection(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"_fields"`) {
		t.Errorf("collection response must NOT contain _fields; got: %s", body)
	}
	if strings.Contains(body, `"_relations"`) {
		t.Errorf("collection response must NOT contain _relations; got: %s", body)
	}
}
