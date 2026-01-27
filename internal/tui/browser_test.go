package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// createTestApp creates a test app with a test metamodel and graph
func createTestApp() *App {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string", Required: false},
				},
			},
			"decision": {
				Label:      "Decision",
				IDPatterns: []string{"DEC-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string", Required: false},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "Implements",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}

	g := graph.New()

	// Add test entities
	g.AddNode(&model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "Test Requirement 1",
			"status": "draft",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "REQ-002",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "Test Requirement 2",
			"status": "approved",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "DEC-001",
		Type: "decision",
		Properties: map[string]interface{}{
			"title":  "Test Decision",
			"status": "proposed",
		},
	})

	app := &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}

	return app
}

func TestBrowserModel_Initialization(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	if browser == nil {
		t.Fatal("NewBrowserModel returned nil")
	}

	if browser.level != LevelTypes {
		t.Errorf("browser.level = %v, want %v", browser.level, LevelTypes)
	}

	if browser.typeIndex != 0 {
		t.Errorf("browser.typeIndex = %d, want 0", browser.typeIndex)
	}

	// Should have loaded types
	if len(browser.types) != 2 {
		t.Errorf("browser.types length = %d, want 2", len(browser.types))
	}
}

func TestBrowserModel_LoadTypes(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Verify types are loaded and sorted by label
	if len(browser.types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(browser.types))
	}

	// Types should be sorted by label: Decision, Requirement
	if browser.types[0].label != "Decision" {
		t.Errorf("first type label = %q, want 'Decision'", browser.types[0].label)
	}
	if browser.types[1].label != "Requirement" {
		t.Errorf("second type label = %q, want 'Requirement'", browser.types[1].label)
	}

	// Check counts
	if browser.types[0].count != 1 {
		t.Errorf("Decision count = %d, want 1", browser.types[0].count)
	}
	if browser.types[1].count != 2 {
		t.Errorf("Requirement count = %d, want 2", browser.types[1].count)
	}
}

func TestBrowserModel_NavigationTypes(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Start at index 0
	if browser.typeIndex != 0 {
		t.Fatalf("typeIndex should start at 0, got %d", browser.typeIndex)
	}

	// Navigate down
	browser.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if browser.typeIndex != 1 {
		t.Errorf("after down, typeIndex = %d, want 1", browser.typeIndex)
	}

	// Navigate down again (should stay at 1, at the end)
	browser.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if browser.typeIndex != 1 {
		t.Errorf("after down at end, typeIndex = %d, want 1", browser.typeIndex)
	}

	// Navigate up
	browser.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if browser.typeIndex != 0 {
		t.Errorf("after up, typeIndex = %d, want 0", browser.typeIndex)
	}

	// Navigate up again (should stay at 0, at the start)
	browser.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if browser.typeIndex != 0 {
		t.Errorf("after up at start, typeIndex = %d, want 0", browser.typeIndex)
	}
}

func TestBrowserModel_NavigationTypesWithJ_K(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Navigate with 'j' (down)
	browser.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if browser.typeIndex != 1 {
		t.Errorf("after 'j', typeIndex = %d, want 1", browser.typeIndex)
	}

	// Navigate with 'k' (up)
	browser.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if browser.typeIndex != 0 {
		t.Errorf("after 'k', typeIndex = %d, want 0", browser.typeIndex)
	}
}

func TestBrowserModel_NavigationTypesWithG_Home_End(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Move to end with 'G'
	browser.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if browser.typeIndex != 1 {
		t.Errorf("after 'G', typeIndex = %d, want 1", browser.typeIndex)
	}

	// Move to start with 'g'
	browser.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if browser.typeIndex != 0 {
		t.Errorf("after 'g', typeIndex = %d, want 0", browser.typeIndex)
	}

	// Move to end with 'end'
	browser.Update(app, tea.KeyMsg{Type: tea.KeyEnd})
	if browser.typeIndex != 1 {
		t.Errorf("after 'end', typeIndex = %d, want 1", browser.typeIndex)
	}

	// Move to start with 'home'
	browser.Update(app, tea.KeyMsg{Type: tea.KeyHome})
	if browser.typeIndex != 0 {
		t.Errorf("after 'home', typeIndex = %d, want 0", browser.typeIndex)
	}
}

