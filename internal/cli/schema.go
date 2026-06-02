package cli

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SchemaCmd is the `rela schema` command tree. Without a subcommand,
// kong routes to SchemaOverviewCmd (default:"withargs"), so flags
// like `--graphviz` work as `rela schema --graphviz`.
type SchemaCmd struct {
	Overview  SchemaOverviewCmd  `cmd:"" default:"withargs" help:"Show metamodel overview."`
	Entities  SchemaEntitiesCmd  `cmd:"" help:"List all entity types."`
	Relations SchemaRelationsCmd `cmd:"" help:"List all relation types."`
	Types     SchemaTypesCmd     `cmd:"" help:"List custom types defined in the metamodel."`
	Entity    SchemaEntityCmd    `cmd:"" help:"Show details for a specific entity type."`
	Relation  SchemaRelationCmd  `cmd:"" help:"Show details for a specific relation type."`
}

// SchemaOverviewCmd is `rela schema overview` (also the default when
// `rela schema` is invoked with no subcommand). The graphviz flags
// live here so `rela schema --graphviz` routes here via default:"withargs".
type SchemaOverviewCmd struct {
	Graphviz    bool     `help:"Output metamodel as GraphViz DOT format."`
	Constraints bool     `help:"Include cardinality constraints in GraphViz output."`
	Exclude     []string `help:"Hide an entity type in --graphviz output (repeatable)."`
	NoBundle    bool     `name:"no-bundle" help:"Disable the hub bundle for 3-4 targets with an isolated node."`
	NoLegend    bool     `name:"no-legend" help:"Disable the legend table for many-target / fully-connected relations."`
}

// Run runs the schema overview (or graphviz output when --graphviz is set).
func (c *SchemaOverviewCmd) Run(ctx context.Context, svc *cliServices) error {
	if c.Graphviz {
		return runSchemaGraphviz(svc.Meta(), c.Constraints, c.Exclude, c.NoBundle, c.NoLegend)
	}
	return runSchemaOverview(ctx, svc)
}

// SchemaEntitiesCmd is `rela schema entities`.
type SchemaEntitiesCmd struct{}

// Run lists all entity types.
func (c *SchemaEntitiesCmd) Run(svc *cliServices) error {
	return runSchemaEntities(svc.Meta())
}

// SchemaRelationsCmd is `rela schema relations`.
type SchemaRelationsCmd struct{}

// Run lists all relation types.
func (c *SchemaRelationsCmd) Run(svc *cliServices) error {
	return runSchemaRelations(svc.Meta())
}

// SchemaTypesCmd is `rela schema types`.
type SchemaTypesCmd struct{}

// Run lists all custom types.
func (c *SchemaTypesCmd) Run(svc *cliServices) error {
	return runSchemaTypes(svc.Meta())
}

// SchemaEntityCmd is `rela schema entity <name>`.
type SchemaEntityCmd struct {
	Name string `arg:"" help:"Entity type name."`
}

// Run shows details for a specific entity type.
func (c *SchemaEntityCmd) Run(svc *cliServices) error {
	return runSchemaEntity(svc.Meta(), c.Name)
}

// SchemaRelationCmd is `rela schema relation <name>`.
type SchemaRelationCmd struct {
	Name string `arg:"" help:"Relation type name."`
}

// Run shows details for a specific relation type.
func (c *SchemaRelationCmd) Run(svc *cliServices) error {
	return runSchemaRelation(svc.Meta(), c.Name)
}

