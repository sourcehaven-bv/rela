package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SwapRelationEndpoints implements [store.BulkMigrator] (TKT-HH7PKJ).
//
// One UPDATE rewrites the whole type, the same set-based shape RenameEntity
// uses for its bulk re-key. The rows KEEP their rel_record_id, so relation
// history stays one continuous lifetime across the reversal — the property
// that a create-then-delete loop above the store cannot have, since a new
// triple mints a fresh lineage.
//
// Endpoints are part of the primary key, so a swap that would land two edges
// on one triple raises a unique violation and the transaction rolls back
// whole. That is the refusal the generic fallback has to pre-compute, and here
// it is the database's own constraint: a both-directions pair (A→B and B→A)
// cannot be reversed, because doing so would merge two distinct edges.
//
// A self-edge (from == to) is unaffected by the exchange and is not counted:
// the UPDATE would rewrite it to itself, which is a no-op that would otherwise
// inflate the reported row count and diverge from the fallback, where such an
// edge is skipped outright.
func (s *Store) SwapRelationEndpoints(ctx context.Context, relType string) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// The old triples are read first so the change feed can tombstone the
	// identities that cease to exist: to an id-keyed client a reversal removes
	// every old triple and adds its mirror, exactly as a rename does.
	oldTriples, err := scanRelations(ctx, tx,
		`SELECT from_id, from_face, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE rel_type = $1 AND from_id <> to_id
		 ORDER BY from_id, from_face, rel_type, to_id`, relType)
	if err != nil {
		return 0, err
	}
	if len(oldTriples) == 0 {
		return 0, nil
	}

	tag, err := tx.Exec(ctx,
		`UPDATE relations
		    SET from_id = to_id, to_id = from_id, seq = nextval('rela_seq')
		  WHERE rel_type = $1 AND from_id <> to_id`, relType)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf(
				"%w: reversing %q would land two edges on one triple — the type holds a pair "+
					"pointing both ways, and swapping would merge two distinct edges",
				store.ErrConflict, relType)
		}
		return 0, err
	}

	for _, r := range oldTriples {
		if err := s.writeRelationTombstone(ctx, tx, r.From, r.FromFace, r.Type, r.To); err != nil {
			return 0, err
		}
	}

	evs := make([]store.Event, 0, len(oldTriples))
	for _, r := range oldTriples {
		evs = append(evs, store.Event{
			Op: store.EventRelationUpdated, RelationType: r.Type, From: r.To, To: r.From,
		})
	}
	for _, ev := range evs {
		s.notify(ctx, tx, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.emitAll(evs)

	return int(tag.RowsAffected()), nil
}
