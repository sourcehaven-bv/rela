package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// createLinkTestApp creates a test app with entities and relations for link testing.
func createLinkTestApp() *App {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"component": {
				Label:    "Component",
				IDPrefix: "CMP-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "Implements",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
			"affects": {
				Label: "Affects",
				From:  []string{"decision"},
				To:    []string{"component"},
			},
			"depends-on": {
				Label: "Depends On",
				From:  []string{"component"},
				To:    []string{"component"},
			},
		},
	}

	g := graph.New()

	// Add test entities
	g.AddNode(&model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title": "First Requirement",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "REQ-002",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title": "Second Requirement",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "DEC-001",
		Type: "decision",
		Properties: map[string]interface{}{
			"title": "Important Decision",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "CMP-001",
		Type: "component",
		Properties: map[string]interface{}{
			"title": "API Component",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "CMP-002",
		Type: "component",
		Properties: map[string]interface{}{
			"title": "Database Component",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "CMP-003",
		Type: "component",
		Properties: map[string]interface{}{
			"title": "Cache Layer",
		},
	})

	return &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}
}

func TestLinkModel_NewLinkModel(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	if l == nil {
		t.Fatal("NewLinkModel returned nil")
	}

	if l.sourceID != "DEC-001" {
		t.Errorf("sourceID = %q, want %q", l.sourceID, "DEC-001")
	}

	if l.sourceType != "decision" {
		t.Errorf("sourceType = %q, want %q", l.sourceType, "decision")
	}

	if l.step != LinkStepRelation {
		t.Errorf("step = %v, want %v", l.step, LinkStepRelation)
	}

	// Decision type should have 2 relations: implements, affects
	if len(l.relations) != 2 {
		t.Errorf("relations count = %d, want 2", len(l.relations))
	}
}

func TestLinkModel_NewLinkModel_UnknownEntity(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "UNKNOWN-001")

	// Should still create model, just with empty source type
	if l == nil {
		t.Fatal("NewLinkModel returned nil")
	}

	if l.sourceID != "UNKNOWN-001" {
		t.Errorf("sourceID = %q, want %q", l.sourceID, "UNKNOWN-001")
	}

	if l.sourceType != "" {
		t.Errorf("sourceType = %q, want empty string", l.sourceType)
	}

	// Unknown type should have no relations
	if len(l.relations) != 0 {
		t.Errorf("relations count = %d, want 0", len(l.relations))
	}
}

func TestLinkModel_LoadRelations(t *testing.T) {
	app := createLinkTestApp()

	tests := []struct {
		name          string
		sourceID      string
		wantRelations int
		wantNames     []string
	}{
		{
			name:          "decision has 2 relations",
			sourceID:      "DEC-001",
			wantRelations: 2,
			wantNames:     []string{"affects", "implements"},
		},
		{
			name:          "component has 1 relation",
			sourceID:      "CMP-001",
			wantRelations: 1,
			wantNames:     []string{"depends-on"},
		},
		{
			name:          "requirement has no outgoing relations",
			sourceID:      "REQ-001",
			wantRelations: 0,
			wantNames:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLinkModel(app, tt.sourceID)

			if len(l.relations) != tt.wantRelations {
				t.Errorf("relations count = %d, want %d", len(l.relations), tt.wantRelations)
			}

			// Check relation names are present
			for _, wantName := range tt.wantNames {
				found := false
				for _, rel := range l.relations {
					if rel.name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected relation %q not found", wantName)
				}
			}
		})
	}
}

func TestLinkModel_LoadRelations_Sorted(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Relations should be sorted by label
	// "Affects" comes before "Implements" alphabetically
	if len(l.relations) < 2 {
		t.Fatalf("expected at least 2 relations, got %d", len(l.relations))
	}

	if l.relations[0].label != "Affects" {
		t.Errorf("first relation label = %q, want 'Affects'", l.relations[0].label)
	}
	if l.relations[1].label != "Implements" {
		t.Errorf("second relation label = %q, want 'Implements'", l.relations[1].label)
	}
}

func TestLinkModel_LoadTargets(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Find "implements" relation index (targets requirements)
	for i, rel := range l.relations {
		if rel.name == "implements" {
			l.relIndex = i
			break
		}
	}

	l.loadTargets(app)

	// Should have 2 requirements as targets
	if len(l.targets) != 2 {
		t.Errorf("targets count = %d, want 2", len(l.targets))
	}

	// Check that all targets are requirements
	for _, target := range l.targets {
		if target.Type != "requirement" {
			t.Errorf("unexpected target type %q, want 'requirement'", target.Type)
		}
	}
}

