// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"slices"
	"strings"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// schemaResourceHandler serves the schema tool and the rela:// resource reads
// in resources.go. One merged type rather than two (the urlHelpers pattern,
// TKT-MGNE5L): both surfaces answer "describe the graph's shape and read one
// row of it" from the same two collaborators — the metamodel and the gated
// [GraphReader]. Identity still arrives on the ctx via
// Server.principalMiddleware; the handler holds no principal.
type schemaResourceHandler struct {
	store GraphReader
	meta  *metamodel.Metamodel
	world store.WorldScope // [Deps.World]
}

// The schema tool answers in two sizes. The overview is one short record per
// type, so an agent can orient itself for a few hundred tokens. The detail
// covers one type, which is what an agent needs before a create or a filter.
// The whole raw metamodel stays available as the rela://metamodel resource.
//
// The DTOs below exist so the output carries only set fields: the metamodel
// structs have no JSON tags and serialize every zero value in PascalCase,
// which made the old list_entity_types answer over 100 KB on a real project.

type entityTypeSummary struct {
	Name       string   `json:"name"`
	Label      string   `json:"label,omitempty"`
	IDPrefixes []string `json:"id_prefixes,omitempty"`
	Count      *int     `json:"count,omitempty"`
}

type relationTypeSummary struct {
	Name  string   `json:"name"`
	From  []string `json:"from"`
	To    []string `json:"to"`
	Count *int     `json:"count,omitempty"`
}

