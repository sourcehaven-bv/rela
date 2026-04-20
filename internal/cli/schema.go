package cli

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	schemaGraphviz    bool
	schemaConstraints bool
	schemaExclude     []string
	schemaNoBundle    bool
	schemaNoLegend    bool
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "View the metamodel schema",
	Long: `Displays information about the loaded metamodel schema.

Without arguments, shows an overview of the metamodel including:
- Entity types count and list
- Relation types count and list
- Custom types count and list

Subcommands:
  overview   - Show metamodel overview (default)
  entities   - List all entity types with descriptions
  relations  - List all relation types with source/target info
  types      - List all custom types defined in the metamodel
  entity     - Show details for a specific entity type
  relation   - Show details for a specific relation type

Flags:
  --graphviz      Output metamodel as GraphViz DOT format
  --constraints   Include cardinality constraints in GraphViz output
  --exclude       Hide an entity type (repeatable; only affects --graphviz)
  --no-bundle     Disable the hub bundle for 3-4 targets with an isolated node
  --no-legend     Disable the legend table for many-target / fully-connected relations

Examples:
  rela schema                    # Overview
  rela schema entities           # List entity types
  rela schema relations          # List relation types
  rela schema types              # List custom types
  rela schema entity service     # Detail for one entity type
  rela schema relation addresses # Detail for one relation type
  rela schema --graphviz         # Output as DOT format
  rela schema --graphviz --constraints  # DOT with cardinality
  rela schema --graphviz --exclude referentie  # hide an entity type`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if schemaGraphviz {
			return runSchemaGraphviz()
		}
		return runSchemaOverview()
	},
}

var schemaOverviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Show metamodel overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSchemaOverview()
	},
}

var schemaEntitiesCmd = &cobra.Command{
	Use:   "entities",
	Short: "List all entity types",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSchemaEntities()
	},
}

var schemaRelationsCmd = &cobra.Command{
	Use:   "relations",
	Short: "List all relation types",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSchemaRelations()
	},
}

var schemaTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List custom types defined in the metamodel",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSchemaTypes()
	},
}

var schemaEntityCmd = &cobra.Command{
	Use:   "entity <name>",
	Short: "Show details for a specific entity type",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSchemaEntity(args[0])
	},
}

var schemaRelationCmd = &cobra.Command{
	Use:   "relation <name>",
	Short: "Show details for a specific relation type",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSchemaRelation(args[0])
	},
}