func runSchemaOverview(ctx context.Context, svc *cliServices) error {
	meta := svc.Meta()
	if out.Format == "json" {
		return out.WriteSchemaOverview(meta)
	}

	out.WriteSectionHeader("Metamodel Overview")
	out.WriteMessage("")

	if meta.Version != "" {
		out.WriteMessage("Version:   %s", meta.Version)
	}
	if meta.Namespace != "" {
		out.WriteMessage("Namespace: %s", meta.Namespace)
	}
	if meta.Version != "" || meta.Namespace != "" {
		out.WriteMessage("")
	}

	entityNames := getSortedEntityNames(meta)
	entityCounts := make(map[string]int)
	maxCount := 0
	st := svc.Store()
	for _, name := range entityNames {
		count, _ := st.CountEntities(ctx, store.EntityQuery{Type: name})
		entityCounts[name] = count
		if count > maxCount {
			maxCount = count
		}
	}

	out.WriteMessage("Entity Types (%d):", len(entityNames))
	for _, name := range entityNames {
		def := meta.Entities[name]
		if count, ok := entityCounts[name]; ok && maxCount > 0 {
			bar := out.WriteBar(count, maxCount)
			out.WriteMessage("  %-14s %s %d", def.Label, bar, count)
		} else {
			out.WriteMessage("  - %s (%s)", def.Label, name)
		}
	}
	out.WriteMessage("")

	relationNames := getSortedRelationNames(meta)
	out.WriteMessage("Relation Types (%d):", len(relationNames))
	for _, name := range relationNames {
		def := meta.Relations[name]
		out.WriteMessage("  - %s (%s)", def.Label, name)
	}
	out.WriteMessage("")

	typeNames := getSortedTypeNames(meta)
	if len(typeNames) > 0 {
		out.WriteMessage("Custom Types (%d):", len(typeNames))
		for _, name := range typeNames {
			typeDef := meta.Types[name]
			out.WriteMessage("  - %s: [%s]", name, strings.Join(typeDef.Values, ", "))
		}
	}
	return nil
}

func runSchemaEntities(meta *metamodel.Metamodel) error {
	if out.Format == "json" {
		return out.WriteSchemaEntities(meta)
	}
	entityNames := getSortedEntityNames(meta)
	if len(entityNames) == 0 {
		out.WriteMessage("No entity types defined")
		return nil
	}
	out.WriteMessage("Entity Types")
	out.WriteMessage("============")
	out.WriteMessage("")
	for _, name := range entityNames {
		def := meta.Entities[name]
		out.WriteMessage("%s (%s)", def.Label, name)
		if len(def.Aliases) > 0 {
			out.WriteMessage("  Aliases: %s", strings.Join(def.Aliases, ", "))
		}
		out.WriteMessage("  ID Prefixes: %s", strings.Join(def.GetIDPrefixes(), ", "))
		propCount := len(def.Properties)
		requiredCount := 0
		for _, prop := range def.Properties {
			if prop.Required {
				requiredCount++
			}
		}
		out.WriteMessage("  Properties: %d (%d required)", propCount, requiredCount)
		out.WriteMessage("")
	}
	return nil
}

func runSchemaRelations(meta *metamodel.Metamodel) error {
	if out.Format == "json" {
		return out.WriteSchemaRelations(meta)
	}
	relationNames := getSortedRelationNames(meta)
	if len(relationNames) == 0 {
		out.WriteMessage("No relation types defined")
		return nil
	}
	out.WriteMessage("Relation Types")
	out.WriteMessage("==============")
	out.WriteMessage("")
	for _, name := range relationNames {
		def := meta.Relations[name]
		out.WriteMessage("%s (%s)", def.Label, name)
		out.WriteMessage("  From: [%s] -> To: [%s]", strings.Join(def.From, ", "), strings.Join(def.To, ", "))
		if def.Inverse != nil && def.Inverse.GetID() != "" {
			out.WriteMessage("  Inverse: %s (%s)", def.Inverse.GetLabel(), def.Inverse.GetID())
		}
		if def.Description != "" {
			out.WriteMessage("  Description: %s", def.Description)
		}
		cardParts := []string{}
		if def.MinOutgoing != nil {
			cardParts = append(cardParts, fmt.Sprintf("min_outgoing=%d", *def.MinOutgoing))
		}
		if def.MaxOutgoing != nil {
			cardParts = append(cardParts, fmt.Sprintf("max_outgoing=%d", *def.MaxOutgoing))
		}
		if def.MinIncoming != nil {
			cardParts = append(cardParts, fmt.Sprintf("min_incoming=%d", *def.MinIncoming))
		}
		if def.MaxIncoming != nil {
			cardParts = append(cardParts, fmt.Sprintf("max_incoming=%d", *def.MaxIncoming))
		}
		if len(cardParts) > 0 {
			out.WriteMessage("  Cardinality: %s", strings.Join(cardParts, ", "))
		}
		if def.Symmetric {
			out.WriteMessage("  Symmetric: yes")
		}
		out.WriteMessage("")
	}
	return nil
}

