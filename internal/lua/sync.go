// Lua bindings for sync connectors (TKT-SM20FG).
//
//	rela.find_by_external_ref(system, id) -> entity table | nil
//	rela.sync.merge(base, ours, theirs, {fields = {...}}) -> result table
//	rela.sync.EMPTY                       -> the "cleared" marker
//
// find_by_external_ref reads through the runtime's reader, so a hidden
// entity or a hidden ref reads as no match. It raises on a system no type
// declares, and when more than one readable entity holds the id. A nil
// answer does not mean the id is free: create still runs the unique check
// against every holder.
//
// rela.sync.merge is pure: it classifies the fields with internal/syncmerge
// and returns {base_unknown, complete, retag, write, content, push,
// conflicts, unchanged}. complete is true when theirs reported every synced
// field; a loop moves its base only after a complete report. write maps property to value, with rela.sync.EMPTY for a
// clear, and goes straight into rela.update_entity; content is the body to
// write, or nil.
//
// Free functions: Runtime is at its plimsoll method cap.
package lua

import (
	"context"
	"errors"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// ExternalRefFinder is the part of a script read surface that answers
// rela.find_by_external_ref. visibility.ScriptReader and
// visibility.UnrestrictedReader satisfy it.
type ExternalRefFinder interface {
	FindByExternalRef(ctx context.Context, holders []visibility.ExternalRefHolder, id string) ([]*entity.Entity, error)
}

// syncReadyReader reports whether the reader serves version history and
// tags. Without it a reader is treated as not ready.
type syncReadyReader interface {
	SyncReady() bool
}

// syncEmptyMarker is the Go value of the rela.sync.EMPTY userdata. A script
// cannot make a userdata holding it, so recognizing it is unforgeable.
type syncEmptyMarker struct{}

// Argument positions of rela.sync.merge(base, ours, theirs, opts).
const (
	argPosMergeBase = iota + 1
	argPosMergeOurs
	argPosMergeTheirs
	argPosMergeOpts
)

// errSyncNeedsHistory is the use-time refusal of D11.
const errSyncNeedsHistory = "sync needs version history (the SQLite or PostgreSQL build)"

type syncBindings struct {
	r     *Runtime
	empty *lua.LUserData
}

// registerSyncBindings installs rela.find_by_external_ref and rela.sync.
func registerSyncBindings(r *Runtime, rela *lua.LTable) {
	empty := r.L.NewUserData()
	empty.Value = syncEmptyMarker{}
	sb := syncBindings{r: r, empty: empty}
	r.L.SetField(rela, "find_by_external_ref", r.L.NewFunction(sb.luaFindByExternalRef))
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "merge", r.L.NewFunction(sb.luaMerge))
	r.L.SetField(tbl, "EMPTY", empty)
	r.L.SetField(rela, "sync", tbl)
}

// isSyncEmpty reports whether v is rela.sync.EMPTY.
func isSyncEmpty(v lua.LValue) bool {
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return false
	}
	_, ok = ud.Value.(syncEmptyMarker)
	return ok
}

// splitEmpty removes the properties a script set to rela.sync.EMPTY from
// props and returns their names: they are unsets, not values.
func splitEmpty(tbl *lua.LTable, props map[string]any) []string {
	var unset []string
	tbl.ForEach(func(k, v lua.LValue) {
		if name, ok := k.(lua.LString); ok && isSyncEmpty(v) {
			unset = append(unset, string(name))
			delete(props, string(name))
		}
	})
	return unset
}

func (sb syncBindings) luaFindByExternalRef(ls *lua.LState) int {
	const binding = "rela.find_by_external_ref"
	system := ls.CheckString(1)
	id := ls.CheckString(2)
	holders := metamodel.ExternalRefProps(sb.r.deps.Meta, system)
	if len(holders) == 0 {
		ls.RaiseError("%s: no type declares an external ref for system %q", binding, system)
		return 0
	}
	rd, ok := sb.r.reader(ls, binding)
	if !ok {
		return 0
	}
	f, ok := rd.(ExternalRefFinder)
	if !ok {
		ls.RaiseError("%s: this runtime cannot look up external refs", binding)
		return 0
	}
	vh := make([]visibility.ExternalRefHolder, len(holders))
	for i, h := range holders {
		vh[i] = visibility.ExternalRefHolder{Type: h.Type, Property: h.Property, Sync: h.Sync}
	}
	found, err := f.FindByExternalRef(sb.r.callerCtx(), vh, id)
	if err != nil {
		if errors.Is(err, store.ErrHistoryUnsupported) {
			ls.RaiseError("%s: %s", binding, errSyncNeedsHistory)
			return 0
		}
		ls.RaiseError("%s: %s", binding, err)
		return 0
	}
	switch len(found) {
	case 0:
		ls.Push(lua.LNil)
	case 1:
		ls.Push(EntityToTable(ls, found[0]))
	default:
		ls.RaiseError("%s: %d entities you can read hold %s id %q", binding, len(found), system, id)
		return 0
	}
	return 1
}

