// Package validationgraph adapts a store-backed reader to the graph surface
// the validation engine's relation-cardinality gates need.
//
// It exists because the two entry points into a validation.Service —
// internal/validator and internal/analysis — have no package in common that
// may hold a store handle. analysis cannot import validator (arch-lint), and
// validation itself must not import internal/store, so before this the only
// shared seam was a helper hung off lua.ReadDeps, which is a capability
// bundle rather than an interface anyone declared. That accident is why the
// gate was outgoing-only: direction was never a parameter.
//
// This package is a leaf: it depends on entity, metamodel and store, and
// nothing depends on it except the wiring sites.
package validationgraph

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/validation"
)

// Reader is the read surface this adapter needs. Consumer-side, and
// deliberately the same two methods lua.EntityReader already offers, so every
// existing wiring site can pass the gated reader it already holds — which is
// what keeps the gate's ACL behavior identical to before the seam existed.
//
// Nil: rejected by [New].
type Reader interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// Graph implements [validation.Graph] over a Reader.
type Graph struct {
	r Reader
}

var _ validation.Graph = (*Graph)(nil)

// New returns a Graph reading through r.
//
// Whatever r is decides what the gate can count: pass a visibility-gated
// reader and constraints count only what the acting identity sees; pass a raw
// store and they count everything. Both are legitimate — the CLI and CI paths
// want the second — but the choice belongs at the wiring site, not here.
//
// Nil r is rejected: a gate that counts nothing because nothing was wired
// would report `min:` violations everywhere and satisfy every `max:`.
//
// A TYPED nil — a nil pointer inside a non-nil interface — is rejected too.
// It survives a plain `!= nil` check at the wiring site and then panics on
// first use, which surfaces as a crash in whichever request happened to
// evaluate a gate rather than as the wiring mistake it is.
func New(r Reader) (*Graph, error) {
	if r == nil || isTypedNil(r) {
		return nil, errors.New("validationgraph: nil reader")
	}
	return &Graph{r: r}, nil
}

// isTypedNil reports whether v holds a nil pointer, map, slice, func or
// channel inside a non-nil interface.
func isTypedNil(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}

// RelatedEntities implements [validation.Graph].
//
// # Why the query is keyed on EntityID, not From
//
// store.RelationQuery honors Direction for EntityID/EntityIDs ONLY; From and
// To are matched unconditionally and independently, and no validation rejects
// a query that sets both. So {From: id, Direction: DirectionIncoming} does not
// mean "edges arriving at id" — it still means "edges leaving id", silently,
// with plausible-looking results. Keying on EntityID is what makes Direction
// load-bearing here.
func (g *Graph) RelatedEntities(
	ctx context.Context, subjectID, relType string, dir validation.Direction, resolveFar bool,
) ([]validation.Related, error) {
	q := store.RelationQuery{
		EntityID:  subjectID,
		Type:      relType,
		Direction: directionOf(dir),
	}

	var out []validation.Related
	for rel, err := range g.r.ListRelations(ctx, q) {
		if err != nil {
			return nil, fmt.Errorf("listing %q relations for %q: %w", relType, subjectID, err)
		}
		if rel == nil {
			continue
		}
		farID := farEndOf(rel, dir)
		if !resolveFar {
			out = append(out, validation.Related{ID: farID})
			continue
		}
		r, rErr := g.resolve(ctx, farID)
		if rErr != nil {
			return nil, fmt.Errorf("reading far entity %q of %q relation: %w", farID, relType, rErr)
		}
		out = append(out, r)
	}
	return out, nil
}

// resolve reads the far entity.
//
// A MISSING entity is reported as an UNRESOLVED element, not an error: the
// edge genuinely exists and the caller must still decide what it means per
// bound. Dropping it would undercount, which a `max:` bound treats as
// success — a gate guarding against something unreadable would pass precisely
// because it could not look.
//
// Any OTHER error is returned. A read gate that failed to build
// (visibility's fail-closed sentinel) is not a dangling reference, and
// flattening the two would turn "this check could not run" into a
// content-shaped "has 0" violation. The gate must say it could not run.
func (g *Graph) resolve(ctx context.Context, farID string) (validation.Related, error) {
	e, err := g.r.GetEntity(ctx, farID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return validation.Related{ID: farID}, nil
	case err != nil:
		return validation.Related{}, err
	case e == nil:
		// A nil entity with no error is a backend contract violation, but
		// treating it as absent keeps the gate honest rather than panicking.
		return validation.Related{ID: farID}, nil
	}
	return validation.Related{
		ID:   e.ID,
		Type: e.Type,
		// Aliases the store's map. Every backend clones on read, so this is
		// safe by the store contract; it is read-only here regardless.
		Properties: e.Properties,
		Resolved:   true,
	}, nil
}

// farEndOf returns the id at the end of rel the subject is NOT on.
//
// Kept here rather than at the call site on purpose: picking the wrong end is
// invisible in the result — real edges counted against the wrong entities —
// so the choice lives in one place, next to the query that selected them.
func farEndOf(rel *entity.Relation, dir validation.Direction) string {
	if dir == validation.DirectionIncoming {
		return rel.From
	}
	return rel.To
}

// directionOf maps the validation-local direction onto the store's.
func directionOf(dir validation.Direction) store.Direction {
	if dir == validation.DirectionIncoming {
		return store.DirectionIncoming
	}
	return store.DirectionOutgoing
}
