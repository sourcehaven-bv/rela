// coverage-ignore: MCP resource handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/views"
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

	// Dynamic resource template: views
	s.mcp.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"rela://view/{name}/{id}",
			"View",
			mcp.WithTemplateDescription("Execute a view and return the result for a specific entity"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleReadView,
	)
}

func (s *Server) handleReadMetamodel(
	_ context.Context, _ mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
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

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	if entity.Type != entityType {
		return nil, fmt.Errorf("entity %s is type %s, not %s", id, entity.Type, entityType)
	}

	text, err := convertEntity(entity, s.graph, true)
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

func (s *Server) handleReadView(
	_ context.Context, request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Parse URI: rela://view/{name}/{id}
	parts := strings.TrimPrefix(uri, "rela://view/")
	segments := strings.SplitN(parts, "/", 2)
	if len(segments) != 2 {
		return nil, fmt.Errorf("invalid view URI: %s", uri)
	}
	viewName, entryID := segments[0], segments[1]

	viewsFile, err := s.repo.LoadViews()
	if err != nil {
		return nil, fmt.Errorf("failed to load views: %w", err)
	}

	viewDef, ok := viewsFile.GetView(viewName)
	if !ok {
		names := viewsFile.ViewNames()
		natsort.Strings(names)
		return nil, fmt.Errorf("view not found: %s (available: %s)", viewName, strings.Join(names, ", "))
	}

	meta := s.getMeta()
	if validationErr := viewDef.Validate(meta, viewName); validationErr != nil {
		return nil, fmt.Errorf("view validation failed: %w", validationErr)
	}

	engine := views.NewEngine(s.graph, meta)
	result, execErr := engine.Execute(viewDef, entryID)
	if execErr != nil {
		return nil, fmt.Errorf("view execution failed: %w", execErr)
	}

	output, fmtErr := views.Format(result, "json", s.graph, meta)
	if fmtErr != nil {
		return nil, fmt.Errorf("failed to format view output: %w", fmtErr)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     output,
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

	relation, ok := s.graph.GetEdge(fromID, relType, toID)
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
