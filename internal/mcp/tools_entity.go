// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

	entities := make([]*entity.Entity, 0)
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
		return mcp.NewToolResultError("entity not found: " + id), nil
	}

	text, err := convertStoreEntity(e, st, true)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleSearchEntities(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entityType := request.GetString("type", "")
	const defaultSearchLimit = 20
	limit := request.GetInt("limit", defaultSearchLimit)

	q := search.Query{Text: query, Limit: limit}
	if entityType != "" {
		q.Types = []string{s.resolveType(entityType)}
	}

	st := s.ws.Store()
	summaries := make([]map[string]interface{}, 0)
	for hit, searchErr := range s.ws.Searcher().Search(ctx, q) {
		if searchErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", searchErr)), nil
		}
		summary := map[string]interface{}{"id": hit.ID, "type": hit.Type}
		if hit.Title != "" {
			summary["title"] = hit.Title
		}
		if e, getErr := st.GetEntity(ctx, hit.ID); getErr == nil {
			if status := e.GetString("status"); status != "" {
				summary["status"] = status
			}
		}
		summaries = append(summaries, summary)
	}

	text, err := marshalJSON(summaries)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleCreateEntity(
	ctx context.Context, request mcp.CallToolRequest,
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
	properties := extractProperties(request)

	// Validate property names early for better error messages
	if errResult := s.validatePropertyNames(resolvedType, properties); errResult != nil {
		return errResult, nil
	}

	result, createErr := s.ws.EntityManager().CreateEntity(ctx,
		&entity.Entity{
			Type:       resolvedType,
			Properties: properties,
			Content:    content,
		},
		entitymanager.CreateOptions{ID: customID},
	)
	if createErr != nil {
		return mcp.NewToolResultError(createErr.Error()), nil
	}
	created := result.Entity

	st := s.ws.Store()
	e, _ := st.GetEntity(ctx, created.ID)
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
		return mcp.NewToolResultError("entity not found: " + id), nil
	}

	properties := extractPropertiesAllowNil(request)
	content := request.GetString("content", "")

	if len(properties) == 0 && content == "" {
		return mcp.NewToolResultError("no updates specified"), nil
	}

	// Validate property names early for better error messages
	if errResult := s.validatePropertyNames(e.Type, properties); errResult != nil {
		return errResult, nil
	}

	// Apply property updates: nil deletes, anything else sets/overwrites.
	for k, v := range properties {
		if v == nil {
			delete(e.Properties, k)
			continue
		}
		e.Properties[k] = v
	}
	if content != "" {
		e.Content = content
	}

	if _, updateErr := s.ws.EntityManager().UpdateEntity(ctx, e); updateErr != nil {
		return mcp.NewToolResultError(updateErr.Error()), nil
	}

	updated, _ := st.GetEntity(ctx, id)
	if updated == nil {
		return mcp.NewToolResultText("Updated " + id), nil
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
		return mcp.NewToolResultError("entity not found: " + id), nil
	}

	// Check for relations (for better error message)
	if !cascade {
		n, _ := st.CountRelations(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionBoth})
		if n > 0 {
			return mcp.NewToolResultError(
				fmt.Sprintf("entity %s has %d relation(s); set cascade=true to delete them too", id, n)), nil
		}
	}

	result, delErr := s.ws.EntityManager().DeleteEntity(ctx, id, cascade)
	if delErr != nil {
		return mcp.NewToolResultError(delErr.Error()), nil
	}
	_ = e // kept for the cascade relation-count check above

	msg := "Deleted " + id
	if cascade && len(result.DeletedRelations) > 0 {
		msg += fmt.Sprintf(" and %d relation(s)", len(result.DeletedRelations))
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

	// Pause watcher during rename
	s.ws.PauseWatching()
	defer s.ws.ResumeWatching()

	result, renameErr := s.ws.EntityManager().RenameEntity(
		ctx, oldID, newID, entitymanager.RenameOptions{DryRun: dryRun})
	if renameErr != nil {
		return mcp.NewToolResultError(renameErr.Error()), nil
	}

	verb := "Renamed"
	if dryRun {
		verb = "Dry run — would rename"
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("%s: %s → %s (%d relations updated)", verb, result.OldID, result.NewID, result.RelationsUpdated)), nil
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
