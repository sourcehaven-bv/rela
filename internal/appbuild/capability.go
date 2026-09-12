//go:build postgres

package appbuild

import "reflect"

// Capability resolvers must never hand back a typed nil.
//
// # Why interface `== nil` is not enough
//
// A nil POINTER boxed into an interface makes that interface non-nil. So when a
// resolver's callee returns an interface type, `v == nil` is true only when the
// callee returned an already-untyped nil — a backend doing `var p *MyStore;
// return p` sails straight through. Every downstream nil-check then passes and
// the panic lands at first use: at write time, in production, far from here.
//
// This was not hypothetical. TKT-L3FNEN widened VersionStore() from a concrete
// pointer to store.VersionService and carried the pointer-shaped `vs == nil`
// check across unchanged — which silently stopped working in the same commit
// that made the hazard reachable by more implementations. The doc comment
// describing the danger was written before the change made it real, so it read
// as still-true.
//
// One helper rather than a check per resolver, deliberately: three hand-rolled
// guards were what let the regression through, because "a guard exists" was
// true at all three sites while "the guard works" was true at one.
//
// Nil: returns the zero T for a nil or nil-wrapping value, so callers get a
// genuinely nil interface and their fallbacks engage.
func nonNilCapability[T comparable](v T) T {
	var zero T
	if v == zero || wrapsNil(v) {
		return zero
	}
	return v
}

// wrapsNil reports whether v is a non-nil interface holding a nil pointer, map,
// slice, channel or func. Reflection is the only way to see through the box.
func wrapsNil(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	default:
		// Not a nilable kind (a struct value, say), so it cannot wrap nil.
		return false
	}
}
