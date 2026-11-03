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
// It issues ONE store query: every edge touching the entity in dir, at
// every tail (a NIL [store.RelationQuery.FromFace]). That query is a
// superset of each scope class's answer, so the dispatch is applied to the
// rows rather than to the query, per relation TYPE (see place):
//
//   - IDENTITY-scoped edges are kept at every tail. An identity edge
//     belongs to the entity, not to a face of it, so it must be visible
//     from every face; filtering by the prime's face would hide an
//     entity's role and containment edges whenever its prime is not the
//     default state.
//   - CONTENT-scoped edges the entity is the SOURCE of are kept only when
//     their tail IS the prime's face, so a non-prime state's content edges
//     stay invisible.
//   - CONTENT-scoped edges the entity is the TARGET of are kept at every
//     tail. The tail of such an edge is a face of the OTHER entity, so
//     filtering it by this entity's face selects an unrelated set
//     (BUG-ISJHML): it hid every edge from a faced source to a faceless
//     target, and let an edge through when the two faces merely shared a
//     name.
//
// The result lists identity edges, then outgoing content edges, then
// incoming content edges, each in store order.
//
// # Incoming content edges need the caller's Owns check
//
// Which face of the source a world serves is a property of the source,
// which this reader does not resolve. So an incoming content edge is
// returned at every tail, and the caller MUST keep it only when
// [RelationReader.Owns] holds for the face the source resolves to.
// Serving it unchecked shows one face's content beside another face —
// for a reader granted only the other face, a disclosure.
//
// # The fallback trap
//
// When the prime came from `otherwise: default` (or from rule 1), its
// face IS the zero Face. Compared as a VALUE, the zero face does not mean
// "unfiltered": it means DEFAULT-TAIL-ONLY. That is correct for the
// outgoing content edges (the default face's own edges) and would be WRONG
// for the identity edges, which no face comparison may touch. The two look
// identical in a debugger — both are "the zero face" — which is why the
// query carries no tail at all and the comparison happens only on the
// content arm, pinned by a named test.
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

	// NIL tail, deliberately. See the fallback trap above.
	q := store.RelationQuery{Direction: dir, FromFace: nil}
	setEndpoint(&q, id, dir)

	var buckets [slotCount][]*entity.Relation
	if err := collect(rr.lister.ListRelations(ctx, q), func(rel *entity.Relation) {
		if sl := rr.place(rel, id, res.Face, dir); sl != slotNone {
			buckets[sl] = append(buckets[sl], rel)
		}
	}); err != nil {
		return nil, err
	}
	return slices.Concat(buckets[slotIdentity], buckets[slotOutgoing], buckets[slotIncoming]), nil
}

// slot is where one edge lands in one entity's [RelationReader.Neighbors]
// result.
type slot int

const (
	slotIdentity slot = iota
	slotOutgoing
	slotIncoming
	slotCount
	slotNone slot = -1
)

// place applies the scope dispatch [RelationReader.Neighbors] documents to
// one edge, for the entity id served at prime and read in dir.
//
// A content self-edge is the entity's own outgoing edge: it is held to the
// prime's face in every direction, and listed once.
func (rr *RelationReader) place(rel *entity.Relation, id string, prime entity.Face, dir store.Direction) slot {
	out := dir != store.DirectionIncoming && rel.From == id
	in := dir != store.DirectionOutgoing && rel.To == id
	switch {
	case !out && !in:
		return slotNone
	case !rr.classes.IsContentScoped(rel.Type):
		return slotIdentity
	case rel.From == id && rel.FromFace != prime:
		return slotNone
	case out:
		return slotOutgoing
	default:
		return slotIncoming
	}
}

// Owns reports whether rel may be served beside its SOURCE when the source
// is served at sourceFace: always for an identity-scoped edge, and for a
// content-scoped edge only when its tail is that face.
//
// Callers apply it to every incoming content edge [RelationReader.Neighbors]
// or [RelationReader.NeighborsForPage] return, with the face the source
// resolved to (BUG-ISJHML). It holds by construction for outgoing edges,
// which are already filtered to the prime's face.
func (rr *RelationReader) Owns(rel *entity.Relation, sourceFace entity.Face) bool {
	return !rr.classes.IsContentScoped(rel.Type) || rel.FromFace == sourceFace
}

// NeighborsForPage is [RelationReader.Neighbors] for a whole page of
// resolved rows in ONE query (every edge touching any row, nil tail) instead
// of one query per row (TKT-1U8XYN). The result is index-aligned with rows:
// rows[i]'s edges are out[i], in the same order Neighbors would return them,
// and a row the world excludes keeps a nil entry.
//
// The per-row contract is preserved exactly: each edge is placed for each
// row it touches by the same rule Neighbors applies, so an outgoing content
// edge counts for a row only when its tail IS that row's face, and an
// incoming content edge is returned at every tail for the caller to check
// with [RelationReader.Owns].
func (rr *RelationReader) NeighborsForPage(
	ctx context.Context, rows []Resolved, dir store.Direction,
) ([][]*entity.Relation, error) {
	// rowIdx maps an id to every row serving it: a page may hold two faces
	// of one entity, and each row gets its own edges.
	rowIdx := make(map[string][]int, len(rows))
	var ids []string
	for i, res := range rows {
		if !res.Found || res.Entity == nil {
			continue
		}
		id := res.Entity.ID
		if _, dup := rowIdx[id]; !dup {
			ids = append(ids, id)
		}
		rowIdx[id] = append(rowIdx[id], i)
	}
	out := make([][]*entity.Relation, len(rows))
	if len(ids) == 0 {
		return out, nil
	}
	buckets := make([][slotCount][]*entity.Relation, len(rows))
	give := func(rel *entity.Relation, id string) {
		for _, i := range rowIdx[id] {
			if sl := rr.place(rel, id, rows[i].Face, dir); sl != slotNone {
				buckets[i][sl] = append(buckets[i][sl], rel)
			}
		}
	}
	q := store.RelationQuery{Direction: dir, FromFace: nil, EntityIDs: ids}
	if err := collect(rr.lister.ListRelations(ctx, q), func(rel *entity.Relation) {
		give(rel, rel.From)
		if rel.To != rel.From {
			give(rel, rel.To)
		}
	}); err != nil {
		return nil, err
	}
	for i, b := range buckets {
		if len(b[slotIdentity])+len(b[slotOutgoing])+len(b[slotIncoming]) > 0 {
			out[i] = slices.Concat(b[slotIdentity], b[slotOutgoing], b[slotIncoming])
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