func runSchemaTypes(meta *metamodel.Metamodel) error {
	if out.Format == "json" {
		return out.WriteSchemaTypes(meta)
	}
	typeNames := getSortedTypeNames(meta)
	if len(typeNames) == 0 {
		out.WriteMessage("No custom types defined")
		return nil
	}
	out.WriteMessage("Custom Types")
	out.WriteMessage("============")
	out.WriteMessage("")
	for _, name := range typeNames {
		typeDef := meta.Types[name]
		out.WriteMessage("%s", name)
		out.WriteMessage("  Values: %s", strings.Join(typeDef.Values, ", "))
		if typeDef.Default != "" {
			out.WriteMessage("  Default: %s", typeDef.Default)
		}
		out.WriteMessage("")
	}
	return nil
}

func runSchemaEntity(meta *metamodel.Metamodel, name string) error {
	resolved := meta.ResolveAlias(name)
	def, ok := meta.GetEntityDef(resolved)
	if !ok {
		return fmt.Errorf("unknown entity type: %s", name)
	}
	if out.Format == "json" {
		return out.WriteSchemaEntityDetail(resolved, def, meta)
	}

	out.WriteMessage("Entity Type: %s", def.Label)
	out.WriteMessage("============%s", strings.Repeat("=", len(def.Label)+1))
	out.WriteMessage("")

	writeEntityBasicInfo(resolved, def)
	out.WriteMessage("")
	out.WriteMessage("Properties:")
	writeEntityProperties(meta, def)
	out.WriteMessage("")
	out.WriteMessage("Relations:")
	writeEntityRelations(meta, resolved)
	return nil
}

func writeEntityBasicInfo(resolved string, def *metamodel.EntityDef) {
	out.WriteMessage("Name: %s", resolved)
	if len(def.Aliases) > 0 {
		out.WriteMessage("Aliases: %s", strings.Join(def.Aliases, ", "))
	}
	out.WriteMessage("ID Prefixes: %s", strings.Join(def.GetIDPrefixes(), ", "))
	if def.RDFType != "" {
		out.WriteMessage("RDF Type: %s", def.RDFType)
	}
	if def.Color != "" || def.BorderColor != "" {
		colors := []string{}
		if def.Color != "" {
			colors = append(colors, "fill="+def.Color)
		}
		if def.BorderColor != "" {
			colors = append(colors, "border="+def.BorderColor)
		}
		out.WriteMessage("Colors: %s", strings.Join(colors, ", "))
	}
}

func writeEntityProperties(meta *metamodel.Metamodel, def *metamodel.EntityDef) {
	propNames := make([]string, 0, len(def.Properties))
	for propName := range def.Properties {
		propNames = append(propNames, propName)
	}
	sort.Slice(propNames, func(i, j int) bool {
		pi := def.Properties[propNames[i]]
		pj := def.Properties[propNames[j]]
		if pi.Required != pj.Required {
			return pi.Required
		}
		return propNames[i] < propNames[j]
	})
	for _, propName := range propNames {
		prop := def.Properties[propName]
		writePropertyDetail(meta, propName, prop)
	}
}

func writePropertyDetail(meta *metamodel.Metamodel, propName string, prop metamodel.PropertyDef) {
	required := ""
	if prop.Required {
		required = " (required)"
	}
	effectiveType := prop.Type
	if effectiveType == "" {
		effectiveType = "string"
	}
	out.WriteMessage("  %s: %s%s", propName, effectiveType, required)
	if prop.Description != "" {
		out.WriteMessage("    Description: %s", prop.Description)
	}
	if prop.Default != "" {
		out.WriteMessage("    Default: %s", prop.Default)
	}
	if len(prop.Values) > 0 {
		out.WriteMessage("    Values: [%s]", strings.Join(prop.Values, ", "))
	} else if customType, ok := meta.Types[prop.Type]; ok {
		out.WriteMessage("    Values: [%s]", strings.Join(customType.Values, ", "))
	}
	if prop.Format != "" {
		out.WriteMessage("    Format: %s", prop.Format)
	}
}

