package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// A busy list as a live shape (TKT-W5ZSYJ). A list subscription whose
// path carries `busy` (a busy subscription) shows each row's busy channels
// of one channel type in `_busy`. A session move that writes no graph
// output starts no full round, so the hub also replays a busy subscription
// when a channel move can change its `_busy`.
//
// Its replay reports a wake on a 200 ([busyWake]): the channel type the
// busy read asked for and the anchors of the rows on the page. The rows
// have passed the row gate and redaction, so a hidden entity is in no
// wake. A busy read that failed with a store error still reports it, since
// the wake does not depend on the answer (see serveRowBusy). A replay
// whose busy read was not served, or whose page is empty, reports none;
// any other status clears it; a replay that did not finish keeps it (see
// applyTopic). The wake never comes from the client.
//
// A topic round replays a busy subscription when the round carries a moved
// key ([realtime.Subscription.TakeMoved]) of the wake's type on an anchor
// of the page, or when the channel subscription lost count. A message-only
// write (a post that moved no session and created no channel, an edit, a
// removal, an action) is not moved, so it replays no board. The replay runs
// as the subscriber and frames only when its own hash moves, so a move on
// a channel the subscriber may not read, on a card it can see, replays the
// board and as a rule sends nothing.
//
// The wake holds every row on the page, not only the rows whose channel
// of the type the subscriber may read. So that rule has one exception,
// an accepted residual. When the board's hash is already stale from a
// move made in another process that no safety turn has caught yet (up to
// one safety period), a move on an unreadable channel of a visible card
// replays the board and the frame for the earlier move leaves at that
// tick. That tells the subscriber when a channel it may not read moved, to
// within one tick, on a card it can see: one bit of timing, never the
// existence of an entity. Narrowing the wake is not done because the read
// role can depend on the channel itself (a creator role), so which anchors
// the subscriber may read is known only for the channels that exist and
// are busy now, not for one that is idle or not created yet. Judging it
// per anchor would add an anchor read and a role prime for every row of
// every busy list request, and would still leave the creator case open.
//
// A busy subscription that a topic round has no room or time for gets no
// fallback frame (see [liveHub.fallback]): a move it may not see may have
// chosen the round, so a refetch frame would tell it one happened. It goes
// to pending and is replayed on a later tick. In a full round it keeps the
// fallback frame, like every list: a full round takes every subscription,
// so neither its selection nor where its cap cuts depends on a channel
// change.
//
// Busy boards also add to the pairs a topic round selects, and moves on
// cards another principal cannot see select them, so they move where a
// topic round's cap cuts. A pair a topic round took only because it was
// pending therefore gets no fallback frame either, whatever its shape:
// otherwise a list or presence pair left over from an earlier cut would
// frame or not depending on those hidden moves.
//
// A syncing busy subscription is marked for a recheck when a round carries
// a moved key of its type, or a resync. It has no wake yet, so the type is
// all there is to match on; the type is configuration, not a secret.
//
// A move made in another process never reaches this process's
// realtime.Local. The inbox safety replay therefore also takes each busy
// subscription with a wake, on its principal's slice (see live_inbox.go),
// which bounds that delay to one safety period.

// busyWake is what a busy list subscription listens to. Its replay
// reports it on a 200 only; it never comes from the client.
type busyWake struct {
	typ string // the channel type the busy read served
	// anchors holds the row ids and id@face addresses on the page: at most
	// two per row, so at most 200 for the largest page.
	anchors map[string]bool
}

// recordBusy fills the replay's probe, when the request has one, with the
// wake of a busy read of channel type typ that was served on rows. An
// empty page records nothing. Each row adds its id, which a family
// channel's key (empty face) matches, and its id@face address, which a
// face channel's key matches; on a type without faces both are the id.
func recordBusy(ctx context.Context, typ string, rows []*entity.Entity) {
	p, _ := ctx.Value(topicProbeKey{}).(*topicProbe)
	if p == nil || len(rows) == 0 {
		return
	}
	w := &busyWake{typ: typ, anchors: make(map[string]bool, 2*len(rows))}
	for _, e := range rows {
		w.anchors[e.ID] = true
		w.anchors[entity.FormatStateRef(e.ID, e.Face)] = true
	}
	p.busy = w
}

// busyAnchor is the anchor of channel key k as a busy wake holds it: the
// id for a family channel, id@face for a face channel.
func busyAnchor(k channels.Key) string {
	if k.Face == "" {
		return k.ID
	}
	return entity.FormatStateRef(k.ID, k.Face)
}

// addMoved adds the round's moved channel keys, by channel type, which
// select busy subscriptions.
func (f *roundFilter) addMoved(keys []channels.Key) {
	for _, k := range keys {
		if f.moved == nil {
			f.moved = map[string]map[string]bool{}
		}
		set := f.moved[k.Type]
		if set == nil {
			set = map[string]bool{}
			f.moved[k.Type] = set
		}
		set[busyAnchor(k)] = true
	}
}

// selectsBusy reports whether the round f replays the busy subscription s.
// One without a wake is never selected: its last replay was refused, or
// its busy read was not served, so no channel move can change it; a full
// round still replays it. The loop runs over the smaller of the wake's
// anchors and the round's anchors of the type.
func (f *roundFilter) selectsBusy(s *liveSub) bool {
	w := s.busyWake
	if w == nil {
		return false
	}
	if f.allChannels {
		return true
	}
	moved := f.moved[w.typ]
	small, large := moved, w.anchors
	if len(w.anchors) < len(moved) {
		small, large = w.anchors, moved
	}
	for a := range small {
		if large[a] {
			return true
		}
	}
	return false
}

// movesBusy reports whether the round f may move the `_busy` of a busy
// list of channel type typ: it lost count, or carries a moved key of typ.
func (f *roundFilter) movesBusy(typ string) bool {
	return f.allChannels || len(f.moved[typ]) > 0
}