func TestLinkModel_LoadTargets_ExcludesSelf(t *testing.T) {
	app := createLinkTestApp()

	// Use component which can link to other components
	l := NewLinkModel(app, "CMP-001")

	// Find "depends-on" relation
	for i, rel := range l.relations {
		if rel.name == "depends-on" {
			l.relIndex = i
			break
		}
	}

	l.loadTargets(app)

	// Should have 2 components (excluding CMP-001 itself)
	if len(l.targets) != 2 {
		t.Errorf("targets count = %d, want 2", len(l.targets))
	}

	// Check that CMP-001 is not in targets
	for _, target := range l.targets {
		if target.ID == "CMP-001" {
			t.Error("self (CMP-001) should be excluded from targets")
		}
	}
}

func TestLinkModel_LoadTargets_Sorted(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "CMP-001")

	// Find "depends-on" relation
	for i, rel := range l.relations {
		if rel.name == "depends-on" {
			l.relIndex = i
			break
		}
	}

	l.loadTargets(app)

	// Should be sorted by ID: CMP-002, CMP-003
	if len(l.targets) < 2 {
		t.Fatalf("expected at least 2 targets, got %d", len(l.targets))
	}

	if l.targets[0].ID != "CMP-002" {
		t.Errorf("first target ID = %q, want 'CMP-002'", l.targets[0].ID)
	}
	if l.targets[1].ID != "CMP-003" {
		t.Errorf("second target ID = %q, want 'CMP-003'", l.targets[1].ID)
	}
}

func TestLinkModel_LoadTargets_InvalidRelIndex(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")
	l.relIndex = 999 // Invalid index

	l.loadTargets(app)

	// Should have no targets
	if len(l.targets) != 0 {
		t.Errorf("targets count = %d, want 0", len(l.targets))
	}
}

func TestLinkModel_ApplyFilter(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "CMP-001")

	// Find "depends-on" relation and load targets
	for i, rel := range l.relations {
		if rel.name == "depends-on" {
			l.relIndex = i
			break
		}
	}
	l.loadTargets(app)

	tests := []struct {
		name      string
		filter    string
		wantCount int
		wantIDs   []string
		wantIndex int
	}{
		{
			name:      "empty filter shows all",
			filter:    "",
			wantCount: 2,
			wantIDs:   []string{"CMP-002", "CMP-003"},
		},
		{
			name:      "filter by ID prefix",
			filter:    "CMP-002",
			wantCount: 1,
			wantIDs:   []string{"CMP-002"},
			wantIndex: 0,
		},
		{
			name:      "filter by title (case insensitive)",
			filter:    "database",
			wantCount: 1,
			wantIDs:   []string{"CMP-002"},
			wantIndex: 0,
		},
		{
			name:      "filter by partial title",
			filter:    "cache",
			wantCount: 1,
			wantIDs:   []string{"CMP-003"},
			wantIndex: 0,
		},
		{
			name:      "filter with no matches",
			filter:    "nonexistent",
			wantCount: 0,
			wantIDs:   nil,
			wantIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l.filterText = tt.filter
			l.targetIndex = 5 // Set to non-zero to verify reset
			l.applyFilter()

			if len(l.filteredList) != tt.wantCount {
				t.Errorf("filteredList count = %d, want %d", len(l.filteredList), tt.wantCount)
			}

			for _, wantID := range tt.wantIDs {
				found := false
				for _, e := range l.filteredList {
					if e.ID == wantID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected entity %q not found in filtered list", wantID)
				}
			}

			// Filter should reset targetIndex to 0
			if tt.filter != "" && l.targetIndex != tt.wantIndex {
				t.Errorf("targetIndex = %d, want %d", l.targetIndex, tt.wantIndex)
			}
		})
	}
}

func TestLinkModel_UpdateRelation_Navigation(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Start at index 0
	if l.relIndex != 0 {
		t.Fatalf("relIndex should start at 0, got %d", l.relIndex)
	}

	// Navigate down with 'j'
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if l.relIndex != 1 {
		t.Errorf("after 'j', relIndex = %d, want 1", l.relIndex)
	}

	// Navigate down again (should stay at 1, at end)
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if l.relIndex != 1 {
		t.Errorf("after 'j' at end, relIndex = %d, want 1", l.relIndex)
	}

	// Navigate up with 'k'
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if l.relIndex != 0 {
		t.Errorf("after 'k', relIndex = %d, want 0", l.relIndex)
	}

	// Navigate up again (should stay at 0, at start)
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if l.relIndex != 0 {
		t.Errorf("after 'k' at start, relIndex = %d, want 0", l.relIndex)
	}
}

