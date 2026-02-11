package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ExportSource represents the source screen for export
type ExportSource int

const (
	ExportFromBrowser ExportSource = iota
	ExportFromGraph
	ExportFromAnalysis
)

// ExportFormat represents available export formats
type ExportFormat int

const (
	FormatJSON ExportFormat = iota
	FormatCSV
	FormatDOT
	FormatMarkdown
)

// ExportMode represents the current mode of the export screen
type ExportMode int

const (
	ExportModeSelect ExportMode = iota
	ExportModeFilename
	ExportModeConfirm
)

type exportOption struct {
	format      ExportFormat
	name        string
	description string
	extension   string
}

// ExportModel handles the export screen
type ExportModel struct {
	source   ExportSource
	mode     ExportMode
	options  []exportOption
	selected int
	filename string
	message  string
	isError  bool

	// Context data
	browserType  string           // Selected entity type from browser (empty = all)
	graphRootID  string           // Root entity for graph export
	graphDepth   int              // Depth for graph export
	analysisData []analysisResult // Analysis results to export
}

// NewExportModel creates a new export model for browser
func NewExportModel(source ExportSource) *ExportModel {
	m := &ExportModel{
		source: source,
		mode:   ExportModeSelect,
	}
	m.setupOptions()
	return m
}

// NewExportModelFromBrowser creates export model for browser context
func NewExportModelFromBrowser(entityType string) *ExportModel {
	m := &ExportModel{
		source:      ExportFromBrowser,
		mode:        ExportModeSelect,
		browserType: entityType,
	}
	m.setupOptions()
	return m
}

// NewExportModelFromGraph creates export model for graph context
func NewExportModelFromGraph(rootID string, depth int) *ExportModel {
	m := &ExportModel{
		source:      ExportFromGraph,
		mode:        ExportModeSelect,
		graphRootID: rootID,
		graphDepth:  depth,
	}
	m.setupOptions()
	return m
}

// NewExportModelFromAnalysis creates export model for analysis context
func NewExportModelFromAnalysis(results []analysisResult) *ExportModel {
	m := &ExportModel{
		source:       ExportFromAnalysis,
		mode:         ExportModeSelect,
		analysisData: results,
	}
	m.setupOptions()
	return m
}

func (e *ExportModel) setupOptions() {
	switch e.source {
	case ExportFromBrowser:
		e.options = []exportOption{
			{FormatJSON, "JSON", "Export entities as JSON", "json"},
			{FormatCSV, "CSV", "Export entities as CSV (current type only)", "csv"},
		}
		e.filename = "entities"
		if e.browserType != "" {
			e.filename = e.browserType + "s"
		}
	case ExportFromGraph:
		e.options = []exportOption{
			{FormatDOT, "DOT (Graphviz)", "Export graph in DOT format", "dot"},
			{FormatJSON, "JSON", "Export graph data as JSON", "json"},
		}
		e.filename = "graph"
		if e.graphRootID != "" {
			e.filename = "graph-" + e.graphRootID
		}
	case ExportFromAnalysis:
		e.options = []exportOption{
			{FormatMarkdown, "Markdown", "Export analysis as Markdown report", "md"},
			{FormatJSON, "JSON", "Export analysis data as JSON", "json"},
		}
		e.filename = "analysis-report"
	}
}

// Update handles key events
func (e *ExportModel) Update(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch e.mode {
	case ExportModeSelect:
		return e.updateSelect(app, msg)
	case ExportModeFilename:
		return e.updateFilename(app, msg)
	case ExportModeConfirm:
		return e.updateConfirm(app, msg)
	}
	return app, nil
}

func (e *ExportModel) updateSelect(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if e.selected < len(e.options)-1 {
			e.selected++
		}
	case "k", "up":
		if e.selected > 0 {
			e.selected--
		}
	case "enter":
		e.mode = ExportModeFilename
		// Set default filename with extension
		e.filename = e.filename + "." + e.options[e.selected].extension
	}
	return app, nil
}

