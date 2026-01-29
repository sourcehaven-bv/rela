// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

func (s *Server) registerTools() {
	// Entity tools
	s.mcp.AddTool(toolListEntities(), s.handleListEntities)
	s.mcp.AddTool(toolShowEntity(), s.handleShowEntity)
	s.mcp.AddTool(toolSearchEntities(), s.handleSearchEntities)
	s.mcp.AddTool(toolCreateEntity(), s.handleCreateEntity)
	s.mcp.AddTool(toolUpdateEntity(), s.handleUpdateEntity)
	s.mcp.AddTool(toolDeleteEntity(), s.handleDeleteEntity)

	// Relation tools
	s.mcp.AddTool(toolListRelations(), s.handleListRelations)
	s.mcp.AddTool(toolCreateRelation(), s.handleCreateRelation)
	s.mcp.AddTool(toolDeleteRelation(), s.handleDeleteRelation)

	// Trace tools
	s.mcp.AddTool(toolTraceFrom(), s.handleTraceFrom)
	s.mcp.AddTool(toolTraceTo(), s.handleTraceTo)
	s.mcp.AddTool(toolFindPath(), s.handleFindPath)

	// Analysis tools
	s.mcp.AddTool(toolAnalyzeOrphans(), s.handleAnalyzeOrphans)
	s.mcp.AddTool(toolAnalyzeCardinality(), s.handleAnalyzeCardinality)
	s.mcp.AddTool(toolAnalyzeProperties(), s.handleAnalyzeProperties)
	s.mcp.AddTool(toolAnalyzeValidations(), s.handleAnalyzeValidations)

	// Schema tools
	s.mcp.AddTool(toolGetMetamodel(), s.handleGetMetamodel)
	s.mcp.AddTool(toolListEntityTypes(), s.handleListEntityTypes)
	s.mcp.AddTool(toolListRelationTypes(), s.handleListRelationTypes)

	// View tools
	s.mcp.AddTool(toolListViews(), s.handleListViews)
	s.mcp.AddTool(toolExecuteView(), s.handleExecuteView)

	// Utility tools
	s.mcp.AddTool(toolRefresh(), s.handleRefresh)
	s.mcp.AddTool(toolExport(), s.handleExport)
}

// --- Tool Definitions ---

