package docs

import (
	"context"
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	rlua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SeedOp is one recorded seed mutation (a create or a link). The manual's
// create()/link() islands record these; they are applied to the in-memory graph
// immediately (for the Tier-A resolvers entity{}/count{}/graph{}) and REPLAYED
// verbatim against a fresh fsstore-backed temp project when a screenshot{} island
// needs the SPA to render them. One recorder → both stores → they cannot diverge
// (DR-S2).
type SeedOp struct {
	// Kind is "create" or "link".
	Kind string
	// create fields:
	Type       string
	ID         string
	Properties map[string]any
	Content    string
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
	if _, err := s.store.CreateRelation(s.ctx, from, relType, to, nil); err != nil {
		return s.fail(ls, "link(%q,%q,%q): %v", from, relType, to, err)
	}
	s.ops = append(s.ops, SeedOp{Kind: "link", From: from, RelType: relType, To: to})
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

// ApplySeed replays recorded seed ops against a store, using RAW store writes
// (no entitymanager) so automations can't mutate the fixture. Used for both the
// in-memory store (phase-2 resolvers) and the screenshot temp project's store
// (Tier B), so the two representations cannot diverge (DR-S2).
func ApplySeed(ctx context.Context, st store.Store, ops []SeedOp) error {
	for _, op := range ops {
		switch op.Kind {
		case "create":
			e := &entity.Entity{ID: op.ID, Type: op.Type, Properties: op.Properties, Content: op.Content}
			if err := st.CreateEntity(ctx, e); err != nil {
				return err
			}
		case "link":
			if _, err := st.CreateRelation(ctx, op.From, op.RelType, op.To, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
