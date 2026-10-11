package sqlitestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// mentionTargetsJSON is the value of entities.mention_targets for content: a
// JSON array, never NULL, because NULL means "not indexed yet".
func mentionTargetsJSON(content string) string {
	targets := mentions.Index(content)
	if targets == nil {
		return "[]"
	}
	b, err := json.Marshal(targets)
	if err != nil { // coverage-ignore: marshaling a []string cannot fail
		return "[]"
	}
	return string(b)
}

// MentionSources implements [store.MentionIndex]. Rows written before the
// v17 rung are indexed first: sqlitestore is single-process, so a pending row
// can only predate this binary, and indexing them here costs one partial-index
// probe once they are done.
func (s *Store) MentionSources(
	ctx context.Context, target string, after entity.Ref, limit int,
) ([]entity.Ref, error) {
	if limit <= 0 {
		return nil, store.ErrInvalidQuery
	}
	if err := reindexMentions(ctx, s); err != nil {
		return nil, fmt.Errorf("sqlitestore: index mentions: %w", err)
	}
	rows, err := s.q().QueryContext(ctx, `
		SELECT m.source_id, m.source_face FROM entity_mention_targets m
		JOIN entities e ON e.id = m.source_id AND e.face = m.source_face
		WHERE m.target = ? AND (m.source_id, m.source_face) > (?, ?)
		ORDER BY m.source_id, m.source_face
		LIMIT ?`, target, after.ID, string(after.Face), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []entity.Ref
	for rows.Next() {
		var id, face string
		if err := rows.Scan(&id, &face); err != nil {
			return nil, err
		}
		out = append(out, entity.Ref{ID: id, Face: entity.Face(face)})
	}
	return out, rows.Err()
}

// mentionReindexBatch is the rows reindexMentions fills per transaction, so
// a large upgraded database does not hold the write lock, or every body in
// memory, for the whole backfill.
const mentionReindexBatch = 500

// reindexMentions fills mention_targets for every row that has none, a batch
// per write transaction so no store write interleaves within one.
func reindexMentions(ctx context.Context, s *Store) error {
	for {
		var pending bool
		err := s.q().QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM entities WHERE mention_targets IS NULL)`).Scan(&pending)
		if err != nil || !pending {
			return err
		}
		err = s.Tx(ctx, func(tx store.Store) error {
			return reindexMentionBatch(store.ContextInTx(ctx), tx)
		})
		if err != nil {
			return err
		}
	}
}

// reindexMentionBatch fills one batch inside the transaction view tx.
func reindexMentionBatch(ctx context.Context, tx store.Store) error {
	view, ok := tx.(*Store)
	if !ok { // unreachable: Tx always hands back our own view type
		return errors.New("sqlitestore: unexpected transaction view type")
	}
	batch, err := unindexedRows(ctx, view.q())
	if err != nil {
		return err
	}
	for _, r := range batch {
		if _, err := view.write(ctx, `UPDATE entities SET mention_targets = ? WHERE id = ? AND face = ?`,
			mentionTargetsJSON(r.content), r.id, r.face); err != nil {
			return err
		}
	}
	return nil
}

// unindexedRow is an entity row whose mention targets are not yet stored.
type unindexedRow struct {
	id, face, content string
}

// unindexedRows reads the next batch of rows reindexMentions must fill.
func unindexedRows(ctx context.Context, q querier) ([]unindexedRow, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, face, content FROM entities WHERE mention_targets IS NULL LIMIT ?`, mentionReindexBatch)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []unindexedRow
	for rows.Next() {
		var r unindexedRow
		if err := rows.Scan(&r.id, &r.face, &r.content); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
