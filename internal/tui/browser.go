package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// BrowserLevel represents the navigation level in the browser
type BrowserLevel int

const (
	LevelTypes BrowserLevel = iota
	LevelEntities
)

// pageSize is the number of items to jump for PageUp/PageDown
const pageSize = 10

// BrowserModel is the main entity browser screen
type BrowserModel struct {
	level        BrowserLevel
	typeIndex    int
	entityIndex  int
	types        []typeItem
	entities     []*model.Entity
	selectedType string
}

type typeItem struct {
	name  string
	label string
	count int
}

// NewBrowserModel creates a new browser model
func NewBrowserModel(app *App) *BrowserModel {
	b := &BrowserModel{
		level: LevelTypes,
	}
	b.loadTypes(app)
	return b
}

func (b *BrowserModel) loadTypes(app *App) {
	b.types = nil
	for typeName, def := range app.metamodel.Entities {
		count := len(app.graph.NodesByType(typeName))
		b.types = append(b.types, typeItem{
			name:  typeName,
			label: def.Label,
			count: count,
		})
	}
	// Sort by label
	sort.Slice(b.types, func(i, j int) bool {
		return natsort.Less(b.types[i].label, b.types[j].label)
	})
}

func (b *BrowserModel) loadEntities(app *App, typeName string) {
	b.entities = app.graph.NodesByType(typeName)
	// Sort by ID
	sort.Slice(b.entities, func(i, j int) bool {
		return natsort.Less(b.entities[i].ID, b.entities[j].ID)
	})
	b.selectedType = typeName
}

// Update handles key events
func (b *BrowserModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch b.level {
	case LevelTypes:
		return b.updateTypes(app, msg)
	case LevelEntities:
		return b.updateEntities(app, msg)
	}
	return app, nil
}

func (b *BrowserModel) updateTypes(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if b.typeIndex < len(b.types)-1 {
			b.typeIndex++
		}
	case "k", "up":
		if b.typeIndex > 0 {
			b.typeIndex--
		}
	case "pgdown", "ctrl+d":
		b.typeIndex += pageSize
		if b.typeIndex > len(b.types)-1 {
			b.typeIndex = len(b.types) - 1
		}
	case "pgup", "ctrl+u":
		b.typeIndex -= pageSize
		if b.typeIndex < 0 {
			b.typeIndex = 0
		}
	case "enter":
		if b.typeIndex < len(b.types) {
			b.loadEntities(app, b.types[b.typeIndex].name)
			b.level = LevelEntities
			b.entityIndex = 0
		}
	case "c":
		app.create = NewCreateModel(app)
		return app, app.pushScreen(ScreenCreate)
	case "a", "A":
		app.analysis = NewAnalysisModel(app)
		return app, app.pushScreen(ScreenAnalysis)
	case "m", "M":
		app.meta = NewMetamodelModel(app)
		return app, app.pushScreen(ScreenMetamodel)
	case "t", "T":
		app.templates = NewTemplatesModel(app)
		return app, app.pushScreen(ScreenTemplates)
	case "g", "home":
		b.typeIndex = 0
	case "G", "end":
		b.typeIndex = len(b.types) - 1
	case "x":
		// Export all entities
		app.export = NewExportModelFromBrowser("")
		return app, app.pushScreen(ScreenExport)
	case "r", "R":
		// Refresh from disk
		if err := app.reloadFromDisk(); err != nil {
			return app, SetMessage("Refresh failed: "+err.Error(), true)
		}
		return app, SetMessage("Refreshed from disk", false)
	}
	return app, nil
}

func (b *BrowserModel) updateEntities(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if b.entityIndex < len(b.entities)-1 {
			b.entityIndex++
		}
	case "k", "up":
		if b.entityIndex > 0 {
			b.entityIndex--
		}
	case "pgdown", "ctrl+d":
		b.entityIndex += pageSize
		if b.entityIndex > len(b.entities)-1 {
			b.entityIndex = len(b.entities) - 1
		}
	case "pgup", "ctrl+u":
		b.entityIndex -= pageSize
		if b.entityIndex < 0 {
			b.entityIndex = 0
		}
	case "enter":
		// Browse all entities of this type with scope navigation
		return b.enterBrowseMode(app)
	case "backspace", "h", "left":
		b.level = LevelTypes
		b.entities = nil
	case "c":
		app.create = NewCreateModel(app)
		app.create.preselectedType = b.selectedType
		return app, app.pushScreen(ScreenCreate)
	case "l":
		if b.entityIndex < len(b.entities) {
			entity := b.entities[b.entityIndex]
			app.link = NewLinkModel(app, entity.ID)
			return app, app.pushScreen(ScreenLink)
		}
	case "g", "home":
		b.entityIndex = 0
	case "G", "end":
		b.entityIndex = len(b.entities) - 1
	case "t", "T":
		app.templates = NewTemplatesModel(app)
		return app, app.pushScreen(ScreenTemplates)
	case "x":
		// Export entities of current type
		app.export = NewExportModelFromBrowser(b.selectedType)
		return app, app.pushScreen(ScreenExport)
	case "r", "R":
		// Refresh from disk
		if err := app.reloadFromDisk(); err != nil {
			return app, SetMessage("Refresh failed: "+err.Error(), true)
		}
		return app, SetMessage("Refreshed from disk", false)
	}
	return app, nil
}

