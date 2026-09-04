package docs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	rlua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SeedOp is one recorded seed mutation (a create or a link). The manual's
// create()/link() islands record these; they are applied to the in-memory graph
// immediately (for the Tier-A resolvers entity{}/count{}/graph{}) and REPLAYED
// verbatim against a fresh fsstore-backed temp project when a screenshot{} island
// needs the SPA to render them. One recorder → both stores → they cannot diverge
// (DR-S2).
type SeedOp struct {
	// Kind is "create", "face", "link" or "edit".
	Kind string
	// create / face fields:
	Type       string
	ID         string
	Properties map[string]any
	Content    string
	// Face is the coordinate a "face" op writes. Only ever set by the face{}
	// verb: a "create" writes the DEFAULT face, which is the zero coordinate.
	Face entity.Face
	// link fields:
	From    string
	RelType string
	To      string
}

// seedWriter is the narrow slice of the store the seed bindings need: raw
// create-only writes, no reads and no entitymanager. Declared here at the call
// site rather than taking a whole store.Store, so the seed cluster cannot grow
// into update/delete without widening this interface first.
type seedWriter interface {
	CreateEntity(ctx context.Context, e *entity.Entity) error
	CreateRelation(ctx context.Context, from, relType, to string, data *store.RelationData) (*entity.Relation, error)
	// GetEntity reads back what seeding wrote. face() and edit() need it —
	// face() to resolve the type of an id it is given, edit() to confirm the
	// entity exists and to return the edited result.
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)

	// UpdateEntity serves edit() against the IN-MEMORY resolver store only,
	// where there is no entitymanager to patch through and only the resulting
	// state matters. The real write path — the one that produces a version row
	// a History figure can photograph — goes through SeedPatcher during replay
	// (see ApplySeedWith). Still no delete: seeding builds fixtures, it does not
	// tear them down.
	UpdateEntity(ctx context.Context, e *entity.Entity) error
}

// seedBindings owns the doc.create()/doc.link() write side: the raw-store
// fixture construction the manual's islands perform, plus the two pieces of
// state that exist only to serve it. It is deliberately separate from
// docRuntime, which is read-side graph interrogation and assertion — nothing
// outside seeding should be able to reach the counter or the op log.
//
// docRuntime keeps ownership of registration (a single doc.* binding table), so
// this type exposes bindings but never installs them.
type seedBindings struct {
	store seedWriter
	// meta validates that a face() coordinate is DECLARED for the type, and
	// maps a declared name to its stored coordinate (the bare face stores as
	// the zero value, so seeding it by name must land on the entity's own row).
	meta *metamodel.Metamodel
	// ctx is stored for the same reason docRuntime stores one: these are
	// gopher-lua callbacks, which cannot take a context parameter.
	ctx context.Context //nolint:containedctx // request-scoped Lua-binding callbacks

	// fail raises a typed resolve BuildError on the owning runtime and unwinds
	// the island. Injected as a callback rather than a back-pointer to
	// docRuntime, which would recreate the coupling this split removes.
	fail func(ls *lua.LState, format string, args ...any) int

	// counts is a per-type auto-id counter for mintID, avoiding a full-store
	// scan per create().
	counts map[string]int

	// ops records every create/link so a screenshot{} or api{} island can replay
	// them against a fresh fsstore temp project (DR-S2). Read by those islands
	// via docRuntime.seed.ops.
	ops []SeedOp
}

// luaCreate seeds an entity directly into the memstore (raw store — no
// validation, no state-machine gate, no ACL). create("risico", {props}, body?)
// returns the created entity as a table (so the author can `link` it).
func (s *seedBindings) luaCreate(ls *lua.LState) int {
	typ := ls.CheckString(1)
	props := map[string]any{}
	if tbl, ok := ls.Get(2).(*lua.LTable); ok {
		props = luaTableToMap(tbl)
	}
	content := ls.OptString(3, "")

	id := s.mintID(typ, props)
	e := &entity.Entity{ID: id, Type: typ, Properties: props, Content: content}
	if err := s.store.CreateEntity(s.ctx, e); err != nil {
		return s.fail(ls, "create(%q): %v", typ, err)
	}
	// Record for replay into the screenshot temp project (DR-S2).
	s.ops = append(s.ops, SeedOp{
		Kind: "create", Type: typ, ID: id, Properties: props, Content: content,
	})
	ls.Push(rlua.EntityToTable(ls, e))
	return 1
}

