// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// scriptsDir is the directory where Lua scripts must be located for lua_run.
const scriptsDir = "scripts"

func toolLuaEval() mcp.Tool {
	return mcp.NewTool("lua_eval",
		mcp.WithDescription(
			"Execute Lua code against the rela graph. "+
				"Use rela.output(data) to return results as JSON. "+
				"Available functions: get_entity, list_entities, search, create_entity, update_entity, "+
				"delete_entity, get_relations, create_relation, delete_relation, trace_from, trace_to, "+
				"find_path, refresh, write_file, get_entity_types, get_relation_types. "+
				"Context: rela.project_root, rela.args."),
		mcp.WithString("code", mcp.Required(),
			mcp.Description("Lua code to execute")),
	)
}

func toolLuaRun() mcp.Tool {
	return mcp.NewTool("lua_run",
		mcp.WithDescription(
			"Execute a Lua script file against the rela graph. "+
				"Scripts must be located in the 'scripts/' directory. "+
				"Use rela.output(data) to return results as JSON."),
		mcp.WithString("path", mcp.Required(),
			mcp.Description("Script filename or path within scripts/ (e.g., 'export.lua' or 'reports/summary.lua')")),
		mcp.WithArray("args",
			mcp.Description("Arguments to pass to the script (available as rela.args)")),
	)
}

func toolLuaList() mcp.Tool {
	return mcp.NewTool("lua_list",
		mcp.WithDescription(
			"List available Lua scripts in the scripts/ directory. "+
				"Only scripts in this directory can be executed via lua_run."),
	)
}

func (s *Server) handleLuaEval(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := req.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var output bytes.Buffer
	if err := s.ws.RunScript(code, workspace.ScriptOptions{Stdout: &output}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Lua error: %s", err.Error())), nil
	}

	result := output.String()
	if result == "" {
		result = "Script executed successfully (no output)"
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleLuaRun(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Parse args if provided
	args := req.GetStringSlice("args", nil)

	var output bytes.Buffer
	if err := s.ws.RunScript(path, workspace.ScriptOptions{
		Stdout: &output,
		Args:   args,
	}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Lua error in %s: %s", path, err.Error())), nil
	}

	result := output.String()
	if result == "" {
		result = "Script executed successfully (no output)"
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleLuaList(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectRoot := s.ws.Paths().Root

	// Only search the scripts/ directory (security restriction)
	scriptsPath := filepath.Join(projectRoot, scriptsDir)

	var scripts []string

	// Walk the scripts directory recursively to find all .lua files
	_ = filepath.WalkDir(scriptsPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return filepath.SkipDir // Skip directories with errors
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".lua") {
			return nil
		}

		// Get relative path from scripts directory
		relPath, _ := filepath.Rel(scriptsPath, path)
		if relPath != "" {
			scripts = append(scripts, relPath)
		}
		return nil
	})

	if len(scripts) == 0 {
		return mcp.NewToolResultText("No Lua scripts found in scripts/ directory"), nil
	}

	var result strings.Builder
	result.WriteString("Available Lua scripts (in scripts/):\n")
	for _, script := range scripts {
		result.WriteString("  ")
		result.WriteString(script)
		result.WriteString("\n")
	}
	result.WriteString("\nUse lua_run with the script name to execute.")

	return mcp.NewToolResultText(result.String()), nil
}
