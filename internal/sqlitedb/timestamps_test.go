package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestFormatTimeSortsInTimeOrder pins the property the sweep's SQL relies on:
// for any two instants, comparing their formatted strings gives the same
// answer as comparing the instants (BUG-HEIAVS).
func TestFormatTimeSortsInTimeOrder(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	amsterdam := time.FixedZone("CEST", 2*60*60)
	for _, tc := range []struct {
		name           string
		earlier, later time.Time
	}{
		// RFC3339Nano rendered these as "…05.1234Z" and "…05.123449999Z",
		// which sorted the earlier one last.
		{"trailing zeros", base.Add(123400000), base.Add(123449999)},
		// RFC3339Nano dropped the fraction entirely for a whole second.
		{"whole second", base, base.Add(1)},
		// A caller-supplied time in another zone must not sort by wall clock.
		{"other offset", base.In(amsterdam), base.Add(time.Second)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.earlier.Before(tc.later))
			require.Less(t, sqlitedb.FormatTime(tc.earlier), sqlitedb.FormatTime(tc.later))
		})
	}
}

// TestMigrateTimestamps pins the v7→v8 step: every stored timestamp is
// rewritten to the fixed-width format, the instant it names is unchanged, and
// running the step again changes nothing.
func TestMigrateTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v7.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	stored := map[string]string{
		"FEAT-1": "2026-01-02T03:04:05.1234Z",           // trimmed fraction
		"FEAT-2": "2026-01-02T03:04:05Z",                // whole second
		"FEAT-3": "2026-01-02T05:04:05.5+02:00",         // another offset
		"FEAT-4": "2026-01-02T03:04:05.000000001Z",      // already fixed-width
		"FEAT-5": "2026-01-02T03:04:05.123456789+00:00", // numeric UTC offset
	}
	const trimmed, fixed = "2026-01-02T03:04:05.1Z", "2026-01-02T03:04:05.100000000Z"
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := raw.Exec(q, args...)
		require.NoErrorf(t, err, "seed statement: %s", q)
	}
	for id, ts := range stored {
		exec(`INSERT INTO entities (id, type, updated_at) VALUES (?, 'feature', ?)`, id, ts)
	}
	// More rows than one page, so the step must page past the first batch.
	exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 6000)
		INSERT INTO entities (id, type, updated_at) SELECT 'BULK-' || i, 'feature', ? FROM n`, trimmed)
	exec(`INSERT INTO relations (from_id, rel_type, to_id, updated_at, rel_record_id)
		VALUES ('FEAT-1', 'depends-on', 'FEAT-2', ?, 1)`, trimmed)
	exec(`INSERT INTO attachments (entity_id, property, file_name, data, size, updated_at)
		VALUES ('FEAT-1', 'file', 'a.txt', x'00', 1, ?)`, trimmed)
	exec(`INSERT INTO schema_versions (hash, projection, captured_at) VALUES ('h', '{}', ?)`, trimmed)
	exec(`INSERT INTO entity_versions (entity_id, op, type, content_hash, schema_hash, created_at)
		VALUES ('FEAT-1', 'create', 'feature', 'c', 'h', ?)`, trimmed)
	exec(`INSERT INTO relation_versions (rel_record_id, op, from_id, rel_type, to_id, content_hash,
		schema_hash, created_at) VALUES (1, 'create', 'FEAT-1', 'depends-on', 'FEAT-2', 'c', 'h', ?)`, trimmed)

	check := func() {
		t.Helper()
		for id, before := range stored {
			var after string
			require.NoError(t, raw.QueryRow(`SELECT updated_at FROM entities WHERE id = ?`, id).Scan(&after))
			want, err := time.Parse(time.RFC3339Nano, before)
			require.NoError(t, err)
			require.Equal(t, sqlitedb.FormatTime(want), after, "entity %s", id)
		}
		var stale int
		require.NoError(t, raw.QueryRow(
			`SELECT count(*) FROM entities WHERE id LIKE 'BULK-%' AND updated_at <> ?`, fixed).Scan(&stale))
		require.Zero(t, stale, "rows past the first page were not rewritten")
		for _, q := range []string{
			`SELECT updated_at FROM relations`,
			`SELECT updated_at FROM attachments`,
			`SELECT captured_at FROM schema_versions`,
			`SELECT created_at FROM entity_versions`,
			`SELECT created_at FROM relation_versions`,
		} {
			var v string
			require.NoError(t, raw.QueryRow(q).Scan(&v))
			require.Equal(t, fixed, v, q)
		}
	}

	// The first open runs the step. Stamping the database back to v7 makes
	// the second open run it again over already-normalized values.
	for i := range 2 {
		exec(`PRAGMA user_version = 7`)
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
		check()
	}
}
