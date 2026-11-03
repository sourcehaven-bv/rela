package datamigration

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// graphReader is what a lua step may read (TKT-ZO6GRZ). It is declared here,
// at the call site, and it has no write method: the step stays a pure
// transform because the binding cannot reach a writer, not because it
// promises not to. store.Store satisfies it.
type graphReader interface {
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// installGraphReads sets the global `rela` table of a migration sandbox to
// the read functions rela.get_entity, rela.get_faces and rela.get_relations.
//
// # What a read sees
//
// Reads go to the raw store: every face row, no ACL, no validation. That is
// the runner's trust model, and the data may be invalid under the new schema
// halfway through a migration.
//
// No write of the step itself is visible to a read in that step. This
// follows from Exec.forEachEntity being collect-then-write: it calls
// migrate() for every entity before it writes any of them. The stores hand
// out copies, so the in-place patch of the entity being migrated does not
// leak into a later read either. Writes of one step land before the next step
// starts, so a later step reads the results of earlier ones.
//
// The guarantee is about the migration's own writes. The runner's lock
// excludes other migration and GC runs, not ordinary writers: a write made
// through a running server during the step can be visible to some reads and
// not to others, exactly as it can to the step's own iteration.
func installGraphReads(ctx context.Context, ls *lua.LState, rd graphReader) {
	t := ls.NewTable()
	ls.SetField(t, "get_entity", ls.NewFunction(func(ls *lua.LState) int {
		return luaGetEntity(ctx, ls, rd)
	}))
	ls.SetField(t, "get_faces", ls.NewFunction(func(ls *lua.LState) int {
		return luaGetFaces(ctx, ls, rd)
	}))
	ls.SetField(t, "get_relations", ls.NewFunction(func(ls *lua.LState) int {
		return luaGetRelations(ctx, ls, rd)
	}))
	ls.SetGlobal("rela", t)
}

// stringArg returns argument 1 as a non-empty string. A number is refused
// rather than coerced the way CheckString would: rela.get_entity(12) is a
// script bug, not a request for entity "12".
func stringArg(ls *lua.LState, fn string) (string, bool) {
	v, ok := ls.Get(1).(lua.LString)
	if !ok {
		ls.RaiseError("%s: id must be a string, got %s", fn, ls.Get(1).Type())
		return "", false
	}
	if v == "" {
		ls.RaiseError("%s: entity id cannot be empty", fn)
		return "", false
	}
	return string(v), true
}

// luaGetEntity implements rela.get_entity(addr) -> table|nil.
//
// "ID@face" reads that row. A bare id reads the family: no row is nil, one
// row is that row, and several rows raise. A migration has no world to rank
// faces by, and choosing one silently would make the script's answer depend
// on a face it never named. rela.get_faces is the way to read them all.
func luaGetEntity(ctx context.Context, ls *lua.LState, rd graphReader) int {
	raw, ok := stringArg(ls, "rela.get_entity")
	if !ok {
		return 0
	}
	addr, err := entity.ParseAddress(raw)
	if err != nil {
		ls.RaiseError("rela.get_entity: %s", err.Error())
		return 0
	}
	e, found, err := readAddress(ctx, rd, addr)
	if err != nil {
		ls.RaiseError("rela.get_entity(%q): %s", raw, err.Error())
		return 0
	}
	if !found {
		ls.Push(lua.LNil)
		return 1
	}
	ls.Push(entityToTable(ls, e))
	return 1
}

// readAddress returns the row addr names; found is false when there is none.
func readAddress(ctx context.Context, rd graphReader, addr entity.Address) (e *entity.Entity, found bool, err error) {
	if ref, named := addr.Named(); named {
		e, err = rd.GetEntity(ctx, ref)
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, nil
		}
		return e, err == nil, err
	}
	rows, err := store.Family(ctx, rd, addr.ID())
	if err != nil {
		return nil, false, err
	}
	switch len(rows) {
	case 0:
		return nil, false, nil
	case 1:
		return rows[0], true, nil
	}
	faces := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Face.IsImplicit() {
			faces = append(faces, "(no face)")
			continue
		}
		faces = append(faces, r.Face.String())
	}
	slices.Sort(faces)
	return nil, false, fmt.Errorf("entity is stored at %d faces (%s); name one as %s@<face> or use rela.get_faces",
		len(rows), strings.Join(faces, ", "), addr.ID())
}

