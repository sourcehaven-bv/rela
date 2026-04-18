// Package script orchestrates script execution for automations.
// It combines lua.Runtime with workspace operations, handling:
// path validation, secure file loading, and entity context injection.
//
// The Engine is stateless - all context (workspace, metamodel, paths, entities)
// is passed at execution time. This avoids circular dependencies: workspace can
// be constructed with an Engine, and the Engine receives workspace access only
// when executing scripts.
package script

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// scriptsDir is the directory where script files must be located.
const scriptsDir = "scripts"

// Engine runs scripts with provided context. It is stateless - all dependencies
// are passed at execution time via Context. This centralizes script execution
// concerns: path validation, secure file loading, sandbox enforcement.
//
// Timeout is handled by lua.Runtime (default 30s, configurable via lua.WithTimeout).
type Engine struct{}

// NewEngine creates a stateless script engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ExecuteCode runs inline script code with the given context.
func (e *Engine) ExecuteCode(code string, ctx metamodel.ScriptContext) error {
	return e.execute(code, ctx, "")
}

// ExecuteFile loads and runs a script file from the scripts/ directory.
// The path must be a local path (no ".." or absolute paths) with .lua extension.
func (e *Engine) ExecuteFile(path string, ctx metamodel.ScriptContext) error {
	scriptCode, err := loadScript(ctx.GetProjectRoot(), path)
	if err != nil {
		return err
	}
	return e.execute(scriptCode, ctx, path)
}

// execute runs Lua code with entity context. scriptPath is used to resolve
// per-script secrets; pass "" for inline code (no secrets loaded).
// Timeout is handled by lua.Runtime (default 30s).
func (e *Engine) execute(code string, ctx metamodel.ScriptContext, scriptPath string) error {
	// Type assert workspace to lua.Services
	svc, ok := ctx.GetWorkspace().(lua.Services)
	if !ok {
		return errors.New("workspace does not provide lua.Services")
	}
	// Ensure Meta and ProjectRoot are populated if not already.
	if svc.Meta == nil {
		svc.Meta = ctx.GetMeta()
	}
	if svc.ProjectRoot == "" {
		svc.ProjectRoot = ctx.GetProjectRoot()
	}

	var output bytes.Buffer
	relaDir := filepath.Join(ctx.GetProjectRoot(), project.CacheDir)
	opts, optErr := lua.LoadContextOptions(relaDir, scriptPath)
	if optErr != nil {
		return fmt.Errorf("lua context: %w", optErr)
	}
	runtime := lua.New(svc, &output, opts...)
	defer runtime.Close()

	// Set entity context as Lua globals
	ls := runtime.LState()
	if ctx.GetEntity() != nil {
		ls.SetGlobal("entity", lua.EntityToTable(ls, ctx.GetEntity()))
	}
	if ctx.GetOldEntity() != nil {
		ls.SetGlobal("old_entity", lua.EntityToTable(ls, ctx.GetOldEntity()))
	}

	return runtime.RunString(code)
}

// loadScript loads a script from the scripts/ directory using os.OpenRoot
// for traversal-resistant file access.
func loadScript(projectRoot, scriptPath string) (string, error) {
	// Security: Validate path is local (no "..", no absolute paths)
	if !filepath.IsLocal(scriptPath) {
		return "", fmt.Errorf(
			"script path must be a local path (no '..' or absolute paths): %s", scriptPath)
	}

	// Security: Must have .lua extension
	if !strings.HasSuffix(scriptPath, ".lua") {
		return "", fmt.Errorf("script must have .lua extension: %s", scriptPath)
	}

	// Use os.OpenRoot for traversal-resistant access.
	// Error messages intentionally omit system paths to prevent information leakage.
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return "", errors.New("cannot access project directory")
	}
	defer root.Close()

	scriptsRoot, err := root.OpenRoot(scriptsDir)
	if err != nil {
		return "", errors.New("cannot access scripts directory")
	}
	defer scriptsRoot.Close()

	scriptFile, err := scriptsRoot.Open(scriptPath)
	if err != nil {
		return "", fmt.Errorf("script not found: %s (must be in scripts/ directory)", scriptPath)
	}
	defer scriptFile.Close()

	content, err := io.ReadAll(scriptFile)
	if err != nil {
		return "", fmt.Errorf("cannot read script: %s", scriptPath)
	}

	return string(content), nil
}
