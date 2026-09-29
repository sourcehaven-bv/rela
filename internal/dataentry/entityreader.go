package dataentry

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// entityReader is the ungated entity/relation read seam over the store.
// Extracted from App (TKT-N26KLB): a single-dependency leaf shared by the read
// handlers, the affordance service, the relation handlers, and (eventually) the
// write handlers. It is deliberately UNGATED — ACL scoping is applied elsewhere
// (visibleReader for the gated single-GET / include-filter path; the analyze
// gate at the issue boundary). These helpers are the raw store reads those
// gated paths and the internal machinery build on.
//
// # It is DEFAULT-WORLD-ONLY, deliberately (TKT-DN37J2)
//
// Every read here goes to the store unresolved, so it returns each entity's
// DEFAULT state — the draft face, under the design doc's example layout.
// That is a decision, not an oversight, and it is safe because of two things
// together:
//
//   - The routes this reader serves (relations, attachments, export,
//     sub-resources, views) are REFUSED a non-default world by
//     worldCapablePath, so no `?world=` request reaches those calls.
//   - The two world-CAPABLE handlers (the collection list and the
//     single-entity GET) call this reader only on their DEFAULT-world branch.
//     Since TKT-WRLDAPI item 4 they have a world branch that goes through
//     worldNeighbors instead, which resolves each link through the request's
//     world (RULING 12).
//
// If a route is ever added to that allowlist, its use of this reader must be
// converted first — a world-bound response assembled partly from
// world-resolved rows and partly from these is the mixed-face bug that would
// be hardest to see, because the entity would look right and its neighbors
// would not. TestWorldCapableRoutesDoNotUseUngatedReader is the guard.
type entityReader struct {
	store store.Store
}

// defaultWorldRows reads, in one batch, the raw row the default world selects
// for each of ids, keyed by id: its default face. An id with no such row is
// absent, and so is every id when the read fails (logged).
//
// Only a route this reader may serve calls it, so the request is in the
// default world (see the type comment). The rows are raw: the caller gates
// the ids before asking and redacts a row before serving any of it.
func (er entityReader) defaultWorldRows(ctx context.Context, ids []string) map[string]*entity.Entity {
	rows, err := loadDefaultFaceRows(ctx, er.store, ids)
	if err != nil {
		slog.Warn("dataentry: entityReader: loading neighbor rows failed", "ids", len(ids), "err", err)
	}
	return rows
}

// defaultWorldHeaders is [entityReader.defaultWorldRows] content-free: for a
// caller that needs a neighbor's properties, such as its title, and never its
// body.
func (er entityReader) defaultWorldHeaders(ctx context.Context, ids []string) map[string]*entity.Entity {
	rows, err := loadDefaultFaceHeaders(ctx, er.store, ids)
	if err != nil {
		slog.Warn("dataentry: entityReader: loading neighbor headers failed", "ids", len(ids), "err", err)
	}
	return rows
}

// writePrepRow reads the raw row ref names, with no gate and no redaction.
//
// It is for write-prep, liveness and relation-source policy evaluation, never
// for a response: a version token or a splice base must hash the stored
// properties, the history routes must tell a live entity from a deleted one
// whether or not the caller may read it, and an affordance gate evaluates the
// relation's true source row. A read that serves a row goes through
// [visibleReader].
func (er entityReader) writePrepRow(ctx context.Context, ref entity.Ref) (*entity.Entity, bool) {
	e, err := er.readWritePrep(ctx, ref)
	if err != nil {
		return nil, false
	}
	return e, true
}

// readWritePrep is [entityReader.writePrepRow] that returns the read error,
// for a write path that must tell a transient fault from a missing row.
func (er entityReader) readWritePrep(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	return er.store.GetEntity(ctx, entity.Ref{ID: ref.ID, Face: ref.Face})
}

// writePrepFamily reads the raw row of every face of id, in face order, with
// no gate and no redaction; empty when id has no stored row. Like
// [entityReader.writePrepRow] it is for policy evaluation, never a response:
// an affordance gate whose relation source has no row at the edge's tail
// judges the write against the whole family.
func (er entityReader) writePrepFamily(ctx context.Context, id string) ([]*entity.Entity, error) {
	families, err := loadStoredFamilies(ctx, er.store, []string{id})
	if err != nil {
		return nil, err
	}
	faces := families[id].faces
	keys := make([]entity.Ref, len(faces))
	for i, f := range faces {
		keys[i] = entity.Ref{ID: id, Face: f}
	}
	rows, err := loadRows(ctx, er.store, keys)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.Entity, 0, len(rows))
	for _, k := range keys {
		if e, ok := rows[k]; ok {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int { return strings.Compare(string(a.Face), string(b.Face)) })
	return out, nil
}

