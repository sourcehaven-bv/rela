package pgstore_test

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// sweepRunFrame is the frame every running pgstore sweep has on its stack.
const sweepRunFrame = "pgstore.(*sweep).run"

// sweepGoroutines counts the goroutines in this process currently inside the
// sweep's run loop. Polled until it holds still, because a sweep stopped by an
// earlier test has returned from run but may not have left its last frame yet.
func sweepGoroutines() int {
	count := func() int {
		buf := make([]byte, 1<<20)
		for {
			n := runtime.Stack(buf, true)
			if n < len(buf) {
				return strings.Count(string(buf[:n]), sweepRunFrame)
			}
			buf = make([]byte, 2*len(buf))
		}
	}
	prev := count()
	for range 50 {
		time.Sleep(10 * time.Millisecond)
		n := count()
		if n == prev {
			return n
		}
		prev = n
	}
	return prev
}

// bareHandle is a DBTX that is not a pool, standing in for a store built over
// a single connection or a transaction. Its methods are never called.
type bareHandle struct{ pgstore.DBTX }

// TestNewVersionSweep_RejectsMissingOrUnusableInputs pins that a sweep that
// could never capture is refused at construction. The old StartVersionSweep
// logged and returned, so a misconfigured deployment silently stopped
// recording create/update history.
func TestNewVersionSweep_RejectsMissingOrUnusableInputs(t *testing.T) {
	overBareHandle, err := pgstore.New(bareHandle{})
	require.NoError(t, err)
	provider := stubProvider{hash: "h", json: []byte(`{}`)}

	tests := []struct {
		name     string
		st       *pgstore.Store
		provider pgstore.ProjectionProvider
	}{
		{name: "nil store", st: nil, provider: provider},
		{name: "nil provider", st: overBareHandle, provider: nil},
		{name: "handle is not a pool", st: overBareHandle, provider: provider},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sw, err := pgstore.NewVersionSweep(tc.st, tc.provider, pgstore.SweepConfig{})
			require.Error(t, err)
			require.Nil(t, sw)
		})
	}
}

// TestVersionSweep_StopWithoutStart pins that teardown needs no knowledge of
// whether the sweep ever started: Stop on a nil or unstarted sweep returns, and
// a sweep stopped before Start never runs.
func TestVersionSweep_StopWithoutStart(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	before := sweepGoroutines()
	sw, err := pgstore.NewVersionSweep(s, stubProvider{hash: "h", json: []byte(`{}`)}, pgstore.SweepConfig{})
	require.NoError(t, err)
	sw.Stop()
	sw.Start()
	require.Equal(t, before, sweepGoroutines(), "a sweep stopped before Start started anyway")

	var nilSweep *pgstore.VersionSweep
	nilSweep.Stop()
}

// TestVersionSweep_StopEndsCapture pins the lifecycle the wiring site relies
// on: Start runs the sweep once however often it is called, and after Stop
// returns the sweep captures nothing more, even when Start is called again.
func TestVersionSweep_StopEndsCapture(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	sw, err := pgstore.NewVersionSweep(s,
		stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 20 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})
	require.NoError(t, err)
	t.Cleanup(sw.Stop)
	before := sweepGoroutines()
	sw.Start()
	sw.Start()
	require.Equal(t, before+1, sweepGoroutines(), "a second Start must not launch a second goroutine")

	settle := func(id string) {
		t.Helper()
		require.NoError(t, s.CreateEntity(ctx, mkEntity(id, id)))
		_, err := pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = $1`, id)
		require.NoError(t, err)
	}
	versions := func(id string) int {
		t.Helper()
		metas, err := s.VersionStore().ListVersions(ctx, entity.Ref{ID: id})
		require.NoError(t, err)
		return len(metas)
	}

	settle("RUN-1")
	require.Eventually(t, func() bool { return versions("RUN-1") == 1 },
		3*time.Second, 10*time.Millisecond, "a started sweep must capture a settled entity")

	sw.Stop()
	sw.Stop()
	sw.Start()
	require.Equal(t, before, sweepGoroutines(), "Start after Stop must not run the sweep again")

	settle("AFTER-1")
	time.Sleep(200 * time.Millisecond) // ten intervals
	require.Zero(t, versions("AFTER-1"), "a stopped sweep must not capture")
}
