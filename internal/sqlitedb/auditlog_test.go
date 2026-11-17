package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v40ToV41AuditLog pins the v40 to v41 rung (TKT-USQBMG6): a
// version-40 file gains the audit tables and indexes with the shape of a
// fresh database, and opening a second time changes nothing.
func TestMigrate_v40ToV41AuditLog(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	tables := []string{"audit_log", "audit_subjects", "audit_anchor"}
	indexes := []string{"audit_log_at_idx", "audit_subjects_seq_idx"}
	want := map[string]any{}
	for _, name := range tables {
		want[name] = tableShape(t, fresh, name)
		require.NotEmpty(t, want[name], name)
	}
	for _, name := range indexes {
		want[name] = indexSQL(t, fresh, name)
		require.NotEmpty(t, want[name], name)
	}

	path := filepath.Join(t.TempDir(), "v40.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP TABLE audit_subjects`,
		`DROP TABLE audit_log`,
		`DROP TABLE audit_anchor`,
		`PRAGMA user_version = 40`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	for _, name := range tables {
		require.Equal(t, want[name], tableShape(t, raw, name), name)
	}
	for _, name := range indexes {
		require.Equal(t, want[name], indexSQL(t, raw, name), name)
	}
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
