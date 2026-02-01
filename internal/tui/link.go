package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// LinkStep represents the current step in link creation
type LinkStep int

const (
	LinkStepRelation LinkStep = iota
	LinkStepTarget
)

// LinkModel handles link creation wizard
type LinkModel struct {
	sourceID     string
	sourceType   string
	step         LinkStep
	relations    []relationChoice
	relIndex     int
	selectedRel  string
	targets      []*model.Entity
	targetIndex  int
	filterText   string
	filteredList []*model.Entity
}

type relationChoice struct {
	name        string
	label       string
	targetTypes []string
}

// NewLinkModel creates a new link wizard
func NewLinkModel(app *App, sourceID string) *LinkModel {
	l := &LinkModel{
		sourceID: sourceID,
		step:     LinkStepRelation,
	}

	// Get source entity type
	if entity, ok := app.graph.GetNode(sourceID); ok {
		l.sourceType = entity.Type
	}

	l.loadRelations(app)
	return l
}

func (l *LinkModel) loadRelations(app *App) {
	l.relations = nil

	// Find relations where source type is in From
	for relName, relDef := range app.metamodel.Relations {
		for _, fromType := range relDef.From {
			if fromType == l.sourceType {
				l.relations = append(l.relations, relationChoice{
					name:        relName,
					label:       relDef.Label,
					targetTypes: relDef.To,
				})
				break
			}
		}
	}

	sort.Slice(l.relations, func(i, j int) bool {
		return l.relations[i].label < l.relations[j].label
	})
}

func (l *LinkModel) loadTargets(app *App) {
	l.targets = nil

	if l.relIndex >= len(l.relations) {
		return
	}

	rel := l.relations[l.relIndex]

	// Get all entities of allowed target types
	for _, targetType := range rel.targetTypes {
		entities := app.graph.NodesByType(targetType)
		for _, e := range entities {
			// Exclude self
			if e.ID != l.sourceID {
				l.targets = append(l.targets, e)
			}
		}
	}

	sort.Slice(l.targets, func(i, j int) bool {
		return l.targets[i].ID < l.targets[j].ID
	})

	l.filteredList = l.targets
}

// Update handles key events
func (l *LinkModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch l.step {
	case LinkStepRelation:
		return l.updateRelation(app, msg)
	case LinkStepTarget:
		return l.updateTarget(app, msg)
	}
	return app, nil
}

func (l *LinkModel) updateRelation(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if l.relIndex < len(l.relations)-1 {
			l.relIndex++
		}
	case "k", "up":
		if l.relIndex > 0 {
			l.relIndex--
		}
	case "enter":
		if l.relIndex < len(l.relations) {
			l.selectedRel = l.relations[l.relIndex].name
			l.loadTargets(app)
			l.step = LinkStepTarget
			l.targetIndex = 0
			l.filterText = ""
		}
	}
	return app, nil
}

func (l *LinkModel) updateTarget(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if l.targetIndex < len(l.filteredList)-1 {
			l.targetIndex++
		}
	case "k", "up":
		if l.targetIndex > 0 {
			l.targetIndex--
		}
	case "enter":
		if l.targetIndex < len(l.filteredList) {
			return l.createLink(app)
		}
	case "backspace":
		if l.filterText != "" {
			l.filterText = l.filterText[:len(l.filterText)-1]
			l.applyFilter()
		} else {
			l.step = LinkStepRelation
		}
	default:
		// Filter by typing
		if len(msg.String()) == 1 {
			l.filterText += msg.String()
			l.applyFilter()
		}
	}
	return app, nil
}

func (l *LinkModel) applyFilter() {
	if l.filterText == "" {
		l.filteredList = l.targets
		return
	}

	filterLower := strings.ToLower(l.filterText)
	l.filteredList = nil

	for _, e := range l.targets {
		idMatch := strings.Contains(strings.ToLower(e.ID), filterLower)
		titleMatch := strings.Contains(strings.ToLower(e.Title()), filterLower)
		if idMatch || titleMatch {
			l.filteredList = append(l.filteredList, e)
		}
	}

	l.targetIndex = 0
}

