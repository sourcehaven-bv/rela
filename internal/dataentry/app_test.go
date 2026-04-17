package dataentry

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// testEntities holds references to entities created by testGraph.
// This avoids the need to look up entities by ID in tests.
type testEntities struct {
	ticket1   *model.Entity
	ticket2   *model.Entity
	component *model.Entity
}

// testMeta returns a metamodel suitable for testing app-level functions.
func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status_type":   {Values: []string{"open", "in_progress", "closed"}},
			"priority_type": {Values: []string{"low", "medium", "high"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "status_type"},
					"priority": {Type: "priority_type"},
				},
			},
			"component": {
				Label:    "Component",
				IDPrefix: "CMP",
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
					{Property: "title", Label: "Title", Link: "detail"},
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

// testGraph returns a graph with some test entities and the entities themselves.
func testGraph(meta *metamodel.Metamodel) (*graph.Graph, testEntities) {
	g := graph.New()
	t1 := testutil.EntityFor(meta, "ticket").ID("TKT-001").With("title", "First Ticket").With("status", "open").With("priority", "high").Build()
	t2 := testutil.EntityFor(meta, "ticket").ID("TKT-002").With("title", "Second Ticket").With("status", "closed").With("priority", "low").Build()
	c1 := testutil.EntityFor(meta, "component").ID("CMP-001").With("name", "Frontend").Build()
	g.AddNode(t1)
	g.AddNode(t2)
	g.AddNode(c1)
	return g, testEntities{ticket1: t1, ticket2: t2, component: c1}
}

// testAppInstance creates a minimal App for testing app-level methods.
// Returns the app and the test entities for direct use without graph lookups.
func testAppInstance() (*App, testEntities) {
	cfg := testConfig()
	meta := testMeta()
	g, entities := testGraph(meta)
	return newAppFromParts(cfg, meta, g), entities
}

func TestValidateConfig(t *testing.T) {
	meta := testMeta()
	emptyYAML := []byte(`version: "1.0"`)

	t.Run("valid config", func(t *testing.T) {
		cfg := testConfig()
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
	})

	t.Run("form with unknown entity type", func(t *testing.T) {
		cfg := &Config{
			Forms: map[string]Form{
				"bad-form": {EntityType: "nonexistent", Fields: []FormField{{Property: "title"}}},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var configErr *ConfigValidationError
		if !errors.As(err, &configErr) {
			t.Fatalf("expected ConfigValidationError, got %T", err)
		}
		if len(configErr.Errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(configErr.Errors), configErr.Errors)
		}
		if got := configErr.Errors[0]; got != `form "bad-form": unknown entity type "nonexistent"` {
			t.Errorf("unexpected error: %s", got)
		}
	})

	t.Run("form with unknown field", func(t *testing.T) {
		cfg := &Config{
			Forms: map[string]Form{
				"bad-form": {EntityType: "ticket", Fields: []FormField{{Property: "nonexistent"}}},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("form with unknown relation", func(t *testing.T) {
		cfg := &Config{
			Forms: map[string]Form{
				"bad-form": {EntityType: "ticket", Relations: []FormRelation{{Relation: "nonexistent"}}},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("list with unknown entity type", func(t *testing.T) {
		cfg := &Config{
			Lists: map[string]List{
				"bad-list": {EntityType: "nonexistent"},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("list with unknown column", func(t *testing.T) {
		cfg := &Config{
			Lists: map[string]List{
				"bad-list": {EntityType: "ticket", Columns: []ListColumn{{Property: "nonexistent"}}},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty config is valid", func(t *testing.T) {
		cfg := &Config{}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
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
	app, _ := testAppInstance()

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
		app2, _ := testAppInstance()
		app2.Cfg().Forms = map[string]Form{
			"default-form": {EntityType: "ticket", Mode: ""},
		}
		got := app2.editFormForType("ticket")
		if got != "default-form" {
			t.Errorf("expected default-form, got %s", got)
		}
	})
}

func TestCreateFormForType(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("returns create form", func(t *testing.T) {
		got := app.createFormForType("ticket")
		if got != "create-ticket" {
			t.Errorf("expected create-ticket, got %s", got)
		}
	})

	t.Run("falls back to edit form", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.Cfg().Forms = map[string]Form{
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
	app, entities := testAppInstance()

	t.Run("returns primary property value", func(t *testing.T) {
		got := app.Meta().DisplayTitle(entities.ticket1.ID, entities.ticket1.Type, entities.ticket1.Properties)
		if got != "First Ticket" {
			t.Errorf("expected 'First Ticket', got %q", got)
		}
	})

	t.Run("falls back to ID for unknown type", func(t *testing.T) {
		e := testutil.Entity("unknown").ID("UNK-001").Build()
		got := app.Meta().DisplayTitle(e.ID, e.Type, e.Properties)
		if got != "UNK-001" {
			t.Errorf("expected 'UNK-001', got %q", got)
		}
	})

	t.Run("falls back to ID when primary property is empty", func(t *testing.T) {
		e := testutil.Entity("ticket").ID("TKT-099").With("title", "").Build()
		got := app.Meta().DisplayTitle(e.ID, e.Type, e.Properties)
		if got != "TKT-099" {
			t.Errorf("expected 'TKT-099', got %q", got)
		}
	})
}

func TestActiveListForEntityType(t *testing.T) {
	app, _ := testAppInstance()

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
	app, _ := testAppInstance()

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
	app, _ := testAppInstance()

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

func TestNavElements(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("flat items with counts", func(t *testing.T) {
		elements := app.navElements("")
		if len(elements) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(elements))
		}
		item0 := elements[0].Item
		if item0 == nil {
			t.Fatal("expected first element to be an item, got group")
		}
		if item0.Label != "Tickets" {
			t.Errorf("expected label 'Tickets', got %q", item0.Label)
		}
		if item0.EntityType != "ticket" {
			t.Errorf("expected entity type 'ticket', got %q", item0.EntityType)
		}
		if item0.Count != 2 {
			t.Errorf("expected count 2, got %d", item0.Count)
		}
		item1 := elements[1].Item
		if item1 == nil {
			t.Fatal("expected second element to be an item, got group")
		}
		if item1.Label != "Components" {
			t.Errorf("expected label 'Components', got %q", item1.Label)
		}
		if item1.Count != 1 {
			t.Errorf("expected count 1, got %d", item1.Count)
		}
	})

	t.Run("dashboard items skip entity lookup", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.Cfg().Navigation = []NavigationEntry{
			{Label: "Dashboard", Dashboard: true},
			{Label: "Tickets", List: "tickets"},
		}
		elements := app2.navElements("")
		if len(elements) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(elements))
		}
		if elements[0].Item == nil || !elements[0].Item.Dashboard {
			t.Error("expected first item to be dashboard")
		}
		if elements[0].Item.EntityType != "" {
			t.Errorf("expected empty entity type for dashboard, got %q", elements[0].Item.EntityType)
		}
	})

	t.Run("filters applied to count", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.Cfg().Lists["tickets"] = List{
			EntityType: "ticket",
			Filters: []FilterConfig{
				{Property: "status", Operator: "=", Value: "open"},
			},
		}
		elements := app2.navElements("")
		// Only TKT-001 has status=open
		if elements[0].Item == nil {
			t.Fatal("expected first element to be an item")
		}
		if elements[0].Item.Count != 1 {
			t.Errorf("expected count 1 (filtered), got %d", elements[0].Item.Count)
		}
	})

	t.Run("groups with items", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.Cfg().Navigation = []NavigationEntry{
			{Label: "Dashboard", Dashboard: true},
			{
				Group: "Tickets",
				Items: []NavigationEntry{
					{Label: "All Tickets", List: "tickets"},
				},
			},
			{Label: "Components", List: "components"},
		}
		elements := app2.navElements("")
		if len(elements) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(elements))
		}
		// First: dashboard item
		if elements[0].Item == nil || !elements[0].Item.Dashboard {
			t.Error("expected first element to be dashboard item")
		}
		// Second: group
		if elements[1].Group == nil {
			t.Fatal("expected second element to be a group")
		}
		grp := elements[1].Group
		if grp.Group != "Tickets" {
			t.Errorf("expected group name 'Tickets', got %q", grp.Group)
		}
		if len(grp.Items) != 1 {
			t.Fatalf("expected 1 item in group, got %d", len(grp.Items))
		}
		if grp.Items[0].Label != "All Tickets" {
			t.Errorf("expected label 'All Tickets', got %q", grp.Items[0].Label)
		}
		if grp.Items[0].Count != 2 {
			t.Errorf("expected count 2, got %d", grp.Items[0].Count)
		}
		// Third: flat item
		if elements[2].Item == nil || elements[2].Item.Label != "Components" {
			t.Error("expected third element to be 'Components' item")
		}
	})

	t.Run("group collapsed from config", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.Cfg().Navigation = []NavigationEntry{
			{
				Group:     "Hidden",
				Collapsed: true,
				Items: []NavigationEntry{
					{Label: "Tickets", List: "tickets"},
				},
			},
		}
		elements := app2.navElements("")
		if elements[0].Group == nil {
			t.Fatal("expected group element")
		}
		if !elements[0].Group.Collapsed {
			t.Error("expected group to be collapsed")
		}
	})

	t.Run("auto-expand group containing active list", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.Cfg().Navigation = []NavigationEntry{
			{
				Group:     "Tickets",
				Collapsed: true,
				Items: []NavigationEntry{
					{Label: "All Tickets", List: "tickets"},
				},
			},
		}
		elements := app2.navElements("tickets")
		if elements[0].Group == nil {
			t.Fatal("expected group element")
		}
		if elements[0].Group.Collapsed {
			t.Error("expected group to be auto-expanded because it contains the active list")
		}
	})
}

func TestFirstNavTarget(t *testing.T) {
	t.Run("flat items returns first", func(t *testing.T) {
		nav := []NavigationEntry{
			{Label: "Tickets", List: "tickets"},
			{Label: "Dashboard", Dashboard: true},
		}
		target := firstNavTarget(nav)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if target.List != "tickets" {
			t.Errorf("expected 'tickets', got %q", target.List)
		}
	})

	t.Run("group first item", func(t *testing.T) {
		nav := []NavigationEntry{
			{
				Group: "Tickets",
				Items: []NavigationEntry{
					{Label: "All Tickets", List: "tickets"},
				},
			},
		}
		target := firstNavTarget(nav)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if target.List != "tickets" {
			t.Errorf("expected 'tickets', got %q", target.List)
		}
	})

	t.Run("empty group skipped", func(t *testing.T) {
		nav := []NavigationEntry{
			{Group: "Empty", Items: []NavigationEntry{}},
			{Label: "Dashboard", Dashboard: true},
		}
		target := firstNavTarget(nav)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if !target.Dashboard {
			t.Error("expected dashboard target")
		}
	})

	t.Run("empty navigation returns nil", func(t *testing.T) {
		target := firstNavTarget(nil)
		if target != nil {
			t.Error("expected nil for empty navigation")
		}
	})
}

func TestUIStateLoadSave(t *testing.T) {
	// Create an app with a workspace backed by memfs
	fs := storage.NewMemFS()
	ctx := &project.Context{
		Root:     "/project",
		CacheDir: "/project/.rela",
	}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	repo := repository.New(fs, ctx)

	app, _ := testAppInstance()
	app.ws = workspace.NewWithGraph(repo, app.Meta(), graphForTest(app))

	t.Run("load returns defaults when file missing", func(t *testing.T) {
		state := app.loadUIState()
		if len(state.CollapsedGroups) != 0 {
			t.Errorf("expected empty collapsed groups, got %v", state.CollapsedGroups)
		}
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		state := UIState{CollapsedGroups: map[string]bool{"Tickets": true}}
		if err := app.saveUIState(state); err != nil {
			t.Fatalf("save error: %v", err)
		}
		loaded := app.loadUIState()
		if !loaded.CollapsedGroups["Tickets"] {
			t.Error("expected Tickets to be collapsed after load")
		}
	})

	t.Run("UIState overrides config default", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.ws = workspace.NewWithGraph(repo, app2.Meta(), graphForTest(app2))
		app2.Cfg().Navigation = []NavigationEntry{
			{
				Group:     "Tickets",
				Collapsed: false,
				Items: []NavigationEntry{
					{Label: "All", List: "tickets"},
				},
			},
		}
		// UIState says collapsed
		state := UIState{CollapsedGroups: map[string]bool{"Tickets": true}}
		if err := app2.saveUIState(state); err != nil {
			t.Fatalf("save error: %v", err)
		}
		elements := app2.navElements("")
		if elements[0].Group == nil {
			t.Fatal("expected group element")
		}
		if !elements[0].Group.Collapsed {
			t.Error("expected group to be collapsed via UIState override")
		}
	})

	t.Run("nil ws is safe", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.ws = nil
		state := app2.loadUIState()
		if len(state.CollapsedGroups) != 0 {
			t.Error("expected empty state")
		}
		if err := app2.saveUIState(state); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestUserDefaultsLoadSave(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := &project.Context{
		Root:     "/project",
		CacheDir: "/project/.rela",
	}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	repo := repository.New(fs, ctx)

	app, _ := testAppInstance()
	app.ws = workspace.NewWithGraph(repo, app.Meta(), graphForTest(app))

	t.Run("load returns nil when file missing", func(t *testing.T) {
		ud := app.loadUserDefaults()
		if ud != nil {
			t.Errorf("expected nil, got %+v", ud)
		}
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		ud := &UserDefaults{
			Defaults:         map[string]string{"priority": "medium"},
			RelationDefaults: map[string]string{"belongs_to": "CMP-001"},
			Overrides: []DefaultOverride{
				{
					Types:            []string{"ticket"},
					Defaults:         map[string]string{"status": "open"},
					RelationDefaults: map[string]string{"assigned_to": "PER-001"},
				},
			},
		}
		if err := app.saveUserDefaults(ud); err != nil {
			t.Fatalf("save error: %v", err)
		}
		loaded := app.loadUserDefaults()
		if loaded == nil {
			t.Fatal("expected non-nil loaded defaults")
		}
		if loaded.Defaults["priority"] != "medium" {
			t.Errorf("expected priority=medium, got %q", loaded.Defaults["priority"])
		}
		if loaded.RelationDefaults["belongs_to"] != "CMP-001" {
			t.Errorf("expected belongs_to=CMP-001, got %q", loaded.RelationDefaults["belongs_to"])
		}
		if len(loaded.Overrides) != 1 {
			t.Fatalf("expected 1 override, got %d", len(loaded.Overrides))
		}
		if loaded.Overrides[0].Defaults["status"] != "open" {
			t.Errorf("expected override status=open, got %q", loaded.Overrides[0].Defaults["status"])
		}
		if loaded.Overrides[0].RelationDefaults["assigned_to"] != "PER-001" {
			t.Errorf("expected override assigned_to=PER-001, got %q", loaded.Overrides[0].RelationDefaults["assigned_to"])
		}
	})

	t.Run("nil ws is safe", func(t *testing.T) {
		app2, _ := testAppInstance()
		app2.ws = nil
		ud := app2.loadUserDefaults()
		if ud != nil {
			t.Errorf("expected nil, got %+v", ud)
		}
		if err := app2.saveUserDefaults(&UserDefaults{}); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestValidateConfigNestedGroups(t *testing.T) {
	meta := testMeta()
	emptyYAML := []byte(`version: "1.0"`)

	t.Run("nested groups rejected", func(t *testing.T) {
		cfg := &Config{
			Navigation: []NavigationEntry{
				{
					Group: "Outer",
					Items: []NavigationEntry{
						{
							Group: "Inner",
							Items: []NavigationEntry{
								{Label: "Tickets", List: "tickets"},
							},
						},
					},
				},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "nested group") {
			t.Errorf("expected nested group error, got: %s", err.Error())
		}
	})

	t.Run("flat groups are valid", func(t *testing.T) {
		cfg := &Config{
			Lists: map[string]List{
				"tickets": {EntityType: "ticket"},
			},
			Navigation: []NavigationEntry{
				{
					Group: "Tickets",
					Items: []NavigationEntry{
						{Label: "All", List: "tickets"},
					},
				},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
	})
}

func TestActiveListForEntityTypeWithGroups(t *testing.T) {
	app, _ := testAppInstance()
	app.Cfg().Navigation = []NavigationEntry{
		{
			Group: "Tickets",
			Items: []NavigationEntry{
				{Label: "All Tickets", List: "tickets"},
			},
		},
		{Label: "Components", List: "components"},
	}

	t.Run("finds list inside group", func(t *testing.T) {
		got := app.activeListForEntityType("ticket")
		if got != "tickets" {
			t.Errorf("expected 'tickets', got %q", got)
		}
	})

	t.Run("finds flat list", func(t *testing.T) {
		got := app.activeListForEntityType("component")
		if got != "components" {
			t.Errorf("expected 'components', got %q", got)
		}
	})
}
