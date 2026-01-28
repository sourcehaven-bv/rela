package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var (
	schemaGraphviz    bool
	schemaConstraints bool
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

Examples:
  rela schema                    # Overview
  rela schema entities           # List entity types
  rela schema relations          # List relation types
  rela schema types              # List custom types
  rela schema entity service     # Detail for one entity type
  rela schema relation addresses # Detail for one relation type
  rela schema --graphviz         # Output as DOT format
  rela schema --graphviz --constraints  # DOT with cardinality`,
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

	// Get entity counts from graph if available
	entityCounts := make(map[string]int)
	maxCount := 0
	if g != nil {
		for _, name := range entityNames {
			count := len(g.NodesByType(name))
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

		if def.Inverse != nil && def.Inverse.Name != "" {
			out.WriteMessage("  Inverse: %s (%s)", def.Inverse.Label, def.Inverse.Name)
		}

		if def.Description != "" {
			out.WriteMessage("  Description: %s", def.Description)
		}

		// Cardinality constraints
		cardParts := []string{}
		if def.SourceMin != nil {
			cardParts = append(cardParts, fmt.Sprintf("source_min=%d", *def.SourceMin))
		}
		if def.SourceMax != nil {
			cardParts = append(cardParts, fmt.Sprintf("source_max=%d", *def.SourceMax))
		}
		if def.TargetMin != nil {
			cardParts = append(cardParts, fmt.Sprintf("target_min=%d", *def.TargetMin))
		}
		if def.TargetMax != nil {
			cardParts = append(cardParts, fmt.Sprintf("target_max=%d", *def.TargetMax))
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
			colors = append(colors, fmt.Sprintf("fill=%s", def.Color))
		}
		if def.BorderColor != "" {
			colors = append(colors, fmt.Sprintf("border=%s", def.BorderColor))
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
			if relDef.Inverse != nil && relDef.Inverse.Name != "" {
				inverseName = relDef.Inverse.Name
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

	if def.Inverse != nil && def.Inverse.Name != "" {
		out.WriteMessage("")
		out.WriteMessage("Inverse:")
		out.WriteMessage("  Name: %s", def.Inverse.Name)
		out.WriteMessage("  Label: %s", def.Inverse.Label)
	}

	if def.Description != "" {
		out.WriteMessage("")
		out.WriteMessage("Description: %s", def.Description)
	}

	// Cardinality constraints
	if def.SourceMin != nil || def.SourceMax != nil || def.TargetMin != nil || def.TargetMax != nil {
		out.WriteMessage("")
		out.WriteMessage("Cardinality Constraints:")
		if def.SourceMin != nil {
			out.WriteMessage("  Source Min: %d", *def.SourceMin)
		}
		if def.SourceMax != nil {
			out.WriteMessage("  Source Max: %d", *def.SourceMax)
		}
		if def.TargetMin != nil {
			out.WriteMessage("  Target Min: %d", *def.TargetMin)
		}
		if def.TargetMax != nil {
			out.WriteMessage("  Target Max: %d", *def.TargetMax)
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

func runSchemaGraphviz() error {
	var sb strings.Builder

	sb.WriteString("digraph metamodel {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=\"filled,rounded\", fontname=\"Helvetica\"];\n")
	sb.WriteString("  edge [fontsize=10, fontname=\"Helvetica\"];\n")
	sb.WriteString("\n")

	// Entity types as nodes
	sb.WriteString("  // Entity types\n")
	entityNames := getSortedEntityNames(meta)
	for i, name := range entityNames {
		def := meta.Entities[name]
		// Use color from metamodel if defined, otherwise use default palette
		fillColor := def.Color
		if fillColor == "" {
			fillColor = defaultEntityColors[i%len(defaultEntityColors)]
		}
		borderColor := def.BorderColor
		if borderColor == "" {
			// Derive border color from fill (darker shade)
			borderColor = darkenColor(fillColor)
		}
		sb.WriteString(fmt.Sprintf("  %s [label=\"%s\", fillcolor=\"%s\", color=\"%s\"];\n",
			name, def.Label, fillColor, borderColor))
	}
	sb.WriteString("\n")

	// Build a map of entity type -> color index for edge coloring
	entityColorIndex := make(map[string]int)
	for i, name := range entityNames {
		entityColorIndex[name] = i
	}

	// Relations as edges
	sb.WriteString("  // Relations\n")
	relationNames := getSortedRelationNames(meta)
	for _, relName := range relationNames {
		relDef := meta.Relations[relName]

		// Build edge label
		edgeLabel := relDef.Label
		if schemaConstraints {
			edgeLabel = buildConstraintLabel(relDef)
		}

		// Create edges for all from->to combinations
		for _, from := range relDef.From {
			// Edge color based on source entity type
			edgeColor := defaultEdgeColors[entityColorIndex[from]%len(defaultEdgeColors)]

			for _, to := range relDef.To {
				sb.WriteString(fmt.Sprintf("  %s -> %s [label=\"%s\", color=\"%s\", fontcolor=\"%s\"];\n",
					from, to, edgeLabel, edgeColor, edgeColor))
			}
		}
	}

	sb.WriteString("}\n")

	fmt.Print(sb.String())
	return nil
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
	// Format: source[min..max] -> target[min..max]
	var parts []string

	sourceCard := formatCardinalityRange(relDef.SourceMin, relDef.SourceMax)
	if sourceCard != "" {
		parts = append(parts, "src:"+sourceCard)
	}

	targetCard := formatCardinalityRange(relDef.TargetMin, relDef.TargetMax)
	if targetCard != "" {
		parts = append(parts, "tgt:"+targetCard)
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
		minStr = fmt.Sprintf("%d", *minVal)
	}
	if maxVal != nil {
		maxStr = fmt.Sprintf("%d", *maxVal)
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
		return m.Entities[names[i]].Label < m.Entities[names[j]].Label
	})
	return names
}

func getSortedRelationNames(m *metamodel.Metamodel) []string {
	names := make([]string, 0, len(m.Relations))
	for name := range m.Relations {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return m.Relations[names[i]].Label < m.Relations[names[j]].Label
	})
	return names
}

func getSortedTypeNames(m *metamodel.Metamodel) []string {
	names := make([]string, 0, len(m.Types))
	for name := range m.Types {
		names = append(names, name)
	}
	sort.Strings(names)
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

	rootCmd.AddCommand(schemaCmd)
}
