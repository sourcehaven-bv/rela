package sqlitedb_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// contentHashTables are the tables the v13→v14 step adds content_hash to.
var contentHashTables = []string{"entities", "relations", "marked_entities", "marked_relations"}

// contentHashObjects are the triggers and indexes init creates over
// content_hash. A test that rewinds a database to before the column must drop
// them first.
var contentHashObjects = []struct{ kind, name string }{
	{"TRIGGER", "entities_clear_content_hash"},
	{"TRIGGER", "relations_clear_content_hash"},
	{"TRIGGER", "entities_insert_clear_content_hash"},
	{"TRIGGER", "relations_insert_clear_content_hash"},
	{"TRIGGER", "entity_versions_insert_clear_content_hash"},
	{"TRIGGER", "entity_versions_delete_clear_content_hash"},
	{"TRIGGER", "relation_versions_insert_clear_content_hash"},
	{"TRIGGER", "relation_versions_delete_clear_content_hash"},
	{"INDEX", "entities_unhashed_idx"},
	{"INDEX", "relations_unhashed_idx"},
}

// openRaw creates a database at the current schema version and returns a
// plain connection to it, so a test can rewind or inspect it underneath
// sqlitedb.
func openRaw(t *testing.T) (path string, raw *sql.DB) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "rela.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err = sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	return path, raw
}

// reopenAt stamps version on the database and opens it through sqlitedb, so
// the ladder runs every rung above version.
func reopenAt(t *testing.T, path string, raw *sql.DB, version int) {
	t.Helper()
	_, err := raw.Exec(`PRAGMA user_version = ` + strconv.Itoa(version))
	require.NoError(t, err)
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())
}

// readEntityHash returns FEAT-1's stored content_hash; nil means not known.
func readEntityHash(t *testing.T, raw *sql.DB) *string {
	t.Helper()
	var h *string
	require.NoError(t, raw.QueryRow(`SELECT content_hash FROM entities WHERE id = 'FEAT-1'`).Scan(&h))
	return h
}

// TestMigrateToContentHash pins that a v13 database gains content_hash on the
// live and soft-delete tables, that existing rows read as "not known" (NULL),
// that running the step again is harmless, and that the trigger clears a
// stored hash when the row's content changes (BUG-1DWMYO).
func TestMigrateToContentHash(t *testing.T) {
	path, raw := openRaw(t)
	for _, o := range contentHashObjects {
		_, err := raw.Exec("DROP " + o.kind + " " + o.name)
		require.NoError(t, err, o.name)
	}
	for _, table := range contentHashTables {
		_, err := raw.Exec("ALTER TABLE " + table + " DROP COLUMN content_hash")
		require.NoError(t, err, table)
	}
	_, err := raw.Exec(`INSERT INTO entities (id, type, updated_at)
		VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00.000000000Z')`)
	require.NoError(t, err)

	// Stamping v13 before each open makes the second open run the step again
	// over columns that now exist.
	for range 2 {
		reopenAt(t, path, raw, 13)
	}

	for _, table := range contentHashTables {
		var n int
		require.NoError(t, raw.QueryRow(
			`SELECT count(*) FROM pragma_table_info(?) WHERE name = 'content_hash'`, table).Scan(&n))
		require.Equalf(t, 1, n, "%s lacks content_hash after migrating", table)
	}
	require.Nil(t, readEntityHash(t, raw), "a pre-existing row must read as not known")

	_, err = raw.Exec(`UPDATE entities SET content_hash = 'h1' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = raw.Exec(`UPDATE entities SET content = '' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.NotNil(t, readEntityHash(t, raw), "an update that changes nothing must keep the hash")

	_, err = raw.Exec(`UPDATE entities SET content = 'changed' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.Nil(t, readEntityHash(t, raw), "a content change must clear the stored hash")
}

// TestMigrateClearsV14Hashes pins that the v14→v15 step forgets hashes stored
// under v14, which promised less than v15 does (TASK-Y73Y9 in Atlas).
func TestMigrateClearsV14Hashes(t *testing.T) {
	path, raw := openRaw(t)
	_, err := raw.Exec(`INSERT INTO entities (id, type, updated_at, content_hash)
		VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00.000000000Z', 'h1')`)
	require.NoError(t, err)
	// The insert trigger clears a hash a row is inserted with, so set it after.
	_, err = raw.Exec(`UPDATE entities SET content_hash = 'h1' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)

	reopenAt(t, path, raw, 14)
	require.Nil(t, readEntityHash(t, raw))
}

// TestVersionTriggersClearContentHash pins the triggers that keep a stored
// hash equal to the latest version's: the sweep selects only rows whose hash
// is NULL, so a hash left standing after a version change hides that row from
// it for good.
func TestVersionTriggersClearContentHash(t *testing.T) {
	// Each case's statements run after FEAT-1 has one version and that
	// version's hash is stored on the row.
	cases := []struct {
		name  string
		write []string
		clear bool
	}{
		{name: "VersionWithSameHash", write: []string{versionSQL(2, "", "update", "h1")}},
		{name: "VersionWithOtherHash", clear: true, write: []string{versionSQL(2, "", "update", "h2")}},
		{name: "DeleteVersion", clear: true, write: []string{versionSQL(2, "", "delete", "h1")}},
		// A face is its own lineage, so another face's version leaves this
		// face's hash alone.
		{name: "VersionOnAnotherFace", write: []string{
			versionSQL(2, "draft", "update", "h2"),
		}},
		{name: "PurgedVersion", clear: true, write: []string{`DELETE FROM entity_versions WHERE vseq = 1`}},
		{name: "RowInsertedWithHash", clear: true, write: []string{
			`DELETE FROM entities WHERE id = 'FEAT-1'`,
			`INSERT INTO entities (id, type, updated_at, content_hash)
				VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00.000000000Z', 'h1')`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, raw := openRaw(t)
			_, err := raw.Exec(`INSERT INTO entities (id, type, updated_at)
				VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00.000000000Z')`)
			require.NoError(t, err)
			_, err = raw.Exec(versionSQL(1, "", "create", "h1"))
			require.NoError(t, err)
			_, err = raw.Exec(`UPDATE entities SET content_hash = 'h1' WHERE id = 'FEAT-1'`)
			require.NoError(t, err)

			for _, q := range tc.write {
				_, err = raw.Exec(q)
				require.NoError(t, err, q)
			}
			if tc.clear {
				require.Nil(t, readEntityHash(t, raw))
			} else {
				require.Equal(t, "h1", *readEntityHash(t, raw))
			}
		})
	}
}

