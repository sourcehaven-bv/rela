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

// Engine runs scripts with provided context. Besides enforcing path
// validation, secure file loading, and sandbox rules at execution time,
// it also owns a process-wide Lua cache (see lua.Cache) shared across
// every runtime it builds so that memoization and TTL semantics span
// repeated script invocations within the same process.
//
// Timeout is handled by lua.Runtime (default 30s, configurable via lua.WithTimeout).
type Engine struct {
	cache *lua.Cache
}

// NewEngine creates a script engine with a fresh Lua cache. Typically one
// Engine is constructed per process; callers that create multiple engines
// (e.g. in tests, or the CLI root + scheduler subcommand) get independent
// caches, which is intentional — each Engine is a logical cache scope.
func NewEngine() *Engine {
	return &Engine{cache: lua.NewCache()}
}

// LuaCache exposes the Engine's shared Lua cache so callers that build
// Lua runtimes outside of ExecuteCode/ExecuteFile (validation rules,
// MCP lua_eval, flow, etc.) can pass it via lua.WithCache and share
// cache state with engine-built runtimes. Also satisfies
// workspace.ScriptExecutor.
func (e *Engine) LuaCache() *lua.Cache {
	return e.cache
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
	runtime, err := NewWriterRuntime(deps, scriptPath, &output, lua.WithCache(e.cache))
	if err != nil {
		return err
	}
	defer runtime.Close()

	// Engine.execute reads the file content itself and runs via
	// RunString, which doesn't set the script path automatically the
	// way RunFile does. Wire it manually so rela.cache.* bindings get
	// a namespace. Inline code (scriptPath == "") keeps the
	// inline/eval identity so cache calls raise loudly instead of
	// silently sharing a nameless namespace.
	runtime.SetScriptPath(scriptPath)

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
