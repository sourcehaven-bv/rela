package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v41ToV42AuditLog pins the v41 to v42 rung (TKT-USQBMG6): a
// version-41 file gains the audit tables and indexes with the shape of a
// fresh database, and opening a second time changes nothing.
func TestMigrate_v41ToV42AuditLog(t *testing.T) {
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

	path := filepath.Join(t.TempDir(), "v41.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP TABLE audit_subjects`,
		`DROP TABLE audit_log`,
		`DROP TABLE audit_anchor`,
		`PRAGMA user_version = 41`,
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

// TestMigrate_v43ToV44AuditSubjectLifecycle pins the v43 to v44 rung
// (TKT-X1ZFO04): audit_subjects gains face and starts with the shape of a
// fresh database, starts is backfilled from each row's record, and opening a
// second time changes nothing.
func TestMigrate_v43ToV44AuditSubjectLifecycle(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantShape := tableShape(t, fresh, "audit_subjects")
	wantIndex := indexSQL(t, fresh, "audit_subjects_starts_idx")
	require.NotEmpty(t, wantIndex)

	path := filepath.Join(t.TempDir(), "v43.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP TABLE audit_subjects`,
		`CREATE TABLE audit_subjects (
			entity_id TEXT NOT NULL,
			seq       INTEGER NOT NULL,
			PRIMARY KEY (entity_id, seq)
		) STRICT`,
		`CREATE INDEX audit_subjects_seq_idx ON audit_subjects(seq)`,
		`INSERT INTO audit_log (seq, at, op, principal_user, doc, prev_hash, hash) VALUES
			(1, 0, 'create-entity', 'u', '{"op":"create-entity","subject":{"kind":"entity","id":"A"}}', '', 'h1'),
			(2, 0, 'update-entity', 'u', '{"op":"update-entity","subject":{"kind":"entity","id":"A"}}', 'h1', 'h2'),
			(3, 0, 'rename-entity', 'u',
			 '{"op":"rename-entity","before":{"kind":"entity","id":"A"},"after":{"kind":"entity","id":"B"}}', 'h2', 'h3')`,
		`INSERT INTO audit_subjects (entity_id, seq) VALUES ('A', 1), ('A', 2), ('A', 3), ('B', 3)`,
		`PRAGMA user_version = 43`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantShape, tableShape(t, raw, "audit_subjects"))
	require.Equal(t, wantIndex, indexSQL(t, raw, "audit_subjects_starts_idx"))
	rows, err := raw.Query(`SELECT entity_id || ':' || seq FROM audit_subjects WHERE starts ORDER BY seq, entity_id`)
	require.NoError(t, err)
	defer rows.Close()
	var starts []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		starts = append(starts, s)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"A:1", "B:3"}, starts)
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
