// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/rename"
)

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
		filtered, filterErr := filterEntities(entities, where)
		if filterErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid filter: %v", filterErr)), nil
		}
		entities = filtered
	}

	sortEntitiesByID(entities)

	// Apply offset/limit
	entities = applyPagination(entities, offset, limit)

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
	id = trimID(id)

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

	type scored struct {
		entity *model.Entity
		score  float64
	}

	var candidates []*model.Entity
	if entityType != "" {
		resolved := s.resolveType(entityType)
		candidates = s.graph.NodesByType(resolved)
	} else {
		candidates = s.graph.AllNodes()
	}

	var scoredResults []scored
	for _, e := range candidates {
		sc := scoreSearch(e, queryLower)
		if sc > 0 {
			scoredResults = append(scoredResults, scored{entity: e, score: sc})
		}
	}

	// Sort by relevance score (descending), then by ID for stability
	sort.SliceStable(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		return scoredResults[i].entity.ID < scoredResults[j].entity.ID
	})

	results := make([]*model.Entity, len(scoredResults))
	for i, sr := range scoredResults {
		results[i] = sr.entity
	}
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

	// Validate property names early for better error messages
	if errResult := s.validatePropertyNames(resolvedType, properties); errResult != nil {
		return errResult, nil
	}

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
		entityID = s.generateEntityID(entityDef)
		if entityID == "" {
			return mcp.NewToolResultError(
				fmt.Sprintf("entity type %s uses manual IDs; provide an 'id' parameter", resolvedType)), nil
		}
	}

	// Create entity
	entity := model.NewEntity(entityID, resolvedType)

	// Load and apply template defaults
	template, templateErr := s.repo.LoadEntityTemplate(resolvedType)
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
		entity.SetString("status", entityDef.GetDefaultStatus(s.getMeta()))
	}

	entity.Content = content

	// Validate
	if errResult := s.validateEntity(entity); errResult != nil {
		return errResult, nil
	}

	// Write to file
	if writeErr := s.repo.WriteEntity(entity, s.getMeta()); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write entity: %v", writeErr)), nil
	}

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
	id = trimID(id)

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	properties := s.extractProperties(request)
	content := request.GetString("content", "")

	if len(properties) == 0 && content == "" {
		return mcp.NewToolResultError("no updates specified"), nil
	}

	// Validate property names early for better error messages
	if errResult := s.validatePropertyNames(entity.Type, properties); errResult != nil {
		return errResult, nil
	}

	// Apply property updates
	for k, v := range properties {
		entity.Properties[k] = v
	}
	if content != "" {
		entity.Content = content
	}

	// Validate
	if errResult := s.validateEntity(entity); errResult != nil {
		return errResult, nil
	}

	// Write to file
	if writeErr := s.repo.WriteEntity(entity, s.getMeta()); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write entity: %v", writeErr)), nil
	}

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
	id = trimID(id)
	cascade := request.GetBool("cascade", false)

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
			if delErr := s.repo.DeleteRelation(rel.From, rel.Type, rel.To); delErr != nil {
				s.logger.Printf("Warning: failed to delete relation file: %v", delErr)
			}
			s.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		}
		for _, rel := range outgoing {
			if delErr := s.repo.DeleteRelation(rel.From, rel.Type, rel.To); delErr != nil {
				s.logger.Printf("Warning: failed to delete relation file: %v", delErr)
			}
			s.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		}
	}

	// Delete entity file
	if delErr := s.repo.DeleteEntity(entity.Type, id, s.getMeta()); delErr != nil {
		s.logger.Printf("Warning: failed to delete entity file: %v", delErr)
	}

	s.graph.RemoveNode(id)
	s.saveCache()

	msg := fmt.Sprintf("Deleted %s", id)
	if cascade && totalRelations > 0 {
		msg += fmt.Sprintf(" and %d relation(s)", totalRelations)
	}
	return mcp.NewToolResultText(msg), nil
}

func (s *Server) handleRenameEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	oldID, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	oldID = trimID(oldID)

	newID, err := request.RequireString("new_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	newID = trimID(newID)

	dryRun := request.GetBool("dry_run", false)

	// Get entity to find type
	entity, ok := s.graph.GetNode(oldID)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", oldID)), nil
	}

	// Pause watcher during rename
	if s.watcher != nil {
		s.watcher.Pause()
		defer s.watcher.Resume()
	}

	opts := rename.Options{DryRun: dryRun}
	result, renameErr := rename.Rename(s.repo, s.getMeta(), s.graph, entity.Type, oldID, newID, opts)
	if renameErr != nil {
		return mcp.NewToolResultError(renameErr.Error()), nil
	}

	if !dryRun {
		s.saveCache()
	}

	return mcp.NewToolResultText(formatRenameResult(result, dryRun)), nil
}

func formatRenameResult(result *rename.Result, dryRun bool) string {
	var sb strings.Builder

	if dryRun {
		sb.WriteString("Dry run - no changes made\n\n")
	}

	sb.WriteString(fmt.Sprintf("Rename: %s → %s\n", result.OldID, result.NewID))
	sb.WriteString(fmt.Sprintf("Entity file: %s\n", result.EntityFile))

	if len(result.RelationsUpdated) > 0 {
		sb.WriteString(fmt.Sprintf("\nRelations updated (%d):\n", len(result.RelationsUpdated)))
		for _, rel := range result.RelationsUpdated {
			sb.WriteString(fmt.Sprintf("  %s --%s--> %s\n", rel.From, rel.Type, rel.To))
		}
	} else {
		sb.WriteString("\nNo relations updated\n")
	}

	if !dryRun && len(result.OldFilesDeleted) > 0 {
		sb.WriteString(fmt.Sprintf("\nOld files deleted (%d)\n", len(result.OldFilesDeleted)))
	}

	return sb.String()
}
