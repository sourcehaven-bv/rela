package worldreader

import (
	"context"
	"errors"
	"iter"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RelationLister is the narrow relation-read surface the dispatcher
// needs. Declared at the call site.
type RelationLister interface {
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// ScopeClassifier answers whether a relation type is CONTENT-scoped
// (attached to one state) or IDENTITY-scoped (attached to the entity as
// such). It is satisfied by the metamodel; declared here so this package
// need not import it.
//
// Identity is the default: a type that declares nothing is identity-
// scoped, which is what keeps a faceless project unchanged.
type ScopeClassifier interface {
	IsContentScoped(relType string) bool
}

// RelationReader issues relation queries already scoped to a world.
//
// It exists so the scope dispatch is UNREPRESENTABLE to omit. A caller
// handed this capability cannot issue a raw nil-tail query through it;
// the only way to read relations on a world-resolved surface is to go
// through Neighbors, which applies the dispatch. Handing out a
// [store.RelationQuery] and trusting callers to set FromFace
// correctly would make the safe path the one you have to remember.
type RelationReader struct {
	lister  RelationLister
	classes ScopeClassifier
}

// NewRelationReader builds the world-scoped relation capability.
func NewRelationReader(lister RelationLister, classes ScopeClassifier) (*RelationReader, error) {
	if lister == nil {
		return nil, errors.New("worldreader: NewRelationReader: lister must be non-nil")
	}
	if classes == nil {
		return nil, errors.New("worldreader: NewRelationReader: classes must be non-nil")
	}
	return &RelationReader{lister: lister, classes: classes}, nil
}

// Neighbors returns the edges of one resolved entity under its world.
//
// The dispatch is per relation TYPE, which is why it cannot be one query
// (Q4). [store.RelationQuery] deliberately gained no world and no new
// selector — DOFYR1's FromFace contract stays frozen — so the two
// classes are queried separately and merged:
//
//   - IDENTITY-scoped types query with a NIL tail. An identity edge
//     belongs to the entity, not to a face of it, so it must be visible
//     from every face; filtering by the prime's face would hide an
//     entity's role and containment edges whenever its prime is not the
//     default state.
//   - CONTENT-scoped types query with the PRIME'S face, so a
//     non-prime state's content edges stay invisible.
//
// # The fallback trap
//
// When the prime came from `otherwise: default` (or from rule 1), its
// face IS the zero Face — and as a FromFace VALUE the zero
// face does not mean "unfiltered". It means DEFAULT-TAIL-ONLY, which
// is a different filter from nil. That is correct for the content-scoped
// query (the default face's own edges) and would be WRONG for the
// identity-scoped one, which must stay nil. The two look identical in a
// debugger — both are "the zero face" — which is why they are
// separated structurally here and pinned by a named test.
func (rr *RelationReader) Neighbors(
	ctx context.Context, res Resolved, dir store.Direction,
) ([]*entity.Relation, error) {
	if !res.Found || res.Entity == nil {
		// The world excludes this entity, so it has no edges IN THIS
		// WORLD. Returning storage's edges here would leak the
		// existence the exclusion is meant to withhold.
		return nil, nil
	}
	id := res.Entity.ID

	// Identity edges: NIL tail, deliberately. See the fallback trap above.
	identityQ := store.RelationQuery{
		Direction: dir,
		FromFace:  nil,
	}
	setEndpoint(&identityQ, id, dir)

	// Content edges: the prime's face BY VALUE, including when that
	// value is the zero face.
	prime := res.Face
	contentQ := store.RelationQuery{
		Direction: dir,
		FromFace:  &prime,
	}
	setEndpoint(&contentQ, id, dir)

	// Merge, keeping each edge under the query whose scope class matches
	// its type. Both queries over-return: the nil-tail query also
	// matches content edges, and the face query also matches identity
	// edges stored with that tail. Classifying on the way out is what
	// makes the merge exact rather than a union with duplicates.
	var out []*entity.Relation
	if err := collect(rr.lister.ListRelations(ctx, identityQ), func(rel *entity.Relation) {
		if !rr.classes.IsContentScoped(rel.Type) {
			out = append(out, rel)
		}
	}); err != nil {
		return nil, err
	}
	if err := collect(rr.lister.ListRelations(ctx, contentQ), func(rel *entity.Relation) {
		if rr.classes.IsContentScoped(rel.Type) {
			out = append(out, rel)
		}
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// NeighborsForPage is [RelationReader.Neighbors] for a whole page of
// resolved rows in ONE identity query plus one content query per distinct
// face on the page, instead of two queries per row (TKT-1U8XYN). The result
// is index-aligned with rows: rows[i]'s edges are out[i], in the same order
// Neighbors would return them (identity edges, then content edges, each in
// store order), and a row the world excludes keeps a nil entry.
//
// The per-row contract is preserved exactly: a content edge counts for a row
// only when the edge's tail face IS that row's face, so an edge between two
// page rows resolved at different faces lands on the row whose face it
// carries and not on the other — precisely what the per-row content query
// (`FromFace = prime`) did.
func (rr *RelationReader) NeighborsForPage(
	ctx context.Context, rows []Resolved, dir store.Direction,
) ([][]*entity.Relation, error) {
	out := make([][]*entity.Relation, len(rows))
	rowIdx := make(map[string]int, len(rows))
	ids := make([]string, 0, len(rows))
	byFace := make(map[entity.Face][]string)
	for i, res := range rows {
		if !res.Found || res.Entity == nil {
			continue
		}
		id := res.Entity.ID
		rowIdx[id] = i
		ids = append(ids, id)
		byFace[res.Face] = append(byFace[res.Face], id)
	}
	if len(ids) == 0 {
		return out, nil
	}
	// assign hands rel to the row(s) at its selected endpoint(s); face, when
	// non-nil, restricts that to rows resolved at exactly that face.
	assign := func(rel *entity.Relation, face *entity.Face) {
		give := func(id string) {
			i, ok := rowIdx[id]
			if !ok || (face != nil && rows[i].Face != *face) {
				return
			}
			out[i] = append(out[i], rel)
		}
		switch dir {
		case store.DirectionOutgoing:
			give(rel.From)
		case store.DirectionIncoming:
			give(rel.To)
		default:
			give(rel.From)
			if rel.To != rel.From {
				give(rel.To)
			}
		}
	}

	identityQ := store.RelationQuery{Direction: dir, FromFace: nil, EntityIDs: ids}
	if err := collect(rr.lister.ListRelations(ctx, identityQ), func(rel *entity.Relation) {
		if !rr.classes.IsContentScoped(rel.Type) {
			assign(rel, nil)
		}
	}); err != nil {
		return nil, err
	}
	faces := make([]entity.Face, 0, len(byFace))
	for f := range byFace {
		faces = append(faces, f)
	}
	slices.Sort(faces)
	for _, f := range faces {
		face := f
		contentQ := store.RelationQuery{Direction: dir, FromFace: &face, EntityIDs: byFace[f]}
		if err := collect(rr.lister.ListRelations(ctx, contentQ), func(rel *entity.Relation) {
			if rr.classes.IsContentScoped(rel.Type) {
				assign(rel, &face)
			}
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// collect drains a relation iterator, aborting on the first error.
func collect(seq iter.Seq2[*entity.Relation, error], keep func(*entity.Relation)) error {
	for rel, err := range seq {
		if err != nil {
			return err
		}
		keep(rel)
	}
	return nil
}

// setEndpoint points the query at id on the side the direction implies.
//
// DirectionBoth uses EntityID (match either endpoint), NOT From: setting
// From alone would silently narrow "both" to outgoing-only, and
// DirectionBoth is the ZERO value, so that mistake would be the default.
func setEndpoint(q *store.RelationQuery, id string, dir store.Direction) {
	switch dir {
	case store.DirectionOutgoing:
		q.From = id
	case store.DirectionIncoming:
		q.To = id
	case store.DirectionBoth:
		q.EntityID = id
	}
}
