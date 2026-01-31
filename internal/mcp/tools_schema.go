// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func (s *Server) handleGetMetamodel(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	meta := s.getMeta()
	result := map[string]interface{}{
		"version":   meta.GetVersion(),
		"namespace": meta.GetNamespace(),
		"entities":  meta.GetEntities(),
		"relations": meta.GetRelations(),
		"types":     meta.GetTypes(),
	}
	if len(meta.Validations) > 0 {
		result["validations"] = meta.Validations
	}

	text, err := marshalJSON(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleListEntityTypes(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	type entityTypeInfo struct {
		Name       string                           `json:"name"`
		Label      string                           `json:"label"`
		IDType     string                           `json:"id_type"`
		IDPrefixes []string                         `json:"id_prefixes,omitempty"`
		Properties map[string]metamodel.PropertyDef `json:"properties"`
		Count      int                              `json:"count"`
	}

	meta := s.getMeta()
	types := meta.EntityTypes()
	sort.Strings(types)

	result := make([]entityTypeInfo, 0, len(types))
	for _, name := range types {
		def, _ := meta.GetEntityDef(name)
		if def == nil {
			continue
		}
		result = append(result, entityTypeInfo{
			Name:       name,
			Label:      def.GetLabel(),
			IDType:     def.GetIDType(),
			IDPrefixes: def.GetIDPrefixes(),
			Properties: def.Properties,
			Count:      len(s.graph.NodesByType(name)),
		})
	}

	text, err := marshalJSON(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleListRelationTypes(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	type relationTypeInfo struct {
		Name        string   `json:"name"`
		Label       string   `json:"label"`
		From        []string `json:"from"`
		To          []string `json:"to"`
		Inverse     string   `json:"inverse,omitempty"`
		Description string   `json:"description,omitempty"`
		Count       int      `json:"count"`
	}

	meta := s.getMeta()
	types := meta.RelationTypes()
	sort.Strings(types)

	result := make([]relationTypeInfo, 0, len(types))
	for _, name := range types {
		def, _ := meta.GetRelationDef(name)
		if def == nil {
			continue
		}
		info := relationTypeInfo{
			Name:        name,
			Label:       def.GetLabel(),
			From:        def.GetFrom(),
			To:          def.GetTo(),
			Description: def.GetDescription(),
			Count:       len(s.graph.RelationsOfType(name)),
		}
		if def.Inverse != nil {
			info.Inverse = def.Inverse.GetID()
		}
		result = append(result, info)
	}

	text, err := marshalJSON(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}