func writeEntityRelations(meta *metamodel.Metamodel, resolved string) {
	hasRelations := false
	for relName, relDef := range meta.Relations {
		isSource := sliceContains(relDef.From, resolved)
		isTarget := sliceContains(relDef.To, resolved)
		if isSource {
			hasRelations = true
			out.WriteMessage("  -> %s -> [%s]", relName, strings.Join(relDef.To, ", "))
		}
		if isTarget {
			hasRelations = true
			inverseName := relName
			if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
				inverseName = relDef.Inverse.GetID()
			}
			out.WriteMessage("  <- %s <- [%s]", inverseName, strings.Join(relDef.From, ", "))
		}
	}
	if !hasRelations {
		out.WriteMessage("  (none)")
	}
}

func runSchemaRelation(meta *metamodel.Metamodel, name string) error {
	def, ok := meta.GetRelationDef(name)
	if !ok {
		return fmt.Errorf("unknown relation type: %s", name)
	}
	if out.Format == "json" {
		return out.WriteSchemaRelationDetail(name, def)
	}
	out.WriteMessage("Relation Type: %s", def.Label)
	out.WriteMessage("==============%s", strings.Repeat("=", len(def.Label)+1))
	out.WriteMessage("")
	out.WriteMessage("Name: %s", name)
	out.WriteMessage("From: [%s]", strings.Join(def.From, ", "))
	out.WriteMessage("To: [%s]", strings.Join(def.To, ", "))
	if def.Inverse != nil && def.Inverse.GetID() != "" {
		out.WriteMessage("")
		out.WriteMessage("Inverse:")
		out.WriteMessage("  ID: %s", def.Inverse.GetID())
		out.WriteMessage("  Label: %s", def.Inverse.GetLabel())
	}
	if def.Description != "" {
		out.WriteMessage("")
		out.WriteMessage("Description: %s", def.Description)
	}
	if def.MinOutgoing != nil || def.MaxOutgoing != nil || def.MinIncoming != nil || def.MaxIncoming != nil {
		out.WriteMessage("")
		out.WriteMessage("Cardinality Constraints:")
		if def.MinOutgoing != nil {
			out.WriteMessage("  Min Outgoing: %d", *def.MinOutgoing)
		}
		if def.MaxOutgoing != nil {
			out.WriteMessage("  Max Outgoing: %d", *def.MaxOutgoing)
		}
		if def.MinIncoming != nil {
			out.WriteMessage("  Min Incoming: %d", *def.MinIncoming)
		}
		if def.MaxIncoming != nil {
			out.WriteMessage("  Max Incoming: %d", *def.MaxIncoming)
		}
	}
	if def.Symmetric {
		out.WriteMessage("")
		out.WriteMessage("Symmetric: yes")
	}
	return nil
}

// Default color palette for entity types (pastel colors for readability)
var defaultEntityColors = []string{
	"#e3f2fd", "#e8f5e9", "#fff3e0", "#f3e5f5",
	"#e0f7fa", "#fce4ec", "#fffde7", "#efebe9",
}

var defaultEdgeColors = []string{
	"#1976d2", "#388e3c", "#f57c00", "#7b1fa2",
	"#0097a7", "#c2185b", "#fbc02d", "#5d4037",
}

const (
	renderPlain  = "plain"
	renderHub    = "hub"
	renderLegend = "legend"
)

const (
	minHubTargets         = 3
	legendTargetThreshold = 5
)

const (
	legendNodeID = "__legend"
	hubIDPrefix  = "__hub_"
)

