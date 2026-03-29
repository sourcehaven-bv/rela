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

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

func toolLuaEval() mcp.Tool {
	return mcp.NewTool("lua_eval",
		mcp.WithDescription(
			"Execute Lua code against the rela graph. "+
				"Use rela.output(data) to return results as JSON. "+
				"Available functions: get_entity, list_entities, search, create_entity, update_entity, "+
				"delete_entity, get_relations, create_relation, delete_relation, trace_from, trace_to, "+
				"find_path, refresh, write_file. "+
				"Context: rela.project_root."),
		mcp.WithString("code", mcp.Required(),
			mcp.Description("Lua code to execute")),
	)
}

func toolLuaRun() mcp.Tool {
	return mcp.NewTool("lua_run",
		mcp.WithDescription(
			"Execute a Lua script file against the rela graph. "+
				"Script path is relative to project root. "+
				"Use rela.output(data) to return results as JSON."),
		mcp.WithString("path", mcp.Required(),
			mcp.Description("Path to Lua script file (relative to project root)")),
		mcp.WithArray("args",
			mcp.Description("Arguments to pass to the script (available as rela.args)")),
	)
}

func toolLuaList() mcp.Tool {
	return mcp.NewTool("lua_list",
		mcp.WithDescription(
			"List available Lua scripts in the project. "+
				"Searches for .lua files in common locations (scripts/, lua/, root)."),
	)
}

func (s *Server) handleLuaEval(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := req.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	projectRoot := s.ws.Paths().Root

	// Capture output
	var output bytes.Buffer

	runtime := lua.New(s.ws, s.ws.Meta(), projectRoot, &output)
	defer runtime.Close()

	if err := runtime.RunString(code); err != nil {
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

	projectRoot := s.ws.Paths().Root

	// Resolve path relative to project root
	scriptPath := path
	if !filepath.IsAbs(path) {
		scriptPath = filepath.Join(projectRoot, path)
	}

	// Capture output
	var output bytes.Buffer

	runtime := lua.New(s.ws, s.ws.Meta(), projectRoot, &output)
	defer runtime.Close()

	if err := runtime.RunFile(scriptPath, args); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Lua error: %s", err.Error())), nil
	}

	result := output.String()
	if result == "" {
		result = "Script executed successfully (no output)"
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleLuaList(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectRoot := s.ws.Paths().Root

	// Directories to search for Lua scripts
	searchDirs := []string{
		"scripts",
		"lua",
		".",
	}

	var scripts []string
	seen := make(map[string]bool)

	for _, dir := range searchDirs {
		searchPath := filepath.Join(projectRoot, dir)
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue // Directory doesn't exist, skip
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".lua") {
				continue
			}

			// Get relative path from project root
			var relPath string
			if dir == "." {
				relPath = entry.Name()
			} else {
				relPath = filepath.Join(dir, entry.Name())
			}

			if seen[relPath] {
				continue
			}
			seen[relPath] = true
			scripts = append(scripts, relPath)
		}
	}

	if len(scripts) == 0 {
		return mcp.NewToolResultText("No Lua scripts found in project"), nil
	}

	var result strings.Builder
	result.WriteString("Available Lua scripts:\n")
	for _, script := range scripts {
		result.WriteString("  ")
		result.WriteString(script)
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}
