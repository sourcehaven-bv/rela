package visibility

import (
	"context"

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
	ids := endpointIDs(rels)
	if len(ids) == 0 {
		return make([]bool, len(rels))
	}
	readable, ok := r.readableHeaders(ctx, ids)
	if !ok {
		return make([]bool, len(rels))
	}
	return endpointVerdicts(rels, readable)
}

// EndpointsReadableErr is [Resolver.EndpointsReadable] for a caller that must
// not act on a partial answer: a failed header read or a gate error is
// returned, and the result is nil. That is safe where the caller answers
// every fault as a failed request, never per relation, as
// [Resolver.ReadableTypes] explains.
func (r *Resolver) EndpointsReadableErr(ctx context.Context, rels []*entity.Relation) ([]bool, error) {
	ids := endpointIDs(rels)
	if len(ids) == 0 {
		return make([]bool, len(rels)), nil
	}
	readable, err := r.scanHeaders(ctx, ids, func(_ string, err error) error { return err })
	if err != nil {
		return nil, err
	}
	return endpointVerdicts(rels, readable), nil
}

// endpointVerdicts applies the head and tail rules of
// [Resolver.EndpointsReadable] to the readable faces of every endpoint.
func endpointVerdicts(rels []*entity.Relation, readable map[string]map[entity.Face]store.EntityHeader) []bool {
	out := make([]bool, len(rels))
	for i, rel := range rels {
		if rel == nil {
			continue
		}
		head := len(readable[rel.To]) > 0
		var tail bool
		if rel.FromFace.IsDefault() {
			tail = len(readable[rel.From]) > 0
		} else {
			_, tail = readable[rel.From][rel.FromFace]
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
