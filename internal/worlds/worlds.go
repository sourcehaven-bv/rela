// Package worlds compiles the metamodel's `worlds:` declarations into
// [store.WorldScope] values — the metamodel-free, per-type ranked
// resolution the storage layer evaluates (TKT-WAV8XP, design doc §4).
//
// # Why a separate package
//
// The compiled form must be metamodel-free: stores must not consult a
// metamodel, and internal/visibility may not import one (arch-lint). But
// compiling obviously needs the metamodel. So the boundary is here — the
// one place that reads `worlds:` / `pointers:` and emits coordinates —
// and nothing downstream of it knows a metamodel exists.
//
// This is also where pointer NAME GRAMMAR is enforced. internal/metamodel
// cannot do it: arch-lint keeps internal/entity a leaf that metamodel may
// not import, and [entity.ParsePointer] is the single codec that turns
// external text into a [entity.Pointer]. Structural validation (mandatory
// `otherwise:`, chains naming declared pointers, one default per type)
// stays in the loader where the rest of the schema is checked; grammar
// lands here. Both run before anything serves a request: Compile is
// called during application assembly, so a bad pointer name is a startup
// failure, never a lurking runtime one.
//
// # What a world compiles to
//
// A per-type ordered chain of coordinates plus a fallback verdict. NOT a
// row predicate: a world picks at most one state per entity (the prime),
// which is a per-family ranked preference — `pointer IN (draft,
// published)` would return two rows for an entity holding both and break
// the invariant everything else leans on (design doc §4.2).
package worlds

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Compiled is the result of compiling a metamodel's worlds: every
// declared world by name, plus the implicit default world.
//
// The ZERO VALUE is usable and means "no declared worlds": [Compiled.Lookup]
// still answers the default world, and [Compiled.Names] is empty. That is
// also what Compile returns for a nil metamodel or one with no `worlds:`.
type Compiled struct {
	byName map[string]store.WorldScope
}

// Default returns the implicit default world — total, every entity via
// its default state. Always available, declared or not.
func Default() store.WorldScope { return store.DefaultWorld() }

// Lookup returns the compiled scope for a world name.
//
// [metamodel.DefaultWorldName] always resolves, even for a project with
// no `worlds:` block, because that world is implicit. Any other unknown
// name returns ok=false — callers fail closed rather than substituting
// the default world, which would silently widen a world-bound surface.
func (c Compiled) Lookup(name string) (scope store.WorldScope, ok bool) {
	if name == metamodel.DefaultWorldName {
		return Default(), true
	}
	scope, ok = c.byName[name]
	return scope, ok
}

