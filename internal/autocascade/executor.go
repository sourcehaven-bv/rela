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
//
// The interface is intentionally narrow: only the two methods Runner
// invokes. Other consumers of *script.Engine (validation rules, MCP
// lua_eval, CLI flow) that need additional methods like LuaCache()
// should depend on a bundle they declare themselves.
type Executor interface {
	// ExecuteCode runs inline script code with entity context.
	ExecuteCode(code string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error

	// ExecuteFile runs a script file from the scripts/ directory.
	ExecuteFile(path string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error
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