func (sb syncBindings) luaMerge(ls *lua.LState) int {
	const binding = "rela.sync.merge"
	rd, ok := sb.r.reader(ls, binding)
	if !ok {
		return 0
	}
	if sr, ready := rd.(syncReadyReader); !ready || !sr.SyncReady() {
		ls.RaiseError("%s: %s", binding, errSyncNeedsHistory)
		return 0
	}
	var base *syncmerge.State
	if t, isTable := ls.Get(argPosMergeBase).(*lua.LTable); isTable {
		s := stateFromTable(t)
		base = &s
	} else if ls.Get(argPosMergeBase) != lua.LNil {
		ls.RaiseError("%s: base must be an entity table or nil", binding)
		return 0
	}
	oursTbl := ls.CheckTable(argPosMergeOurs)
	theirsTbl := ls.CheckTable(argPosMergeTheirs)
	opts := ls.CheckTable(argPosMergeOpts)

	entityType := lua.LVAsString(oursTbl.RawGetString("type"))
	var names []string
	fieldsTbl, ok := opts.RawGetString("fields").(*lua.LTable)
	if !ok {
		ls.RaiseError("%s: opts.fields must be a list of field names", binding)
		return 0
	}
	fieldsTbl.ForEach(func(_, v lua.LValue) { names = append(names, lua.LVAsString(v)) })
	fields, err := syncmerge.FieldsFor(sb.r.deps.Meta, entityType, names)
	if err != nil {
		ls.RaiseError("%s: %s", binding, err)
		return 0
	}
	theirs := syncmerge.Theirs{Properties: map[string]any{}}
	if pt, ok := theirsTbl.RawGetString("properties").(*lua.LTable); ok {
		pt.ForEach(func(k, v lua.LValue) {
			if isSyncEmpty(v) {
				theirs.Properties[lua.LVAsString(k)] = syncmerge.Empty
				return
			}
			theirs.Properties[lua.LVAsString(k)] = luaValueToGo(v)
		})
	}
	// D6: absent content is not reported, EMPTY clears, a string is the
	// body; anything else is a connector bug, not a value to guess at.
	switch c := theirsTbl.RawGetString("content"); {
	case c == lua.LNil:
	case isSyncEmpty(c):
		s := ""
		theirs.Content = &s
	case c.Type() == lua.LTString:
		s := lua.LVAsString(c)
		theirs.Content = &s
	default:
		ls.RaiseError("%s: theirs.content must be a string or rela.sync.EMPTY, got %s", binding, c.Type())
		return 0
	}
	res, err := syncmerge.Merge(fields, base, stateFromTable(oursTbl), theirs)
	if err != nil {
		ls.RaiseError("%s: %s", binding, err)
		return 0
	}
	ls.Push(sb.resultTable(ls, res))
	return 1
}

// stateFromTable reads an entity table (rela.get_entity,
// rela.version_by_tag) as a merge side.
func stateFromTable(t *lua.LTable) syncmerge.State {
	s := syncmerge.State{Properties: map[string]any{}}
	if pt, ok := t.RawGetString("properties").(*lua.LTable); ok {
		if m, ok := luaValueToGo(pt).(map[string]any); ok {
			s.Properties = m
		}
	}
	s.Content = lua.LVAsString(t.RawGetString("content"))
	if rt, ok := t.RawGetString("redacted").(*lua.LTable); ok {
		rt.ForEach(func(k, _ lua.LValue) { s.Redacted = append(s.Redacted, lua.LVAsString(k)) })
	}
	return s
}

func (sb syncBindings) resultTable(ls *lua.LState, res syncmerge.Result) *lua.LTable {
	out := ls.NewTable()
	out.RawSetString("base_unknown", lua.LBool(res.BaseUnknown))
	out.RawSetString("complete", lua.LBool(res.Complete))
	out.RawSetString("retag", lua.LBool(res.Retag))
	write := ls.NewTable()
	for k, v := range res.Write {
		if v == nil {
			write.RawSetString(k, sb.empty)
			continue
		}
		write.RawSetString(k, GoToLuaValue(ls, v))
	}
	out.RawSetString("write", write)
	if res.WriteContent != nil {
		out.RawSetString("content", lua.LString(*res.WriteContent))
	}
	push := ls.NewTable()
	for k, v := range res.Push {
		if v == nil || metamodel.IsEmptyValue(v) {
			push.RawSetString(k, sb.empty)
			continue
		}
		push.RawSetString(k, GoToLuaValue(ls, v))
	}
	out.RawSetString("push", push)
	conflicts := ls.NewTable()
	for _, c := range res.Conflicts {
		ct := ls.NewTable()
		ct.RawSetString("field", lua.LString(c.Field))
		ct.RawSetString("base", GoToLuaValue(ls, c.Base))
		ct.RawSetString("ours", GoToLuaValue(ls, c.Ours))
		ct.RawSetString("theirs", GoToLuaValue(ls, c.Theirs))
		conflicts.Append(ct)
	}
	out.RawSetString("conflicts", conflicts)
	unchanged := ls.NewTable()
	for _, f := range res.Unchanged {
		unchanged.Append(lua.LString(f))
	}
	out.RawSetString("unchanged", unchanged)
	return out
}
