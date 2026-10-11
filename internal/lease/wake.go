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
func (s *Service) Watch(ctx context.Context) error {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	expiry := time.NewTimer(time.Hour)
	expiry.Stop()
	defer expiry.Stop()

	version, haveVersion := s.version(ctx)
	s.arm(ctx, expiry)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-expiry.C:
			s.hub.wake()
			s.arm(ctx, expiry)
		case <-ticker.C:
			v, ok := s.version(ctx)
			if !ok || (haveVersion && v == version) {
				continue
			}
			changed := haveVersion
			version, haveVersion = v, true
			if changed {
				s.hub.wake()
				s.arm(ctx, expiry)
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

// arm sets t to fire at the earliest live expiry, or stops it when there is
// none. A failed read leaves t stopped; the next version change re-arms it.
func (s *Service) arm(ctx context.Context, t *time.Timer) {
	t.Stop()
	now := s.clock.Now()
	next, ok, err := s.store.NextExpiry(ctx, now)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("lease: watch could not read the next expiry", "error", err)
		}
		return
	}
	if ok {
		t.Reset(max(next.Sub(now), 0))
	}
}
