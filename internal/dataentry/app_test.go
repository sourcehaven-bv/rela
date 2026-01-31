package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// testMeta returns a metamodel suitable for testing app-level functions.
func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status_type":   {Values: []string{"open", "in_progress", "closed"}},
			"priority_type": {Values: []string{"low", "medium", "high"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "status_type"},
					"priority": {Type: "priority_type"},
				},
			},
			"component": {
				Label: "Component",
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"depends_on": {
				Label: "depends on",
				From:  []string{"ticket"},
				To:    []string{"ticket"},
			},
			"belongs_to": {
				Label: "belongs to",
				From:  []string{"ticket"},
				To:    []string{"component"},
			},
		},
	}
}

// testConfig returns a config suitable for testing app-level functions.
func testConfig() *Config {
	return &Config{
		App: AppConfig{Name: "Test App"},
		Navigation: []NavigationEntry{
			{Label: "Tickets", List: "tickets"},
			{Label: "Components", List: "components"},
		},
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Title:      "Tickets",
				Columns: []ListColumn{
					{Property: "title", Label: "Title", Link: true},
					{Property: "status", Label: "Status"},
				},
			},
			"components": {
				EntityType: "component",
				Title:      "Components",
				Columns: []ListColumn{
					{Property: "name", Label: "Name"},
				},
			},
		},
		Forms: map[string]Form{
			"edit-ticket": {
				EntityType: "ticket",
				Mode:       "edit",
				Fields: []FormField{
					{Property: "title"},
					{Property: "status"},
				},
			},
			"create-ticket": {
				EntityType: "ticket",
				Mode:       "create",
				Fields: []FormField{
					{Property: "title"},
				},
			},
			"edit-component": {
				EntityType: "component",
				Mode:       "edit",
				Fields: []FormField{
					{Property: "name"},
				},
			},
		},
		Styles: map[string]map[string]string{
			"status_type": {
				"open":   "green",
				"closed": "red",
			},
		},
	}
}

// testGraph returns a graph with some test entities.
func testGraph() *graph.Graph {
	g := graph.New()
	e1 := model.NewEntity("TKT-001", "ticket")
	e1.SetString("title", "First Ticket")
	e1.SetString("status", "open")
	e1.SetString("priority", "high")
	g.AddNode(e1)

	e2 := model.NewEntity("TKT-002", "ticket")
	e2.SetString("title", "Second Ticket")
	e2.SetString("status", "closed")
	e2.SetString("priority", "low")
	g.AddNode(e2)

	c1 := model.NewEntity("CMP-001", "component")
	c1.SetString("name", "Frontend")
	g.AddNode(c1)

	return g
}

// testAppInstance creates a minimal App without templates for testing app-level methods.
func testAppInstance() *App {
	cfg := testConfig()
	meta := testMeta()
	g := testGraph()
	styleMap, styledTypes := buildStyleMap(cfg, meta)
	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		styleMap:    styleMap,
		styledTypes: styledTypes,
	}
}

