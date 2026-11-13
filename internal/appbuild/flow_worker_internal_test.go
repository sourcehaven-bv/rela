package appbuild

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// fakeDue answers DueRound from a script and records each call.
type fakeDue struct {
	mu      sync.Mutex
	reports []flowengine.DueReport
	err     error
	afters  []flowrun.DueCursor
	users   []string
}

func (f *fakeDue) DueRound(ctx context.Context, after flowrun.DueCursor) (flowengine.DueReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.afters = append(f.afters, after)
	f.users = append(f.users, principal.From(ctx).User)
	i := len(f.afters) - 1
	if i >= len(f.reports) {
		return flowengine.DueReport{}, f.err
	}
	return f.reports[i], nil
}

func (f *fakeDue) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.afters)
}

// fullRound is a round that hit the cap, ending at run n.
func fullRound(n int) flowengine.DueReport {
	return flowengine.DueReport{
		Advanced: flowengine.MaxDuePerRound, More: true,
		Next: flowrun.DueCursor{At: time.Unix(int64(n), 0), ID: flowrun.RunID(fmt.Sprintf("FLR-%d", n))},
	}
}

func TestFlowWorker_TickCap(t *testing.T) {
	tests := []struct {
		name     string
		reports  []flowengine.DueReport
		err      error
		calls    int
		advanced int
	}{
		{
			name:     "stops after flowWorkerRounds full rounds",
			reports:  []flowengine.DueReport{fullRound(1), fullRound(2), fullRound(3), fullRound(4), fullRound(5)},
			calls:    flowWorkerRounds,
			advanced: flowWorkerRounds * flowengine.MaxDuePerRound,
		},
		{
			name:     "stops when a round is not full",
			reports:  []flowengine.DueReport{fullRound(1), {Advanced: 3}},
			calls:    2,
			advanced: flowengine.MaxDuePerRound + 3,
		},
		{
			name:    "stops at an error",
			reports: []flowengine.DueReport{fullRound(1)},
			err:     errors.New("store down"),
			calls:   2,
			// The failed round reports nothing.
			advanced: flowengine.MaxDuePerRound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			due := &fakeDue{reports: tc.reports, err: tc.err}
			w, err := newFlowWorker(due, memoryLeaderLock())
			require.NoError(t, err)
			advanced, _ := w.runTick(context.Background())
			require.Equal(t, tc.advanced, advanced)
			require.Equal(t, tc.calls, due.calls())
			require.Equal(t, flowrun.DueCursor{}, due.afters[0], "a first tick starts at the front")
			for i := 1; i < len(due.afters); i++ {
				require.Equal(t, tc.reports[i-1].Next, due.afters[i], "each round resumes after the last")
			}
		})
	}
}

// TestFlowWorker_NextTickResumes pins that a tick whose last round was
// full leaves the next tick to go on from there, and that a tick whose
// rounds ran out sends the next one back to the front.
func TestFlowWorker_NextTickResumes(t *testing.T) {
	due := &fakeDue{reports: []flowengine.DueReport{
		fullRound(1), fullRound(2), fullRound(3), fullRound(4), // first tick: cap hit
		{Advanced: 1}, // second tick: the rest
	}}
	w, err := newFlowWorker(due, memoryLeaderLock())
	require.NoError(t, err)
	w.runTick(context.Background())
	w.runTick(context.Background())
	w.runTick(context.Background())
	require.Equal(t, fullRound(4).Next, due.afters[4], "the second tick goes on after the first")
	require.Equal(t, flowrun.DueCursor{}, due.afters[5], "the third tick starts at the front")
}

// dueFunc is a dueRounds made of a function.
type dueFunc func(context.Context, flowrun.DueCursor) (flowengine.DueReport, error)

func (f dueFunc) DueRound(ctx context.Context, after flowrun.DueCursor) (flowengine.DueReport, error) {
	return f(ctx, after)
}

