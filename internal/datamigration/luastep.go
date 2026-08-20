package datamigration

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// luaStep is the escape hatch: an operator-authored PURE TRANSFORM. The
// script defines
//
//	function migrate(entity)
//	  -- entity = { id, type, content, properties = {...} }
//	  return { properties = {a="b"}, unset = {"old"}, content = "..." }
//	end
//
// and the RUNNER applies the returned patch — the script never holds a
// write handle. That is the load-bearing property: a migration's input is by
// definition invalid under the new schema, so routing writes through the
// entitymanager (validation, automations, state machines) would reject or
// mutate mid-migration state. No rela.* module, no I/O, no os/io/debug
// libraries: the sandbox mirrors internal/lua's openSafeLibraries. Returning
// nil leaves the entity unchanged. Scripts MUST be idempotent — crash
// recovery re-runs the whole step.
type luaStep struct {
	// Entity restricts the transform to one entity type; "*" runs it over
	// every entity (including types unknown to the new schema).
	Entity string `yaml:"entity"`
	// Script is a project-root-relative path (conventionally under
	// migrations/) to the Lua source.
	Script string `yaml:"script"`
}

func (s *luaStep) Kind() string   { return "lua" }
func (s *luaStep) Target() string { return s.Entity + " ← " + s.Script }

func (s *luaStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.Script == "" {
		return fmt.Errorf("entity and script are required")
	}
	if s.Entity != "*" && !entityInShape(from, s.Entity) && !entityInShape(to, s.Entity) {
		return fmt.Errorf("entity type %q is in neither the from- nor the to-schema", s.Entity)
	}
	// Path discipline: the script must resolve inside the project (no
	// absolute paths, no traversal). fs.ValidPath enforces exactly that.
	clean := path.Clean(s.Script)
	if !fs.ValidPath(clean) || clean == "." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("script path %q must be project-relative without traversal", s.Script)
	}
	return nil
}

func (s *luaStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	src, err := fs.ReadFile(x.ScriptFS, path.Clean(s.Script))
	if err != nil {
		return res, fmt.Errorf("read script: %w", err)
	}

	ls := newSandboxedState()
	defer ls.Close()
	if err := ls.DoString(string(src)); err != nil {
		return res, fmt.Errorf("load script: %w", err)
	}
	migrateFn := ls.GetGlobal("migrate")
	if migrateFn.Type() != lua.LTFunction {
		return res, fmt.Errorf("script must define `function migrate(entity)`")
	}

	err = x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		ls.Push(migrateFn)
		ls.Push(entityToTable(ls, e))
		if err := ls.PCall(1, 1, nil); err != nil {
			return false, fmt.Errorf("migrate(): %w", err)
		}
		ret := ls.Get(-1)
		ls.Pop(1)
		return applyLuaPatch(e, ret)
	}, &res)
	return res, err
}

// newSandboxedState builds a Lua state with only safe libraries — the same
// set (and the same base-function removals) as internal/lua's runtime
// sandbox: no io, no os, no debug, no load/dofile, no raw table access.
func newSandboxedState() *lua.LState {
	ls := lua.NewState(lua.Options{
		SkipOpenLibs:  true,
		CallStackSize: 1024,
		RegistrySize:  1024 * 64,
	})
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		ls.Push(ls.NewFunction(lib.fn))
		ls.Push(lua.LString(lib.name))
		ls.Call(1, 0)
	}
	for _, g := range []string{
		"loadfile", "dofile", "load", "loadstring",
		"rawget", "rawset", "rawequal", "rawlen", "getmetatable", "setmetatable",
	} {
		ls.SetGlobal(g, lua.LNil)
	}
	return ls
}

// entityToTable converts an entity to the Lua value migrate() receives.
// (A local, dependency-free mirror of lua.EntityToTable — importing the full
// runtime package for one conversion would drag the whole binding surface
// into this package.)
func entityToTable(ls *lua.LState, e *entity.Entity) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(e.ID))
	t.RawSetString("type", lua.LString(e.Type))
	t.RawSetString("content", lua.LString(e.Content))
	props := ls.NewTable()
	for k, v := range e.Properties {
		props.RawSetString(k, goToLua(ls, v))
	}
	t.RawSetString("properties", props)
	return t
}

