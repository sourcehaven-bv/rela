// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func (s *Server) handleTraceFrom(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return s.handleTrace(request, func(snap *workspace.Snapshot, id string, depth int) *model.TraceResult {
		return snap.TraceFrom(id, depth)
	}, "No dependencies found")
}

func (s *Server) handleTraceTo(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return s.handleTrace(request, func(snap *workspace.Snapshot, id string, depth int) *model.TraceResult {
		return snap.TraceTo(id, depth)
	}, "No upstream dependencies found")
}

func (s *Server) handleTrace(
	request mcp.CallToolRequest,
	traceFn func(*workspace.Snapshot, string, int) *model.TraceResult,
	emptyMsg string,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)
	maxDepth := request.GetInt("max_depth", 0)

	snap := s.ws.Snapshot()
	if _, ok := snap.GetEntity(id); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	result := traceFn(snap, id, maxDepth)
	if result == nil {
		return mcp.NewToolResultText(emptyMsg), nil
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

	snap := s.ws.Snapshot()
	if _, ok := snap.GetEntity(from); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("source entity not found: %s", from)), nil
	}
	if _, ok := snap.GetEntity(to); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("target entity not found: %s", to)), nil
	}

	path := snap.FindPath(from, to)
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