func TestBrowserModel_SelectType(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Select first type (Decision) with Enter
	browser.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Should be at LevelEntities now
	if browser.level != LevelEntities {
		t.Errorf("browser.level = %v, want %v", browser.level, LevelEntities)
	}

	// Should have selected decision type
	if browser.selectedType != "decision" {
		t.Errorf("browser.selectedType = %q, want 'decision'", browser.selectedType)
	}

	// Should have loaded entities
	if len(browser.entities) != 1 {
		t.Errorf("browser.entities length = %d, want 1", len(browser.entities))
	}

	// Entity index should be 0
	if browser.entityIndex != 0 {
		t.Errorf("browser.entityIndex = %d, want 0", browser.entityIndex)
	}
}

func TestBrowserModel_LoadEntities(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Load requirement entities
	browser.loadEntities(app, "requirement")

	if len(browser.entities) != 2 {
		t.Errorf("expected 2 requirement entities, got %d", len(browser.entities))
	}

	// Entities should be sorted by ID
	if browser.entities[0].ID != "REQ-001" {
		t.Errorf("first entity ID = %q, want 'REQ-001'", browser.entities[0].ID)
	}
	if browser.entities[1].ID != "REQ-002" {
		t.Errorf("second entity ID = %q, want 'REQ-002'", browser.entities[1].ID)
	}

	// Load decision entities
	browser.loadEntities(app, "decision")

	if len(browser.entities) != 1 {
		t.Errorf("expected 1 decision entity, got %d", len(browser.entities))
	}

	if browser.entities[0].ID != "DEC-001" {
		t.Errorf("decision entity ID = %q, want 'DEC-001'", browser.entities[0].ID)
	}
}

func TestBrowserModel_NavigationEntities(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Load requirement entities (2 entities)
	browser.loadEntities(app, "requirement")
	browser.level = LevelEntities

	// Start at index 0
	if browser.entityIndex != 0 {
		t.Fatalf("entityIndex should start at 0, got %d", browser.entityIndex)
	}

	// Navigate down
	browser.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if browser.entityIndex != 1 {
		t.Errorf("after down, entityIndex = %d, want 1", browser.entityIndex)
	}

	// Navigate down again (should stay at 1, at the end)
	browser.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if browser.entityIndex != 1 {
		t.Errorf("after down at end, entityIndex = %d, want 1", browser.entityIndex)
	}

	// Navigate up
	browser.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if browser.entityIndex != 0 {
		t.Errorf("after up, entityIndex = %d, want 0", browser.entityIndex)
	}
}

func TestBrowserModel_BackToTypes(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Load entities
	browser.loadEntities(app, "requirement")
	browser.level = LevelEntities

	// Press backspace to go back
	browser.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})

	// Should be back at LevelTypes
	if browser.level != LevelTypes {
		t.Errorf("browser.level = %v, want %v", browser.level, LevelTypes)
	}

	// Entities should be cleared
	if browser.entities != nil {
		t.Errorf("browser.entities should be nil after going back, got %v", browser.entities)
	}
}

func TestBrowserModel_BackToTypesWithH(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Load entities
	browser.loadEntities(app, "requirement")
	browser.level = LevelEntities

	// Press 'h' to go back
	browser.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	// Should be back at LevelTypes
	if browser.level != LevelTypes {
		t.Errorf("browser.level = %v, want %v", browser.level, LevelTypes)
	}
}

func TestBrowserModel_ViewTypes(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	view := browser.View(80, 40)

	// Should contain "Entity Types"
	if !contains(view, "Entity Types") {
		t.Error("View should contain 'Entity Types'")
	}

	// Should contain both type labels
	if !contains(view, "Decision") {
		t.Error("View should contain 'Decision'")
	}
	if !contains(view, "Requirement") {
		t.Error("View should contain 'Requirement'")
	}

	// Should show counts
	if !contains(view, "(1)") {
		t.Error("View should contain '(1)' for Decision count")
	}
	if !contains(view, "(2)") {
		t.Error("View should contain '(2)' for Requirement count")
	}

	// Should have selection marker for first item
	if !contains(view, "►") {
		t.Error("View should contain '►' selection marker")
	}
}

