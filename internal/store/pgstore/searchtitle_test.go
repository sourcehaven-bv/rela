package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

func searchTitleOf(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var title string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT search_title FROM entities WHERE id = $1 AND face = ''`, id).Scan(&title))
	return title
}

// Every write computes search_title from the recorded title map: create,
// update, rename and a type change (TKT-M58EQ1W).
func TestSearchTitle_WritesKeepItCurrent(t *testing.T) {
	pool := newScopedPool(t)
	st, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "title", "person": "name"}))

	e := entity.New("N-1", "note")
	e.SetString("title", "Quarterly Review")
	require.NoError(t, st.CreateEntity(ctx, e))
	require.Equal(t, "quarterly review", searchTitleOf(t, pool, "N-1"))

	e.SetString("title", "Annual Review")
	require.NoError(t, st.UpdateEntity(ctx, e))
	require.Equal(t, "annual review", searchTitleOf(t, pool, "N-1"))

	untitled := entity.New("X-Mixed", "misc")
	require.NoError(t, st.CreateEntity(ctx, untitled))
	require.Equal(t, "x-mixed", searchTitleOf(t, pool, "X-Mixed"), "a type without a title ranks by its id")

	_, err = st.RenameFamily(ctx, "X-Mixed", "X-Renamed")
	require.NoError(t, err)
	require.Equal(t, "x-renamed", searchTitleOf(t, pool, "X-Renamed"))

	converted := entity.New("X-Renamed", "person")
	converted.SetString("name", "Ada Lovelace")
	require.NoError(t, st.UpdateEntity(ctx, converted))
	require.Equal(t, "ada lovelace", searchTitleOf(t, pool, "X-Renamed"))

	// A writer whose search_path does not name the schema, such as an admin
	// session or a restore, still gets the schema's title map.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var schema string
	require.NoError(t, tx.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema))
	_, err = tx.Exec(ctx, `SET LOCAL search_path TO pg_catalog`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO `+pgx.Identifier{schema, "entities"}.Sanitize()+
		` (id, type, properties) VALUES ('N-2', 'note', '{"title": "Outside Writer"}')`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, "outside writer", searchTitleOf(t, pool, "N-2"))

	// An empty title ranks by the id, like a missing one.
	empty := entity.New("N-3", "note")
	empty.SetString("title", "")
	require.NoError(t, st.CreateEntity(ctx, empty))
	require.Equal(t, "n-3", searchTitleOf(t, pool, "N-3"))

	// A schema renamed after the migration keeps working (DDL rolls back).
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	renamed := schema + "_renamed"
	_, err = tx.Exec(ctx, `ALTER SCHEMA `+pgx.Identifier{schema}.Sanitize()+` RENAME TO `+pgx.Identifier{renamed}.Sanitize())
	require.NoError(t, err)
	var title string
	require.NoError(t, tx.QueryRow(ctx, `INSERT INTO `+pgx.Identifier{renamed, "entities"}.Sanitize()+
		` (id, type, properties) VALUES ('N-4', 'note', '{"title": "Renamed Schema"}') RETURNING search_title`).Scan(&title))
	require.Equal(t, "renamed schema", title)
}

// The heal pass and the trigger compute the same value: a full pass over rows
// the trigger wrote rewrites none of them. A row written while the map lacked
// its type, as a restore that loads entities before the map does, is fixed by
// the full pass although no queue names it.
func TestSearchTitle_HealAgreesWithTrigger(t *testing.T) {
	pool := newScopedPool(t)
	st, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "title"}))

	for i, title := range []string{"Mixed Case", "", "ÉCOLE Ünïcode"} {
		e := entity.New(fmt.Sprintf("N-%d", i), "note")
		e.SetString("title", title)
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	untitled := entity.New("Misc-1", "misc")
	require.NoError(t, st.CreateEntity(ctx, untitled))
	restored := entity.New("P-1", "person")
	restored.SetString("name", "Grace")
	require.NoError(t, st.CreateEntity(ctx, restored))
	_, err = pool.Exec(ctx, `INSERT INTO search_title_props (type, prop) VALUES ('person', 'name')`)
	require.NoError(t, err)
	require.Equal(t, "p-1", searchTitleOf(t, pool, "P-1"))

	xmins := func() map[string]string {
		out := map[string]string{}
		rows, err := pool.Query(ctx, `SELECT id, xmin::text FROM entities WHERE id <> 'P-1'`)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var id, xmin string
			require.NoError(t, rows.Scan(&id, &xmin))
			out[id] = xmin
		}
		require.NoError(t, rows.Err())
		return out
	}
	before := xmins()
	require.NoError(t, pgstore.HealSearchTitlesForTest(ctx, pool, true))
	require.Equal(t, before, xmins(), "the full pass rewrote a row the trigger wrote")
	require.Equal(t, "grace", searchTitleOf(t, pool, "P-1"))
}

