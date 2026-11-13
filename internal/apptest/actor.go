package apptest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	relalua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Argument positions of the actor methods, after the actor (1), the ref or
// type (2) and the properties or relation type (3).
const (
	argOpts      = 4 // create's and update's options, link's and unlink's to
	argLinkProps = 5 // link's relation properties
)

// actorMethod is one actor method. Its Lua arguments start at index 2,
// after the actor itself.
type actorMethod func(ctx context.Context, ls *lua.LState, w *world) int

// actor returns the Lua table that acts as user. Every method binds user's
// identity afresh, so a role change made in the case applies to the next
// call.
func (fr *fileRun) actor(ls *lua.LState, user string) *lua.LTable {
	a := ls.NewTable()
	a.RawSetString("user", lua.LString(user))
	methods := map[string]actorMethod{
		"create":     fr.create,
		"update":     fr.update,
		"transition": fr.transition,
		"delete":     fr.deleteEntity,
		"link":       fr.link,
		"unlink":     fr.unlink,
		"get":        fr.get,
		"list":       fr.list,
	}
	for name, m := range methods {
		a.RawSetString(name, ls.NewFunction(func(ls *lua.LState) int {
			if ls.Get(1) != a {
				ls.RaiseError("call actor methods with a colon: actor:%s(...)", name)
			}
			w := fr.world(ls, name)
			ctx, err := w.as(user)
			fr.check(ls, err)
			return m(ctx, ls, w)
		}))
	}
	return a
}

// address reads a ref argument: an id, an "ID@face" address or an entity
// table. It returns the address the write path takes, and the id and face.
func address(ls *lua.LState, n int) (addr, id string, face entity.Face) {
	switch v := ls.Get(n).(type) {
	case lua.LString:
		addr = string(v)
		var f string
		id, f, _ = strings.Cut(addr, "@")
		return addr, id, entity.Face(f)
	case *lua.LTable:
		id = lua.LVAsString(v.RawGetString("id"))
		face = entity.Face(lua.LVAsString(v.RawGetString("face")))
		if id == "" {
			ls.ArgError(n, "the entity table has no id")
		}
		addr = id
		if face != "" {
			addr += "@" + string(face)
		}
		return addr, id, face
	}
	ls.ArgError(n, "expected an id, an ID@face address or an entity table")
	return "", "", ""
}

// props reads an optional table argument as properties.
func props(ls *lua.LState, n int) map[string]any {
	t := ls.OptTable(n, nil)
	if t == nil {
		return map[string]any{}
	}
	m, ok := relalua.ToGo(t).(map[string]any)
	if !ok {
		if t.Len() == 0 {
			return map[string]any{}
		}
		ls.ArgError(n, "expected a table of properties keyed by name")
	}
	return m
}

// pushWritten returns an entity and its warnings to Lua, or raises the
// automation errors of the write.
func (fr *fileRun) pushWritten(ls *lua.LState, e *entity.Entity, warnings []entity.Warning, autoErrs []string) int {
	if len(autoErrs) > 0 {
		fr.raise(ls, fmt.Errorf("automation failed: %s", strings.Join(autoErrs, "; ")))
	}
	ls.Push(relalua.EntityToTable(ls, e))
	if len(warnings) == 0 {
		// Always a table, so a case can take #warnings.
		ls.Push(ls.NewTable())
	} else {
		ls.Push(relalua.WarningsToTable(ls, warnings))
	}
	return 2
}

// create is actor:create(type, props, opts): a write through the entity
// manager. Required properties props leaves out are generated; a required
// file property gets a generated attachment through the attachment service,
// which writes the property as the actor and so needs update on the type.
// opts takes id and face; a type
// that declares faces needs face, and a type with manual ids gets one
// generated when id is not given.
func (fr *fileRun) create(ctx context.Context, ls *lua.LState, w *world) int {
	typ := ls.CheckString(2)
	p := props(ls, 3)
	meta := w.svc.Meta()
	if _, ok := meta.Entities[typ]; !ok {
		ls.ArgError(2, fmt.Sprintf("the schema declares no entity type %q", typ))
	}
	files, err := w.fake.fill(typ, p)
	fr.check(ls, err)
	opts := optionTable(ls, argOpts, "id", "face")
	id := lua.LVAsString(opts.RawGetString("id"))
	if id == "" {
		id = w.fake.manualID(typ)
	}
	face := entity.Face(lua.LVAsString(opts.RawGetString("face")))
	if faces := metamodel.FaceOrderOf(meta, typ); len(faces) > 0 && face == "" {
		fr.raise(ls, fmt.Errorf("%s declares faces (%s); pass {face = ...} to create", typ, strings.Join(faces, ", ")))
	}
	res, err := w.svc.EntityManager().CreateEntity(ctx,
		&entity.Entity{Type: typ, Properties: p}, entity.CreateOptions{ID: id, Face: face})
	fr.check(ls, err)
	e, warnings := res.Entity, res.Warnings
	if len(files) > 0 {
		e, err = w.attachGenerated(ctx, e, files)
		fr.check(ls, err)
		warnings = withoutUnsetFiles(warnings, files)
	}
	return fr.pushWritten(ls, e, warnings, res.AutomationErrors)
}

// hiddenTargetError marks a not-found the ACL caused: the target exists, but the
// actor may not read it, so the write path answers as if it did not.
// t.expect_denied counts it as a denial.
type hiddenTargetError struct{ error }

func (h hiddenTargetError) Unwrap() error { return h.error }

