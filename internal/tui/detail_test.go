package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// createTestAppWithRelations creates a test app with entities and relations
func createTestAppWithRelations() *App {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string", Required: true},
					"status":      {Type: "string", Required: false},
					"priority":    {Type: "string", Required: false},
					"description": {Type: "string", Required: false},
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
			"title":       "Test Requirement 1",
			"status":      "draft",
			"priority":    "high",
			"description": "This is a test requirement",
		},
		Content: "# Details\n\nSome detailed content here.",
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

	// Add relations
	g.AddEdge(&model.Relation{
		From: "DEC-001",
		Type: "implements",
		To:   "REQ-001",
	})
	g.AddEdge(&model.Relation{
		From: "DEC-001",
		Type: "implements",
		To:   "REQ-002",
	})

	app := &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}

	return app
}

func TestDetailModel_Initialization(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	if detail == nil {
		t.Fatal("NewDetailModel returned nil")
	}

	if detail.entityID != "REQ-001" {
		t.Errorf("detail.entityID = %q, want 'REQ-001'", detail.entityID)
	}

	if detail.entity == nil {
		t.Fatal("detail.entity is nil")
	}

	if detail.entity.ID != "REQ-001" {
		t.Errorf("detail.entity.ID = %q, want 'REQ-001'", detail.entity.ID)
	}

	// Should have loaded relations
	if len(detail.incoming) == 0 {
		t.Error("detail.incoming should not be empty")
	}

	if len(detail.allRels) == 0 {
		t.Error("detail.allRels should not be empty")
	}
}

func TestDetailModel_LoadRelations(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// REQ-001 has 1 incoming relation from DEC-001
	if len(detail.incoming) != 1 {
		t.Errorf("expected 1 incoming relation, got %d", len(detail.incoming))
	}

	// REQ-001 has no outgoing relations
	if len(detail.outgoing) != 0 {
		t.Errorf("expected 0 outgoing relations, got %d", len(detail.outgoing))
	}

	// Check the incoming relation
	if detail.incoming[0].From != "DEC-001" {
		t.Errorf("incoming relation from = %q, want 'DEC-001'", detail.incoming[0].From)
	}
	if detail.incoming[0].Type != "implements" {
		t.Errorf("incoming relation type = %q, want 'implements'", detail.incoming[0].Type)
	}

	// Check DEC-001 (has outgoing relations)
	detail2 := NewDetailModel(app, "DEC-001")
	if len(detail2.outgoing) != 2 {
		t.Errorf("expected 2 outgoing relations from DEC-001, got %d", len(detail2.outgoing))
	}
}

func TestDetailModel_ScrollNavigation(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Start at scrollPos 0
	if detail.scrollPos != 0 {
		t.Fatalf("scrollPos should start at 0, got %d", detail.scrollPos)
	}

	// Scroll down
	detail.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if detail.scrollPos != 1 {
		t.Errorf("after down, scrollPos = %d, want 1", detail.scrollPos)
	}

	// Scroll up
	detail.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if detail.scrollPos != 0 {
		t.Errorf("after up, scrollPos = %d, want 0", detail.scrollPos)
	}

	// Scroll up again (should stay at 0)
	detail.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if detail.scrollPos != 0 {
		t.Errorf("after up at start, scrollPos = %d, want 0", detail.scrollPos)
	}
}

func TestDetailModel_RelationModeToggle(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Start in scroll mode
	if detail.relMode {
		t.Fatal("should start in scroll mode (relMode = false)")
	}

	// Toggle to relation mode
	detail.Update(app, tea.KeyMsg{Type: tea.KeyTab})
	if !detail.relMode {
		t.Error("after tab, should be in relation mode")
	}

	// Toggle back to scroll mode
	detail.Update(app, tea.KeyMsg{Type: tea.KeyTab})
	if detail.relMode {
		t.Error("after second tab, should be in scroll mode")
	}
}

func TestDetailModel_RelationNavigation(t *testing.T) {
	app := createTestAppWithRelations()
	// Use DEC-001 which has 2 outgoing relations
	detail := NewDetailModel(app, "DEC-001")

	// Switch to relation mode
	detail.relMode = true

	// Start at relIndex 0
	if detail.relIndex != 0 {
		t.Fatalf("relIndex should start at 0, got %d", detail.relIndex)
	}

	// Navigate down
	detail.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if detail.relIndex != 1 {
		t.Errorf("after down, relIndex = %d, want 1", detail.relIndex)
	}

	// Navigate down again (should stay at 1, at the end)
	detail.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if detail.relIndex != 1 {
		t.Errorf("after down at end, relIndex = %d, want 1", detail.relIndex)
	}

	// Navigate up
	detail.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if detail.relIndex != 0 {
		t.Errorf("after up, relIndex = %d, want 0", detail.relIndex)
	}

	// Navigate up again (should stay at 0)
	detail.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if detail.relIndex != 0 {
		t.Errorf("after up at start, relIndex = %d, want 0", detail.relIndex)
	}
}

