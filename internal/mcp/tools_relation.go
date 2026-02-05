// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func (s *Server) handleListRelations(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	relType := request.GetString("type", "")
	from := request.GetString("from", "")
	to := request.GetString("to", "")
	limit := request.GetInt("limit", 0)
	offset := request.GetInt("offset", 0)

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
	if valErr := s.getMeta().ValidateRelation(relType, fromEntity.Type, toEntity.Type); valErr != nil {
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
	template, templateErr := s.repo.LoadRelationTemplate(relType)
	if templateErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load template: %v", templateErr)), nil
	}
	if template != nil {
		markdown.ApplyRelationTemplate(relation, template)
	}

	if writeErr := s.repo.WriteRelation(relation); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write relation: %v", writeErr)), nil
	}

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

	_, exists := s.graph.GetEdge(fromID, relType, toID)
	if !exists {
		return mcp.NewToolResultError(
			fmt.Sprintf("relation not found: %s --%s--> %s", fromID, relType, toID)), nil
	}

	if delErr := s.repo.DeleteRelation(fromID, relType, toID); delErr != nil {
		s.logger.Printf("Warning: failed to delete relation file: %v", delErr)
	}

	s.graph.RemoveEdge(fromID, relType, toID)
	s.saveCache()

	return mcp.NewToolResultText(
		fmt.Sprintf("Removed link: %s --%s--> %s", fromID, relType, toID)), nil
}