// TestFlowWorker_CutRoundResumes pins that a round cut off by its
// deadline ends the tick, and that the next tick goes on after the run it
// cut off rather than starting at that run again.
func TestFlowWorker_CutRoundResumes(t *testing.T) {
	cut := fullRound(7).Next
	var afters []flowrun.DueCursor
	due := dueFunc(func(_ context.Context, after flowrun.DueCursor) (flowengine.DueReport, error) {
		afters = append(afters, after)
		if len(afters) == 1 {
			return flowengine.DueReport{Advanced: 6, Next: cut}, context.DeadlineExceeded
		}
		return flowengine.DueReport{}, nil
	})
	w, err := newFlowWorker(due, memoryLeaderLock())
	require.NoError(t, err)
	advanced, _ := w.runTick(context.Background())
	require.Equal(t, 6, advanced)
	require.Len(t, afters, 1, "a cut round ends the tick")
	w.runTick(context.Background())
	require.Equal(t, []flowrun.DueCursor{{}, cut}, afters)
}

// TestFlowWorker_StopsLeadingWhenTheLockIsLost pins that a leader whose
// lock check fails stops running due rounds.
func TestFlowWorker_StopsLeadingWhenTheLockIsLost(t *testing.T) {
	var mu sync.Mutex
	lost := false
	takes := 0
	lock := func(context.Context) (heldLock, bool, error) {
		mu.Lock()
		defer mu.Unlock()
		takes++
		if takes > 1 {
			return heldLock{}, false, nil // another process took over
		}
		return heldLock{release: func() {}, check: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			if lost {
				return errors.New("session gone")
			}
			return nil
		}}, true, nil
	}
	due := &fakeDue{}
	w, err := newFlowWorker(due, lock)
	require.NoError(t, err)
	w.tick, w.takeover = time.Millisecond, time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runFlowWorkerLoop(ctx, w)
	require.Eventually(t, func() bool { return due.calls() >= 2 }, 5*time.Second, time.Millisecond)

	mu.Lock()
	lost = true
	mu.Unlock()
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return takes > 1
	}, 5*time.Second, time.Millisecond, "the worker went back to trying the lock")
	n := due.calls()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, n, due.calls(), "no rounds without the lock")
	cancel()
	<-done
}

func TestNewFlowWorker_RejectsNil(t *testing.T) {
	_, err := newFlowWorker(nil, memoryLeaderLock())
	require.Error(t, err)
	_, err = newFlowWorker(&fakeDue{}, nil)
	require.Error(t, err)
}

// TestFlowWorker_OnlyTheLeaderRuns pins that of two workers sharing a
// lock, only the holder runs due rounds, as system:flow.
func TestFlowWorker_OnlyTheLeaderRuns(t *testing.T) {
	var mu sync.Mutex
	taken := false
	lock := func(context.Context) (heldLock, bool, error) {
		mu.Lock()
		defer mu.Unlock()
		if taken {
			return heldLock{}, false, nil
		}
		taken = true
		return heldLock{release: func() {
			mu.Lock()
			taken = false
			mu.Unlock()
		}}, true, nil
	}
	first, second := &fakeDue{}, &fakeDue{}
	w1, err := newFlowWorker(first, lock)
	require.NoError(t, err)
	w2, err := newFlowWorker(second, lock)
	require.NoError(t, err)
	for _, w := range []*flowWorker{w1, w2} {
		w.tick, w.takeover = time.Millisecond, time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())
	done1 := runFlowWorkerLoop(ctx, w1)
	require.Eventually(t, func() bool { return first.calls() >= 3 }, 5*time.Second, time.Millisecond)
	done2 := runFlowWorkerLoop(ctx, w2)
	// Let the second worker try the held lock many times.
	n := first.calls()
	require.Eventually(t, func() bool { return first.calls() >= n+20 }, 5*time.Second, time.Millisecond)
	cancel()
	<-done1
	<-done2

	// The first worker's release on cancel frees the lock while the second
	// may still be trying it; a worker whose context ended does not lead.
	require.Zero(t, second.calls(), "the worker without the lock runs nothing")
	for _, u := range first.users {
		require.Equal(t, principal.UserFlow, u)
	}
}

// runFlowWorkerLoop runs w.run in the background and returns a channel closed
// once it returns.
func runFlowWorkerLoop(ctx context.Context, w *flowWorker) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(ctx)
	}()
	return done
}

// memoryLeaderLock is a lock that is always free.
func memoryLeaderLock() leaderLock {
	return func(context.Context) (heldLock, bool, error) { return heldLock{release: func() {}}, true, nil }
}
