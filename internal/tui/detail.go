package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// DetailModel shows entity details
type DetailModel struct {
	entityID  string
	entity    *model.Entity
	incoming  []*model.Relation
	outgoing  []*model.Relation
	scrollPos int
	relMode   bool // Toggle between scroll mode and relation navigation
	relIndex  int
	allRels   []*model.Relation

	// Delete confirmation state
	confirmDelete       bool
	confirmDeleteEntity bool // true = delete entity, false = delete relation
}

// NewDetailModel creates a new detail view
func NewDetailModel(app *App, entityID string) *DetailModel {
	d := &DetailModel{
		entityID: entityID,
	}
	d.load(app)
	return d
}

func (d *DetailModel) load(app *App) {
	entity, ok := app.graph.GetNode(d.entityID)
	if !ok {
		return
	}
	d.entity = entity
	d.incoming = app.graph.IncomingEdges(d.entityID)
	d.outgoing = app.graph.OutgoingEdges(d.entityID)

	// Combine relations for navigation
	d.allRels = append(d.allRels, d.incoming...)
	d.allRels = append(d.allRels, d.outgoing...)
}

// Update handles key events
func (d *DetailModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirmation dialog
	if d.confirmDelete {
		switch msg.String() {
		case "y", "Y":
			d.confirmDelete = false
			if d.confirmDeleteEntity {
				return d.deleteEntity(app)
			}
			return d.deleteRelation(app)
		case "n", "N", "esc":
			d.confirmDelete = false
			return app, SetMessage("Delete cancelled", false)
		default:
			// Any other key cancels (default is No)
			d.confirmDelete = false
			return app, SetMessage("Delete cancelled", false)
		}
	}

	switch msg.String() {
	case "j", "down":
		if d.relMode {
			if d.relIndex < len(d.allRels)-1 {
				d.relIndex++
			}
		} else {
			d.scrollPos++
		}
	case "k", "up":
		if d.relMode {
			if d.relIndex > 0 {
				d.relIndex--
			}
		} else {
			if d.scrollPos > 0 {
				d.scrollPos--
			}
		}
	case "tab":
		d.relMode = !d.relMode
		d.relIndex = 0
	case "enter":
		if d.relMode && len(d.allRels) > 0 {
			rel := d.allRels[d.relIndex]
			targetID := rel.To
			if rel.To == d.entityID {
				targetID = rel.From
			}
			app.detail = NewDetailModel(app, targetID)
			return app, app.pushScreen(ScreenDetail)
		}
	case "l":
		app.link = NewLinkModel(app, d.entityID)
		return app, app.pushScreen(ScreenLink)
	case "g":
		app.graphView = NewGraphViewModel(app, d.entityID)
		return app, app.pushScreen(ScreenGraph)
	case "e":
		// Edit entity
		if d.entity != nil && d.entity.FilePath != "" {
			return app, d.editEntity(app)
		}
	case "E":
		// Edit selected relation
		if d.relMode && len(d.allRels) > 0 {
			rel := d.allRels[d.relIndex]
			if rel.FilePath != "" {
				return app, d.editRelation(app, rel)
			}
		}
	case "d":
		// Delete/unlink selected relation (only in relation mode)
		if d.relMode && len(d.allRels) > 0 {
			d.confirmDelete = true
			d.confirmDeleteEntity = false
		}
	case "D":
		// Delete entity
		if d.entity != nil {
			d.confirmDelete = true
			d.confirmDeleteEntity = true
		}
	}
	return app, nil
}

// editorFinishedMsg is sent when the editor closes
type editorFinishedMsg struct {
	err        error
	entityID   string
	isRelation bool
	relIndex   int
}

// editEntity launches the editor for the current entity
func (d *DetailModel) editEntity(_ *App) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, d.entity.FilePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, entityID: d.entityID, isRelation: false}
	})
}

// editRelation launches the editor for the selected relation
func (d *DetailModel) editRelation(_ *App, rel *model.Relation) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, rel.FilePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, entityID: d.entityID, isRelation: true, relIndex: d.relIndex}
	})
}

// reloadEntity reloads the entity from disk and updates the graph
func (d *DetailModel) reloadEntity(app *App) error {
	if d.entity == nil || d.entity.FilePath == "" {
		return nil
	}

	entity, err := markdown.ReadEntity(d.entity.FilePath, app.metamodel)
	if err != nil {
		return err
	}

	// Update in graph
	app.graph.AddNode(entity)

	// Reload detail model
	d.entity = entity
	d.allRels = nil
	d.incoming = app.graph.IncomingEdges(d.entityID)
	d.outgoing = app.graph.OutgoingEdges(d.entityID)
	d.allRels = append(d.allRels, d.incoming...)
	d.allRels = append(d.allRels, d.outgoing...)

	// Save cache
	_ = app.graph.SaveCache(app.project.CachePath)

	// Refresh browser
	app.browser.Refresh(app)

	return nil
}

