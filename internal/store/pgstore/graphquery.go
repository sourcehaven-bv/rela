package pgstore

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// GraphQuery delegates to the shared naive implementation. A
// SQL-pushdown variant (one CTE for endpoint expansion + one for
// entity-side inherit-through + WHERE EXISTS for the inbound match)
// is the natural follow-up; until then pgstore uses the same iterate-
// and-filter approach as fsstore and memstore.
func (s *Store) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return graphquerynaive.Run(ctx, s, q)
}

// GraphCount delegates to the shared naive implementation.
func (s *Store) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	return graphquerynaive.Count(ctx, s, q)
}
