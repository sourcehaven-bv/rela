// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
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

func (s *Server) handleLuaEval(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := req.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	projectRoot := s.ws.Paths().Root

	// Capture output
	var output bytes.Buffer

	opts := []lua.Option{lua.WithContext(ctx)}
	// Soft-fail on misconfigured ai.yaml: log + continue without AI
	// rather than crashing every Lua tool call. The MCP client will
	// see the not_configured error if their script tries to call ai.*.
	// ErrConfigNotFound is the normal "no AI" state and is silent.
	provider, providerErr := ai.LoadProvider(s.ws.Paths().CacheDir)
	switch {
	case errors.Is(providerErr, ai.ErrConfigNotFound):
		// no AI configured
	case providerErr != nil:
		slog.Warn("ai: failed to load config; AI bindings disabled", "error", providerErr)
	default:
		opts = append(opts, lua.WithAIProvider(provider))
	}
	runtime := lua.New(s.ws, s.ws.Snapshot().Meta(), projectRoot, &output, opts...)
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

func (s *Server) handleLuaRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Security: Validate path is local (no "..", no absolute paths)
	if !filepath.IsLocal(path) {
		return mcp.NewToolResultError("script path must be a local path (no '..' or absolute paths allowed)"), nil
	}

	// Security: Must have .lua extension
	if !strings.HasSuffix(path, ".lua") {
		return mcp.NewToolResultError("script must have .lua extension"), nil
	}

	// Parse args if provided
	args := req.GetStringSlice("args", nil)

	projectRoot := s.ws.Paths().Root

	// Security: Scripts must be in the scripts/ directory
	// Use os.Root for traversal-resistant path access
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("cannot open project root: %s", err.Error())), nil
	}
	defer root.Close()

	// Verify script exists using traversal-resistant API
	scriptsRoot, err := root.OpenRoot(scriptsDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scripts directory not found: %s", err.Error())), nil
	}
	defer scriptsRoot.Close()

	// Read script content using traversal-resistant API to prevent symlink escapes
	scriptFile, err := scriptsRoot.Open(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("script not found: %s (scripts must be in the scripts/ directory)", path)), nil
	}
	defer scriptFile.Close()

	// Read script content
	scriptContent, err := io.ReadAll(scriptFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("cannot read script: %s", err.Error())), nil
	}

	// Capture output
	var output bytes.Buffer

	opts := []lua.Option{lua.WithContext(ctx)}
	// Soft-fail on misconfigured ai.yaml: log + continue without AI
	// rather than crashing every Lua tool call.
	if provider, providerErr := ai.LoadProvider(s.ws.Paths().CacheDir); providerErr != nil {
		slog.Warn("ai: failed to load config; AI bindings disabled", "error", providerErr)
	} else if provider != nil {
		opts = append(opts, lua.WithAIProvider(provider))
	}
	sec, secErr := secrets.Load(s.ws.Paths().CacheDir, path)
	if secErr != nil && !errors.Is(secErr, secrets.ErrNotFound) {
		slog.Warn("secrets: failed to load", "error", secErr)
	} else if len(sec) > 0 {
		opts = append(opts, lua.WithSecrets(sec))
	}
	runtime := lua.New(s.ws, s.ws.Snapshot().Meta(), projectRoot, &output, opts...)
	defer runtime.Close()

	// Set script args before execution
	runtime.SetArgs(args)

	// Execute script content directly (bypasses symlink escapes since we read via os.Root)
	if err := runtime.RunString(string(scriptContent)); err != nil {
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
