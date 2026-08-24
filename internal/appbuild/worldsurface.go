package appbuild

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// metamodelCanon adapts the metamodel to worldreader.TypeCanonicalizer.
//
// It lives HERE rather than in worldreader because that package must not
// import metamodel (arch-lint pins it to entity + store). A compiled
// WorldScope is metamodel-free by construction, and the alias table is
// the one metamodel fact the resolver needs — so it arrives as a narrow
// interface satisfied at the wiring site.
type metamodelCanon struct{ meta *metamodel.Metamodel }

func (c metamodelCanon) CanonicalType(name string) (string, bool) {
	if c.meta == nil {
		return name, false
	}
	return c.meta.ResolveAlias(name), true
}

// metamodelScopes adapts the metamodel to worldreader.ScopeClassifier.
//
// Identity is the DEFAULT: a relation type that declares no scope, or one
// the metamodel does not know, is identity-scoped. That is the direction
// that keeps a pointerless project unchanged, and it is also the safe
// direction here — an identity edge is visible from every face, so
// misclassifying a content edge as identity shows an edge that a stricter
// reading would hide, rather than hiding one a reader needs.
type metamodelScopes struct{ meta *metamodel.Metamodel }

func (c metamodelScopes) IsContentScoped(relType string) bool {
	if c.meta == nil {
		return false
	}
	def, ok := c.meta.GetRelationDef(relType)
	if !ok {
		return false
	}
	return def.Scope.IsContent()
}

// RelationScopes returns svc's relation scope classifier, for surfaces that
// resolve LINKS through a world (TKT-WRLDAPI item 4).
//
// Returned as the metamodel-free [worldreader.ScopeClassifier] interface, so
// a consumer that may not import internal/metamodel's relation defs — and
// must not reimplement the identity-vs-content dispatch — can supply the one
// metamodel fact worldreader.RelationReader needs.
//
// A package-level function for the same reason [WorldSurface] and
// [CompiledWorlds] are: Services sits at its plimsoll exported-method cap.
func RelationScopes(svc *Services) worldreader.ScopeClassifier {
	if svc == nil {
		return metamodelScopes{}
	}
	return metamodelScopes{meta: svc.meta}
}

// CompiledWorlds returns svc's compiled world map, for surfaces that offer
// request-level world selection (TKT-DN37J2).
//
// The returned value satisfies a consumer-side lookup interface by having a
// Lookup(name) (store.WorldScope, bool) method, so a consumer that may not
// import internal/worlds (internal/dataentry, per arch-lint) can still
// resolve a world NAME to its metamodel-free compiled scope.
//
// A package-level function rather than a Services method for the same reason
// [WorldSurface] is one: Services sits at its plimsoll exported-method cap,
// and that cap is a ratchet to narrow rather than raise.
func CompiledWorlds(svc *Services) worlds.Compiled {
	if svc == nil {
		return worlds.Compiled{}
	}
	return svc.worlds
}

// WorldSurface builds a read surface bound to the named world.
//
// A package-level FUNCTION rather than a method on [Services], deliberately.
// Services sits at its god-object load line (25 exported methods, pinned by a
// plimsoll directive), and the project rule is to split the type rather than
// raise the number. Nothing here needs to be a method: it reads three fields
// off svc and composes worldreader types, so a function keeps the composition
// root's public surface flat while the wiring stays in one place.
//
// The world is resolved by NAME from the compiled set and then FIXED: the
// returned surface has no world parameter and no setter (Q10). A caller
// that wants a different world builds a different surface. Request-level
// world selection is deliberately not available here — it needs its own
// grant check, and shipping a plumbed-but-ungated world parameter is the
// half-built thing this arc declined to build.
//
// Passing the empty name yields the DEFAULT world: every entity via its
// default state, byte-identical to the pre-worlds system.
//
// searcher may be nil. When it is not, and the world is non-default, it
// must be able to honor that world — NewSurface refuses otherwise
// (RULING 3). No searcher can today, so a world-bound surface is
// currently search-free by construction rather than by remembering.
func WorldSurface(
	svc *Services, name string, searcher worldreader.WorldAwareSearcher,
) (*worldreader.Surface, error) {
	if svc == nil {
		return nil, errors.New("appbuild: WorldSurface: services must be non-nil")
	}
	scope := store.DefaultWorld()
	if name != "" {
		found, ok := svc.worlds.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("appbuild: no such world %q", name)
		}
		scope = found
	}

	resolver, err := worldreader.NewResolver(svc.store, scope, metamodelCanon{meta: svc.meta})
	if err != nil {
		return nil, fmt.Errorf("appbuild: world surface: %w", err)
	}
	relations, err := worldreader.NewRelationReader(svc.store, metamodelScopes{meta: svc.meta})
	if err != nil {
		return nil, fmt.Errorf("appbuild: world surface: %w", err)
	}
	surface, err := worldreader.NewSurface(resolver, relations, searcher)
	if err != nil {
		return nil, fmt.Errorf("appbuild: world surface %q: %w", name, err)
	}
	return surface, nil
}
