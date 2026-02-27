package dataentry

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
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
				{From: StringOrSlice{"entry"}, Follow: "belongs_to", CollectAs: StringOrSlice{"components"}},
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
	tmpl, err := template.New("").Funcs(templateFuncs(styleMap, styledTypes)).Parse(allTemplates())
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}

	// Set up a repo for tests that need to read/write cache files
	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	repo := repository.New(fs, ctx)

	ws := workspace.NewWithGraph(repo, meta, g)

	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		tmpl:        tmpl,
		styleMap:    styleMap,
		styledTypes: styledTypes,
		ws:          ws,
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

	t.Run("multi-select column renders list values", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Add a multi-select type and property
		app.meta.Types["applies_to_type"] = metamodel.CustomType{
			Values: []string{"client", "provider", "employee"},
		}
		app.meta.Entities["ticket"].Properties["applies_to"] = metamodel.PropertyDef{
			Type: "applies_to_type",
		}
		// Add a list with multi-select column
		app.Cfg.Lists["tickets"] = List{
			EntityType: "ticket",
			Title:      "Tickets",
			Columns: []ListColumn{
				{Property: "title", Label: "Title", Link: true},
				{Property: "applies_to", Label: "Applies To"},
			},
		}
		// Add entity with multi-select values as []string
		e := model.NewEntity("TKT-003", "ticket")
		e.SetString("title", "Multi-select Test")
		e.Properties["applies_to"] = []string{"client", "provider"}
		app.g.AddNode(e)

		r := httptest.NewRequest(http.MethodGet, "/list/tickets", http.NoBody)
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// Should contain both values (rendered as badges or comma-separated)
		if !strings.Contains(body, "client") {
			t.Error("expected 'client' in multi-select column")
		}
		if !strings.Contains(body, "provider") {
			t.Error("expected 'provider' in multi-select column")
		}
	})

	t.Run("multi-select column renders []interface{} from YAML", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.meta.Types["tag_type"] = metamodel.CustomType{
			Values: []string{"bug", "feature", "docs"},
		}
		app.meta.Entities["ticket"].Properties["tags"] = metamodel.PropertyDef{
			Type: "tag_type",
		}
		app.Cfg.Lists["tickets"] = List{
			EntityType: "ticket",
			Title:      "Tickets",
			Columns: []ListColumn{
				{Property: "title", Label: "Title"},
				{Property: "tags", Label: "Tags"},
			},
		}
		// Simulate YAML-parsed values ([]interface{})
		e := model.NewEntity("TKT-004", "ticket")
		e.SetString("title", "YAML Test")
		e.Properties["tags"] = []interface{}{"bug", "feature"}
		app.g.AddNode(e)

		r := httptest.NewRequest(http.MethodGet, "/list/tickets", http.NoBody)
		w := httptest.NewRecorder()
		app.handleList(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "bug") {
			t.Error("expected 'bug' in multi-select column")
		}
		if !strings.Contains(body, "feature") {
			t.Error("expected 'feature' in multi-select column")
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

	t.Run("edit form shows incoming relations as selected", func(t *testing.T) {
		app := newHandlerTestApp(t)

		// Add an incoming relation: TKT-001 --belongs_to--> CMP-001
		// This means CMP-001 has an incoming "belongs_to" edge from TKT-001.
		app.g.AddEdge(model.NewRelation("TKT-001", "belongs_to", "CMP-001"))

		// Add a form for component with an incoming relation
		app.Cfg.Forms["edit-component-incoming"] = Form{
			EntityType: "component",
			Mode:       "edit",
			Fields:     []FormField{{Property: "name"}},
			Relations: []FormRelation{{
				Relation:   "belongs_to",
				Direction:  DirectionIncoming,
				TargetType: "ticket",
				Label:      "Tickets",
			}},
		}

		r := httptest.NewRequest(http.MethodGet, "/form/edit-component-incoming/CMP-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// TKT-001 should be selected (it's the source of the incoming belongs_to edge)
		if !strings.Contains(body, `value="TKT-001" selected`) {
			t.Error("expected TKT-001 to be pre-selected as incoming relation")
		}
	})

	t.Run("edit form shows outgoing relations as selected", func(t *testing.T) {
		app := newHandlerTestApp(t)

		// TKT-001 --depends_on--> TKT-002 already added in newHandlerTestApp

		// Add a form for ticket with an outgoing relation
		app.Cfg.Forms["edit-ticket-outgoing"] = Form{
			EntityType: "ticket",
			Mode:       "edit",
			Fields:     []FormField{{Property: "title"}},
			Relations: []FormRelation{{
				Relation:   "depends_on",
				Direction:  DirectionOutgoing,
				TargetType: "ticket",
				Label:      "Dependencies",
			}},
		}

		r := httptest.NewRequest(http.MethodGet, "/form/edit-ticket-outgoing/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// TKT-002 should be selected (it's the target of the outgoing depends_on edge)
		if !strings.Contains(body, `value="TKT-002" selected`) {
			t.Error("expected TKT-002 to be pre-selected as outgoing relation")
		}
	})

	t.Run("edit form pre-selects multi-select property values", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Add a multi-select property type
		app.meta.Types["role_type"] = metamodel.CustomType{
			Values: []string{"admin", "editor", "viewer"},
		}
		app.meta.Entities["ticket"].Properties["roles"] = metamodel.PropertyDef{
			Type: "role_type",
			List: true,
		}
		// Add form with multi-select field
		app.Cfg.Forms["edit-ticket-roles"] = Form{
			EntityType: "ticket",
			Mode:       "edit",
			Fields: []FormField{
				{Property: "title"},
				{Property: "roles"},
			},
		}
		// Add entity with multi-select values
		e := model.NewEntity("TKT-ROLES", "ticket")
		e.SetString("title", "Role Test")
		e.Properties["roles"] = []string{"admin", "viewer"}
		app.g.AddNode(e)

		r := httptest.NewRequest(http.MethodGet, "/form/edit-ticket-roles/TKT-ROLES", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// Both selected values should have "selected" attribute
		if !strings.Contains(body, `value="admin"`) || !strings.Contains(body, "selected") {
			t.Error("expected 'admin' option to be selected")
		}
		if !strings.Contains(body, `value="viewer"`) {
			t.Error("expected 'viewer' option in form")
		}
		// editor should NOT be selected
		if strings.Contains(body, `value="editor" selected`) {
			t.Error("did not expect 'editor' to be selected")
		}
	})

	t.Run("prefills relation from link params via inverse", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Add inverse to depends_on so we can test inverse matching.
		rels := app.meta.Relations
		dep := rels["depends_on"]
		dep.Inverse = &metamodel.InverseDef{ID: "dependency_of"}
		rels["depends_on"] = dep

		// Add a form with a relation using the inverse name.
		app.Cfg.Forms["create-dep-ticket"] = Form{
			EntityType: "ticket",
			Fields:     []FormField{{Property: "title"}},
			Relations: []FormRelation{{
				Relation:   "dependency_of",
				TargetType: "ticket",
				Label:      "Dependency Of",
			}},
		}

		r := httptest.NewRequest(http.MethodGet,
			"/form/create-dep-ticket?link_relation=depends_on&link_peer=TKT-001&link_as=to",
			http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// The TKT-001 option should be marked as selected.
		if !strings.Contains(body, `value="TKT-001" selected`) {
			t.Error("expected TKT-001 to be pre-selected in the dependency_of relation")
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

	t.Run("link existing button rendered for traversal section", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/view/ticket_detail/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleView(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Link existing") {
			t.Error("expected 'Link existing' button in view page for traversal section")
		}
		if !strings.Contains(body, "openLinkExisting") {
			t.Error("expected openLinkExisting JS call in view page")
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

func TestHandleToggleGroup(t *testing.T) {
	t.Run("toggles group collapsed state", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Set up a workspace with cache dir for UI state persistence
		fs := storage.NewMemFS()
		ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
		_ = fs.MkdirAll(ctx.CacheDir, 0o755)
		app.ws = workspace.NewWithGraph(repository.New(fs, ctx), app.meta, app.g)

		// Toggle "Tickets" group to collapsed
		body := strings.NewReader("group=Tickets")
		r := httptest.NewRequest(http.MethodPost, "/api/ui/toggle-group", body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleToggleGroup(w, r)
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}

		state := app.loadUIState()
		if !state.CollapsedGroups["Tickets"] {
			t.Error("expected Tickets to be collapsed")
		}

		// Toggle again to expand
		body = strings.NewReader("group=Tickets")
		r = httptest.NewRequest(http.MethodPost, "/api/ui/toggle-group", body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w = httptest.NewRecorder()
		app.handleToggleGroup(w, r)
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}

		state = app.loadUIState()
		if state.CollapsedGroups["Tickets"] {
			t.Error("expected Tickets to be expanded after second toggle")
		}
	})

	t.Run("missing group returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		body := strings.NewReader("")
		r := httptest.NewRequest(http.MethodPost, "/api/ui/toggle-group", body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleToggleGroup(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("GET not allowed", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/ui/toggle-group", http.NoBody)
		w := httptest.NewRecorder()
		app.handleToggleGroup(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleSettings(t *testing.T) {
	t.Run("renders settings page", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = nil
		r := httptest.NewRequest(http.MethodGet, "/settings", http.NoBody)
		w := httptest.NewRecorder()
		app.handleSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Settings") {
			t.Error("expected 'Settings' in page")
		}
	})

	t.Run("renders settings with existing defaults", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = &UserDefaults{
			Defaults: map[string]string{"priority": "high"},
		}
		r := httptest.NewRequest(http.MethodGet, "/settings", http.NoBody)
		w := httptest.NewRecorder()
		app.handleSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HTMX request returns partial", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = nil
		r := httptest.NewRequest(http.MethodGet, "/settings", http.NoBody)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleSaveSettings(t *testing.T) {
	t.Run("saves global property defaults", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = nil

		form := url.Values{
			"default_prop[priority]": {"high"},
			"default_prop[status]":   {"open"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if app.userDefaults == nil {
			t.Fatal("expected userDefaults to be set")
		}
		if app.userDefaults.Defaults["priority"] != "high" {
			t.Errorf("expected priority=high, got %q", app.userDefaults.Defaults["priority"])
		}
		if app.userDefaults.Defaults["status"] != "open" {
			t.Errorf("expected status=open, got %q", app.userDefaults.Defaults["status"])
		}
	})

	t.Run("saves global relation defaults", func(t *testing.T) {
		app := newHandlerTestApp(t)

		form := url.Values{
			"default_rel[belongs_to]": {"CMP-001"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if app.userDefaults.RelationDefaults["belongs_to"] != "CMP-001" {
			t.Errorf("expected belongs_to=CMP-001, got %q", app.userDefaults.RelationDefaults["belongs_to"])
		}
	})

	t.Run("saves override groups", func(t *testing.T) {
		app := newHandlerTestApp(t)

		form := url.Values{
			"override[0][types]":           {"ticket"},
			"override[0][prop][priority]":  {"high"},
			"override[0][rel][depends_on]": {"TKT-002"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if len(app.userDefaults.Overrides) != 1 {
			t.Fatalf("expected 1 override, got %d", len(app.userDefaults.Overrides))
		}
		o := app.userDefaults.Overrides[0]
		if len(o.Types) != 1 || o.Types[0] != "ticket" {
			t.Errorf("expected types=[ticket], got %v", o.Types)
		}
		if o.Defaults["priority"] != "high" {
			t.Errorf("expected priority=high, got %q", o.Defaults["priority"])
		}
		if o.RelationDefaults["depends_on"] != "TKT-002" {
			t.Errorf("expected depends_on=TKT-002, got %q", o.RelationDefaults["depends_on"])
		}
	})

	t.Run("skips overrides without types", func(t *testing.T) {
		app := newHandlerTestApp(t)

		form := url.Values{
			"override[0][prop][priority]": {"high"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if len(app.userDefaults.Overrides) != 0 {
			t.Errorf("expected 0 overrides (no types), got %d", len(app.userDefaults.Overrides))
		}
	})

	t.Run("empty values are not saved", func(t *testing.T) {
		app := newHandlerTestApp(t)

		form := url.Values{
			"default_prop[priority]":  {""},
			"default_rel[belongs_to]": {""},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if len(app.userDefaults.Defaults) != 0 {
			t.Errorf("expected 0 defaults, got %d", len(app.userDefaults.Defaults))
		}
		if len(app.userDefaults.RelationDefaults) != 0 {
			t.Errorf("expected 0 relation defaults, got %d", len(app.userDefaults.RelationDefaults))
		}
	})

	t.Run("GET not allowed", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/settings", http.NoBody)
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("persists to file and reloads", func(t *testing.T) {
		app := newHandlerTestApp(t)

		form := url.Values{
			"default_prop[priority]": {"medium"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleSaveSettings(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Reload from file
		loaded := app.loadUserDefaults()
		if loaded == nil {
			t.Fatal("expected non-nil loaded defaults")
		}
		if loaded.Defaults["priority"] != "medium" {
			t.Errorf("expected priority=medium, got %q", loaded.Defaults["priority"])
		}
	})
}

func TestHandleFormWithUserDefaults(t *testing.T) {
	t.Run("create form uses user defaults for property", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = &UserDefaults{
			Defaults: map[string]string{"status": "in_progress"},
		}
		// Use the edit-ticket form which has the status field, for creating
		app.Cfg.Forms["create-ticket-status"] = Form{
			EntityType: "ticket",
			Mode:       "create",
			Fields: []FormField{
				{Property: "title"},
				{Property: "status"},
			},
		}
		r := httptest.NewRequest(http.MethodGet, "/form/create-ticket-status", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// The status field should be pre-selected to "in_progress"
		if !strings.Contains(body, `value="in_progress" selected`) {
			t.Error("expected user default 'in_progress' to be selected in form")
		}
	})

	t.Run("create form uses user defaults for relation", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = &UserDefaults{
			RelationDefaults: map[string]string{"belongs_to": "CMP-001"},
		}
		// Need a form with a relation field
		app.Cfg.Forms["create-ticket-rel"] = Form{
			EntityType: "ticket",
			Mode:       "create",
			Fields:     []FormField{{Property: "title"}},
			Relations: []FormRelation{{
				Relation:   "belongs_to",
				TargetType: "component",
				Label:      "Component",
			}},
		}
		r := httptest.NewRequest(http.MethodGet, "/form/create-ticket-rel", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// CMP-001 should be pre-selected as the default relation target
		if !strings.Contains(body, `value="CMP-001" selected`) {
			t.Error("expected CMP-001 to be pre-selected as user default relation")
		}
	})

	t.Run("edit form does not use user defaults", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = &UserDefaults{
			Defaults: map[string]string{"status": "in_progress"},
		}
		// TKT-001 has status=open, user default is in_progress.
		// Edit should show actual value, not user default.
		r := httptest.NewRequest(http.MethodGet, "/form/edit-ticket/TKT-001", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// The actual value "open" should be in the form, not the user default
		if !strings.Contains(body, "open") {
			t.Error("expected actual value 'open' in edit form")
		}
	})

	t.Run("override takes precedence in create form", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.userDefaults = &UserDefaults{
			Defaults: map[string]string{"priority": "low"},
			Overrides: []DefaultOverride{
				{
					Types:    []string{"ticket"},
					Defaults: map[string]string{"priority": "high"},
				},
			},
		}
		// Add priority to create-ticket form fields
		app.Cfg.Forms["create-ticket"] = Form{
			EntityType: "ticket",
			Mode:       "create",
			Fields: []FormField{
				{Property: "title"},
				{Property: "priority"},
			},
		}
		r := httptest.NewRequest(http.MethodGet, "/form/create-ticket", http.NoBody)
		w := httptest.NewRecorder()
		app.handleForm(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		// Should use override "high", not global "low"
		if !strings.Contains(body, `value="high" selected`) {
			t.Error("expected override 'high' to be selected for ticket priority")
		}
	})
}

func TestHandleIndexWithGroupedNav(t *testing.T) {
	t.Run("first item inside group", func(t *testing.T) {
		app := newHandlerTestApp(t)
		app.Cfg.Navigation = []NavigationEntry{
			{
				Group: "Tickets",
				Items: []NavigationEntry{
					{Label: "All Tickets", List: "tickets"},
				},
			},
		}
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()
		app.handleIndex(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "TKT-001") {
			t.Error("expected list page content")
		}
	})
}

func TestHandleLinkCandidates(t *testing.T) {
	t.Run("missing params returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/link-candidates", http.NoBody)
		w := httptest.NewRecorder()
		app.handleLinkCandidates(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("returns candidates excluding already linked", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// TKT-001 depends_on TKT-002 (added in newHandlerTestApp)
		r := httptest.NewRequest(http.MethodGet,
			"/api/link-candidates?relation=depends_on&link_as=to&peer=TKT-001&entity_types=ticket",
			http.NoBody)
		w := httptest.NewRecorder()
		app.handleLinkCandidates(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var candidates []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Type  string `json:"type"`
		}
		if err := json.NewDecoder(w.Body).Decode(&candidates); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		// TKT-002 is already linked, TKT-001 is self — expect empty
		for _, c := range candidates {
			if c.ID == "TKT-002" {
				t.Error("TKT-002 should be excluded (already linked)")
			}
			if c.ID == "TKT-001" {
				t.Error("TKT-001 should be excluded (self)")
			}
		}
	})

	t.Run("filters by search query", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Add a third ticket
		e3 := model.NewEntity("TKT-003", "ticket")
		e3.SetString("title", "Third Ticket")
		app.g.AddNode(e3)

		r := httptest.NewRequest(http.MethodGet,
			"/api/link-candidates?relation=depends_on&link_as=to&peer=TKT-001&entity_types=ticket&q=Third",
			http.NoBody)
		w := httptest.NewRecorder()
		app.handleLinkCandidates(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var candidates []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(w.Body).Decode(&candidates); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}
		if candidates[0].ID != "TKT-003" {
			t.Errorf("expected TKT-003, got %s", candidates[0].ID)
		}
	})

	t.Run("returns empty array when no candidates", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet,
			"/api/link-candidates?relation=depends_on&link_as=to&peer=TKT-001&entity_types=ticket&q=nonexistent",
			http.NoBody)
		w := httptest.NewRecorder()
		app.handleLinkCandidates(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if strings.TrimSpace(body) != "[]" {
			t.Errorf("expected empty JSON array, got %s", body)
		}
	})
}

func TestHandleLinkExisting(t *testing.T) {
	t.Run("method not allowed for GET", func(t *testing.T) {
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/link-existing", http.NoBody)
		w := httptest.NewRecorder()
		app.handleLinkExisting(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("missing params returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{"relation": {"depends_on"}}
		r := httptest.NewRequest(http.MethodPost, "/api/link-existing", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleLinkExisting(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("unknown relation returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{
			"relation": {"nonexistent"},
			"link_as":  {"to"},
			"peer":     {"TKT-001"},
			"target":   {"TKT-002"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/link-existing", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleLinkExisting(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("unknown peer returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{
			"relation": {"depends_on"},
			"link_as":  {"to"},
			"peer":     {"NONEXISTENT"},
			"target":   {"TKT-002"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/link-existing", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleLinkExisting(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("unknown target returns 400", func(t *testing.T) {
		app := newHandlerTestApp(t)
		form := url.Values{
			"relation": {"depends_on"},
			"link_as":  {"to"},
			"peer":     {"TKT-001"},
			"target":   {"NONEXISTENT"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/link-existing", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		app.handleLinkExisting(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func newTestRepository(t *testing.T, tmpDir string) *repository.Repository {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	ctx := &project.Context{Root: tmpDir}
	return repository.New(fs, ctx)
}

func TestValidationErrorsToFieldMap(t *testing.T) {
	t.Run("converts required property error", func(t *testing.T) {
		errs := []*metamodel.ValidationError{
			{Type: metamodel.ValidationErrorRequired, Property: "title", Message: "This field is required"},
		}
		result := validationErrorsToFieldMap(errs)
		if result["title"] != "This field is required" {
			t.Errorf("expected 'This field is required', got %q", result["title"])
		}
	})

	t.Run("converts invalid value error", func(t *testing.T) {
		errs := []*metamodel.ValidationError{
			{Type: metamodel.ValidationErrorInvalidValue, Property: "status", Message: "Invalid value"},
		}
		result := validationErrorsToFieldMap(errs)
		if result["status"] != "Invalid value" {
			t.Errorf("expected 'Invalid value', got %q", result["status"])
		}
	})

	t.Run("handles multiple errors", func(t *testing.T) {
		errs := []*metamodel.ValidationError{
			{Type: metamodel.ValidationErrorRequired, Property: "title", Message: "This field is required"},
			{Type: metamodel.ValidationErrorRequired, Property: "status", Message: "This field is required"},
		}
		result := validationErrorsToFieldMap(errs)
		if len(result) != 2 {
			t.Errorf("expected 2 errors, got %d", len(result))
		}
	})

	t.Run("skips entity-level errors without property", func(t *testing.T) {
		errs := []*metamodel.ValidationError{
			{Type: metamodel.ValidationErrorIDPrefix, Property: "", Message: "ID prefix mismatch"},
		}
		result := validationErrorsToFieldMap(errs)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("handles empty errors", func(t *testing.T) {
		result := validationErrorsToFieldMap(nil)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}

func TestHandleCreateWithValidationErrors(t *testing.T) {
	t.Run("returns 422 with validation errors for missing required field", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Configure temp directory for repository to avoid writing to real filesystem
		tmpDir := t.TempDir()
		repo := newTestRepository(t, tmpDir)

		// Make title required in the metamodel
		entDef := app.meta.Entities["ticket"]
		titleProp := entDef.Properties["title"]
		titleProp.Required = true
		entDef.Properties["title"] = titleProp
		app.meta.Entities["ticket"] = entDef

		// Rebuild workspace with updated repo and meta
		app.ws = workspace.NewWithGraph(repo, app.meta, app.g)

		// Submit form without title (required field)
		form := url.Values{
			"_form_id":   {"create-ticket"},
			"_entity_id": {"TKT-NEW"},
			"status":     {"open"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/create", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleCreate(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422, got %d", w.Code)
		}

		// Verify HX-Retarget and HX-Reswap headers for HTMX swap override
		if got := w.Header().Get("HX-Retarget"); got != "#content" {
			t.Errorf("expected HX-Retarget=#content, got %q", got)
		}
		if got := w.Header().Get("HX-Reswap"); got != "innerHTML" {
			t.Errorf("expected HX-Reswap=innerHTML, got %q", got)
		}

		body := w.Body.String()
		if !strings.Contains(body, "field-error") {
			t.Error("expected field-error class in response")
		}
		if !strings.Contains(body, "This field is required") {
			t.Error("expected validation error message in response")
		}
	})
}

func TestHandleUpdateWithValidationErrors(t *testing.T) {
	t.Run("returns 422 with validation errors for invalid field", func(t *testing.T) {
		app := newHandlerTestApp(t)
		// Configure temp directory for repository to avoid writing to real filesystem
		tmpDir := t.TempDir()
		repo := newTestRepository(t, tmpDir)

		// Make title required in the metamodel
		entDef := app.meta.Entities["ticket"]
		titleProp := entDef.Properties["title"]
		titleProp.Required = true
		entDef.Properties["title"] = titleProp
		app.meta.Entities["ticket"] = entDef

		// Rebuild workspace with updated repo and meta
		app.ws = workspace.NewWithGraph(repo, app.meta, app.g)

		// Submit form with empty title (required field)
		form := url.Values{
			"_form_id":   {"edit-ticket"},
			"_entity_id": {"TKT-001"},
			"title":      {""}, // Empty required field
			"status":     {"open"},
		}
		r := httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		app.handleUpdate(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422, got %d", w.Code)
		}

		// Verify HX-Retarget and HX-Reswap headers
		if got := w.Header().Get("HX-Retarget"); got != "#content" {
			t.Errorf("expected HX-Retarget=#content, got %q", got)
		}
		if got := w.Header().Get("HX-Reswap"); got != "innerHTML" {
			t.Errorf("expected HX-Reswap=innerHTML, got %q", got)
		}

		body := w.Body.String()
		if !strings.Contains(body, "field-error") {
			t.Error("expected field-error class in response")
		}
	})
}
