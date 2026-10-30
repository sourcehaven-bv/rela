package dataentry

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// The inbox as a live shape (TKT-9YRRM5). An inbox subscription, to
// /api/v1/_inbox or /api/v1/_inbox/summary, is replayed and hashed like
// any other; what is new is when the hub replays it.
//
// Its replay reports a wake set on a 200 ([inboxWake]): whose markers,
// whose principal key and which channels can move the answer. A round
// replays the subscription when a durable channel change, a moved marker
// of its reader, or a post that addresses its principal touches that set
// ([roundFilter.selects]). Typing and ephemeral signals are not durable,
// so keystrokes replay no inbox.
//
// Inbox replays run only on the inbox grid. The grid divides wall-clock
// time since the hub's first tick into periods of inboxGrid ticks; the
// first tick of each period is a grid tick. The slot of a tick comes from
// the time the clock sent with it, not from how many ticks ran: a ticker
// drops ticks while a round runs long, and counting finished ticks would
// let a slow round (which a hidden change can cause) shift every later
// grid tick. A subscription a round selects on another tick waits in
// pending for the next grid tick, with no frame of its own.
//
// The grid fixes when an inbox round starts. It does not fix when its
// frames leave: frames go out when the whole round has finished, and how
// long that takes depends on how many replays the round runs, which
// changes the subscriber cannot see also decide. That is the same as for
// every full round a hidden entity edit starts.
//
// An inbox subscription that a round has no room or time for is deferred
// to pending with no fallback frame (see [liveHub.fallback]): its round
// may have been chosen by a change it cannot see, so a refetch frame
// would tell it one happened.
//
// The safety replay replays every inbox subscription that has a wake set
// once per safety period, so a write in another process reaches the badge
// by then. It is staggered: each principal has a fixed slice, from a
// stable hash of its share key, and each grid tick replays the slices
// whose turn it is (and any a skipped grid period missed). So a grid tick
// replays about one slice in safetyPeriod/inboxGrid of the pairs, and the
// summary and page of one principal still share one round and its memo.
// The safety replay also takes each busy list subscription that has a
// wake, on the same slice of its principal, since a session move in
// another process never reaches this process's realtime.Local either (see
// live_busy.go). Which pairs a safety turn takes depends only on the clock
// and the principal, never on a change, so it adds no oracle.
//
// The safety clock (safetyPeriod, safetySlices, safetySlice and
// liveSafetyMax) is therefore shared by both shapes, and so is the grid it
// runs on: a change to either moves the latency and the cost of busy
// boards as well as of inboxes. Only the deferral of an inbox replay to
// the next grid tick is inbox-specific.
//
// The grid is chosen at wiring ([SetInboxGrid]) from the channel store:
// one full inbox round must finish within one grid period. The file
// channel store lists each reader's markers and mentions by scanning
// every channel, so it gets the wider [LiveInboxGridScan].
//
// The summary and the default page of one principal read the same
// [realtime.Service.Inbox] call: a round memo shares it for one round and
// one principal, and is never shared across principals.

const (
	// LiveInboxGrid is the tick spacing of inbox replays over a database
	// channel store: at most every 2 s.
	LiveInboxGrid = 4
	// LiveInboxGridScan is the tick spacing of inbox replays over the
	// file channel store: at most every 4 s.
	LiveInboxGridScan = 8
	// liveSafetyMax bounds the safety replay spacing: 28 ticks is
	// 14 s, within the 15 s the docs promise. The spacing is the largest
	// multiple of the grid at or below it. Busy list subscriptions share
	// it: a move in another process reaches an open board within it.
	liveSafetyMax = 28
)

// SetInboxGrid sets the tick spacing of inbox replays: [LiveInboxGrid] or
// [LiveInboxGridScan], chosen by the wiring from the channel store. Call
// it before [App.StartWatching], like [SetChannels]. A package function
// because App is at its plimsoll method cap.
func SetInboxGrid(a *App, ticks int) error {
	if ticks < 1 || ticks > liveSafetyMax {
		return fmt.Errorf("dataentry: inbox grid %d is outside 1..%d ticks", ticks, liveSafetyMax)
	}
	a.live.inboxGrid = uint64(ticks)
	return nil
}

// safetyPeriod is the tick spacing of the safety replay.
func (h *liveHub) safetyPeriod() uint64 { return liveSafetyMax / h.inboxGrid * h.inboxGrid }

