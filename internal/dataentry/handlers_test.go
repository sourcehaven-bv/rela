package dataentry

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// newHandlerTestApp builds a full App (including parsed templates) for handler tests.
func newHandlerTestApp(t *testing.T) *App {
	t.Helper()
	meta := testMeta()
	cfg := testConfig()
	g := testGraph()

	// Add a relation for testing edge display
	g.AddEdge(model.NewRelation("TKT-001", "depends_on", "TKT-002"))

	// Add view config
	cfg.Views = map[string]ViewConfig{
		"ticket_detail": {
			Title: "Ticket Detail",
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "belongs_to", CollectAs: "components"},
			},
			Sections: []ViewSection{
				{Heading: "Properties", Source: "entry", Display: "properties", Fields: []ViewSectionField{
					{Property: "title"}, {Property: "status"},
				}},
				{Heading: "Components", Source: "components", Display: "list"},
			},
		},
	}

	// Add dashboard config
	cfg.Dashboard = &DashboardConfig{
		Title: "Dashboard",
		Cards: []DashboardCard{
			{Title: "All Tickets", Query: "type:ticket", Display: "count"},
		},
	}

	styleMap, styledTypes := buildStyleMap(cfg, meta)
	tmpl, err := template.New("").Funcs(templateFuncs(styleMap, styledTypes)).Parse(allTemplates)
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}

	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		tmpl:        tmpl,
		styleMap:    styleMap,
		styledTypes: styledTypes,
	}
}