func dotID(s string) string {
	if s == "" {
		return `""`
	}
	for i, r := range s {
		first := i == 0
		valid := r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(!first && r >= '0' && r <= '9')
		if !valid {
			var sb strings.Builder
			sb.Grow(len(s) + 2)
			sb.WriteByte('"')
			for _, c := range s {
				if c == '\\' || c == '"' {
					sb.WriteByte('\\')
				}
				sb.WriteRune(c)
			}
			sb.WriteByte('"')
			return sb.String()
		}
	}
	return s
}

type relPair struct {
	source string
	rel    string
	to     []string
	render string
	relDef metamodel.RelationDef
	srcIdx int
}

func runSchemaGraphviz(meta *metamodel.Metamodel, constraints bool, exclude []string, noBundle, noLegend bool) error {
	entityNames, relPairs := prepareSchemaGraph(meta, exclude)
	classifyRenderings(entityNames, relPairs, noBundle, noLegend)

	var sb strings.Builder
	sb.WriteString("digraph metamodel {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=\"filled,rounded\", fontname=\"Helvetica\"];\n")
	sb.WriteString("  edge [fontsize=10, fontname=\"Helvetica\"];\n")
	sb.WriteString("\n")

	visible := visibleEntities(entityNames, relPairs)

	sb.WriteString("  // Entity types\n")
	entityColorIndex := make(map[string]int)
	for i, name := range entityNames {
		entityColorIndex[name] = i
		if !visible[name] {
			continue
		}
		def := meta.Entities[name]
		fillColor := def.Color
		if fillColor == "" {
			fillColor = defaultEntityColors[i%len(defaultEntityColors)]
		}
		borderColor := def.BorderColor
		if borderColor == "" {
			borderColor = darkenColor(fillColor)
		}
		fmt.Fprintf(&sb, "  %s [label=%q, fillcolor=%q, color=%q];\n",
			dotID(name), def.Label, fillColor, borderColor)
	}
	sb.WriteString("\n")

	sb.WriteString("  // Relations\n")
	var legendEntries []relPair
	hubIdx := 0
	for _, p := range relPairs {
		edgeLabel := p.relDef.Label
		if constraints {
			edgeLabel = buildConstraintLabel(p.relDef)
		}
		edgeColor := defaultEdgeColors[entityColorIndex[p.source]%len(defaultEdgeColors)]

		switch p.render {
		case renderPlain:
			for _, to := range p.to {
				fmt.Fprintf(&sb, "  %s -> %s [label=%q, color=%q, fontcolor=%q];\n",
					dotID(p.source), dotID(to), edgeLabel, edgeColor, edgeColor)
			}
		case renderHub:
			hub := fmt.Sprintf("%s%d", hubIDPrefix, hubIdx)
			hubIdx++
			fmt.Fprintf(&sb, "  %s [shape=point, width=0.05, height=0.05, label=\"\"];\n", hub)
			fmt.Fprintf(&sb, "  %s -> %s [label=%q, color=%q, fontcolor=%q, arrowhead=none];\n",
				dotID(p.source), hub, edgeLabel, edgeColor, edgeColor)
			for _, to := range p.to {
				fmt.Fprintf(&sb, "  %s -> %s [color=%q];\n", hub, dotID(to), edgeColor)
			}
		case renderLegend:
			legendEntries = append(legendEntries, p)
		}
	}

	if len(legendEntries) > 0 {
		sb.WriteString("\n")
		sb.WriteString(renderLegendNode(meta, legendEntries, entityNames))
	}

	sb.WriteString("}\n")
	fmt.Print(sb.String())
	return nil
}

