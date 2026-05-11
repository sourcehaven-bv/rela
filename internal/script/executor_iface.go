package script

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// Executor is the interface satisfied by [Engine] for running
// automation Lua. It exists here (rather than in the consumer package
// per the project's "interfaces at the call site" rule) because
// arch-lint prevents the consumer's package (internal/autocascade)
// from importing every package whose types it would otherwise need.
//
// The interface is identical in shape to workspace.ScriptExecutor —
// which is retained as a type alias for backwards-compatibility with
// callers that named the type before the move. New code should
// reference script.Executor directly.
//
// Deliberate transgression of the consumer-side rule: documented in
// PLAN-V6UR Decisions #5 and in autocascade's package doc.
type Executor interface {
	// ExecuteCode runs inline script code with entity context.
	ExecuteCode(code string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error

	// ExecuteFile runs a script file from the scripts/ directory.
	ExecuteFile(path string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error

	// LuaCache returns the executor's shared Lua cache, or nil if
	// the executor does not provide one. Callers that build Lua
	// runtimes directly (validation rules, lua_eval, flow, etc.)
	// pass this via lua.WithCache so every runtime in the process
	// shares cache state.
	LuaCache() *lua.Cache
}

// NopExecutor is a no-op [Executor] for tests that don't trigger Lua
// automations. It panics if actually called, making it obvious when
// a test unexpectedly triggers Lua execution.
var NopExecutor Executor = nopExecutor{}

type nopExecutor struct{}

func (nopExecutor) ExecuteCode(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("script.NopExecutor: Lua execution not expected in this context")
}

func (nopExecutor) ExecuteFile(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("script.NopExecutor: Lua execution not expected in this context")
}

func (nopExecutor) LuaCache() *lua.Cache { return nil }
