package appbuild

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// blockingRound is an outputsRound that blocks until its context ends.
type blockingRound struct {
	started chan struct{}
	runs    atomic.Int32
}

func (b *blockingRound) Run(ctx context.Context) (realtime.OutputsReport, error) {
	b.runs.Add(1)
	b.started <- struct{}{}
	<-ctx.Done()
	return realtime.OutputsReport{More: true}, ctx.Err()
}

// TestWorker_OutputsRoundBesideTheLoop pins that a round of the outputs
// runner never blocks the leader loop: startOutputs returns at once, a
// second start while a round runs starts no second goroutine, and the
// round ends with the lead, which drops the rerun the second start asked
// for.
func TestWorker_OutputsRoundBesideTheLoop(t *testing.T) {
	round := &blockingRound{started: make(chan struct{}, 2)}
	w := &realtimeWorker{outputs: round}
	ctx, cancel := context.WithCancel(context.Background())
	var rounds sync.WaitGroup

	returned := make(chan bool)
	go func() { returned <- w.startOutputs(ctx, &rounds) }()
	select {
	case ok := <-returned:
		require.True(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("startOutputs blocked the loop")
	}
	<-round.started
	require.False(t, w.startOutputs(ctx, &rounds), "one runner goroutine at a time")

	cancel()
	rounds.Wait()
	require.Equal(t, int32(1), round.runs.Load(), "a canceled lead runs no rerun")
	require.False(t, w.outputsBusy.Load(), "a round that ended frees the next")
	ctx2, cancel2 := context.WithCancel(context.Background())
	require.True(t, w.startOutputs(ctx2, &rounds))
	<-round.started
	cancel2()
	rounds.Wait()
}

// gatedRound is an outputsRound whose rounds each wait for a release.
type gatedRound struct {
	started chan struct{}
	release chan struct{}
	runs    atomic.Int32
}

func (g *gatedRound) Run(ctx context.Context) (realtime.OutputsReport, error) {
	g.runs.Add(1)
	g.started <- struct{}{}
	select {
	case <-g.release:
		return realtime.OutputsReport{}, nil
	case <-ctx.Done():
		return realtime.OutputsReport{}, ctx.Err()
	}
}

// TestWorker_OutputsWakeDuringRoundReruns pins BUG-LQ0WOC1: a wake that
// arrives while a round runs is not lost. The round runs once more after
// it ends, so a mark written mid-round does not wait for the minute tick.
func TestWorker_OutputsWakeDuringRoundReruns(t *testing.T) {
	round := &gatedRound{started: make(chan struct{}, 3), release: make(chan struct{})}
	w := &realtimeWorker{outputs: round}
	ctx := t.Context()
	var rounds sync.WaitGroup

	require.True(t, w.startOutputs(ctx, &rounds))
	<-round.started
	// Two wakes during the round ask for one more round, not two.
	require.False(t, w.startOutputs(ctx, &rounds))
	require.False(t, w.startOutputs(ctx, &rounds))
	round.release <- struct{}{}

	select {
	case <-round.started:
	case <-time.After(5 * time.Second):
		t.Fatal("a wake during the round did not run another round")
	}
	round.release <- struct{}{}
	rounds.Wait()
	require.Equal(t, int32(2), round.runs.Load())
	require.False(t, w.outputsBusy.Load(), "the rerun frees the next round")
}

// seqRound is an outputsRound that records, for each round, the highest
// wake sequence number published before the round started.
type seqRound struct {
	published *atomic.Int64
	covered   atomic.Int64
	runs      atomic.Int64
}

func (r *seqRound) Run(context.Context) (realtime.OutputsReport, error) {
	r.runs.Add(1)
	seen := r.published.Load()
	for {
		old := r.covered.Load()
		if seen <= old || r.covered.CompareAndSwap(old, seen) {
			break
		}
	}
	return realtime.OutputsReport{}, nil
}

// TestWorker_OutputsNoLostWake stresses the release window of the runner:
// many goroutines wake it at once, and after the runner ends some round
// must have started after the last wake.
func TestWorker_OutputsNoLostWake(t *testing.T) {
	const wakers, wakes = 8, 200
	for range 20 {
		var published atomic.Int64
		round := &seqRound{published: &published}
		w := &realtimeWorker{outputs: round}
		var rounds, callers sync.WaitGroup
		for range wakers {
			callers.Go(func() {
				for range wakes {
					published.Add(1)
					w.startOutputs(context.Background(), &rounds)
				}
			})
		}
		callers.Wait()
		rounds.Wait()
		require.Equal(t, published.Load(), round.covered.Load(), "a wake was lost")
		require.False(t, w.outputsBusy.Load())
		require.LessOrEqual(t, round.runs.Load(), published.Load(), "wakes coalesce")
	}
}
