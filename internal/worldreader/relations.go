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
//   - CONTENT-scoped types the entity is the SOURCE of query with the
//     PRIME'S face, so a non-prime state's content edges stay invisible.
//   - CONTENT-scoped types the entity is the TARGET of query with a NIL
//     tail. The tail of such an edge is a face of the OTHER entity, so
//     filtering it by this entity's face selects an unrelated set
//     (BUG-ISJHML): it hid every edge from a faced source to a faceless
//     target, and let an edge through when the two faces merely shared a
//     name.
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

	// Merge, keeping each edge under the query whose scope class matches
	// its type. The queries over-return: the nil-tail query also matches
	// content edges, and the face query also matches identity edges
	// stored with that tail. Classifying on the way out is what makes the
	// merge exact rather than a union with duplicates.
	var out []*entity.Relation
	if err := collect(rr.lister.ListRelations(ctx, identityQ), func(rel *entity.Relation) {
		if !rr.classes.IsContentScoped(rel.Type) {
			out = append(out, rel)
		}
	}); err != nil {
		return nil, err
	}

	// Outgoing content edges: the prime's face BY VALUE, including when
	// that value is the zero face.
	prime := res.Face
	if dir != store.DirectionIncoming {
		outQ := store.RelationQuery{Direction: store.DirectionOutgoing, From: id, FromFace: &prime}
		if err := collect(rr.lister.ListRelations(ctx, outQ), func(rel *entity.Relation) {
			if rr.classes.IsContentScoped(rel.Type) {
				out = append(out, rel)
			}
		}); err != nil {
			return nil, err
		}
	}

	// Incoming content edges: NIL tail, because the tail is the source's
	// face. A self-edge is the entity's own outgoing edge, so it is taken
	// from the query above when that ran, and held to the prime's face
	// otherwise.
	if dir != store.DirectionOutgoing {
		inQ := store.RelationQuery{Direction: store.DirectionIncoming, To: id, FromFace: nil}
		if err := collect(rr.lister.ListRelations(ctx, inQ), func(rel *entity.Relation) {
			if !rr.classes.IsContentScoped(rel.Type) {
				return
			}
			if rel.From == id && (dir == store.DirectionBoth || rel.FromFace != prime) {
				return
			}
			out = append(out, rel)
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
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
// resolved rows in ONE identity query, one outgoing content query per
// distinct face on the page and one incoming content query, instead of up
// to three queries per row (TKT-1U8XYN). The result is index-aligned with
// rows: rows[i]'s edges are out[i], in the same order Neighbors would return
// them (identity edges, then outgoing content edges, then incoming content
// edges, each in store order), and a row the world excludes keeps a nil
// entry.
//
// The per-row contract is preserved exactly: an outgoing content edge counts
// for a row only when the edge's tail face IS that row's face, and an
// incoming content edge is returned at every tail for the caller to check
// with [RelationReader.Owns], as Neighbors documents.
func (rr *RelationReader) NeighborsForPage(
	ctx context.Context, rows []Resolved, dir store.Direction,
) ([][]*entity.Relation, error) {
	pg := page{rows: rows, out: make([][]*entity.Relation, len(rows)), rowIdx: make(map[string]int, len(rows))}
	byFace := make(map[entity.Face][]string)
	for i, res := range rows {
		if !res.Found || res.Entity == nil {
			continue
		}
		id := res.Entity.ID
		pg.rowIdx[id] = i
		pg.ids = append(pg.ids, id)
		byFace[res.Face] = append(byFace[res.Face], id)
	}
	if len(pg.ids) == 0 {
		return pg.out, nil
	}
	if err := rr.pageIdentity(ctx, &pg, dir); err != nil {
		return nil, err
	}
	if dir != store.DirectionIncoming {
		if err := rr.pageOutgoingContent(ctx, &pg, byFace); err != nil {
			return nil, err
		}
	}
	if dir != store.DirectionOutgoing {
		if err := rr.pageIncomingContent(ctx, &pg, dir); err != nil {
			return nil, err
		}
	}
	return pg.out, nil
}

// page is the working state of one [RelationReader.NeighborsForPage] call.
type page struct {
	rows   []Resolved
	out    [][]*entity.Relation
	rowIdx map[string]int
	ids    []string
}

func (pg *page) give(id string, rel *entity.Relation) {
	if i, ok := pg.rowIdx[id]; ok {
		pg.out[i] = append(pg.out[i], rel)
	}
}

// pageIdentity reads the page's identity-scoped edges with a nil tail.
func (rr *RelationReader) pageIdentity(ctx context.Context, pg *page, dir store.Direction) error {
	identityQ := store.RelationQuery{Direction: dir, FromFace: nil, EntityIDs: pg.ids}
	return collect(rr.lister.ListRelations(ctx, identityQ), func(rel *entity.Relation) {
		if rr.classes.IsContentScoped(rel.Type) {
			return
		}
		switch dir {
		case store.DirectionOutgoing:
			pg.give(rel.From, rel)
		case store.DirectionIncoming:
			pg.give(rel.To, rel)
		default:
			pg.give(rel.From, rel)
			if rel.To != rel.From {
				pg.give(rel.To, rel)
			}
		}
	})
}

// pageOutgoingContent reads the outgoing content-scoped edges, one query per
// distinct face on the page, keeping an edge only for a row served at its
// tail face.
func (rr *RelationReader) pageOutgoingContent(
	ctx context.Context, pg *page, byFace map[entity.Face][]string,
) error {
	faces := make([]entity.Face, 0, len(byFace))
	for f := range byFace {
		faces = append(faces, f)
	}
	slices.Sort(faces)
	for _, f := range faces {
		face := f
		outQ := store.RelationQuery{Direction: store.DirectionOutgoing, FromFace: &face, EntityIDs: byFace[f]}
		if err := collect(rr.lister.ListRelations(ctx, outQ), func(rel *entity.Relation) {
			if i, ok := pg.rowIdx[rel.From]; ok && rr.classes.IsContentScoped(rel.Type) && pg.rows[i].Face == face {
				pg.out[i] = append(pg.out[i], rel)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

// pageIncomingContent reads the incoming content-scoped edges with a nil
// tail; see [RelationReader.Neighbors] for the self-edge rule.
func (rr *RelationReader) pageIncomingContent(ctx context.Context, pg *page, dir store.Direction) error {
	inQ := store.RelationQuery{Direction: store.DirectionIncoming, FromFace: nil, EntityIDs: pg.ids}
	return collect(rr.lister.ListRelations(ctx, inQ), func(rel *entity.Relation) {
		i, ok := pg.rowIdx[rel.To]
		if !ok || !rr.classes.IsContentScoped(rel.Type) {
			return
		}
		if rel.From == rel.To && (dir == store.DirectionBoth || rel.FromFace != pg.rows[i].Face) {
			return
		}
		pg.out[i] = append(pg.out[i], rel)
	})
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
