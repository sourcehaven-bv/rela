// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// schemaResourceHandler serves the schema tools (get_schema / get_metamodel /
// list_entity_types / list_relation_types) and the rela:// resource reads in
// resources.go. One merged type rather than two (the urlHelpers pattern,
// TKT-MGNE5L): both surfaces answer "describe the graph's shape and read one
// row of it" from the same two collaborators — the metamodel and the gated
// [GraphReader]. Identity still arrives on the ctx via
// Server.principalMiddleware; the handler holds no principal.
type schemaResourceHandler struct {
	store GraphReader
	meta  *metamodel.Metamodel
}

func (h schemaResourceHandler) handleGetSchema(
	_ context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	meta := h.meta
	result := map[string]any{
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
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

func (h schemaResourceHandler) handleListEntityTypes(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	type entityTypeInfo struct {
		Name       string                           `json:"name"`
		Label      string                           `json:"label"`
		IDType     string                           `json:"id_type"`
		IDPrefixes []string                         `json:"id_prefixes,omitempty"`
		Properties map[string]metamodel.PropertyDef `json:"properties"`
		Count      int                              `json:"count"`
	}

	meta := h.meta
	st := h.store
	types := meta.EntityTypes()
	natsort.Strings(types)

	result := make([]entityTypeInfo, 0, len(types))
	for _, name := range types {
		def, _ := meta.GetEntityDef(name)
		if def == nil {
			continue
		}
		count, _ := st.CountEntities(ctx, store.EntityQuery{Type: name})
		result = append(result, entityTypeInfo{
			Name:       name,
			Label:      def.GetLabel(),
			IDType:     def.GetIDType(),
			IDPrefixes: def.GetIDPrefixes(),
			Properties: def.Properties,
			Count:      count,
		})
	}

	text, err := marshalJSON(result)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

func (h schemaResourceHandler) handleListRelationTypes(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	type relationTypeInfo struct {
		Name        string   `json:"name"`
		Label       string   `json:"label"`
		From        []string `json:"from"`
		To          []string `json:"to"`
		Inverse     string   `json:"inverse,omitempty"`
		Description string   `json:"description,omitempty"`
		Count       int      `json:"count"`
	}

	meta := h.meta
	st := h.store
	types := meta.RelationTypes()
	natsort.Strings(types)

	result := make([]relationTypeInfo, 0, len(types))
	for _, name := range types {
		def, _ := meta.GetRelationDef(name)
		if def == nil {
			continue
		}
		count, _ := st.CountRelations(ctx, store.RelationQuery{Type: name})
		info := relationTypeInfo{
			Name:        name,
			Label:       def.GetLabel(),
			From:        def.GetFrom(),
			To:          def.GetTo(),
			Description: def.GetDescription(),
			Count:       count,
		}
		if def.Inverse != nil {
			info.Inverse = def.Inverse.GetID()
		}
		result = append(result, info)
	}

	text, err := marshalJSON(result)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}
