package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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
// A package-level function for the same reason [CompiledWorlds] is: Services
// sits at its plimsoll exported-method cap.
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
// [RelationScopes] is one: Services sits at its plimsoll exported-method cap,
// and that cap is a ratchet to narrow rather than raise.
func CompiledWorlds(svc *Services) worlds.Compiled {
	if svc == nil {
		return worlds.Compiled{}
	}
	return svc.worlds
}
