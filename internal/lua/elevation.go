// Lua bindings for ACL elevation: rela.bypass_acl and the scoped `admin`
// handle it passes to its closure (TKT-D8T148, TKT-ACSBSA, TKT-Y3JVFK).
package lua

import (
	"context"
	"errors"
	"slices"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// elevationBindings implements rela.bypass_acl. A type of its own rather than
// more methods on [Runtime] (the urlHelpers rationale in urls.go): the binding
// needs exactly the three elevated deps plus the caller-context closure, so it
// holds those and nothing else. Registration stays on Runtime —
// registerBindings builds one only when an elevated handle was wired, so the
// structural-absence guarantee documented there is unchanged.
type elevationBindings struct {
	em       Mutator                // nil: elevated writes withheld — admin write methods absent
	er       EntityReader           // nil: admin read methods present but raising
	recorder ElevationRecorder      // nil: no post-closure read audit record
	ctxFn    func() context.Context // the runtime's callerCtx
}

// luaBypassACL implements rela.bypass_acl(fn) (TKT-D8T148). It invokes fn with
// a single argument `admin`: a handle whose reads and writes skip the ACL,
// backed by the elevated Mutator and/or EntityReader wired into WriteDeps.
// Elevation is therefore an OBJECT CAPABILITY scoped to the closure — the gated
// rela.* bindings are never elevated; only access through `admin` bypasses ACL.
//
// The handle carries only the capabilities that were wired (TKT-Y3JVFK): a
// document render gets an elevated reader and no Mutator, so `admin` has the
// three read methods and no write methods at all. See newElevatedHandle.
//
// After fn returns (or raises), `admin` is INVALIDATED: its methods raise. A
// script that squirrels `admin` into a global and calls it later gets a dead
// handle, so the lexical scope is enforced, not merely conventional (mirrors
// the frozen rela.principal). fn's return value(s) propagate to the caller; a
// raise inside fn propagates too (a failed elevated write must surface).
func (b *elevationBindings) luaBypassACL(ls *lua.LState) int {
	fn := ls.CheckFunction(1)
	if b.em == nil && b.er == nil {
		// Defensive: the binding is only registered when at least one elevated
		// handle is set, but fail loud rather than silently no-op if that ever
		// drifts. Note this raises only when BOTH are absent — a reader-only
		// elevation is legitimate (TKT-Y3JVFK), and a manager-only one keeps
		// its existing behavior of raising per-method inside readGuard.
		ls.RaiseError("rela.bypass_acl: no elevated handle is available")
		return 0
	}

	// live gates every admin.* call; set false after fn returns so a captured
	// handle is dead outside the closure's dynamic extent.
	live := true
	// reads accumulates the distinct elevated read bindings this closure
	// used, for the single post-closure audit record (TKT-ACSBSA).
	reads := &readUsage{}
	admin := newElevatedHandle(ls, b.em, b.er, &live, reads, b.ctxFn)

	// Invalidate on every exit path (normal return or Lua error). pcall keeps
	// the runtime alive so we can flip `live` before re-raising.
	//
	// The audit record rides the SAME defer, so a closure that reads raw data
	// and then raises still leaves a trace. Recording only on the success
	// path would let a script read everything and erase the evidence by
	// failing — the exact shape an attacker would choose.
	defer func() {
		live = false
		recordElevatedReads(b.ctxFn(), b.recorder, reads)
	}()

	ls.Push(fn)
	ls.Push(admin)
	// Protected call so we can guarantee invalidation even when fn raises,
	// then re-surface the error to the caller.
	if err := ls.PCall(1, lua.MultRet, nil); err != nil {
		live = false
		ls.RaiseError("rela.bypass_acl: %s", err.Error())
		return 0
	}
	// Return whatever fn returned (already on the stack after PCall).
	return ls.GetTop() - 1
}

// newElevatedHandle builds the `admin` table passed to a rela.bypass_acl
// closure. Its methods route to the elevated Mutator `em` (writes) and the
// elevated EntityReader `er` (reads), and check `*live` first, so they raise
// once the closure has returned. No principal, no nested bypass.
//
// Write surface: create_relation, delete_relation, delete_entity — the
// link/unlink + remove operations the system-invariant use cases (e.g.
// authorship stamping via created-by) need. create_entity / update_entity are a
// deliberate follow-up: they marshal a full entity table and aren't required by
// the motivating case; gating elevated *entity* creation is a larger surface
// best added with its own tests.
//
// Read surface (TKT-ACSBSA): get_entity, list_entities, get_relations —
// mirroring the gated rela.* bindings one-for-one so a script can lift a read
// into the closure without rewriting it. Reads are RAW: full properties, no
// row gate, no redaction. A half-elevated read is a confusing contract and the
// closure is already the boundary.
//
// A nil `er` leaves the three read methods present but RAISING, not absent.
// Absence would make `if admin.get_entity then` silently take the
// no-elevation branch on a misconfigured deployment; raising names the
// missing capability.
//
// That reasoning fits the cascade path, where reader and mutator are wired
// together under one check so a missing reader really is a wiring bug. The
// document path has a THIRD state the dichotomy does not cover: a wiring site
// may grant no elevation bundle at all, in which case neither handle is set
// and `rela.bypass_acl` is absent outright rather than present-and-raising
// (see documentService.elevatedDeps). All three states fail closed; they just
// fail in different shapes, so don't read this comment as "nil reader always
// means misconfiguration".
func newElevatedHandle(
	ls *lua.LState, em Mutator, er EntityReader, live *bool, reads *readUsage,
	ctxFn func() context.Context,
) *lua.LTable {
	t := ls.NewTable()
	guard := func(name string) bool {
		if !*live {
			ls.RaiseError("rela.bypass_acl: handle %q used outside its closure (invalidated)", name)
			return false
		}
		return true
	}
	// readGuard adds the wired-reader check to the liveness check. Both are
	// required before any elevated read touches the store.
	//
	// It deliberately does NOT mark the binding as used. Marking here would
	// audit a read that never reached the store — the argument-validation
	// raises (empty id, empty type) fire AFTER this guard, so a closure doing
	// only `pcall(admin.get_entity, "")` would produce an `acl-bypass-read`
	// row claiming a disclosure that never happened. Each method calls
	// reads.mark immediately before its er.* call instead, so the audit row
	// means what it says.
	readGuard := func(name string) bool {
		if !guard(name) {
			return false
		}
		if er == nil {
			ls.RaiseError("rela.bypass_acl: %s: no elevated reader is configured for this runtime", name)
			return false
		}
		return true
	}
	// Write methods are ABSENT (not present-and-raising) when no elevated
	// Mutator was wired — the asymmetry with `er` above is deliberate
	// (TKT-Y3JVFK). A nil `er` means "elevation was intended but the reader is
	// missing", i.e. a misconfiguration, so raising names the missing
	// capability. A nil `em` means the caller deliberately withheld elevated
	// writes, so `admin.delete_entity == nil` is the honest contract and a
	// script probing `if admin.delete_entity then` correctly learns it cannot
	// write past the ACL.
	//
	// Note the scope of that guarantee: it removes the ELEVATED write path,
	// not writing as such. A document render still holds the ordinary gated
	// rela.* write bindings (TKT-PX5YL7), so "this handle cannot bypass the
	// ACL to write" is the claim — not "this surface cannot mutate".
	if em != nil {
		registerElevatedWrites(ls, t, em, guard, ctxFn)
	}
	registerElevatedReads(ls, t, er, readGuard, ctxFn, reads)
	return t
}

// readUsage accumulates which elevated read bindings a bypass_acl closure
// actually used, so the post-closure audit record can name them. Order is
// first-use, and each binding appears once — the record answers "what kind
// of raw access happened", not "how many times".
//
// Not safe for concurrent use, and does not need to be: a Lua state is
// single-goroutine, and one readUsage is scoped to one closure.
type readUsage struct{ names []string }

// mark records a use of binding `name`, ignoring repeats.
func (u *readUsage) mark(name string) {
	if slices.Contains(u.names, name) {
		return
	}
	u.names = append(u.names, name)
}

// recordElevatedReads emits the single post-closure audit notification when
// the closure used its read elevation. Silent when no recorder is wired or
// when the closure performed no elevated reads — a bypass_acl block that
// only writes is already covered by entitymanager's OpACLBypass rows, and
// an empty record would just add noise to the log.
func recordElevatedReads(ctx context.Context, rec ElevationRecorder, u *readUsage) {
	if rec == nil || len(u.names) == 0 {
		return
	}
	rec.RecordElevatedRead(ctx, u.names)
}

// registerElevatedWrites adds the raw write methods to the `admin` table
// (TKT-D8T148). Split out for the same reason as registerElevatedReads — the
// function-length limit — and called only when an elevated Mutator was wired,
// so a read-only elevation has no write methods at all (TKT-Y3JVFK).
//
// create_entity / update_entity remain absent by design: they marshal a full
// entity table and were deferred with their own tests as the follow-up noted
// in newElevatedHandle's doc.
func registerElevatedWrites(
	ls *lua.LState, t *lua.LTable, em Mutator, guard func(string) bool,
	ctxFn func() context.Context,
) {
	ls.SetField(t, "create_relation", ls.NewFunction(func(s *lua.LState) int {
		if !guard("create_relation") {
			return 0
		}
		from, relType, to := s.CheckString(1), s.CheckString(2), s.CheckString(3)
		if _, err := em.CreateRelation(ctxFn(), from, relType, to, entity.RelationOptions{}); err != nil {
			s.RaiseError("bypass_acl create_relation error: %s", err.Error())
			return 0
		}
		s.Push(lua.LTrue)
		return 1
	}))
	ls.SetField(t, "delete_relation", ls.NewFunction(func(s *lua.LState) int {
		if !guard("delete_relation") {
			return 0
		}
		from, relType, to := s.CheckString(1), s.CheckString(2), s.CheckString(3)
		if err := em.DeleteRelation(ctxFn(), from, relType, to); err != nil {
			s.RaiseError("bypass_acl delete_relation error: %s", err.Error())
			return 0
		}
		s.Push(lua.LTrue)
		return 1
	}))
	ls.SetField(t, "delete_entity", ls.NewFunction(func(s *lua.LState) int {
		if !guard("delete_entity") {
			return 0
		}
		id := s.CheckString(1)
		cascade := s.OptBool(2, false)
		if _, err := em.DeleteEntity(ctxFn(), id, cascade); err != nil {
			s.RaiseError("bypass_acl delete_entity error: %s", err.Error())
			return 0
		}
		s.Push(lua.LTrue)
		return 1
	}))
}

// registerElevatedReads adds the three raw read methods to the `admin` table
// (TKT-ACSBSA). Split out of newElevatedHandle to keep each function within
// the length limit; `readGuard` carries both the liveness and wired-reader
// checks so neither can be forgotten at an individual method.
//
// Deliberately NOT sharing code with the gated luaGetEntity / luaListEntities
// / luaGetRelations bindings: those funnel through r.reader() (which resolves
// VisibleReader), and a shared helper parameterized by reader would be one
// edit away from letting a gated binding read raw. The duplication here is
// small and it keeps the two read paths physically separate.
func registerElevatedReads(
	ls *lua.LState, t *lua.LTable, er EntityReader, readGuard func(string) bool,
	ctxFn func() context.Context, reads *readUsage,
) {
	ls.SetField(t, "get_entity", ls.NewFunction(elevatedGetEntity(er, readGuard, ctxFn, reads)))
	ls.SetField(t, "list_entities", ls.NewFunction(elevatedListEntities(er, readGuard, ctxFn, reads)))
	ls.SetField(t, "get_relations", ls.NewFunction(elevatedGetRelations(er, readGuard, ctxFn, reads)))
}

// elevatedGetEntity builds admin.get_entity(id) -> table|nil.
//
// Returns nil on a miss, matching rela.get_entity. The two nils mean
// different things, though: under elevation a nil means the entity
// genuinely does not exist, where the gated binding's nil is the
// deliberately ambiguous "missing or hidden" that keeps it oracle-free.
func elevatedGetEntity(
	er EntityReader, readGuard func(string) bool, ctxFn func() context.Context,
	reads *readUsage,
) func(*lua.LState) int {
	return func(s *lua.LState) int {
		if !readGuard("get_entity") {
			return 0
		}
		id := s.CheckString(1)
		if id == "" {
			s.RaiseError("bypass_acl get_entity: entity ID cannot be empty")
			return 0
		}
		reads.mark("get_entity")
		e, err := er.GetEntity(ctxFn(), id)
		if err != nil {
			// Only a genuine MISS is nil. Any other error (store down, driver
			// failure) RAISES — masking it as nil would make the documented
			// contract ("nil means it does not exist") false, and would break
			// the motivating use case: a uniqueness check that reads nil on a
			// transient outage concludes "no duplicate" and lets the invariant
			// the elevated read exists to enforce be violated. The two list
			// bindings already raise on iteration errors; this keeps the three
			// consistent.
			if errors.Is(err, store.ErrNotFound) {
				s.Push(lua.LNil)
				return 1
			}
			s.RaiseError("bypass_acl get_entity error: %s", err.Error())
			return 0
		}
		s.Push(EntityToTable(s, e))
		return 1
	}
}

// elevatedListEntities builds admin.list_entities(type) -> table.
//
// No filter-expression argument: rela.list_entities' filter is a
// convenience over an already-gated set, and adding an expression parser to
// the elevated path widens it for no gain — a script can filter the
// returned table in Lua. Unbounded, like its gated counterpart
// (TKT-YWDGZD tracks paging for both).
func elevatedListEntities(
	er EntityReader, readGuard func(string) bool, ctxFn func() context.Context,
	reads *readUsage,
) func(*lua.LState) int {
	return func(s *lua.LState) int {
		if !readGuard("list_entities") {
			return 0
		}
		entityType := s.CheckString(1)
		if entityType == "" {
			s.RaiseError("bypass_acl list_entities: entity type cannot be empty")
			return 0
		}
		reads.mark("list_entities")
		result := s.NewTable()
		idx := 1
		for e, err := range er.ListEntities(ctxFn(), store.EntityQuery{Type: entityType}) {
			if err != nil {
				s.RaiseError("bypass_acl list_entities error: %s", err.Error())
				return 0
			}
			result.RawSetInt(idx, EntityToTable(s, e))
			idx++
		}
		s.Push(result)
		return 1
	}
}

// elevatedGetRelations builds admin.get_relations(opts?) -> table, with
// opts.{from,type,to}.
//
// NOT peer-gated (unlike rela.get_relations): an edge is returned even when
// neither endpoint would be visible to the caller. Re-adding the peer drop
// here would look like a safety improvement and would silently make the
// elevated view incomplete.
func elevatedGetRelations(
	er EntityReader, readGuard func(string) bool, ctxFn func() context.Context,
	reads *readUsage,
) func(*lua.LState) int {
	return func(s *lua.LState) int {
		if !readGuard("get_relations") {
			return 0
		}
		q, err := relationQuery(s)
		if err != nil {
			s.RaiseError("bypass_acl get_relations: %s", err.Error())
			return 0
		}
		reads.mark("get_relations")
		result := s.NewTable()
		idx := 1
		for rel, err := range er.ListRelations(ctxFn(), q) {
			if err != nil {
				s.RaiseError("bypass_acl get_relations error: %s", err.Error())
				return 0
			}
			result.RawSetInt(idx, relationToTable(s, rel))
			idx++
		}
		s.Push(result)
		return 1
	}
}