func goToLua(ls *lua.LState, v any) lua.LValue {
	switch val := v.(type) {
	case nil:
		return lua.LNil
	case string:
		return lua.LString(val)
	case bool:
		return lua.LBool(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case []any:
		t := ls.NewTable()
		for _, item := range val {
			t.Append(goToLua(ls, item))
		}
		return t
	case map[string]any:
		t := ls.NewTable()
		for k, item := range val {
			t.RawSetString(k, goToLua(ls, item))
		}
		return t
	default:
		return lua.LString(fmt.Sprintf("%v", val))
	}
}

// applyLuaPatch applies migrate()'s return value to the entity in place.
// nil/false = no change. Anything else must be a table with optional
// `properties` (upserts), `unset` (property names to remove) and `content`.
func applyLuaPatch(e *entity.Entity, ret lua.LValue) (bool, error) {
	switch ret.Type() {
	case lua.LTNil:
		return false, nil
	case lua.LTBool:
		if !lua.LVAsBool(ret) {
			return false, nil
		}
		return false, fmt.Errorf("migrate() returned true — return a patch table or nil")
	case lua.LTTable:
	default:
		return false, fmt.Errorf("migrate() returned %s — return a patch table or nil", ret.Type())
	}
	patch := ret.(*lua.LTable)
	changed := false

	if propsVal := patch.RawGetString("properties"); propsVal.Type() == lua.LTTable {
		var iterErr error
		propsVal.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			key, ok := k.(lua.LString)
			if !ok {
				iterErr = fmt.Errorf("patch properties keys must be strings")
				return
			}
			if e.Properties == nil {
				e.Properties = map[string]any{}
			}
			e.Properties[string(key)] = luaToGo(v)
			changed = true
		})
		if iterErr != nil {
			return false, iterErr
		}
	} else if propsVal.Type() != lua.LTNil {
		return false, fmt.Errorf("patch `properties` must be a table")
	}

	if unsetVal := patch.RawGetString("unset"); unsetVal.Type() == lua.LTTable {
		t := unsetVal.(*lua.LTable)
		for i := 1; i <= t.Len(); i++ {
			name, ok := t.RawGetInt(i).(lua.LString)
			if !ok {
				return false, fmt.Errorf("patch `unset` must list property names")
			}
			if _, has := e.Properties[string(name)]; has {
				delete(e.Properties, string(name))
				changed = true
			}
		}
	} else if unsetVal.Type() != lua.LTNil {
		return false, fmt.Errorf("patch `unset` must be a list of property names")
	}

	if contentVal := patch.RawGetString("content"); contentVal.Type() == lua.LTString {
		if newContent := string(contentVal.(lua.LString)); newContent != e.Content {
			e.Content = newContent
			changed = true
		}
	} else if contentVal.Type() != lua.LTNil {
		return false, fmt.Errorf("patch `content` must be a string")
	}

	return changed, nil
}

// luaToGo converts a returned patch value to the store representation.
// Array-shaped tables become []any, map-shaped tables map[string]any.
func luaToGo(v lua.LValue) any {
	switch val := v.(type) {
	case lua.LString:
		return string(val)
	case lua.LBool:
		return bool(val)
	case lua.LNumber:
		f := float64(val)
		if f == float64(int(f)) {
			return int(f)
		}
		return f
	case *lua.LTable:
		if n := val.Len(); n > 0 {
			out := make([]any, 0, n)
			for i := 1; i <= n; i++ {
				out = append(out, luaToGo(val.RawGetInt(i)))
			}
			return out
		}
		out := map[string]any{}
		val.ForEach(func(k, item lua.LValue) {
			if key, ok := k.(lua.LString); ok {
				out[string(key)] = luaToGo(item)
			}
		})
		if len(out) == 0 {
			return []any{}
		}
		return out
	default:
		return nil
	}
}