// versionSQL inserts a version of FEAT-1 at face with the given sequence, op
// and hash.
func versionSQL(vseq int, face, op, hash string) string {
	return fmt.Sprintf(`INSERT INTO entity_versions
		(vseq, entity_id, face, op, type, properties, content, content_hash, schema_hash,
		 principal_user, principal_tool, created_at)
		VALUES (%d, 'FEAT-1', '%s', '%s', 'feature', '{}', '', '%s', 's', 'u', 't', '2026-01-01T00:00:00Z')`,
		vseq, face, op, hash)
}

// TestRelationVersionTriggersClearContentHash is TestVersionTriggersClearContentHash
// for relations, whose lineage is the row's rel_record_id.
func TestRelationVersionTriggersClearContentHash(t *testing.T) {
	const version = `INSERT INTO relation_versions
		(vseq, rel_record_id, op, from_id, rel_type, to_id, content_hash, schema_hash, created_at)
		VALUES (%d, 7, '%s', 'FEAT-1', 'depends-on', 'FEAT-2', '%s', 's', '2026-01-01T00:00:00Z')`
	cases := []struct {
		name  string
		write string
		clear bool
	}{
		{name: "VersionWithSameHash", write: fmt.Sprintf(version, 2, "update", "h1")},
		{name: "VersionWithOtherHash", clear: true, write: fmt.Sprintf(version, 2, "update", "h2")},
		{name: "DeleteVersion", clear: true, write: fmt.Sprintf(version, 2, "delete", "h1")},
		{name: "PurgedVersion", clear: true, write: `DELETE FROM relation_versions WHERE vseq = 1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, raw := openRaw(t)
			for _, q := range []string{
				`INSERT INTO relations (from_id, rel_type, to_id, updated_at, rel_record_id)
					VALUES ('FEAT-1', 'depends-on', 'FEAT-2', '2026-01-01T00:00:00Z', 7)`,
				fmt.Sprintf(version, 1, "create", "h1"),
				`UPDATE relations SET content_hash = 'h1'`,
				tc.write,
			} {
				_, err := raw.Exec(q)
				require.NoError(t, err, q)
			}
			var h *string
			require.NoError(t, raw.QueryRow(`SELECT content_hash FROM relations`).Scan(&h))
			if tc.clear {
				require.Nil(t, h)
			} else {
				require.Equal(t, "h1", *h)
			}
		})
	}
}