// luaLink seeds a relation into the memstore. link(from, type, to) where from
// may be an id string or an entity table (as returned by create).
func (s *seedBindings) luaLink(ls *lua.LState) int {
	from := idArg(ls, 1)
	relType := ls.CheckString(2)
	to := idArg(ls, 3)

	// An optional 4th argument names the SOURCE FACE the edge hangs off:
	// link("POL-1", "implements", "CTL-1", "published").
	//
	// A content-scoped relation belongs to one face, so a manual explaining
	// what a world does with edges needs to seed one that way — and reading it
	// back is face-exact, with no fallback, so an edge seeded on the wrong face
	// is invisible rather than borrowed. Omitted means the bare-id tail, which
	// is what an identity-scoped edge has and what every pre-faces project has.
	var tail entity.Face
	// link(from, type, to, fromFace?) — the optional face trails the triple.
	const linkFaceArg = 4
	if fromFace := ls.OptString(linkFaceArg, ""); fromFace != "" {
		typ := s.typeOf(from)
		def, ok := s.meta.GetEntityDef(typ)
		if !ok {
			return s.fail(ls, "link: cannot resolve the type of %q", from)
		}
		if _, declared := def.Faces[fromFace]; !declared {
			return s.fail(ls, "link(%q,%q,%q,%q): %q is not a declared face of %q",
				from, relType, to, fromFace, fromFace, typ)
		}
		tail = entity.Face(metamodel.StoredFace(s.meta, typ, fromFace))
	}

	if _, err := s.store.CreateRelation(s.ctx, from, relType, to,
		&store.RelationData{FromFace: tail}); err != nil {
		return s.fail(ls, "link(%q,%q,%q): %v", from, relType, to, err)
	}
	s.ops = append(s.ops, SeedOp{
		Kind: "link", From: from, RelType: relType, To: to, Face: tail,
	})
	return 0
}

// mintID derives a stable id for a seeded entity: an explicit props.id if given,
// else "<type>-<n>" from a per-type counter. Fixture ids need only be unique
// within the build; the counter avoids an O(n²) full-store scan per create.
func (s *seedBindings) mintID(typ string, props map[string]any) string {
	if v, ok := props["id"].(string); ok && v != "" {
		return v
	}
	if s.counts == nil {
		s.counts = map[string]int{}
	}
	s.counts[typ]++
	return fmt.Sprintf("%s-%d", typ, s.counts[typ])
}

// SeedPatcher applies one targeted property change through the real write
// path. It is the consumer-side narrowing of entitymanager.EntityManager that
// the `edit` seed op needs — declared here, at the call site, so internal/docs
// never depends on the entitymanager (which it must not: arch-lint forbids the
// edge, and the doc runtime's own store is a throwaway memstore with no write
// machinery behind it).
//
// The wiring site supplies it: docscapture passes its temp project's manager,
// and the in-memory phase-2 replay passes nil.
//
// Nil: accepted — ApplySeed then applies an `edit` as a raw store write. That
// is the right degradation for the in-memory resolver store, which has no
// entitymanager at all and only needs the RESULTING STATE to be correct. What
// it loses is the version capture, which only a real backend records anyway.
type SeedPatcher interface {
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
}

// ApplySeed replays recorded seed ops against a store, using RAW store writes
// (no entitymanager) so automations can't mutate the fixture. Used for both the
// in-memory store (phase-2 resolvers) and the screenshot temp project's store
// (Tier B), so the two representations cannot diverge (DR-S2).
//
// The one exception is the `edit` op, which is routed through patcher when one
// is supplied — see ApplySeedWith. This preserves the fixture rule for
// everything that CREATES the fixture, while letting a manual demonstrate a
// real edit.
func ApplySeed(ctx context.Context, st store.Store, ops []SeedOp) error {
	return ApplySeedWith(ctx, st, nil, ops)
}

