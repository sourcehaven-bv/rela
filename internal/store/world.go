package store

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ErrInvalidQuery reports a query whose fields contradict each other, as
// opposed to one that simply matches nothing. Today the only such pair is
// AllStates together with a non-default World on an [EntityQuery]: raw
// storage truth and world resolution are opposite intents, and silently
// picking one is a precedence rule nobody remembers.
var ErrInvalidQuery = errors.New("store: invalid query")

// Fallback is what a world does with an entity whose type declares
// pointers but none the world selects — resolution rule 3 (design doc
// §4.1). It is compiled from the metamodel's mandatory `otherwise:`.
//
// The ZERO VALUE IS EXCLUSION, deliberately. A half-built or
// default-constructed resolution then hides content rather than leaking a
// draft into a published world, which is the direction this whole feature
// exists to fail in. Note this is the OPPOSITE-feeling zero value from
// [WorldScope]'s — see that type's doc.
type Fallback int

const (
	// FallbackExclude contributes nothing: the entity is absent from the
	// world, and absence IS the publication bit (design doc §4.1 —
	// "unpublished" literally means "nonexistent in world:published",
	// which lines up with the row gate's hidden-equals-nonexistent
	// contract). The safe zero value.
	FallbackExclude Fallback = iota
	// FallbackDefaultState resolves to the entity's default state.
	FallbackDefaultState
)

// String names the verdict. Worth having early: this is an int enum whose
// zero means EXCLUDE while the adjacent [WorldScope] zero means "include
// everything via the default state", so a bare `fallback=0` in a log line
// or a test failure is exactly the confusion these types are documented
// against.
func (f Fallback) String() string {
	switch f {
	case FallbackExclude:
		return "exclude"
	case FallbackDefaultState:
		return "default-state"
	default:
		return fmt.Sprintf("Fallback(%d)", int(f))
	}
}

// TypeResolution is one entity type's resolution under one world: an
// ordered candidate chain plus the verdict when none of it exists.
type TypeResolution struct {
	// Chain is the ordered candidate coordinates; index 0 wins. The
	// first coordinate that EXISTS for an entity is that entity's prime.
	// Length 1 is the common case (`select: published`).
	//
	// Ordering is the entire semantic content: a world is a per-family
	// ranked preference, NOT a row predicate. `pointer IN (a, b)` would
	// return two rows for an entity holding both and break the
	// at-most-one-prime invariant everything leans on (design doc §4.2).
	Chain []entity.Pointer

	// Fallback is the verdict when no Chain coordinate has a row.
	Fallback Fallback
}

// WorldScope is a compiled world: the resolution function expressed in
// the only vocabulary a store speaks — entity type to ranked pointer
// coordinates. It is built by internal/worlds from the metamodel, and it
// is metamodel-FREE by construction so that stores (which must not
// consult a metamodel) and internal/visibility (which arch-lint forbids
// from importing metamodel) can both hold one.
//
// # The two zero values mean different things, on purpose
//
// The zero WorldScope is the DEFAULT WORLD: total, every entity present
// via its default state, byte-identical to the pre-worlds system — which
// is what lets every existing query construction site keep working
// untouched. The zero [Fallback], by contrast, is EXCLUSION. Both choices
// are the safe direction for their own type: an unconfigured scope must
// not start hiding things, and an unconfigured fallback must not start
// revealing them.
//
// # Absence is not the zero value
//
// A type ABSENT from the compiled map contributes its default state in
// every world (resolution rule 1: a type declaring no pointers needs no
// per-type work, so a mixed graph costs nothing). That is the OPPOSITE
// of the zero TypeResolution, which excludes. Because Go's map-index
// syntax silently returns the zero value on a miss, the map is
// unexported and reachable only through [WorldScope.For], which returns
// the two-valued answer. Do not add a getter that collapses them.
//
// Stores only ever EQUALITY-MATCH the coordinates in a chain; they never
// parse or inspect one (see [entity.Pointer]).
//
// # Type keys are CANONICAL
//
// The compiler keys this map on canonical entity-type names only, never
// on aliases. A store cannot resolve an alias — it holds no metamodel —
// so a caller querying by a type name it took from user input or from a
// stored row must canonicalize (metamodel.GetEntityDef / ResolveAlias)
// BEFORE calling [WorldScope.For]. Skipping that is fail-open, not
// fail-closed: an alias reaches For as an unknown type, which is ok=false,
// which is rule 1 — the default state served in a world that meant to
// exclude it.
type WorldScope struct {
	byType map[string]TypeResolution
}

// NewWorldScope compiles a per-type resolution map into a WorldScope.
// Passing an empty or nil map yields the default world.
//
// The map and every Chain in it are copied, so the caller may reuse or
// mutate its input: a WorldScope is handed to every backend and must not
// change underfoot. The copy must be DEEP — a shallow map copy leaves
// each TypeResolution.Chain sharing the caller's backing array, and one
// assembled Compiled is handed to every tenant, so a consumer sorting or
// truncating a chain in place would silently re-scope every reader's
// world. That is the unbounded direction: it can turn `select: published`
// into `select: draft`.
func NewWorldScope(byType map[string]TypeResolution) WorldScope {
	if len(byType) == 0 {
		return WorldScope{}
	}
	cp := make(map[string]TypeResolution, len(byType))
	for typ, res := range byType {
		cp[typ] = TypeResolution{Chain: slices.Clone(res.Chain), Fallback: res.Fallback}
	}
	return WorldScope{byType: cp}
}

// DefaultWorld returns the implicit total world: every entity contributes
// its default state. It is the zero value, named for call sites that want
// to say so explicitly rather than passing a bare WorldScope{}.
func DefaultWorld() WorldScope { return WorldScope{} }

// IsDefaultWorld reports whether w resolves every entity to its default
// state. Backends branch on this for the historical fast path: it must
// reduce to exactly the pre-worlds empty-pointer query, allocating
// nothing, so a project that never declares a pointer pays nothing.
func (w WorldScope) IsDefaultWorld() bool { return len(w.byType) == 0 }

// For returns the resolution w applies to entityType, and whether one is
// declared at all.
//
// ok=false means the type has NO content states in this world and
// contributes its default state (rule 1) — it does NOT mean "exclude".
// Callers must branch on ok; see the type doc for why this is not a
// plain map lookup.
//
// The returned Chain is a copy: the caller owns it and may sort, filter
// or truncate it in place without re-scoping the shared WorldScope.
func (w WorldScope) For(entityType string) (res TypeResolution, ok bool) {
	res, ok = w.byType[entityType]
	if !ok {
		return TypeResolution{}, false
	}
	return TypeResolution{Chain: slices.Clone(res.Chain), Fallback: res.Fallback}, true
}

// Types returns the entity types w declares a resolution for. The order
// is unspecified. Intended for backends composing a multi-type query and
// for diagnostics; single-type paths use [WorldScope.For].
func (w WorldScope) Types() []string {
	if len(w.byType) == 0 {
		return nil
	}
	out := make([]string, 0, len(w.byType))
	for typ := range w.byType {
		out = append(out, typ)
	}
	return out
}
