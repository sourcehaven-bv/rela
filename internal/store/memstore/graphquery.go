package memstore

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// GraphQuery delegates to the shared naive implementation; memstore
// has no index advantage over the generic algorithm.
func (m *MemStore) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return graphquerynaive.Run(ctx, m, q)
}

// GraphCount delegates to the shared naive implementation.
func (m *MemStore) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	return graphquerynaive.Count(ctx, m, q)
}

// MatchingIDs delegates to the shared naive implementation.
func (m *MemStore) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	return graphquerynaive.MatchingIDs(ctx, m, q, ids)
}