func (e *ExportModel) updateFilename(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Perform export
		err := e.performExport(app)
		if err != nil {
			e.message = err.Error()
			e.isError = true
		} else {
			e.message = fmt.Sprintf("Exported to %s", e.filename)
			e.isError = false
		}
		e.mode = ExportModeConfirm
	case "backspace":
		if e.filename != "" {
			e.filename = e.filename[:len(e.filename)-1]
		}
	case "esc":
		e.mode = ExportModeSelect
	default:
		// Add printable characters
		if len(msg.String()) == 1 {
			e.filename += msg.String()
		}
	}
	return app, nil
}

func (e *ExportModel) updateConfirm(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		return app, app.popScreen()
	}
	return app, nil
}

func (e *ExportModel) performExport(app *App) error {
	opt := e.options[e.selected]

	// Ensure filename has extension
	if !strings.HasSuffix(e.filename, "."+opt.extension) {
		e.filename = e.filename + "." + opt.extension
	}

	// Create full path in project root
	fullPath := filepath.Join(app.project.Root, e.filename)

	switch e.source {
	case ExportFromBrowser:
		return e.exportBrowser(app, opt.format, fullPath)
	case ExportFromGraph:
		return e.exportGraph(app, opt.format, fullPath)
	case ExportFromAnalysis:
		return e.exportAnalysis(app, opt.format, fullPath)
	}
	return fmt.Errorf("unknown export source")
}

func (e *ExportModel) exportBrowser(app *App, format ExportFormat, path string) error {
	var entities []*model.Entity

	if e.browserType != "" {
		entities = app.graph.NodesByType(e.browserType)
	} else {
		entities = app.graph.AllNodes()
	}

	switch format {
	case FormatJSON:
		return e.writeJSON(path, entities)
	case FormatCSV:
		return e.writeCSV(path, entities)
	}
	return fmt.Errorf("unsupported format for browser export")
}

func (e *ExportModel) exportGraph(app *App, format ExportFormat, path string) error {
	entities := app.graph.AllNodes()
	edges := app.graph.AllEdges()

	switch format {
	case FormatDOT:
		return e.writeDOT(app, path, entities, edges)
	case FormatJSON:
		data := map[string]interface{}{
			"nodes": entities,
			"edges": edges,
		}
		return e.writeJSON(path, data)
	}
	return fmt.Errorf("unsupported format for graph export")
}

func (e *ExportModel) exportAnalysis(_ *App, format ExportFormat, path string) error {
	switch format {
	case FormatMarkdown:
		return e.writeAnalysisMarkdown(path)
	case FormatJSON:
		return e.writeJSON(path, e.analysisData)
	}
	return fmt.Errorf("unsupported format for analysis export")
}

func (e *ExportModel) writeJSON(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return os.WriteFile(path, jsonData, 0644)
}

func (e *ExportModel) writeCSV(path string, entities []*model.Entity) error {
	var sb strings.Builder

	// Collect all unique property keys
	keys := make(map[string]bool)
	for _, ent := range entities {
		for k := range ent.Properties {
			keys[k] = true
		}
	}

	// Build header
	headers := []string{"id", "type"}
	for k := range keys {
		headers = append(headers, k)
	}
	sb.WriteString(strings.Join(headers, ",") + "\n")

	// Write rows
	for _, ent := range entities {
		row := []string{
			escapeCSV(ent.ID),
			escapeCSV(ent.Type),
		}
		for _, k := range headers[2:] {
			if v, ok := ent.Properties[k]; ok {
				row = append(row, escapeCSV(fmt.Sprintf("%v", v)))
			} else {
				row = append(row, "")
			}
		}
		sb.WriteString(strings.Join(row, ",") + "\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

func (e *ExportModel) writeDOT(app *App, path string, entities []*model.Entity, edges []*model.Relation) error {
	var sb strings.Builder

	sb.WriteString("digraph architecture {\n")
	sb.WriteString("  rankdir=TB;\n")
	sb.WriteString("  node [shape=box, style=filled];\n")
	sb.WriteString("\n")

	// Group nodes by type
	typeGroups := make(map[string][]*model.Entity)
	for _, ent := range entities {
		typeGroups[ent.Type] = append(typeGroups[ent.Type], ent)
	}

	// Write nodes grouped by type (as subgraphs for clustering)
	for entityType, group := range typeGroups {
		sb.WriteString(fmt.Sprintf("  subgraph cluster_%s {\n", entityType))
		sb.WriteString(fmt.Sprintf("    label=\"%ss\";\n", strings.ToUpper(entityType[:1])+entityType[1:]))

		// Get color from metamodel
		color := "#FFFFFF"
		if def, ok := app.metamodel.GetEntityDef(entityType); ok && def.Color != "" {
			color = def.Color
		}

		for _, ent := range group {
			label := escapeDOTLabel(ent.Title())
			if label == "" {
				label = ent.ID
			} else {
				label = ent.ID + "\\n" + label
			}
			sb.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\", fillcolor=\"%s\"];\n",
				ent.ID, label, color))
		}

		sb.WriteString("  }\n\n")
	}

	// Write edges
	for _, edge := range edges {
		sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n",
			edge.From, edge.To, edge.Type))
	}

	sb.WriteString("}\n")

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