func TestLinkModel_UpdateRelation_ArrowKeys(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Navigate down with arrow
	l.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	if l.relIndex != 1 {
		t.Errorf("after down, relIndex = %d, want 1", l.relIndex)
	}

	// Navigate up with arrow
	l.Update(app, tea.KeyMsg{Type: tea.KeyUp})
	if l.relIndex != 0 {
		t.Errorf("after up, relIndex = %d, want 0", l.relIndex)
	}
}

func TestLinkModel_UpdateRelation_Enter(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Press enter to select first relation
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Should advance to target step
	if l.step != LinkStepTarget {
		t.Errorf("step = %v, want %v", l.step, LinkStepTarget)
	}

	// Should have selected the relation
	if l.selectedRel == "" {
		t.Error("selectedRel should not be empty")
	}

	// Should have loaded targets
	if l.filteredList == nil {
		t.Error("filteredList should be loaded")
	}

	// targetIndex should be reset
	if l.targetIndex != 0 {
		t.Errorf("targetIndex = %d, want 0", l.targetIndex)
	}

	// filterText should be reset
	if l.filterText != "" {
		t.Errorf("filterText = %q, want empty", l.filterText)
	}
}

func TestLinkModel_UpdateTarget_Navigation(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "CMP-001")

	// Select depends-on relation and advance to target step
	for i, rel := range l.relations {
		if rel.name == "depends-on" {
			l.relIndex = i
			break
		}
	}
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Should be at target step with 2 targets
	if l.step != LinkStepTarget {
		t.Fatalf("step = %v, want %v", l.step, LinkStepTarget)
	}
	if len(l.filteredList) != 2 {
		t.Fatalf("filteredList count = %d, want 2", len(l.filteredList))
	}

	// Navigate down
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if l.targetIndex != 1 {
		t.Errorf("after 'j', targetIndex = %d, want 1", l.targetIndex)
	}

	// Navigate up
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if l.targetIndex != 0 {
		t.Errorf("after 'k', targetIndex = %d, want 0", l.targetIndex)
	}
}

func TestLinkModel_UpdateTarget_Filter(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "CMP-001")

	// Select depends-on relation and advance to target step
	for i, rel := range l.relations {
		if rel.name == "depends-on" {
			l.relIndex = i
			break
		}
	}
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Type filter text
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	if l.filterText != "da" {
		t.Errorf("filterText = %q, want 'da'", l.filterText)
	}

	// Should filter to "Database" component
	if len(l.filteredList) != 1 {
		t.Errorf("filteredList count = %d, want 1", len(l.filteredList))
	}

	// Backspace to remove filter char
	l.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})
	if l.filterText != "d" {
		t.Errorf("after backspace, filterText = %q, want 'd'", l.filterText)
	}

	// More backspace to clear filter
	l.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})
	if l.filterText != "" {
		t.Errorf("after second backspace, filterText = %q, want empty", l.filterText)
	}
}

func TestLinkModel_UpdateTarget_BackToRelation(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "CMP-001")

	// Advance to target step
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Backspace with empty filter should go back
	l.Update(app, tea.KeyMsg{Type: tea.KeyBackspace})

	if l.step != LinkStepRelation {
		t.Errorf("step = %v, want %v", l.step, LinkStepRelation)
	}
}

func TestLinkModel_Help_RelationStep(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")
	l.step = LinkStepRelation

	help := l.Help()

	if len(help) == 0 {
		t.Fatal("Help should return items for relation step")
	}

	// Check for expected help items
	foundNavigate := false
	foundSelect := false
	foundCancel := false
	for _, item := range help {
		switch item[1] {
		case "navigate":
			foundNavigate = true
		case "select":
			foundSelect = true
		case "cancel":
			foundCancel = true
		}
	}

	if !foundNavigate {
		t.Error("Help should contain 'navigate'")
	}
	if !foundSelect {
		t.Error("Help should contain 'select'")
	}
	if !foundCancel {
		t.Error("Help should contain 'cancel'")
	}
}

