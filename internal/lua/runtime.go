// Package lua provides a Lua scripting runtime for rela with bindings
// to query entities, relations, and output results.
package lua

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// Runtime wraps gopher-lua VM with rela bindings.
type Runtime struct {
	L           *lua.LState
	ws          *workspace.Workspace
	meta        *metamodel.Metamodel
	stdout      io.Writer
	projectRoot string
}

// New creates a Runtime with rela bindings registered.
func New(ws *workspace.Workspace, meta *metamodel.Metamodel, projectRoot string, stdout io.Writer) *Runtime {
	L := lua.NewState()

	r := &Runtime{
		L:           L,
		ws:          ws,
		meta:        meta,
		stdout:      stdout,
		projectRoot: projectRoot,
	}

	r.registerBindings()
	return r
}

// RunFile executes a Lua script file with arguments.
func (r *Runtime) RunFile(path string, args []string) error {
	// Set rela.args
	argsTable := r.L.NewTable()
	for i, arg := range args {
		argsTable.RawSetInt(i+1, lua.LString(arg))
	}
	relaTable, ok := r.L.GetGlobal("rela").(*lua.LTable)
	if !ok {
		return fmt.Errorf("rela module not initialized")
	}
	relaTable.RawSetString("args", argsTable)

	return r.L.DoFile(path)
}

// Close releases Lua VM resources.
func (r *Runtime) Close() {
	r.L.Close()
}

// registerBindings sets up the rela module with all functions.
func (r *Runtime) registerBindings() {
	rela := r.L.NewTable()

	// Entity functions
	r.L.SetField(rela, "get_entity", r.L.NewFunction(r.luaGetEntity))
	r.L.SetField(rela, "list_entities", r.L.NewFunction(r.luaListEntities))

	// Relation functions
	r.L.SetField(rela, "get_relations", r.L.NewFunction(r.luaGetRelations))

	// Graph traversal
	r.L.SetField(rela, "trace_from", r.L.NewFunction(r.luaTraceFrom))
	r.L.SetField(rela, "trace_to", r.L.NewFunction(r.luaTraceTo))

	// Output functions
	r.L.SetField(rela, "output", r.L.NewFunction(r.luaOutput))
	r.L.SetField(rela, "write_file", r.L.NewFunction(r.luaWriteFile))

	// Context
	r.L.SetField(rela, "project_root", lua.LString(r.projectRoot))
	r.L.SetField(rela, "args", r.L.NewTable()) // Will be set before running script

	r.L.SetGlobal("rela", rela)
}

// luaGetEntity implements rela.get_entity(id) -> table|nil
func (r *Runtime) luaGetEntity(ls *lua.LState) int {
	id := ls.CheckString(1)

	entity, found := r.ws.GetEntity(id)
	if !found {
		ls.Push(lua.LNil)
		return 1
	}

	ls.Push(entityToTable(ls, entity))
	return 1
}

// luaListEntities implements rela.list_entities(type, filter?) -> table
func (r *Runtime) luaListEntities(ls *lua.LState) int {
	entityType := ls.CheckString(1)
	if entityType == "" {
		ls.RaiseError("entity type cannot be empty")
		return 0
	}
	filterExpr := ls.OptString(2, "")

	entities := r.ws.EntitiesByType(entityType)

	// Apply filter if provided
	if filterExpr != "" {
		f, err := filter.Parse(filterExpr)
		if err != nil {
			ls.RaiseError("invalid filter: %s", err.Error())
			return 0
		}

		entityDef, found := r.meta.GetEntityDef(entityType)
		if !found {
			ls.RaiseError("unknown entity type: %s", entityType)
			return 0
		}

		filters := []*filter.Filter{f}
		filtered := make([]*model.Entity, 0)
		for _, e := range entities {
			match, err := filter.MatchAll(e, filters, entityDef, r.meta)
			if err != nil {
				ls.RaiseError("filter error: %s", err.Error())
				return 0
			}
			if match {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	result := ls.NewTable()
	for i, e := range entities {
		result.RawSetInt(i+1, entityToTable(ls, e))
	}
	ls.Push(result)
	return 1
}

// luaGetRelations implements rela.get_relations(opts?) -> table
// opts can have: from, type, to
func (r *Runtime) luaGetRelations(ls *lua.LState) int {
	var fromFilter, typeFilter, toFilter string

	// Parse options table if provided
	if ls.GetTop() >= 1 && ls.Get(1).Type() == lua.LTTable {
		opts := ls.CheckTable(1)
		if v, ok := opts.RawGetString("from").(lua.LString); ok {
			fromFilter = string(v)
		}
		if v, ok := opts.RawGetString("type").(lua.LString); ok {
			typeFilter = string(v)
		}
		if v, ok := opts.RawGetString("to").(lua.LString); ok {
			toFilter = string(v)
		}
	}

	relations := r.ws.AllRelations()

	result := ls.NewTable()
	idx := 1
	for _, rel := range relations {
		// Apply filters
		if fromFilter != "" && rel.From != fromFilter {
			continue
		}
		if typeFilter != "" && rel.Type != typeFilter {
			continue
		}
		if toFilter != "" && rel.To != toFilter {
			continue
		}

		result.RawSetInt(idx, relationToTable(ls, rel))
		idx++
	}
	ls.Push(result)
	return 1
}

// luaTraceFrom implements rela.trace_from(id, depth?) -> table|nil
func (r *Runtime) luaTraceFrom(ls *lua.LState) int {
	id := ls.CheckString(1)
	maxDepth := ls.OptInt(2, 0)

	trace := r.ws.TraceFrom(id, maxDepth)
	if trace == nil {
		ls.Push(lua.LNil)
		return 1
	}
	ls.Push(traceResultToTable(ls, trace))
	return 1
}

// luaTraceTo implements rela.trace_to(id, depth?) -> table|nil
func (r *Runtime) luaTraceTo(ls *lua.LState) int {
	id := ls.CheckString(1)
	maxDepth := ls.OptInt(2, 0)

	trace := r.ws.TraceTo(id, maxDepth)
	if trace == nil {
		ls.Push(lua.LNil)
		return 1
	}
	ls.Push(traceResultToTable(ls, trace))
	return 1
}

// luaOutput implements rela.output(data) - JSON encode to stdout
func (r *Runtime) luaOutput(ls *lua.LState) int {
	data := ls.CheckAny(1)

	goData := luaValueToGo(data)

	encoder := json.NewEncoder(r.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(goData); err != nil {
		ls.RaiseError("JSON encoding error: %s", err.Error())
		return 0
	}
	return 0
}

// luaWriteFile implements rela.write_file(path, content)
// Path must be within the project root for security.
func (r *Runtime) luaWriteFile(ls *lua.LState) int {
	path := ls.CheckString(1)
	content := ls.CheckString(2)

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		ls.RaiseError("invalid path: %s", err.Error())
		return 0
	}

	// Validate path is within project root
	absRoot, err := filepath.Abs(r.projectRoot)
	if err != nil {
		ls.RaiseError("invalid project root: %s", err.Error())
		return 0
	}

	// Ensure the path starts with project root (after cleaning)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		ls.RaiseError("write_file: path must be within project root")
		return 0
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		ls.RaiseError("write file error: %s", err.Error())
		return 0
	}
	return 0
}

// entityToTable converts a model.Entity to a Lua table.
func entityToTable(ls *lua.LState, e *model.Entity) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(e.ID))
	t.RawSetString("type", lua.LString(e.Type))
	t.RawSetString("content", lua.LString(e.Content))

	props := ls.NewTable()
	for k, v := range e.Properties {
		props.RawSetString(k, goToLuaValue(ls, v))
	}
	t.RawSetString("properties", props)

	return t
}