func TestDetailModel_View(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	view := detail.View(80, 40)

	// Should contain entity ID
	if !contains(view, "REQ-001") {
		t.Error("View should contain entity ID 'REQ-001'")
	}

	// Should contain title
	if !contains(view, "Test Requirement 1") {
		t.Error("View should contain entity title")
	}

	// Should contain properties
	if !contains(view, "Status:") {
		t.Error("View should contain 'Status:' label")
	}
	if !contains(view, "draft") {
		t.Error("View should contain 'draft' status")
	}

	if !contains(view, "Priority:") {
		t.Error("View should contain 'Priority:' label")
	}
	if !contains(view, "high") {
		t.Error("View should contain 'high' priority")
	}

	if !contains(view, "Description:") {
		t.Error("View should contain 'Description:' label")
	}

	// Should contain relations section
	if !contains(view, "Relations") {
		t.Error("View should contain 'Relations' section")
	}

	if !contains(view, "Incoming:") {
		t.Error("View should contain 'Incoming:' label")
	}

	if !contains(view, "DEC-001") {
		t.Error("View should contain 'DEC-001' in relations")
	}

	// Should contain content section
	if !contains(view, "Content") {
		t.Error("View should contain 'Content' section")
	}
	if !contains(view, "Details") {
		t.Error("View should contain content text")
	}
}

func TestDetailModel_ViewInRelationMode(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Switch to relation mode
	detail.relMode = true

	view := detail.View(80, 40)

	// Should show relation navigation indicator
	if !contains(view, "navigation mode") {
		t.Error("View should indicate navigation mode")
	}

	// Should show selection marker on relation
	if !contains(view, "►") {
		t.Error("View should show selection marker on relation")
	}
}

func TestDetailModel_ViewWithoutRelations(t *testing.T) {
	// Create app with entity that has no relations
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}

	g := graph.New()
	g.AddNode(&model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title": "Standalone Requirement",
		},
	})

	app := &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}

	detail := NewDetailModel(app, "REQ-001")
	view := detail.View(80, 40)

	// Should not contain Relations section when there are no relations
	if contains(view, "Relations") {
		t.Error("View should not contain 'Relations' section when there are no relations")
	}
}

func TestDetailModel_ViewNotFound(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "NONEXISTENT")

	view := detail.View(80, 40)

	// Should show "Entity not found" message
	if !contains(view, "Entity not found") {
		t.Error("View should contain 'Entity not found' for non-existent entity")
	}
}

func TestDetailModel_Help(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Help in scroll mode
	help := detail.Help()
	if len(help) == 0 {
		t.Error("Help should return items for scroll mode")
	}

	// Check for expected keys in scroll mode
	foundScroll := false
	foundRelMode := false
	for _, item := range help {
		if item[1] == "scroll" {
			foundScroll = true
		}
		if item[1] == "rel mode" {
			foundRelMode = true
		}
	}
	if !foundScroll {
		t.Error("Help should contain 'scroll' in scroll mode")
	}
	if !foundRelMode {
		t.Error("Help should contain 'rel mode' in scroll mode")
	}

	// Help in relation mode
	detail.relMode = true
	help = detail.Help()
	if len(help) == 0 {
		t.Error("Help should return items for relation mode")
	}

	// Check for expected keys in relation mode
	foundNavigate := false
	foundFollow := false
	foundScrollMode := false
	for _, item := range help {
		if item[1] == "navigate" {
			foundNavigate = true
		}
		if item[1] == "follow" {
			foundFollow = true
		}
		if item[1] == "scroll mode" {
			foundScrollMode = true
		}
	}
	if !foundNavigate {
		t.Error("Help should contain 'navigate' in relation mode")
	}
	if !foundFollow {
		t.Error("Help should contain 'follow' in relation mode")
	}
	if !foundScrollMode {
		t.Error("Help should contain 'scroll mode' in relation mode")
	}
}

func TestDetailModel_HelpInConfirmDelete(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Enter confirm delete mode
	detail.confirmDelete = true

	help := detail.Help()
	if len(help) == 0 {
		t.Error("Help should return items for confirm delete mode")
	}

	// Check for expected keys in confirm delete mode
	foundConfirm := false
	foundCancel := false
	for _, item := range help {
		if item[1] == "confirm" {
			foundConfirm = true
		}
		if item[1] == "cancel" {
			foundCancel = true
		}
	}
	if !foundConfirm {
		t.Error("Help should contain 'confirm' in confirm delete mode")
	}
	if !foundCancel {
		t.Error("Help should contain 'cancel' in confirm delete mode")
	}
}

