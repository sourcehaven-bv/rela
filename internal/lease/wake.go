package lease

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Hub fans a wake-up out to every subscriber. It is its own type so that
// the services of successive assemblies over one store can share it: a
// subscriber then keeps hearing about changes after a schema reload rebuilt
// the service. Safe for concurrent use.
type Hub struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

// NewHub returns a hub without subscribers.
func NewHub() *Hub { return &Hub{subs: map[*subscriber]struct{}{}} }

// subscriber is one Subscribe call. Its channel has capacity one, so wakes
// that arrive before the subscriber reads coalesce into one.
type subscriber struct {
	ch chan struct{}
}

func (h *Hub) subscribe() (wake <-chan struct{}, cancel func()) {
	sub := &subscriber{ch: make(chan struct{}, 1)}
	h.mu.Lock()
	h.subs[sub] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return sub.ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, sub)
			h.mu.Unlock()
		})
	}
}

func (h *Hub) wake() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs {
		select {
		case sub.ch <- struct{}{}:
		default:
		}
	}
}

// Subscribe returns a channel that receives a value whenever leases may have
// changed: a claim, a status change, a release, a rename or delete hook, a
// write seen by [Service.Watch], or an expiry. Wakes coalesce: a subscriber
// that has not read yet gets one value for any number of changes. The
// subscriber reads the leases it shows again on each wake.
//
// cancel stops further wakes and may be called more than once. The channel is
// not closed, so a receiver also selects on its own done signal.
func (s *Service) Subscribe() (wake <-chan struct{}, cancel func()) {
	return s.hub.subscribe()
}

// Watch wakes subscribers for changes this Service does not make itself:
// writes by other processes, seen by polling [Store.Version], and expiries,
// by a timer at the earliest expiry the store reports. It returns nil when
// ctx is cancelled; the caller's ctx owns its lifetime.
//
// Long-running hosts start it once (rela-server, the desktop app, `rela
// mcp`); one-shot CLI commands do not. A failed poll is logged and retried at
// the next interval.
//
// The timer is armed from [Store.NextExpiry]. It is armed again after every
// version change, and on every tick while it is not armed: after a failed
// NextExpiry read, and on the first version read that succeeds after the
// baseline read failed, since the timer may then miss changes made before.
//
// A plain renew moves no version, so the timer may fire for a lease that was
// extended since. When it fires, the watcher asks the store again whether a
// lease that was live when the timer was armed has expired by now, and wakes
// only then. One case reads as no lapse although one happened: a write in
// another process that removes the expired lease in the moment between its
// expiry and the timer's read. Subscribers then learn of it at the next
// change.
func (s *Service) Watch(ctx context.Context) error {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	w := &expiryWatch{s: s, timer: time.NewTimer(time.Hour)}
	w.timer.Stop()
	defer w.timer.Stop()

	version, haveVersion := s.version(ctx)
	w.arm(ctx, s.clock.Now())
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.timer.C:
			at := s.clock.Now()
			if at.Before(w.target) {
				at = w.target // the timer has waited out the delay
			}
			if w.lapsed(ctx, at) {
				s.hub.wake()
			}
			w.arm(ctx, at)
		case <-ticker.C:
			v, ok := s.version(ctx)
			changed := ok && haveVersion && v != version
			first := ok && !haveVersion
			if ok {
				version, haveVersion = v, true
			}
			if changed {
				s.hub.wake()
			}
			if changed || first || !w.armed {
				w.arm(ctx, s.clock.Now())
			}
		}
	}
}

// version reads the store version, logging a failure.
func (s *Service) version(ctx context.Context) (uint64, bool) {
	v, err := s.store.Version(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("lease: watch could not read the lease version", "error", err)
		}
		return 0, false
	}
	return v, true
}

// expiryWatch is the expiry timer of one [Service.Watch] run.
type expiryWatch struct {
	s     *Service
	timer *time.Timer
	// armed reports that the last arm read the store. The timer is stopped
	// when it is false, or when no lease was live.
	armed bool
	// from is the instant the timer was last armed at.
	from time.Time
	// target is the expiry the timer fires for.
	target time.Time
}

// arm sets the timer to fire at the earliest expiry of a lease live at now,
// or stops it when there is none. A failed read leaves the timer stopped and
// armed false.
func (w *expiryWatch) arm(ctx context.Context, now time.Time) {
	w.timer.Stop()
	next, ok, err := w.s.store.NextExpiry(ctx, now)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("lease: watch could not read the next expiry", "error", err)
		}
		w.armed = false
		return
	}
	w.armed, w.from = true, now
	if ok {
		w.target = next
		w.timer.Reset(max(next.Sub(now), 0))
	}
}

// lapsed reports whether a lease that was live when the timer was armed has
// expired by at. A failed read answers true: a needless wake costs a re-read,
// a missed one leaves a lapsed lease on screen.
func (w *expiryWatch) lapsed(ctx context.Context, at time.Time) bool {
	next, ok, err := w.s.store.NextExpiry(ctx, w.from)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("lease: watch could not read the next expiry", "error", err)
		}
		return true
	}
	return ok && !next.After(at)
}
