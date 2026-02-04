// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

func (s *Server) handleListViews(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	viewsFile, err := s.loadViews()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load views: %v", err)), nil
	}

	names := viewsFile.ViewNames()
	natsort.Strings(names)

	if len(names) == 0 {
		return mcp.NewToolResultText("No views defined (views.yaml not found or empty)"), nil
	}

	type viewInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		EntryType   string `json:"entry_type"`
		Parameter   string `json:"parameter"`
	}

	result := make([]viewInfo, 0, len(names))
	for _, name := range names {
		viewDef, _ := viewsFile.GetView(name)
		result = append(result, viewInfo{
			Name:        name,
			Description: viewDef.Description,
			EntryType:   viewDef.Entry.Type,
			Parameter:   viewDef.Entry.Parameter,
		})
	}

	text, err := marshalJSON(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

func (s *Server) handleExecuteView(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	viewName, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entryID, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	format := request.GetString("format", "json")

	viewsFile, loadErr := s.loadViews()
	if loadErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load views: %v", loadErr)), nil
	}

	viewDef, ok := viewsFile.GetView(viewName)
	if !ok {
		names := viewsFile.ViewNames()
		natsort.Strings(names)
		return mcp.NewToolResultError(
			fmt.Sprintf("view not found: %s (available: %s)", viewName, strings.Join(names, ", "))), nil
	}

	meta := s.getMeta()
	if validationErr := viewDef.Validate(meta, viewName); validationErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("view validation failed: %v", validationErr)), nil
	}

	engine := views.NewEngine(s.graph, meta)
	result, execErr := engine.Execute(viewDef, entryID)
	if execErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("view execution failed: %v", execErr)), nil
	}

	output, fmtErr := views.Format(result, format, s.graph, meta)
	if fmtErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format output: %v", fmtErr)), nil
	}

	return mcp.NewToolResultText(output), nil
}

func (s *Server) loadViews() (*views.File, error) {
	return s.repo.LoadViews()
}