func prepareSchemaGraph(m *metamodel.Metamodel, schemaExclude []string) ([]string, []relPair) {
	excluded := make(map[string]bool, len(schemaExclude))
	for _, name := range schemaExclude {
		excluded[name] = true
	}

	allNames := getSortedEntityNames(m)
	entityNames := make([]string, 0, len(allNames))
	for _, name := range allNames {
		if !excluded[name] {
			entityNames = append(entityNames, name)
		}
	}

	colorIndex := make(map[string]int, len(allNames))
	for i, name := range allNames {
		colorIndex[name] = i
	}

	var pairs []relPair
	for _, relName := range getSortedRelationNames(m) {
		relDef := m.Relations[relName]
		for _, from := range relDef.From {
			if excluded[from] {
				continue
			}
			targets := make([]string, 0, len(relDef.To))
			for _, to := range relDef.To {
				if !excluded[to] {
					targets = append(targets, to)
				}
			}
			if len(targets) == 0 {
				continue
			}
			pairs = append(pairs, relPair{
				source: from,
				rel:    relName,
				to:     targets,
				render: renderPlain,
				relDef: relDef,
				srcIdx: colorIndex[from],
			})
		}
	}
	return entityNames, pairs
}

func classifyRenderings(entityNames []string, pairs []relPair, noBundle, noLegend bool) {
	for i := range pairs {
		p := &pairs[i]
		n := len(p.to)
		switch {
		case n < minHubTargets:
			p.render = renderPlain
		case n >= legendTargetThreshold && !noLegend:
			p.render = renderLegend
		case n >= legendTargetThreshold && noLegend:
			p.render = renderPlain
		default:
			p.render = ""
		}
	}

	inDegree := make(map[string]int, len(entityNames))
	for _, p := range pairs {
		if p.render == renderLegend {
			continue
		}
		for _, t := range p.to {
			inDegree[t]++
		}
	}

	for i := range pairs {
		p := &pairs[i]
		if p.render != "" {
			continue
		}
		anyIsolated := false
		for _, t := range p.to {
			if inDegree[t]-1 <= 0 {
				anyIsolated = true
				break
			}
		}
		switch {
		case anyIsolated && !noBundle:
			p.render = renderHub
		case !noLegend:
			p.render = renderLegend
			for _, t := range p.to {
				inDegree[t]--
			}
		default:
			p.render = renderPlain
		}
	}
}

func visibleEntities(entityNames []string, pairs []relPair) map[string]bool {
	seenAny := make(map[string]bool, len(entityNames))
	seenDrawn := make(map[string]bool, len(entityNames))
	for _, p := range pairs {
		seenAny[p.source] = true
		for _, t := range p.to {
			seenAny[t] = true
		}
		if p.render == renderLegend {
			continue
		}
		seenDrawn[p.source] = true
		for _, t := range p.to {
			seenDrawn[t] = true
		}
	}
	visible := make(map[string]bool, len(entityNames))
	for _, name := range entityNames {
		visible[name] = !seenAny[name] || seenDrawn[name]
	}
	return visible
}

func renderLegendNode(meta *metamodel.Metamodel, entries []relPair, entityNames []string) string {
	total := len(entityNames)
	labelOf := make(map[string]string, total)
	for _, name := range entityNames {
		labelOf[name] = meta.Entities[name].Label
	}

	var tbl strings.Builder
	tbl.WriteString(`<<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4">`)
	tbl.WriteString(`<TR><TD ALIGN="LEFT"><B>Collapsed relations</B></TD></TR>`)
	for _, e := range entries {
		src := html.EscapeString(meta.Entities[e.source].Label)
		rel := html.EscapeString(e.relDef.Label)
		fmt.Fprintf(&tbl,
			`<TR><TD ALIGN="LEFT" SIDES="LTR"><B>%s</B> <I>%s</I></TD></TR>`,
			src, rel)
		effTotal := total
		srcInTargets := false
		for _, t := range e.to {
			if t == e.source {
				srcInTargets = true
				break
			}
		}
		if !srcInTargets {
			effTotal--
		}
		fmt.Fprintf(&tbl,
			`<TR><TD ALIGN="LEFT" SIDES="LBR"><I>%s</I></TD></TR>`,
			formatTargets(e.to, labelOf, effTotal, e.source, srcInTargets))
	}
	tbl.WriteString(`</TABLE>>`)

	var sb strings.Builder
	sb.WriteString("  // Legend\n")
	fmt.Fprintf(&sb,
		"  %s [shape=plaintext, margin=0, style=solid, fillcolor=white, label=%s];\n",
		legendNodeID, tbl.String())
	return sb.String()
}