// reloadRelation reloads the relation from disk and updates the graph
func (d *DetailModel) reloadRelation(app *App, oldRel *model.Relation) error {
	if oldRel == nil || oldRel.FilePath == "" {
		return nil
	}

	newRel, err := markdown.ReadRelation(oldRel.FilePath)
	if err != nil {
		return err
	}

	// Remove old relation and add new one
	app.graph.RemoveEdge(oldRel.From, oldRel.Type, oldRel.To)
	app.graph.AddEdge(newRel)

	// Reload relations for this entity
	d.allRels = nil
	d.incoming = app.graph.IncomingEdges(d.entityID)
	d.outgoing = app.graph.OutgoingEdges(d.entityID)
	d.allRels = append(d.allRels, d.incoming...)
	d.allRels = append(d.allRels, d.outgoing...)

	// Clamp relation index if needed
	if d.relIndex >= len(d.allRels) {
		d.relIndex = len(d.allRels) - 1
		if d.relIndex < 0 {
			d.relIndex = 0
		}
	}

	// Save cache
	_ = app.graph.SaveCache(app.project.CachePath)

	return nil
}

// deleteRelation deletes the currently selected relation
func (d *DetailModel) deleteRelation(app *App) (tea.Model, tea.Cmd) {
	if d.relIndex >= len(d.allRels) {
		return app, SetMessage("No relation selected", true)
	}

	rel := d.allRels[d.relIndex]

	// Delete the relation file
	if rel.FilePath != "" {
		if err := markdown.DeleteRelation(rel.FilePath); err != nil {
			return app, SetMessage(fmt.Sprintf("Failed to delete relation: %v", err), true)
		}
	}

	// Remove from graph
	app.graph.RemoveEdge(rel.From, rel.Type, rel.To)

	// Save cache
	_ = app.graph.SaveCache(app.project.CachePath)

	// Reload relations for this entity
	d.allRels = nil
	d.incoming = app.graph.IncomingEdges(d.entityID)
	d.outgoing = app.graph.OutgoingEdges(d.entityID)
	d.allRels = append(d.allRels, d.incoming...)
	d.allRels = append(d.allRels, d.outgoing...)

	// Clamp relation index if needed
	if d.relIndex >= len(d.allRels) {
		d.relIndex = len(d.allRels) - 1
		if d.relIndex < 0 {
			d.relIndex = 0
		}
	}

	// If no more relations, exit relation mode
	if len(d.allRels) == 0 {
		d.relMode = false
	}

	// Refresh browser
	app.browser.Refresh(app)

	return app, SetMessage(fmt.Sprintf("Deleted relation: %s --%s--> %s", rel.From, rel.Type, rel.To), false)
}

// deleteEntity deletes the current entity and its relations
func (d *DetailModel) deleteEntity(app *App) (tea.Model, tea.Cmd) {
	if d.entity == nil {
		return app, SetMessage("No entity to delete", true)
	}

	entityID := d.entityID
	totalRelations := len(d.incoming) + len(d.outgoing)

	// Delete all relations first (cascade)
	for _, rel := range d.incoming {
		if rel.FilePath != "" {
			// Ignore errors - file may already be deleted
			_ = markdown.DeleteRelation(rel.FilePath)
		}
		app.graph.RemoveEdge(rel.From, rel.Type, rel.To)
	}
	for _, rel := range d.outgoing {
		if rel.FilePath != "" {
			// Ignore errors - file may already be deleted
			_ = markdown.DeleteRelation(rel.FilePath)
		}
		app.graph.RemoveEdge(rel.From, rel.Type, rel.To)
	}

	// Delete entity file
	filePath := d.entity.FilePath
	if filePath != "" {
		if err := markdown.DeleteEntity(filePath); err != nil && !os.IsNotExist(err) {
			return app, SetMessage(fmt.Sprintf("Failed to delete entity file: %v", err), true)
		}
	}

	// Remove from graph
	app.graph.RemoveNode(entityID)

	// Save cache
	_ = app.graph.SaveCache(app.project.CachePath)

	// Refresh browser
	app.browser.Refresh(app)

	// Build success message
	msg := fmt.Sprintf("Deleted %s", entityID)
	if totalRelations > 0 {
		msg += fmt.Sprintf(" and %d relation(s)", totalRelations)
	}

	// Pop back to browser/previous screen
	return app, tea.Batch(
		PopScreen(),
		SetMessage(msg, false),
	)
}

