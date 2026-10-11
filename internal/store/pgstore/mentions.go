package pgstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// mentionReindexInterval is how often the reindex pass looks for rows an
// older binary wrote without indexing them (see 0024_mention_targets.sql).
// The pending set is served by a partial index, so a pass over a caught-up
// table is one index probe.
const mentionReindexInterval = 5 * time.Minute

// mentionReindexBatch bounds the rows one reindex query reads, so a first
// backfill over a large table holds no long transaction and keeps memory flat.
const mentionReindexBatch = 500

// contentHashSQL is the database side of [contentHash]. The two must agree
// byte for byte, or the reindex pass would never consider a row settled.
const contentHashSQL = `encode(sha256(convert_to(content, 'UTF8')), 'hex')`

// mentionColumns returns the values for entities.mention_targets and
// entities.mentions_hash. The targets are never nil: pgx writes a nil slice
// as NULL, which means "not indexed yet".
func mentionColumns(content string) (targets []string, hash string) {
	targets = mentions.Index(content)
	if targets == nil {
		targets = []string{}
	}
	return targets, contentHash(content)
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// MentionSources implements [store.MentionIndex]. Soft-deleted rows live in
// marked_entities, so reading entities is already the live set.
func (s *Store) MentionSources(
	ctx context.Context, target string, after entity.Ref, limit int,
) ([]entity.Ref, error) {
	if limit <= 0 {
		return nil, store.ErrInvalidQuery
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, face FROM entities
		WHERE mention_targets @> ARRAY[$1::text]
		  AND (id, face) > ($2, $3)
		ORDER BY id, face
		LIMIT $4`, target, after.ID, after.Face.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

// reindexMentions fills mention_targets for every row that has none, batch by
// batch. Each update is conditioned on the content still hashing to what was
// parsed, so a write that lands meanwhile is never overwritten with targets
// from the older body; that row is left to the writer, which indexes it
// itself. Several processes may run this at once: the condition makes their
// updates idempotent.
func reindexMentions(ctx context.Context, s *Store) error {
	for {
		rows, err := s.db.Query(ctx, `
			SELECT id, face, content, `+contentHashSQL+`
			FROM entities WHERE mention_targets IS NULL
			ORDER BY id, face LIMIT $1`, mentionReindexBatch)
		if err != nil {
			return err
		}
		type pending struct{ id, face, content, hash string }
		var batch []pending
		for rows.Next() {
			var p pending
			if err := rows.Scan(&p.id, &p.face, &p.content, &p.hash); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		progressed := false
		for _, p := range batch {
			targets, hash := mentionColumns(p.content)
			tag, err := s.db.Exec(ctx, `
				UPDATE entities SET mention_targets = $3, mentions_hash = $4
				WHERE id = $1 AND face = $2 AND mention_targets IS NULL
				  AND `+contentHashSQL+` = $5`,
				p.id, p.face, targets, hash, p.hash)
			if err != nil {
				return err
			}
			if tag.RowsAffected() > 0 {
				progressed = true
			}
		}
		// A short batch is the end of the queue. A batch with no progress means
		// every row in it changed under us; its writers indexed those rows, but
		// stop rather than spin in case one did not.
		if len(batch) < mentionReindexBatch || !progressed {
			return nil
		}
	}
}

// mentionReindexer runs reindexMentions at start and on a timer until
// stopped. Started by Open, stopped by Close, like the change-feed listener.
type mentionReindexer struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// startMentionReindexer detaches from openCtx's cancellation: the loop
// outlives Open and ends only at stop.
func startMentionReindexer(openCtx context.Context, s *Store) *mentionReindexer {
	ctx, cancel := context.WithCancel(context.WithoutCancel(openCtx))
	r := &mentionReindexer{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		t := time.NewTicker(mentionReindexInterval)
		defer t.Stop()
		for {
			if err := reindexMentions(ctx, s); err != nil && ctx.Err() == nil {
				slog.Warn("pgstore: mention reindex failed; retrying on the next tick", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return r
}

// stop cancels the loop and waits for it to exit. Nil-safe and idempotent.
func (r *mentionReindexer) stop() {
	if r == nil {
		return
	}
	r.cancel()
	<-r.done
}