// luaGetFaces implements rela.get_faces(id) -> table: every stored row of
// id, one per face, sorted by face; empty when there is none. It is how a
// script reads an entity it reached through a relation, whose head is a
// bare id, when that entity's type has faces.
func luaGetFaces(ctx context.Context, ls *lua.LState, rd graphReader) int {
	id, ok := stringArg(ls, "rela.get_faces")
	if !ok {
		return 0
	}
	if strings.Contains(id, "@") {
		ls.RaiseError("rela.get_faces: takes a bare id, got %q", id)
		return 0
	}
	rows, err := store.Family(ctx, rd, id)
	if err != nil {
		ls.RaiseError("rela.get_faces(%q): %s", id, err.Error())
		return 0
	}
	slices.SortFunc(rows, func(a, b *entity.Entity) int { return strings.Compare(a.Face.String(), b.Face.String()) })
	result := ls.NewTable()
	for _, e := range rows {
		result.Append(entityToTable(ls, e))
	}
	ls.Push(result)
	return 1
}

// luaGetRelations implements rela.get_relations(opts?) -> table.
//
// opts takes from, type and to, as in the normal scripting API, but checked
// more strictly (see relationQueryArg), and a store error raises instead of
// returning the rows read so far.
func luaGetRelations(ctx context.Context, ls *lua.LState, rd graphReader) int {
	q, err := relationQueryArg(ls)
	if err != nil {
		ls.RaiseError("rela.get_relations: %s", err.Error())
		return 0
	}
	result := ls.NewTable()
	for rel, err := range rd.ListRelations(ctx, q) {
		if err != nil {
			ls.RaiseError("rela.get_relations: %s", err.Error())
			return 0
		}
		result.Append(relationToTable(ls, rel))
	}
	ls.Push(result)
	return 1
}

// relationQueryArg reads get_relations' options table.
//
// Every mistake that would otherwise widen the query raises, because the
// script reads a whole-graph answer as the filtered one it asked for: a
// non-table argument (the normal API returns every relation for
// get_relations(id)), an unknown key (a typo such as `form`), a value that
// is not a string, an empty string, and an empty table. The last one is what
// `{from = e.properties.owner}` becomes when owner is missing, since Lua
// cannot tell a nil value from an absent key. Every relation is still
// available, but only by calling get_relations() with no argument.
//
// The checks are a stricter copy of internal/lua's relationQuery, not a
// shared helper: datamigration may not import the scripting runtime.
func relationQueryArg(ls *lua.LState) (store.RelationQuery, error) {
	var q store.RelationQuery
	arg := ls.Get(1)
	switch arg.Type() {
	case lua.LTNil:
		return q, nil
	case lua.LTTable:
	default:
		return q, fmt.Errorf("argument must be an options table like {from = id}, got %s", arg.Type())
	}
	opts, ok := arg.(*lua.LTable)
	if !ok {
		return q, errors.New("argument must be an options table")
	}
	dst := map[string]*string{"from": &q.From, "type": &q.Type, "to": &q.To}
	var bad error
	n := 0
	opts.ForEach(func(k, v lua.LValue) {
		if bad != nil {
			return
		}
		n++
		key, isStr := k.(lua.LString)
		field, known := dst[string(key)]
		if !isStr || !known {
			bad = fmt.Errorf("unknown option %s (want from, type or to)", k.String())
			return
		}
		val, isStr := v.(lua.LString)
		if !isStr {
			bad = fmt.Errorf("option %q must be a string, got %s", string(key), v.Type())
			return
		}
		if val == "" {
			bad = fmt.Errorf("option %q is empty", string(key))
			return
		}
		*field = string(val)
	})
	if bad != nil {
		return q, bad
	}
	if n == 0 {
		return q, errors.New("empty options table (a nil filter value?); call get_relations() with no argument to read every relation")
	}
	return q, nil
}

// relationToTable mirrors the normal API's relation table, except that
// `properties` is always present.
func relationToTable(ls *lua.LState, rel *entity.Relation) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("from", lua.LString(rel.From))
	t.RawSetString("type", lua.LString(rel.Type))
	t.RawSetString("to", lua.LString(rel.To))
	t.RawSetString("from_face", lua.LString(rel.FromFace.String()))
	t.RawSetString("content", lua.LString(rel.Content))
	props := ls.NewTable()
	for k, v := range rel.Properties {
		props.RawSetString(k, goToLua(ls, v))
	}
	t.RawSetString("properties", props)
	return t
}
