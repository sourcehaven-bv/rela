package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	// Entity tools
	s.mcp.AddTool(toolListEntities(), s.handleListEntities)
	s.mcp.AddTool(toolShowEntity(), s.handleShowEntity)
	s.mcp.AddTool(toolSearchEntities(), s.handleSearchEntities)
	s.mcp.AddTool(toolCreateEntity(), s.handleCreateEntity)
	s.mcp.AddTool(toolUpdateEntity(), s.handleUpdateEntity)
	s.mcp.AddTool(toolDeleteEntity(), s.handleDeleteEntity)
	s.mcp.AddTool(toolRenameEntity(), s.handleRenameEntity)

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
	s.mcp.AddTool(toolAnalyzeSchema(), s.handleAnalyzeSchema)

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

	// Lua scripting tools
	s.mcp.AddTool(toolLuaEval(), s.handleLuaEval)
	s.mcp.AddTool(toolLuaRun(), s.handleLuaRun)
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
		mcp.WithBoolean("cascade", mcp.Description("Also delete all relations (default false)")),
	)
}

func toolRenameEntity() mcp.Tool {
	return mcp.NewTool("rename_entity",
		mcp.WithDescription("Rename an entity's ID, updating all relations that reference it"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Current entity ID")),
		mcp.WithString("new_id", mcp.Required(), mcp.Description("New entity ID")),
		mcp.WithBoolean("dry_run", mcp.Description("Preview changes without applying (default false)")),
	)
}

func toolListRelations() mcp.Tool {
	return mcp.NewTool("list_relations",
		mcp.WithDescription("List relations, optionally filtered by type, source, or target"),
		mcp.WithString("type", mcp.Description("Relation type to filter by")),
		mcp.WithString("from", mcp.Description("Source entity ID")),
		mcp.WithString("to", mcp.Description("Target entity ID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return")),
		mcp.WithNumber("offset", mcp.Description("Number of results to skip")),
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

func toolAnalyzeSchema() mcp.Tool {
	return mcp.NewTool("analyze_schema",
		mcp.WithDescription("Analyze metamodel schema usage to find unused entity types, relation types, and custom types"),
		mcp.WithNumber("threshold", mcp.Description("Show types with instance count <= threshold (0 = only unused, default 0)")),
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
