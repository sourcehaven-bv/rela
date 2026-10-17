package sqlitestore

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// searchTextReindexBatch is the rows reindexSearchText fills per
// transaction, so a large upgraded database does not hold the write lock for
// the whole backfill.
const searchTextReindexBatch = 500

// reindexSearchText fills entities.search_text for every row that has none:
// the rows written before the v20 rung (BUG-YG126C). Until it runs, such a
// row is out of the search index. [New] runs it, so a store never serves a
// search over a half-filled index. Each batch is one write transaction.
func reindexSearchText(ctx context.Context, s *Store) error {
	for {
		var pending bool
		err := s.q().QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM entities WHERE search_text IS NULL)`).Scan(&pending)
		if err != nil || !pending {
			return err
		}
		err = s.Tx(ctx, func(tx store.Store) error {
			return reindexSearchTextBatch(store.ContextInTx(ctx), tx)
		})
		if err != nil {
			return err
		}
	}
}

// reindexSearchTextBatch fills one batch inside the transaction view tx.
func reindexSearchTextBatch(ctx context.Context, tx store.Store) error {
	view, ok := tx.(*Store)
	if !ok { // unreachable: Tx always hands back our own view type
		return errors.New("sqlitestore: unexpected transaction view type")
	}
	rows, err := view.q().QueryContext(ctx,
		`SELECT id, face, properties, content FROM entities WHERE search_text IS NULL LIMIT ?`,
		searchTextReindexBatch)
	if err != nil {
		return err
	}
	var batch []*entity.Entity
	for rows.Next() {
		var id, face, props, content string
		if err := rows.Scan(&id, &face, &props, &content); err != nil {
			_ = rows.Close()
			return err
		}
		p, err := unmarshalProps(props)
		if err != nil {
			_ = rows.Close()
			return err
		}
		batch = append(batch, &entity.Entity{ID: id, Face: entity.Face(face), Properties: p, Content: content})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, e := range batch {
		if _, err := view.write(ctx, `UPDATE entities SET search_text = ? WHERE id = ? AND face = ?`,
			search.IndexText(e), e.ID, string(e.Face)); err != nil {
			return err
		}
	}
	return nil
}
