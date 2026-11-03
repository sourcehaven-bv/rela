package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"slices"
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
// builds its own channel store, so each attaches its store's endpoint here.
// The newest attachment still attached is used: detaching it falls back to
// the one before, so a reload the host rejects leaves the previous
// assembly's endpoint in use. Sends go to it, and the follower follows it,
// re-following on a swap.
type hintRelay struct {
	mu sync.Mutex
	// attached are the attachments in attach order; the last is current.
	attached []*hintAttachment
	// swapped is closed and replaced whenever the current one changes.
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
	r.attached = append(r.attached, a)
	r.swap()
	return a
}

// detach removes a. When a was current, the newest attachment left takes
// over.
//
// Nil: a is accepted (nothing attached).
func (r *hintRelay) detach(a *hintAttachment) {
	if a == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.Index(r.attached, a)
	if i < 0 {
		return
	}
	wasCurrent := i == len(r.attached)-1
	r.attached = slices.Delete(r.attached, i, i+1)
	if wasCurrent {
		r.swap()
	}
}

// swap wakes the follower to re-follow. r.mu must be held.
func (r *hintRelay) swap() {
	close(r.swapped)
	r.swapped = make(chan struct{})
}

func (r *hintRelay) current() (cur *hintAttachment, swapped <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.attached); n > 0 {
		cur = r.attached[n-1]
	}
	return cur, r.swapped
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
// endpoint until ctx ends, and re-follows after a swap. Each endpoint calls
// gap once it is positioned, which covers what was sent between the two.
// An endpoint that cannot follow is logged, and the relay waits for the
// next swap.
func (r *hintRelay) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	for {
		cur, swapped := r.current()
		if cur != nil {
			if err := r.followOne(ctx, cur, swapped, deliver, gap); err != nil {
				slog.Warn("channels: channel hints unavailable until the next reload or restart; "+
					"other processes' changes show at the next refetch", "error", err)
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

// retireChannelHints sends the hints s's process still has queued, then
// detaches s's endpoint. It runs when an assembly retires, after its
// writers (the job queue, the realtime worker) stopped and while its
// endpoint is still attached, so a short-lived command's last writes reach
// the others. The Local is shared with later assemblies and stays open; its
// sender stops in [Services.Close].
func retireChannelHints(s *Services) {
	if s.realtimeLocal != nil && s.hintAttach != nil {
		ctx, cancel := context.WithTimeout(context.Background(), hintFlushTimeout)
		if err := s.realtimeLocal.Flush(ctx); err != nil {
			slog.Warn("channels: channel hints not flushed", "error", err)
		}
		cancel()
	}
	if s.channelHints != nil {
		s.channelHints.detach(s.hintAttach)
	}
}
