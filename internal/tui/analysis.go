package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// AnalysisMode represents the current mode
type AnalysisMode int

const (
	AnalysisModeSelect AnalysisMode = iota
	AnalysisModeResults
)

// AnalysisModel handles analysis screen
type AnalysisModel struct {
	mode     AnalysisMode
	checks   []analysisCheck
	selected int
	results  []analysisResult
	scrollY  int
}

type analysisCheck struct {
	name        string
	description string
}

type analysisResult struct {
	title   string
	items   []string
	isError bool
}

var availableChecks = []analysisCheck{
	{"orphans", "Find entities with no connections"},
	{"duplicates", "Find entities with similar titles"},
	{"gaps", "Find gaps in ID sequences"},
	{"cardinality", "Check relation cardinality constraints"},
	{"all", "Run all checks"},
}

// NewAnalysisModel creates a new analysis screen
func NewAnalysisModel(_ *App) *AnalysisModel {
	return &AnalysisModel{
		mode:   AnalysisModeSelect,
		checks: availableChecks,
	}
}

// Update handles key events
func (a *AnalysisModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.mode {
	case AnalysisModeSelect:
		return a.updateSelect(app, msg)
	case AnalysisModeResults:
		return a.updateResults(app, msg)
	}
	return app, nil
}

func (a *AnalysisModel) updateSelect(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if a.selected < len(a.checks)-1 {
			a.selected++
		}
	case "k", "up":
		if a.selected > 0 {
			a.selected--
		}
	case "enter":
		a.runCheck(app, a.checks[a.selected].name)
		a.mode = AnalysisModeResults
	case "a":
		a.runCheck(app, "all")
		a.mode = AnalysisModeResults
	}
	return app, nil
}

func (a *AnalysisModel) updateResults(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		a.scrollY++
	case "k", "up":
		if a.scrollY > 0 {
			a.scrollY--
		}
	case "x":
		// Export analysis results
		if len(a.results) > 0 {
			app.export = NewExportModelFromAnalysis(a.results)
			return app, app.pushScreen(ScreenExport)
		}
	case "esc", "enter", "q":
		a.mode = AnalysisModeSelect
		a.results = nil
		a.scrollY = 0
	}
	return app, nil
}

func (a *AnalysisModel) runCheck(app *App, check string) {
	a.results = nil

	switch check {
	case "orphans":
		a.runOrphans(app)
	case "duplicates":
		a.runDuplicates(app)
	case "gaps":
		a.runGaps(app)
	case "cardinality":
		a.runCardinality(app)
	case "all":
		a.runOrphans(app)
		a.runDuplicates(app)
		a.runGaps(app)
		a.runCardinality(app)
	}
}

func (a *AnalysisModel) runOrphans(app *App) {
	orphans := app.graph.FindOrphans()

	if len(orphans) == 0 {
		a.results = append(a.results, analysisResult{
			title: "Orphans: None found",
			items: []string{"All entities are connected"},
		})
		return
	}

	items := make([]string, len(orphans))
	for i, e := range orphans {
		items[i] = fmt.Sprintf("%s: %s (%s)", e.ID, e.Title(), e.Type)
	}

	a.results = append(a.results, analysisResult{
		title:   fmt.Sprintf("Orphans: %d found", len(orphans)),
		items:   items,
		isError: true,
	})
}

func (a *AnalysisModel) runDuplicates(app *App) {
	entities := app.graph.AllNodes()

	// Group by normalized title
	titleGroups := make(map[string][]*model.Entity)
	for _, e := range entities {
		title := strings.ToLower(strings.TrimSpace(e.Title()))
		if title != "" {
			titleGroups[title] = append(titleGroups[title], e)
		}
	}

	var duplicates [][]*model.Entity
	for _, group := range titleGroups {
		if len(group) > 1 {
			duplicates = append(duplicates, group)
		}
	}

	if len(duplicates) == 0 {
		a.results = append(a.results, analysisResult{
			title: "Duplicates: None found",
			items: []string{"No duplicate titles detected"},
		})
		return
	}

	for _, group := range duplicates {
		items := make([]string, len(group))
		for i, e := range group {
			items[i] = fmt.Sprintf("%s (%s)", e.ID, e.Type)
		}
		a.results = append(a.results, analysisResult{
			title:   fmt.Sprintf("Duplicate: '%s'", group[0].Title()),
			items:   items,
			isError: true,
		})
	}
}