func TestDetailModel_ViewConfirmDeleteEntity(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Enter confirm delete entity mode
	detail.confirmDelete = true
	detail.confirmDeleteEntity = true

	view := detail.View(80, 40)

	// Should show confirmation message
	if !contains(view, "Delete entity REQ-001?") {
		t.Error("View should show delete entity confirmation")
	}

	// Should show relation count
	if !contains(view, "1 relation") {
		t.Error("View should show relation count in confirmation")
	}

	// Should show y/N prompt
	if !contains(view, "(y/N)") {
		t.Error("View should show y/N prompt")
	}
}

func TestDetailModel_ViewConfirmDeleteRelation(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Enter confirm delete relation mode
	detail.relMode = true
	detail.confirmDelete = true
	detail.confirmDeleteEntity = false

	view := detail.View(80, 40)

	// Should show confirmation message for relation
	if !contains(view, "Delete relation") {
		t.Error("View should show delete relation confirmation")
	}

	if !contains(view, "DEC-001") {
		t.Error("View should show relation details")
	}

	// Should show y/N prompt
	if !contains(view, "(y/N)") {
		t.Error("View should show y/N prompt")
	}
}

func TestDetailModel_CancelDeleteWithN(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Enter confirm delete mode
	detail.confirmDelete = true
	detail.confirmDeleteEntity = true

	// Press 'n' to cancel
	_, cmd := detail.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	// Should exit confirm delete mode
	if detail.confirmDelete {
		t.Error("confirmDelete should be false after pressing 'n'")
	}

	// Should return a message command
	if cmd == nil {
		t.Error("Update should return a SetMessage command")
	}

	// Get the message
	msg := cmd()
	if setMsg, ok := msg.(setMessageMsg); ok {
		if setMsg.message != "Delete cancelled" {
			t.Errorf("message = %q, want 'Delete cancelled'", setMsg.message)
		}
		if setMsg.isError {
			t.Error("message should not be an error")
		}
	} else {
		t.Error("cmd should return setMessageMsg")
	}
}

func TestDetailModel_CancelDeleteWithEsc(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Enter confirm delete mode
	detail.confirmDelete = true
	detail.confirmDeleteEntity = true

	// Press 'esc' to cancel
	detail.Update(app, tea.KeyMsg{Type: tea.KeyEsc})

	// Should exit confirm delete mode
	if detail.confirmDelete {
		t.Error("confirmDelete should be false after pressing 'esc'")
	}
}

func TestDetailModel_CancelDeleteWithAnyKey(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Enter confirm delete mode
	detail.confirmDelete = true
	detail.confirmDeleteEntity = true

	// Press any other key to cancel (default is No)
	detail.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	// Should exit confirm delete mode
	if detail.confirmDelete {
		t.Error("confirmDelete should be false after pressing any key")
	}
}

func TestDetailModel_EnterDeleteRelationMode(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Switch to relation mode
	detail.relMode = true

	// Press 'd' to enter delete relation mode
	detail.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	// Should enter confirm delete mode
	if !detail.confirmDelete {
		t.Error("confirmDelete should be true after pressing 'd'")
	}
	if detail.confirmDeleteEntity {
		t.Error("confirmDeleteEntity should be false for relation delete")
	}
}

func TestDetailModel_EnterDeleteEntityMode(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "REQ-001")

	// Press 'D' to enter delete entity mode
	detail.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})

	// Should enter confirm delete mode
	if !detail.confirmDelete {
		t.Error("confirmDelete should be true after pressing 'D'")
	}
	if !detail.confirmDeleteEntity {
		t.Error("confirmDeleteEntity should be true for entity delete")
	}
}

func TestDetailModel_AllRelsContainsIncomingAndOutgoing(t *testing.T) {
	app := createTestAppWithRelations()
	detail := NewDetailModel(app, "DEC-001")

	// DEC-001 has 2 outgoing relations and 0 incoming
	expectedTotal := len(detail.incoming) + len(detail.outgoing)

	if len(detail.allRels) != expectedTotal {
		t.Errorf("allRels length = %d, want %d (incoming + outgoing)", len(detail.allRels), expectedTotal)
	}

	if len(detail.allRels) != 2 {
		t.Errorf("allRels length = %d, want 2", len(detail.allRels))
	}
}
