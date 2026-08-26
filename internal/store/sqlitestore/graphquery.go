package sqlitestore

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// GraphQuery, GraphCount and MatchingIDs delegate to the shared,
// backend-agnostic implementation, exactly as fsstore does.
//
// Delegating rather than writing SQL pushdown is a deliberate first step, not
// an oversight: the naive implementation is the behavioral reference every
// backend is verified against, so starting here guarantees this backend agrees
// with the others from day one. SQLite supports recursive CTEs, so pushdown is
// available as a later optimization — and when it lands it must reuse
// graphquerynaive.DepthCap so the two paths bound traversal identically.

func (s *Store) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return graphquerynaive.Run(ctx, s, q)
}

func (s *Store) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	return graphquerynaive.Count(ctx, s, q)
}

func (s *Store) MatchingIDs(
	ctx context.Context, q store.GraphQuery, ids []string,
) (map[string]bool, error) {
	return graphquerynaive.MatchingIDs(ctx, s, q, ids)
}
