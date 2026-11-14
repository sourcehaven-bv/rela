package mapimport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"

	relalua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ProjectFiles reads project files: the hook script and the lib/ modules it
// requires. config.Loader satisfies it.
type ProjectFiles interface {
	Load(ctx context.Context, name string) ([]byte, error)
}

// hookRunner calls one mapping's hook function. It is a pure transform: the
// state has the safe standard libraries and require over lib/, and no rela
// bindings, so the hook cannot read the graph, write, or reach the network.
type hookRunner struct {
	ls *lua.LState
	fn lua.LValue
}

func newHook(ctx context.Context, files ProjectFiles, h *Hook) (*hookRunner, error) {
	if files == nil {
		return nil, errors.New("hook: no project files to load the script from")
	}
	src, err := files.Load(ctx, h.Script)
	if err != nil {
		return nil, fmt.Errorf("hook: read %s: %w", h.Script, err)
	}
	ls := relalua.NewSandbox(files, nil, nil)
	// print goes to stderr, so it cannot corrupt `-o json` on stdout.
	ls.SetGlobal("print", ls.NewFunction(hookPrint))
	// A runaway hook ends with the command.
	ls.SetContext(ctx)
	if err := ls.DoString(string(src)); err != nil {
		ls.Close()
		return nil, fmt.Errorf("hook: load %s: %w", h.Script, err)
	}
	fn := ls.GetGlobal(h.function())
	if fn.Type() != lua.LTFunction {
		ls.Close()
		return nil, fmt.Errorf("hook: %s defines no function %s(draft, record)", h.Script, h.function())
	}
	return &hookRunner{ls: ls, fn: fn}, nil
}

func (h *hookRunner) close() { h.ls.Close() }

func hookPrint(ls *lua.LState) int {
	parts := make([]string, ls.GetTop())
	for i := range parts {
		parts[i] = ls.ToStringMeta(ls.Get(i + 1)).String()
	}
	_, _ = fmt.Fprintln(os.Stderr, strings.Join(parts, "\t"))
	return 0
}

// apply calls the hook with d and the source record and merges the draft it
// returns into d. keep is false when the hook returns nil.
func (h *hookRunner) apply(d *draft, def *metamodel.EntityDef, rec *Node) (keep bool, err error) {
	ls := h.ls
	ls.Push(h.fn)
	ls.Push(draftTable(ls, d))
	ls.Push(relalua.GoToLuaValue(ls, rec.plain()))
	if err := ls.PCall(2, 1, nil); err != nil {
		return false, err
	}
	ret := ls.Get(-1)
	ls.Pop(1)
	switch t := ret.(type) {
	case *lua.LNilType:
		return false, nil
	case *lua.LTable:
		before := d.key
		if err := mergeDraft(d, def, t); err != nil {
			return false, err
		}
		if d.key = d.identity(def); d.key != before {
			return false, fmt.Errorf("the hook changed the key from %q to %q; a key must come from the mapping",
				before, d.key)
		}
		return true, nil
	default:
		return false, fmt.Errorf("returned %s; return the draft table or nil", ret.Type())
	}
}

func draftTable(ls *lua.LState, d *draft) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("type", lua.LString(d.em.Type))
	t.RawSetString("mapping", lua.LString(d.em.name))
	t.RawSetString("key", lua.LString(d.key))
	if d.id != "" {
		t.RawSetString("id", lua.LString(d.id))
	}
	t.RawSetString("properties", relalua.GoToLuaValue(ls, d.props))
	if d.content != nil {
		t.RawSetString("content", lua.LString(*d.content))
	}
	rels := ls.NewTable()
	for name, keys := range d.rels {
		list := ls.NewTable()
		for _, k := range keys {
			list.Append(lua.LString(k))
		}
		rels.RawSetString(name, list)
	}
	t.RawSetString("relations", rels)
	return t
}

