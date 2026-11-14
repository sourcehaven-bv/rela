package sqliterecords_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/recordstest"
	"github.com/Sourcehaven-BV/rela/internal/records/sqliterecords"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

func TestConformance(t *testing.T) {
	recordstest.RunAll(t, func(t *testing.T) records.Store {
		t.Helper()
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		s, err := sqliterecords.New(db.DB())
		require.NoError(t, err)
		return s
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := sqliterecords.New(nil)
	require.Error(t, err)
	_, err = sqliterecords.NewInTx(nil)
	require.Error(t, err)
}
