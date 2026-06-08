package fsstore

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// GraphQuery delegates to the shared naive implementation. An
// index-backed implementation that exploits fsstore's by-from /
// by-to relation indexes (once added) is a natural follow-up.
func (s *FSStore) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return graphquerynaive.Run(ctx, s, q)
}

// GraphCount currently delegates to the shared naive implementation.
func (s *FSStore) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	return graphquerynaive.Count(ctx, s, q)
}

// MatchingIDs delegates to the shared naive implementation.
func (s *FSStore) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	return graphquerynaive.MatchingIDs(ctx, s, q, ids)
}
