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
	"time"

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
	cache  *lua.Cache
	routes lua.RouteCatalog // nil unless the engine renders documents with rela.url
}

// EngineOption configures an Engine at construction.
type EngineOption func(*Engine)

// WithRouteCatalog wires a frontend-route catalog into the engine. When
// set, ExecuteDocument registers rela.url on its runtimes. Other script
// execution paths (ExecuteCode, ExecuteFile, ExecuteAction) never see it —
// they have no frontend to target.
func WithRouteCatalog(c lua.RouteCatalog) EngineOption {
	return func(e *Engine) { e.routes = c }
}

// NewEngine creates a script engine with a fresh Lua cache. Typically one
// Engine is constructed per process; callers that create multiple engines
// (e.g. in tests, or the CLI root + scheduler subcommand) get independent
// caches, which is intentional — each Engine is a logical cache scope.
func NewEngine(opts ...EngineOption) *Engine {
	e := &Engine{cache: lua.NewCache()}
	for _, opt := range opts {
		opt(e)
	}
	return e
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

// ExecuteDocument loads and runs a Lua script in document-rendering mode.
// The script's stdout is captured into the caller-supplied writer; that
// output is the rendered markdown. The data-entry layer then converts it
// to HTML and rewrites any app-relative /form/... href to append a
// return_to query param; legacy edit:// / create:// schemes pass through
// unchanged with a warning.
//
// documentID is the key under documents: in data-entry.yaml (exposed to
// the script as rela.document.id). entryID is the ID of the entity being
// rendered (exposed as rela.document.entry_id). timeout overrides the
// default lua timeout when non-zero.
//
// This method exists as a typed seam — intentionally NOT taking variadic
// lua.Option — so callers cannot inject arbitrary opts (e.g., forge
// WithOutputDir or WithActionMode). Mirrors the ExecuteAction shape.
//
// rela.url is registered here (and only here) via the engine's route
// catalog. Other writer runtimes — CLI scripts, scheduler, MCP lua_run,
// actions, automations — are not wired with a catalog: they have no
// frontend to target and rela.url would be meaningless there.
func (e *Engine) ExecuteDocument(
	path string,
	deps lua.WriteDeps,
	stdout io.Writer,
	documentID string,
	entryID string,
	timeout time.Duration,
) error {
	scriptCode, err := loadScript(deps.ProjectRoot, path)
	if err != nil {
		return err
	}

	opts := []lua.Option{
		lua.WithDocumentMode(documentID, entryID),
		lua.WithCache(e.cache),
	}
	if e.routes != nil {
		opts = append(opts, lua.WithRouteCatalog(e.routes))
	}
	if timeout > 0 {
		opts = append(opts, lua.WithTimeout(timeout))
	}

	runtime, err := NewWriterRuntime(deps, path, stdout, opts...)
	if err != nil {
		return err
	}
	defer runtime.Close()

	// NewWriterRuntime receives `path` for per-script secret loading, but
	// rela.cache.* namespacing is driven by a separate scriptPath field
	// that RunFile/RunFileContent set. We're invoking RunString, so wire
	// the path explicitly — otherwise rela.cache.* inside a document
	// script would hit the inline/eval guard and raise.
	runtime.SetScriptPath(path)

	return runtime.RunString(scriptCode)
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

// CheckDocumentScriptExists verifies a document script can be loaded.
// Used at config-load time (dataentry.NewApp) to fail fast when a
// data-entry.yaml `documents:` entry points at a missing or malformed
// script, instead of deferring the error to the first HTTP render.
// Mirrors CheckActionScriptExists.
func CheckDocumentScriptExists(projectRoot, scriptPath string) error {
	_, err := loadScript(projectRoot, scriptPath)
	return err
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
