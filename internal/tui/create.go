package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	mdmodel "github.com/Sourcehaven-BV/rela/internal/model"
)

// propertyInput represents a required property being filled in
type propertyInput struct {
	key   string // property key (e.g., "name", "title")
	label string // display label (e.g., "Name", "Title")
	value string // current input value
}

// CreateModel handles entity creation
type CreateModel struct {
	types           []typeChoice
	selectedIndex   int
	preselectedType string
	inputMode       bool

	// Multi-property input support
	requiredProps    []propertyInput // list of required properties to fill
	currentPropIndex int             // which property is currently being edited
	cursorPos        int             // cursor position within current input
}

type typeChoice struct {
	name  string
	label string
}

// NewCreateModel creates a new create screen
func NewCreateModel(app *App) *CreateModel {
	c := &CreateModel{}
	c.loadTypes(app)
	return c
}

func (c *CreateModel) loadTypes(app *App) {
	c.types = nil
	for typeName, def := range app.metamodel.Entities {
		c.types = append(c.types, typeChoice{
			name:  typeName,
			label: def.Label,
		})
	}
	sort.Slice(c.types, func(i, j int) bool {
		return c.types[i].label < c.types[j].label
	})

	// Preselect type if set
	if c.preselectedType != "" {
		for i, t := range c.types {
			if t.name == c.preselectedType {
				c.selectedIndex = i
				break
			}
		}
	}
}

// getRequiredProperties returns the list of required properties for the selected entity type
func (c *CreateModel) getRequiredProperties(app *App) []propertyInput {
	if c.selectedIndex >= len(c.types) {
		return nil
	}

	typeName := c.types[c.selectedIndex].name
	entityDef, ok := app.metamodel.GetEntityDef(typeName)
	if !ok {
		return nil
	}

	var props []propertyInput

	// Collect required properties
	for propKey, propDef := range entityDef.Properties {
		if propDef.Required {
			// Create a nice label from the property key
			caser := cases.Title(language.English)
			label := caser.String(propKey)
			props = append(props, propertyInput{
				key:   propKey,
				label: label,
				value: "",
			})
		}
	}

	// Sort properties alphabetically for consistent ordering
	// But prefer "title" or "name" to be first if present
	sort.Slice(props, func(i, j int) bool {
		// Prioritize common primary properties
		priority := map[string]int{"title": 0, "name": 1}
		pi, pok := priority[props[i].key]
		pj, jok := priority[props[j].key]
		if pok && jok {
			return pi < pj
		}
		if pok {
			return true
		}
		if jok {
			return false
		}
		return props[i].key < props[j].key
	})

	return props
}

// initInputMode initializes the input mode with required properties
func (c *CreateModel) initInputMode(app *App) {
	c.requiredProps = c.getRequiredProperties(app)
	c.currentPropIndex = 0
	c.cursorPos = 0

	// If no required properties found, add a default "title" property
	// This handles entity types that don't explicitly mark any property as required
	if len(c.requiredProps) == 0 {
		c.requiredProps = []propertyInput{
			{key: "title", label: "Title", value: ""},
		}
	}
}

// getCurrentInput returns the current input value being edited
func (c *CreateModel) getCurrentInput() string {
	if c.currentPropIndex < len(c.requiredProps) {
		return c.requiredProps[c.currentPropIndex].value
	}
	return ""
}

// setCurrentInput sets the current input value
func (c *CreateModel) setCurrentInput(value string) {
	if c.currentPropIndex < len(c.requiredProps) {
		c.requiredProps[c.currentPropIndex].value = value
	}
}

// Update handles key events
func (c *CreateModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if c.inputMode {
		return c.updateInput(app, msg)
	}
	return c.updateTypeSelect(app, msg)
}

func (c *CreateModel) updateTypeSelect(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if c.selectedIndex < len(c.types)-1 {
			c.selectedIndex++
		}
	case "k", "up":
		if c.selectedIndex > 0 {
			c.selectedIndex--
		}
	case "enter":
		c.inputMode = true
		c.initInputMode(app)
	case "g", "home":
		c.selectedIndex = 0
	case "G", "end":
		c.selectedIndex = len(c.types) - 1
	}
	return app, nil
}

func (c *CreateModel) updateInput(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	currentInput := c.getCurrentInput()

	switch msg.String() {
	case "enter":
		// Check if current input is non-empty
		if strings.TrimSpace(currentInput) != "" {
			// Move to next property or create entity
			if c.currentPropIndex < len(c.requiredProps)-1 {
				c.currentPropIndex++
				c.cursorPos = 0
			} else {
				// All properties filled, create entity
				return c.createEntity(app)
			}
		}
	case "esc":
		c.inputMode = false
		c.requiredProps = nil
		c.currentPropIndex = 0
	case "backspace":
		if c.cursorPos > 0 && currentInput != "" {
			newValue := currentInput[:c.cursorPos-1] + currentInput[c.cursorPos:]
			c.setCurrentInput(newValue)
			c.cursorPos--
		}
	case "left":
		if c.cursorPos > 0 {
			c.cursorPos--
		}
	case "right":
		if c.cursorPos < len(currentInput) {
			c.cursorPos++
		}
	case "tab":
		// Tab moves to next property (even if current is empty)
		if c.currentPropIndex < len(c.requiredProps)-1 {
			c.currentPropIndex++
			c.cursorPos = len(c.requiredProps[c.currentPropIndex].value)
		}
	case "shift+tab":
		// Shift+tab moves to previous property
		if c.currentPropIndex > 0 {
			c.currentPropIndex--
			c.cursorPos = len(c.requiredProps[c.currentPropIndex].value)
		}
	default:
		// Insert character - use KeyRunes type for proper character detection
		// This handles both regular keyboard input and automated testing (e.g., tmux send-keys)
		if msg.Type == tea.KeyRunes {
			chars := string(msg.Runes)
			newValue := currentInput[:c.cursorPos] + chars + currentInput[c.cursorPos:]
			c.setCurrentInput(newValue)
			c.cursorPos += len(chars)
		} else if msg.Type == tea.KeySpace {
			newValue := currentInput[:c.cursorPos] + " " + currentInput[c.cursorPos:]
			c.setCurrentInput(newValue)
			c.cursorPos++
		}
	}
	return app, nil
}