func TestBrowserModel_ViewEntities(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Load entities
	browser.loadEntities(app, "requirement")
	browser.level = LevelEntities

	view := browser.View(80, 40)

	// Should contain type label
	if !contains(view, "Requirements") {
		t.Error("View should contain 'Requirements'")
	}

	// Should contain entity IDs
	if !contains(view, "REQ-001") {
		t.Error("View should contain 'REQ-001'")
	}
	if !contains(view, "REQ-002") {
		t.Error("View should contain 'REQ-002'")
	}

	// Should contain status
	if !contains(view, "draft") {
		t.Error("View should contain 'draft' status")
	}
	if !contains(view, "approved") {
		t.Error("View should contain 'approved' status")
	}

	// Should have selection marker
	if !contains(view, "►") {
		t.Error("View should contain '►' selection marker")
	}
}

func TestBrowserModel_ViewEntitiesEmpty(t *testing.T) {
	// Create app with no entities for a type
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
			},
		},
	}

	app := &App{
		metamodel: meta,
		graph:     graph.New(),
		project:   &project.Context{Root: "/tmp/test"},
	}

	browser := NewBrowserModel(app)
	browser.loadEntities(app, "requirement")
	browser.level = LevelEntities

	view := browser.View(80, 40)

	// Should show "No entities found" message
	if !contains(view, "No entities found") {
		t.Error("View should contain 'No entities found'")
	}
}

func TestBrowserModel_Help(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Help at LevelTypes
	help := browser.Help()
	if len(help) == 0 {
		t.Error("Help should return items for LevelTypes")
	}

	// Check that it contains expected keys
	foundNavigate := false
	foundSelect := false
	for _, item := range help {
		if item[1] == "navigate" {
			foundNavigate = true
		}
		if item[1] == "select" {
			foundSelect = true
		}
	}
	if !foundNavigate {
		t.Error("Help should contain 'navigate'")
	}
	if !foundSelect {
		t.Error("Help should contain 'select'")
	}

	// Help at LevelEntities
	browser.level = LevelEntities
	help = browser.Help()
	if len(help) == 0 {
		t.Error("Help should return items for LevelEntities")
	}

	// Check that it contains expected keys
	foundView := false
	foundBack := false
	for _, item := range help {
		if item[1] == "view" {
			foundView = true
		}
		if item[1] == "back" {
			foundBack = true
		}
	}
	if !foundView {
		t.Error("Help should contain 'view'")
	}
	if !foundBack {
		t.Error("Help should contain 'back'")
	}
}

func TestBrowserModel_Refresh(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Add a new entity
	app.graph.AddNode(&model.Entity{
		ID:   "REQ-003",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title": "New Requirement",
		},
	})

	// Refresh
	browser.Refresh(app)

	// Type count should be updated
	for _, typ := range browser.types {
		if typ.name == "requirement" {
			if typ.count != 3 {
				t.Errorf("after refresh, requirement count = %d, want 3", typ.count)
			}
		}
	}
}

func TestBrowserModel_RefreshInEntityLevel(t *testing.T) {
	app := createTestApp()
	browser := NewBrowserModel(app)

	// Load entities
	browser.loadEntities(app, "requirement")
	browser.level = LevelEntities

	// Should have 2 entities
	if len(browser.entities) != 2 {
		t.Fatalf("expected 2 entities before refresh, got %d", len(browser.entities))
	}

	// Add a new entity
	app.graph.AddNode(&model.Entity{
		ID:   "REQ-003",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title": "New Requirement",
		},
	})

	// Refresh
	browser.Refresh(app)

	// Should have 3 entities now
	if len(browser.entities) != 3 {
		t.Errorf("after refresh, entities length = %d, want 3", len(browser.entities))
	}
}