func TestHandleIndex(t *testing.T) {
	t.Run("redirects to first list", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()
		app.handleIndex(w, r)
		// handleIndex rewrites the path and calls handleList internally
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Tickets") {
			t.Error("expected list page for tickets")
		}
	})

	t.Run("non-root path returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/notfound", http.NoBody)
		w := httptest.NewRecorder()
		app.handleIndex(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("no navigation returns error", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.Cfg.Navigation = nil
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()
		app.handleIndex(w, r)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("dashboard as first nav item", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.Cfg.Navigation = []NavigationEntry{
			{Label: "Dashboard", Dashboard: true},
			{Label: "Tickets", List: "tickets"},
		}
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()
		app.handleIndex(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleList(t *testing.T) {
	t.Run("renders list page", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/list/tickets", http.NoBody)
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "TKT-001") {
			t.Error("expected TKT-001 in list")
		}
		if !strings.Contains(body, "TKT-002") {
			t.Error("expected TKT-002 in list")
		}
	})

	t.Run("unknown list returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/list/nonexistent", http.NoBody)
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("sorting via query params", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/list/tickets?sort=title&sort_dir=desc", http.NoBody)
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.Cfg.Lists["tickets"] = List{
			EntityType: "ticket",
			Title:      "Tickets",
			Columns:    []ListColumn{{Property: "title", Label: "Title"}},
			PageSize:   1,
		}
		r := httptest.NewRequest(http.MethodGet, "/list/tickets?page=1", http.NoBody)
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HTMX request returns partial", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/list/tickets", http.NoBody)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleForm(t *testing.T) {
	t.Run("renders create form", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/form/create-ticket", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Title") {
			t.Error("expected form field 'Title'")
		}
	})

	t.Run("renders edit form with entity", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/form/edit-ticket/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "First Ticket") {
			t.Error("expected entity title in edit form")
		}
	})

	t.Run("unknown form returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/form/nonexistent", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("HTMX request returns partial", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/form/edit-ticket/TKT-001", http.NoBody)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleEntity(t *testing.T) {
	t.Run("renders entity detail", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleEntity(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "TKT-001") {
			t.Error("expected entity ID in detail page")
		}
	})

	t.Run("legacy URL redirects", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/entity/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleEntity(w, r)
		if w.Code != http.StatusMovedPermanently {
			t.Errorf("expected 301, got %d", w.Code)
		}
		loc := w.Header().Get("Location")
		if loc != "/entity/ticket/TKT-001" {
			t.Errorf("expected redirect to /entity/ticket/TKT-001, got %s", loc)
		}
	})

	t.Run("legacy URL with HTMX sets HX-Replace-Url", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/entity/TKT-001", http.NoBody)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleEntity(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if got := w.Header().Get("HX-Replace-Url"); got != "/entity/ticket/TKT-001" {
			t.Errorf("expected HX-Replace-Url /entity/ticket/TKT-001, got %s", got)
		}
	})

	t.Run("unknown entity returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/NONEXISTENT", http.NoBody)
		w := httptest.NewRecorder()
		app.handleEntity(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleView(t *testing.T) {
	t.Run("renders view page", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/view/ticket_detail/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleView(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "TKT-001") {
			t.Error("expected entity ID in view page")
		}
	})

	t.Run("unknown view returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/view/nonexistent/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleView(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("malformed path returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/view/ticket_detail", http.NoBody)
		w := httptest.NewRecorder()
		app.handleView(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("wrong entry type returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/view/ticket_detail/CMP-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleView(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("HTMX request returns partial", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/view/ticket_detail/TKT-001", http.NoBody)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleView(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleSearch(t *testing.T) {
	t.Run("renders search page without query", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/search", http.NoBody)
		w := httptest.NewRecorder()
		app.handleSearch(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("search with query", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/search?q=type:ticket", http.NoBody)
		w := httptest.NewRecorder()
		app.handleSearch(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "TKT-001") {
			t.Error("expected TKT-001 in search results")
		}
	})

	t.Run("free text search", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/search?q=First", http.NoBody)
		w := httptest.NewRecorder()
		app.handleSearch(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "TKT-001") {
			t.Error("expected TKT-001 in search results for 'First'")
		}
	})

	t.Run("HTMX search-results target", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/search?q=type:ticket", http.NoBody)
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Target", "search-results")
		w := httptest.NewRecorder()
		app.handleSearch(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleDashboard(t *testing.T) {
	t.Run("renders dashboard", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
		w := httptest.NewRecorder()
		app.handleDashboard(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "All Tickets") {
			t.Error("expected dashboard card title")
		}
	})

	t.Run("no dashboard config returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.Cfg.Dashboard = nil
		r := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
		w := httptest.NewRecorder()
		app.handleDashboard(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleCreate(t *testing.T) {
	t.Run("method not allowed for GET", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/create", http.NoBody)
		w := httptest.NewRecorder()
		app.handleCreate(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("unknown form returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{"_form_id": {"nonexistent"}}
		r := httptest.NewRequest(http.MethodPost, "/api/create", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleCreate(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleUpdate(t *testing.T) {
	t.Run("method not allowed for GET", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/update", http.NoBody)
		w := httptest.NewRecorder()
		app.handleUpdate(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{
			"_entity_id": {"NONEXISTENT"},
			"_form_id":   {"edit-ticket"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleUpdate(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("unknown form returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{
			"_entity_id": {"TKT-001"},
			"_form_id":   {"nonexistent"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleUpdate(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleDelete(t *testing.T) {
	t.Run("method not allowed for GET", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/delete", http.NoBody)
		w := httptest.NewRecorder()
		app.handleDelete(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{"_entity_id": {"NONEXISTENT"}}
		r := httptest.NewRequest(http.MethodPost, "/api/delete", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleDelete(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleInlineCreate(t *testing.T) {
	t.Run("method not allowed for GET", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/inline-create", http.NoBody)
		w := httptest.NewRecorder()
		app.handleInlineCreate(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("expected JSON content type, got %s", ct)
		}
	})

	t.Run("unknown form returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{"_form_id": {"nonexistent"}}
		r := httptest.NewRequest(http.MethodPost, "/api/inline-create", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleInlineCreate(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleInlineForm(t *testing.T) {
	t.Run("renders inline form HTML", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/inline-form/create-ticket", http.NoBody)
		w := httptest.NewRecorder()
		app.handleInlineForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "form-group") {
			t.Error("expected form-group in inline form HTML")
		}
	})

	t.Run("unknown form returns 404", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/inline-form/nonexistent", http.NoBody)
		w := httptest.NewRecorder()
		app.handleInlineForm(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleExecuteQuery(t *testing.T) {
	app := newHandlerTestApp(t)

	t.Run("type query returns matching entities", func(t *testing.T) {
		results := app.executeQuery("type:ticket")
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("free text query", func(t *testing.T) {
		results := app.executeQuery("First")
		if len(results) != 1 || results[0].ID != "TKT-001" {
			t.Errorf("expected [TKT-001], got %v", collectIDs(results))
		}
	})

	t.Run("empty query returns nil", func(t *testing.T) {
		results := app.executeQuery("")
		if results != nil {
			t.Errorf("expected nil for empty query, got %d results", len(results))
		}
	})

	t.Run("property filter query", func(t *testing.T) {
		results := app.executeQuery("type:ticket status:open")
		if len(results) != 1 || results[0].ID != "TKT-001" {
			t.Errorf("expected [TKT-001], got %v", collectIDs(results))
		}
	})
}