// View renders the detail screen
func (d *DetailModel) View(_, height int) string {
	if d.entity == nil {
		return "Entity not found"
	}

	// Show confirmation dialog if active
	if d.confirmDelete {
		confirmStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

		var confirmMsg string
		if d.confirmDeleteEntity {
			confirmMsg = fmt.Sprintf("Delete entity %s?", d.entityID)
			totalRels := len(d.incoming) + len(d.outgoing)
			if totalRels > 0 {
				confirmMsg += fmt.Sprintf(" (and %d relation(s))", totalRels)
			}
		} else if d.relIndex < len(d.allRels) {
			rel := d.allRels[d.relIndex]
			confirmMsg = fmt.Sprintf("Delete relation %s --%s--> %s?", rel.From, rel.Type, rel.To)
		}
		confirmMsg += " (y/N)"

		return confirmStyle.Render(confirmMsg)
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	idStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("220"))

	selectedRelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	// Build all lines first
	lines := []string{
		idStyle.Render(d.entity.ID) + labelStyle.Render(fmt.Sprintf(" (%s)", d.entity.Type)),
		headerStyle.Render(d.entity.Title()),
		"",
	}

	// Properties
	lines = append(lines, sectionStyle.Render("Properties"))

	if status := d.entity.GetString("status"); status != "" {
		lines = append(lines, labelStyle.Render("  Status: ")+valueStyle.Render(status))
	}

	if priority := d.entity.GetString("priority"); priority != "" {
		lines = append(lines, labelStyle.Render("  Priority: ")+valueStyle.Render(priority))
	}

	if desc := d.entity.Description(); desc != "" {
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lines = append(lines, labelStyle.Render("  Description: ")+valueStyle.Render(desc))
	}

	// Other properties
	for key, val := range d.entity.Properties {
		if key != "title" && key != "status" && key != "priority" && key != "description" {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  %s: ", key))+valueStyle.Render(fmt.Sprintf("%v", val)))
		}
	}

	// Relations
	if len(d.incoming) > 0 || len(d.outgoing) > 0 {
		lines = append(lines, "")
		relHeader := sectionStyle.Render("Relations")
		if d.relMode {
			relHeader += labelStyle.Render(" (navigation mode - Tab to exit)")
		} else {
			relHeader += labelStyle.Render(" (Tab to navigate)")
		}
		lines = append(lines, relHeader)

		relIdx := 0

		// Incoming relations
		if len(d.incoming) > 0 {
			lines = append(lines, labelStyle.Render("  Incoming:"))
			for _, rel := range d.incoming {
				marker := "    "
				style := valueStyle
				if d.relMode && relIdx == d.relIndex {
					marker = "  ► "
					style = selectedRelStyle
				}
				line := fmt.Sprintf("%s← %s %s %s",
					marker,
					rel.From,
					labelStyle.Render(rel.Type),
					d.entityID)
				lines = append(lines, style.Render(line))
				relIdx++
			}
		}

		// Outgoing relations
		if len(d.outgoing) > 0 {
			lines = append(lines, labelStyle.Render("  Outgoing:"))
			for _, rel := range d.outgoing {
				marker := "    "
				style := valueStyle
				if d.relMode && relIdx == d.relIndex {
					marker = "  ► "
					style = selectedRelStyle
				}
				line := fmt.Sprintf("%s→ %s %s %s",
					marker,
					d.entityID,
					labelStyle.Render(rel.Type),
					rel.To)
				lines = append(lines, style.Render(line))
				relIdx++
			}
		}
	}

	// Content
	if d.entity.Content != "" {
		lines = append(lines, "", sectionStyle.Render("Content"))
		content := d.entity.Content
		// Split content into lines
		contentLines := strings.Split(content, "\n")
		for _, cl := range contentLines {
			lines = append(lines, valueStyle.Render("  "+cl))
		}
	}

	// Apply scrolling
	visibleCount := height - 2
	if visibleCount < 5 {
		visibleCount = 5
	}

	// Clamp scrollPos
	maxScroll := len(lines) - visibleCount
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.scrollPos > maxScroll {
		d.scrollPos = maxScroll
	}

	endIdx := d.scrollPos + visibleCount
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	var sb strings.Builder
	for i := d.scrollPos; i < endIdx; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}

	// Show scroll indicator if needed
	if len(lines) > visibleCount {
		sb.WriteString(labelStyle.Render(fmt.Sprintf("\n[%d-%d of %d] ↑/↓ scroll", d.scrollPos+1, endIdx, len(lines))))
	}

	return sb.String()
}

// Help returns help items
func (d *DetailModel) Help() [][2]string {
	if d.confirmDelete {
		return [][2]string{
			{"y", "confirm"},
			{"n/esc", "cancel"},
		}
	}
	if d.relMode {
		return [][2]string{
			{"↑/↓", "navigate"},
			{"tab", "scroll mode"},
			{"enter", "follow"},
			{"e", "edit entity"},
			{"E", "edit relation"},
			{"d", "delete relation"},
			{"D", "delete entity"},
			{"l", "link"},
			{"g", "graph"},
		}
	}
	return [][2]string{
		{"↑/↓", "scroll"},
		{"tab", "rel mode"},
		{"e", "edit"},
		{"D", "delete"},
		{"l", "link"},
		{"g", "graph"},
	}
}
