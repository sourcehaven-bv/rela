// Lua bindings for piles: the rela.piles table (TKT-K3RJLH).
package lua

import (
	"context"
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// maxPileEntities bounds the entities one rela.piles.add or remove call may
// name. It matches the size of a full pile, so a larger list could never be
// held anyway.
const maxPileEntities = 500

// errPilesUnavailable is raised by every rela.piles function on a runtime
// that was built without the piles capability.
var errPilesUnavailable = errors.New("piles are not available in this context")

// PileSummary is one of the acting user's piles as the bindings see it.
// Items are newest first and include refs the caller can no longer read;
// the bindings filter them through [ReadDeps.VisibleReader].
type PileSummary struct {
	ID    string
	Name  string
	Icon  string
	Items []entity.Ref
}

// PilePush is one rela.piles.add call: refs already resolved through the
// script's own read gate, pushed onto the pile named Pile.
type PilePush struct {
	// Owner is the target user. Empty means the acting user.
	Owner string
	// Pile is the pile name.
	Pile string
	// Refs are the resolved write targets, in the order the script named
	// them.
	Refs []entity.Ref
	// Create makes the pile when it does not exist.
	Create bool
}

// PileReader is the read half of the piles capability. Both methods act on
// the acting user's piles only (the principal on ctx); nothing here reads
// another user's pile.
type PileReader interface {
	// ListPiles returns the acting user's piles.
	ListPiles(ctx context.Context) ([]PileSummary, error)
	// FindPile returns the acting user's pile with that name.
	FindPile(ctx context.Context, name string) (PileSummary, error)
}

// PileWriter is the write half of the piles capability.
type PileWriter interface {
	// PushToPile adds refs to a pile by name, possibly another user's, and
	// returns how many were new.
	PushToPile(ctx context.Context, p PilePush) (int, error)
	// RemoveFromPile drops refs from one of the acting user's piles, by id.
	RemoveFromPile(ctx context.Context, pileID string, refs []entity.Ref) error
}

// PileFuncs adapts plain functions to [PileReader] and [PileWriter], so the
// wiring site can supply the piles service as closures without this package
// importing it. Build one with [NewPileFuncs].
type PileFuncs struct {
	List   func(ctx context.Context) ([]PileSummary, error)
	Find   func(ctx context.Context, name string) (PileSummary, error)
	Push   func(ctx context.Context, p PilePush) (int, error)
	Remove func(ctx context.Context, pileID string, refs []entity.Ref) error
}

// NewPileFuncs returns f after checking it is complete.
//
// Nil: every function is required and a nil one is rejected.
func NewPileFuncs(f PileFuncs) (*PileFuncs, error) {
	if f.List == nil || f.Find == nil || f.Push == nil || f.Remove == nil {
		return nil, errors.New("lua: NewPileFuncs requires List, Find, Push and Remove")
	}
	return &f, nil
}

// ListPiles serves [PileReader.ListPiles] through the injected List function.
func (f *PileFuncs) ListPiles(ctx context.Context) ([]PileSummary, error) { return f.List(ctx) }

// FindPile serves [PileReader.FindPile] through the injected Find function.
func (f *PileFuncs) FindPile(ctx context.Context, name string) (PileSummary, error) {
	return f.Find(ctx, name)
}

// PushToPile serves [PileWriter.PushToPile] through the injected Push function.
func (f *PileFuncs) PushToPile(ctx context.Context, p PilePush) (int, error) { return f.Push(ctx, p) }

// RemoveFromPile serves [PileWriter.RemoveFromPile] through the injected Remove function.
func (f *PileFuncs) RemoveFromPile(ctx context.Context, pileID string, refs []entity.Ref) error {
	return f.Remove(ctx, pileID, refs)
}

// headerResolver is the batched, header-only read the pile bindings use.
// A failed header read is returned, so it is never mistaken for an empty
// pile. [visibility.ScriptReader], [visibility.UnrestrictedReader] and
// [visibility.DenyReader] provide it.
type headerResolver interface {
	ResolveHeadersErr(ctx context.Context, refs []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error)
}

// pileBindings implements rela.piles. A type of its own, like
// elevationBindings, to keep [Runtime] under its method load line.
type pileBindings struct {
	reader PileReader   // nil: every function raises errPilesUnavailable
	writer PileWriter   // nil: add and remove raise errPilesUnavailable
	rd     EntityReader // ReadDeps.VisibleReader; nil raises like reader()
	meta   *metamodel.Metamodel
	ctxFn  func() context.Context
}

// register installs rela.piles on rela. list and items are present on every
// runtime; add and remove only when writes is set, like the other write
// bindings. The table is present even without a piles capability, so a
// script gets a message naming the problem instead of "attempt to index a nil
// value".
func (b *pileBindings) register(ls *lua.LState, rela *lua.LTable, writes bool) {
	t := ls.NewTable()
	ls.SetField(t, "list", ls.NewFunction(b.luaList))
	ls.SetField(t, "items", ls.NewFunction(b.luaItems))
	if writes {
		ls.SetField(t, "add", ls.NewFunction(b.luaAdd))
		ls.SetField(t, "remove", ls.NewFunction(b.luaRemove))
	}
	ls.SetField(rela, "piles", t)
}

// luaAdd implements rela.piles.add{pile=, entities=, owner=?, create=?} ->
// number of entities that were new on the pile.
//
// Each entity resolves as every write does ([resolveWriteTarget]): it must be
// readable by the script, and a bare id of a faced type raises the ambiguity
// error. One batched header read settles the entities it can (see
// [pileWriteRefs]). owner pushes to another user's pile; it
// must name the acting user or an existing person entity; such a push is
// write-only, so it returns 0 and a missing pile or full owner is a silent
// no-op. create defaults to true.
func (b *pileBindings) luaAdd(ls *lua.LState) int {
	opts := ls.CheckTable(1)
	if b.writer == nil {
		ls.RaiseError("rela.piles.add: %s", errPilesUnavailable)
		return 0
	}
	name, ok := pileNameArg(ls, opts, "rela.piles.add")
	if !ok {
		return 0
	}
	owner, ok := optStringField(ls, opts, "owner", "rela.piles.add")
	if !ok {
		return 0
	}
	create := true
	switch v := opts.RawGetString("create").(type) {
	case *lua.LNilType:
	case lua.LBool:
		create = bool(v)
	default:
		ls.RaiseError("rela.piles.add: create must be a boolean, got %s", v.Type())
		return 0
	}
	addrs, ok := pileAddresses(ls, opts, "rela.piles.add")
	if !ok {
		return 0
	}
	ctx := b.ctxFn()
	refs, ok := b.pileWriteRefs(ctx, ls, addrs)
	if !ok {
		return 0
	}
	added, err := b.writer.PushToPile(ctx, PilePush{Owner: owner, Pile: name, Refs: refs, Create: create})
	if err != nil {
		ls.RaiseError("rela.piles.add: %s", err.Error())
		return 0
	}
	ls.Push(lua.LNumber(added))
	return 1
}

// pileWriteRefs resolves the entities of an add to the refs the pile stores,
// in the order the script named them, as the HTTP add does. One batched
// header read settles every address it can: a named face the script may
// read, and a bare id of a faceless type. Any other address of a readable
// family goes to [resolveWriteTarget], which raises the ambiguity error for
// a bare id of a faced type. An address with no readable family raises
// "entity not found" without a further read. A failed header read raises.
func (b *pileBindings) pileWriteRefs(ctx context.Context, ls *lua.LState, addrs []string) ([]entity.Ref, bool) {
	const binding = "rela.piles.add"
	hr, ok := b.headers(ls, binding)
	if !ok {
		return nil, false
	}
	refs := make([]entity.Ref, len(addrs))
	for i, addr := range addrs {
		ref, err := entity.ParseRef(addr)
		if err != nil || ref.IsZero() {
			ls.RaiseError("entity not found: %s", addr)
			return nil, false
		}
		refs[i] = ref
	}
	resolved, err := hr.ResolveHeadersErr(ctx, refs)
	if err != nil {
		ls.RaiseError("%s: %s", binding, err.Error())
		return nil, false
	}
	for i, ref := range refs {
		res, found := resolved[ref]
		switch {
		case found && res.Served() && (!ref.Face.IsImplicit() || res.Header.Face.IsImplicit()):
			// A named face as named, or a faceless type's bare id.
		case found && res.Family:
			if refs[i], ok = resolveWriteTarget(ctx, ls, b.rd, addrs[i]); !ok {
				return nil, false
			}
		default:
			ls.RaiseError("entity not found: %s", addrs[i])
			return nil, false
		}
	}
	return refs, true
}

// luaRemove implements rela.piles.remove{pile=, entities=} -> true, on the
// acting user's own pile.
//
// It reads no graph: `ID@face` removes that face and a bare id removes every
// face of the id that is on the pile. A ref that is not on the pile is
// ignored, so removing a hidden, deleted or never-added entity looks the
// same.
func (b *pileBindings) luaRemove(ls *lua.LState) int {
	opts := ls.CheckTable(1)
	if b.writer == nil || b.reader == nil {
		ls.RaiseError("rela.piles.remove: %s", errPilesUnavailable)
		return 0
	}
	name, ok := pileNameArg(ls, opts, "rela.piles.remove")
	if !ok {
		return 0
	}
	addrs, ok := pileAddresses(ls, opts, "rela.piles.remove")
	if !ok {
		return 0
	}
	ctx := b.ctxFn()
	p, err := b.reader.FindPile(ctx, name)
	if err != nil {
		ls.RaiseError("rela.piles.remove: %s", err.Error())
		return 0
	}
	refs, err := refsOnPile(p.Items, addrs)
	if err != nil {
		ls.RaiseError("rela.piles.remove: %s", err.Error())
		return 0
	}
	if len(refs) > 0 {
		if err := b.writer.RemoveFromPile(ctx, p.ID, refs); err != nil {
			ls.RaiseError("rela.piles.remove: %s", err.Error())
			return 0
		}
	}
	ls.Push(lua.LTrue)
	return 1
}

// refsOnPile maps addresses onto the pile's items: `ID@face` names that
// item, a bare id every item with that id.
func refsOnPile(items []entity.Ref, addrs []string) ([]entity.Ref, error) {
	var out []entity.Ref
	seen := make(map[entity.Ref]bool, len(addrs))
	for _, addr := range addrs {
		ref, err := entity.ParseRef(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid entity address %q: %w", addr, err)
		}
		for _, it := range items {
			match := it == ref || (ref.Face.IsImplicit() && it.ID == ref.ID)
			if match && !seen[it] {
				seen[it] = true
				out = append(out, it)
			}
		}
	}
	return out, nil
}

// luaList implements rela.piles.list() -> array of {id, name, icon, count}
// for the acting user's piles. count is the number of items the script can
// read, answered for every pile by one batched read.
func (b *pileBindings) luaList(ls *lua.LState) int {
	if b.reader == nil {
		ls.RaiseError("rela.piles.list: %s", errPilesUnavailable)
		return 0
	}
	hr, ok := b.headers(ls, "rela.piles.list")
	if !ok {
		return 0
	}
	ctx := b.ctxFn()
	ps, err := b.reader.ListPiles(ctx)
	if err != nil {
		ls.RaiseError("rela.piles.list: %s", err.Error())
		return 0
	}
	var all []entity.Ref
	for _, p := range ps {
		all = append(all, p.Items...)
	}
	resolved, err := hr.ResolveHeadersErr(ctx, all)
	if err != nil {
		ls.RaiseError("rela.piles.list: %s", err.Error())
		return 0
	}
	out := ls.NewTable()
	for _, p := range ps {
		count := 0
		for _, ref := range p.Items {
			if resolved[ref].Served() {
				count++
			}
		}
		t := ls.NewTable()
		t.RawSetString("id", lua.LString(p.ID))
		t.RawSetString("name", lua.LString(p.Name))
		t.RawSetString("icon", lua.LString(p.Icon))
		t.RawSetString("count", lua.LNumber(count))
		out.Append(t)
	}
	ls.Push(out)
	return 1
}

// luaItems implements rela.piles.items(name) -> array of
// {id, face, address, type, title}, newest first, for the items of the acting
// user's pile that the script can read. One batched header read answers
// every item.
func (b *pileBindings) luaItems(ls *lua.LState) int {
	name := ls.CheckString(1)
	if b.reader == nil {
		ls.RaiseError("rela.piles.items: %s", errPilesUnavailable)
		return 0
	}
	hr, ok := b.headers(ls, "rela.piles.items")
	if !ok {
		return 0
	}
	ctx := b.ctxFn()
	p, err := b.reader.FindPile(ctx, name)
	if err != nil {
		ls.RaiseError("rela.piles.items: %s", err.Error())
		return 0
	}
	resolved, err := hr.ResolveHeadersErr(ctx, p.Items)
	if err != nil {
		ls.RaiseError("rela.piles.items: %s", err.Error())
		return 0
	}
	out := ls.NewTable()
	for _, ref := range p.Items {
		h, ok := resolved[ref]
		if !ok || !h.Served() {
			continue
		}
		t := ls.NewTable()
		t.RawSetString("id", lua.LString(h.Header.ID))
		t.RawSetString("face", lua.LString(h.Header.Face.String()))
		t.RawSetString("address", lua.LString(ref.String()))
		t.RawSetString("type", lua.LString(h.Header.Type))
		t.RawSetString("title", lua.LString(b.title(h)))
		out.Append(t)
	}
	ls.Push(out)
	return 1
}

// title is the item's display title, or its id when the metamodel derives
// none.
func (b *pileBindings) title(h visibility.ResolvedHeader) string {
	if b.meta == nil {
		return h.Header.ID
	}
	return b.meta.DisplayTitle(h.Header.ID, h.Header.Type, h.Header.Properties)
}

// entityReader returns the script's read gate, raising when it is absent
// (as the Runtime does for the other read bindings).
func (b *pileBindings) entityReader(ls *lua.LState, binding string) (EntityReader, bool) {
	if b.rd == nil {
		ls.RaiseError("%s: no reader is configured for this runtime", binding)
		return nil, false
	}
	return b.rd, true
}

// headers returns the script's read gate as a batched header resolver. Every
// gated reader provides it, so its absence is a wiring bug and raises.
func (b *pileBindings) headers(ls *lua.LState, binding string) (headerResolver, bool) {
	rd, ok := b.entityReader(ls, binding)
	if !ok {
		return nil, false
	}
	hr, ok := rd.(headerResolver)
	if !ok {
		ls.RaiseError("%s: the reader cannot resolve items", binding)
		return nil, false
	}
	return hr, true
}

// pileNameArg reads the required `pile` field.
func pileNameArg(ls *lua.LState, opts *lua.LTable, binding string) (string, bool) {
	v, ok := opts.RawGetString("pile").(lua.LString)
	if !ok || v == "" {
		ls.RaiseError("%s: pile (a pile name) is required", binding)
		return "", false
	}
	return string(v), true
}

// optStringField reads an optional string field; any other type raises.
func optStringField(ls *lua.LState, opts *lua.LTable, field, binding string) (string, bool) {
	switch v := opts.RawGetString(field).(type) {
	case *lua.LNilType:
		return "", true
	case lua.LString:
		return string(v), true
	default:
		ls.RaiseError("%s: %s must be a string, got %s", binding, field, v.Type())
		return "", false
	}
}

// pileAddresses reads the required `entities` array. Each element is an
// address string (`ID` or `ID@face`) or an entity table, whose id and face
// form the address.
func pileAddresses(ls *lua.LState, opts *lua.LTable, binding string) ([]string, bool) {
	list, ok := opts.RawGetString("entities").(*lua.LTable)
	if !ok {
		ls.RaiseError("%s: entities (an array of ids or entities) is required", binding)
		return nil, false
	}
	n := list.Len()
	if n > maxPileEntities {
		ls.RaiseError("%s: at most %d entities per call, got %d", binding, maxPileEntities, n)
		return nil, false
	}
	addrs := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		addr, err := pileAddress(list.RawGetInt(i))
		if err != nil {
			ls.RaiseError("%s: entities[%d]: %s", binding, i, err.Error())
			return nil, false
		}
		addrs = append(addrs, addr)
	}
	return addrs, true
}

// pileAddress turns one element of `entities` into an address.
func pileAddress(v lua.LValue) (string, error) {
	switch v := v.(type) {
	case lua.LString:
		if v == "" {
			return "", errors.New("empty id")
		}
		return string(v), nil
	case *lua.LTable:
		id, ok := v.RawGetString("id").(lua.LString)
		if !ok || id == "" {
			return "", errors.New("entity table without an id")
		}
		face, _ := v.RawGetString("face").(lua.LString)
		return entity.FormatStateRef(string(id), entity.Face(face)), nil
	default:
		return "", fmt.Errorf("expected an id or an entity table, got %s", v.Type())
	}
}