type propertySchema struct {
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	List     bool   `json:"list,omitempty"`
	Unique   bool   `json:"unique,omitempty"`
	Computed bool   `json:"computed,omitempty"`
	// Rollup marks a read-only aggregate over related entities, computed
	// on read with the caller's access (TKT-6I2OPG).
	Rollup      bool     `json:"rollup,omitempty"`
	Values      []string `json:"values,omitempty"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	// Min and Max are an integer property's declared bounds (TKT-BRAM1A).
	// A value outside them is written with a warning. Nil: accepted — unset,
	// and always nil on a non-integer property.
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}

// relationEnd is one relation type as seen from an entity type: the relation
// and the types at its other end.
type relationEnd struct {
	Relation string   `json:"relation"`
	Types    []string `json:"types"`
}

type entityTypeDetail struct {
	Name            string                    `json:"name"`
	Label           string                    `json:"label,omitempty"`
	Description     string                    `json:"description,omitempty"`
	IDType          string                    `json:"id_type"`
	IDPrefixes      []string                  `json:"id_prefixes,omitempty"`
	DisplayProperty string                    `json:"display_property,omitempty"`
	Count           *int                      `json:"count,omitempty"`
	Properties      map[string]propertySchema `json:"properties"`
	Outgoing        []relationEnd             `json:"outgoing,omitempty"`
	Incoming        []relationEnd             `json:"incoming,omitempty"`
	Validations     []string                  `json:"validations,omitempty"`
}

type relationTypeDetail struct {
	Name        string                    `json:"name"`
	Label       string                    `json:"label,omitempty"`
	Description string                    `json:"description,omitempty"`
	From        []string                  `json:"from"`
	To          []string                  `json:"to"`
	Inverse     string                    `json:"inverse,omitempty"`
	Symmetric   bool                      `json:"symmetric,omitempty"`
	Owning      bool                      `json:"owning,omitempty"`
	MinOutgoing *int                      `json:"min_outgoing,omitempty"`
	MaxOutgoing *int                      `json:"max_outgoing,omitempty"`
	MinIncoming *int                      `json:"min_incoming,omitempty"`
	MaxIncoming *int                      `json:"max_incoming,omitempty"`
	Properties  map[string]propertySchema `json:"properties,omitempty"`
	Count       *int                      `json:"count,omitempty"`
}

func (h schemaResourceHandler) handleSchema(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	typeArg := strings.TrimSpace(newToolRequest(request).GetString("type", ""))

	result, ok := h.describe(ctx, typeArg)
	if !ok {
		return errorResult("unknown entity or relation type: " + typeArg), nil
	}

	text, err := marshalJSON(result)
	if err != nil { // coverage-ignore: defensive: the schema DTOs hold strings, ints and bools; cannot fail.
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

// describe returns the overview when typeArg is empty, else the detail of the
// entity or relation type it names. ok is false for an unknown type.
func (h schemaResourceHandler) describe(ctx context.Context, typeArg string) (any, bool) {
	if typeArg == "" {
		return h.overview(ctx), true
	}
	// Exact names first, so a relation named like a plural (`tests`) is not
	// read as the entity type its singular names.
	if def, ok := h.meta.GetEntityDef(typeArg); ok {
		return h.entityDetail(ctx, typeArg, def), true
	}
	if def, ok := h.meta.GetRelationDef(typeArg); ok {
		return h.relationDetail(ctx, typeArg, def), true
	}
	resolved := typeResolver{meta: h.meta}.resolveType(typeArg)
	if def, ok := h.meta.GetEntityDef(resolved); ok {
		return h.entityDetail(ctx, resolved, def), true
	}
	return nil, false
}

// overview returns one summary record per entity and relation type.
func (h schemaResourceHandler) overview(ctx context.Context) any {
	entityTypes := h.meta.EntityTypes()
	natsort.Strings(entityTypes)
	entities := make([]entityTypeSummary, 0, len(entityTypes))
	for _, name := range entityTypes {
		def, _ := h.meta.GetEntityDef(name)
		if def == nil {
			continue
		}
		count := countOrNil(h.store.CountEntities(ctx, store.EntityQuery{Type: name, Faces: store.InWorld(h.world)}))
		entities = append(entities, entityTypeSummary{
			Name:       name,
			Label:      labelUnlessName(def.Label, name),
			IDPrefixes: def.GetIDPrefixes(),
			Count:      count,
		})
	}

	relationTypes := h.meta.RelationTypes()
	natsort.Strings(relationTypes)
	relations := make([]relationTypeSummary, 0, len(relationTypes))
	for _, name := range relationTypes {
		def, _ := h.meta.GetRelationDef(name)
		if def == nil {
			continue
		}
		count := countOrNil(h.store.CountRelations(ctx, store.RelationQuery{Type: name}))
		relations = append(relations, relationTypeSummary{
			Name: name, From: def.From, To: def.To, Count: count,
		})
	}

	return struct {
		EntityTypes   []entityTypeSummary   `json:"entity_types"`
		RelationTypes []relationTypeSummary `json:"relation_types"`
	}{entities, relations}
}

// entityDetail describes one entity type: its properties, the relations it
// can take part in, and the custom validation rules scoped to it.
func (h schemaResourceHandler) entityDetail(
	ctx context.Context, name string, def *metamodel.EntityDef,
) entityTypeDetail {
	count := countOrNil(h.store.CountEntities(ctx, store.EntityQuery{Type: name, Faces: store.InWorld(h.world)}))
	detail := entityTypeDetail{
		Name:            name,
		Label:           labelUnlessName(def.Label, name),
		Description:     def.Description,
		IDType:          def.GetIDType(),
		IDPrefixes:      def.GetIDPrefixes(),
		DisplayProperty: def.DisplayProperty,
		Count:           count,
		Properties:      h.propertySchemas(def.Properties),
	}

	relationTypes := h.meta.RelationTypes()
	natsort.Strings(relationTypes)
	for _, relName := range relationTypes {
		rel, _ := h.meta.GetRelationDef(relName)
		if rel == nil {
			continue
		}
		if slices.Contains(rel.From, name) {
			detail.Outgoing = append(detail.Outgoing, relationEnd{Relation: relName, Types: rel.To})
		}
		if slices.Contains(rel.To, name) {
			detail.Incoming = append(detail.Incoming, relationEnd{Relation: relName, Types: rel.From})
		}
	}

	// A rule with no entity type applies to every type.
	for _, rule := range h.meta.Validations {
		if rule.EntityType != name && rule.EntityType != "" {
			continue
		}
		text := rule.Description
		if text == "" {
			text = rule.Name
		}
		detail.Validations = append(detail.Validations, text)
	}
	return detail
}

// countOrNil is a schema count, or nil when the count failed. A nil count is
// left out of the JSON: a gated count that failed must not read as zero rows
// (TKT-QZTROQ).
func countOrNil(n int, err error) *int {
	if err != nil {
		return nil
	}
	return &n
}

// relationDetail describes one relation type.
func (h schemaResourceHandler) relationDetail(
	ctx context.Context, name string, def *metamodel.RelationDef,
) relationTypeDetail {
	count := countOrNil(h.store.CountRelations(ctx, store.RelationQuery{Type: name}))
	detail := relationTypeDetail{
		Name:        name,
		Label:       labelUnlessName(def.Label, name),
		Description: def.Description,
		From:        def.From,
		To:          def.To,
		Symmetric:   def.Symmetric,
		Owning:      def.Owning,
		MinOutgoing: def.MinOutgoing,
		MaxOutgoing: def.MaxOutgoing,
		MinIncoming: def.MinIncoming,
		MaxIncoming: def.MaxIncoming,
		Count:       count,
	}
	if def.Inverse != nil {
		detail.Inverse = def.Inverse.GetID()
	}
	if len(def.Properties) > 0 {
		detail.Properties = h.propertySchemas(def.Properties)
	}
	return detail
}

// propertySchemas converts property definitions to their compact form. A
// property typed with a custom enum type lists that type's values, so the
// caller does not need a second lookup to learn what it may write.
func (h schemaResourceHandler) propertySchemas(defs map[string]metamodel.PropertyDef) map[string]propertySchema {
	out := make(map[string]propertySchema, len(defs))
	for name, pd := range defs {
		values := pd.Values
		if len(values) == 0 {
			if ct, ok := h.meta.Types[pd.Type]; ok {
				values = ct.Values
			}
		}
		ps := propertySchema{
			Type:        pd.Type,
			Required:    pd.Required,
			List:        pd.List,
			Unique:      pd.Unique,
			Computed:    pd.Computed != "",
			Rollup:      pd.Rollup != nil,
			Values:      values,
			Default:     pd.Default,
			Description: pd.Description,
		}
		if pd.Type == metamodel.PropertyTypeInteger {
			ps.Min, ps.Max = pd.Min, pd.Max
		}
		out[name] = ps
	}
	return out
}

// labelUnlessName drops a label that only repeats the type name.
func labelUnlessName(label, name string) string {
	if strings.EqualFold(label, name) {
		return ""
	}
	return label
}
