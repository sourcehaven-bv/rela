package docs

import (
	lua "github.com/yuin/gopher-lua"
)

// argString reads a string argument that may be passed positionally
// (fn("risico")) or as a named table field (fn{type="risico"}), reading the
// given key from a table first arg. Returns "" when absent.
func argString(ls *lua.LState, key string) string {
	switch v := ls.Get(1).(type) {
	case lua.LString:
		return string(v)
	case *lua.LTable:
		if s, ok := ls.GetField(v, key).(lua.LString); ok {
			return string(s)
		}
	}
	return ""
}

// argTable returns the first argument as a table, or nil.
func argTable(ls *lua.LState) *lua.LTable {
	if t, ok := ls.Get(1).(*lua.LTable); ok {
		return t
	}
	return nil
}

// fieldString reads a string field from a table, "" when absent.
func fieldString(ls *lua.LState, tbl *lua.LTable, key string) string {
	if tbl == nil {
		return ""
	}
	if s, ok := ls.GetField(tbl, key).(lua.LString); ok {
		return string(s)
	}
	return ""
}

// fieldInt reads an integer field from a table, returning def when absent.
func fieldInt(ls *lua.LState, tbl *lua.LTable, key string, def int) int {
	if tbl == nil {
		return def
	}
	if n, ok := ls.GetField(tbl, key).(lua.LNumber); ok {
		return int(n)
	}
	return def
}

// fieldStringSlice reads an array-of-strings field ({"a","b"}) from a table.
func fieldStringSlice(ls *lua.LState, tbl *lua.LTable, key string) []string {
	if tbl == nil {
		return nil
	}
	arr, ok := ls.GetField(tbl, key).(*lua.LTable)
	if !ok {
		return nil
	}
	var out []string
	arr.ForEach(func(_, v lua.LValue) {
		if s, ok := v.(lua.LString); ok {
			out = append(out, string(s))
		}
	})
	return out
}

// idArg reads an entity id from argument n: a string id, or a table with an
// "id" field (as returned by create()).
func idArg(ls *lua.LState, n int) string {
	switch v := ls.Get(n).(type) {
	case lua.LString:
		return string(v)
	case *lua.LTable:
		if s, ok := ls.GetField(v, "id").(lua.LString); ok {
			return string(s)
		}
	}
	return ""
}

// luaTableToMap converts a Lua table to a Go map, string keys only, scalar
// values. Nested tables are flattened to nil (seed props are scalar).
func luaTableToMap(t *lua.LTable) map[string]any {
	out := map[string]any{}
	t.ForEach(func(k, v lua.LValue) {
		ks, ok := k.(lua.LString)
		if !ok {
			return
		}
		switch vv := v.(type) {
		case lua.LString:
			out[string(ks)] = string(vv)
		case lua.LNumber:
			out[string(ks)] = float64(vv)
		case lua.LBool:
			out[string(ks)] = bool(vv)
		}
	})
	return out
}