// relationToTable converts a model.Relation to a Lua table.
func relationToTable(ls *lua.LState, rel *model.Relation) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("from", lua.LString(rel.From))
	t.RawSetString("type", lua.LString(rel.Type))
	t.RawSetString("to", lua.LString(rel.To))

	if len(rel.Properties) > 0 {
		props := ls.NewTable()
		for k, v := range rel.Properties {
			props.RawSetString(k, goToLuaValue(ls, v))
		}
		t.RawSetString("properties", props)
	}

	return t
}

// traceResultToTable converts a trace result tree to a Lua table.
func traceResultToTable(ls *lua.LState, trace *model.TraceResult) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(trace.ID))
	t.RawSetString("type", lua.LString(trace.Type))
	t.RawSetString("title", lua.LString(trace.Title))
	t.RawSetString("depth", lua.LNumber(trace.Depth))
	t.RawSetString("relation", lua.LString(trace.Relation))
	t.RawSetString("incoming", lua.LBool(trace.Incoming))

	// Convert children recursively
	children := ls.NewTable()
	for i, child := range trace.Children {
		children.RawSetInt(i+1, traceResultToTable(ls, child))
	}
	t.RawSetString("children", children)

	return t
}

// goToLuaValue converts a Go value to a Lua value.
func goToLuaValue(ls *lua.LState, v interface{}) lua.LValue {
	if v == nil {
		return lua.LNil
	}
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	case []interface{}:
		t := ls.NewTable()
		for i, item := range val {
			t.RawSetInt(i+1, goToLuaValue(ls, item))
		}
		return t
	case []string:
		t := ls.NewTable()
		for i, item := range val {
			t.RawSetInt(i+1, lua.LString(item))
		}
		return t
	case map[string]interface{}:
		t := ls.NewTable()
		for k, item := range val {
			t.RawSetString(k, goToLuaValue(ls, item))
		}
		return t
	default:
		// Fallback: convert to string
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

// luaValueToGo converts a Lua value to a Go value.
func luaValueToGo(lv lua.LValue) interface{} {
	switch v := lv.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		return luaTableToGo(v)
	case *lua.LNilType:
		return nil
	default:
		return nil
	}
}

// maxArraySize is the maximum size for arrays converted from Lua tables.
const maxArraySize = 100000

// luaTableToGo converts a Lua table to a Go map or slice.
func luaTableToGo(t *lua.LTable) interface{} {
	// Check if it's an array (sequential positive integer keys starting at 1)
	isArray := true
	maxN := 0
	t.ForEach(func(k, _ lua.LValue) {
		if kn, ok := k.(lua.LNumber); ok {
			f := float64(kn)
			// Must be a positive integer within bounds
			if f != math.Floor(f) || f < 1 || f > maxArraySize {
				isArray = false
				return
			}
			n := int(f)
			if n > maxN {
				maxN = n
			}
		} else {
			isArray = false
		}
	})

	if isArray && maxN > 0 {
		arr := make([]interface{}, maxN)
		t.ForEach(func(k, v lua.LValue) {
			if kn, ok := k.(lua.LNumber); ok {
				idx := int(kn) - 1
				if idx >= 0 && idx < maxN {
					arr[idx] = luaValueToGo(v)
				}
			}
		})
		return arr
	}

	// It's a map
	m := make(map[string]interface{})
	t.ForEach(func(k, v lua.LValue) {
		var key string
		switch kv := k.(type) {
		case lua.LString:
			key = string(kv)
		case lua.LNumber:
			key = fmt.Sprintf("%v", float64(kv))
		default:
			key = k.String()
		}
		m[key] = luaValueToGo(v)
	})
	return m
}
