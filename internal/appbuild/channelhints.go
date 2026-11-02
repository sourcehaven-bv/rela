package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// hintFlushTimeout bounds the flush of queued hints when a process closes.
const hintFlushTimeout = 3 * time.Second

// hintEndpoint is a channel backend's hint transport between processes:
// the file journal of the file store, or NOTIFY on the postgres store.
type hintEndpoint interface {
	realtime.HintSender
	realtime.HintSource
}

// hintEndpointFor returns the hint transport for the channel store cs over
// st, or nil when the backend admits one process only (sqlite, memory) and
// so has nobody to tell.
//
// Nil: returned for a backend without one.
func hintEndpointFor(cs channels.Store, st store.Store) hintEndpoint {
	if ep := storeHintsFor(cs, st); ep != nil {
		return ep
	}
	fc, ok := cs.(*filechannels.Store)
	if !ok {
		return nil
	}
	feed, err := filechannels.NewHintFeed(fc)
	if err != nil {
		return nil // an in-memory directory: no other process shares it
	}
	return feed
}

// hintRelay is the hint transport a [realtime.Local] keeps for its whole
// life. A Local outlives the assemblies that share it, and each assembly
// builds its own channel store, so each attaches its store's endpoint here;
// the last one attached is used. Sends go to it, and the follower follows
// it, re-following on a swap.
type hintRelay struct {
	mu  sync.Mutex
	cur *hintAttachment
	// swapped is closed and replaced on every attach and detach.
	swapped chan struct{}
}

// hintAttachment is one assembly's endpoint on a relay.
type hintAttachment struct{ ep hintEndpoint }

// errNoHintEndpoint is a send while no assembly is attached.
var errNoHintEndpoint = errors.New("channels: no hint endpoint attached")

func newHintRelay() *hintRelay { return &hintRelay{swapped: make(chan struct{})} }

// attach makes ep the relay's endpoint and returns the attachment to
// detach.
func (r *hintRelay) attach(ep hintEndpoint) *hintAttachment {
	a := &hintAttachment{ep: ep}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cur = a
	close(r.swapped)
	r.swapped = make(chan struct{})
	return a
}

// detach removes a when it is still the relay's endpoint. A later
// assembly's endpoint stays.
//
// Nil: a is accepted (nothing attached).
func (r *hintRelay) detach(a *hintAttachment) {
	if a == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cur != a {
		return
	}
	r.cur = nil
	close(r.swapped)
	r.swapped = make(chan struct{})
}

func (r *hintRelay) current() (cur *hintAttachment, swapped <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cur, r.swapped
}

// SendHint implements [realtime.HintSender] on the current endpoint.
func (r *hintRelay) SendHint(ctx context.Context, payload []byte) error {
	cur, _ := r.current()
	if cur == nil {
		return errNoHintEndpoint
	}
	return cur.ep.SendHint(ctx, payload)
}

// FollowHints implements [realtime.HintSource]: it follows the current
// endpoint until ctx ends, and re-follows after a swap, with a gap, since
// payloads sent between the two are lost. An endpoint that cannot follow
// is logged, and the relay waits for the next swap.
func (r *hintRelay) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	first := true
	for {
		cur, swapped := r.current()
		if cur != nil {
			if !first {
				gap()
			}
			first = false
			if err := r.followOne(ctx, cur, swapped, deliver, gap); err != nil {
				slog.Warn("channels: channel hints unavailable; other processes' changes show at the next refetch",
					"error", err)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-swapped:
		}
	}
}

// followOne follows cur's endpoint until ctx ends, the relay swaps, or the
// endpoint fails, and returns the endpoint's error.
func (r *hintRelay) followOne(
	ctx context.Context, cur *hintAttachment, swapped <-chan struct{}, deliver func([]byte), gap func(),
) error {
	fctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- cur.ep.FollowHints(fctx, deliver, gap) }()
	select {
	case <-swapped:
		cancel()
		return <-errc
	case err := <-errc:
		return err
	}
}

// FollowChannels runs the channel hint follower of s (see
// [realtime.Local.FollowHints]) until ctx is cancelled, and returns a
// channel closed once it has stopped. Long-running hosts call it next to
// [WatchLeases], so changes other processes make reach this process's live
// subscribers. A Services whose backend has no hints, or whose follower
// already runs, returns a channel that closes at once or with ctx.
func FollowChannels(ctx context.Context, s *Services) <-chan struct{} {
	done := make(chan struct{})
	if s.realtimeLocal == nil || s.channelHints == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		if err := s.realtimeLocal.FollowHints(ctx, s.channelHints); err != nil {
			slog.Warn("channels: hint follower stopped", "error", err)
		}
	}()
	return done
}

// closeChannelHints flushes the hints s's process still has queued, so a
// short-lived command's last write reaches the others, and stops the
// sender. It runs once, from [Services.Close], on its own bounded context,
// as closeJobQueue does.
//
// The directive below is on the declaration because contextcheck reports
// this function at every caller of Services.Close; nolintlint does not see
// that use and calls it unused.
//
//nolint:contextcheck,nolintlint // a close has no request ctx to inherit; the flush is bounded
func closeChannelHints(s *Services) {
	if s.realtimeLocal == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), hintFlushTimeout)
	defer cancel()
	if err := s.realtimeLocal.Close(ctx); err != nil {
		slog.Warn("channels: channel hints not flushed", "error", err)
	}
}
