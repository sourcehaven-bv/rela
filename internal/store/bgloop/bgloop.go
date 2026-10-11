// Package bgloop runs one function on a background goroutine with a
// start-once, stop-and-wait lifecycle.
//
// It exists so the store backends' background services (the version
// reconciliation sweeps in pgstore and sqlitestore) share one implementation
// of the Start/Stop handshake rather than a copy each. It is a leaf: standard
// library only.
package bgloop

import (
	"context"
	"errors"
	"sync"
)

// Loop owns one background goroutine running a caller-supplied function.
//
// The function receives a context that is canceled by [Loop.Stop] and must
// return promptly once it is. A Loop runs at most once: Start after Start, or
// Start after Stop, does nothing. Build a new Loop to run again.
type Loop struct {
	run func(ctx context.Context)

	// lifecycleMu guards started, cancel and done: the Start/Stop handshake,
	// not the work run does.
	lifecycleMu sync.Mutex
	started     bool
	cancel      context.CancelFunc
	done        chan struct{}
}

// New returns a Loop that will call run on its own goroutine once started.
//
// Nil: run is rejected, because a Loop with nothing to run is a wiring fault.
func New(run func(ctx context.Context)) (*Loop, error) {
	if run == nil {
		return nil, errors.New("bgloop: nil run function")
	}
	return &Loop{run: run}, nil
}

// Start launches run on a detached context, independent of any request. Only
// the first call starts it.
func (l *Loop) Start() {
	l.lifecycleMu.Lock()
	defer l.lifecycleMu.Unlock()
	if l.started {
		return
	}
	l.started = true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	l.cancel, l.done = cancel, done
	go func() {
		defer close(done)
		l.run(ctx)
	}()
}

// Stop cancels run's context and waits for run to return. The wait is
// unbounded: run decides how quickly it honors the cancellation.
//
// Safe to call more than once, concurrently, before Start (a later Start
// then does nothing), and on a nil receiver.
func (l *Loop) Stop() {
	if l == nil {
		return
	}
	l.lifecycleMu.Lock()
	l.started = true // a stopped loop never starts later
	cancel, done := l.cancel, l.done
	l.lifecycleMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}
