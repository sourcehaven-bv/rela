package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

var (
	exportFormat        string
	exportWithRelations bool
	exportAll           bool
)

// ExportEntity represents an entity for export with optional relation data
type ExportEntity struct {
	ID         string                 `json:"id" yaml:"id"`
	Type       string                 `json:"type" yaml:"type"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
	Relations  *ExportRelations       `json:"relations,omitempty" yaml:"relations,omitempty"`
}

// ExportRelations contains relation data grouped by relation type
type ExportRelations struct {
	Outgoing map[string][]RelationTarget `json:"outgoing,omitempty" yaml:"outgoing,omitempty"`
	Incoming map[string][]RelationTarget `json:"incoming,omitempty" yaml:"incoming,omitempty"`
}

// RelationTarget represents a related entity
type RelationTarget struct {
	ID    string `json:"id" yaml:"id"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
}

// ExportRelation represents a relation for export
type ExportRelation struct {
	From       string                 `json:"from" yaml:"from"`
	Relation   string                 `json:"relation" yaml:"relation"`
	To         string                 `json:"to" yaml:"to"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// FullExport represents the complete export of all entities and relations
type FullExport struct {
	Entities  []ExportEntity   `json:"entities" yaml:"entities"`
	Relations []ExportRelation `json:"relations" yaml:"relations"`
}

var exportCmd = &cobra.Command{
	Use:   "export [type]",
	Short: "Export entities in JSON, CSV, or YAML format",
	Long: `Export entities to structured formats for external tool integration.

Supported formats:
  json  - JSON array of objects (default)
  csv   - CSV with headers
  yaml  - YAML format

Examples:
  # Export all controls as JSON
  rela export control --format json

  # Export controls with their relations
  rela export control --with-relations

  # Export all entities and relations
  rela export --all --format json

  # Export as CSV for spreadsheet import
  rela export control --format csv

  # Use with jq for custom reports
  rela export control --format json | jq '.[] | select(.properties.status == "draft")'

  # Use with mlr (Miller) for CSV filtering
  rela export control --format csv | mlr --csv filter '$status == "applicable"'`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine what to export
		if exportAll {
			return exportAllData()
		}

		if len(args) == 0 {
			return fmt.Errorf("please specify an entity type to export, or use --all to export everything")
		}

		typeName := args[0]
		// Handle plural form
		typeName = strings.TrimSuffix(typeName, "s")

		resolvedType, _, err := resolveEntityType(typeName)
		if err != nil {
			return err
		}

		return exportEntities(resolvedType)
	},
}

func exportEntities(entityType string) error {
	entities := g.NodesByType(entityType)

	// Sort by ID for consistent output
	sort.Slice(entities, func(i, j int) bool {
		return natsort.Less(entities[i].ID, entities[j].ID)
	})

	if len(entities) == 0 {
		// Output empty array/list in the appropriate format
		switch exportFormat {
		case "json":
			fmt.Println("[]")
		case "yaml":
			fmt.Println("[]")
		case "csv":
			// Just headers for empty CSV
			return nil
		}
		return nil
	}

	exportData := make([]ExportEntity, 0, len(entities))
	for _, e := range entities {
		exp := entityToExport(e)
		if exportWithRelations {
			exp.Relations = getEntityRelations(e.ID)
		}
		exportData = append(exportData, exp)
	}

	return writeExport(exportData, entities)
}

func exportAllData() error {
	allEntities := g.AllNodes()
	allEdges := g.AllEdges()

	// Sort entities by type, then ID
	sort.Slice(allEntities, func(i, j int) bool {
		if allEntities[i].Type != allEntities[j].Type {
			return natsort.Less(allEntities[i].Type, allEntities[j].Type)
		}
		return natsort.Less(allEntities[i].ID, allEntities[j].ID)
	})

	// Sort edges by from, relation, to
	sort.Slice(allEdges, func(i, j int) bool {
		if allEdges[i].From != allEdges[j].From {
			return natsort.Less(allEdges[i].From, allEdges[j].From)
		}
		if allEdges[i].Type != allEdges[j].Type {
			return natsort.Less(allEdges[i].Type, allEdges[j].Type)
		}
		return natsort.Less(allEdges[i].To, allEdges[j].To)
	})

	exportEntities := make([]ExportEntity, 0, len(allEntities))
	for _, e := range allEntities {
		exp := entityToExport(e)
		if exportWithRelations {
			exp.Relations = getEntityRelations(e.ID)
		}
		exportEntities = append(exportEntities, exp)
	}

	exportRelations := make([]ExportRelation, 0, len(allEdges))
	for _, r := range allEdges {
		exportRelations = append(exportRelations, ExportRelation{
			From:       r.From,
			Relation:   r.Type,
			To:         r.To,
			Properties: r.Properties,
		})
	}

	fullExport := FullExport{
		Entities:  exportEntities,
		Relations: exportRelations,
	}

	switch exportFormat {
	case "json":
		return writeJSON(fullExport)
	case "yaml":
		return writeYAML(fullExport)
	case "csv":
		return fmt.Errorf("CSV format is not supported for --all export (use JSON or YAML)")
	default:
		return fmt.Errorf("unsupported format: %s (use json, csv, or yaml)", exportFormat)
	}
}

func entityToExport(e *model.Entity) ExportEntity {
	// Create a copy of properties to include in export
	props := make(map[string]interface{})
	for k, v := range e.Properties {
		props[k] = v
	}

	return ExportEntity{
		ID:         e.ID,
		Type:       e.Type,
		Properties: props,
	}
}

func getEntityRelations(entityID string) *ExportRelations {
	outgoing := g.OutgoingEdges(entityID)
	incoming := g.IncomingEdges(entityID)

	if len(outgoing) == 0 && len(incoming) == 0 {
		return nil
	}

	relations := &ExportRelations{
		Outgoing: make(map[string][]RelationTarget),
		Incoming: make(map[string][]RelationTarget),
	}

	for _, rel := range outgoing {
		target := RelationTarget{ID: rel.To}
		if node, ok := g.GetNode(rel.To); ok {
			target.Title = node.Title()
		}
		relations.Outgoing[rel.Type] = append(relations.Outgoing[rel.Type], target)
	}

	for _, rel := range incoming {
		source := RelationTarget{ID: rel.From}
		if node, ok := g.GetNode(rel.From); ok {
			source.Title = node.Title()
		}
		relations.Incoming[rel.Type] = append(relations.Incoming[rel.Type], source)
	}

	// Remove empty maps
	if len(relations.Outgoing) == 0 {
		relations.Outgoing = nil
	}
	if len(relations.Incoming) == 0 {
		relations.Incoming = nil
	}

	return relations
}

func writeExport(data []ExportEntity, entities []*model.Entity) error {
	switch exportFormat {
	case "json":
		return writeJSON(data)
	case "yaml":
		return writeYAML(data)
	case "csv":
		return writeCSV(data, entities)
	default:
		return fmt.Errorf("unsupported format: %s (use json, csv, or yaml)", exportFormat)
	}
}

func writeJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func writeYAML(data interface{}) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

func writeCSV(data []ExportEntity, entities []*model.Entity) error {
	if len(data) == 0 {
		return nil
	}

	// Collect all property keys from all entities
	propKeys := collectPropertyKeys(entities)

	// Build headers: id, type, then properties, optionally relations
	headers := []string{"id", "type"}
	headers = append(headers, propKeys...)

	if exportWithRelations {
		headers = append(headers, "relations_outgoing", "relations_incoming")
	}

	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	if err := writer.Write(headers); err != nil {
		return err
	}

	// Write rows
	for _, exp := range data {
		row := make([]string, len(headers))
		row[0] = exp.ID
		row[1] = exp.Type

		// Fill in property values
		for i, key := range propKeys {
			if val, ok := exp.Properties[key]; ok {
				row[i+2] = formatValue(val)
			}
		}

		// Add relation columns if requested
		if exportWithRelations {
			outIdx := len(propKeys) + 2
			inIdx := len(propKeys) + 3

			if exp.Relations != nil {
				row[outIdx] = formatRelationsMap(exp.Relations.Outgoing)
				row[inIdx] = formatRelationsMap(exp.Relations.Incoming)
			}
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func collectPropertyKeys(entities []*model.Entity) []string {
	keySet := make(map[string]bool)
	for _, e := range entities {
		for k := range e.Properties {
			keySet[k] = true
		}
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	natsort.Strings(keys)

	// Move common properties to the front
	priority := []string{"title", "status", "description"}
	result := make([]string, 0, len(keys))

	for _, p := range priority {
		if keySet[p] {
			result = append(result, p)
			delete(keySet, p)
		}
	}

	for _, k := range keys {
		if keySet[k] {
			result = append(result, k)
		}
	}

	return result
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		// For other types, use JSON encoding
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func formatRelationsMap(m map[string][]RelationTarget) string {
	if len(m) == 0 {
		return ""
	}

	parts := make([]string, 0)
	// Sort relation types for consistent output
	relTypes := make([]string, 0, len(m))
	for rt := range m {
		relTypes = append(relTypes, rt)
	}
	natsort.Strings(relTypes)

	for _, rt := range relTypes {
		targets := m[rt]
		ids := make([]string, 0, len(targets))
		for _, t := range targets {
			ids = append(ids, t.ID)
		}
		parts = append(parts, fmt.Sprintf("%s:%s", rt, strings.Join(ids, ",")))
	}

	return strings.Join(parts, ";")
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format (json, csv, yaml)")
	exportCmd.Flags().BoolVar(&exportWithRelations, "with-relations", false, "Include relation data in export")
	exportCmd.Flags().BoolVar(&exportAll, "all", false, "Export all entities and relations")

	rootCmd.AddCommand(exportCmd)
}
