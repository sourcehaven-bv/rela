package sqliteaudit_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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
