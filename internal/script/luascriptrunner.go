package script

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// Executor is the consumer-side interface the [LuaScriptRunner] needs
// from its Lua-based script executor. *script.Engine satisfies it
// structurally; tests can pass a stub that only implements these
// methods.
type Executor interface {
	ExecuteCode(ctx context.Context, code string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error
	ExecuteFile(ctx context.Context, path string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error
}

// LuaScriptRunner adapts a Lua-based [Executor] to the
// runtime-agnostic [autocascade.ScriptRunner] interface that
// [autocascade.Runner] consumes.
//
// Lifecycle: one LuaScriptRunner is built per wiring scope (workspace,
// MCP server, app-build, etc.). The runner holds the **static** half
// of lua.WriteDeps — [lua.ReadDeps] (Store/Tracer/Searcher/Meta/
// ProjectRoot) — captured at construction. The **dynamic** half — the
// [autocascade.Mutator] that scripts call back into for graph writes —
// is passed per-call via [Run]. That split is how the construction
// cycle between EntityManager and the Lua write-deps bundle is broken:
// Runner is built before EntityManager exists; EntityManager
// (a Mutator) supplies itself when dispatching.
//
// Engine-specific concerns live here, not in autocascade:
//   - lua.ReadDeps captured at construction.
//   - lua.WriteDeps assembled inside Run from readDeps + per-call mutator.
//   - *lua.ScriptError patching (rewriting the Path field to
//     "automation:<name>" for inline blocks) happens inside Run.
//
// autocascade.Runner is therefore independent of any specific script
// runtime; replacing Lua with another engine would mean providing a
// different ScriptRunner adapter, not changing Runner.
type LuaScriptRunner struct {
	exec     Executor
	readDeps lua.ReadDeps
}

// NewLuaScriptRunner returns a LuaScriptRunner bound to exec and the
// static read deps. Returns nil if exec is nil — Runner records each
// scripted action as an error when ScriptRunner is nil, which is the
// right behavior for misconfigured deployments.
func NewLuaScriptRunner(exec Executor, readDeps lua.ReadDeps) *LuaScriptRunner {
	if exec == nil {
		return nil
	}
	return &LuaScriptRunner{exec: exec, readDeps: readDeps}
}

// Run dispatches the action to the underlying executor. The mutator
// argument supplies the per-cascade graph-write handle for the
// constructed lua.WriteDeps.
//
// Errors returned by the executor are *not* wrapped here beyond the
// Lua-error-path patching: Runner.executeScriptActions slog-Warns
// with the automation name and appends err.Error() to Outcome.Errors,
// which is the surface the API layer reads.
func (l *LuaScriptRunner) Run(ctx context.Context, action autocascade.ScriptAction, m autocascade.Mutator) error {
	if action.Code == "" && action.FilePath == "" {
		return nil
	}
	if m == nil {
		// Lua scripts may invoke rela.create_entity et al., which require
		// a non-nil EntityManager in lua.WriteDeps. Reject up-front rather
		// than letting the engine nil-deref on the first call.
		return errors.New("script: LuaScriptRunner.Run: mutator is required")
	}
	deps := lua.WriteDeps{
		ReadDeps: l.readDeps,
		// m satisfies autocascade.Mutator (5 methods); the same five
		// are lua.Mutator's surface so the assignment type-checks. The
		// duplication of the interface is intentional per CLAUDE.md
		// "interfaces at the call site".
		EntityManager: m,
	}
	// TKT-D8T148: when the action is allow_acl_bypass AND the mutator offers
	// the elevated capability, expose an elevated write handle so the script
	// can call rela.bypass_acl(fn). Both conditions are required: operator opt-in
	// (the flag) and a Mutator that chooses to provide elevation. A Mutator
	// without ElevatedProvider (e.g. a restricted double) simply can't elevate.
	if action.AllowACLBypass {
		if ep, ok := m.(autocascade.ElevatedProvider); ok {
			deps.ElevatedManager = ep.Elevated()
		}
	}
	var err error
	switch {
	case action.Code != "":
		err = l.exec.ExecuteCode(ctx, action.Code, deps, action.NewEntity, action.OldEntity)
	case action.FilePath != "":
		err = l.exec.ExecuteFile(ctx, action.FilePath, deps, action.NewEntity, action.OldEntity)
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

// Compile-time assertion that *LuaScriptRunner satisfies the
// consumer-side ScriptRunner interface.
var _ autocascade.ScriptRunner = (*LuaScriptRunner)(nil)

// Compile-time assertion that [autocascade.Mutator] and [lua.Mutator]
// are structurally equivalent. Run assigns the autocascade-typed
// argument into the lua-typed field at line ~84; that assignment
// type-checks only as long as the two interfaces declare the same
// methods. This is the only file that imports both packages, so it
// is the only place the invariant can be pinned.
var _ lua.Mutator = autocascade.Mutator(nil)
