//go:build sqlite

package appbuild

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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

	t.Run("sweep is built", func(t *testing.T) {
		sw, err := newVersionSweep(sqliteTestStore(t), metamodel.DefaultMetamodel())
		require.NoError(t, err)
		require.NotNil(t, sw,
			"newVersionSweep built no sweep for sqlitestore; create and update "+
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

	got, err := svc.ListVersions(ctx, entity.Ref{ID: "FEAT-1"})
	require.NoError(t, err)
	require.Len(t, got, 1, "the version just written did not read back")
}

// TestSQLiteSweepNeedsTheMetamodel pins that a sweep with no projection to
// stamp is refused when the store has one, rather than built and left to panic
// on its first tick.
func TestSQLiteSweepNeedsTheMetamodel(t *testing.T) {
	sw, err := newVersionSweep(sqliteTestStore(t), nil)
	require.Error(t, err)
	require.Nil(t, sw)
}

// TestSQLiteVersionTaggerIsReachable proves the tagger resolves on the sqlite
// build with the metamodel projection, that a missing metamodel is an error
// rather than a tagger that cannot capture or a silent "unsupported", and
// that the lookup resolves without a projection.
func TestSQLiteVersionTaggerIsReachable(t *testing.T) {
	st := sqliteTestStore(t)
	tg, err := versionTaggerFor(st, metamodel.DefaultMetamodel())
	require.NoError(t, err)
	require.NotNil(t, tg)
	_, err = versionTaggerFor(st, nil)
	require.Error(t, err)
	require.NotNil(t, versionTagReaderFor(st))
}