func TestValidateConfig(t *testing.T) {
	meta := testMeta()

	t.Run("valid config", func(t *testing.T) {
		cfg := testConfig()
		errs := validateConfig(cfg, meta)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("form with unknown entity type", func(t *testing.T) {
		cfg := &Config{
			Forms: map[string]Form{
				"bad-form": {EntityType: "nonexistent", Fields: []FormField{{Property: "title"}}},
			},
		}
		errs := validateConfig(cfg, meta)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
		if got := errs[0]; got != `form "bad-form": unknown entity type "nonexistent"` {
			t.Errorf("unexpected error: %s", got)
		}
	})

	t.Run("form with unknown field", func(t *testing.T) {
		cfg := &Config{
			Forms: map[string]Form{
				"bad-form": {EntityType: "ticket", Fields: []FormField{{Property: "nonexistent"}}},
			},
		}
		errs := validateConfig(cfg, meta)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("form with unknown relation", func(t *testing.T) {
		cfg := &Config{
			Forms: map[string]Form{
				"bad-form": {EntityType: "ticket", Relations: []FormRelation{{Relation: "nonexistent"}}},
			},
		}
		errs := validateConfig(cfg, meta)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("list with unknown entity type", func(t *testing.T) {
		cfg := &Config{
			Lists: map[string]List{
				"bad-list": {EntityType: "nonexistent"},
			},
		}
		errs := validateConfig(cfg, meta)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("list with unknown column", func(t *testing.T) {
		cfg := &Config{
			Lists: map[string]List{
				"bad-list": {EntityType: "ticket", Columns: []ListColumn{{Property: "nonexistent"}}},
			},
		}
		errs := validateConfig(cfg, meta)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("empty config is valid", func(t *testing.T) {
		cfg := &Config{}
		errs := validateConfig(cfg, meta)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})
}

func TestBuildStyleMap(t *testing.T) {
	t.Run("explicit styles mapped", func(t *testing.T) {
		cfg := &Config{
			Styles: map[string]map[string]string{
				"status_type": {"open": "green", "closed": "red"},
			},
		}
		meta := &metamodel.Metamodel{
			Types: map[string]metamodel.CustomType{},
		}
		sm, st := buildStyleMap(cfg, meta)
		if !st["status_type"] {
			t.Error("expected status_type to be styled")
		}
		if sm["status_type"]["open"] != "badge-green" {
			t.Errorf("expected badge-green, got %s", sm["status_type"]["open"])
		}
		if sm["status_type"]["closed"] != "badge-red" {
			t.Errorf("expected badge-red, got %s", sm["status_type"]["closed"])
		}
	})

	t.Run("unknown color falls back to badge-gray", func(t *testing.T) {
		cfg := &Config{
			Styles: map[string]map[string]string{
				"status_type": {"open": "chartreuse"},
			},
		}
		meta := &metamodel.Metamodel{
			Types: map[string]metamodel.CustomType{},
		}
		sm, _ := buildStyleMap(cfg, meta)
		if sm["status_type"]["open"] != "badge-gray" {
			t.Errorf("expected badge-gray for unknown color, got %s", sm["status_type"]["open"])
		}
	})

	t.Run("auto-assigned for custom types", func(t *testing.T) {
		cfg := &Config{}
		meta := &metamodel.Metamodel{
			Types: map[string]metamodel.CustomType{
				"priority_type": {Values: []string{"low", "medium", "high"}},
			},
		}
		sm, st := buildStyleMap(cfg, meta)
		if !st["priority_type"] {
			t.Error("expected priority_type to be styled")
		}
		if sm["priority_type"]["low"] == "" {
			t.Error("expected auto-assigned class for 'low'")
		}
	})

	t.Run("explicitly styled types not overridden", func(t *testing.T) {
		cfg := &Config{
			Styles: map[string]map[string]string{
				"priority_type": {"low": "blue"},
			},
		}
		meta := &metamodel.Metamodel{
			Types: map[string]metamodel.CustomType{
				"priority_type": {Values: []string{"low", "medium", "high"}},
			},
		}
		sm, _ := buildStyleMap(cfg, meta)
		if sm["priority_type"]["low"] != "badge-blue" {
			t.Errorf("expected explicit badge-blue, got %s", sm["priority_type"]["low"])
		}
		// "medium" should not be auto-assigned since the type was explicitly styled
		if _, ok := sm["priority_type"]["medium"]; ok {
			t.Error("did not expect auto-assigned style for 'medium' in explicitly styled type")
		}
	})
}

func TestEditFormForType(t *testing.T) {
	app := testAppInstance()

	t.Run("returns edit form", func(t *testing.T) {
		got := app.editFormForType("ticket")
		if got != "edit-ticket" {
			t.Errorf("expected edit-ticket, got %s", got)
		}
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		got := app.editFormForType("nonexistent")
		if got != "" {
			t.Errorf("expected empty, got %s", got)
		}
	})

	t.Run("form with empty mode treated as edit", func(t *testing.T) {
		app2 := testAppInstance()
		app2.Cfg.Forms = map[string]Form{
			"default-form": {EntityType: "ticket", Mode: ""},
		}
		got := app2.editFormForType("ticket")
		if got != "default-form" {
			t.Errorf("expected default-form, got %s", got)
		}
	})
}

func TestCreateFormForType(t *testing.T) {
	app := testAppInstance()

	t.Run("returns create form", func(t *testing.T) {
		got := app.createFormForType("ticket")
		if got != "create-ticket" {
			t.Errorf("expected create-ticket, got %s", got)
		}
	})

	t.Run("falls back to edit form", func(t *testing.T) {
		app2 := testAppInstance()
		app2.Cfg.Forms = map[string]Form{
			"edit-ticket": {EntityType: "ticket", Mode: "edit"},
		}
		got := app2.createFormForType("ticket")
		if got != "edit-ticket" {
			t.Errorf("expected edit-ticket as fallback, got %s", got)
		}
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		got := app.createFormForType("nonexistent")
		if got != "" {
			t.Errorf("expected empty, got %s", got)
		}
	})
}

func TestEntityDisplayTitle(t *testing.T) {
	app := testAppInstance()

	t.Run("returns primary property value", func(t *testing.T) {
		e, _ := app.g.GetNode("TKT-001")
		got := app.entityDisplayTitle(e)
		if got != "First Ticket" {
			t.Errorf("expected 'First Ticket', got %q", got)
		}
	})

	t.Run("falls back to ID for unknown type", func(t *testing.T) {
		e := &model.Entity{ID: "UNK-001", Type: "unknown", Properties: map[string]interface{}{}}
		got := app.entityDisplayTitle(e)
		if got != "UNK-001" {
			t.Errorf("expected 'UNK-001', got %q", got)
		}
	})

	t.Run("falls back to ID when primary property is empty", func(t *testing.T) {
		e := &model.Entity{ID: "TKT-099", Type: "ticket", Properties: map[string]interface{}{"title": ""}}
		got := app.entityDisplayTitle(e)
		if got != "TKT-099" {
			t.Errorf("expected 'TKT-099', got %q", got)
		}
	})
}

func TestActiveListForEntityType(t *testing.T) {
	app := testAppInstance()

	t.Run("returns matching list", func(t *testing.T) {
		got := app.activeListForEntityType("ticket")
		if got != "tickets" {
			t.Errorf("expected 'tickets', got %q", got)
		}
	})

	t.Run("returns empty for no match", func(t *testing.T) {
		got := app.activeListForEntityType("nonexistent")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestActiveListFromReferer(t *testing.T) {
	app := testAppInstance()

	t.Run("extracts list from referer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001", http.NoBody)
		r.Header.Set("Referer", "http://localhost:8080/list/tickets")
		got := app.activeListFromReferer(r)
		if got != "tickets" {
			t.Errorf("expected 'tickets', got %q", got)
		}
	})

	t.Run("returns empty for missing referer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001", http.NoBody)
		got := app.activeListFromReferer(r)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for non-list referer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001", http.NoBody)
		r.Header.Set("Referer", "http://localhost:8080/search")
		got := app.activeListFromReferer(r)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for unknown list", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001", http.NoBody)
		r.Header.Set("Referer", "http://localhost:8080/list/nonexistent")
		got := app.activeListFromReferer(r)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestResolveActiveList(t *testing.T) {
	app := testAppInstance()

	t.Run("from query param takes precedence", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001?from=components", http.NoBody)
		got := app.resolveActiveList("ticket", r)
		if got != "components" {
			t.Errorf("expected 'components', got %q", got)
		}
	})

	t.Run("from param must be known list", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001?from=nonexistent", http.NoBody)
		got := app.resolveActiveList("ticket", r)
		// Should fall through to entity type matching
		if got != "tickets" {
			t.Errorf("expected 'tickets', got %q", got)
		}
	})

	t.Run("falls back to entity type match", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/ticket/TKT-001", http.NoBody)
		got := app.resolveActiveList("ticket", r)
		if got != "tickets" {
			t.Errorf("expected 'tickets', got %q", got)
		}
	})

	t.Run("falls back to referer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/entity/unknown/UNK-001", http.NoBody)
		r.Header.Set("Referer", "http://localhost:8080/list/components")
		got := app.resolveActiveList("unknown", r)
		if got != "components" {
			t.Errorf("expected 'components', got %q", got)
		}
	})
}

func TestNavItems(t *testing.T) {
	app := testAppInstance()

	t.Run("returns items with counts", func(t *testing.T) {
		items := app.navItems()
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].Label != "Tickets" {
			t.Errorf("expected label 'Tickets', got %q", items[0].Label)
		}
		if items[0].EntityType != "ticket" {
			t.Errorf("expected entity type 'ticket', got %q", items[0].EntityType)
		}
		if items[0].Count != 2 {
			t.Errorf("expected count 2, got %d", items[0].Count)
		}
		if items[1].Label != "Components" {
			t.Errorf("expected label 'Components', got %q", items[1].Label)
		}
		if items[1].Count != 1 {
			t.Errorf("expected count 1, got %d", items[1].Count)
		}
	})

	t.Run("dashboard items skip entity lookup", func(t *testing.T) {
		app2 := testAppInstance()
		app2.Cfg.Navigation = []NavigationEntry{
			{Label: "Dashboard", Dashboard: true},
			{Label: "Tickets", List: "tickets"},
		}
		items := app2.navItems()
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].Dashboard != true {
			t.Error("expected first item to be dashboard")
		}
		if items[0].EntityType != "" {
			t.Errorf("expected empty entity type for dashboard, got %q", items[0].EntityType)
		}
	})

	t.Run("filters applied to count", func(t *testing.T) {
		app2 := testAppInstance()
		app2.Cfg.Lists["tickets"] = List{
			EntityType: "ticket",
			Filters: []FilterConfig{
				{Property: "status", Operator: "=", Value: "open"},
			},
		}
		items := app2.navItems()
		// Only TKT-001 has status=open
		if items[0].Count != 1 {
			t.Errorf("expected count 1 (filtered), got %d", items[0].Count)
		}
	})
}