func toolListEntities() mcp.Tool {
	return mcp.NewTool("list_entities",
		mcp.WithDescription("List entities, optionally filtered by type and property expressions"),
		mcp.WithString("type", mcp.Description("Entity type to filter by (e.g. requirement, decision)")),
		mcp.WithString("where", mcp.Description("Filter expression (e.g. status=accepted, priority!=low)")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return")),
		mcp.WithNumber("offset", mcp.Description("Number of results to skip")),
	)
}

func toolShowEntity() mcp.Tool {
	return mcp.NewTool("show_entity",
		mcp.WithDescription("Get full entity details including properties, content, and relations"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entity ID (e.g. REQ-001)")),
	)
}

func toolSearchEntities() mcp.Tool {
	return mcp.NewTool("search_entities",
		mcp.WithDescription("Full-text search across entity titles and properties"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
		mcp.WithString("type", mcp.Description("Restrict search to entity type")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results (default 20)")),
	)
}

func toolCreateEntity() mcp.Tool {
	return mcp.NewTool("create_entity",
		mcp.WithDescription("Create a new entity of the specified type"),
		mcp.WithString("type", mcp.Required(), mcp.Description("Entity type (e.g. requirement, decision)")),
		mcp.WithObject("properties", mcp.Required(),
			mcp.Description("Property map (e.g. {\"title\": \"...\", \"status\": \"draft\"})")),
		mcp.WithString("content", mcp.Description("Markdown body content")),
		mcp.WithString("id", mcp.Description("Custom entity ID (auto-generated if omitted)")),
	)
}

func toolUpdateEntity() mcp.Tool {
	return mcp.NewTool("update_entity",
		mcp.WithDescription("Update an existing entity's properties or content"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entity ID to update")),
		mcp.WithObject("properties", mcp.Description("Properties to set or update")),
		mcp.WithString("content", mcp.Description("New markdown body content")),
	)
}

func toolDeleteEntity() mcp.Tool {
	return mcp.NewTool("delete_entity",
		mcp.WithDescription("Delete an entity and optionally its relations"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entity ID to delete")),
		mcp.WithBoolean("cascade", mcp.Description("Also delete all relations (default true)")),
	)
}

func toolListRelations() mcp.Tool {
	return mcp.NewTool("list_relations",
		mcp.WithDescription("List relations, optionally filtered by type, source, or target"),
		mcp.WithString("type", mcp.Description("Relation type to filter by")),
		mcp.WithString("from", mcp.Description("Source entity ID")),
		mcp.WithString("to", mcp.Description("Target entity ID")),
	)
}

func toolCreateRelation() mcp.Tool {
	return mcp.NewTool("create_relation",
		mcp.WithDescription("Create a relation between two entities"),
		mcp.WithString("from", mcp.Required(), mcp.Description("Source entity ID")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Relation type (e.g. addresses, implements)")),
		mcp.WithString("to", mcp.Required(), mcp.Description("Target entity ID")),
		mcp.WithString("content", mcp.Description("Markdown content for the relation")),
	)
}

func toolDeleteRelation() mcp.Tool {
	return mcp.NewTool("delete_relation",
		mcp.WithDescription("Delete a relation between two entities"),
		mcp.WithString("from", mcp.Required(), mcp.Description("Source entity ID")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Relation type")),
		mcp.WithString("to", mcp.Required(), mcp.Description("Target entity ID")),
	)
}

func toolTraceFrom() mcp.Tool {
	return mcp.NewTool("trace_from",
		mcp.WithDescription("Trace all dependencies from an entity (both outgoing and incoming edges)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entity ID to trace from")),
		mcp.WithNumber("max_depth", mcp.Description("Maximum trace depth (0 = unlimited)")),
	)
}

func toolTraceTo() mcp.Tool {
	return mcp.NewTool("trace_to",
		mcp.WithDescription("Trace upstream dependencies to an entity (following incoming edges)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entity ID to trace to")),
		mcp.WithNumber("max_depth", mcp.Description("Maximum trace depth (0 = unlimited)")),
	)
}

func toolFindPath() mcp.Tool {
	return mcp.NewTool("find_path",
		mcp.WithDescription("Find the shortest path between two entities"),
		mcp.WithString("from", mcp.Required(), mcp.Description("Source entity ID")),
		mcp.WithString("to", mcp.Required(), mcp.Description("Target entity ID")),
	)
}

func toolAnalyzeOrphans() mcp.Tool {
	return mcp.NewTool("analyze_orphans",
		mcp.WithDescription("Find entities with no connections (orphans)"),
		mcp.WithString("type", mcp.Description("Filter by entity type")),
	)
}

func toolAnalyzeCardinality() mcp.Tool {
	return mcp.NewTool("analyze_cardinality",
		mcp.WithDescription("Check relation cardinality constraints defined in the metamodel"),
	)
}

func toolAnalyzeProperties() mcp.Tool {
	return mcp.NewTool("analyze_properties",
		mcp.WithDescription("Validate entity property values against the metamodel schema"),
	)
}

func toolAnalyzeValidations() mcp.Tool {
	return mcp.NewTool("analyze_validations",
		mcp.WithDescription("Run custom validation rules defined in the metamodel"),
	)
}

func toolGetMetamodel() mcp.Tool {
	return mcp.NewTool("get_metamodel",
		mcp.WithDescription("Get the full metamodel definition (entity types, relations, properties, validations)"),
	)
}

func toolListEntityTypes() mcp.Tool {
	return mcp.NewTool("list_entity_types",
		mcp.WithDescription("List available entity types with their property schemas"),
	)
}

func toolListRelationTypes() mcp.Tool {
	return mcp.NewTool("list_relation_types",
		mcp.WithDescription("List available relation types with their constraints"),
	)
}

func toolListViews() mcp.Tool {
	return mcp.NewTool("list_views",
		mcp.WithDescription("List available view definitions from views.yaml"),
	)
}

func toolExecuteView() mcp.Tool {
	return mcp.NewTool("execute_view",
		mcp.WithDescription(
			"Execute a view definition to generate complete context for an entity. "+
				"Views are declarative graph traversals defined in views.yaml that efficiently "+
				"gather all related entities and their relationships around a starting entity."),
		mcp.WithString("name", mcp.Required(), mcp.Description("View name (as defined in views.yaml)")),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entry entity ID")),
		mcp.WithString("format", mcp.Description("Output format: json (default) or yaml"),
			mcp.Enum("json", "yaml")),
	)
}

func toolRefresh() mcp.Tool {
	return mcp.NewTool("refresh",
		mcp.WithDescription("Force re-sync the graph from disk (reload all entities and relations)"),
	)
}

func toolExport() mcp.Tool {
	return mcp.NewTool("export",
		mcp.WithDescription("Export entities and relations in JSON, YAML, or CSV format"),
		mcp.WithString("format", mcp.Required(),
			mcp.Description("Output format"), mcp.Enum("json", "yaml", "csv")),
		mcp.WithString("type", mcp.Description("Entity type to export (omit for all)")),
	)
}

// --- Tool Handlers ---

func (s *Server) handleListEntities(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")
	where := request.GetString("where", "")
	limit := request.GetInt("limit", 0)
	offset := request.GetInt("offset", 0)

	var entities []*model.Entity
	if entityType != "" {
		resolved := s.resolveType(entityType)
		entities = s.graph.NodesByType(resolved)
	} else {
		entities = s.graph.AllNodes()
	}

	// Apply filter
	if where != "" {
		f, err := filter.Parse(where)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid filter: %v", err)), nil
		}
		var filtered []*model.Entity
		for _, e := range entities {
			val, ok := e.Properties[f.Property]
			if !ok {
				if f.Operator == filter.OpNotEqual {
					filtered = append(filtered, e)
				}
				continue
			}
			if filter.MatchValue(val, f) {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	sortEntitiesByID(entities)

	// Apply offset/limit
	if offset > 0 {
		if offset >= len(entities) {
			entities = nil
		} else {
			entities = entities[offset:]
		}
	}
	if limit > 0 && limit < len(entities) {
		entities = entities[:limit]
	}

	text, err := convertEntitiesList(entities)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleShowEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	text, err := convertEntity(entity, s.graph, true)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleSearchEntities(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entityType := request.GetString("type", "")
	const defaultSearchLimit = 20
	limit := request.GetInt("limit", defaultSearchLimit)

	queryLower := strings.ToLower(query)

	var results []*model.Entity
	var candidates []*model.Entity
	if entityType != "" {
		resolved := s.resolveType(entityType)
		candidates = s.graph.NodesByType(resolved)
	} else {
		candidates = s.graph.AllNodes()
	}

	for _, e := range candidates {
		if matchesSearch(e, queryLower) {
			results = append(results, e)
		}
	}

	sortEntitiesByID(results)
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	text, err := convertEntitiesList(results)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleCreateEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	typeName, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	content := request.GetString("content", "")
	customID := request.GetString("id", "")

	// Resolve type
	resolvedType, entityDef, resolveErr := s.resolveEntityType(typeName)
	if resolveErr != nil {
		return mcp.NewToolResultError(resolveErr.Error()), nil
	}

	// Parse properties from the request
	properties := s.extractProperties(request)

	// Generate or validate ID
	var entityID string
	if customID != "" {
		if validErr := model.ValidateID(customID); validErr != nil {
			return mcp.NewToolResultError(validErr.Error()), nil
		}
		if _, exists := s.graph.GetNode(customID); exists {
			return mcp.NewToolResultError(fmt.Sprintf("entity with ID %s already exists", customID)), nil
		}
		entityID = customID
	} else {
		if entityDef.IsManualID() {
			return mcp.NewToolResultError(
				fmt.Sprintf("entity type %s uses manual IDs; provide an 'id' parameter", resolvedType)), nil
		}
		prefixes := entityDef.GetIDPrefixes()
		if len(prefixes) == 0 {
			return mcp.NewToolResultError(
				fmt.Sprintf("no ID prefixes defined for type %s", resolvedType)), nil
		}
		entityID = model.GenerateNextID(s.graph.AllIDs(), prefixes[0])
	}

	// Create entity
	entity := model.NewEntity(entityID, resolvedType)

	// Load and apply template defaults
	template, templateErr := markdown.LoadEntityTemplate(s.projectCtx, resolvedType)
	if templateErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load template: %v", templateErr)), nil
	}
	if template != nil {
		markdown.ApplyEntityTemplate(entity, template)
	}

	// Apply properties (override template defaults)
	for k, v := range properties {
		entity.Properties[k] = v
	}

	// Set default status if not provided
	if entity.GetString("status") == "" {
		entity.SetString("status", entityDef.GetDefaultStatus(s.meta))
	}

	entity.Content = content

	// Validate
	errs := s.meta.ValidateEntity(entity)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return mcp.NewToolResultError(fmt.Sprintf("validation errors:\n  %s", strings.Join(msgs, "\n  "))), nil
	}

	// Write to file
	plural := entityDef.GetDirPlural(resolvedType)
	filePath := s.projectCtx.EntityFilePathWithPlural(plural, entityID)
	if writeErr := markdown.WriteEntity(entity, filePath); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write entity: %v", writeErr)), nil
	}

	entity.FilePath = filePath
	s.graph.AddNode(entity)
	s.saveCache()

	text, err := convertEntity(entity, s.graph, false)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Created %s %s\n\n%s", resolvedType, entityID, text)), nil
}

func (s *Server) handleUpdateEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	properties := s.extractProperties(request)
	content := request.GetString("content", "")

	if len(properties) == 0 && content == "" {
		return mcp.NewToolResultError("no updates specified"), nil
	}

	// Apply property updates
	for k, v := range properties {
		entity.Properties[k] = v
	}
	if content != "" {
		entity.Content = content
	}

	// Validate
	errs := s.meta.ValidateEntity(entity)
	if len(errs) > 0 {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", errs[0])), nil
	}

	// Write to file
	filePath := entity.FilePath
	if filePath == "" {
		entityDef, _ := s.meta.GetEntityDef(entity.Type)
		if entityDef != nil {
			plural := entityDef.GetDirPlural(entity.Type)
			filePath = s.projectCtx.EntityFilePathWithPlural(plural, id)
		} else {
			filePath = s.projectCtx.EntityFilePath(entity.Type, id)
		}
	}

	if writeErr := markdown.WriteEntity(entity, filePath); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write entity: %v", writeErr)), nil
	}

	entity.FilePath = filePath
	s.graph.AddNode(entity)
	s.saveCache()

	text, convertErr := convertEntity(entity, s.graph, true)
	if convertErr != nil {
		return mcp.NewToolResultError(convertErr.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Updated %s\n\n%s", id, text)), nil
}

func (s *Server) handleDeleteEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	cascade := request.GetBool("cascade", true)

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	incoming := s.graph.IncomingEdges(id)
	outgoing := s.graph.OutgoingEdges(id)
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return mcp.NewToolResultError(
			fmt.Sprintf("entity %s has %d relation(s); set cascade=true to delete them too", id, totalRelations)), nil
	}

	// Delete relations
	if cascade {
		for _, rel := range incoming {
			_ = markdown.DeleteRelation(rel.FilePath)
			s.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		}
		for _, rel := range outgoing {
			_ = markdown.DeleteRelation(rel.FilePath)
			s.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		}
	}

	// Delete entity file
	filePath := entity.FilePath
	if filePath == "" {
		entityDef, _ := s.meta.GetEntityDef(entity.Type)
		if entityDef != nil {
			plural := entityDef.GetDirPlural(entity.Type)
			filePath = s.projectCtx.EntityFilePathWithPlural(plural, id)
		} else {
			filePath = s.projectCtx.EntityFilePath(entity.Type, id)
		}
	}
	_ = markdown.DeleteEntity(filePath)

	s.graph.RemoveNode(id)
	s.saveCache()

	msg := fmt.Sprintf("Deleted %s", id)
	if cascade && totalRelations > 0 {
		msg += fmt.Sprintf(" and %d relation(s)", totalRelations)
	}
	return mcp.NewToolResultText(msg), nil
}

func (s *Server) handleListRelations(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	relType := request.GetString("type", "")
	from := request.GetString("from", "")
	to := request.GetString("to", "")

	edges := s.graph.AllEdges()

	filtered := make([]*model.Relation, 0, len(edges))
	for _, e := range edges {
		if relType != "" && e.Type != relType {
			continue
		}
		if from != "" && e.From != from {
			continue
		}
		if to != "" && e.To != to {
			continue
		}
		filtered = append(filtered, e)
	}

	sortRelations(filtered)

	text, err := convertRelationsList(filtered)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleCreateRelation(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	fromID, err := request.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	relType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toID, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	content := request.GetString("content", "")

	// Check entities exist
	fromEntity, ok := s.graph.GetNode(fromID)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("source entity not found: %s", fromID)), nil
	}
	toEntity, ok := s.graph.GetNode(toID)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("target entity not found: %s", toID)), nil
	}

	// Validate relation
	if valErr := s.meta.ValidateRelation(relType, fromEntity.Type, toEntity.Type); valErr != nil {
		return mcp.NewToolResultError(valErr.Error()), nil
	}

	// Check duplicate
	if _, exists := s.graph.GetEdge(fromID, relType, toID); exists {
		return mcp.NewToolResultError(
			fmt.Sprintf("relation already exists: %s --%s--> %s", fromID, relType, toID)), nil
	}

	relation := model.NewRelation(fromID, relType, toID)
	relation.Content = content

	// Load and apply template
	template, templateErr := markdown.LoadRelationTemplate(s.projectCtx, relType)
	if templateErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load template: %v", templateErr)), nil
	}
	if template != nil {
		markdown.ApplyRelationTemplate(relation, template)
	}

	filePath := s.projectCtx.RelationFilePath(fromID, relType, toID)
	if writeErr := markdown.WriteRelation(relation, filePath); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write relation: %v", writeErr)), nil
	}

	relation.FilePath = filePath
	s.graph.AddEdge(relation)
	s.saveCache()

	return mcp.NewToolResultText(
		fmt.Sprintf("Created link: %s --%s--> %s", fromID, relType, toID)), nil
}

