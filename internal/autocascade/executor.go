package autocascade

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// Executor is the interface Runner uses to execute automation Lua
// scripts. Defined here, at the consumer, per CLAUDE.md
// "Consumer-side interfaces for callbacks and cycles". In production
// the implementation is *script.Engine, which satisfies it
// structurally; in tests, callers pass [NopExecutor] or a stub.
//
// Executor lives in autocascade because that's where it's *consumed*.
// The script package implements it without importing autocascade
// (Go's structural typing makes this work) — there is no package
// cycle and no need for the script package to know about autocascade.
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
	panic("autocascade.NopExecutor: Lua execution not expected in this context")
}

func (nopExecutor) ExecuteFile(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("autocascade.NopExecutor: Lua execution not expected in this context")
}

func (nopExecutor) LuaCache() *lua.Cache { return nil }
