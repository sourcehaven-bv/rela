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
// second start while a round runs is skipped, and the round ends with the
// lead.
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
	require.False(t, w.startOutputs(ctx, &rounds), "one round at a time")

	cancel()
	rounds.Wait()
	require.Equal(t, int32(1), round.runs.Load())
	require.False(t, w.outputsBusy.Load(), "a round that ended frees the next")
	ctx2, cancel2 := context.WithCancel(context.Background())
	require.True(t, w.startOutputs(ctx2, &rounds))
	<-round.started
	cancel2()
	rounds.Wait()
}