func (s *Server) handleDeleteRelation(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	fromID, err := request.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	relType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toID, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	relation, exists := s.graph.GetEdge(fromID, relType, toID)
	if !exists {
		return mcp.NewToolResultError(
			fmt.Sprintf("relation not found: %s --%s--> %s", fromID, relType, toID)), nil
	}

	filePath := relation.FilePath
	if filePath == "" {
		filePath = s.projectCtx.RelationFilePath(fromID, relType, toID)
	}
	_ = markdown.DeleteRelation(filePath)

	s.graph.RemoveEdge(fromID, relType, toID)
	s.saveCache()

	return mcp.NewToolResultText(
		fmt.Sprintf("Removed link: %s --%s--> %s", fromID, relType, toID)), nil
}

func (s *Server) handleTraceFrom(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	maxDepth := request.GetInt("max_depth", 0)

	if _, ok := s.graph.GetNode(id); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	result := s.graph.TraceFrom(id, maxDepth)
	if result == nil {
		return mcp.NewToolResultText("No dependencies found"), nil
	}

	text, err := convertTraceResult(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleTraceTo(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	maxDepth := request.GetInt("max_depth", 0)

	if _, ok := s.graph.GetNode(id); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	result := s.graph.TraceTo(id, maxDepth)
	if result == nil {
		return mcp.NewToolResultText("No upstream dependencies found"), nil
	}

	text, err := convertTraceResult(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleFindPath(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	from, err := request.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	to, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if _, ok := s.graph.GetNode(from); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("source entity not found: %s", from)), nil
	}
	if _, ok := s.graph.GetNode(to); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("target entity not found: %s", to)), nil
	}

	path := s.graph.FindPath(from, to)
	if path == nil {
		return mcp.NewToolResultText(
			fmt.Sprintf("No path found between %s and %s", from, to)), nil
	}

	text, err := convertPathSteps(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleAnalyzeOrphans(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")

	orphans := s.graph.FindOrphans()
	if entityType != "" {
		resolved := s.resolveType(entityType)
		var filtered []*model.Entity
		for _, o := range orphans {
			if o.Type == resolved {
				filtered = append(filtered, o)
			}
		}
		orphans = filtered
	}

	if len(orphans) == 0 {
		return mcp.NewToolResultText("No orphan entities found"), nil
	}

	sortEntitiesByID(orphans)
	text, err := convertEntitiesList(orphans)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d orphan entities:\n\n%s", len(orphans), text)), nil
}

type cardinalityViolation struct {
	EntityID string `json:"entity_id"`
	Relation string `json:"relation"`
	Message  string `json:"message"`
}

func (s *Server) handleAnalyzeCardinality(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var violations []cardinalityViolation

	for relName, relDef := range s.meta.Relations {
		violations = append(violations, s.checkCardinalityForRelation(relName, relDef)...)
	}

	if len(violations) == 0 {
		return mcp.NewToolResultText("All cardinality constraints satisfied"), nil
	}

	text, err := marshalJSON(violations)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d cardinality violations:\n\n%s", len(violations), text)), nil
}

func (s *Server) checkCardinalityForRelation(
	relName string, relDef metamodel.RelationDef,
) []cardinalityViolation {
	var violations []cardinalityViolation

	// Check source constraints (outgoing edges from source types)
	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.From, relDef.SourceMin, relDef.SourceMax, true)...)

	// Check target constraints (incoming edges to target types)
	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.To, relDef.TargetMin, relDef.TargetMax, false)...)

	return violations
}

