// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func (s *Server) handleExport(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	format, err := request.RequireString("format")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entityType := request.GetString("type", "")

	st := s.ws.Store()
	q := store.EntityQuery{}
	if entityType != "" {
		q.Type = s.resolveType(entityType)
	}

	entities := make([]*entity.Entity, 0)
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		entities = append(entities, e)
	}
	sortStoreEntitiesByID(entities)

	relations := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		relations = append(relations, r)
	}
	sortStoreRelations(relations)

	switch format {
	case "json":
		return s.exportJSON(entities, relations, entityType)
	case "yaml":
		return s.exportYAML(entities, relations, entityType)
	case "csv":
		return s.exportCSV(entities)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported format: %s", format)), nil
	}
}

func (s *Server) exportJSON(
	entities []*entity.Entity, relations []*entity.Relation, entityType string,
) (*mcp.CallToolResult, error) {
	if entityType != "" {
		summaries := make([]map[string]interface{}, len(entities))
		for i, e := range entities {
			summaries[i] = map[string]interface{}{
				"id":         e.ID,
				"type":       e.Type,
				"properties": e.Properties,
			}
		}
		text, err := marshalJSON(summaries)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	}

	exportEntities := make([]map[string]interface{}, len(entities))
	for i, e := range entities {
		exportEntities[i] = map[string]interface{}{
			"id":         e.ID,
			"type":       e.Type,
			"properties": e.Properties,
		}
	}
	exportRelations := make([]map[string]interface{}, len(relations))
	for i, r := range relations {
		exportRelations[i] = map[string]interface{}{
			"from":     r.From,
			"relation": r.Type,
			"to":       r.To,
		}
	}

	text, err := marshalJSON(map[string]interface{}{
		"entities":  exportEntities,
		"relations": exportRelations,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) exportYAML(
	entities []*entity.Entity, relations []*entity.Relation, entityType string,
) (*mcp.CallToolResult, error) {
	var data interface{}
	if entityType != "" {
		summaries := make([]map[string]interface{}, len(entities))
		for i, e := range entities {
			summaries[i] = map[string]interface{}{
				"id":         e.ID,
				"type":       e.Type,
				"properties": e.Properties,
			}
		}
		data = summaries
	} else {
		exportRelations := make([]map[string]interface{}, len(relations))
		for i, r := range relations {
			exportRelations[i] = map[string]interface{}{
				"from":     r.From,
				"relation": r.Type,
				"to":       r.To,
			}
		}
		exportEntities := make([]map[string]interface{}, len(entities))
		for i, e := range entities {
			exportEntities[i] = map[string]interface{}{
				"id":         e.ID,
				"type":       e.Type,
				"properties": e.Properties,
			}
		}
		data = map[string]interface{}{
			"entities":  exportEntities,
			"relations": exportRelations,
		}
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("YAML encoding failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func (s *Server) exportCSV(entities []*entity.Entity) (*mcp.CallToolResult, error) {
	if len(entities) == 0 {
		return mcp.NewToolResultText(""), nil
	}

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

	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	headers := append([]string{"id", "type"}, keys...)
	_ = writer.Write(headers)

	for _, e := range entities {
		row := make([]string, len(headers))
		row[0] = e.ID
		row[1] = e.Type
		for i, k := range keys {
			if v, ok := e.Properties[k]; ok {
				row[i+2] = fmt.Sprintf("%v", v)
			}
		}
		_ = writer.Write(row)
	}
	writer.Flush()

	return mcp.NewToolResultText(buf.String()), nil
}
