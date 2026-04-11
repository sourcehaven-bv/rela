// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func (s *Server) handleListEntities(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")
	where := request.GetString("where", "")
	limit := request.GetInt("limit", 0)
	offset := request.GetInt("offset", 0)

	snap := s.ws.Snapshot()
	var entities []*model.Entity
	if entityType != "" {
		resolved := s.resolveType(entityType)
		entities = snap.EntitiesByType(resolved)
	} else {
		entities = snap.AllEntities()
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

	snap := s.ws.Snapshot()
	entity, ok := snap.GetEntity(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	text, err := convertEntity(entity, snap, true)
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

	// Search via Bleve index (returns results sorted by relevance).
	// Fetch extra when type filtering is needed since some results may be discarded.
	snap := s.ws.Snapshot()
	words := strings.Fields(query)
	fetchLimit := limit
	if entityType != "" {
		fetchLimit = limit * 2
	}
	entities, _, err := snap.Search(words, nil, fetchLimit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Filter by type if specified.
	var results []*model.Entity
	if entityType != "" {
		resolved := s.resolveType(entityType)
		for _, e := range entities {
			if e.Type == resolved {
				results = append(results, e)
				if len(results) >= limit {
					break
				}
			}
		}
	} else {
		results = entities
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
	resolvedType, _, resolveErr := s.resolveEntityType(typeName)
	if resolveErr != nil {
		return mcp.NewToolResultError(resolveErr.Error()), nil
	}

	// Parse properties from the request
	properties := s.extractProperties(request)

	// Validate property names early for better error messages
	if errResult := s.validatePropertyNames(resolvedType, properties); errResult != nil {
		return errResult, nil
	}

	entity, _, createErr := s.ws.CreateEntity(resolvedType, workspace.CreateOptions{
		ID:         customID,
		Properties: properties,
		Content:    content,
	})
	if createErr != nil {
		return mcp.NewToolResultError(createErr.Error()), nil
	}

	snap := s.ws.Snapshot()
	text, err := convertEntity(entity, snap, false)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Created %s %s\n\n%s", resolvedType, entity.ID, text)), nil
}

func (s *Server) handleUpdateEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)

	entity, ok := s.ws.GetEntity(id)
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

	// Clone for automation (old state)
	oldEntity := entity.Clone()

	// Apply property updates
	for k, v := range properties {
		entity.Properties[k] = v
	}
	if content != "" {
		entity.Content = content
	}

	if _, updateErr := s.ws.UpdateEntity(entity, oldEntity); updateErr != nil {
		return mcp.NewToolResultError(updateErr.Error()), nil
	}

	snap := s.ws.Snapshot()
	text, convertErr := convertEntity(entity, snap, true)
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

	snap := s.ws.Snapshot()
	entity, ok := snap.GetEntity(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	// Check for relations (for better error message)
	incoming := snap.IncomingRelations(id)
	outgoing := snap.OutgoingRelations(id)
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return mcp.NewToolResultError(
			fmt.Sprintf("entity %s has %d relation(s); set cascade=true to delete them too", id, totalRelations)), nil
	}

	result, delErr := s.ws.DeleteEntity(entity.Type, id, cascade)
	if delErr != nil {
		return mcp.NewToolResultError(delErr.Error()), nil
	}

	msg := fmt.Sprintf("Deleted %s", id)
	if cascade && result.RelationsDeleted > 0 {
		msg += fmt.Sprintf(" and %d relation(s)", result.RelationsDeleted)
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
	snap := s.ws.Snapshot()
	entity, ok := snap.GetEntity(oldID)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", oldID)), nil
	}

	// Pause watcher during rename
	s.ws.PauseWatching()
	defer s.ws.ResumeWatching()

	result, renameErr := s.ws.Rename(entity.Type, oldID, newID, rename.Options{DryRun: dryRun})
	if renameErr != nil {
		return mcp.NewToolResultError(renameErr.Error()), nil
	}

	if !dryRun {
		if cacheErr := s.ws.SaveCache(); cacheErr != nil {
			s.logger.Warn("failed to save cache", "error", cacheErr)
		}
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
