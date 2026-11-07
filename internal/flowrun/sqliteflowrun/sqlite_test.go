package sqliteflowrun_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqliteflowrun"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// The conformance suite runs in sqlflowrun, over this adapter. These tests
// cover what is the adapter's own.

func TestNewRejectsNil(t *testing.T) {
	_, err := sqliteflowrun.New(nil)
	require.Error(t, err)
}

// TestReopen: a run and its definition survive closing and reopening the
// database.
func TestReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rela.db")
	t0 := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)

	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	require.NoError(t, err)
	s, err := sqliteflowrun.New(db.DB())
	require.NoError(t, err)
	created, err := s.Create(ctx, flowrun.NewRun{
		ID: "FR-1", Flow: "onboard", Version: "0123456789abcdef", Definition: []byte("def"),
		Subject: entity.Ref{ID: "EMP-1"}, Singleton: true, At: t0,
		Instance: flow.Instance{State: "open", Generation: 1, EnteredAt: t0},
	})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	db, err = sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s, err = sqliteflowrun.New(db.DB())
	require.NoError(t, err)
	got, err := s.Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, created, got)
	def, err := s.Definition(ctx, "onboard", "0123456789abcdef")
	require.NoError(t, err)
	require.Equal(t, []byte("def"), def)
}
