package docs

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// registerModule installs the `doc` table on the Lua state with the emit
// helpers, resolvers, and raw-store seed bindings. Everything closes over dr
// (no methods on the shared lua.Runtime — keeps its god-object surface flat).
// The common emit/resolver names are also aliased into the global scope so an
// island can write `h2("x")` / `typeref{...}` without the `doc.` prefix.
func (dr *docRuntime) registerModule() {
	L := dr.rt.LState()
	doc := L.NewTable()

	fns := map[string]lua.LGFunction{
		// Emit helpers — append markdown to the statement-island buffer.
		"h1": dr.emitHeading(1),
		"h2": dr.emitHeading(2),
		"h3": dr.emitHeading(3),
		"md": dr.luaMD,
		// Resolvers.
		"count":        dr.luaCount,
		"typeref":      dr.luaTyperef,
		"values":       dr.luaValues,
		"relations":    dr.luaRelations,
		"entity":       dr.luaEntity,
		"lifecycle":    dr.luaLifecycle,
		"graph":        dr.luaGraph,
		"roles_matrix": dr.luaRolesMatrix,
		"description":  dr.luaDescription,
		// Seed (raw store — no entitymanager, no validation).
		"create": dr.seed.luaCreate,
		"link":   dr.seed.luaLink,
		// Tier B — browser capture (fails loud when no capturer is wired).
		"screenshot": dr.tierB.luaScreenshot,

		// Assertions (TKT-DOCASSERT): a manual proves its own claims.
		"shows":   dr.luaShows,
		"refuses": dr.luaRefuses,
		"permits": dr.luaPermits,
		"api":     dr.tierB.luaAPI,
	}
	for name, fn := range fns {
		nf := L.NewFunction(fn)
		L.SetField(doc, name, nf)
		L.SetGlobal(name, nf) // ergonomic global alias
	}
	L.SetGlobal("doc", doc)
}

// emit appends s to the statement-island output buffer.
func (dr *docRuntime) emit(s string) { dr.out.WriteString(s) }

// emitHeading returns a binding that emits a Markdown ATX heading of the given
// level followed by a blank line.
func (dr *docRuntime) emitHeading(level int) lua.LGFunction {
	prefix := strings.Repeat("#", level)
	return func(ls *lua.LState) int {
		text := ls.CheckString(1)
		dr.emit(fmt.Sprintf("%s %s\n\n", prefix, text))
		return 0
	}
}

// luaMD emits a block of markdown followed by a blank line.
func (dr *docRuntime) luaMD(ls *lua.LState) int {
	dr.emit(ls.CheckString(1) + "\n\n")
	return 0
}

// luaCount returns the number of seeded entities of a type (echo-friendly).
// count{type="risico"} or count("risico").
func (dr *docRuntime) luaCount(ls *lua.LState) int {
	typ := argString(ls, "type")
	n := 0
	for _, err := range dr.store.ListEntities(dr.ctx, store.EntityQuery{Type: typ}) {
		if err != nil {
			return dr.luaFail(ls, "count{type=%q}: %v", typ, err)
		}
		n++
	}
	ls.Push(lua.LNumber(n))
	return 1
}