func (c *CreateModel) createEntity(app *App) (tea.Model, tea.Cmd) {
	if c.selectedIndex >= len(c.types) {
		return app, nil
	}

	typeName := c.types[c.selectedIndex].name
	entityDef, ok := app.metamodel.GetEntityDef(typeName)
	if !ok {
		return app, SetMessage("Unknown entity type", true)
	}

	// Generate ID
	if len(entityDef.IDPatterns) == 0 {
		return app, SetMessage("No ID patterns for type", true)
	}

	prefix := entityDef.IDPatterns[0]
	existingIDs := app.graph.AllIDs()
	entityID := mdmodel.GenerateNextID(existingIDs, prefix)

	// Create entity
	entity := mdmodel.NewEntity(entityID, typeName)

	// Load and apply template defaults first (if template exists)
	template, err := markdown.LoadEntityTemplate(app.project, typeName)
	if err == nil && template != nil {
		markdown.ApplyEntityTemplate(entity, template)
	}

	// Set all required properties from user input (overrides template)
	for _, prop := range c.requiredProps {
		entity.SetString(prop.key, prop.value)
	}

	// Set status if not already set by template or user input
	if entity.GetString("status") == "" {
		defaultStatus := entityDef.GetDefaultStatus(app.metamodel)
		entity.SetString("status", defaultStatus)
	}

	// Write file using proper plural from metamodel
	plural := entityDef.GetDirPlural(typeName)
	filePath := app.project.EntityFilePathWithPlural(plural, entityID)
	if err := markdown.WriteEntity(entity, filePath); err != nil {
		return app, SetMessage(fmt.Sprintf("Failed to create: %v", err), true)
	}

	// Add to graph
	entity.FilePath = filePath
	app.graph.AddNode(entity)

	// Save cache
	_ = app.graph.SaveCache(app.project.CachePath)

	// Refresh browser
	app.browser.Refresh(app)

	// Go back and show message
	return app, tea.Batch(
		PopScreen(),
		SetMessage(fmt.Sprintf("Created %s %s", typeName, entityID), false),
	)
}

// View renders the create screen
func (c *CreateModel) View(_, _ int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(50)

	inactiveInputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(50)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	activeLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	filledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("82"))

	if c.inputMode && len(c.requiredProps) > 0 {
		sb.WriteString(titleStyle.Render("Create " + c.types[c.selectedIndex].label))
		sb.WriteString("\n\n")

		// Show all required properties
		for i, prop := range c.requiredProps {
			isActive := i == c.currentPropIndex

			// Label
			if isActive {
				sb.WriteString(activeLabelStyle.Render(prop.label + ":"))
			} else {
				sb.WriteString(labelStyle.Render(prop.label + ":"))
			}
			sb.WriteString("\n")

			// Input field
			switch {
			case isActive:
				// Show input with cursor
				cursor := "_"
				displayText := prop.value[:c.cursorPos] + cursor + prop.value[c.cursorPos:]
				sb.WriteString(inputStyle.Render(displayText))
			case prop.value != "":
				// Show filled value
				sb.WriteString(inactiveInputStyle.Render(filledStyle.Render(prop.value)))
			default:
				// Show empty inactive input
				sb.WriteString(inactiveInputStyle.Render(""))
			}
			sb.WriteString("\n\n")
		}

		// Instructions
		if len(c.requiredProps) > 1 {
			sb.WriteString(labelStyle.Render("Press Enter to continue, Tab to switch fields, Esc to cancel"))
		} else {
			sb.WriteString(labelStyle.Render("Press Enter to create, Esc to cancel"))
		}
	} else {
		sb.WriteString(titleStyle.Render("Select Entity Type"))
		sb.WriteString("\n\n")

		for i, t := range c.types {
			marker := "  "
			style := normalStyle
			if i == c.selectedIndex {
				marker = "► "
				style = selectedStyle
			}
			sb.WriteString(fmt.Sprintf("%s%s\n", marker, style.Render(t.label)))
		}
	}

	return sb.String()
}

// Help returns help items
func (c *CreateModel) Help() [][2]string {
	if c.inputMode {
		if len(c.requiredProps) > 1 {
			return [][2]string{
				{"enter", "next/create"},
				{"tab", "switch field"},
				{"esc", "cancel"},
			}
		}
		return [][2]string{
			{"enter", "create"},
			{"esc", "cancel"},
		}
	}
	return [][2]string{
		{"up/down", "navigate"},
		{"enter", "select"},
		{"esc", "cancel"},
	}
}
