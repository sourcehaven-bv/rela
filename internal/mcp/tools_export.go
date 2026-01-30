// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func (s *Server) handleRefresh(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// Reload metamodel in case it changed
	newMeta, err := metamodel.Load(s.projectCtx.MetamodelPath)
	if err != nil {
		s.logger.Printf("Metamodel reload error (keeping previous version): %v", err)
	} else {
		s.setMeta(newMeta)
	}

	syncResult, err := markdown.SyncFromFiles(s.projectCtx, s.getMeta(), s.graph)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("sync failed: %v", err)), nil
	}

	s.saveCache()

	msg := fmt.Sprintf("Refreshed: %d entities, %d relations loaded",
		syncResult.EntitiesLoaded, syncResult.RelationsLoaded)
	if len(syncResult.Errors) > 0 {
		msg += fmt.Sprintf(" (%d errors)", len(syncResult.Errors))
	}
	return mcp.NewToolResultText(msg), nil
}

func (s *Server) handleExport(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	format, err := request.RequireString("format")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entityType := request.GetString("type", "")

	var entities []*model.Entity
	if entityType != "" {
		resolved := s.resolveType(entityType)
		entities = s.graph.NodesByType(resolved)
	} else {
		entities = s.graph.AllNodes()
	}
	sortEntitiesByID(entities)

	edges := s.graph.AllEdges()
	sortRelations(edges)

	switch format {
	case "json":
		return s.exportJSON(entities, edges, entityType)
	case "yaml":
		return s.exportYAML(entities, edges, entityType)
	case "csv":
		return s.exportCSV(entities)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported format: %s", format)), nil
	}
}

func (s *Server) exportJSON(
	entities []*model.Entity, edges []*model.Relation, entityType string,
) (*mcp.CallToolResult, error) {
	if entityType != "" {
		// Single type: just entities
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

	// Full export
	exportEntities := make([]map[string]interface{}, len(entities))
	for i, e := range entities {
		exportEntities[i] = map[string]interface{}{
			"id":         e.ID,
			"type":       e.Type,
			"properties": e.Properties,
		}
	}
	exportRelations := make([]map[string]interface{}, len(edges))
	for i, r := range edges {
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
	entities []*model.Entity, edges []*model.Relation, entityType string,
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
		exportRelations := make([]map[string]interface{}, len(edges))
		for i, r := range edges {
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

func (s *Server) exportCSV(entities []*model.Entity) (*mcp.CallToolResult, error) {
	if len(entities) == 0 {
		return mcp.NewToolResultText(""), nil
	}

	// Collect all property keys
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
	sort.Strings(keys)

	// Build CSV
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
