//go:build sqlite

package appbuild_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// sweepGoroutineMarker is the frame every running sqlite version sweep has on
// its stack.
const sweepGoroutineMarker = "sqlitestore.(*sweep).run"

// runningSweeps counts the sqlite sweep goroutines alive in this process.
func runningSweeps() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), sweepGoroutineMarker)
		}
		buf = make([]byte, 2*len(buf))
	}
}

// settledSweepCount returns runningSweeps once it holds still. A sweep stopped
// by an earlier test has closed its done channel but may not have left its
// last frame yet, and counting it would skew every expectation after it.
func settledSweepCount() int {
	prev := runningSweeps()
	for range 50 {
		time.Sleep(10 * time.Millisecond)
		n := runningSweeps()
		if n == prev {
			return n
		}
		prev = n
	}
	return prev
}

// requireSweeps waits until exactly want sweep goroutines are running.
func requireSweeps(t *testing.T, want int, msg string) {
	t.Helper()
	require.Eventually(t, func() bool { return runningSweeps() == want },
		2*time.Second, 10*time.Millisecond, msg)
}

// assembleSQLiteTenant opens a fresh sqlite database and assembles base over
// it, the way a multi-tenant host would. The database outlives the Services so
// the test can still write to a tenant whose Services it closed.
func assembleSQLiteTenant(t *testing.T, base *appbuild.SharedBase, name string) *appbuild.Services {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), name+".db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(db)
	require.NoError(t, err)
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// TestServicesClose_StopsOnlyItsOwnVersionSweep pins the ownership TKT-IPXEZQ
// moved out of the store: the sweep belongs to the Services that assembled it.
// Closing one tenant's Services must end that tenant's sweep goroutine (no
// leak), and must leave a sibling tenant's sweep running (the SharedBase
// invariant in storage.md: Close tears down only what its own Assemble built).
func TestServicesClose_StopsOnlyItsOwnVersionSweep(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "20ms")
	t.Setenv("RELA_VERSION_SWEEP_IDLE", "1ns")
	t.Setenv("RELA_VERSION_SWEEP_MAX_STALENESS", "1ns")

	before := settledSweepCount()
	base := newSharedBase(t)
	a := assembleSQLiteTenant(t, base, "tenant-a")
	b := assembleSQLiteTenant(t, base, "tenant-b")
	requireSweeps(t, before+2, "each assembled tenant starts its own sweep")

	require.NoError(t, a.Close())
	requireSweeps(t, before+1, "Services.Close left its sweep goroutine running")

	ctx := context.Background()
	versions := func(svc *appbuild.Services, id string) int {
		t.Helper()
		vp, ok := svc.Store().(store.VersionServiceProvider)
		require.True(t, ok)
		metas, err := vp.VersionStore().ListVersions(ctx, entity.Ref{ID: id})
		require.NoError(t, err)
		return len(metas)
	}

	// B's sweep still captures.
	require.NoError(t, b.Store().CreateEntity(ctx, entity.New("DOC-B", "doc")))
	require.Eventually(t, func() bool { return versions(b, "DOC-B") == 1 },
		3*time.Second, 10*time.Millisecond, "closing tenant A stopped tenant B's sweep")

	// A's does not: its store and database are still usable, but nothing
	// sweeps them any more.
	require.NoError(t, a.Store().CreateEntity(ctx, entity.New("DOC-A", "doc")))
	time.Sleep(200 * time.Millisecond) // ten intervals
	require.Zero(t, versions(a, "DOC-A"), "a closed tenant's sweep still captured")

	require.NoError(t, b.Close())
	requireSweeps(t, before, "Services.Close left its sweep goroutine running")
}

// TestCloseAssembly_StopsTheVersionSweep pins that a reload retiring a
// superseded assembly also retires its sweep. The successor assembled over the
// same store runs its own, so the store keeps exactly one.
func TestCloseAssembly_StopsTheVersionSweep(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", time.Hour.String())

	before := settledSweepCount()
	base := newSharedBase(t)
	origin := assembleSQLiteTenant(t, base, "reload")
	successor, err := base.ForReassembly(origin).Assemble(
		origin.Store(), origin.Searcher(), origin.VisibleSearcher(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = successor.Close() })
	requireSweeps(t, before+2, "origin and successor each start a sweep")

	origin.CloseAssembly()
	requireSweeps(t, before+1, "CloseAssembly left the superseded sweep running")
}
