package lua

import (
	"bytes"
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// luaExecutionTimeout is the maximum time allowed for a Lua script to run.
// This prevents infinite loops or resource exhaustion from automation scripts.
const luaExecutionTimeout = 30 * time.Second

// Executor implements workspace.LuaExecutor by creating a Lua runtime,
// setting entity globals, and executing code.
type Executor struct {
	ws          WorkspaceInterface
	meta        *metamodel.Metamodel
	projectRoot string
}

// NewExecutor creates a LuaExecutor that can run Lua code with entity context.
func NewExecutor(ws WorkspaceInterface, meta *metamodel.Metamodel, projectRoot string) *Executor {
	return &Executor{
		ws:          ws,
		meta:        meta,
		projectRoot: projectRoot,
	}
}

// ExecuteCode runs Lua code with entity and oldEntity available as globals.
// This implements workspace.LuaExecutor.
func (e *Executor) ExecuteCode(code string, entity, oldEntity *model.Entity) error {
	var output bytes.Buffer
	runtime := New(e.ws, e.meta, e.projectRoot, &output)
	defer runtime.Close()

	// Set execution timeout to prevent infinite loops or resource exhaustion.
	ls := runtime.LState()
	ctx, cancel := context.WithTimeout(context.Background(), luaExecutionTimeout)
	defer cancel()
	ls.SetContext(ctx)

	// Set entity context as Lua globals
	if entity != nil {
		ls.SetGlobal("entity", EntityToTable(ls, entity))
	}
	if oldEntity != nil {
		ls.SetGlobal("old_entity", EntityToTable(ls, oldEntity))
	}

	// Execute the code
	return runtime.RunString(code)
}
