package dataentry

import (
	"context"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// An entity ADDRESS arrives in a URL path segment as the bare id or as
// `ID@face`, naming one stored content state (TKT-SLFURL). It stays a string
// until it reaches [visibleReader.address], which hands it to
// [visibility.Resolver.Address]: a named face is read literally under every
// world, and a bare id is resolved by the request's world. A resolved row
// gives its own address back with [entity.Entity.Ref].
//
// The path carries the operator's declared face name, the same vocabulary
// `_world.face` and `_faces[].label` use, and that name IS the coordinate the
// row is stored at (BUG-HC6I2T), so an address needs no translation and no
// face answers to two spellings. An address the grammar rejects names no row
// and gets the same not-found a missing row does: a distinct 400 would only
// tell a caller which strings are worth probing.

// isExplicitAddress reports whether the address names a face. A malformed
// address names none.
func isExplicitAddress(raw string) bool {
	ref, err := entity.ParseRef(raw)
	return err == nil && !ref.Face.IsImplicit()
}

// addressedProvenance labels a face served because the caller named it, as
// opposed to one the world resolved (see [worldProvenance]). The rule is
// `unscoped`, since the address chose the face, not the world. The world
// name is still the request's, because neighbors on the response resolve
// through it.
func addressedProvenance(ctx context.Context, e *entity.Entity) *v1.EntityWorld {
	if e == nil {
		return nil
	}
	name := worldFromContext(ctx).name
	if name == "" {
		name = metamodel.DefaultWorldName
	}
	return &v1.EntityWorld{
		Name: name,
		Face: e.Face.String(),
		Via:  ruleUnscoped,
	}
}
