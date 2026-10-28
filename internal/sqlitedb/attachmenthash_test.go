package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// attachmentsShape returns the column list of attachments as
// name:type:notnull:default, in declaration order.
func attachmentsShape(t *testing.T, raw *sql.DB) []string {
	t.Helper()
	rows, err := raw.Query(`SELECT name, type, "notnull", coalesce(dflt_value, '') FROM pragma_table_info('attachments')`)
	require.NoError(t, err)
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name, typ, def string
		var notnull int
		require.NoError(t, rows.Scan(&name, &typ, &notnull, &def))
		cols = append(cols, name+":"+typ+":"+string(rune('0'+notnull))+":"+def)
	}
	require.NoError(t, rows.Err())
	return cols
}

// TestMigrate_v28ToV29AttachmentSHA256 pins the v28 to v29 rung
// (TKT-1TS55R): a version-28 file gains attachments.sha256 with the shape of
// a fresh database, existing rows get the hash of their bytes, and opening a
// second time is harmless.
func TestMigrate_v28ToV29AttachmentSHA256(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantShape := attachmentsShape(t, fresh)

	path := filepath.Join(t.TempDir(), "v28.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`ALTER TABLE attachments DROP COLUMN sha256`,
		`PRAGMA user_version = 28`,
		`INSERT INTO attachments (entity_id, property, file_name, data, size, updated_at)
			VALUES ('T-1', 'spec', 'a.txt', X'616263', 3, '2026-01-01T00:00:00Z'),
			       ('T-1', 'spec', 'empty.txt', X'', 0, '2026-01-01T00:00:00Z')`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantShape, attachmentsShape(t, raw), "an upgraded table must match a fresh one")
	want := map[string]string{
		"a.txt":     "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"empty.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	for name, sum := range want {
		var got string
		require.NoError(t, raw.QueryRow(`SELECT sha256 FROM attachments WHERE file_name = ?`, name).Scan(&got))
		require.Equal(t, sum, got, name)
	}
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
