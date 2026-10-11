package sqlitestore_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// sweepRunFrame is the frame every running sqlitestore sweep has on its stack.
const sweepRunFrame = "sqlitestore.(*sweep).run"

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

// TestNewVersionSweep_RejectsMissingOrUnusableInputs pins that a sweep that
// could never capture safely is refused at construction rather than logged
// and skipped.
func TestNewVersionSweep_RejectsMissingOrUnusableInputs(t *testing.T) {
	s := open(t)

	// A view carries its own versionMu, so a sweep built over one would not
	// exclude a purge on the root store.
	var view *sqlitestore.Store
	require.NoError(t, s.Tx(t.Context(), func(tx store.Store) error {
		var ok bool
		view, ok = tx.(*sqlitestore.Store)
		require.True(t, ok)
		return nil
	}))

	tests := []struct {
		name     string
		st       *sqlitestore.Store
		provider store.ProjectionProvider
	}{
		{name: "nil store", st: nil, provider: fixedProjection{}},
		{name: "nil provider", st: s, provider: nil},
		{name: "transaction view", st: view, provider: fixedProjection{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sw, err := sqlitestore.NewVersionSweep(tc.st, tc.provider, store.SweepConfig{})
			require.Error(t, err)
			require.Nil(t, sw)
		})
	}
}

// TestVersionSweep_StopEndsCapture pins the lifecycle the wiring site relies
// on: Start runs the sweep once however often it is called, and after Stop
// returns the sweep captures nothing more, even when Start is called again.
func TestVersionSweep_StopEndsCapture(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	sw, err := sqlitestore.NewVersionSweep(s, fixedProjection{}, store.SweepConfig{
		Interval: 20 * time.Millisecond, Idle: time.Nanosecond, MaxStaleness: time.Nanosecond, Batch: 100,
	})
	require.NoError(t, err)
	t.Cleanup(sw.Stop)
	before := sweepGoroutines()
	sw.Start()
	sw.Start()
	require.Equal(t, before+1, sweepGoroutines(), "a second Start must not launch a second goroutine")

	create := func(id string) {
		t.Helper()
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "feature")))
	}
	versions := func(id string) int {
		t.Helper()
		metas, err := s.VersionStore().ListVersions(ctx, entity.Ref{ID: id})
		require.NoError(t, err)
		return len(metas)
	}

	create("FEAT-RUN")
	require.Eventually(t, func() bool { return versions("FEAT-RUN") == 1 },
		3*time.Second, 10*time.Millisecond, "a started sweep must capture a settled entity")

	sw.Stop()
	sw.Stop()
	sw.Start()
	require.Equal(t, before, sweepGoroutines(), "Start after Stop must not run the sweep again")

	create("FEAT-AFTER")
	time.Sleep(200 * time.Millisecond) // ten intervals
	require.Zero(t, versions("FEAT-AFTER"), "a stopped sweep must not capture")
}

// TestVersionSweep_StopWithoutStart pins that teardown needs no knowledge of
// whether the sweep ever started, and that a sweep stopped before Start never
// runs.
func TestVersionSweep_StopWithoutStart(t *testing.T) {
	before := sweepGoroutines()
	sw, err := sqlitestore.NewVersionSweep(open(t), fixedProjection{}, store.SweepConfig{})
	require.NoError(t, err)
	sw.Stop()
	sw.Start()
	require.Equal(t, before, sweepGoroutines(), "a sweep stopped before Start started anyway")

	var nilSweep *sqlitestore.VersionSweep
	nilSweep.Stop()
}
