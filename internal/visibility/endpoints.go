package visibility

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// EndpointsReadable reports, for each relation in rels, whether the ctx
// principal may read both of its endpoints. The result is aligned with rels;
// a nil relation is unreadable.
//
// The two ends are gated differently (RR-2IK76Z):
//
//   - The HEAD (To) is entity level. It is readable when the principal may
//     read some stored face of it: the [Resolver.Family] question.
//   - The TAIL (From) of a content-scoped edge attaches to one face,
//     Relation.FromFace. That face must exist and be readable: the
//     [Resolver.Ref] question. A tail with no face is entity level, like the
//     head.
//
// It reads headers only, in ONE store query for every endpoint of every
// relation, then runs one PermitsReadMany and one face-set lookup per
// distinct endpoint type. The cost is therefore independent of len(rels)
// (RR-S4S8ZG). There is no claimed type: each endpoint is gated on the type
// it is stored under.
//
// Fails closed: a failed header read hides every relation, and a gate error
// hides every endpoint of that type. Both are logged.
func (r *Resolver) EndpointsReadable(ctx context.Context, rels []*entity.Relation) []bool {
	out := make([]bool, len(rels))
	ids := endpointIDs(rels)
	if len(ids) == 0 {
		return out
	}
	stored, ok := r.storedFaces(ctx, ids)
	if !ok {
		return out
	}
	readable := r.readableFaces(ctx, stored)
	for i, rel := range rels {
		if rel == nil {
			continue
		}
		head := len(readable[rel.To]) > 0
		var tail bool
		if rel.FromFace.IsDefault() {
			tail = len(readable[rel.From]) > 0
		} else {
			tail = readable[rel.From][rel.FromFace]
		}
		out[i] = head && tail
	}
	return out
}

// endpointIDs returns the distinct endpoint ids of rels, in first-seen order.
func endpointIDs(rels []*entity.Relation) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, rel := range rels {
		if rel == nil {
			continue // fail-closed: a nil relation must not panic the filter
		}
		for _, id := range [2]string{rel.From, rel.To} {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// endpointFaces is what the header read found for one endpoint id.
type endpointFaces struct {
	typ   string
	faces []entity.Face
}

// storedFaces reads the type and every stored face of ids, in one header
// query. ok is false when the read failed.
func (r *Resolver) storedFaces(ctx context.Context, ids []string) (map[string]*endpointFaces, bool) {
	stored := make(map[string]*endpointFaces, len(ids))
	q := store.EntityQuery{IDs: ids, AllStates: true}
	for h, err := range store.ListEntityHeaders(ctx, r.load, q) {
		if err != nil {
			slog.Warn("visibility: relation endpoint read failed; hiding the relations",
				"endpoints", len(ids), "err", err)
			return nil, false
		}
		ef := stored[h.ID]
		if ef == nil {
			ef = &endpointFaces{typ: h.Type}
			stored[h.ID] = ef
		}
		if h.Type != ef.typ {
			// One id stored under two types is corrupt data. Neither claim
			// can be trusted, so the endpoint is hidden.
			ef.faces = nil
			ef.typ = ""
			continue
		}
		ef.faces = append(ef.faces, h.Face)
	}
	return stored, true
}

// readableFaces returns, per endpoint id, the set of its stored faces the
// principal may read. An id missing from the result is unreadable.
func (r *Resolver) readableFaces(
	ctx context.Context, stored map[string]*endpointFaces,
) map[string]map[entity.Face]bool {
	byType := make(map[string][]string)
	for id, ef := range stored {
		if ef.typ != "" {
			byType[ef.typ] = append(byType[ef.typ], id)
		}
	}
	out := make(map[string]map[entity.Face]bool, len(stored))
	for typ, ids := range byType {
		perm, err := r.gate.PermitsReadMany(ctx, typ, ids)
		if err != nil {
			slog.Warn("visibility: PermitsReadMany failed; hiding the type's endpoints fail-closed",
				"type", typ, "candidates", len(ids), "err", err)
			continue
		}
		faces, err := ReadableFaces(ctx, r.gate, typ)
		if err != nil {
			slog.Warn("visibility: readable faces failed; hiding the type's endpoints fail-closed",
				"type", typ, "err", err)
			continue
		}
		for _, id := range ids {
			if !perm[id] {
				continue
			}
			for _, f := range stored[id].faces {
				if faces.Contains(f) {
					if out[id] == nil {
						out[id] = make(map[entity.Face]bool)
					}
					out[id][f] = true
				}
			}
		}
	}
	return out
}