// safetySlices is the number of safety slices: grid periods per safety
// period.
func (h *liveHub) safetySlices() uint64 { return h.safetyPeriod() / h.inboxGrid }

// inboxTick is what one tick does for inbox subscriptions: whether it is
// a grid tick, and which safety slices (a bit each) it replays.
type inboxTick struct {
	grid   bool
	slices uint64
}

// pack encodes t in one word: the grid flag in the top bit, the slices
// below it. liveSafetyMax keeps the slices under 63 bits.
func (t inboxTick) pack() uint64 {
	v := t.slices
	if t.grid {
		v |= 1 << 63
	}
	return v
}

// inboxTickNow is what the tick in progress does for inbox subscriptions.
func (h *liveHub) inboxTickNow() inboxTick {
	v := h.inboxNow.Load()
	return inboxTick{grid: v&(1<<63) != 0, slices: v &^ (1 << 63)}
}

// inboxClock places ticks on the inbox grid by wall-clock time. Worker
// goroutine only.
type inboxClock struct {
	started  bool
	epoch    time.Time // the first tick's time: slot 0
	lastGrid uint64    // the grid period of the last grid tick
}

// tickAt returns what the tick the clock sent at t does, for a grid of
// grid ticks of window each and slices safety slices.
func (c *inboxClock) tickAt(t time.Time, window time.Duration, grid, slices uint64) inboxTick {
	if !c.started {
		c.started, c.epoch, c.lastGrid = true, t, 0
		return inboxTick{grid: true, slices: 1}
	}
	// The nearest slot: tickers run late, not early. A clock that went
	// back counts as slot 0.
	slot := uint64(0)
	if n := (t.Sub(c.epoch) + window/2) / window; n > 0 {
		slot = uint64(n)
	}
	period := slot / grid
	if period <= c.lastGrid {
		return inboxTick{}
	}
	tk := inboxTick{grid: true}
	if period-c.lastGrid >= slices {
		tk.slices = 1<<slices - 1
	} else {
		for p := c.lastGrid + 1; p <= period; p++ {
			tk.slices |= 1 << (p % slices)
		}
	}
	c.lastGrid = period
	return tk
}

// safetySlice is the safety slice of the principal with share key fp.
func (h *liveHub) safetySlice(fp string) uint64 {
	if h.safetySliceOf != nil {
		return h.safetySliceOf(fp) % h.safetySlices()
	}
	f := fnv.New64a()
	_, _ = f.Write([]byte(fp))
	return f.Sum64() % h.safetySlices()
}

// safetyDue reports whether this tick's safety replay takes the inbox or
// busy list subscription s of the principal with share key fp, on the tick
// now. One without a wake is not taken: its last replay was refused, or
// its busy read was not served, so no channel change can move it; a full
// round still replays it. A busy list takes part because a move in another
// process never reaches this process's realtime.Local.
func (h *liveHub) safetyDue(now inboxTick, fp string, s *liveSub) bool {
	if now.slices == 0 {
		return false
	}
	eligible := (s.inbox && s.wake != nil) || s.busyWake != nil
	return eligible && now.slices&(1<<h.safetySlice(fp)) != 0
}

// inboxWake is what an inbox subscription listens to. Its replay reports
// it on a 200 only; it never comes from the client.
type inboxWake struct {
	user         string          // principal key: addressed targets
	reader       channels.Reader // own marker writes
	mentionKinds bool            // kinds ask for mention or reply
	types        map[string]bool // channel types with a waiting state
	keys         map[string]bool // channelTopic of each WakeKey
}

// recordInbox fills the replay's probe, when the request has one, with
// the wake set of the inbox answer v to req. WakeKeys holds at most
// [channels.MaxReadWindow] plus [realtime.MaxInboxMentions] keys, so the
// set is bounded. A summary reads the default request, so it always asks
// for mentions.
func recordInbox(ctx context.Context, v realtime.InboxView, req realtime.InboxRequest) {
	p, _ := ctx.Value(topicProbeKey{}).(*topicProbe)
	if p == nil {
		return
	}
	w := &inboxWake{
		user: v.Reader.User, reader: v.Reader, mentionKinds: asksMentions(req.Kinds),
		types: make(map[string]bool, len(v.WaitingTypes)), keys: make(map[string]bool, len(v.WakeKeys)),
	}
	for _, t := range v.WaitingTypes {
		w.types[t] = true
	}
	for _, k := range v.WakeKeys {
		w.keys[channelTopic(entity.FormatStateRef(k.ID, k.Face), k.Type, k.Name)] = true
	}
	p.inbox = w
}

