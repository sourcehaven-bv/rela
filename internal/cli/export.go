package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ExportCmd exports entities in JSON, CSV, or YAML format.
type ExportCmd struct {
	Type          string `arg:"" optional:"" help:"Entity type to export."`
	Format        string `short:"f" default:"json" help:"Output format (json, csv, yaml)."`
	WithRelations bool   `name:"with-relations" help:"Include relation data in export."`
	All           bool   `help:"Export all entities and relations."`
}

// ExportEntity is the per-entity export shape.
type ExportEntity struct {
	ID         string                 `json:"id" yaml:"id"`
	Type       string                 `json:"type" yaml:"type"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
	Relations  *ExportRelations       `json:"relations,omitempty" yaml:"relations,omitempty"`
}

// ExportRelations groups relations by direction and type.
type ExportRelations struct {
	Outgoing map[string][]RelationTarget `json:"outgoing,omitempty" yaml:"outgoing,omitempty"`
	Incoming map[string][]RelationTarget `json:"incoming,omitempty" yaml:"incoming,omitempty"`
}

// RelationTarget references a related entity.
type RelationTarget struct {
	ID    string `json:"id" yaml:"id"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
}

// ExportRelation is the per-relation export shape.
type ExportRelation struct {
	From       string                 `json:"from" yaml:"from"`
	Relation   string                 `json:"relation" yaml:"relation"`
	To         string                 `json:"to" yaml:"to"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// FullExport is the --all export shape.
type FullExport struct {
	Entities  []ExportEntity   `json:"entities" yaml:"entities"`
	Relations []ExportRelation `json:"relations" yaml:"relations"`
}

// Run dispatches `rela export [type]`.
func (c *ExportCmd) Run(ctx context.Context, svc *cliServices) error {
	if c.All {
		return c.exportAllData(ctx, svc)
	}
	if c.Type == "" {
		return errors.New("please specify an entity type to export, or use --all to export everything")
	}

	typeName := strings.TrimSuffix(c.Type, "s")
	resolvedType, _, err := resolveEntityType(svc.Meta(), typeName)
	if err != nil {
		return err
	}
	return c.exportEntities(ctx, svc, resolvedType)
}

func (c *ExportCmd) exportEntities(ctx context.Context, svc *cliServices, entityType string) error {
	st := svc.Store()
	entities := make([]*entity.Entity, 0)
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: entityType}) {
		if err != nil {
			return err
		}
		entities = append(entities, e)
	}

	sort.Slice(entities, func(i, j int) bool {
		return natsort.Less(entities[i].ID, entities[j].ID)
	})

	if len(entities) == 0 {
		switch c.Format {
		case "json":
			fmt.Println("[]")
		case "yaml":
			fmt.Println("[]")
		case "csv":
			return nil
		}
		return nil
	}

	exportData := make([]ExportEntity, 0, len(entities))
	for _, e := range entities {
		exp := entityToExport(e)
		if c.WithRelations {
			exp.Relations = getEntityRelations(ctx, svc, e.ID)
		}
		exportData = append(exportData, exp)
	}

	return c.writeExport(exportData, entities)
}

func (c *ExportCmd) exportAllData(ctx context.Context, svc *cliServices) error {
	st := svc.Store()

	allEntities := make([]*entity.Entity, 0)
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			return err
		}
		allEntities = append(allEntities, e)
	}

	allEdges := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			return err
		}
		allEdges = append(allEdges, r)
	}

	sort.Slice(allEntities, func(i, j int) bool {
		if allEntities[i].Type != allEntities[j].Type {
			return natsort.Less(allEntities[i].Type, allEntities[j].Type)
		}
		return natsort.Less(allEntities[i].ID, allEntities[j].ID)
	})

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
		if c.WithRelations {
			exp.Relations = getEntityRelations(ctx, svc, e.ID)
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

	fullExport := FullExport{Entities: exportEntities, Relations: exportRelations}

	switch c.Format {
	case "json":
		return writeJSON(fullExport)
	case "yaml":
		return writeYAML(fullExport)
	case "csv":
		return errors.New("CSV format is not supported for --all export (use JSON or YAML)")
	default:
		return fmt.Errorf("unsupported format: %s (use json, csv, or yaml)", c.Format)
	}
}

func entityToExport(e *entity.Entity) ExportEntity {
	props := make(map[string]interface{})
	for k, v := range e.Properties {
		props[k] = v
	}
	return ExportEntity{ID: e.ID, Type: e.Type, Properties: props}
}

func getEntityRelations(ctx context.Context, svc *cliServices, entityID string) *ExportRelations {
	st := svc.Store()
	relations := &ExportRelations{
		Outgoing: make(map[string][]RelationTarget),
		Incoming: make(map[string][]RelationTarget),
	}
	outQ := store.RelationQuery{EntityID: entityID, Direction: store.DirectionOutgoing}
	for rel, err := range st.ListRelations(ctx, outQ) {
		if err != nil {
			break
		}
		target := RelationTarget{ID: rel.To}
		if node, err := st.GetEntity(ctx, rel.To); err == nil {
			target.Title = node.Title()
		}
		relations.Outgoing[rel.Type] = append(relations.Outgoing[rel.Type], target)
	}
	inQ := store.RelationQuery{EntityID: entityID, Direction: store.DirectionIncoming}
	for rel, err := range st.ListRelations(ctx, inQ) {
		if err != nil {
			break
		}
		source := RelationTarget{ID: rel.From}
		if node, err := st.GetEntity(ctx, rel.From); err == nil {
			source.Title = node.Title()
		}
		relations.Incoming[rel.Type] = append(relations.Incoming[rel.Type], source)
	}
	if len(relations.Outgoing) == 0 {
		relations.Outgoing = nil
	}
	if len(relations.Incoming) == 0 {
		relations.Incoming = nil
	}
	if relations.Outgoing == nil && relations.Incoming == nil {
		return nil
	}
	return relations
}

func (c *ExportCmd) writeExport(data []ExportEntity, entities []*entity.Entity) error {
	switch c.Format {
	case "json":
		return writeJSON(data)
	case "yaml":
		return writeYAML(data)
	case "csv":
		return c.writeCSV(data, entities)
	default:
		return fmt.Errorf("unsupported format: %s (use json, csv, or yaml)", c.Format)
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

func (c *ExportCmd) writeCSV(data []ExportEntity, entities []*entity.Entity) error {
	if len(data) == 0 {
		return nil
	}

	propKeys := collectPropertyKeys(entities)
	headers := []string{"id", "type"}
	headers = append(headers, propKeys...)
	if c.WithRelations {
		headers = append(headers, "relations_outgoing", "relations_incoming")
	}

	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	if err := writer.Write(headers); err != nil {
		return err
	}
	for _, exp := range data {
		row := make([]string, len(headers))
		row[0] = exp.ID
		row[1] = exp.Type
		for i, key := range propKeys {
			if val, ok := exp.Properties[key]; ok {
				row[i+2] = formatValue(val)
			}
		}
		if c.WithRelations {
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

func collectPropertyKeys(entities []*entity.Entity) []string {
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
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func formatRelationsMap(m map[string][]RelationTarget) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0)
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
