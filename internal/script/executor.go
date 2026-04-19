// Package script orchestrates script execution for automations and user-
// initiated script runs. It combines lua.Runtime with secure file loading
// and entity context injection.
//
// The Engine is stateless — all context (deps, cacheDir, entities) is passed
// at execution time. This avoids circular dependencies: workspace holds an
// Engine, and the Engine receives the deps it needs only when invoked.
package script

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// scriptsDir is the directory where script files must be located.
const scriptsDir = "scripts"

// Engine runs scripts with provided context. It is stateless - all dependencies
// are passed at execution time. This centralizes script execution concerns:
// path validation, secure file loading, sandbox enforcement.
//
// Timeout is handled by lua.Runtime (default 30s, configurable via lua.WithTimeout).
type Engine struct{}

// NewEngine creates a stateless script engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ExecuteCode runs inline script code with entity context.
//
// AI config and per-script secrets are loaded from deps.ProjectRoot/.rela.
// newEntity/oldEntity are optional — nil when no entity is in scope.
func (e *Engine) ExecuteCode(code string, deps lua.WriteDeps,
	newEntity, oldEntity *entity.Entity) error {
	return e.execute(code, deps, "", newEntity, oldEntity)
}

// ExecuteFile loads and runs a script file from the scripts/ directory.
// The path must be a local path (no ".." or absolute paths) with .lua extension.
func (e *Engine) ExecuteFile(path string, deps lua.WriteDeps,
	newEntity, oldEntity *entity.Entity) error {
	scriptCode, err := loadScript(deps.ProjectRoot, path)
	if err != nil {
		return err
	}
	return e.execute(scriptCode, deps, path, newEntity, oldEntity)
}

// execute runs Lua code with entity context. scriptPath is used to resolve
// per-script secrets; pass "" for inline code (no secrets loaded).
// Timeout is handled by lua.Runtime (default 30s).
func (e *Engine) execute(code string, deps lua.WriteDeps, scriptPath string,
	newEntity, oldEntity *entity.Entity) error {
	var output bytes.Buffer
	runtime, err := NewWriterRuntime(deps, scriptPath, &output)
	if err != nil {
		return err
	}
	defer runtime.Close()

	ls := runtime.LState()
	if newEntity != nil {
		ls.SetGlobal("entity", lua.EntityToTable(ls, newEntity))
	}
	if oldEntity != nil {
		ls.SetGlobal("old_entity", lua.EntityToTable(ls, oldEntity))
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
