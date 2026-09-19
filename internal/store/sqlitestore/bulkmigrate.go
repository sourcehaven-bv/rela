package sqlitestore

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SwapRelationEndpoints implements [store.BulkMigrator] (TKT-HH7PKJ).
//
// One UPDATE rewrites the whole type. The rows keep their rel_record_id, so
// relation history stays one continuous lifetime across the reversal — a
// create-then-delete loop above the store would mint a fresh lineage per edge.
//
// Endpoints are part of the primary key, so a swap that would land two edges
// on one triple fails the constraint and the statement applies nothing: SQLite
// evaluates the whole UPDATE as one statement, so a both-directions pair is
// refused rather than half-applied.
//
// Self-edges are excluded from both the count and the rewrite: the exchange is
// a no-op for them, and counting them would diverge from the generic fallback,
// where they are skipped.
func (s *Store) SwapRelationEndpoints(ctx context.Context, relType string) (int, error) {
	swapped, err := s.relationsToSwap(ctx, relType)
	if err != nil {
		return 0, err
	}
	if len(swapped) == 0 {
		return 0, nil
	}

	res, err := s.write(ctx,
		`UPDATE relations SET from_id = to_id, to_id = from_id
		  WHERE rel_type = ? AND from_id <> to_id`, relType)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, fmt.Errorf(
				"%w: reversing %q would land two edges on one triple — the type holds a pair "+
					"pointing both ways, and swapping would merge two distinct edges",
				store.ErrConflict, relType)
		}
		return 0, fmt.Errorf("sqlitestore: swap relation endpoints: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlitestore: swap relation endpoints: %w", err)
	}

	for _, r := range swapped {
		s.emit(store.Event{
			Op: store.EventRelationUpdated, RelationType: r.Type, From: r.To, To: r.From,
		})
	}
	return int(n), nil
}

// relationsToSwap reads the triples the swap will rewrite, so the events can
// name them after the rows have moved.
func (s *Store) relationsToSwap(ctx context.Context, relType string) ([]*entity.Relation, error) {
	var out []*entity.Relation
	for r, err := range s.ListRelations(ctx, store.RelationQuery{Type: relType}) {
		if err != nil {
			return nil, err
		}
		if r.From == r.To {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