// checkTargets is [fileRun.check] for a call on existing entities: a
// not-found for an id the store holds is recorded as a [hiddenTargetError].
func (fr *fileRun) checkTargets(ctx context.Context, ls *lua.LState, w *world, err error, ids ...string) {
	if err == nil {
		return
	}
	notFound := errors.Is(err, store.ErrNotFound) || errors.Is(err, entitymanager.ErrEntityNotFound)
	if notFound && w.anyExists(ctx, ids) {
		err = hiddenTargetError{err}
	}
	fr.raise(ls, err)
}

// update is actor:update(ref, props, opts): a patch that changes only the
// named properties. opts.unset lists properties to remove.
func (fr *fileRun) update(ctx context.Context, ls *lua.LState, w *world) int {
	addr, id, _ := address(ls, 2)
	patch := entity.Patch{Properties: props(ls, 3)}
	if u, ok := optionTable(ls, argOpts, "unset").RawGetString("unset").(*lua.LTable); ok {
		u.ForEach(func(_, v lua.LValue) { patch.MetaUnset = append(patch.MetaUnset, lua.LVAsString(v)) })
	}
	res, err := w.svc.EntityManager().PatchEntity(ctx, addr, patch)
	fr.checkTargets(ctx, ls, w, err, id)
	return fr.pushWritten(ls, res.Entity, res.Warnings, res.AutomationErrors)
}

// transition is actor:transition(ref, value, prop): update one property,
// "status" unless prop names another.
func (fr *fileRun) transition(ctx context.Context, ls *lua.LState, w *world) int {
	addr, id, _ := address(ls, 2)
	value := ls.CheckString(3)
	prop := ls.OptString(argOpts, "status")
	res, err := w.svc.EntityManager().PatchEntity(ctx, addr,
		entity.Patch{Properties: map[string]any{prop: value}})
	fr.checkTargets(ctx, ls, w, err, id)
	return fr.pushWritten(ls, res.Entity, res.Warnings, res.AutomationErrors)
}

// deleteEntity is actor:delete(ref, opts); opts.cascade also removes the
// entity's relations.
func (fr *fileRun) deleteEntity(ctx context.Context, ls *lua.LState, w *world) int {
	addr, id, _ := address(ls, 2)
	cascade := lua.LVAsBool(optionTable(ls, 3, "cascade").RawGetString("cascade"))
	_, err := w.svc.EntityManager().DeleteEntity(ctx, addr, cascade)
	fr.checkTargets(ctx, ls, w, err, id)
	return 0
}

// relationKey reads actor:link and actor:unlink's from, type and to.
func relationKey(ls *lua.LState) entity.RelationKey {
	_, from, face := address(ls, 2)
	typ := ls.CheckString(3)
	_, to, _ := address(ls, argOpts)
	return entity.RelationKey{From: from, FromFace: face, Type: typ, To: to}
}

// link is actor:link(from, type, to, props).
func (fr *fileRun) link(ctx context.Context, ls *lua.LState, w *world) int {
	key := relationKey(ls)
	opts := entity.RelationOptions{Properties: props(ls, argLinkProps)}
	_, err := w.svc.EntityManager().CreateRelation(ctx, key, opts)
	fr.checkTargets(ctx, ls, w, err, key.From, key.To)
	return 0
}

// unlink is actor:unlink(from, type, to).
func (fr *fileRun) unlink(ctx context.Context, ls *lua.LState, w *world) int {
	key := relationKey(ls)
	fr.checkTargets(ctx, ls, w, w.svc.EntityManager().DeleteRelation(ctx, key), key.From, key.To)
	return 0
}

// get is actor:get(ref): the entity as the actor reads it, or nil when it
// does not exist or the actor may not read it.
func (fr *fileRun) get(ctx context.Context, ls *lua.LState, w *world) int {
	addr, _, _ := address(ls, 2)
	e, err := w.svc.GatedReads().Reader.Resolve(ctx, addr)
	if errors.Is(err, store.ErrNotFound) {
		ls.Push(lua.LNil)
		return 1
	}
	fr.check(ls, err)
	ls.Push(relalua.EntityToTable(ls, e))
	return 1
}

// list is actor:list(type): every entity of type the actor may read, in the
// default world, ordered by id.
func (fr *fileRun) list(ctx context.Context, ls *lua.LState, w *world) int {
	typ := ls.CheckString(2)
	if _, ok := w.svc.Meta().Entities[typ]; !ok {
		ls.ArgError(2, fmt.Sprintf("the schema declares no entity type %q", typ))
	}
	q := store.EntityQuery{Type: typ, Faces: store.InWorld(w.svc.LuaReadDeps().World)}
	var got []*entity.Entity
	for e, err := range w.svc.GatedReads().Reader.ListEntities(ctx, q) {
		fr.check(ls, err)
		got = append(got, e)
	}
	slices.SortFunc(got, func(a, b *entity.Entity) int { return strings.Compare(a.ID, b.ID) })
	out := ls.NewTable()
	for _, e := range got {
		out.Append(relalua.EntityToTable(ls, e))
	}
	ls.Push(out)
	return 1
}

// withoutUnsetFiles drops the "required property unset" warnings of the
// file properties create attached a file to after the write.
func withoutUnsetFiles(warnings []entity.Warning, files []string) []entity.Warning {
	return slices.DeleteFunc(slices.Clone(warnings), func(wn entity.Warning) bool {
		return wn.Code == "required_property_unset" &&
			slices.ContainsFunc(files, func(f string) bool { return wn.Path == "/properties/"+f })
	})
}
