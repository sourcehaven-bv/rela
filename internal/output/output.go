package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Format represents the output format
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"

	// UI layout constants
	tableTitleMaxLen   = 50
	headerSeparatorLen = 60
	traceNodeMaxLen    = 40

	// Box drawing characters
	boxTopLeft     = "┌"
	boxTopRight    = "┐"
	boxBottomLeft  = "└"
	boxBottomRight = "┘"
	boxHorizontal  = "─"
	boxVertical    = "│"

	// Bar chart constants
	barMaxWidth = 8
)

// Writer handles formatted output
type Writer struct {
	Format  Format
	Out     io.Writer
	NoColor bool
}

// New creates a new output writer
func New(format Format) *Writer {
	return &Writer{
		Format: format,
		Out:    os.Stdout,
	}
}

// NewWithWriter creates a new output writer with a custom writer
func NewWithWriter(w io.Writer, format Format) *Writer {
	return &Writer{
		Format:  format,
		Out:     w,
		NoColor: true, // Disable color for custom writers (typically used in tests)
	}
}

// WriteEntities outputs a list of entities
func (w *Writer) WriteEntities(entities []*model.Entity) error {
	if w.Format == FormatJSON {
		return w.writeJSON(entities)
	}
	return w.writeEntitiesTable(entities, false)
}

// WriteEntitiesWithSummary outputs a list of entities with a footer summary
func (w *Writer) WriteEntitiesWithSummary(entities []*model.Entity) error {
	if w.Format == FormatJSON {
		return w.writeJSON(entities)
	}
	return w.writeEntitiesTable(entities, true)
}

func (w *Writer) writeEntitiesTable(entities []*model.Entity, showSummary bool) error {
	table := tablewriter.NewWriter(w.Out)
	table.SetHeader([]string{"ID", "Type", "Title", "Status"})
	table.SetBorder(false)
	table.SetColumnSeparator(" ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// Track status counts for summary
	statusCounts := make(map[string]int)

	for _, e := range entities {
		status := e.GetString("status")
		statusCounts[status]++
		statusDisplay := colorizeStatus(status, w.NoColor)
		table.Append([]string{
			e.ID,
			e.Type,
			truncate(e.Title(), tableTitleMaxLen),
			statusDisplay,
		})
	}

	table.Render()

	// Write footer summary if requested
	if showSummary && len(entities) > 0 {
		summary := w.buildEntitySummary(len(entities), statusCounts)
		w.WriteFooterSummary(summary)
	}

	return nil
}

func (w *Writer) buildEntitySummary(total int, statusCounts map[string]int) string {
	entityWord := "entities"
	if total == 1 {
		entityWord = "entity"
	}

	// Build status breakdown
	var parts []string
	statusOrder := []string{"accepted", "draft", "proposed", "deprecated", "rejected", "retired"}
	for _, status := range statusOrder {
		if count, ok := statusCounts[status]; ok && count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, status))
		}
	}
	// Add any other statuses not in our predefined order
	for status, count := range statusCounts {
		found := false
		for _, s := range statusOrder {
			if s == status {
				found = true
				break
			}
		}
		if !found && status != "" && count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, status))
		}
	}

	if len(parts) > 0 {
		return fmt.Sprintf("%d %s (%s)", total, entityWord, strings.Join(parts, ", "))
	}
	return fmt.Sprintf("%d %s", total, entityWord)
}

