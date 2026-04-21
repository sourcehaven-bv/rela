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

// Thresholds for the (source, relation) classifier:
//   - fewer than minHubTargets targets always render as plain edges
//   - minHubTargets..legendTargetThreshold-1 may hub-bundle when at least one
//     target is otherwise isolated; otherwise they collapse into the legend
//   - legendTargetThreshold or more unconditionally collapse into the legend
const (
	minHubTargets         = 3
	legendTargetThreshold = 5
)

// legendNodeID and hubIDPrefix are reserved identifiers used for synthetic
// nodes in the generated DOT. They are intentionally underscore-prefixed and
// the classifier assumes no user-defined entity type uses these names.
const (
	legendNodeID = "__legend"
	hubIDPrefix  = "__hub_"
)

// dotID returns a DOT-safe identifier for an arbitrary string. DOT accepts
// unquoted identifiers only when they match [_A-Za-z][_0-9A-Za-z]*. Anything
// else — including hyphens, dots, spaces, or an empty string — must be
// double-quoted. The function always emits the quoted form when the input
// needs it, otherwise returns the raw identifier untouched.
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
			// Escape backslashes and quotes per DOT's quoted-string syntax.
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
			dotID(name), def.Label, fillColor, borderColor)
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

// classifyRenderings assigns a render bucket to each pair. The "otherwise
// connected" check — whether a target has any edge to non-legend pairs — is
// computed in two passes to avoid a snapshot-based lie: if pair A targets T
// and pair B (≥5 targets) also targets T, classifying A as plain or hub
// against a snapshot that assumed B was plain would be wrong, because B will
// actually collapse into the legend and contribute nothing to T's connectedness.
//
//  1. Unconditional-legend pairs (n ≥ legendTargetThreshold) are classified
//     first and contribute 0 edges.
//  2. The incoming-degree is computed from the remaining pairs, modeling the
//     final diagram before the ambiguous 3-4 pairs are classified.
//  3. Each 3-4 pair is classified against that degree, then if it ends up as
//     legend its targets' degrees are decremented (because its edges also
//     disappear) so later pairs see the true reduced degree.
func classifyRenderings(entityNames []string, pairs []relPair) {
	// Pass 1: handle the always-legend / always-plain buckets.
	for i := range pairs {
		p := &pairs[i]
		n := len(p.to)
		switch {
		case n < minHubTargets:
			p.render = renderPlain
		case n >= legendTargetThreshold && !schemaNoLegend:
			p.render = renderLegend
		case n >= legendTargetThreshold && schemaNoLegend:
			p.render = renderPlain
		default:
			p.render = "" // deferred to pass 2
		}
	}

	// Pass 2: incoming-degree for every target, counting only pairs whose edges
	// will actually be drawn (plain + deferred). Legend-classified pairs are
	// excluded because their edges never appear.
	inDegree := make(map[string]int, len(entityNames))
	for _, p := range pairs {
		if p.render == renderLegend {
			continue
		}
		for _, t := range p.to {
			inDegree[t]++
		}
	}

	// Pass 3: classify the deferred pairs, adjusting inDegree on-the-fly when
	// a deferred pair collapses (so downstream classifications see the true
	// connectedness).
	for i := range pairs {
		p := &pairs[i]
		if p.render != "" {
			continue
		}
		// Count "other" incoming edges for each target: inDegree[t] counts
		// this pair's own edge plus edges from other non-legend pairs, so
		// subtract 1 to get the "otherwise connected" signal.
		anyIsolated := false
		for _, t := range p.to {
			if inDegree[t]-1 <= 0 {
				anyIsolated = true
				break
			}
		}
		// Prefer hub for isolated targets, legend otherwise. Respect --no-*
		// fallbacks: when the preferred collapse is disabled, fall back to
		// the other collapse mode before giving up and drawing plain edges.
		switch {
		case anyIsolated && !schemaNoBundle:
			p.render = renderHub
		case !schemaNoLegend:
			p.render = renderLegend
			// Legend collapse makes this pair's edges disappear too — keep
			// inDegree honest so later deferred pairs see reality.
			for _, t := range p.to {
				inDegree[t]--
			}
		default:
			p.render = renderPlain
		}
	}
}

// visibleEntities returns the set of entity types that should appear in the
// DOT body. An entity is hidden only when it participates in one or more pairs
// and every such pair is collapsed into the legend — keeping it in the body
// would produce a disconnected node. Entities that belong to no pair at all
// (e.g. leaf types in a tiny metamodel) stay visible.
func visibleEntities(entityNames []string, pairs []relPair) map[string]bool {
	// Two sets: every entity that appears in any pair, and every entity that
	// appears in a pair whose edges will actually be drawn. The final decision
	// is: hidden iff in seenAny but not in seenDrawn.
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
	tbl.WriteString(`<TR><TD ALIGN="LEFT"><B>Collapsed relations</B></TD></TR>`)
	for _, e := range entries {
		src := html.EscapeString(meta.Entities[e.source].Label)
		rel := html.EscapeString(e.relDef.Label)
		fmt.Fprintf(&tbl,
			`<TR><TD ALIGN="LEFT" SIDES="LTR"><B>%s</B> <I>%s</I></TD></TR>`,
			src, rel)
		// Effective total excludes the source itself when it's not in its own
		// target set — otherwise the legend would read "any entity except Src",
		// listing Src as an exception against its own row.
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

// formatTargets picks one of three rendering modes based on how close the
// target set size is to the effective total number of targetable entity types:
//   - equals effectiveTotal: "any entity"
//   - at least effectiveTotal-2: "any entity except X, Y"
//   - otherwise: sorted labels, 2 per line, left-aligned with <BR ALIGN="LEFT"/>.
//
// When `srcInTargets` is false, the source entity is not a valid target and
// must be excluded from the "except" complement. `source` is the id of that
// entity so the complement iteration can skip it.
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
