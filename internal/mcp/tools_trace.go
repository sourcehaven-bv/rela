// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleTraceFrom(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)
	maxDepth := request.GetInt("max_depth", 0)

	g := s.ws.Graph()
	if _, ok := g.GetNode(id); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	result := g.TraceFrom(id, maxDepth)
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
	id = trimID(id)
	maxDepth := request.GetInt("max_depth", 0)

	g := s.ws.Graph()
	if _, ok := g.GetNode(id); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	result := g.TraceTo(id, maxDepth)
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
	from = trimID(from)
	to, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	to = trimID(to)

	g := s.ws.Graph()
	if _, ok := g.GetNode(from); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("source entity not found: %s", from)), nil
	}
	if _, ok := g.GetNode(to); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("target entity not found: %s", to)), nil
	}

	path := g.FindPath(from, to)
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
