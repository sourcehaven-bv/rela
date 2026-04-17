// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func (s *Server) handleListEntities(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")
	where := request.GetString("where", "")
	limit := request.GetInt("limit", 0)
	offset := request.GetInt("offset", 0)

	st := s.ws.Store()
	q := store.EntityQuery{}
	if entityType != "" {
		q.Type = s.resolveType(entityType)
	}

	var entities []*entity.Entity
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		entities = append(entities, e)
	}

	// Apply filter
	if where != "" {
		filtered, filterErr := filterStoreEntities(entities, where)
		if filterErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid filter: %v", filterErr)), nil
		}
		entities = filtered
	}

	sortStoreEntitiesByID(entities)

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

	summaries := make([]map[string]interface{}, len(entities))
	for i, e := range entities {
		summaries[i] = convertStoreEntitySummary(e)
	}
	text, err := marshalJSON(summaries)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleShowEntity(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)

	st := s.ws.Store()
	e, getErr := st.GetEntity(ctx, id)
	if getErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	text, err := convertStoreEntity(e, st, true)
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
	words := strings.Fields(query)
	fetchLimit := limit
	if entityType != "" {
		fetchLimit = limit * 2
	}
	entities, _, err := s.ws.Search(words, nil, fetchLimit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Filter by type if specified and convert to summaries.
	resolved := ""
	if entityType != "" {
		resolved = s.resolveType(entityType)
	}
	var summaries []map[string]interface{}
	for _, e := range entities {
		if resolved != "" && e.Type != resolved {
			continue
		}
		summary := map[string]interface{}{"id": e.ID, "type": e.Type}
		if title := e.Title(); title != "" {
			summary["title"] = title
		}
		if status := e.GetString("status"); status != "" {
			summary["status"] = status
		}
		summaries = append(summaries, summary)
		if limit > 0 && len(summaries) >= limit {
			break
		}
	}

	text, err := marshalJSON(summaries)
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

	created, _, createErr := s.ws.CreateEntity(resolvedType, workspace.CreateOptions{
		ID:         customID,
		Properties: properties,
		Content:    content,
	})
	if createErr != nil {
		return mcp.NewToolResultError(createErr.Error()), nil
	}

	st := s.ws.Store()
	e, _ := st.GetEntity(context.Background(), created.ID)
	if e == nil {
		// Fallback: return minimal info
		return mcp.NewToolResultText(fmt.Sprintf("Created %s %s", resolvedType, created.ID)), nil
	}

	text, err := convertStoreEntity(e, st, false)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Created %s %s\n\n%s", resolvedType, created.ID, text)), nil
}

func (s *Server) handleUpdateEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)

	e, ok := s.ws.GetEntity(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	properties := s.extractProperties(request)
	content := request.GetString("content", "")

	if len(properties) == 0 && content == "" {
		return mcp.NewToolResultError("no updates specified"), nil
	}

	// Validate property names early for better error messages
	if errResult := s.validatePropertyNames(e.Type, properties); errResult != nil {
		return errResult, nil
	}

	// Clone for automation (old state)
	oldEntity := e.Clone()

	// Apply property updates
	for k, v := range properties {
		e.Properties[k] = v
	}
	if content != "" {
		e.Content = content
	}

	if _, updateErr := s.ws.UpdateEntity(model.EntityFromDomain(e), model.EntityFromDomain(oldEntity)); updateErr != nil {
		return mcp.NewToolResultError(updateErr.Error()), nil
	}

	st := s.ws.Store()
	updated, _ := st.GetEntity(context.Background(), id)
	if updated == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Updated %s", id)), nil
	}

	text, convertErr := convertStoreEntity(updated, st, true)
	if convertErr != nil {
		return mcp.NewToolResultError(convertErr.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Updated %s\n\n%s", id, text)), nil
}

func (s *Server) handleDeleteEntity(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)
	cascade := request.GetBool("cascade", false)

	st := s.ws.Store()
	e, getErr := st.GetEntity(ctx, id)
	if getErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	// Check for relations (for better error message)
	if !cascade {
		n, _ := st.CountRelations(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionBoth})
		if n > 0 {
			return mcp.NewToolResultError(
				fmt.Sprintf("entity %s has %d relation(s); set cascade=true to delete them too", id, n)), nil
		}
	}

	result, delErr := s.ws.DeleteEntity(e.Type, id, cascade)
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
	ctx context.Context, request mcp.CallToolRequest,
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

	st := s.ws.Store()
	e, getErr := st.GetEntity(ctx, oldID)
	if getErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", oldID)), nil
	}

	// Pause watcher during rename
	s.ws.PauseWatching()
	defer s.ws.ResumeWatching()

	result, renameErr := s.ws.Rename(e.Type, oldID, newID, rename.Options{DryRun: dryRun})
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

// filterStoreEntities applies a where clause to entity.Entity slices.
func filterStoreEntities(entities []*entity.Entity, where string) ([]*entity.Entity, error) {
	parts := strings.SplitN(where, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected property=value, got %q", where)
	}
	key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	var filtered []*entity.Entity
	for _, e := range entities {
		if e.GetAttributeString(key) == value {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// sortStoreEntitiesByID sorts entity.Entity slices by ID using natural ordering.
func sortStoreEntitiesByID(entities []*entity.Entity) {
	sort.Slice(entities, func(i, j int) bool {
		return natsort.Less(entities[i].ID, entities[j].ID)
	})
}
