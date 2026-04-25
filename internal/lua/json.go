// Lua bindings for the rela.json.* submodule.
//
// JSON encode/decode helpers exposed on the rela namespace alongside
// rela.md.* (markdown). They are placed under rela rather than http
// because nothing about JSON serialization is HTTP-specific — scripts
// reach for them in flow scripts, document renderers, and ad-hoc
// transformations as often as in API calls.
//
// Convention split (matches http.* and ai.*):
//
//	rela.json.encode(value)     -> string  (raises on encoding failure)
//	rela.json.decode(string)    -> (value, nil) | (nil, err_table)
//
// json.encode raises because the only ways it can fail are programming
// errors (cycles past the cycle-marker fallback, unsupported value
// types). json.decode returns (nil, err_table) because invalid JSON
// typically comes from external data — scripts should branch on the
// error rather than wrap each call in pcall.
//
// The error table for decode failures uses kind="bad_response" — the
// same kind http.json_decode previously used — so existing scripts
// migrate without changing their error handling.
package lua

import (
	"encoding/json"

	lua "github.com/yuin/gopher-lua"
)

// jsonMaxDecodeDepth caps recursion when converting decoded JSON back into
// Lua values. Past this depth a sentinel string is substituted to prevent
// stack-overflow DoS from a hostile or pathological input. Encode-side
// cycle protection comes from luaValueToGo's existing detection.
const jsonMaxDecodeDepth = 64

// jsonMaxDepthSentinel is what goJSONToLua substitutes when a decoded
// JSON branch is deeper than jsonMaxDecodeDepth. Visible to the script
// so the truncation surfaces (rather than silently dropping data).
const jsonMaxDepthSentinel = "<max-depth>"

// registerJSONModule adds the rela.json submodule to the rela table.
func (r *Runtime) registerJSONModule(rela *lua.LTable) {
	j := r.L.NewTable()
	r.L.SetField(j, "encode", r.L.NewFunction(luaJSONEncode))
	r.L.SetField(j, "decode", r.L.NewFunction(luaJSONDecode))
	r.L.SetField(rela, "json", j)
}

// luaJSONEncode implements rela.json.encode(value) -> string.
// Raises on wrong arg type or encoding failure. Self-referential tables
// are handled by luaValueToGo's existing cycle detection — the offending
// branch encodes as the string "<cyclic reference>" rather than crashing.
func luaJSONEncode(ls *lua.LState) int {
	val := ls.CheckAny(1)
	goVal := luaValueToGo(val)
	data, err := json.Marshal(goVal)
	if err != nil {
		ls.RaiseError("json.encode: %s", err.Error())
		return 0
	}
	ls.Push(lua.LString(string(data)))
	return 1
}

// luaJSONDecode implements rela.json.decode(string) -> (value, nil) | (nil, err_table).
// Wrong arg type raises; invalid JSON returns (nil, err_table) with kind="bad_response".
func luaJSONDecode(ls *lua.LState) int {
	str := ls.CheckString(1)
	var goVal interface{}
	if err := json.Unmarshal([]byte(str), &goVal); err != nil {
		ls.Push(lua.LNil)
		tbl := ls.NewTable()
		tbl.RawSetString("kind", lua.LString("bad_response"))
		tbl.RawSetString("status", lua.LNumber(0))
		tbl.RawSetString("message", lua.LString("json.decode: "+err.Error()))
		tbl.RawSetString("retry_after", lua.LNumber(0))
		tbl.RawSetString("details", lua.LString(err.Error()))
		ls.Push(tbl)
		return 2
	}
	ls.Push(goJSONToLua(ls, goVal))
	ls.Push(lua.LNil)
	return 2
}

// goJSONToLua converts a Go value (from json.Unmarshal) to a Lua value.
// Recursion is capped at jsonMaxDecodeDepth to prevent stack-overflow
// DoS from deeply nested JSON.
func goJSONToLua(ls *lua.LState, val interface{}) lua.LValue {
	return goJSONToLuaSafe(ls, val, 0)
}

func goJSONToLuaSafe(ls *lua.LState, val interface{}, depth int) lua.LValue {
	if depth >= jsonMaxDecodeDepth {
		return lua.LString(jsonMaxDepthSentinel)
	}
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := ls.NewTable()
		for i, item := range v {
			tbl.RawSetInt(i+1, goJSONToLuaSafe(ls, item, depth+1))
		}
		return tbl
	case map[string]interface{}:
		tbl := ls.NewTable()
		for key, item := range v {
			tbl.RawSetString(key, goJSONToLuaSafe(ls, item, depth+1))
		}
		return tbl
	default:
		// json.Unmarshal only produces the cases above, so this branch
		// should be unreachable. Treat as a programming error.
		ls.RaiseError("json.decode: unexpected value type %T from json.Unmarshal", v)
		return lua.LNil
	}
}