// asksMentions reports whether kinds ask for the mention or reply kind.
// No kinds ask for every kind.
func asksMentions(kinds []string) bool {
	return len(kinds) == 0 || slices.Contains(kinds, realtime.InboxKindMention) ||
		slices.Contains(kinds, realtime.InboxKindReply)
}

// inboxMemo shares one [realtime.Service.Inbox] call per principal and
// request among the replays of one round. It lives for one round only.
type inboxMemo struct {
	mu    sync.Mutex
	calls map[string]func() (realtime.InboxView, error)
}

type inboxMemoKey struct{}

// withInboxMemo attaches a fresh memo to ctx, for one round.
func withInboxMemo(ctx context.Context) context.Context {
	return context.WithValue(ctx, inboxMemoKey{}, &inboxMemo{calls: map[string]func() (realtime.InboxView, error){}})
}

// inboxFor reads the inbox for req: through the round's memo during a
// live replay, straight from the service otherwise. A memoized call that
// a context error ended, while ctx is still live, is read again: the
// replay that ran it timed out, not this one.
func (h *channelsHandler) inboxFor(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error) {
	m, _ := ctx.Value(inboxMemoKey{}).(*inboxMemo)
	if m == nil {
		return h.inbox.Inbox(ctx, req)
	}
	key := principal.From(ctx).ShareKey() + "\n" + inboxRequestKey(req)
	m.mu.Lock()
	call := m.calls[key]
	if call == nil {
		call = sync.OnceValues(func() (realtime.InboxView, error) { return h.inbox.Inbox(ctx, req) })
		m.calls[key] = call
	}
	m.mu.Unlock()
	v, err := call()
	ctxErr := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	if ctxErr && ctx.Err() == nil {
		return h.inbox.Inbox(ctx, req)
	}
	return v, err
}

// inboxRequestKey is the canonical form of req: the default limit
// written out, kinds sorted without duplicates, and all four kinds the
// same as none.
func inboxRequestKey(req realtime.InboxRequest) string {
	limit := req.Limit
	if limit == 0 {
		limit = realtime.DefaultInboxLimit
	}
	kinds := slices.Clone(req.Kinds)
	slices.Sort(kinds)
	kinds = slices.Compact(kinds)
	if len(kinds) == len(allInboxKinds) {
		kinds = nil
	}
	return strconv.Itoa(limit) + "\n" + strings.Join(kinds, ",") + "\n" + req.Cursor
}

var allInboxKinds = []string{
	realtime.InboxKindWaiting, realtime.InboxKindUnread, realtime.InboxKindMention, realtime.InboxKindReply,
}

// selectsInbox reports whether the round f replays the inbox subscription
// s. One without a wake set is never selected: its last replay was
// refused, so no channel change can move it; a full round still replays
// it. The loops run over the round's durable changes, which
// [realtimelimits.MaxSubscriberChanges] bounds.
func (f *roundFilter) selectsInbox(s *liveSub) bool {
	w := s.wake
	if w == nil {
		return false
	}
	if f.allChannels || len(f.readTopics[w.reader]) > 0 || w.mentionKinds && f.addressed[w.user] {
		return true
	}
	for t := range f.types {
		if w.types[t] {
			return true
		}
	}
	for t := range f.durable {
		if w.keys[t] {
			return true
		}
	}
	return false
}

// addInbox adds the round's durable channel changes and addressed
// principal keys, which select inbox subscriptions.
func (f *roundFilter) addInbox(durable []channels.Key, addressed []string) {
	for _, k := range durable {
		if f.durable == nil {
			f.durable, f.types = map[string]bool{}, map[string]bool{}
		}
		f.durable[channelTopic(entity.FormatStateRef(k.ID, k.Face), k.Type, k.Name)] = true
		f.types[k.Type] = true
	}
	for _, u := range addressed {
		if f.addressed == nil {
			f.addressed = map[string]bool{}
		}
		f.addressed[u] = true
	}
}