// entityType returns the type of the entity with the given ID, or empty
// string if it can't be resolved. The relation GET handlers call it on a
// relation endpoint's ID to emit a `type` field per edge, so SPA clients can
// construct JSON:API §9 resource identifiers without guessing. The caller has
// already gated the id (visibleRelationIDs); see [storedTypeOf].
func (er entityReader) entityType(ctx context.Context, id string) string {
	return storedTypeOf(ctx, er.store, id)
}

// outgoingRelations returns all outgoing relations for id.
//
// A store error truncates the slice to what was read before the failure. The
// callers are response-serialization paths where a partial relations map is
// degraded-but-usable and a hard failure would 500 the whole entity; we log
// the error (so it isn't invisible) rather than propagate it. TODO(TKT-N26KLB):
// the relations map silently dropping edges on a store error is a latent
// correctness gap inherited from App — revisit whether these paths should
// surface a partial-result warning.
func (er entityReader) outgoingRelations(ctx context.Context, id string) []*entity.Relation {
	return er.relations(ctx, id, store.DirectionOutgoing)
}

// outgoingRelationsOnFace returns the outgoing relations tailed at the face
// the address names — the general form of [entityReader.outgoingRelations],
// whose unfiltered query returns the union of every face's edges.
//
// That union is wrong for a faced source: a `scope: content` edge belongs to
// ONE face, so serving the union presents another face's links as this one's
// (BUG-VFHUWO). The zero face selects default-tail edges only, which is what a
// caller addressing a bare id means, so a faceless project is unaffected.
//
// Identity-scoped edges are stored at the zero tail, so a faced address does
// NOT see them here. Callers that need both tails ask twice — this method
// answers exactly what the address names.
func (er entityReader) outgoingRelationsOnFace(ctx context.Context, ref entity.Ref) []*entity.Relation {
	face := ref.Face
	rels, err := listRelationsCtx(ctx, er.store, store.RelationQuery{
		EntityID: ref.ID, Direction: store.DirectionOutgoing, FromFace: &face,
	})
	if err != nil {
		slog.Warn("dataentry: entityReader: listing outgoing relations failed; result truncated",
			"entity", ref.ID, "face", string(ref.Face), "err", err)
	}
	return rels
}

// incomingRelations returns all incoming relations for id. Same error handling
// as outgoingRelations.
func (er entityReader) incomingRelations(ctx context.Context, id string) []*entity.Relation {
	return er.relations(ctx, id, store.DirectionIncoming)
}

// pageRelations loads every edge touching any of entities in ONE query and
// splits them per row: outgoing[i] holds the edges whose source is
// entities[i], incoming[i] those whose target is. Index-aligned with
// entities; a row with no edges keeps a nil entry. An edge between two page
// rows appears in both rows' slices, once each — the same result the former
// per-row outgoing+incoming pair produced (TKT-1U8XYN).
func (er entityReader) pageRelations(
	ctx context.Context, entities []*entity.Entity,
) (outgoing, incoming [][]*entity.Relation) {
	outgoing = make([][]*entity.Relation, len(entities))
	incoming = make([][]*entity.Relation, len(entities))
	if len(entities) == 0 {
		return outgoing, incoming
	}
	rowIdx := make(map[string]int, len(entities))
	ids := make([]string, 0, len(entities))
	for i, e := range entities {
		if _, dup := rowIdx[e.ID]; dup {
			continue
		}
		rowIdx[e.ID] = i
		ids = append(ids, e.ID)
	}
	rels, err := listRelationsCtx(ctx, er.store, store.RelationQuery{EntityIDs: ids, Direction: store.DirectionBoth})
	if err != nil {
		slog.Warn("dataentry: entityReader: listing page relations failed; result truncated",
			"rows", len(ids), "err", err)
	}
	for _, r := range rels {
		if i, ok := rowIdx[r.From]; ok {
			outgoing[i] = append(outgoing[i], r)
		}
		if i, ok := rowIdx[r.To]; ok {
			incoming[i] = append(incoming[i], r)
		}
	}
	return outgoing, incoming
}

func (er entityReader) relations(ctx context.Context, id string, dir store.Direction) []*entity.Relation {
	rels, err := listRelationsCtx(ctx, er.store, store.RelationQuery{EntityID: id, Direction: dir})
	if err != nil {
		slog.Warn("dataentry: entityReader: listing relations failed; result truncated",
			"entity", id, "direction", dir, "err", err)
	}
	return rels
}
