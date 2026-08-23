package sqlitestore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func open(t *testing.T, opts ...sqlitestore.Option) *sqlitestore.Store {
	t.Helper()
	s, err := sqlitestore.Open(sqlitestore.Options{
		Path: filepath.Join(t.TempDir(), "conformance.db"),
	}, opts...)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func factory(t *testing.T) store.Store {
	t.Helper()
	return open(t)
}

// searchFactory pairs the store with the generic linear searcher. SQLite has
// FTS5 available, but a native searcher is deliberately out of scope here
// (DEC-LFSYNY stage 3): search.Visible wraps ANY Searcher, so bleve/linear is
// a valid pairing and FTS5 is a later optimization rather than an entry
// requirement.
func searchFactory(t *testing.T) (store.Store, search.Searcher) {
	t.Helper()
	idx := search.NewLinearSearch()
	// The index must be an OBSERVER, not a Subscribe consumer: observer
	// callbacks are synchronous and carry the entity, whereas events may be
	// dropped when a subscriber is slow.
	s := open(t, sqlitestore.WithObserver(idx))
	return s, search.New(s, idx)
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
func TestConformance(t *testing.T) {
	storetest.RunAll(t, factory, searchFactory, visibleSearchFactory, storetest.Capabilities{
		Attachments: true,
		TxRollback:  true,
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
		s, err := sqlitestore.Open(sqlitestore.Options{
			Path: filepath.Join(dir, fmt.Sprintf("fuzz%d.db", n.Add(1))),
		})
		if err != nil {
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