// mergeDraft replaces d's id, properties, content and relations with the
// returned table's.
func mergeDraft(d *draft, def *metamodel.EntityDef, t *lua.LTable) error {
	switch v := t.RawGetString("id").(type) {
	case lua.LString:
		d.id = string(v)
	case *lua.LNilType:
		d.id = ""
	default:
		return errors.New("draft.id must be a string")
	}
	switch v := t.RawGetString("content").(type) {
	case lua.LString:
		s := string(v)
		d.content = &s
	case *lua.LNilType:
		d.content = nil
	default:
		return errors.New("draft.content must be a string")
	}
	props, ok := t.RawGetString("properties").(*lua.LTable)
	if !ok {
		return errors.New("draft.properties must be a table")
	}
	merged := map[string]any{}
	var iterErr error
	props.ForEach(func(k, v lua.LValue) {
		name, isStr := k.(lua.LString)
		if !isStr {
			iterErr = errors.New("draft.properties keys must be strings")
			return
		}
		pd, known := def.Properties[string(name)]
		if !known {
			iterErr = fmt.Errorf("draft.properties.%s: type %q has no such property", name, d.em.Type)
			return
		}
		raw := relalua.ToGo(v)
		if raw == nil {
			return
		}
		value, err := hookValue(pd, raw)
		if err != nil {
			iterErr = fmt.Errorf("draft.properties.%s: %w", name, err)
			return
		}
		merged[string(name)] = value
	})
	if iterErr != nil {
		return iterErr
	}
	d.props = merged
	d.absent = d.absent[:0]
	for _, name := range sortedKeys(d.em.Properties) {
		if _, has := merged[name]; !has {
			d.absent = append(d.absent, name)
		}
	}
	return mergeRelations(d, t)
}

func mergeRelations(d *draft, t *lua.LTable) error {
	rels, ok := t.RawGetString("relations").(*lua.LTable)
	if !ok {
		return errors.New("draft.relations must be a table")
	}
	known := map[string]bool{}
	for _, rm := range d.em.Relations {
		known[rm.Name] = true
	}
	out := map[string][]string{}
	var iterErr error
	rels.ForEach(func(k, v lua.LValue) {
		name, isStr := k.(lua.LString)
		if !isStr || !known[string(name)] {
			iterErr = fmt.Errorf("draft.relations.%s: the mapping declares no such relation", k)
			return
		}
		list, isTable := v.(*lua.LTable)
		if !isTable {
			iterErr = fmt.Errorf("draft.relations.%s must be a list of keys", name)
			return
		}
		var keys []string
		for i := 1; i <= list.Len(); i++ {
			s, isS := list.RawGetInt(i).(lua.LString)
			if !isS {
				iterErr = fmt.Errorf("draft.relations.%s must be a list of strings", name)
				return
			}
			if s != "" {
				keys = append(keys, string(s))
			}
		}
		out[string(name)] = dedup(keys)
	})
	if iterErr != nil {
		return iterErr
	}
	d.rels = out
	return nil
}

// hookValue converts a value the hook returned to the stored form: a string
// becomes an external ref's id, and a scalar on a list property a one-item
// list. Nil: never passed; the caller drops a nil value.
func hookValue(pd metamodel.PropertyDef, v any) (any, error) {
	if pd.Type == metamodel.PropertyTypeExternalRef {
		if s, ok := v.(string); ok {
			v = map[string]any{"id": s}
		}
		ref, err := metamodel.ParseExternalRef(v)
		if err != nil {
			return nil, err
		}
		return ref.Map(), nil
	}
	if items, isList := v.([]any); isList {
		out := make([]any, len(items))
		for i, it := range items {
			c, err := hookScalar(pd, it)
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	}
	c, err := hookScalar(pd, v)
	if err != nil || !pd.List {
		return c, err
	}
	return []any{c}, nil
}

// hookScalar converts one scalar from the hook as the mapping converts
// source text: Lua has one number type, so an integer property gets an
// int64, and text on an integer or boolean property is parsed.
func hookScalar(pd metamodel.PropertyDef, v any) (any, error) {
	switch t := v.(type) {
	case float64:
		if pd.Type != metamodel.PropertyTypeInteger {
			return v, nil
		}
		if t != math.Trunc(t) {
			return nil, fmt.Errorf("%v is not an integer", t)
		}
		return int64(t), nil
	case string:
		if pd.Type == metamodel.PropertyTypeInteger || pd.Type == metamodel.PropertyTypeBoolean {
			return coerce(pd, t)
		}
	}
	return v, nil
}
