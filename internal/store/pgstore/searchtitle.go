package pgstore

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// searchTitleHealInterval is how often the heal pass looks for rows whose
// search_title is out of date. A caught-up table costs two index probes.
const searchTitleHealInterval = 5 * time.Minute

// searchTitleHealBatch bounds the rows one heal statement rewrites, so a first
// pass over a large table holds no long transaction. Each rewrite also updates
// the row's index entries, which is what makes a row cost about a millisecond.
const searchTitleHealBatch = 500

// searchTitleAdvisoryLockKey ("RELT") lets one process per schema run a heal
// pass at a time; the others skip it. Schema-scoped like the sweep's lock (see
// tryAdvisoryLock).
const searchTitleAdvisoryLockKey int64 = 0x52_45_4c_54

// SyncSearchTitles records titles as the title property of each type, which
// the database uses to compute entities.search_title (see
// 0047_search_title.sql). A type whose property changed, appeared or
// disappeared is queued for the heal pass, which recomputes its rows; rows
// written from now on get the new value from the trigger.
//
// Open calls it with the map given to [WithSearchTitles]. Two processes with
// different maps on one schema take turns: each open records its own map and
// requeues the types that differ, so ranking follows the last one to open.
func SyncSearchTitles(ctx context.Context, db DBTX, titles SearchTitles) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize concurrent opens, so two of them cannot both read the old map
	// and each requeue only half of the difference.
	if _, err = tx.Exec(ctx, `LOCK TABLE search_title_props IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	current := map[string]string{}
	rows, err := tx.Query(ctx, `SELECT type, prop FROM search_title_props`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var typ, prop string
		if err := rows.Scan(&typ, &prop); err != nil {
			rows.Close()
			return err
		}
		current[typ] = prop
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var changed []string
	for typ, prop := range titles {
		if current[typ] != prop {
			changed = append(changed, typ)
		}
	}
	for typ := range current {
		if _, ok := titles[typ]; !ok {
			changed = append(changed, typ)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	slices.Sort(changed)
	// Not nil for an empty map: pgx sends a nil slice as NULL, and
	// "type = ANY(NULL)" would keep every row the DELETE should remove.
	types := make([]string, 0, len(titles))
	types = append(types, slices.Sorted(maps.Keys(titles))...)
	props := make([]string, len(types))
	for i, typ := range types {
		props[i] = titles[typ]
	}
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM search_title_props WHERE NOT (type = ANY($1::text[]))`, []any{types}},
		{`INSERT INTO search_title_props (type, prop) SELECT * FROM unnest($1::text[], $2::text[])
		  ON CONFLICT (type) DO UPDATE SET prop = EXCLUDED.prop`, []any{types, props}},
		// Bumping gen stops a heal pass that is already working on the type
		// from taking it off the queue (see healSearchTitles).
		{`INSERT INTO search_title_pending (type) SELECT unnest($1::text[])
		  ON CONFLICT (type) DO UPDATE SET gen = search_title_pending.gen + 1`, []any{changed}},
	} {
		if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// searchTitleSQL is the value search_title must hold for the row aliased e.
// It is the trigger's expression in 0047_search_title.sql, with the title map
// read through the store's search_path; TestSearchTitle_HealAgreesWithTrigger
// holds the two together.
const searchTitleSQL = `lower(coalesce(nullif(e.properties ->> ` +
	`(SELECT p.prop FROM search_title_props p WHERE p.type = e.type), ''), e.id))`

// healSearchTitles recomputes search_title where it may be out of date: rows
// of the types SyncSearchTitles queued, then either every row (full) or only
// rows still holding the migration's empty default. It rewrites only rows
// whose value differs, in batches, and takes a type off the queue once its
// rows are done, so a pass that is interrupted resumes where the next one
// starts.
//
// The full check catches rows written while the title map was missing or
// different, which no queue records: a restore loads entities before
// search_title_props, for one. It reads every row, so the healer runs it
// once per open.
func healSearchTitles(ctx context.Context, db DBTX, full bool) error {
	type queued struct {
		typ string
		gen int64
	}
	var pending []queued
	rows, err := db.Query(ctx, `SELECT type, gen FROM search_title_pending ORDER BY type`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var q queued
		if err := rows.Scan(&q.typ, &q.gen); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, q := range pending {
		if err := healBatches(ctx, db, `type = $1`, q.typ); err != nil {
			return err
		}
		// A sync that requeued the type since we read it bumped gen; leave
		// the entry for the next pass, which sees the new map.
		if _, err := db.Exec(ctx, `DELETE FROM search_title_pending WHERE type = $1 AND gen = $2`,
			q.typ, q.gen); err != nil {
			return err
		}
	}
	if full {
		return healBatches(ctx, db, `true`)
	}
	return healBatches(ctx, db, `search_title = ''`)
}

// healBatches rewrites, in (id, face) order, the rows matching where whose
// search_title differs from searchTitleSQL. The keyset makes one pass read
// each candidate row once. where may use $1 for arg.
func healBatches(ctx context.Context, db DBTX, where string, arg ...any) error {
	n := len(arg)
	afterID, afterFace := "", ""
	for {
		args := append(slices.Clone(arg), afterID, afterFace, searchTitleHealBatch)
		var lastID, lastFace *string
		var count int
		err := db.QueryRow(ctx, `
			WITH batch AS (
				SELECT id, face FROM entities
				WHERE `+where+` AND (id, face) > ($`+strconv.Itoa(n+1)+`, $`+strconv.Itoa(n+2)+`)
				ORDER BY id, face LIMIT $`+strconv.Itoa(n+3)+`
			), healed AS (
				UPDATE entities e SET search_title = `+searchTitleSQL+`
				FROM batch b
				WHERE e.id = b.id AND e.face = b.face
				  AND e.search_title IS DISTINCT FROM `+searchTitleSQL+`
			)
			SELECT (array_agg(id ORDER BY id DESC, face DESC))[1],
			       (array_agg(face ORDER BY id DESC, face DESC))[1], count(*) FROM batch`,
			args...).Scan(&lastID, &lastFace, &count)
		if err != nil {
			return err
		}
		if count < searchTitleHealBatch || lastID == nil || lastFace == nil {
			return nil
		}
		afterID, afterFace = *lastID, *lastFace
	}
}

// searchTitleHealer runs healSearchTitles at open and on a timer until
// stopped. Started by Open, stopped by Close, like the mention reindexer.
type searchTitleHealer struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// startSearchTitleHealer detaches from openCtx's cancellation: the loop
// outlives Open and ends only at stop. The first pass is the full one.
func startSearchTitleHealer(openCtx context.Context, db DBTX) *searchTitleHealer {
	ctx, cancel := context.WithCancel(context.WithoutCancel(openCtx))
	h := &searchTitleHealer{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(h.done)
		t := time.NewTicker(searchTitleHealInterval)
		defer t.Stop()
		full := true
		for {
			ran, err := healSearchTitlesLocked(ctx, db, full)
			if err != nil && ctx.Err() == nil {
				slog.Warn("pgstore: search title heal failed; retrying on the next tick", "error", err)
			}
			if ran && err == nil {
				full = false
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return h
}

// healSearchTitlesLocked runs one pass on a connection holding the schema's
// heal lock, and reports false without running when another process holds
// it. A db that is not a pool (a store inside Tx) runs the pass unguarded.
func healSearchTitlesLocked(ctx context.Context, db DBTX, full bool) (bool, error) {
	pool, ok := db.(*pgxpool.Pool)
	if !ok {
		return true, healSearchTitles(ctx, db, full)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	locked, err := tryAdvisoryLock(ctx, conn, searchTitleAdvisoryLockKey)
	if err != nil || !locked {
		return false, err
	}
	defer func() {
		// Detached, so the lock is released during shutdown too; a closing
		// connection releases it with itself.
		_, _ = conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1::int, hashtext(current_schema()))`, searchTitleAdvisoryLockKey)
	}()
	return true, healSearchTitles(ctx, conn, full)
}

// stop cancels the loop and waits for it to exit. Nil-safe and idempotent.
func (h *searchTitleHealer) stop() {
	if h == nil {
		return
	}
	h.cancel()
	<-h.done
}
