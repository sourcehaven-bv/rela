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
	"context"
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
//
// ctx threads into Lua write bindings so downstream audit records
// carry the caller's Principal / triggered_by, and cancellation
// propagates into in-flight Lua. Callers without a meaningful ctx
// pass context.Background() explicitly.
func (e *Engine) ExecuteCode(ctx context.Context, code string, deps lua.WriteDeps,
	newEntity, oldEntity *entity.Entity) error {
	return e.execute(ctx, code, deps, "", newEntity, oldEntity)
}

// ExecuteFile loads and runs a script file from the scripts/ directory.
// The path must be a local path (no ".." or absolute paths) with .lua
// extension. ctx semantics match [ExecuteCode].
func (e *Engine) ExecuteFile(ctx context.Context, path string, deps lua.WriteDeps,
	newEntity, oldEntity *entity.Entity) error {
	scriptCode, err := loadScript(deps.ProjectRoot, path)
	if err != nil {
		return err
	}
	return e.execute(ctx, scriptCode, deps, path, newEntity, oldEntity)
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
	if timeout > 0 {
		opts = append(opts, lua.WithTimeout(timeout))
	}

	runtime, err := NewWriterRuntime(deps, path, stdout, opts...)
	if err != nil {
		return err
	}
	defer runtime.Close()

	// RunFileContent (not RunString) so gopher-lua receives the script
	// path as the chunkname — that lands in the message handler's frame
	// captures and lets ScriptError.Source populate from the right file.
	// Doubles as the rela.cache.* namespace, so SetScriptPath is no
	// longer needed alongside it.
	if runErr := runtime.RunFileContent(path, []byte(scriptCode), nil); runErr != nil {
		return wrapScriptError(lua.SurfaceDocument, scriptsDir, path, entryID,
			runtime.ErrorFrames(), nil, runErr, deps.ProjectRoot)
	}
	return nil
}

// wrapScriptError builds a *lua.ScriptError from a runtime failure.
// Captured output is left for the caller to attach via AttachCapturedOutput
// when they own the buffer (e.g. the document renderer).
//
// For inline / synthetic paths (containing '<' or empty), no prefix is
// applied — the path is used as-is for envelope display, and the source
// FS is left out since there's nothing on disk to slice.
func wrapScriptError(surface lua.Surface, subdir, scriptPath, entityID string,
	frames []lua.StackFrame, capturedOutput []byte, runErr error,
	projectRoot string) error {
	envelopePath := scriptPath
	useSourceFS := false
	if scriptPath != "" && !strings.ContainsAny(scriptPath, "<>") {
		envelopePath = filepath.ToSlash(filepath.Join(subdir, scriptPath))
		useSourceFS = true
		for i := range frames {
			if frames[i].Path == scriptPath {
				frames[i].Path = envelopePath
			}
		}
	}
	in := lua.BuildInput{
		Surface:        surface,
		Path:           envelopePath,
		EntityID:       entityID,
		Frames:         frames,
		CapturedOutput: capturedOutput,
		Err:            runErr,
	}
	if useSourceFS {
		in.SourceFS = os.DirFS(projectRoot)
	}
	return lua.BuildScriptError(in)
}

// execute runs Lua code with entity context. scriptPath is used to resolve
// per-script secrets; pass "" for inline code (no secrets loaded).
// Timeout is handled by lua.Runtime (default 30s). ctx threads through
// to Lua write bindings so downstream Manager calls receive the
// caller's Principal / triggered_by values.
func (e *Engine) execute(ctx context.Context, code string, deps lua.WriteDeps, scriptPath string,
	newEntity, oldEntity *entity.Entity) error {
	var output bytes.Buffer
	runtime, err := NewWriterRuntime(deps, scriptPath, &output,
		lua.WithCache(e.cache), lua.WithContext(ctx))
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
	entityID := ""
	if newEntity != nil {
		ls.SetGlobal("entity", lua.EntityToTable(ls, newEntity))
		entityID = newEntity.ID
	}
	if oldEntity != nil {
		ls.SetGlobal("old_entity", lua.EntityToTable(ls, oldEntity))
		if entityID == "" {
			entityID = oldEntity.ID
		}
	}

	// ctx is threaded into the runtime via lua.WithContext(ctx) above; RunString
	// applies it to the LState. contextcheck can't follow that flow across the
	// gopher-lua SetContext boundary.
	//nolint:contextcheck // ctx threaded via WithContext; see comment above
	if runErr := runtime.RunString(code); runErr != nil {
		// Surface tag is "automation" — this seam is invoked from the
		// automation engine (workspace) for both inline `lua: |` blocks
		// and `script:` files. Captured stdout is omitted: automations
		// are not a UI surface and operators rarely use print() there.
		path := scriptPath
		if path == "" {
			// Inline automation block — give it an identity stable enough
			// for the envelope to disambiguate. The automation name
			// would be better but isn't plumbed here; defer until the
			// automation engine wraps with that context.
			path = "<inline>"
		}
		return wrapScriptError(lua.SurfaceAutomation, scriptsDir, path, entityID,
			runtime.ErrorFrames(), nil, runErr, deps.ProjectRoot)
	}
	return nil
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