// Names returns the declared world names in sorted order, NOT including
// the implicit default world.
func (c Compiled) Names() []string {
	out := make([]string, 0, len(c.byName))
	for name := range c.byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Compile turns a metamodel's declarations into compiled world scopes.
//
// It reports EVERY problem it finds rather than the first, matching the
// loader's collect-then-report discipline: an operator fixing a schema
// should see the whole list. A nil metamodel compiles to just the
// implicit default world.
//
// Errors name the entity type, the offending coordinate, and the grammar,
// so a schema typo is as diagnosable here as it would be from the loader.
func Compile(m *metamodel.Metamodel) (Compiled, error) {
	if m == nil {
		return Compiled{}, nil
	}
	pointers, errs := declaredPointers(m)
	// Bail BEFORE compiling anything. A type whose pointer names all failed
	// the grammar has an empty entry in `pointers`, which compileWorld would
	// read as rule 1 — "declares no pointers, contributes its default state
	// in every world" — the exact opposite of what a type declaring content
	// states means. Today that scope is discarded, so the fail-open is only
	// latent; returning here keeps it unreachable even if a future caller
	// wants a best-effort compile.
	if err := joinErrors(errs); err != nil {
		return Compiled{}, err
	}
	if len(m.Worlds) == 0 {
		// No worlds declared: nothing to compile, but the pointer
		// grammar still had to hold — a project may declare states
		// before it declares any world that selects them.
		return Compiled{}, nil
	}

	byName := make(map[string]store.WorldScope, len(m.Worlds))
	for _, name := range sortedWorldNames(m) {
		byName[name] = compileWorld(m, m.Worlds[name], pointers)
	}
	return Compiled{byName: byName}, nil
}

// declaredPointers validates every declared pointer name against the
// codec grammar and returns them as parsed coordinates, keyed by entity
// type. Types declaring no pointers are absent from the result — that
// absence is what rule 1 compiles to.
func declaredPointers(m *metamodel.Metamodel) (map[string]map[string]entity.Pointer, []error) {
	var errs []error
	out := make(map[string]map[string]entity.Pointer)
	for _, typeName := range sortedTypeNames(m) {
		def := m.Entities[typeName]
		if len(def.Pointers) == 0 {
			continue
		}
		parsed := make(map[string]entity.Pointer, len(def.Pointers))
		for _, name := range sortedPointerNames(def) {
			p, err := entity.ParsePointer(name)
			if err != nil {
				errs = append(errs, fmt.Errorf(
					"entity %q: invalid pointer name %q: %w", typeName, name, err))
				continue
			}
			parsed[name] = p
		}
		out[typeName] = parsed
	}
	return out, errs
}

// compileWorld builds one world's per-type resolution.
//
// Rule 1 (a type declaring no pointers) is compiled as ABSENCE from the
// map, not as an entry — so a mixed graph costs nothing per pointerless
// type, and the store's fast paths stay untouched for them.
func compileWorld(
	m *metamodel.Metamodel,
	def metamodel.WorldDef,
	pointers map[string]map[string]entity.Pointer,
) store.WorldScope {
	// Anything that is not an explicit `otherwise: default` compiles to
	// exclusion, deliberately: the loader rejects an unrecognized value, so
	// this branch is unreachable for a loaded schema, and if it ever became
	// reachable the fail-closed direction is the one to land in.
	fallback := store.FallbackExclude
	if def.Otherwise == metamodel.OtherwiseDefault {
		fallback = store.FallbackDefaultState
	}

	byType := make(map[string]store.TypeResolution)
	for _, typeName := range sortedTypeNames(m) {
		declared := pointers[typeName]
		if len(declared) == 0 {
			continue // rule 1: absent from the map
		}
		rawChain, _ := def.ChainFor(typeName)
		byType[typeName] = store.TypeResolution{
			Chain:    resolveChain(rawChain, declared),
			Fallback: fallback,
		}
	}
	return store.NewWorldScope(byType)
}

// resolveChain maps a world's declared chain onto one type's coordinates,
// dropping coordinates the type does not declare and deduplicating.
//
// Dropping is CORRECT, not lenient: a world selecting `published` applies
// to types that have a published state, and a type without one falls to
// the world's `otherwise:` — that is resolution rule 3, and it is exactly
// what the mandatory `otherwise:` exists to answer. The loader separately
// rejects a chain no type at all could satisfy (a typo) and an override
// naming a coordinate its own type lacks (a mistake).
//
// Dedup happens AFTER mapping so a chain that repeats a coordinate
// collapses rather than ranking it twice.
func resolveChain(raw []string, declared map[string]entity.Pointer) []entity.Pointer {
	if len(raw) == 0 {
		return nil
	}
	var chain []entity.Pointer
	seen := make(map[entity.Pointer]bool, len(raw))
	for _, name := range raw {
		p, ok := declared[name]
		if !ok {
			// Not declared by THIS type: rule 3 territory, not an error.
			// A name no type declares is caught by the loader; one that
			// failed the grammar was already reported by
			// declaredPointers, so it is absent here and must not be
			// double-reported.
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		chain = append(chain, p)
	}
	return chain
}

func sortedWorldNames(m *metamodel.Metamodel) []string {
	out := make([]string, 0, len(m.Worlds))
	for name := range m.Worlds {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sortedTypeNames(m *metamodel.Metamodel) []string {
	out := make([]string, 0, len(m.Entities))
	for name := range m.Entities {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sortedPointerNames(def metamodel.EntityDef) []string {
	out := make([]string, 0, len(def.Pointers))
	for name := range def.Pointers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// joinErrors collapses collected errors into one, or nil when empty.
//
// Every problem is reported, not just the first: an operator fixing a
// schema should see the whole list, matching the metamodel loader's
// collect-then-report discipline.
func joinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("worlds: %w", errs[0])
	}
	// errors.Join, not a string join: flattening to text would make
	// errors.Is/As work for one problem and stop working for two, which is
	// the kind of asymmetry that surprises a caller much later.
	return fmt.Errorf("worlds: %d problems: %w", len(errs), errors.Join(errs...))
}
