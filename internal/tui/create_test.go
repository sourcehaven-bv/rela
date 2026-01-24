package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// TestCreateModel_GetRequiredProperties tests that the create wizard
// correctly identifies required properties from the metamodel.
// Issue #2: The TUI create wizard hardcodes "Title:" as the only input field,
// ignoring the actual required properties defined in the metamodel.
func TestCreateModel_GetRequiredProperties(t *testing.T) {
	// Create a metamodel with different required properties for different types
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"stakeholder": {
				Label:      "Stakeholder",
				IDPatterns: []string{"STK-"},
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
					"role": {Type: "string", Required: false},
				},
			},
			"asset": {
				Label:      "Asset",
				IDPatterns: []string{"AST-"},
				Properties: map[string]metamodel.PropertyDef{
					"name":        {Type: "string", Required: true},
					"description": {Type: "string", Required: true},
				},
			},
		},
	}

	tests := []struct {
		name             string
		entityType       string
		wantProperties   []string
		wantFirstPropKey string
	}{
		{
			name:             "requirement type requires title",
			entityType:       "requirement",
			wantProperties:   []string{"title"},
			wantFirstPropKey: "title",
		},
		{
			name:             "stakeholder type requires name (not title)",
			entityType:       "stakeholder",
			wantProperties:   []string{"name"},
			wantFirstPropKey: "name",
		},
		{
			name:             "asset type requires name and description",
			entityType:       "asset",
			wantProperties:   []string{"name", "description"},
			wantFirstPropKey: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				metamodel: meta,
				graph:     graph.New(),
				project:   &project.Context{Root: "/tmp/test"},
			}

			c := NewCreateModel(app)

			// Select the entity type
			for i, typ := range c.types {
				if typ.name == tt.entityType {
					c.selectedIndex = i
					break
				}
			}

			// Get required properties
			props := c.getRequiredProperties(app)

			// Check that we got the expected number of properties
			if len(props) != len(tt.wantProperties) {
				t.Errorf("getRequiredProperties() returned %d properties, want %d",
					len(props), len(tt.wantProperties))
			}

			// Check that the first property is correct
			if len(props) > 0 && props[0].key != tt.wantFirstPropKey {
				t.Errorf("first required property = %q, want %q",
					props[0].key, tt.wantFirstPropKey)
			}

			// Check that all expected properties are present
			propKeys := make(map[string]bool)
			for _, p := range props {
				propKeys[p.key] = true
			}
			for _, want := range tt.wantProperties {
				if !propKeys[want] {
					t.Errorf("expected property %q not found in required properties", want)
				}
			}
		})
	}
}

// TestCreateModel_InputCapture tests that text input is captured correctly.
// Issue #3: When creating entities via TUI, the title/name input is not
// captured correctly. The created entity has whitespace instead of entered text.
func TestCreateModel_InputCapture(t *testing.T) {
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

	app := &App{
		metamodel: meta,
		graph:     graph.New(),
		project:   &project.Context{Root: "/tmp/test"},
	}

	c := NewCreateModel(app)
	// Properly initialize input mode (this also sets up requiredProps)
	c.inputMode = true
	c.initInputMode(app)

	// Simulate typing "Hello World" using KeyRunes (the correct way)
	testInputs := []string{"H", "e", "l", "l", "o", " ", "W", "o", "r", "l", "d"}

	for _, char := range testInputs {
		var msg tea.KeyMsg
		if char == " " {
			msg = tea.KeyMsg{Type: tea.KeySpace}
		} else {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(char)}
		}
		c.Update(app, msg)
	}

	// Check that the input was captured correctly
	// Get the current input value
	input := c.getCurrentInput()
	if input != "Hello World" {
		t.Errorf("input capture failed: got %q, want %q", input, "Hello World")
	}
}

// TestCreateModel_ViewShowsCorrectLabel tests that the View function
// shows the correct property label from the metamodel, not hardcoded "Title:".
func TestCreateModel_ViewShowsCorrectLabel(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"stakeholder": {
				Label:      "Stakeholder",
				IDPatterns: []string{"STK-"},
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
	}

	app := &App{
		metamodel: meta,
		graph:     graph.New(),
		project:   &project.Context{Root: "/tmp/test"},
	}

	c := NewCreateModel(app)

	// Find and select stakeholder type
	for i, typ := range c.types {
		if typ.name == "stakeholder" {
			c.selectedIndex = i
			break
		}
	}

	// Enter input mode
	c.inputMode = true
	c.initInputMode(app)

	// Render the view
	view := c.View(80, 40)

	// The view should show "Name:" not "Title:"
	if !contains(view, "Name:") {
		t.Errorf("View should show 'Name:' for stakeholder type, but it doesn't")
	}

	// The view should NOT show "Title:" for stakeholder type
	// (unless there's a "Title" in the header like "Create Stakeholder")
	// We're specifically looking for the label of the input field
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
