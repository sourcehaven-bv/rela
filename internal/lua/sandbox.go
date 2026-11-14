package lua

import (
	lua "github.com/yuin/gopher-lua"
)

// NewSandbox returns a bare Lua state with the script sandbox: the safe
// standard libraries only (no io, os, debug or load*) and rela's own require
// over files' lib/ directory. It registers no rela bindings. A host that
// defines its own module (the `rela test` runner) builds on it, so its code
// runs under the same restrictions as a project script.
//
// moduleDirs are project directories require searches after lib/, in order
// (`rela test` passes tests/). A module in lib/ wins over one of the same
// name in a later directory.
//
// preload maps module names to loaders. The first require of a preloaded
// name calls its loader, and every require returns that value; lib/ is not
// consulted for the name. A preloaded name may use a reserved first segment
// ("rela.test"), which a lib/ module may not.
//
// Nil: files is accepted — require then loads only the preloaded modules and
// the standard libraries. moduleDirs and preload are accepted.
//
// The caller closes the state.
func NewSandbox(
	files ProjectFiles, moduleDirs []string, preload map[string]func(*lua.LState) lua.LValue,
) *lua.LState {
	ls := lua.NewState(lua.Options{
		SkipOpenLibs:  true,
		CallStackSize: 1024,
		RegistrySize:  1024 * 64,
	})
	openSafeLibraries(ls)
	installRequire(ls, files).extraDirs = moduleDirs
	if len(preload) > 0 {
		installPreload(ls, preload)
	}
	return ls
}

// installPreload wraps the state's require so it answers the names in
// preload from their loaders, each called once.
func installPreload(ls *lua.LState, preload map[string]func(*lua.LState) lua.LValue) {
	inner := ls.GetGlobal("require")
	loaded := map[string]lua.LValue{}
	ls.SetGlobal("require", ls.NewFunction(func(ls *lua.LState) int {
		name, ok := ls.Get(1).(lua.LString)
		load, found := preload[string(name)]
		if !ok || !found {
			ls.Push(inner)
			ls.Push(ls.Get(1))
			ls.Call(1, 1)
			return 1
		}
		v, done := loaded[string(name)]
		if !done {
			v = load(ls)
			loaded[string(name)] = v
		}
		ls.Push(v)
		return 1
	}))
}

// ToGo converts a Lua value to the Go value the write path takes: a string,
// bool, int64 or float64, a []any for a sequence, a map[string]any for a
// table with keys, or nil.
func ToGo(lv lua.LValue) any {
	return luaValueToGo(lv)
}
