package sqliteaudit_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit/sqlaudittest"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqliteaudit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

func TestConformance(t *testing.T) {
	sqlaudittest.RunAll(t, func(t *testing.T) sqlaudittest.Harness {
		t.Helper()
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		log, err := sqliteaudit.New(db.DB())
		require.NoError(t, err)
		raw, err := sqliteexec.New(db.DB())
		require.NoError(t, err)
		return sqlaudittest.Harness{Log: log, Raw: raw}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := sqliteaudit.New(nil)
	require.Error(t, err)
}

func TestNewInTxRejectsNil(t *testing.T) {
	_, err := sqliteaudit.NewInTx(nil)
	require.Error(t, err)
}

// TestNewInTxFollowsTheTransaction pins that an append through NewInTx
// commits and rolls back with the transaction it joined.
func TestNewInTxFollowsTheTransaction(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	log, err := sqliteaudit.New(db.DB())
	require.NoError(t, err)

	appendIn := func(end string) {
		t.Helper()
		conn, cErr := db.DB().Conn(ctx)
		require.NoError(t, cErr)
		defer func() { _ = conn.Close() }()
		_, cErr = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		require.NoError(t, cErr)
		inTx, cErr := sqliteaudit.NewInTx(conn)
		require.NoError(t, cErr)
		require.NoError(t, inTx.AppendAll(ctx, []audit.Record{
			{Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"}},
			{Op: audit.OpUpdateEntity, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"}},
		}))
		_, cErr = conn.ExecContext(ctx, end)
		require.NoError(t, cErr)
	}

	appendIn("ROLLBACK")
	v, err := log.Verify(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 0, v.Rows, "a rolled-back transaction keeps none of its records")

	appendIn("COMMIT")
	v, err = log.Verify(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, v.Rows, "a committed transaction keeps all of its records")
}
