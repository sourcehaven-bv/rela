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
	"fmt"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/validation"
)

// Reader is the read surface this adapter needs. Consumer-side, and
// deliberately the same two methods lua.EntityReader already offers, so every
// existing wiring site can pass the gated reader it already holds — which is
// what keeps the gate's ACL behaviour identical to before the seam existed.
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
func New(r Reader) (*Graph, error) {
	if r == nil {
		return nil, fmt.Errorf("validationgraph: nil reader")
	}
	return &Graph{r: r}, nil
}

// RelatedEntities implements [validation.Graph].
//
// # Why the query is keyed on EntityID, not From
//
// store.RelationQuery honours Direction for EntityID/EntityIDs ONLY; From and
// To are matched unconditionally and independently, and no validation rejects
// a query that sets both. So {From: id, Direction: DirectionIncoming} does not
// mean "edges arriving at id" — it still means "edges leaving id", silently,
// with plausible-looking results. Keying on EntityID is what makes Direction
// load-bearing here.
func (g *Graph) RelatedEntities(
	ctx context.Context, subjectID, relType string, dir validation.Direction,
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
		out = append(out, g.resolve(ctx, farEndOf(rel, dir)))
	}
	return out, nil
}

// resolve reads the far entity, reporting failure as an UNRESOLVED element
// rather than an error.
//
// The distinction matters: dropping the edge would undercount, which a `max:`
// bound treats as success — so a gate guarding against something unreadable
// would pass precisely because it could not look. The caller decides what an
// unresolved edge means for each bound.
func (g *Graph) resolve(ctx context.Context, farID string) validation.Related {
	e, err := g.r.GetEntity(ctx, farID)
	if err != nil || e == nil {
		return validation.Related{ID: farID}
	}
	return validation.Related{
		ID:         e.ID,
		Type:       e.Type,
		Properties: e.Properties,
		Resolved:   true,
	}
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
