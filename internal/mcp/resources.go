// coverage-ignore: MCP resource handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerResources() {
	// Static resource: metamodel
	s.mcp.AddResource(
		mcp.NewResource(
			"rela://metamodel",
			"Metamodel Schema",
			mcp.WithResourceDescription("The project's metamodel definition (entity types, relations, properties)"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleReadMetamodel,
	)

	// Dynamic resource template: entities
	s.mcp.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"rela://entity/{type}/{id}",
			"Entity",
			mcp.WithTemplateDescription("Read a specific entity with its properties, content, and relations"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleReadEntity,
	)

	// Dynamic resource template: relations
	s.mcp.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"rela://relation/{from}/{type}/{to}",
			"Relation",
			mcp.WithTemplateDescription("Read a specific relation between two entities"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleReadRelation,
	)
}

func (s *Server) handleReadMetamodel(
	_ context.Context, _ mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	snap := s.ws.Snapshot()
	meta := snap.Meta()
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
		return nil, fmt.Errorf("failed to marshal metamodel: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "rela://metamodel",
			MIMEType: "application/json",
			Text:     text,
		},
	}, nil
}

func (s *Server) handleReadEntity(
	_ context.Context, request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Parse URI: rela://entity/{type}/{id}
	parts := strings.TrimPrefix(uri, "rela://entity/")
	segments := strings.SplitN(parts, "/", 2)
	if len(segments) != 2 {
		return nil, fmt.Errorf("invalid entity URI: %s", uri)
	}
	entityType, id := segments[0], segments[1]

	snap := s.ws.Snapshot()
	entity, ok := snap.GetEntity(id)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	if entity.Type != entityType {
		return nil, fmt.Errorf("entity %s is type %s, not %s", id, entity.Type, entityType)
	}

	text, err := convertEntity(entity, snap, true)
	if err != nil {
		return nil, fmt.Errorf("failed to convert entity: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     text,
		},
	}, nil
}

func (s *Server) handleReadRelation(
	_ context.Context, request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Parse URI: rela://relation/{from}/{type}/{to}
	parts := strings.TrimPrefix(uri, "rela://relation/")
	segments := strings.SplitN(parts, "/", 3)
	if len(segments) != 3 {
		return nil, fmt.Errorf("invalid relation URI: %s", uri)
	}
	fromID, relType, toID := segments[0], segments[1], segments[2]

	snap := s.ws.Snapshot()
	relation, ok := snap.GetRelation(fromID, relType, toID)
	if !ok {
		return nil, fmt.Errorf("relation not found: %s --%s--> %s", fromID, relType, toID)
	}

	text, err := convertRelation(relation)
	if err != nil {
		return nil, fmt.Errorf("failed to convert relation: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     text,
		},
	}, nil
}