func (s *Server) checkCardinalityBound(
	relName string, entityTypes []string, minVal, maxVal *int, outgoing bool,
) []cardinalityViolation {
	var violations []cardinalityViolation

	for _, entityType := range entityTypes {
		for _, e := range s.graph.NodesByType(entityType) {
			var edges []*model.Relation
			if outgoing {
				edges = s.graph.OutgoingEdges(e.ID)
			} else {
				edges = s.graph.IncomingEdges(e.ID)
			}
			count := countEdgesByType(edges, relName)

			direction := ""
			if !outgoing {
				direction = "incoming "
			}

			if minVal != nil && *minVal > 0 && count < *minVal {
				violations = append(violations, cardinalityViolation{
					EntityID: e.ID, Relation: relName,
					Message: fmt.Sprintf("must have at least %d %s'%s' relation(s), has %d",
						*minVal, direction, relName, count),
				})
			}
			if maxVal != nil && count > *maxVal {
				violations = append(violations, cardinalityViolation{
					EntityID: e.ID, Relation: relName,
					Message: fmt.Sprintf("has more than %d %s'%s' relation(s): %d",
						*maxVal, direction, relName, count),
				})
			}
		}
	}

	return violations
}

func (s *Server) handleAnalyzeProperties(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	type entityErrors struct {
		EntityID   string   `json:"entity_id"`
		EntityType string   `json:"entity_type"`
		Errors     []string `json:"errors"`
	}

	var allErrors []entityErrors
	for _, entity := range s.graph.AllNodes() {
		errs := s.meta.ValidateEntity(entity)
		if len(errs) > 0 {
			errStrings := make([]string, len(errs))
			for i, e := range errs {
				errStrings[i] = e.Error()
			}
			allErrors = append(allErrors, entityErrors{
				EntityID:   entity.ID,
				EntityType: entity.Type,
				Errors:     errStrings,
			})
		}
	}

	if len(allErrors) == 0 {
		return mcp.NewToolResultText("All entity properties are valid"), nil
	}

	text, err := marshalJSON(allErrors)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	errorCount := 0
	for _, ee := range allErrors {
		errorCount += len(ee.Errors)
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d property errors across %d entities:\n\n%s",
			errorCount, len(allErrors), text)), nil
}

