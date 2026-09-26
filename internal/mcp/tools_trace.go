// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// traceHandler serves the trace_from / trace_to / find_path tools. A type of
// its own rather than more methods on [Server] (the urlHelpers pattern,
// TKT-YUETL7): tracing needs exactly two collaborators — the gated
// [GraphReader] for existence probes and the tracer for traversal. Identity
// still arrives on the ctx via Server.principalMiddleware; the handler holds
// no principal.
type traceHandler struct {
	store  GraphReader
	tracer tracer.Tracer
}

func (h traceHandler) handleTraceFrom(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	return h.handleTrace(ctx, request, func(t tracer.Tracer, id string, depth int) *tracer.TraceResult {
		return t.TraceFrom(ctx, id, depth)
	}, "No dependencies found")
}

func (h traceHandler) handleTraceTo(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	return h.handleTrace(ctx, request, func(t tracer.Tracer, id string, depth int) *tracer.TraceResult {
		return t.TraceTo(ctx, id, depth)
	}, "No upstream dependencies found")
}

func (h traceHandler) handleTrace(
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
	if refused := wholeEntityRef(id); refused != nil {
		return refused, nil
	}
	maxDepth := args.GetInt("max_depth", 0)

	// Existence probe BEFORE traversal. This must go through h.store —
	// the gated GraphReader — not a raw handle: under a networked wiring a
	// hidden entity's GetEntity returns not-found, so "hidden" and "absent"
	// produce the identical message and the probe is not an existence
	// oracle (RR-FTJUUE). A raw probe here would defeat the gated tracer
	// below, since it answers "is it in the store?" rather than "may this
	// principal see it?".
	if _, getErr := h.store.GetEntity(ctx, id); getErr != nil {
		return entityReadFailed("entity", id, getErr), nil
	}

	result := traceFn(h.tracer, id, maxDepth)
	if result == nil {
		return textResult(emptyMsg), nil
	}

	text, err := convertTraceResult(result)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

func (h traceHandler) handleFindPath(
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
	for _, ref := range []string{from, to} {
		if refused := wholeEntityRef(ref); refused != nil {
			return refused, nil
		}
	}

	st := h.store
	if _, fromErr := st.GetEntity(ctx, from); fromErr != nil {
		return entityReadFailed("source entity", from, fromErr), nil
	}
	if _, toErr := st.GetEntity(ctx, to); toErr != nil {
		return entityReadFailed("target entity", to, toErr), nil
	}

	path := h.tracer.FindPath(ctx, from, to)
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
