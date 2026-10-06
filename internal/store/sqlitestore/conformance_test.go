package sqlitestore_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// open builds a store over a fresh database.
//
// Two steps rather than one because the store no longer owns the file:
// sqlitedb opens it (and is what the cleanup closes), sqlitestore borrows the
// handle.
func open(t *testing.T, opts ...sqlitestore.Option) *sqlitestore.Store {
	t.Helper()
	return openAt(t, filepath.Join(t.TempDir(), "conformance.db"), opts...)
}

func openAt(t *testing.T, path string, opts ...sqlitestore.Option) *sqlitestore.Store {
	t.Helper()
	s, _ := openWithDB(t, path, opts...)
	return s
}

// openWithDB opens a store at path and also returns its database, for the
// search backend that reads the database directly.
func openWithDB(t *testing.T, path string, opts ...sqlitestore.Option) (*sqlitestore.Store, *sqlitedb.DB) {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	s, err := sqlitestore.New(db, opts...)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, db
}

func factory(t *testing.T) store.Store {
	t.Helper()
	return open(t)
}

// searchFactory pairs the store with its FTS5 search backend (DEC-10Z731),
// which is what the sqlite build ships.
func searchFactory(t *testing.T) (store.Store, search.Searcher) {
	t.Helper()
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "conformance.db"))
	backend, err := sqlitestore.NewSearchBackend(db)
	if err != nil {
		t.Fatalf("NewSearchBackend: %v", err)
	}
	return s, search.New(s, backend)
}

func visibleSearchFactory(t *testing.T) (store.Store, search.Searcher, search.VisibleSearcher) {
	t.Helper()
	s, searcher := searchFactory(t)
	v, err := search.NewVisible(searcher, s)
	if err != nil {
		t.Fatalf("NewVisible: %v", err)
	}
	return s, searcher, v
}

// TestConformance runs the shared suite. TxRollback is declared because this
// backend takes the STRONG Tx contract (DEC-8UIL0) — rollback on error and
// post-commit-only event delivery — which the spike measured SQLite provides.
// Versioning is declared because TKT-4NU9ZD made sqlitestore the second
// backend to implement store.VersionService, which is what moved the version
// contract out of pgstore's own tests and into storetest.
func TestConformance(t *testing.T) {
	storetest.RunAll(t, factory, searchFactory, visibleSearchFactory, storetest.Capabilities{
		SoftDelete: true,
		Observers: func(t *testing.T, obs ...store.EntityObserver) store.Store {
			t.Helper()
			opts := make([]sqlitestore.Option, 0, len(obs))
			for _, o := range obs {
				opts = append(opts, sqlitestore.WithObserver(o))
			}
			return open(t, opts...)
		},
		Attachments: true,
		TxRollback:  true,
		Versioning:  true,
		SweepNow: func(t *testing.T, s store.Store) {
			t.Helper()
			require.NoError(t, s.(*sqlitestore.Store).SweepNow(t.Context(), fixedProjection{}, immediateSweep(100)))
		},
	})
}

// TestTxAbnormalExit covers what RunTxRollbackTests cannot see: a panic in fn
// and a context cancelled at commit. Both abandoned an open transaction on a
// pooled connection before the fix, which poisoned every later transaction AND
// made the uncommitted write durable.
func TestTxAbnormalExit(t *testing.T) {
	storetest.RunTxAbnormalExitTests(t, factory)
}

// TestTxObserverIsolation asserts observers never witness a rolled-back write.
// Without it a rollback leaves the search index holding a phantom entity.
func TestTxObserverIsolation(t *testing.T) {
	storetest.RunTxObserverIsolationTests(t, func(t *testing.T, o store.EntityObserver) store.Store {
		t.Helper()
		return open(t, sqlitestore.WithObserver(o))
	})
}

// TestTxStress is the mixed-workload soak with a deadlock watchdog. It is the
// test that justified the whole spike: modernc's locking under sustained
// multi-connection contention was the one thing that could not be established
// from documentation.
func TestTxStress(t *testing.T) {
	storetest.RunTxStressTest(t, factory)
}

func fuzzFactory() storetest.FuzzFactory {
	var n atomic.Int64
	// FuzzFactory has no *testing.T, so no t.TempDir(). One shared directory
	// the OS reclaims, with a unique file per iteration; each store is closed
	// by the harness.
	dir, err := os.MkdirTemp("", "sqlitestore-fuzz")
	if err != nil {
		panic(err)
	}
	return func() store.Store {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
			Path: filepath.Join(dir, fmt.Sprintf("fuzz%d.db", n.Add(1))),
		})
		if err != nil {
			panic(err)
		}
		s, err := sqlitestore.New(db)
		if err != nil {
			_ = db.Close()
			panic(err)
		}
		return s
	}
}

func FuzzRelationKeyCollision(f *testing.F) {
	storetest.FuzzRelationKeyCollision(f, fuzzFactory())
}

func FuzzAttachmentKeyCollision(f *testing.F) {
	storetest.FuzzAttachmentKeyCollision(f, fuzzFactory())
}

func FuzzRenameKeyCollapse(f *testing.F) {
	storetest.FuzzRenameKeyCollapse(f, fuzzFactory())
}

func FuzzConcurrentOps(f *testing.F) {
	storetest.FuzzConcurrentOps(f, fuzzFactory())
}

func FuzzCloneNestedValues(f *testing.F) {
	storetest.FuzzCloneNestedValues(f, fuzzFactory())
}

func FuzzPropertyValuesTypeZoo(f *testing.F) {
	storetest.FuzzPropertyValuesTypeZoo(f, fuzzFactory())
}

func TestGraphDifferential(t *testing.T) {
	storetest.RunGraphDifferential(t, factory)
}