func (s *Server) handleAnalyzeValidations(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	rules := s.meta.Validations
	if len(rules) == 0 {
		return mcp.NewToolResultText("No custom validation rules defined in metamodel"), nil
	}

	type ruleResult struct {
		Rule       string   `json:"rule"`
		Severity   string   `json:"severity"`
		Violations []string `json:"violations"`
	}

	var results []ruleResult
	for _, rule := range rules {
		violations := s.checkValidationRule(rule)
		if len(violations) > 0 {
			ids := make([]string, len(violations))
			for i, v := range violations {
				ids[i] = v.ID
			}
			results = append(results, ruleResult{
				Rule:       rule.Description,
				Severity:   rule.GetSeverity(),
				Violations: ids,
			})
		}
	}

	if len(results) == 0 {
		return mcp.NewToolResultText(
			fmt.Sprintf("All %d validation rules passed", len(rules))), nil
	}

	text, err := marshalJSON(results)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found validation issues:\n\n%s", text)), nil
}

func (s *Server) handleGetMetamodel(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	result := map[string]interface{}{
		"version":   s.meta.GetVersion(),
		"namespace": s.meta.GetNamespace(),
		"entities":  s.meta.GetEntities(),
		"relations": s.meta.GetRelations(),
		"types":     s.meta.GetTypes(),
	}
	if len(s.meta.Validations) > 0 {
		result["validations"] = s.meta.Validations
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

	types := s.meta.EntityTypes()
	sort.Strings(types)

	result := make([]entityTypeInfo, 0, len(types))
	for _, name := range types {
		def, _ := s.meta.GetEntityDef(name)
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

	types := s.meta.RelationTypes()
	sort.Strings(types)

	result := make([]relationTypeInfo, 0, len(types))
	for _, name := range types {
		def, _ := s.meta.GetRelationDef(name)
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

func (s *Server) handleListViews(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	viewsFile, err := s.loadViews()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load views: %v", err)), nil
	}

	names := viewsFile.ViewNames()
	sort.Strings(names)

	if len(names) == 0 {
		return mcp.NewToolResultText("No views defined (views.yaml not found or empty)"), nil
	}

	type viewInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		EntryType   string `json:"entry_type"`
		Parameter   string `json:"parameter"`
	}

	result := make([]viewInfo, 0, len(names))
	for _, name := range names {
		viewDef, _ := viewsFile.GetView(name)
		result = append(result, viewInfo{
			Name:        name,
			Description: viewDef.Description,
			EntryType:   viewDef.Entry.Type,
			Parameter:   viewDef.Entry.Parameter,
		})
	}

	text, err := marshalJSON(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleExecuteView(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	viewName, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entryID, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	format := request.GetString("format", "json")

	viewsFile, loadErr := s.loadViews()
	if loadErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load views: %v", loadErr)), nil
	}

	viewDef, ok := viewsFile.GetView(viewName)
	if !ok {
		names := viewsFile.ViewNames()
		sort.Strings(names)
		return mcp.NewToolResultError(
			fmt.Sprintf("view not found: %s (available: %s)", viewName, strings.Join(names, ", "))), nil
	}

	if validationErr := viewDef.Validate(s.meta, viewName); validationErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("view validation failed: %v", validationErr)), nil
	}

	engine := views.NewEngine(s.graph, s.meta)
	result, execErr := engine.Execute(viewDef, entryID)
	if execErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("view execution failed: %v", execErr)), nil
	}

	output, fmtErr := views.Format(result, format, s.graph, s.meta)
	if fmtErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format output: %v", fmtErr)), nil
	}

	return mcp.NewToolResultText(output), nil
}