// enterBrowseMode creates a browse scope from the current entity list and enters detail view
func (b *BrowserModel) enterBrowseMode(app *App) (tea.Model, tea.Cmd) {
	if len(b.entities) == 0 {
		return app, nil
	}

	// Collect entity IDs
	ids := make([]string, len(b.entities))
	for i, entity := range b.entities {
		ids[i] = entity.ID
	}

	// Create scope label using the type label
	typeLabel := b.selectedType
	for _, t := range b.types {
		if t.name == b.selectedType {
			typeLabel = t.label
			break
		}
	}
	label := fmt.Sprintf("%d %ss", len(ids), typeLabel)

	// Create browse scope starting at the currently selected entity
	scope := NewBrowseScope(ids, label, ScreenBrowser)
	if scope == nil {
		return app, nil
	}

	// Set scope to currently selected item
	scope.SetIndex(b.entityIndex)

	// Create detail model with scope
	app.detail = NewDetailModelWithScope(app, scope)
	if app.detail == nil {
		return app, SetMessage("Failed to open entity", true)
	}

	return app, app.pushScreen(ScreenDetail)
}

// View renders the browser
func (b *BrowserModel) View(width, height int) string {
	switch b.level {
	case LevelTypes:
		return b.viewTypes(width, height)
	case LevelEntities:
		return b.viewEntities(width, height)
	}
	return ""
}

func (b *BrowserModel) viewTypes(_, height int) string {
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

	sb.WriteString(titleStyle.Render("Entity Types"))
	sb.WriteString("\n\n")

	// Calculate visible range for scrolling
	// Header takes ~3 lines (title + margin + blank), scroll indicator takes 2 lines
	headerLines := 3
	scrollIndicatorLines := 2
	needsScroll := len(b.types) > height-headerLines
	visibleCount := height - headerLines
	if needsScroll {
		visibleCount -= scrollIndicatorLines
	}
	if visibleCount < 1 {
		visibleCount = 1
	}

	startIdx := 0
	if b.typeIndex >= visibleCount {
		startIdx = b.typeIndex - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(b.types) {
		endIdx = len(b.types)
	}

	for i := startIdx; i < endIdx; i++ {
		t := b.types[i]
		marker := "  "
		style := normalStyle
		if i == b.typeIndex {
			marker = "► "
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%-20s", marker, t.label)
		count := countStyle.Render(fmt.Sprintf("(%d)", t.count))
		sb.WriteString(style.Render(line) + " " + count + "\n")
	}

	// Show scroll indicator
	if needsScroll {
		scrollInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf("\n[%d/%d]", b.typeIndex+1, len(b.types)))
		sb.WriteString(scrollInfo)
	}

	return sb.String()
}

func (b *BrowserModel) viewEntities(_, height int) string {
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

	idStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Find label for selected type
	typeLabel := b.selectedType
	for _, t := range b.types {
		if t.name == b.selectedType {
			typeLabel = t.label
			break
		}
	}

	sb.WriteString(titleStyle.Render(typeLabel + "s"))
	sb.WriteString("\n\n")

	if len(b.entities) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No entities found"))
		return sb.String()
	}

	// Calculate visible range for scrolling
	// Header takes ~3 lines (title + margin + blank), scroll indicator takes 2 lines
	headerLines := 3
	scrollIndicatorLines := 2
	needsScroll := len(b.entities) > height-headerLines
	visibleCount := height - headerLines
	if needsScroll {
		visibleCount -= scrollIndicatorLines
	}
	if visibleCount < 1 {
		visibleCount = 1
	}

	startIdx := 0
	if b.entityIndex >= visibleCount {
		startIdx = b.entityIndex - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(b.entities) {
		endIdx = len(b.entities)
	}

	for i := startIdx; i < endIdx; i++ {
		e := b.entities[i]
		marker := "  "
		style := normalStyle
		if i == b.entityIndex {
			marker = "► "
			style = selectedStyle
		}

		id := idStyle.Render(e.ID)
		if i == b.entityIndex {
			id = style.Render(e.ID)
		}

		title := e.Title()
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		status := statusStyle.Render(fmt.Sprintf("[%s]", e.GetString("status")))

		line := fmt.Sprintf("%s%-15s %s %s", marker, id, style.Render(title), status)
		sb.WriteString(line + "\n")
	}

	// Show scroll indicator
	if needsScroll {
		scrollInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf("\n[%d/%d]", b.entityIndex+1, len(b.entities)))
		sb.WriteString(scrollInfo)
	}

	return sb.String()
}

// Help returns the help items for this screen
func (b *BrowserModel) Help() [][2]string {
	switch b.level {
	case LevelTypes:
		return [][2]string{
			{"↑/↓", "navigate"},
			{"PgUp/PgDn", "page"},
			{"enter", "select"},
			{"c", "create"},
			{"/", "search"},
			{"a", "analyze"},
			{"r", "refresh"},
		}
	case LevelEntities:
		return [][2]string{
			{"↑/↓", "navigate"},
			{"PgUp/PgDn", "page"},
			{"enter", "browse"},
			{"←", "back"},
			{"c", "create"},
			{"l", "link"},
		}
	}
	return nil
}

// Refresh reloads the data
func (b *BrowserModel) Refresh(app *App) {
	b.loadTypes(app)
	if b.level == LevelEntities && b.selectedType != "" {
		b.loadEntities(app, b.selectedType)
	}
}
