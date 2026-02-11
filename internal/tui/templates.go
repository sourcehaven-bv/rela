package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// TemplatesDir is the directory name for templates
const TemplatesDir = "templates"

// TemplateInfo holds information about a template
type TemplateInfo struct {
	Name       string // Template name (filename without .md)
	EntityType string // Entity type this template is for
	FilePath   string // Full path to template file
}

// TemplatesModel handles template management
type TemplatesModel struct {
	templates     []TemplateInfo
	entityTypes   []string
	typeIndex     int
	templateIndex int
	level         TemplateLevel
	selectedType  string

	// Delete confirmation state
	confirmDelete bool

	// Create mode state
	createMode      bool
	createInput     string
	createCursorPos int
}

// TemplateLevel represents the navigation level
type TemplateLevel int

const (
	TemplateLevelTypes TemplateLevel = iota
	TemplateLevelTemplates
)

// NewTemplatesModel creates a new templates screen
func NewTemplatesModel(app *App) *TemplatesModel {
	t := &TemplatesModel{
		level: TemplateLevelTypes,
	}
	t.load(app)
	return t
}

func (t *TemplatesModel) load(app *App) {
	templatesDir := filepath.Join(app.project.Root, TemplatesDir)

	// Get all entity types from metamodel
	t.entityTypes = nil
	for typeName := range app.metamodel.Entities {
		t.entityTypes = append(t.entityTypes, typeName)
	}
	natsort.Strings(t.entityTypes)

	// Load templates
	t.templates = nil

	// Check if templates directory exists
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return
	}

	// Walk through templates directory
	_ = filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // propagate error to stop walk on access issues
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Determine entity type from parent directory or filename
		relPath, _ := filepath.Rel(templatesDir, path)
		parts := strings.Split(relPath, string(filepath.Separator))

		var entityType, name string
		if len(parts) >= 2 {
			// Template is in a subdirectory: templates/{entityType}/{name}.md
			entityType = parts[0]
			name = strings.TrimSuffix(parts[len(parts)-1], ".md")
		} else {
			// Template is at root: templates/{entityType}.md (entity type = name)
			name = strings.TrimSuffix(info.Name(), ".md")
			entityType = name
		}

		t.templates = append(t.templates, TemplateInfo{
			Name:       name,
			EntityType: entityType,
			FilePath:   path,
		})

		return nil
	})

	// Sort templates by entity type then name
	sort.Slice(t.templates, func(i, j int) bool {
		if t.templates[i].EntityType != t.templates[j].EntityType {
			return natsort.Less(t.templates[i].EntityType, t.templates[j].EntityType)
		}
		return natsort.Less(t.templates[i].Name, t.templates[j].Name)
	})
}

func (t *TemplatesModel) templatesForType(entityType string) []TemplateInfo {
	var result []TemplateInfo
	for _, tmpl := range t.templates {
		if tmpl.EntityType == entityType {
			result = append(result, tmpl)
		}
	}
	return result
}

func (t *TemplatesModel) templateCountForType(entityType string) int {
	count := 0
	for _, tmpl := range t.templates {
		if tmpl.EntityType == entityType {
			count++
		}
	}
	return count
}

// Update handles key events
func (t *TemplatesModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle delete confirmation
	if t.confirmDelete {
		switch msg.String() {
		case "y", "Y":
			t.confirmDelete = false
			return t.deleteSelectedTemplate(app)
		case "n", "N", "esc":
			t.confirmDelete = false
			return app, nil
		}
		return app, nil
	}

	// Handle create mode
	if t.createMode {
		return t.updateCreate(app, msg)
	}

	switch t.level {
	case TemplateLevelTypes:
		return t.updateTypes(app, msg)
	case TemplateLevelTemplates:
		return t.updateTemplates(app, msg)
	}
	return app, nil
}

func (t *TemplatesModel) updateTypes(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if t.typeIndex < len(t.entityTypes)-1 {
			t.typeIndex++
		}
	case "k", "up":
		if t.typeIndex > 0 {
			t.typeIndex--
		}
	case "enter":
		if t.typeIndex < len(t.entityTypes) {
			t.selectedType = t.entityTypes[t.typeIndex]
			t.level = TemplateLevelTemplates
			t.templateIndex = 0
		}
	case "c":
		// Create template for current type
		if t.typeIndex < len(t.entityTypes) {
			t.selectedType = t.entityTypes[t.typeIndex]
			t.createMode = true
			t.createInput = ""
			t.createCursorPos = 0
		}
	case "g", "home":
		t.typeIndex = 0
	case "G", "end":
		t.typeIndex = len(t.entityTypes) - 1
	}
	return app, nil
}