func (s *Server) loadViews() (*views.File, error) {
	viewsPath := filepath.Join(s.projectCtx.Root, "views.yaml")
	return views.Load(viewsPath)
}

func (s *Server) handleRefresh(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	syncResult, err := markdown.SyncFromFiles(s.projectCtx, s.meta, s.graph)
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

// --- Helper Functions ---

func (s *Server) resolveType(typeName string) string {
	resolved := s.meta.ResolveAlias(typeName)
	if _, ok := s.meta.GetEntityDef(resolved); ok {
		return resolved
	}
	// Try stripping plural
	for _, suffix := range []string{"ies", "es", "s"} {
		replacements := map[string]string{"ies": "y", "es": "", "s": ""}
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + replacements[suffix]
			resolved = s.meta.ResolveAlias(singular)
			if _, ok := s.meta.GetEntityDef(resolved); ok {
				return resolved
			}
		}
	}
	return typeName
}

func (s *Server) resolveEntityType(typeName string) (string, *metamodel.EntityDef, error) {
	resolved := s.resolveType(typeName)
	def, ok := s.meta.GetEntityDef(resolved)
	if !ok {
		return "", nil, fmt.Errorf("unknown entity type: %s", typeName)
	}
	return resolved, def, nil
}

func (s *Server) extractProperties(request mcp.CallToolRequest) map[string]interface{} {
	args := request.GetArguments()
	propsRaw, ok := args["properties"]
	if !ok {
		return nil
	}

	switch p := propsRaw.(type) {
	case map[string]interface{}:
		return p
	case string:
		// Try to parse as JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(p), &result); err == nil {
			return result
		}
	}
	return nil
}

