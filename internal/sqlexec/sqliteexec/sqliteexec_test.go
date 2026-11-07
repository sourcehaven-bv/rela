package sqliteexec_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

func openDB(t *testing.T) *sqlitedb.DB {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.DB().ExecContext(context.Background(), `CREATE TABLE t (k TEXT PRIMARY KEY, v INTEGER) STRICT`)
	require.NoError(t, err)
	return db
}

func count(t *testing.T, q sqlexec.Querier) int {
	t.Helper()
	var n int
	require.NoError(t, q.QueryRow(context.Background(), `SELECT COUNT(*) FROM t`).Scan(&n))
	return n
}

func TestNewRejectsNil(t *testing.T) {
	_, err := sqliteexec.New(nil)
	require.Error(t, err)
	_, err = sqliteexec.NewInTx(nil, "x")
	require.Error(t, err)
}

func TestNewInTxRejectsHostileSavepoint(t *testing.T) {
	db := openDB(t)
	conn, err := db.DB().Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	for _, name := range []string{"", "a; DROP TABLE t", "Sp", "1sp", "sp-1"} {
		_, err := sqliteexec.NewInTx(conn, name)
		require.Error(t, err, name)
	}
}

// TestTransact: a transaction commits when fn succeeds and rolls back every
// statement when it fails; rows and the no-rows sentinel read through.
func TestTransact(t *testing.T) {
	ctx := context.Background()
	h, err := sqliteexec.New(openDB(t).DB())
	require.NoError(t, err)

	require.NoError(t, h.Transact(ctx, func(q sqlexec.Querier) error {
		n, execErr := q.Exec(ctx, `INSERT INTO t (k, v) VALUES (?, ?), (?, ?)`, "a", 1, "b", 2)
		require.NoError(t, execErr)
		require.Equal(t, int64(2), n)
		return nil
	}))
	boom := errors.New("boom")
	err = h.Transact(ctx, func(q sqlexec.Querier) error {
		_, execErr := q.Exec(ctx, `INSERT INTO t (k, v) VALUES ('c', 3)`)
		require.NoError(t, execErr)
		return boom
	})
	require.ErrorIs(t, err, boom)
	require.Equal(t, 2, count(t, h))

	rows, err := h.Query(ctx, `SELECT k FROM t ORDER BY k`)
	require.NoError(t, err)
	var keys []string
	for rows.Next() {
		var k string
		require.NoError(t, rows.Scan(&k))
		keys = append(keys, k)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Equal(t, []string{"a", "b"}, keys)

	var v int
	require.ErrorIs(t, h.QueryRow(ctx, `SELECT v FROM t WHERE k = 'z'`).Scan(&v), sqlexec.ErrNoRows)
	_, err = h.Exec(ctx, `INSERT INTO nope VALUES (1)`)
	require.Error(t, err)
	bad, err := h.Query(ctx, `SELECT * FROM nope`)
	require.Error(t, err)
	require.Nil(t, bad)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.Error(t, h.Transact(cancelled, func(sqlexec.Querier) error { return nil }))
}

// TestNewInTx: writes through a joined handle commit or roll back with the
// owner's transaction, and a failed write undoes only its own savepoint.
func TestNewInTx(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	run := func(commit bool, body func(h sqlexec.DB)) {
		conn, err := db.DB().Conn(ctx)
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		require.NoError(t, err)
		h, err := sqliteexec.NewInTx(conn, "sp")
		require.NoError(t, err)
		body(h)
		end := "ROLLBACK"
		if commit {
			end = "COMMIT"
		}
		_, err = conn.ExecContext(ctx, end)
		require.NoError(t, err)
	}
	insert := func(k string, fail error) func(sqlexec.Querier) error {
		return func(q sqlexec.Querier) error {
			if _, err := q.Exec(ctx, `INSERT INTO t (k, v) VALUES (?, 0)`, k); err != nil {
				return err
			}
			return fail
		}
	}
	boom := errors.New("boom")

	run(true, func(h sqlexec.DB) {
		require.NoError(t, h.Transact(ctx, insert("a", nil)))
		require.ErrorIs(t, h.Transact(ctx, insert("b", boom)), boom)
	})
	run(false, func(h sqlexec.DB) {
		require.NoError(t, h.Transact(ctx, insert("c", nil)))
	})
	plain, err := sqliteexec.New(db.DB())
	require.NoError(t, err)
	require.Equal(t, 1, count(t, plain), "only a survives")
}