func (l *LinkModel) createLink(app *App) (tea.Model, tea.Cmd) {
	if l.targetIndex >= len(l.filteredList) {
		return app, nil
	}

	target := l.filteredList[l.targetIndex]

	// Create relation
	relation := model.NewRelation(l.sourceID, l.selectedRel, target.ID)

	// Load and apply template defaults (if template exists)
	template, err := app.repo.LoadRelationTemplate(l.selectedRel)
	if err == nil && template != nil {
		markdown.ApplyRelationTemplate(relation, template)
	}

	if err := app.repo.WriteRelation(relation); err != nil {
		return app, SetMessage(fmt.Sprintf("Failed to create link: %v", err), true)
	}

	// Add to graph
	app.graph.AddEdge(relation)

	// Save cache
	_ = app.repo.SaveCache(app.graph)

	// Refresh browser
	app.browser.Refresh(app)

	return app, tea.Batch(
		PopScreen(),
		SetMessage(fmt.Sprintf("Created link: %s --%s--> %s", l.sourceID, l.selectedRel, target.ID), false),
	)
}

// View renders the link wizard
func (l *LinkModel) View(_, height int) string {
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

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	idStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Create Link from %s", l.sourceID)))
	sb.WriteString("\n\n")

	switch l.step {
	case LinkStepRelation:
		sb.WriteString(labelStyle.Render("Step 1: Select relation type"))
		sb.WriteString("\n\n")

		if len(l.relations) == 0 {
			sb.WriteString(labelStyle.Render("No relations available for this entity type"))
		} else {
			for i, rel := range l.relations {
				marker := "  "
				style := normalStyle
				if i == l.relIndex {
					marker = "► "
					style = selectedStyle
				}
				targets := strings.Join(rel.targetTypes, ", ")
				sb.WriteString(fmt.Sprintf("%s%s %s\n",
					marker,
					style.Render(rel.label),
					labelStyle.Render(fmt.Sprintf("→ [%s]", targets))))
			}
		}

	case LinkStepTarget:
		sb.WriteString(labelStyle.Render(fmt.Sprintf("Step 2: Select target (%s)", l.selectedRel)))
		sb.WriteString("\n")

		if l.filterText != "" {
			sb.WriteString(labelStyle.Render(fmt.Sprintf("Filter: %s", l.filterText)))
		}
		sb.WriteString("\n\n")

		if len(l.filteredList) == 0 {
			sb.WriteString(labelStyle.Render("No matching targets"))
		} else {
			visibleCount := height - 8
			if visibleCount < 5 {
				visibleCount = 5
			}

			startIdx := 0
			if l.targetIndex >= visibleCount {
				startIdx = l.targetIndex - visibleCount + 1
			}
			endIdx := startIdx + visibleCount
			if endIdx > len(l.filteredList) {
				endIdx = len(l.filteredList)
			}

			for i := startIdx; i < endIdx; i++ {
				entity := l.filteredList[i]
				marker := "  "
				style := normalStyle
				if i == l.targetIndex {
					marker = "► "
					style = selectedStyle
				}

				id := idStyle.Render(entity.ID)
				if i == l.targetIndex {
					id = style.Render(entity.ID)
				}

				title := entity.Title()
				if len(title) > 35 {
					title = title[:32] + "..."
				}

				sb.WriteString(fmt.Sprintf("%s%-15s %s %s\n",
					marker,
					id,
					style.Render(title),
					labelStyle.Render(fmt.Sprintf("(%s)", entity.Type))))
			}

			if len(l.filteredList) > visibleCount {
				sb.WriteString(labelStyle.Render(fmt.Sprintf("\n[%d/%d]", l.targetIndex+1, len(l.filteredList))))
			}
		}
	}

	return sb.String()
}

// Help returns help items
func (l *LinkModel) Help() [][2]string {
	switch l.step {
	case LinkStepRelation:
		return [][2]string{
			{"↑/↓", "navigate"},
			{"enter", "select"},
			{"esc", "cancel"},
		}
	case LinkStepTarget:
		return [][2]string{
			{"↑/↓", "navigate"},
			{"type", "filter"},
			{"enter", "create"},
			{"←", "back"},
		}
	}
	return nil
}
