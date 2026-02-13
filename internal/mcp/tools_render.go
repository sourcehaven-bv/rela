// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/transclusion"
)

func (s *Server) handleRenderEntity(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id = trimID(id)

	// Check entity exists
	if _, ok := s.graph.GetNode(id); !ok {
		return mcp.NewToolResultError(fmt.Sprintf("entity not found: %s", id)), nil
	}

	// Build render options
	includeFrontmatter := request.GetBool("include_frontmatter", false)
	maxDepth := request.GetInt("max_depth", transclusion.DefaultMaxDepth)
	stripComments := request.GetBool("strip_comments", true)

	opts := transclusion.RenderOptions{
		IncludeFrontmatter: includeFrontmatter,
		MaxDepth:           maxDepth,
		StripComments:      stripComments,
	}

	// Create resolver and render
	resolver := transclusion.NewResolver(s.graph)
	if maxDepth > 0 {
		resolver = resolver.WithMaxDepth(maxDepth)
	}

	rendered, err := resolver.RenderEntity(id, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("render failed: %v", err)), nil
	}

	return mcp.NewToolResultText(rendered), nil
}
