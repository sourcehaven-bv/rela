package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v34ToV35FlowRuns pins the v34 to v35 rung (TKT-PV9R0GK): a
// version-34 file gains flow_runs, flow_definitions and their indexes with
// the shape of a fresh database, and opening a second time changes nothing.
func TestMigrate_v34ToV35FlowRuns(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	tables := []string{"flow_definitions", "flow_runs"}
	indexes := []string{
		"flow_runs_due_idx", "flow_runs_subject_idx", "flow_runs_flow_idx",
		"flow_runs_definition_idx", "flow_runs_singleton_key",
	}
	want := map[string]any{}
	for _, name := range tables {
		want[name] = tableShape(t, fresh, name)
		require.NotEmpty(t, want[name], name)
	}
	for _, name := range indexes {
		want[name] = indexSQL(t, fresh, name)
		require.NotEmpty(t, want[name], name)
	}

	path := filepath.Join(t.TempDir(), "v34.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP TABLE flow_runs`,
		`DROP TABLE flow_definitions`,
		`PRAGMA user_version = 34`,
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
