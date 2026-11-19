package lua

import (
	"context"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// instantGuarded names the rela.* bindings that act outside the store or
// read the version history directly. The store answers a read bound to an
// instant from the state at that instant and refuses its writes
// (store.ErrReadOnlyInstant). These bindings bypass that seam, so under an
// instant they would write today's state from a page showing the past, or
// read history past the instant. They refuse instead.
var instantGuarded = []string{
	"attach_file", "attach_render", "write_file", "sync",
	"history", "get_version", "version_by_tag", "tagged_entities", "version_token",
	"tag_version", "untag_version", "get_mentioned_by",
}

// instantGuardedModules are the global modules whose every function has an
// external side effect.
var instantGuardedModules = []string{"ai", "http", "mail"}

// guardInstantBindings wraps the bindings listed above so that a call made
// while the caller's context is bound to a read instant raises an error. The
// check runs per call, because the context is the caller's, not the one the
// runtime was built with.
func guardInstantBindings(ls *lua.LState, rela *lua.LTable, ctxFn func() context.Context) {
	wrap := func(tbl *lua.LTable, name string) {
		fn, ok := tbl.RawGetString(name).(*lua.LFunction)
		if !ok {
			return
		}
		tbl.RawSetString(name, ls.NewFunction(instantGuard{name: name, fn: fn, ctxFn: ctxFn}.call))
	}
	for _, name := range instantGuarded {
		wrap(rela, name)
	}
	if piles, ok := rela.RawGetString("piles").(*lua.LTable); ok {
		wrap(piles, "add")
		wrap(piles, "remove")
	}
	if oauth, ok := rela.RawGetString("oauth").(*lua.LTable); ok {
		wrap(oauth, "access_token") // a token is for a call outside the store
		wrap(oauth, "invalidate")
	}
	for _, mod := range instantGuardedModules {
		tbl, ok := ls.GetGlobal(mod).(*lua.LTable)
		if !ok {
			continue
		}
		var names []string
		tbl.ForEach(func(k, _ lua.LValue) {
			if name, ok := k.(lua.LString); ok {
				names = append(names, string(name))
			}
		})
		for _, name := range names {
			wrap(tbl, name)
		}
	}
}

// instantGuard is one wrapped binding. It registers as a method value, not a
// closure, because contextcheck follows a closure back to every runtime
// constructor (see registerMailModule).
type instantGuard struct {
	name  string
	fn    *lua.LFunction
	ctxFn func() context.Context
}

// call raises an error while the caller's context is bound to a read instant,
// and otherwise calls the wrapped binding with the same arguments and returns
// its results.
func (g instantGuard) call(ls *lua.LState) int {
	if _, past := store.ReadInstant(g.ctxFn()); past {
		ls.RaiseError("rela: %s is not available while reading a past instant", g.name)
		return 0
	}
	n := ls.GetTop()
	ls.Push(g.fn)
	for i := 1; i <= n; i++ {
		ls.Push(ls.Get(i))
	}
	ls.Call(n, lua.MultRet)
	return ls.GetTop() - n
}
