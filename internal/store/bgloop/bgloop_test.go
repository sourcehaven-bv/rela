package bgloop_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store/bgloop"
)

func TestNew_RejectsNilRun(t *testing.T) {
	l, err := bgloop.New(nil)
	if err == nil || l != nil {
		t.Fatalf("New(nil) = %v, %v; want nil, error", l, err)
	}
}

// TestLoop_Lifecycle pins the contract the store sweeps rely on: run starts
// once however often Start is called, Stop waits for it to return, and a
// stopped loop never starts again.
func TestLoop_Lifecycle(t *testing.T) {
	var runs, returned atomic.Int32
	running := make(chan struct{}, 2)
	l, err := bgloop.New(func(ctx context.Context) {
		runs.Add(1)
		running <- struct{}{}
		<-ctx.Done()
		returned.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}

	l.Start()
	l.Start()
	<-running
	// A second goroutine, if Start launched one, may not be scheduled yet.
	// Give it time to show up; a correct Loop never produces one.
	time.Sleep(50 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("two Starts ran run %d times, want 1", got)
	}

	l.Stop()
	if got := returned.Load(); got != 1 {
		t.Fatalf("Stop returned before run did: returned = %d", got)
	}
	l.Stop()
	l.Start()
	time.Sleep(50 * time.Millisecond)

	if got := runs.Load(); got != 1 {
		t.Fatalf("run called %d times, want 1", got)
	}
}

func TestLoop_ConcurrentStopWaitsForRun(t *testing.T) {
	var returned atomic.Bool
	started := make(chan struct{})
	l, err := bgloop.New(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		returned.Store(true)
	})
	if err != nil {
		t.Fatal(err)
	}
	l.Start()
	<-started

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			l.Stop()
			if !returned.Load() {
				t.Error("a concurrent Stop returned before run did")
			}
		})
	}
	wg.Wait()
}

func TestLoop_StopWithoutStart(t *testing.T) {
	ran := make(chan struct{}, 1)
	l, err := bgloop.New(func(context.Context) { ran <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	l.Stop()
	l.Start()
	select {
	case <-ran:
		t.Fatal("a loop stopped before Start ran anyway")
	default:
	}

	var nilLoop *bgloop.Loop
	nilLoop.Stop()
}
