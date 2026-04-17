// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

func (s *Server) handleTraceFrom(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return s.handleTrace(ctx, request, func(t tracer.Tracer, id string, depth int) *tracer.TraceResult {
		return t.TraceFrom(ctx, id, depth)
	}, "No dependencies found")
}

func (s *Server) handleTraceTo(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return s.handleTrace(ctx, request, func(t tracer.Tracer, id string, depth int) *tracer.TraceResult {
		return t.TraceTo(ctx, id, depth)
	}, "No upstream dependencies found")
}

func (s *Server) handleTrace(
	ctx context.Context,
	request mcp.CallToolRequest,
	traceFn func(tracer.Tracer, string, int) *tracer.TraceResult,
	emptyMsg string,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)
	maxDepth := request.GetInt("max_depth", 0)

	if _, err := s.ws.Store().GetEntity(ctx, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	result := traceFn(s.ws.Tracer(), id, maxDepth)
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
	ctx context.Context, request mcp.CallToolRequest,
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

	st := s.ws.Store()
	if _, err := st.GetEntity(ctx, from); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("source entity not found: %s", from)), nil
	}
	if _, err := st.GetEntity(ctx, to); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("target entity not found: %s", to)), nil
	}

	path := s.ws.Tracer().FindPath(ctx, from, to)
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