func (t *TemplatesModel) updateTemplates(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	templates := t.templatesForType(t.selectedType)

	switch msg.String() {
	case "j", "down":
		if t.templateIndex < len(templates)-1 {
			t.templateIndex++
		}
	case "k", "up":
		if t.templateIndex > 0 {
			t.templateIndex--
		}
	case "backspace", "h", "left":
		t.level = TemplateLevelTypes
		t.templateIndex = 0
	case "e", "enter":
		// Edit template
		if t.templateIndex < len(templates) {
			return app, t.editTemplate(templates[t.templateIndex])
		}
	case "c":
		// Create new template
		t.createMode = true
		t.createInput = ""
		t.createCursorPos = 0
	case "d":
		// Delete template
		if t.templateIndex < len(templates) {
			t.confirmDelete = true
		}
	case "g", "home":
		t.templateIndex = 0
	case "G", "end":
		t.templateIndex = len(templates) - 1
	}
	return app, nil
}

func (t *TemplatesModel) updateCreate(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if t.createInput != "" {
			return t.createTemplate(app)
		}
	case "esc":
		t.createMode = false
		t.createInput = ""
	case "backspace":
		if t.createCursorPos > 0 && t.createInput != "" {
			t.createInput = t.createInput[:t.createCursorPos-1] + t.createInput[t.createCursorPos:]
			t.createCursorPos--
		}
	case "left":
		if t.createCursorPos > 0 {
			t.createCursorPos--
		}
	case "right":
		if t.createCursorPos < len(t.createInput) {
			t.createCursorPos++
		}
	default:
		// Insert character
		if len(msg.String()) == 1 {
			t.createInput = t.createInput[:t.createCursorPos] + msg.String() + t.createInput[t.createCursorPos:]
			t.createCursorPos++
		} else if msg.Type == tea.KeySpace {
			t.createInput = t.createInput[:t.createCursorPos] + " " + t.createInput[t.createCursorPos:]
			t.createCursorPos++
		}
	}
	return app, nil
}

// templateEditorFinishedMsg is sent when template editor closes
type templateEditorFinishedMsg struct {
	err error
}

func (t *TemplatesModel) editTemplate(tmpl TemplateInfo) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, tmpl.FilePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return templateEditorFinishedMsg{err: err}
	})
}

func (t *TemplatesModel) createTemplate(app *App) (tea.Model, tea.Cmd) {
	templatesDir := filepath.Join(app.project.Root, TemplatesDir)
	typeDir := filepath.Join(templatesDir, t.selectedType)

	// Ensure directory exists
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return app, SetMessage(fmt.Sprintf("Failed to create directory: %v", err), true)
	}

	// Create template file
	filename := t.createInput
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	filePath := filepath.Join(typeDir, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return app, SetMessage("Template already exists", true)
	}

	// Get entity definition for default template
	entityDef, ok := app.metamodel.GetEntityDef(t.selectedType)

	// Create default template content
	var content strings.Builder
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("# Template for %s\n", t.selectedType))
	content.WriteString("# Variables: {{id}}, {{title}}, {{date}}, {{type}}\n")
	content.WriteString("---\n\n")
	content.WriteString("# {{title}}\n\n")

	// Add default properties based on entity definition
	if ok && entityDef.Properties != nil {
		content.WriteString("## Properties\n\n")
		for propName := range entityDef.Properties {
			if propName != "title" && propName != "id" && propName != "type" {
				content.WriteString(fmt.Sprintf("- %s: \n", propName))
			}
		}
		content.WriteString("\n")
	}

	content.WriteString("## Description\n\n")
	content.WriteString("_Add description here._\n")

	if err := os.WriteFile(filePath, []byte(content.String()), 0644); err != nil {
		return app, SetMessage(fmt.Sprintf("Failed to create template: %v", err), true)
	}

	// Reset create mode
	t.createMode = false
	t.createInput = ""

	// Reload templates
	t.load(app)

	// Switch to templates level for this type
	t.level = TemplateLevelTemplates

	// Open editor for the new template
	return app, tea.Batch(
		SetMessage(fmt.Sprintf("Created template %s", filename), false),
		t.editTemplate(TemplateInfo{
			Name:       strings.TrimSuffix(filename, ".md"),
			EntityType: t.selectedType,
			FilePath:   filePath,
		}),
	)
}