// A sync that requeues a type bumps its generation, so a heal pass that read
// the queue before it cannot take the type off; an empty map clears the
// recorded one.
func TestSearchTitle_SyncQueue(t *testing.T) {
	pool := newScopedPool(t)
	ctx := context.Background()
	gen := func() int64 {
		var g int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT gen FROM search_title_pending WHERE type = 'note'`).Scan(&g))
		return g
	}
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "title"}))
	first := gen()
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "label"}))
	require.Greater(t, gen(), first)

	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{}))
	var props int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_title_props`).Scan(&props))
	require.Zero(t, props)
}

// countingDB counts the queries a store issues.
type countingDB struct {
	pgstore.DBTX
	queries int
}

func (c *countingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.queries++
	return c.DBTX.Query(ctx, sql, args...)
}

// A limited visible search whose first page fills the limit stops there: it
// does not read a second page it would throw away.
func TestSearchVisible_StopsAtLimit(t *testing.T) {
	pool := newScopedPool(t)
	ctx := context.Background()
	writer, err := pgstore.New(pool)
	require.NoError(t, err)
	for i := range 5 {
		e := entity.New(fmt.Sprintf("N-%d", i), "note")
		e.Content = "telemetry"
		require.NoError(t, writer.CreateEntity(ctx, e))
	}
	db := &countingDB{DBTX: pool}
	st, err := pgstore.New(db)
	require.NoError(t, err)
	scope := map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
	q := search.Query{Text: "telemetry", Limit: 2, World: store.TrivialScope()}
	var hits int
	for _, err := range st.SearchVisible(ctx, q, scope) {
		require.NoError(t, err)
		hits++
	}
	require.Equal(t, 2, hits)
	require.Equal(t, 1, db.queries)
}

// A changed title map queues the types it changes, and the heal pass
// recomputes their rows and the rows the migration left empty. Rows already
// current are not rewritten.
func TestSearchTitle_HealRecomputesChangedTypes(t *testing.T) {
	pool := newScopedPool(t)
	st, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "title"}))

	// More rows than one heal batch, so the keyset continues.
	const notes = 1203
	for i := range notes {
		e := entity.New(fmt.Sprintf("N-%05d", i), "note")
		e.SetString("title", "Title")
		e.SetString("label", fmt.Sprintf("Label %d", i))
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	p := entity.New("P-1", "person")
	p.SetString("name", "Grace")
	require.NoError(t, st.CreateEntity(ctx, p))
	// A row from before the migration.
	_, err = pool.Exec(ctx, `ALTER TABLE entities DISABLE TRIGGER entities_search_title`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE entities SET search_title = '' WHERE id = 'P-1'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE entities ENABLE TRIGGER entities_search_title`)
	require.NoError(t, err)

	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "label", "person": "name"}))
	require.Equal(t, "title", searchTitleOf(t, pool, "N-00000"), "rows are recomputed by the heal pass, not by the sync")

	require.NoError(t, pgstore.HealSearchTitlesForTest(ctx, pool, false))
	require.Equal(t, "label 0", searchTitleOf(t, pool, "N-00000"))
	require.Equal(t, fmt.Sprintf("label %d", notes-1), searchTitleOf(t, pool, fmt.Sprintf("N-%05d", notes-1)))
	require.Equal(t, "grace", searchTitleOf(t, pool, "P-1"))
	var pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_title_pending`).Scan(&pending))
	require.Zero(t, pending, "healed types leave the queue")

	// An unchanged map queues nothing.
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "label", "person": "name"}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_title_pending`).Scan(&pending))
	require.Zero(t, pending)

	// Removing a type from the map makes its rows rank by id.
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"note": "label"}))
	require.NoError(t, pgstore.HealSearchTitlesForTest(ctx, pool, false))
	require.Equal(t, "p-1", searchTitleOf(t, pool, "P-1"))
}
