//go:build sqlite

package appbuild

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// This file is the counterpart to backendneutral_version_test.go. That one
// proves the resolvers can be satisfied WITHOUT naming a backend; this one
// proves the backend this build actually ships satisfies them.
//
// Both are needed, and the gap between them is what went wrong. sqlitestore
// implemented the full store.VersionService, its conformance suite passed, and
// the capability was still unreachable from a running app: versionServiceFor
// lived behind //go:build !postgres and returned nil, so every history read on
// the sqlite build saw "versioning not available" with nothing failing
// anywhere. A compile-time interface assertion would not have caught it — the
// store satisfied the interface the whole time. Only asking the RESOLVER does.

// sqliteTestStore opens a throwaway store over a fresh database file.
func sqliteTestStore(t *testing.T) *sqlitestore.Store {
	t.Helper()

	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "wiring.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	st, err := sqlitestore.New(db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestSQLiteStoreSatisfiesTheVersionCapabilities is the regression guard.
//
// It asks the resolvers rather than asserting the interfaces, because the
// interfaces were never the broken part: the wiring was.
func TestSQLiteStoreSatisfiesTheVersionCapabilities(t *testing.T) {
	t.Run("version service is reachable", func(t *testing.T) {
		svc := versionServiceFor(sqliteTestStore(t))
		require.NotNil(t, svc,
			"versionServiceFor returned nil for sqlitestore; every history read "+
				"and every synchronous rename/delete capture is silently dropped")
	})

	t.Run("sweep capability is reachable", func(t *testing.T) {
		var st store.Store = sqliteTestStore(t)
		_, ok := st.(versionSweeper)
		require.True(t, ok,
			"sqlitestore does not satisfy store.VersionSweeper; create and update "+
				"versions would never be captured, since only rename and delete "+
				"are synchronous")
	})
}

// TestSQLiteVersionServiceIsUsable goes one step past reachability: a handle
// that is non-nil but backed by nothing would pass the test above and fail at
// the first write. The cheapest proof it is real is a round trip.
func TestSQLiteVersionServiceIsUsable(t *testing.T) {
	svc := versionServiceFor(sqliteTestStore(t))
	require.NotNil(t, svc)

	ctx := context.Background()
	require.NoError(t, svc.WriteVersion(ctx, store.VersionInput{
		EntityID:   "FEAT-1",
		Type:       "feature",
		Op:         store.VersionOpDelete,
		Content:    "gone",
		SchemaHash: "schema-1",
		Projection: []byte(`{"v":1}`),
	}))

	got, err := svc.ListVersions(ctx, "FEAT-1")
	require.NoError(t, err)
	require.Len(t, got, 1, "the version just written did not read back")
}

// TestSQLiteSweepStartsFromTheSharedResolver pins the other half of the wiring.
// startVersionSweepIfSupported is what assemble calls; a sweep that never
// starts means create/update versions are never captured, which looks exactly
// like "nobody edited anything" until someone opens a history page.
func TestSQLiteSweepStartsFromTheSharedResolver(t *testing.T) {
	st := sqliteTestStore(t)

	// Through the resolver assemble calls, not through the store's method
	// directly — the method was never in doubt, the dispatch to it was.
	//
	// The cadence override keeps any tick from running during the test: a nil
	// metamodel would panic in the provider. What is under test is that the
	// call reaches the store and the goroutine is stoppable, not what a tick
	// does; that is the conformance suite's job.
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", time.Hour.String())
	startVersionSweepIfSupported(st, nil)

	// Close must stop it. A sweep still running after Close would keep issuing
	// statements against a database the caller is about to close, which is a
	// shutdown-time error message on every single run.
	require.NoError(t, st.Close())
}