func (s *Server) saveCache() {
	if s.projectCtx != nil && s.graph != nil {
		if err := s.graph.SaveCache(s.projectCtx.CachePath); err != nil {
			s.logger.Printf("Warning: failed to save cache: %v", err)
		}
	}
}

func (s *Server) checkValidationRule(rule metamodel.ValidationRule) []*model.Entity {
	whenFilters, err := filter.ParseAll(rule.When)
	if err != nil {
		return nil
	}
	thenFilters, err := filter.ParseAll(rule.Then)
	if err != nil {
		return nil
	}

	var entities []*model.Entity
	if rule.EntityType != "" {
		entities = s.graph.NodesByType(rule.EntityType)
	} else {
		entities = s.graph.AllNodes()
	}

	var violations []*model.Entity
	for _, entity := range entities {
		entityDef, ok := s.meta.GetEntityDef(entity.Type)
		if !ok {
			continue
		}

		if len(whenFilters) > 0 {
			matches, matchErr := filter.MatchAll(entity, whenFilters, entityDef, s.meta)
			if matchErr != nil || !matches {
				continue
			}
		}

		satisfies, matchErr := filter.MatchAll(entity, thenFilters, entityDef, s.meta)
		if matchErr != nil || !satisfies {
			violations = append(violations, entity)
		}
	}

	return violations
}

func matchesSearch(e *model.Entity, queryLower string) bool {
	if strings.Contains(strings.ToLower(e.ID), queryLower) {
		return true
	}
	for _, v := range e.Properties {
		if str, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(str), queryLower) {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(e.Content), queryLower)
}

func countEdgesByType(edges []*model.Relation, relType string) int {
	count := 0
	for _, e := range edges {
		if e.Type == relType {
			count++
		}
	}
	return count
}