// ApplySeedWith is ApplySeed with a write path for the `edit` op.
//
// # Why `edit` is not a raw store write like the others
//
// create/face/link build the fixture, and writing them raw is deliberate: an
// automation firing on a seeded entity would mutate what the manual says it
// created. An `edit` is different in kind — it does not establish the fixture,
// it DEMONSTRATES a change to it, and a manual's claim about what an edit does
// is only worth as much as the path it took. Routed through the entitymanager,
// an edit is a genuine write: it is authorized, audited, attributed to a
// principal, and — the reason this exists — CAPTURED AS A VERSION on a backend
// that versions.
//
// A raw store write produces no version row at all, so a History section built
// on seeded edits would photograph an empty timeline while claiming to show a
// history. That is the failure this closes.
func ApplySeedWith(ctx context.Context, st store.Store, patcher SeedPatcher, ops []SeedOp) error {
	for _, op := range ops {
		switch op.Kind {
		case "create":
			e := &entity.Entity{ID: op.ID, Type: op.Type, Properties: op.Properties, Content: op.Content}
			if err := st.CreateEntity(ctx, e); err != nil {
				return err
			}
		case "face":
			// A non-default face of an existing entity. Written through the
			// same raw store path as a create, because a doc fixture is
			// exactly what the author wrote — see the seeding rules above.
			e := &entity.Entity{
				ID: op.ID, Type: op.Type, Face: op.Face,
				Properties: op.Properties, Content: op.Content,
			}
			if err := st.CreateEntity(ctx, e); err != nil {
				return err
			}
		case "edit":
			if err := applyEdit(ctx, st, patcher, op); err != nil {
				return err
			}
		case "link":
			// Face carries the edge's source tail, so a content-scoped edge
			// replays onto the same face it was seeded on.
			if _, err := st.CreateRelation(ctx, op.From, op.RelType, op.To,
				&store.RelationData{FromFace: op.Face}); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedEditStore is the slice of a store applyEdit needs when no patcher is
// supplied: read the entity back, write it whole. Narrowed at the call site so
// the seed bindings can pass their own seedWriter-shaped handle rather than a
// full store.Store.
type seedEditStore interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	UpdateEntity(ctx context.Context, e *entity.Entity) error
}

// applyEdit applies one `edit` op: through the entitymanager when the caller
// supplied one, else as a raw store write.
//
// The raw fallback is not a silent degradation — it is what the in-memory
// resolver store needs, having no write machinery at all (see [SeedPatcher]).
// It is deliberately a read-modify-write, which the codebase forbids on real
// write paths: here the "read" is of a fixture this same build just seeded and
// the store is a throwaway, so there are no hidden properties to clobber. The
// patcher path, which is the one a real backend takes, uses PatchEntity
// precisely so that rule holds where it matters.
func applyEdit(ctx context.Context, st seedEditStore, patcher SeedPatcher, op SeedOp) error {
	patch := entity.Patch{Properties: op.Properties}
	if op.Content != "" {
		content := op.Content
		patch.Content = &content
	}
	if patcher != nil {
		_, err := patcher.PatchEntity(ctx, op.ID, patch)
		return err
	}

	e, err := st.GetEntity(ctx, op.ID)
	if err != nil {
		return err
	}
	patch.Apply(e)
	return st.UpdateEntity(ctx, e)
}

// luaFace seeds a NON-DEFAULT face of an entity that already exists:
// face("policy", "POL-1", "published", {props}, body?).
//
// # Why this is a separate verb from create()
//
// `create` writes the entity's DEFAULT face — the one a bare id addresses — and
// that is the only face a real create can write: a create names no face, and a
// second face arrives later by a copy rather than by creation. A manual
// demonstrating worlds needs an entity that HAS a second face, which in a real
// project arrives by a copy/publish action rather than by creation. Seeding it
// directly keeps the fixture honest about what it is: a fixture, not a
// re-enactment of the write path.
//
// The coordinate must be DECLARED for the type. An undeclared one would sit in
// the store answering to no world, so every world assertion about it would pass
// for the wrong reason.
func (s *seedBindings) luaFace(ls *lua.LState) int {
	typ := ls.CheckString(1)
	id := ls.CheckString(2)
	coord := ls.CheckString(3)

	def, ok := s.meta.GetEntityDef(typ)
	if !ok {
		return s.fail(ls, "face(%q): no such entity type", typ)
	}
	if _, declared := def.Faces[coord]; !declared {
		return s.fail(ls, "face(%q, %q, %q): %q is not a declared face of %q "+
			"(schema.yaml declares: %s)", typ, id, coord, coord, typ,
			strings.Join(sortedFaceNames(def), ", "))
	}

	// face(type, id, coord, props?, body?) — the two optional arguments trail
	// the three required ones.
	const (
		facePropsArg   = 4
		faceContentArg = 5
	)
	props := map[string]any{}
	if tbl, ok := ls.Get(facePropsArg).(*lua.LTable); ok {
		props = luaTableToMap(tbl)
	}
	content := ls.OptString(faceContentArg, "")

	// The STORED coordinate, not the declared name: a `bare_face` face is
	// stored under the zero coordinate, so seeding it by name must land on the
	// entity's own row rather than minting a second one.
	stored := entity.Face(metamodel.StoredFace(s.meta, typ, coord))
	e := &entity.Entity{ID: id, Type: typ, Face: stored, Properties: props, Content: content}
	if err := s.store.CreateEntity(s.ctx, e); err != nil {
		return s.fail(ls, "face(%q, %q, %q): %v", typ, id, coord, err)
	}
	s.ops = append(s.ops, SeedOp{
		Kind: "face", Type: typ, ID: id, Face: stored,
		Properties: props, Content: content,
	})
	ls.Push(rlua.EntityToTable(ls, e))
	return 1
}

// luaEdit records an EDIT of an already-seeded entity:
// edit("POL-1", {status="doing"}, body?).
//
// # Why this is a verb and not just a second create()
//
// create/face/link build a fixture; re-creating an entity with different
// properties would produce the same final STATE but no record that it changed.
// A manual explaining version history needs the change itself to be real,
// because the history page is a picture of changes, not of states. So an edit
// replays through the entitymanager (see [ApplySeedWith]) — authorized,
// attributed, and captured as a version by a backend that versions.
//
// # It refuses to assert nothing
//
// An edit naming no properties and no body would be a write that changes
// nothing: it would still append a version on some backends and none on
// others, so the manual's timeline would differ by backend for a call that
// says nothing. Refusing follows the same house rule api{} states — a call
// with no claim passes whatever the system does.
func (s *seedBindings) luaEdit(ls *lua.LState) int {
	id := ls.CheckString(1)

	// edit(id, props?, body?) — both optional individually, but not together.
	const (
		editPropsArg   = 2
		editContentArg = 3
	)
	props := map[string]any{}
	if tbl, ok := ls.Get(editPropsArg).(*lua.LTable); ok {
		props = luaTableToMap(tbl)
	}
	content := ls.OptString(editContentArg, "")
	if len(props) == 0 && content == "" {
		return s.fail(ls, "edit(%q): changes nothing. Give properties or a body — "+
			"an edit with no change still writes, so the manual would claim a change "+
			"the reader cannot see", id)
	}

	e, err := s.store.GetEntity(s.ctx, id)
	if err != nil || e == nil {
		return s.fail(ls, "edit(%q): no such seeded entity (create it first)", id)
	}

	// Apply to the in-memory resolver store immediately, so a later entity{} /
	// count{} island sees the edited value — the same one-recorder-both-stores
	// property create() has (DR-S2).
	op := SeedOp{Kind: "edit", Type: e.Type, ID: id, Properties: props, Content: content}
	if aerr := applyEdit(s.ctx, s.store, nil, op); aerr != nil {
		return s.fail(ls, "edit(%q): %v", id, aerr)
	}
	s.ops = append(s.ops, op)

	edited, gerr := s.store.GetEntity(s.ctx, id)
	if gerr != nil {
		return s.fail(ls, "edit(%q): %v", id, gerr)
	}
	ls.Push(rlua.EntityToTable(ls, edited))
	return 1
}

// typeOf reports the seeded entity's type, for validating a face name against
// the right declaration.
func (s *seedBindings) typeOf(id string) string {
	e, err := s.store.GetEntity(s.ctx, id)
	if err != nil || e == nil {
		return ""
	}
	return e.Type
}

// sortedFaceNames lists a type's declared face names for a failure message.
func sortedFaceNames(def *metamodel.EntityDef) []string {
	out := make([]string, 0, len(def.Faces))
	for name := range def.Faces {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