const maxDOTLabelLen = 40

func escapeDOTLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	// Truncate long labels
	if len(s) > maxDOTLabelLen {
		s = s[:maxDOTLabelLen-3] + "..."
	}
	return s
}

func (e *ExportModel) writeAnalysisMarkdown(path string) error {
	var sb strings.Builder

	sb.WriteString("# Analysis Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	for _, result := range e.analysisData {
		status := "OK"
		if result.isError {
			status = "WARNING"
		}
		sb.WriteString(fmt.Sprintf("## %s [%s]\n\n", result.title, status))

		for _, item := range result.items {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// View renders the export screen
func (e *ExportModel) View(_, _ int) string {
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

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220")).
		Bold(true)

	switch e.mode {
	case ExportModeSelect:
		sb.WriteString(titleStyle.Render("Export"))
		sb.WriteString("\n\n")

		// Show context info
		switch e.source {
		case ExportFromBrowser:
			if e.browserType != "" {
				sb.WriteString(labelStyle.Render(fmt.Sprintf("Exporting: %ss\n\n", e.browserType)))
			} else {
				sb.WriteString(labelStyle.Render("Exporting: all entities\n\n"))
			}
		case ExportFromGraph:
			sb.WriteString(labelStyle.Render(fmt.Sprintf("Exporting graph from: %s (depth: %d)\n\n", e.graphRootID, e.graphDepth)))
		case ExportFromAnalysis:
			sb.WriteString(labelStyle.Render(fmt.Sprintf("Exporting: %d analysis results\n\n", len(e.analysisData))))
		}

		sb.WriteString(labelStyle.Render("Select format:"))
		sb.WriteString("\n\n")

		for i, opt := range e.options {
			marker := "  "
			style := normalStyle
			if i == e.selected {
				marker = "> "
				style = selectedStyle
			}

			sb.WriteString(fmt.Sprintf("%s%s\n", marker, style.Render(opt.name)))
			sb.WriteString(fmt.Sprintf("    %s\n", labelStyle.Render(opt.description)))
		}

	case ExportModeFilename:
		sb.WriteString(titleStyle.Render("Export"))
		sb.WriteString("\n\n")

		sb.WriteString(labelStyle.Render("Format: "))
		sb.WriteString(normalStyle.Render(e.options[e.selected].name))
		sb.WriteString("\n\n")

		sb.WriteString(labelStyle.Render("Filename: "))
		sb.WriteString(inputStyle.Render(e.filename))
		sb.WriteString(normalStyle.Render("_"))
		sb.WriteString("\n\n")

		sb.WriteString(labelStyle.Render("Press Enter to export, Esc to go back"))

	case ExportModeConfirm:
		sb.WriteString(titleStyle.Render("Export"))
		sb.WriteString("\n\n")

		if e.isError {
			sb.WriteString(errorStyle.Render("Error: " + e.message))
		} else {
			sb.WriteString(successStyle.Render("Success: " + e.message))
		}

		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Press Enter to close"))
	}

	return sb.String()
}

// Help returns help items
func (e *ExportModel) Help() [][2]string {
	switch e.mode {
	case ExportModeSelect:
		return [][2]string{
			{"j/k", "navigate"},
			{"enter", "select"},
			{"esc", "cancel"},
		}
	case ExportModeFilename:
		return [][2]string{
			{"type", "filename"},
			{"enter", "export"},
			{"esc", "back"},
		}
	case ExportModeConfirm:
		return [][2]string{
			{"enter", "close"},
		}
	}
	return nil
}