// WriteEntity outputs a single entity with details
func (w *Writer) WriteEntity(entity *model.Entity, incoming, outgoing []*model.Relation) error {
	if w.Format == FormatJSON {
		data := map[string]interface{}{
			"entity":   entity,
			"incoming": incoming,
			"outgoing": outgoing,
		}
		return w.writeJSON(data)
	}

	// Header
	fmt.Fprintf(w.Out, "%s %s\n", color.CyanString(entity.ID), color.HiBlackString("(%s)", entity.Type))
	fmt.Fprintln(w.Out, strings.Repeat("─", headerSeparatorLen))

	// Properties
	if title := entity.Title(); title != "" {
		fmt.Fprintf(w.Out, "Title:  %s\n", title)
	}
	if status := entity.GetString("status"); status != "" {
		fmt.Fprintf(w.Out, "Status: %s\n", colorizeStatus(status, w.NoColor))
	}
	if priority := entity.GetString("priority"); priority != "" {
		fmt.Fprintf(w.Out, "Priority: %s\n", priority)
	}
	if desc := entity.Description(); desc != "" {
		fmt.Fprintf(w.Out, "Description: %s\n", truncate(desc, 100))
	}

	// Other properties
	for key, value := range entity.Properties {
		if key != "title" && key != "status" && key != "priority" && key != "description" {
			fmt.Fprintf(w.Out, "%s: %v\n", key, value)
		}
	}

	// Relations
	if len(incoming) > 0 || len(outgoing) > 0 {
		fmt.Fprintln(w.Out)
		fmt.Fprintln(w.Out, color.YellowString("Relations:"))
	}

	for _, rel := range incoming {
		fmt.Fprintf(w.Out, "  ← %s %s %s\n",
			color.GreenString(rel.From),
			color.HiBlackString(rel.Type),
			entity.ID)
	}

	for _, rel := range outgoing {
		fmt.Fprintf(w.Out, "  → %s %s %s\n",
			entity.ID,
			color.HiBlackString(rel.Type),
			color.GreenString(rel.To))
	}

	// Content
	if entity.Content != "" {
		fmt.Fprintln(w.Out)
		fmt.Fprintln(w.Out, color.YellowString("Content:"))
		fmt.Fprintln(w.Out, entity.Content)
	}

	return nil
}

// WriteRelations outputs a list of relations
func (w *Writer) WriteRelations(relations []*model.Relation) error {
	if w.Format == FormatJSON {
		return w.writeJSON(relations)
	}

	table := tablewriter.NewWriter(w.Out)
	table.SetHeader([]string{"From", "Relation", "To"})
	table.SetBorder(false)
	table.SetColumnSeparator(" ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, r := range relations {
		table.Append([]string{r.From, r.Type, r.To})
	}

	table.Render()
	return nil
}

// WriteTrace outputs a trace result as a tree
func (w *Writer) WriteTrace(result *graph.TraceResult) error {
	if w.Format == FormatJSON {
		return w.writeJSON(result)
	}

	w.writeTraceNode(result, "", true)
	return nil
}

func (w *Writer) writeTraceNode(node *graph.TraceResult, prefix string, isLast bool) {
	if node == nil {
		return
	}

	// Determine the connector
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	// Print this node
	relInfo := ""
	if node.Relation != "" {
		relInfo = color.HiBlackString(" [%s]", node.Relation)
	}

	fmt.Fprintf(w.Out, "%s%s%s %s%s\n",
		prefix,
		connector,
		color.CyanString(node.ID),
		truncate(node.Title, traceNodeMaxLen),
		relInfo)

	// Print children
	newPrefix := prefix
	if prefix != "" {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		w.writeTraceNode(child, newPrefix, i == len(node.Children)-1)
	}
}

// WritePath outputs a path between nodes
func (w *Writer) WritePath(path []graph.PathStep) error {
	if w.Format == FormatJSON {
		return w.writeJSON(path)
	}

	if len(path) == 0 {
		fmt.Fprintln(w.Out, "No path found")
		return nil
	}

	// Header with hop count
	hops := len(path) - 1
	hopText := "hop"
	if hops != 1 {
		hopText = "hops"
	}
	if w.NoColor {
		fmt.Fprintf(w.Out, "Path: %s → %s (%d %s)\n\n",
			path[0].ID, path[len(path)-1].ID, hops, hopText)
	} else {
		fmt.Fprintf(w.Out, "%s %s → %s %s\n\n",
			color.HiBlackString("Path:"),
			color.CyanString(path[0].ID),
			color.CyanString(path[len(path)-1].ID),
			color.HiBlackString("(%d %s)", hops, hopText))
	}

	for i, step := range path {
		if i > 0 {
			fmt.Fprintf(w.Out, "  │ %s\n", color.HiBlackString(step.Relation))
			fmt.Fprintln(w.Out, "  ▼")
		}
		fmt.Fprintf(w.Out, "%s %s\n",
			color.CyanString(step.ID),
			color.HiBlackString("(%s)", step.Type))
	}

	return nil
}

