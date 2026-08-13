// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

func (s *Server) handleTraceFrom(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	return s.handleTrace(ctx, request, func(t tracer.Tracer, id string, depth int) *tracer.TraceResult {
		return t.TraceFrom(ctx, id, depth)
	}, "No dependencies found")
}

func (s *Server) handleTraceTo(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	return s.handleTrace(ctx, request, func(t tracer.Tracer, id string, depth int) *tracer.TraceResult {
		return t.TraceTo(ctx, id, depth)
	}, "No upstream dependencies found")
}

func (s *Server) handleTrace(
	ctx context.Context,
	request *mcpgo.CallToolRequest,
	traceFn func(tracer.Tracer, string, int) *tracer.TraceResult,
	emptyMsg string,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	id, err := args.RequireString("id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	id = trimID(id)
	maxDepth := args.GetInt("max_depth", 0)

	if _, getErr := s.deps.Store.GetEntity(ctx, id); getErr != nil {
		return errorResult("entity not found: " + id), nil
	}

	result := traceFn(s.deps.Tracer, id, maxDepth)
	if result == nil {
		return textResult(emptyMsg), nil
	}

	text, err := convertTraceResult(result)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

func (s *Server) handleFindPath(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	from, err := args.RequireString("from")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	from = trimID(from)
	to, err := args.RequireString("to")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	to = trimID(to)

	st := s.deps.Store
	if _, fromErr := st.GetEntity(ctx, from); fromErr != nil {
		return errorResult("source entity not found: " + from), nil
	}
	if _, toErr := st.GetEntity(ctx, to); toErr != nil {
		return errorResult("target entity not found: " + to), nil
	}

	path := s.deps.Tracer.FindPath(ctx, from, to)
	if path == nil {
		return textResult(
			fmt.Sprintf("No path found between %s and %s", from, to)), nil
	}

	text, err := convertPathSteps(path)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}
