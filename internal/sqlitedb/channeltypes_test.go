package sqlitedb_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// releasedChannelTypesDDL is channel_types as v18 shipped it (v26.10.11),
// before the cardinality column. v19 added channel_type_refs and left this
// table as it was.
const releasedChannelTypesDDL = `CREATE TABLE channel_types (
	type        TEXT PRIMARY KEY,
	anchor_mode TEXT NOT NULL
) STRICT`

// channelTypesShape renders channel_types' columns: name, type, not-null
// flag, default and primary-key position, in column order.
func channelTypesShape(t *testing.T, raw *sql.DB) []string {
	t.Helper()
	rows, err := raw.Query(`SELECT name, type, "notnull", coalesce(dflt_value, 'NULL'), pk
		FROM pragma_table_info('channel_types') ORDER BY cid`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name, typ, dflt string
		var notNull, pk int
		require.NoError(t, rows.Scan(&name, &typ, &notNull, &dflt, &pk))
		out = append(out, fmt.Sprintf("%s %s notnull=%d default=%s pk=%d", name, typ, notNull, dflt, pk))
	}
	require.NoError(t, rows.Err())
	return out
}

// openChannelDB opens and closes path through sqlitedb, so the schema and ladder
// run, and returns a plain handle on the file.
func openChannelDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

// TestMigrateChannelTypesCardinality pins the v19 to v20 rung: a v19
// channel_types table gains cardinality with the shape a fresh database
// has, an existing row reads as ” (recorded before cardinality was kept),
// and running the rung again is harmless.
func TestMigrateChannelTypesCardinality(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	want := channelTypesShape(t, fresh)
	require.Len(t, want, 3)

	path := filepath.Join(t.TempDir(), "v19.db")
	raw := openChannelDB(t, path)
	_, err := raw.Exec(`DROP TABLE channel_types`)
	require.NoError(t, err)
	_, err = raw.Exec(releasedChannelTypesDDL)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO channel_types (type, anchor_mode) VALUES ('discussion', 'family')`)
	require.NoError(t, err)

	// Stamping v19 before each open makes the second open run the rung
	// again over a column that now exists.
	for i := range 2 {
		_, err = raw.Exec(`PRAGMA user_version = 19`)
		require.NoError(t, err)
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, want, channelTypesShape(t, raw), "an upgraded table must match a fresh one")
	var card string
	require.NoError(t, raw.QueryRow(`SELECT cardinality FROM channel_types WHERE type = 'discussion'`).Scan(&card))
	require.Empty(t, card, "a row recorded before v20 has no cardinality")
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
