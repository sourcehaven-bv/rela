// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func (s *Server) handleListRelations(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	relType := request.GetString("type", "")
	from := request.GetString("from", "")
	to := request.GetString("to", "")
	limit := request.GetInt("limit", 0)
	offset := request.GetInt("offset", 0)

	st := s.ws.Store()
	q := store.RelationQuery{Type: relType, From: from, To: to}

	all := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		all = append(all, r)
	}

	sortStoreRelations(all)

	// Apply offset/limit
	if offset > 0 {
		if offset >= len(all) {
			all = nil
		} else {
			all = all[offset:]
		}
	}
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}

	text, err := convertStoreRelationsList(all)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleCreateRelation(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	fromID, err := request.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	fromID = trimID(fromID)
	relType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	relType = strings.TrimSpace(relType)
	toID, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toID = trimID(toID)

	// Treat an empty `content` string from the MCP request as "leave alone"
	// rather than "set body to empty". MCP clients can omit the field or
	// pass null to mean the same; an explicit "" today never reaches a
	// no-content-meant-empty case in practice.
	opts := entitymanager.RelationOptions{
		Properties: extractProperties(request),
		Content:    nilIfEmpty(request.GetString("content", "")),
	}

	if _, createErr := s.ws.EntityManager().CreateRelation(ctx, fromID, relType, toID, opts); createErr != nil {
		return mcp.NewToolResultError(createErr.Error()), nil
	}

	return mcp.NewToolResultText(
		fmt.Sprintf("Created link: %s --%s--> %s", fromID, relType, toID)), nil
}

func (s *Server) handleDeleteRelation(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	fromID, err := request.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	fromID = trimID(fromID)
	relType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	relType = strings.TrimSpace(relType)
	toID, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toID = trimID(toID)

	st := s.ws.Store()
	if _, getErr := st.GetRelation(ctx, fromID, relType, toID); getErr != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("relation not found: %s --%s--> %s", fromID, relType, toID)), nil
	}

	if delErr := s.ws.EntityManager().DeleteRelation(ctx, fromID, relType, toID); delErr != nil {
		return mcp.NewToolResultError(delErr.Error()), nil
	}

	return mcp.NewToolResultText(
		fmt.Sprintf("Removed link: %s --%s--> %s", fromID, relType, toID)), nil
}

// nilIfEmpty returns nil when s is empty, else &s. Used to translate
// "absent / empty string" inputs from the MCP layer into the
// leave-alone semantic of entitymanager.RelationOptions.Content.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
