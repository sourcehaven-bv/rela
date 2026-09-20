package store

import "context"

// EntityRef names one entity in a [Position].
type EntityRef struct {
	ID   string
	Type string
}

// Position is where one entity sits in the ordered result of a GraphQuery.
// Index is 1-based. Prev and Next are nil at the ends.
type Position struct {
	Index int
	Total int
	Prev  *EntityRef
	Next  *EntityRef
}

// PositionQueryer answers "where is id in q's ordered result, and who are its
// neighbors" without shipping the result (TKT-U9DYW4). OPTIONAL, type-asserted
// like [MatchedCounter], with [GraphPosition] as the generic fallback. It
// exists because scope navigation asks this once per detail page, and the
// fallback reads every matched row to answer with three ids.
//
// q.OrderBy decides the order exactly as it does for [GraphQueryer.GraphQuery];
// q.Limit and q.Offset are ignored. found is false when q does not match id.
type PositionQueryer interface {
	GraphPosition(ctx context.Context, q GraphQuery, id string) (pos Position, found bool, err error)
}

// GraphPosition answers a position question on any GraphQueryer, using the
// backend's [PositionQueryer] when it has one and a scan of
// [GraphQueryHeaders] otherwise.
func GraphPosition(ctx context.Context, gq GraphQueryer, q GraphQuery, id string) (Position, bool, error) {
	if pq, ok := gq.(PositionQueryer); ok {
		return pq.GraphPosition(ctx, q, id)
	}
	q.Limit, q.Offset = 0, 0
	var (
		pos   Position
		prev  *EntityRef
		found bool
	)
	for h, err := range GraphQueryHeaders(ctx, gq, q) {
		if err != nil {
			return Position{}, false, err
		}
		pos.Total++
		ref := &EntityRef{ID: h.ID, Type: h.Type}
		switch {
		case !found && h.ID == id:
			found = true
			pos.Index = pos.Total
			pos.Prev = prev
		case found && pos.Next == nil && pos.Total == pos.Index+1:
			pos.Next = ref
		}
		prev = ref
	}
	if !found {
		return Position{}, false, nil
	}
	return pos, true, nil
}