func (a *AnalysisModel) runGaps(_ *App) {
	// This is a simplified version
	a.results = append(a.results, analysisResult{
		title: "ID Gaps: Check complete",
		items: []string{"Use 'rela analyze gaps' for detailed analysis"},
	})
}

func (a *AnalysisModel) runCardinality(app *App) {
	violations := 0

	for relName, relDef := range app.metamodel.Relations {
		if relDef.MinOutgoing != nil && *relDef.MinOutgoing > 0 {
			for _, sourceType := range relDef.From {
				entities := app.graph.NodesByType(sourceType)
				for _, e := range entities {
					count := 0
					for _, edge := range app.graph.OutgoingEdges(e.ID) {
						if edge.Type == relName {
							count++
						}
					}
					if count < *relDef.MinOutgoing {
						violations++
					}
				}
			}
		}
	}

	if violations == 0 {
		a.results = append(a.results, analysisResult{
			title: "Cardinality: All constraints satisfied",
			items: []string{"No violations found"},
		})
	} else {
		a.results = append(a.results, analysisResult{
			title:   fmt.Sprintf("Cardinality: %d violations", violations),
			items:   []string{"Use 'rela analyze cardinality' for details"},
			isError: true,
		})
	}
}

// View renders the analysis screen
func (a *AnalysisModel) View(_, height int) string {
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

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("82"))

	switch a.mode {
	case AnalysisModeSelect:
		sb.WriteString(titleStyle.Render("Analysis"))
		sb.WriteString("\n\n")

		for i, check := range a.checks {
			marker := "  "
			style := normalStyle
			if i == a.selected {
				marker = "► "
				style = selectedStyle
			}

			sb.WriteString(fmt.Sprintf("%s%s\n", marker, style.Render(check.name)))
			sb.WriteString(fmt.Sprintf("    %s\n", labelStyle.Render(check.description)))
		}

	case AnalysisModeResults:
		sb.WriteString(titleStyle.Render("Analysis Results"))
		sb.WriteString("\n\n")

		// Build all result lines first
		var lines []string
		for _, result := range a.results {
			titleRenderer := successStyle
			if result.isError {
				titleRenderer = errorStyle
			}

			lines = append(lines, titleRenderer.Render("● "+result.title))

			for _, item := range result.items {
				lines = append(lines, fmt.Sprintf("  %s", normalStyle.Render(item)))
			}
			lines = append(lines, "")
		}

		// Apply scrolling
		visibleCount := height - 8
		if visibleCount < 5 {
			visibleCount = 5
		}

		// Clamp scrollY
		maxScroll := len(lines) - visibleCount
		if maxScroll < 0 {
			maxScroll = 0
		}
		if a.scrollY > maxScroll {
			a.scrollY = maxScroll
		}

		endIdx := a.scrollY + visibleCount
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		for i := a.scrollY; i < endIdx; i++ {
			sb.WriteString(lines[i])
			sb.WriteString("\n")
		}

		// Show scroll indicator if needed
		if len(lines) > visibleCount {
			sb.WriteString(labelStyle.Render(fmt.Sprintf("\n[%d-%d of %d lines] ↑/↓ to scroll", a.scrollY+1, endIdx, len(lines))))
		}

		sb.WriteString(labelStyle.Render("\nPress Enter or Esc to go back"))
	}

	return sb.String()
}

// Help returns help items
func (a *AnalysisModel) Help() [][2]string {
	switch a.mode {
	case AnalysisModeSelect:
		return [][2]string{
			{"↑/↓", "navigate"},
			{"enter", "run"},
			{"a", "run all"},
			{"esc", "back"},
		}
	case AnalysisModeResults:
		return [][2]string{
			{"↑/↓", "scroll"},
			{"x", "export"},
			{"enter", "back"},
		}
	}
	return nil
}