// WriteMessage outputs a simple message
func (w *Writer) WriteMessage(format string, args ...interface{}) {
	fmt.Fprintf(w.Out, format+"\n", args...)
}

// WriteSuccess outputs a success message
func (w *Writer) WriteSuccess(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(w.Out, color.GreenString("✓ ")+msg)
}

// WriteError outputs an error message
func (w *Writer) WriteError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(w.Out, color.RedString("✗ ")+msg)
}

// WriteWarning outputs a warning message
func (w *Writer) WriteWarning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(w.Out, color.YellowString("⚠ ")+msg)
}

// WriteInfo outputs an info message
func (w *Writer) WriteInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(w.Out, color.CyanString("ℹ ")+msg)
}

func (w *Writer) writeJSON(data interface{}) error {
	encoder := json.NewEncoder(w.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func colorizeStatus(status string, noColor bool) string {
	if noColor {
		return status
	}
	switch status {
	case "accepted":
		return color.GreenString(status)
	case "draft":
		return color.YellowString(status)
	case "proposed":
		return color.BlueString(status)
	case "deprecated", "rejected", "retired":
		return color.RedString(status)
	default:
		return status
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// WriteSectionHeader writes a styled section header with box drawing
func (w *Writer) WriteSectionHeader(title string) {
	if w.NoColor {
		fmt.Fprintf(w.Out, "─── %s ───\n", title)
		return
	}
	header := fmt.Sprintf("─── %s ───", title)
	fmt.Fprintln(w.Out, color.CyanString(header))
}

// WriteSummaryBox writes a boxed summary line
func (w *Writer) WriteSummaryBox(items []string) {
	content := strings.Join(items, "  │  ")
	width := len(content) + 4

	topBorder := boxTopLeft + strings.Repeat(boxHorizontal, width) + boxTopRight
	bottomBorder := boxBottomLeft + strings.Repeat(boxHorizontal, width) + boxBottomRight

	if w.NoColor {
		fmt.Fprintln(w.Out, topBorder)
		fmt.Fprintf(w.Out, "%s  %s  %s\n", boxVertical, content, boxVertical)
		fmt.Fprintln(w.Out, bottomBorder)
		return
	}
	fmt.Fprintln(w.Out, color.HiBlackString(topBorder))
	fmt.Fprintf(w.Out, "%s  %s  %s\n", color.HiBlackString(boxVertical), content, color.HiBlackString(boxVertical))
	fmt.Fprintln(w.Out, color.HiBlackString(bottomBorder))
}

// WriteBar writes a small inline bar visualization
func (w *Writer) WriteBar(value, maxValue int) string {
	if maxValue == 0 {
		return ""
	}
	barLen := (value * barMaxWidth) / maxValue
	if barLen == 0 && value > 0 {
		barLen = 1
	}
	bar := strings.Repeat("█", barLen)
	if w.NoColor {
		return bar
	}
	return color.CyanString(bar)
}

// WriteSeparator writes a subtle horizontal separator
func (w *Writer) WriteSeparator() {
	sep := strings.Repeat("─", headerSeparatorLen)
	if w.NoColor {
		fmt.Fprintln(w.Out, sep)
		return
	}
	fmt.Fprintln(w.Out, color.HiBlackString(sep))
}

// WriteFooterSummary writes a subtle footer summary line
func (w *Writer) WriteFooterSummary(text string) {
	w.WriteSeparator()
	if w.NoColor {
		fmt.Fprintf(w.Out, "  %s\n", text)
		return
	}
	fmt.Fprintf(w.Out, "  %s\n", color.HiBlackString(text))
}

// Analysis output methods for JSON format

// PropertyValidationResult represents validation errors for JSON output
type PropertyValidationResult struct {
	EntityID   string   `json:"entity_id"`
	EntityType string   `json:"entity_type"`
	Errors     []string `json:"errors"`
}

// AnalysisResult represents the result of an analysis command for JSON output
type AnalysisResult struct {
	Status  string      `json:"status"` // "success", "warning", "error"
	Message string      `json:"message"`
	Count   int         `json:"count,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// WriteAnalysisResult outputs an analysis result in the appropriate format
func (w *Writer) WriteAnalysisResult(result AnalysisResult) error {
	if w.Format == FormatJSON {
		return w.writeJSON(result)
	}

	// Text format based on status
	switch result.Status {
	case "success":
		w.WriteSuccess("%s", result.Message)
	case "warning":
		w.WriteWarning("%s", result.Message)
	case "error":
		w.WriteError("%s", result.Message)
	default:
		w.WriteMessage("%s", result.Message)
	}
	return nil
}

// Schema output methods for JSON format

// WriteSchemaOverview outputs the metamodel overview as JSON
func (w *Writer) WriteSchemaOverview(m SchemaMetamodel) error {
	data := map[string]interface{}{
		"version":   m.GetVersion(),
		"namespace": m.GetNamespace(),
		"entities":  m.GetEntities(),
		"relations": m.GetRelations(),
		"types":     m.GetTypes(),
	}
	return w.writeJSON(data)
}

// WriteSchemaEntities outputs entity types as JSON
func (w *Writer) WriteSchemaEntities(m SchemaMetamodel) error {
	return w.writeJSON(m.GetEntities())
}

// WriteSchemaRelations outputs relation types as JSON
func (w *Writer) WriteSchemaRelations(m SchemaMetamodel) error {
	return w.writeJSON(m.GetRelations())
}

// WriteSchemaTypes outputs custom types as JSON
func (w *Writer) WriteSchemaTypes(m SchemaMetamodel) error {
	return w.writeJSON(m.GetTypes())
}

// WriteSchemaEntityDetail outputs a single entity type as JSON
func (w *Writer) WriteSchemaEntityDetail(name string, def SchemaEntityDef, _ SchemaMetamodel) error {
	data := map[string]interface{}{
		"name":        name,
		"label":       def.GetLabel(),
		"aliases":     def.GetAliases(),
		"id_patterns": def.GetIDPatterns(),
		"properties":  def.GetProperties(),
	}
	if rdfType := def.GetRDFType(); rdfType != "" {
		data["rdf_type"] = rdfType
	}
	if entityColor := def.GetColor(); entityColor != "" {
		data["color"] = entityColor
	}
	if borderColor := def.GetBorderColor(); borderColor != "" {
		data["border_color"] = borderColor
	}
	return w.writeJSON(data)
}

// WriteSchemaRelationDetail outputs a single relation type as JSON
func (w *Writer) WriteSchemaRelationDetail(name string, def SchemaRelationDef) error {
	data := map[string]interface{}{
		"name":  name,
		"label": def.GetLabel(),
		"from":  def.GetFrom(),
		"to":    def.GetTo(),
	}
	if desc := def.GetDescription(); desc != "" {
		data["description"] = desc
	}
	if inv := def.GetInverse(); inv != nil {
		data["inverse"] = inv
	}
	if def.IsSymmetric() {
		data["symmetric"] = true
	}
	if srcMin := def.GetSourceMin(); srcMin != nil {
		data["source_min"] = *srcMin
	}
	if srcMax := def.GetSourceMax(); srcMax != nil {
		data["source_max"] = *srcMax
	}
	if tgtMin := def.GetTargetMin(); tgtMin != nil {
		data["target_min"] = *tgtMin
	}
	if tgtMax := def.GetTargetMax(); tgtMax != nil {
		data["target_max"] = *tgtMax
	}
	return w.writeJSON(data)
}

// SchemaMetamodel interface for metamodel schema output
type SchemaMetamodel interface {
	GetVersion() string
	GetNamespace() string
	GetEntities() interface{}
	GetRelations() interface{}
	GetTypes() interface{}
}

// SchemaEntityDef interface for entity definition output
type SchemaEntityDef interface {
	GetLabel() string
	GetAliases() []string
	GetIDPatterns() []string
	GetProperties() interface{}
	GetRDFType() string
	GetColor() string
	GetBorderColor() string
}

// SchemaRelationDef interface for relation definition output
type SchemaRelationDef interface {
	GetLabel() string
	GetFrom() []string
	GetTo() []string
	GetDescription() string
	GetInverse() interface{}
	IsSymmetric() bool
	GetSourceMin() *int
	GetSourceMax() *int
	GetTargetMin() *int
	GetTargetMax() *int
}
