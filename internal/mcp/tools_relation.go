// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func (s *Server) handleListRelations(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	relType := request.GetString("type", "")
	from := request.GetString("from", "")
	to := request.GetString("to", "")
	limit := request.GetInt("limit", 0)
	offset := request.GetInt("offset", 0)

	edges := s.ws.Graph().AllEdges()

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

	// Apply offset/limit
	if offset > 0 {
		if offset >= len(filtered) {
			filtered = nil
		} else {
			filtered = filtered[offset:]
		}
	}
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

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

	content := request.GetString("content", "")
	properties := s.extractProperties(request)

	opts := workspace.CreateRelationOptions{
		Properties: properties,
		Content:    content,
	}

	if _, createErr := s.ws.CreateRelation(fromID, relType, toID, opts); createErr != nil {
		return mcp.NewToolResultError(createErr.Error()), nil
	}

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

	_, exists := s.ws.Graph().GetEdge(fromID, relType, toID)
	if !exists {
		return mcp.NewToolResultError(
			fmt.Sprintf("relation not found: %s --%s--> %s", fromID, relType, toID)), nil
	}

	if delErr := s.ws.DeleteRelation(fromID, relType, toID); delErr != nil {
		return mcp.NewToolResultError(delErr.Error()), nil
	}

	return mcp.NewToolResultText(
		fmt.Sprintf("Removed link: %s --%s--> %s", fromID, relType, toID)), nil
}