func TestLinkModel_Help_TargetStep(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")
	l.step = LinkStepTarget

	help := l.Help()

	if len(help) == 0 {
		t.Fatal("Help should return items for target step")
	}

	// Check for expected help items
	foundNavigate := false
	foundFilter := false
	foundCreate := false
	foundBack := false
	for _, item := range help {
		switch item[1] {
		case "navigate":
			foundNavigate = true
		case "filter":
			foundFilter = true
		case "create":
			foundCreate = true
		case "back":
			foundBack = true
		}
	}

	if !foundNavigate {
		t.Error("Help should contain 'navigate'")
	}
	if !foundFilter {
		t.Error("Help should contain 'filter'")
	}
	if !foundCreate {
		t.Error("Help should contain 'create'")
	}
	if !foundBack {
		t.Error("Help should contain 'back'")
	}
}

func TestLinkModel_View_RelationStep(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	view := l.View(80, 40)

	// Should contain title with source ID
	if !contains(view, "DEC-001") {
		t.Error("View should contain source ID 'DEC-001'")
	}

	// Should contain step indicator
	if !contains(view, "Step 1") {
		t.Error("View should contain 'Step 1'")
	}

	// Should show relation labels
	if !contains(view, "Affects") {
		t.Error("View should contain relation label 'Affects'")
	}
	if !contains(view, "Implements") {
		t.Error("View should contain relation label 'Implements'")
	}

	// Should have selection marker
	if !contains(view, "►") {
		t.Error("View should contain '►' selection marker")
	}
}

func TestLinkModel_View_RelationStep_NoRelations(t *testing.T) {
	app := createLinkTestApp()

	// Use requirement which has no outgoing relations
	l := NewLinkModel(app, "REQ-001")

	view := l.View(80, 40)

	// Should contain message about no relations
	if !contains(view, "No relations available") {
		t.Error("View should contain 'No relations available'")
	}
}

func TestLinkModel_View_TargetStep(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Select "Implements" relation (second one, after "Affects")
	l.Update(app, tea.KeyMsg{Type: tea.KeyDown})
	// Advance to target step
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	view := l.View(80, 40)

	// Should contain step indicator
	if !contains(view, "Step 2") {
		t.Error("View should contain 'Step 2'")
	}

	// Should show target entity IDs (requirements)
	if !contains(view, "REQ-001") || !contains(view, "REQ-002") {
		t.Errorf("View should contain target entity IDs REQ-001 and REQ-002, got:\n%s", view)
	}

	// Should have selection marker
	if !contains(view, "►") {
		t.Error("View should contain '►' selection marker")
	}
}

func TestLinkModel_View_TargetStep_WithFilter(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Advance to target step
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Type filter
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	l.Update(app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	view := l.View(80, 40)

	// Should show filter text
	if !contains(view, "Filter:") {
		t.Error("View should contain 'Filter:'")
	}
	if !contains(view, "fi") {
		t.Error("View should contain filter text 'fi'")
	}
}

func TestLinkModel_View_TargetStep_NoMatches(t *testing.T) {
	app := createLinkTestApp()

	l := NewLinkModel(app, "DEC-001")

	// Advance to target step
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Type filter that matches nothing
	l.filterText = "zzzzz"
	l.applyFilter()

	view := l.View(80, 40)

	// Should show no matching targets message
	if !contains(view, "No matching targets") {
		t.Error("View should contain 'No matching targets'")
	}
}

func TestLinkModel_View_TargetStep_Scrolling(t *testing.T) {
	// Create app with many target entities
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"source": {Label: "Source", IDPrefix: "SRC-"},
			"target": {Label: "Target", IDPrefix: "TGT-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"links-to": {
				Label: "Links To",
				From:  []string{"source"},
				To:    []string{"target"},
			},
		},
	}

	g := graph.New()
	g.AddNode(&model.Entity{ID: "SRC-001", Type: "source", Properties: map[string]interface{}{"title": "Source"}})

	// Add many targets
	for i := 1; i <= 30; i++ {
		g.AddNode(&model.Entity{
			ID:         fmt.Sprintf("TGT-%03d", i),
			Type:       "target",
			Properties: map[string]interface{}{"title": "Target"},
		})
	}

	app := &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
	}

	l := NewLinkModel(app, "SRC-001")

	// Advance to target step
	l.Update(app, tea.KeyMsg{Type: tea.KeyEnter})

	// Move to middle of list
	l.targetIndex = 15

	// Render with small height
	view := l.View(80, 12)

	// Should show scroll indicator
	if !contains(view, "[16/30]") {
		t.Errorf("View should contain scroll indicator '[16/30]', got:\n%s", view)
	}
}
