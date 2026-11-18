package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration0053_AuditSubjectLifecycle pins migration 0053 (TKT-X1ZFO04):
// a schema at version 51 gains face and starts on audit_subjects and the
// starts index; starts is backfilled from each row's record (a create of the
// id, or a rename into it), and a second run changes nothing.
func TestMigration0053_AuditSubjectLifecycle(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`DROP INDEX audit_subjects_starts_idx`,
		`ALTER TABLE audit_subjects DROP COLUMN face, DROP COLUMN starts`,
		`UPDATE schema_version SET version = 52`,
		`INSERT INTO audit_log (seq, at, op, principal_user, doc, prev_hash, hash) VALUES
			(1, 0, 'create-entity', 'u', '{"op":"create-entity","subject":{"kind":"entity","id":"A"}}', '', 'h1'),
			(2, 0, 'update-entity', 'u', '{"op":"update-entity","subject":{"kind":"entity","id":"A"}}', 'h1', 'h2'),
			(3, 0, 'rename-entity', 'u',
			 '{"op":"rename-entity","before":{"kind":"entity","id":"A"},"after":{"kind":"entity","id":"B"}}', 'h2', 'h3')`,
		`INSERT INTO audit_subjects (entity_id, seq) VALUES ('A', 1), ('A', 2), ('A', 3), ('B', 3)`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	rows, err := pool.Query(ctx,
		`SELECT entity_id || ':' || seq || ':' || face FROM audit_subjects WHERE starts ORDER BY seq, entity_id`)
	require.NoError(t, err)
	var starts []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		starts = append(starts, s)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"A:1:", "B:3:"}, starts)
	var indexes int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = 'audit_subjects_starts_idx'`).Scan(&indexes))
	require.Equal(t, 1, indexes)
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, target, current)
}
