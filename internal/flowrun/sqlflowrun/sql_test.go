package sqlflowrun_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/flowruntest"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqlflowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/sqliteflowrun"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

var t0 = time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)

func openDB(t *testing.T) *sqlitedb.DB {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newStore(t *testing.T, db *sqlitedb.DB) *sqlflowrun.Store {
	t.Helper()
	s, err := sqliteflowrun.New(db.DB())
	require.NoError(t, err)
	return s
}

// TestConformance runs the shared contract over SQLite, the database every
// build can test without a server. pgflowrun runs it over PostgreSQL.
// Second is another store over the same database handle: its writes
// serialize on SQLite's write lock rather than on the first store's mutex.
func TestConformance(t *testing.T) {
	t.Parallel()
	flowruntest.RunAll(t, func(t *testing.T) flowruntest.Harness {
		t.Helper()
		db := openDB(t)
		return flowruntest.Harness{
			Store:  newStore(t, db),
			Second: func(t *testing.T) flowrun.Store { t.Helper(); return newStore(t, db) },
		}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := sqlflowrun.New(nil, sqlflowrun.Dialect{Name: "x"})
	require.Error(t, err)
}

func newRun(id string) flowrun.NewRun {
	return flowrun.NewRun{
		ID: flowrun.RunID(id), Flow: "onboard", Version: "0123456789abcdef", Definition: []byte("d"),
		Subject: entity.Ref{ID: "EMP-" + id}, At: t0,
		Instance: flow.Instance{State: "open", Generation: 1, EnteredAt: t0, NextDueAt: t0.Add(time.Hour)},
	}
}

// TestDamagedState: a run whose stored state does not decode fails the
// reads that touch it, naming the run.
func TestDamagedState(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	s := newStore(t, db)
	_, err := s.Create(ctx, newRun("FR-1"))
	require.NoError(t, err)
	_, err = db.DB().ExecContext(ctx, `UPDATE flow_runs SET doc = 'oops' WHERE id = 'FR-1'`)
	require.NoError(t, err)

	_, err = s.Get(ctx, "FR-1")
	require.ErrorContains(t, err, "FR-1")
	_, err = s.List(ctx, flowrun.Query{Limit: 10})
	require.Error(t, err)
	_, err = s.ListDue(ctx, t0.Add(2*time.Hour), flowrun.DueCursor{}, 10)
	require.Error(t, err)
}

// TestDatabaseErrors: a database that fails reports the error on every
// method rather than a zero result.
func TestDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	s := newStore(t, db)
	_, err := s.Create(ctx, newRun("FR-1"))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = s.Create(ctx, newRun("FR-2"))
	require.Error(t, err)
	_, err = s.Get(ctx, "FR-1")
	require.Error(t, err)
	_, err = s.Update(ctx, "FR-1", 1, newRun("FR-1").Instance, t0)
	require.Error(t, err)
	require.Error(t, s.Delete(ctx, "FR-1", 1))
	_, err = s.ListDue(ctx, t0, flowrun.DueCursor{}, 10)
	require.Error(t, err)
	_, err = s.List(ctx, flowrun.Query{Limit: 10})
	require.Error(t, err)
	_, err = s.RekeySubject(ctx, "EMP-FR-1", "EMP-X")
	require.Error(t, err)
	_, err = s.Definition(ctx, "onboard", "0123456789abcdef")
	require.Error(t, err)
	_, err = s.PruneDefinitions(ctx, t0)
	require.Error(t, err)
}

// TestDueUsesIndex: the due query is answered from the partial due index,
// not a scan of every run.
func TestDueUsesIndex(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	rows, err := db.DB().QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT id FROM flow_runs
		WHERE next_due_at > 0 AND next_due_at <= ? AND (next_due_at > ? OR (next_due_at = ? AND id > ?))
		ORDER BY next_due_at, id LIMIT ?`, 1, 0, 0, "", 10)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
		plan.WriteString(detail + "\n")
	}
	require.NoError(t, rows.Err())
	require.Contains(t, plan.String(), "flow_runs_due_idx")
}
