package realtime

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// Follower tuning. hintResyncGap is the shortest time between two resyncs
// a gap or a resync hint causes; extra ones inside it fold into one at its
// end. hintInboxSize bounds the payloads waiting to be applied; past it,
// payloads are dropped and turn into a gap.
const (
	hintResyncGap = 5 * time.Second
	hintInboxSize = 256
)

// hintIn is the receive side of one [Local]: whether a follower runs, and
// the resync limit.
type hintIn struct {
	mu      sync.Mutex
	running bool
	// resyncs counts the resyncs the follower made.
	resyncs atomic.Int64
	// after makes the timer of a deferred resync; tests replace it.
	after func(time.Duration) <-chan time.Time
}

// FollowHints applies the hints src delivers until ctx ends, and returns
// src's error when it cannot follow at all. It runs once per Local: a call
// while another runs returns nil at once, so overlapping assemblies that
// share the Local start one follower.
//
// It resyncs every subscriber once at start, since hints sent before it
// ran are lost. Each payload from another origin is applied as a local
// mark that is never sent again. A gap, and a resync hint, resync every
// subscriber, at most once per 5 s.
func (l *Local) FollowHints(ctx context.Context, src HintSource) error {
	l.in.mu.Lock()
	if l.in.running {
		l.in.mu.Unlock()
		return nil
	}
	l.in.running = true
	l.in.mu.Unlock()
	defer func() {
		l.in.mu.Lock()
		l.in.running = false
		l.in.mu.Unlock()
	}()
	f := &follower{local: l, inbox: make(chan []byte, hintInboxSize), poke: make(chan struct{}, 1)}
	return f.run(ctx, src)
}

// follower is one FollowHints run.
type follower struct {
	local *Local
	inbox chan []byte
	poke  chan struct{}
	// gap is set by the source's goroutine and taken by the run loop.
	gap atomic.Bool

	lastResync time.Time
	deferred   <-chan time.Time
}

// deliver hands payload to the run loop without blocking. A full inbox
// turns into a gap.
func (f *follower) deliver(payload []byte) {
	select {
	case f.inbox <- slices.Clone(payload):
	default:
		f.markGap()
	}
}

// markGap asks the run loop for a resync.
func (f *follower) markGap() {
	f.gap.Store(true)
	select {
	case f.poke <- struct{}{}:
	default:
	}
}

func (f *follower) run(ctx context.Context, src HintSource) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srcDone := make(chan error, 1)
	go func() { srcDone <- src.FollowHints(ctx, f.deliver, f.markGap) }()
	f.resync("follower start")
	for {
		select {
		case <-ctx.Done():
			<-srcDone
			return nil
		case err := <-srcDone:
			return err
		case p := <-f.inbox:
			f.apply(p)
		case <-f.poke:
			if f.gap.Swap(false) {
				f.requestResync("gap")
			}
		case <-f.deferred:
			f.deferred = nil
			f.resync("deferred")
		}
	}
}

// apply decodes and applies one payload. A payload that does not decode is
// a gap; one from this Local is dropped.
func (f *follower) apply(payload []byte) {
	p, err := decodeHints(payload)
	if err != nil {
		slog.Debug("realtime: unusable channel hint; resyncing", "error", err)
		f.requestResync("unusable hint")
		return
	}
	if p.Origin == f.local.origin {
		return
	}
	l := f.local
	for _, h := range p.Hints {
		switch h.Kind {
		case hintResync:
			f.requestResync("resync hint")
		case hintMessage:
			l.markAs(markKindMessage, false, h.Keys...)
		case hintMoved:
			l.markAs(markKindMoved, false, h.Keys...)
		case hintAddressed:
			kind := markKindMessage
			if h.Of == hintMoved {
				kind = markKindMoved
			}
			l.markAddressed(h.Targets, h.Keys[0], kind, false)
		case hintRead:
			l.markFor(channels.Reader{User: h.User, OrgID: h.Org}, false, h.Keys...)
		}
	}
}

// requestResync resyncs now, or at the end of the current resync window
// when one ran less than hintResyncGap ago.
func (f *follower) requestResync(reason string) {
	if f.deferred != nil {
		return
	}
	now := f.local.clock()
	if f.lastResync.IsZero() || now.Sub(f.lastResync) >= hintResyncGap {
		f.resync(reason)
		return
	}
	after := f.local.in.after
	if after == nil {
		after = time.After
	}
	f.deferred = after(hintResyncGap - now.Sub(f.lastResync))
}

// resync marks every subscriber for a full recheck, counts it and logs it.
func (f *follower) resync(reason string) {
	f.lastResync = f.local.clock()
	n := f.local.in.resyncs.Add(1)
	slog.Info("realtime: channel subscribers resync", "reason", reason, "count", n)
	f.local.markResync()
}

// markResync sets the resync flag of every subscriber and wakes it: every
// channel it follows may have changed.
func (l *Local) markResync() {
	l.subMu.Lock()
	subs := make([]*Subscription, 0, len(l.subs))
	for s := range l.subs {
		subs = append(subs, s)
	}
	l.subMu.Unlock()
	for _, s := range subs {
		s.mu.Lock()
		s.resync = true
		clear(s.changed)
		clear(s.reads)
		clear(s.addressed)
		clear(s.durable)
		clear(s.moved)
		s.nReads = 0
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}
