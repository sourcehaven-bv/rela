package docs

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	rlua "github.com/Sourcehaven-BV/rela/internal/lua"
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
		"create": dr.luaCreate,
		"link":   dr.luaLink,
		// Tier B — browser capture (fails loud when no capturer is wired).
		"screenshot": dr.luaScreenshot,

		// Assertions (TKT-DOCASSERT): a manual proves its own claims.
		"shows":   dr.luaShows,
		"refuses": dr.luaRefuses,
		"permits": dr.luaPermits,
		"api":     dr.luaAPI,
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

// luaCreate seeds an entity directly into the memstore (raw store — no
// validation, no state-machine gate, no ACL). create("risico", {props}, body?)
// returns the created entity as a table (so the author can `link` it).
func (dr *docRuntime) luaCreate(ls *lua.LState) int {
	typ := ls.CheckString(1)
	props := map[string]any{}
	if tbl, ok := ls.Get(2).(*lua.LTable); ok {
		props = luaTableToMap(tbl)
	}
	content := ls.OptString(3, "")

	id := dr.mintID(typ, props)
	e := &entity.Entity{ID: id, Type: typ, Properties: props, Content: content}
	if err := dr.store.CreateEntity(dr.ctx, e); err != nil {
		return dr.luaFail(ls, "create(%q): %v", typ, err)
	}
	// Record for replay into the screenshot temp project (DR-S2).
	dr.seedOps = append(dr.seedOps, SeedOp{
		Kind: "create", Type: typ, ID: id, Properties: props, Content: content,
	})
	ls.Push(rlua.EntityToTable(ls, e))
	return 1
}

// luaLink seeds a relation into the memstore. link(from, type, to) where from
// may be an id string or an entity table (as returned by create).
func (dr *docRuntime) luaLink(ls *lua.LState) int {
	from := idArg(ls, 1)
	relType := ls.CheckString(2)
	to := idArg(ls, 3)
	if _, err := dr.store.CreateRelation(dr.ctx, from, relType, to, nil); err != nil {
		return dr.luaFail(ls, "link(%q,%q,%q): %v", from, relType, to, err)
	}
	dr.seedOps = append(dr.seedOps, SeedOp{Kind: "link", From: from, RelType: relType, To: to})
	return 0
}

// mintID derives a stable id for a seeded entity: an explicit props.id if given,
// else "<type>-<n>" from a per-type counter. Fixture ids need only be unique
// within the build; the counter avoids an O(n²) full-store scan per create.
func (dr *docRuntime) mintID(typ string, props map[string]any) string {
	if v, ok := props["id"].(string); ok && v != "" {
		return v
	}
	if dr.seedCounts == nil {
		dr.seedCounts = map[string]int{}
	}
	dr.seedCounts[typ]++
	return fmt.Sprintf("%s-%d", typ, dr.seedCounts[typ])
}