func runSchemaOverview() error {
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

	// Entity types with counts
	entityNames := getSortedEntityNames(meta)

	// Get entity counts from workspace if available
	entityCounts := make(map[string]int)
	maxCount := 0
	if ws != nil {
		st := ws.Store()
		for _, name := range entityNames {
			count, _ := st.CountEntities(context.Background(), store.EntityQuery{Type: name})
			entityCounts[name] = count
			if count > maxCount {
				maxCount = count
			}
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

	// Relation types
	relationNames := getSortedRelationNames(meta)
	out.WriteMessage("Relation Types (%d):", len(relationNames))
	for _, name := range relationNames {
		def := meta.Relations[name]
		out.WriteMessage("  - %s (%s)", def.Label, name)
	}
	out.WriteMessage("")

	// Custom types
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

func runSchemaEntities() error {
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

func runSchemaRelations() error {
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

		// Cardinality constraints
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

func runSchemaTypes() error {
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

func runSchemaEntity(name string) error {
	// Resolve alias
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
	writeEntityProperties(def)
	out.WriteMessage("")
	out.WriteMessage("Relations:")
	writeEntityRelations(resolved)

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

func writeEntityProperties(def *metamodel.EntityDef) {
	// Sort properties by name, with required properties first
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
		writePropertyDetail(propName, prop)
	}
}

func writePropertyDetail(propName string, prop metamodel.PropertyDef) {
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

func writeEntityRelations(resolved string) {
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

func runSchemaRelation(name string) error {
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

	// Cardinality constraints
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
	"#e3f2fd", // light blue
	"#e8f5e9", // light green
	"#fff3e0", // light orange
	"#f3e5f5", // light purple
	"#e0f7fa", // light cyan
	"#fce4ec", // light pink
	"#fffde7", // light yellow
	"#efebe9", // light brown
}

// Default color palette for relation edges
var defaultEdgeColors = []string{
	"#1976d2", // blue
	"#388e3c", // green
	"#f57c00", // orange
	"#7b1fa2", // purple
	"#0097a7", // cyan
	"#c2185b", // pink
	"#fbc02d", // yellow
	"#5d4037", // brown
}

// Classification buckets for each (sourceEntity, relation) pair.
const (
	renderPlain  = "plain"
	renderHub    = "hub"
	renderLegend = "legend"
)

// legendTargetThreshold is the target count at which a many-target relation
// unconditionally collapses into the legend (instead of a hub-bundle, which is
// only used for 3-4 targets with at least one otherwise-isolated node).
const legendTargetThreshold = 5

// relPair is a classified pair with its effective (post-exclude) target list.
type relPair struct {
	source string
	rel    string
	to     []string
	render string
	relDef metamodel.RelationDef
	srcIdx int // color index of the source in the entity palette
}

func runSchemaGraphviz() error {
	entityNames, relPairs := prepareSchemaGraph(meta)
	classifyRenderings(entityNames, relPairs)

	var sb strings.Builder
	sb.WriteString("digraph metamodel {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=\"filled,rounded\", fontname=\"Helvetica\"];\n")
	sb.WriteString("  edge [fontsize=10, fontname=\"Helvetica\"];\n")
	sb.WriteString("\n")

	// Determine which entity types are visible in the final diagram. An entity
	// is omitted when the exclude filter dropped it, or when it has no edges
	// remaining (including hub-bundled edges) and no incoming edges.
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
			name, def.Label, fillColor, borderColor)
	}
	sb.WriteString("\n")

	sb.WriteString("  // Relations\n")
	var legendEntries []relPair
	hubIdx := 0
	for _, p := range relPairs {
		edgeLabel := p.relDef.Label
		if schemaConstraints {
			edgeLabel = buildConstraintLabel(p.relDef)
		}
		edgeColor := defaultEdgeColors[entityColorIndex[p.source]%len(defaultEdgeColors)]

		switch p.render {
		case renderPlain:
			for _, to := range p.to {
				fmt.Fprintf(&sb, "  %s -> %s [label=%q, color=%q, fontcolor=%q];\n",
					p.source, to, edgeLabel, edgeColor, edgeColor)
			}
		case renderHub:
			hub := fmt.Sprintf("__hub_%d", hubIdx)
			hubIdx++
			fmt.Fprintf(&sb, "  %s [shape=point, width=0.05, height=0.05, label=\"\"];\n", hub)
			fmt.Fprintf(&sb, "  %s -> %s [label=%q, color=%q, fontcolor=%q, arrowhead=none];\n",
				p.source, hub, edgeLabel, edgeColor, edgeColor)
			for _, to := range p.to {
				fmt.Fprintf(&sb, "  %s -> %s [color=%q];\n", hub, to, edgeColor)
			}
		case renderLegend:
			legendEntries = append(legendEntries, p)
		}
	}

	if len(legendEntries) > 0 {
		sb.WriteString("\n")
		sb.WriteString(renderLegendNode(legendEntries, entityNames))
	}

	sb.WriteString("}\n")

	fmt.Print(sb.String())
	return nil
}

// prepareSchemaGraph applies --exclude filtering and returns the visible entity
// list plus one relPair per (source, relation) in the post-exclude metamodel.
func prepareSchemaGraph(m *metamodel.Metamodel) ([]string, []relPair) {
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

// classifyRenderings assigns a render bucket to each pair using the
// total-degree snapshot (computed assuming all pairs are plain) to decide
// whether a target is "otherwise connected".
func classifyRenderings(entityNames []string, pairs []relPair) {
	// Count potential edges per entity across every pair. For each pair we
	// count +1 for the source (one outgoing edge per target) and +1 for each
	// target (one incoming edge from the source). The snapshot is frozen —
	// reclassifying a pair later doesn't change the counts.
	degree := make(map[string]int, len(entityNames))
	for _, p := range pairs {
		degree[p.source] += len(p.to)
		for _, t := range p.to {
			degree[t]++
		}
	}

	for i := range pairs {
		p := &pairs[i]
		n := len(p.to)

		if schemaNoLegend && schemaNoBundle {
			p.render = renderPlain
			continue
		}
		if n >= legendTargetThreshold {
			if !schemaNoLegend {
				p.render = renderLegend
			}
			continue
		}
		if n >= 3 {
			// Count the "other" connections each target has: total minus the
			// contribution this pair makes (one incoming edge from source).
			anyIsolated := false
			for _, t := range p.to {
				if degree[t]-1 <= 0 {
					anyIsolated = true
					break
				}
			}
			// Choose the best available rendering given flag overrides.
			// Preferred: hub when any target is isolated, legend otherwise.
			// When the preferred mode is disabled, fall back to the other
			// collapse mode before giving up and drawing plain edges.
			switch {
			case anyIsolated && !schemaNoBundle:
				p.render = renderHub
			case anyIsolated && !schemaNoLegend:
				p.render = renderLegend
			case !anyIsolated && !schemaNoLegend:
				p.render = renderLegend
			default:
				p.render = renderPlain
			}
		}
	}
}

// visibleEntities returns the set of entity types that should appear in the
// DOT body. An entity is hidden only when it participates in one or more pairs
// and every such pair is collapsed into the legend — keeping it in the body
// would produce a disconnected node. Entities that belong to no pair at all
// (e.g. leaf types in a tiny metamodel) stay visible.
func visibleEntities(entityNames []string, pairs []relPair) map[string]bool {
	visible := make(map[string]bool, len(entityNames))
	participates := make(map[string]bool, len(entityNames))
	for _, name := range entityNames {
		visible[name] = true
	}
	for _, p := range pairs {
		participates[p.source] = true
		for _, t := range p.to {
			participates[t] = true
		}
		if p.render == renderLegend {
			continue
		}
		visible[p.source] = true
		for _, t := range p.to {
			visible[t] = true
		}
	}
	// For every entity that participates in pairs but only via legend-collapsed
	// ones, mark it hidden.
	for name := range participates {
		onlyLegend := true
		for _, p := range pairs {
			involved := p.source == name
			if !involved {
				for _, t := range p.to {
					if t == name {
						involved = true
						break
					}
				}
			}
			if involved && p.render != renderLegend {
				onlyLegend = false
				break
			}
		}
		if onlyLegend {
			visible[name] = false
		}
	}
	return visible
}

// renderLegendNode emits the __legend plaintext node containing an HTML-like
// TABLE with one two-row block per legend entry.
func renderLegendNode(entries []relPair, entityNames []string) string {
	total := len(entityNames)
	labelOf := make(map[string]string, total)
	for _, name := range entityNames {
		labelOf[name] = meta.Entities[name].Label
	}

	var tbl strings.Builder
	tbl.WriteString(`<<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4">`)
	tbl.WriteString(`<TR><TD ALIGN="LEFT"><B>Universal relations</B></TD></TR>`)
	for _, e := range entries {
		src := html.EscapeString(meta.Entities[e.source].Label)
		rel := html.EscapeString(e.relDef.Label)
		fmt.Fprintf(&tbl,
			`<TR><TD ALIGN="LEFT" SIDES="LTR"><B>%s</B> <I>%s</I></TD></TR>`,
			src, rel)
		fmt.Fprintf(&tbl,
			`<TR><TD ALIGN="LEFT" SIDES="LBR"><I>%s</I></TD></TR>`,
			formatTargets(e.to, labelOf, total))
	}
	tbl.WriteString(`</TABLE>>`)

	var sb strings.Builder
	sb.WriteString("  // Legend\n")
	fmt.Fprintf(&sb,
		"  __legend [shape=plaintext, margin=0, style=solid, fillcolor=white, label=%s];\n",
		tbl.String())
	return sb.String()
}

// formatTargets picks one of three rendering modes based on how close the
// target set size is to the total number of entity types:
//   - all entities (after the legend can exclude self): "any entity"
//   - at least total-2: "any entity except X, Y"
//   - otherwise: sorted labels, 2 per line, left-aligned with <BR ALIGN="LEFT"/>.
func formatTargets(to []string, labelOf map[string]string, total int) string {
	if total == 0 {
		return ""
	}
	switch {
	case len(to) == total:
		return "any entity"
	case len(to) >= total-2:
		included := make(map[string]bool, len(to))
		for _, t := range to {
			included[t] = true
		}
		missing := make([]string, 0, total-len(to))
		names := make([]string, 0, total)
		for n := range labelOf {
			names = append(names, n)
		}
		sort.Strings(names)
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

// darkenColor takes a hex color and returns a darker version for borders
func darkenColor(hex string) string {
	// Simple mapping from light pastels to darker borders
	colorMap := map[string]string{
		"#e3f2fd": "#1565c0", // light blue -> dark blue
		"#e8f5e9": "#2e7d32", // light green -> dark green
		"#fff3e0": "#ef6c00", // light orange -> dark orange
		"#f3e5f5": "#6a1b9a", // light purple -> dark purple
		"#e0f7fa": "#00838f", // light cyan -> dark cyan
		"#fce4ec": "#ad1457", // light pink -> dark pink
		"#fffde7": "#f9a825", // light yellow -> dark yellow
		"#efebe9": "#4e342e", // light brown -> dark brown
		"white":   "#666666", // white -> gray
	}
	if dark, ok := colorMap[hex]; ok {
		return dark
	}
	// Default to a generic dark gray if color not in map
	return "#555555"
}

func buildConstraintLabel(relDef metamodel.RelationDef) string {
	label := relDef.Label

	// Add cardinality if defined
	cardinality := formatCardinality(relDef)
	if cardinality != "" {
		label += "\\n" + cardinality
	}

	return label
}

func formatCardinality(relDef metamodel.RelationDef) string {
	// Format: out[min..max] in[min..max]
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

// Helper functions

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

func init() {
	schemaCmd.AddCommand(schemaOverviewCmd)
	schemaCmd.AddCommand(schemaEntitiesCmd)
	schemaCmd.AddCommand(schemaRelationsCmd)
	schemaCmd.AddCommand(schemaTypesCmd)
	schemaCmd.AddCommand(schemaEntityCmd)
	schemaCmd.AddCommand(schemaRelationCmd)

	schemaCmd.Flags().BoolVar(&schemaGraphviz, "graphviz", false, "Output metamodel as GraphViz DOT format")
	schemaCmd.Flags().BoolVar(&schemaConstraints, "constraints", false, "Include cardinality constraints in GraphViz output")
	schemaCmd.Flags().StringSliceVar(&schemaExclude, "exclude", nil,
		"Hide an entity type in --graphviz output (repeatable)")
	schemaCmd.Flags().BoolVar(&schemaNoBundle, "no-bundle", false,
		"Disable the hub bundle for 3-4 targets with an isolated node")
	schemaCmd.Flags().BoolVar(&schemaNoLegend, "no-legend", false,
		"Disable the legend table for many-target / fully-connected relations")

	rootCmd.AddCommand(schemaCmd)
}
