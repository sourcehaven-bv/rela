package workspace

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// luaScriptRunner adapts the workspace's Lua-based [ScriptExecutor]
// to the runtime-agnostic [autocascade.ScriptRunner] interface that
// Runner consumes. One adapter is constructed per Runner.Process call
// so the per-request lua.WriteDeps bundle is bound at the right
// scope.
//
// Engine-specific concerns live here, not in autocascade:
//   - lua.WriteDeps is captured at construction.
//   - *lua.ScriptError patching (rewriting the Path field to
//     "automation:<name>" for inline blocks) happens inside Run.
//
// autocascade.Runner is therefore independent of any specific script
// runtime; replacing Lua with another engine would mean providing a
// different ScriptRunner adapter, not changing Runner.
type luaScriptRunner struct {
	exec ScriptExecutor
	deps lua.WriteDeps
}

// newLuaScriptRunner is constructed inside Workspace dispatch sites
// (createEntity / updateEntity) per cascade invocation. Returns nil
// if exec is nil — Runner records each scripted action as an error
// when ScriptRunner is nil, which is the right behavior for
// misconfigured deployments.
func newLuaScriptRunner(exec ScriptExecutor, deps lua.WriteDeps) *luaScriptRunner {
	if exec == nil {
		return nil
	}
	return &luaScriptRunner{exec: exec, deps: deps}
}

// Run dispatches the action to the underlying executor.
//
// Errors returned by the executor are *not* wrapped here beyond the
// Lua-error-path patching: Runner.executeScriptActions slog-Warns
// with the automation name and appends err.Error() to Outcome.Errors,
// which is the surface the API layer reads.
func (l *luaScriptRunner) Run(_ context.Context, action autocascade.ScriptAction) error {
	var err error
	switch {
	case action.Code != "":
		err = l.exec.ExecuteCode(action.Code, l.deps, action.NewEntity, action.OldEntity)
	case action.FilePath != "":
		err = l.exec.ExecuteFile(action.FilePath, l.deps, action.NewEntity, action.OldEntity)
	default:
		return nil
	}
	if err == nil {
		return nil
	}
	return formatScriptError(action, err)
}

// formatScriptError patches lua.ScriptError envelopes with the
// automation identity. For inline `lua: |` blocks, the Lua engine
// has no FilePath and tags the envelope with "<inline>" — overwrite
// the Path with "automation:<name>" so error messages identify the
// failing block.
//
// Mutates the *lua.ScriptError in place. The error is freshly built
// by the engine for this invocation; no other reference holds it.
func formatScriptError(action autocascade.ScriptAction, err error) error {
	var se *lua.ScriptError
	if errors.As(err, &se) {
		if action.FilePath == "" && action.Name != "" {
			se.Path = "automation:" + action.Name
		}
		return se
	}
	return err
}

// Compile-time assertion that *luaScriptRunner satisfies the
// consumer-side ScriptRunner interface.
var _ autocascade.ScriptRunner = (*luaScriptRunner)(nil)