func formatTargets(
	to []string,
	labelOf map[string]string,
	effectiveTotal int,
	source string,
	srcInTargets bool,
) string {
	if effectiveTotal <= 0 {
		return ""
	}
	switch {
	case len(to) == effectiveTotal:
		return "any entity"
	case len(to) >= effectiveTotal-2:
		included := make(map[string]bool, len(to))
		for _, t := range to {
			included[t] = true
		}
		names := make([]string, 0, len(labelOf))
		for n := range labelOf {
			if !srcInTargets && n == source {
				continue
			}
			names = append(names, n)
		}
		sort.Strings(names)
		missing := make([]string, 0, effectiveTotal-len(to))
		for _, n := range names {
			if !included[n] {
				missing = append(missing, html.EscapeString(labelOf[n]))
			}
		}
		return fmt.Sprintf(`any entity<BR ALIGN="LEFT"/>except %s<BR ALIGN="LEFT"/>`,
			strings.Join(missing, ", "))
	default:
		labels := make([]string, 0, len(to))
		for _, t := range to {
			labels = append(labels, html.EscapeString(labelOf[t]))
		}
		sort.Strings(labels)
		const perLine = 2
		var lines []string
		for i := 0; i < len(labels); i += perLine {
			end := i + perLine
			if end > len(labels) {
				end = len(labels)
			}
			lines = append(lines, strings.Join(labels[i:end], ", "))
		}
		return strings.Join(lines, `<BR ALIGN="LEFT"/>`) + `<BR ALIGN="LEFT"/>`
	}
}

func darkenColor(hex string) string {
	colorMap := map[string]string{
		"#e3f2fd": "#1565c0",
		"#e8f5e9": "#2e7d32",
		"#fff3e0": "#ef6c00",
		"#f3e5f5": "#6a1b9a",
		"#e0f7fa": "#00838f",
		"#fce4ec": "#ad1457",
		"#fffde7": "#f9a825",
		"#efebe9": "#4e342e",
		"white":   "#666666",
	}
	if dark, ok := colorMap[hex]; ok {
		return dark
	}
	return "#555555"
}

func buildConstraintLabel(relDef metamodel.RelationDef) string {
	label := relDef.Label
	cardinality := formatCardinality(relDef)
	if cardinality != "" {
		label += "\\n" + cardinality
	}
	return label
}

func formatCardinality(relDef metamodel.RelationDef) string {
	var parts []string
	outCard := formatCardinalityRange(relDef.MinOutgoing, relDef.MaxOutgoing)
	if outCard != "" {
		parts = append(parts, "out:"+outCard)
	}
	inCard := formatCardinalityRange(relDef.MinIncoming, relDef.MaxIncoming)
	if inCard != "" {
		parts = append(parts, "in:"+inCard)
	}
	return strings.Join(parts, " ")
}

func formatCardinalityRange(minVal, maxVal *int) string {
	if minVal == nil && maxVal == nil {
		return ""
	}
	minStr := "0"
	maxStr := "*"
	if minVal != nil {
		minStr = strconv.Itoa(*minVal)
	}
	if maxVal != nil {
		maxStr = strconv.Itoa(*maxVal)
	}
	if minStr == maxStr {
		return minStr
	}
	return minStr + ".." + maxStr
}

func getSortedEntityNames(m *metamodel.Metamodel) []string {
	names := make([]string, 0, len(m.Entities))
	for name := range m.Entities {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return natsort.Less(m.Entities[names[i]].Label, m.Entities[names[j]].Label)
	})
	return names
}

func getSortedRelationNames(m *metamodel.Metamodel) []string {
	names := make([]string, 0, len(m.Relations))
	for name := range m.Relations {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return natsort.Less(m.Relations[names[i]].Label, m.Relations[names[j]].Label)
	})
	return names
}

func getSortedTypeNames(m *metamodel.Metamodel) []string {
	names := make([]string, 0, len(m.Types))
	for name := range m.Types {
		names = append(names, name)
	}
	natsort.Strings(names)
	return names
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