func (t *TemplatesModel) deleteSelectedTemplate(app *App) (tea.Model, tea.Cmd) {
	templates := t.templatesForType(t.selectedType)
	if t.templateIndex >= len(templates) {
		return app, nil
	}

	tmpl := templates[t.templateIndex]
	if err := os.Remove(tmpl.FilePath); err != nil {
		return app, SetMessage(fmt.Sprintf("Failed to delete: %v", err), true)
	}

	// Reload templates
	t.load(app)

	// Adjust index if needed
	newTemplates := t.templatesForType(t.selectedType)
	if t.templateIndex >= len(newTemplates) {
		t.templateIndex = len(newTemplates) - 1
		if t.templateIndex < 0 {
			t.templateIndex = 0
		}
	}

	return app, SetMessage(fmt.Sprintf("Deleted template %s", tmpl.Name), false)
}

// View renders the templates screen
func (t *TemplatesModel) View(_, height int) string {
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

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(50)

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220")).
		Bold(true)

	// Handle delete confirmation
	if t.confirmDelete {
		templates := t.templatesForType(t.selectedType)
		if t.templateIndex < len(templates) {
			tmpl := templates[t.templateIndex]
			sb.WriteString(warningStyle.Render("Delete Template?"))
			sb.WriteString("\n\n")
			sb.WriteString(fmt.Sprintf("Are you sure you want to delete '%s'?\n\n", tmpl.Name))
			sb.WriteString(labelStyle.Render("Press [y] to confirm, [n] or [esc] to cancel"))
			return sb.String()
		}
	}

	// Handle create mode
	if t.createMode {
		sb.WriteString(titleStyle.Render(fmt.Sprintf("Create Template for %s", t.selectedType)))
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Template Name:"))
		sb.WriteString("\n")

		// Show input with cursor
		cursor := "_"
		displayText := t.createInput[:t.createCursorPos] + cursor + t.createInput[t.createCursorPos:]
		sb.WriteString(inputStyle.Render(displayText))
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Press Enter to create, Esc to cancel"))
		return sb.String()
	}

	switch t.level {
	case TemplateLevelTypes:
		sb.WriteString(titleStyle.Render("Templates by Entity Type"))
		sb.WriteString("\n\n")

		if len(t.entityTypes) == 0 {
			sb.WriteString(labelStyle.Render("No entity types defined in metamodel"))
			return sb.String()
		}

		for i, entityType := range t.entityTypes {
			marker := "  "
			style := normalStyle
			if i == t.typeIndex {
				marker = "> "
				style = selectedStyle
			}

			count := t.templateCountForType(entityType)
			countText := countStyle.Render(fmt.Sprintf("(%d templates)", count))

			sb.WriteString(fmt.Sprintf("%s%s %s\n", marker, style.Render(entityType), countText))
		}

	case TemplateLevelTemplates:
		sb.WriteString(titleStyle.Render(fmt.Sprintf("Templates for %s", t.selectedType)))
		sb.WriteString("\n\n")

		templates := t.templatesForType(t.selectedType)

		if len(templates) == 0 {
			sb.WriteString(labelStyle.Render("No templates for this type"))
			sb.WriteString("\n\n")
			sb.WriteString(labelStyle.Render("Press [c] to create a new template"))
			return sb.String()
		}

		// Calculate visible range for scrolling
		visibleCount := height - 4
		if visibleCount < 1 {
			visibleCount = 1
		}

		startIdx := 0
		if t.templateIndex >= visibleCount {
			startIdx = t.templateIndex - visibleCount + 1
		}
		endIdx := startIdx + visibleCount
		if endIdx > len(templates) {
			endIdx = len(templates)
		}

		for i := startIdx; i < endIdx; i++ {
			tmpl := templates[i]
			marker := "  "
			style := normalStyle
			if i == t.templateIndex {
				marker = "> "
				style = selectedStyle
			}

			sb.WriteString(fmt.Sprintf("%s%s\n", marker, style.Render(tmpl.Name)))
		}

		// Show scroll indicator
		if len(templates) > visibleCount {
			scrollInfo := countStyle.Render(fmt.Sprintf("\n[%d/%d]", t.templateIndex+1, len(templates)))
			sb.WriteString(scrollInfo)
		}
	}

	return sb.String()
}

// Help returns help items
func (t *TemplatesModel) Help() [][2]string {
	if t.createMode {
		return [][2]string{
			{"enter", "create"},
			{"esc", "cancel"},
		}
	}

	if t.confirmDelete {
		return [][2]string{
			{"y", "confirm"},
			{"n/esc", "cancel"},
		}
	}

	switch t.level {
	case TemplateLevelTypes:
		return [][2]string{
			{"up/dn", "navigate"},
			{"enter", "select"},
			{"c", "create"},
		}
	case TemplateLevelTemplates:
		return [][2]string{
			{"up/dn", "navigate"},
			{"e", "edit"},
			{"c", "create"},
			{"d", "delete"},
			{"<-", "back"},
		}
	}
	return nil
}
