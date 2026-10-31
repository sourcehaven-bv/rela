package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// readerOf is the reader of a test caller.
func readerOf(a *fakeAccess) channels.Reader { return channels.ReaderOf(authorOf(a.p)) }

// roomRead returns the read marker the room on TKT-1 lists for a.
func roomRead(t *testing.T, h *harness, a *fakeAccess) *ChannelRead {
	t.Helper()
	got, err := h.svc.ListAnchor(as(a), "ticket", "TKT-1", "")
	require.NoError(t, err)
	for _, tc := range got.Types {
		for _, c := range tc.Channels {
			if c.Type == "room" {
				return c.Read
			}
		}
	}
	t.Fatal("no room listed")
	return nil
}

func markRead(t *testing.T, h *harness, a *fakeAccess, addr RawAddress, seq *int64) MarkReadResult {
	t.Helper()
	got, err := h.svc.MarkRead(as(a), addr, seq)
	require.NoError(t, err)
	return got
}

func seqPtr(n int64) *int64 { return new(n) }

// TestListAnchor_Read pins that the channel list carries the caller's own
// marker and unread count, and no read without a marker (TKT-2B2AIA
// AC 7): a post marks its author's channel read, and another reader's
// count rises.
func TestListAnchor_Read(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice, bob := user("alice"), user("bob")
	h.post(t, alice, text("one"))
	require.Equal(t, &ChannelRead{Seq: 1}, roomRead(t, h, alice), "a post marks the author's channel read")
	require.Nil(t, roomRead(t, h, bob), "no marker, no read")

	markRead(t, h, bob, room, nil)
	h.post(t, alice, text("two"))
	require.Equal(t, &ChannelRead{Seq: 1, Unread: 1}, roomRead(t, h, bob))
	require.Equal(t, &ChannelRead{Seq: 2}, roomRead(t, h, alice))
}

// TestListAnchor_ReadHiddenType pins that a channel the caller may not
// read carries no read, even when the caller holds a marker on it
// (TKT-2B2AIA AC 9).
func TestListAnchor_ReadHiddenType(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice, bob := user("alice"), user("bob")
	th, err := h.svc.Create(as(bob), "ticket", "TKT-1", "threads", CreateInput{})
	require.NoError(t, err)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "threads", Name: th.Name}
	_, _, err = h.store.SetRead(context.Background(), k, readerOf(alice), 0, h.clock.Now())
	require.NoError(t, err, "a marker held from before the caller lost access")
	h.store.reads.Store(0)
	got, err := h.svc.ListAnchor(as(alice), "ticket", "TKT-1", "")
	require.NoError(t, err)
	for _, tc := range got.Types {
		if tc.Type == "threads" {
			require.Empty(t, tc.Channels, "alice reads no thread bob created")
		}
	}
	require.Zero(t, h.store.reads.Load(), "no stored channel alice may read, no marker read")
}

// TestListAnchor_ReadsChunks pins that the list reads the markers in
// chunks of the read-key bound (TKT-2B2AIA AC 10). The bound is lowered
// to 1 here; production uses [channels.MaxReadKeys].
func TestListAnchor_ReadsChunks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice := user("alice")
	h.svc.bounds.readKeys = 1
	h.post(t, alice, text("one"))
	for range 2 {
		_, err := h.svc.Create(as(alice), "ticket", "TKT-1", "threads", CreateInput{})
		require.NoError(t, err)
	}
	h.store.reads.Store(0)
	got, err := h.svc.ListAnchor(as(alice), "ticket", "TKT-1", "")
	require.NoError(t, err)
	require.Equal(t, int64(3), h.store.reads.Load())
	n := 0
	for _, tc := range got.Types {
		for _, c := range tc.Channels {
			require.NotNil(t, c.Read, "%s/%s", c.Type, c.Name)
			n++
		}
	}
	require.Equal(t, 3, n)
}

// TestHead_Read pins the head's read (TKT-2B2AIA AC 7): set with a
// marker, nil without, and no marker read for an unstored channel.
func TestHead_Read(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice, bob := user("alice"), user("bob")
	h.post(t, alice, text("one"))
	markRead(t, h, bob, room, nil)
	h.post(t, alice, text("two"))
	head, err := h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Equal(t, &ChannelRead{Seq: 1, Unread: 1}, head.Channel.Read)
	head, err = h.svc.Head(as(user("dave")), room)
	require.NoError(t, err)
	require.Nil(t, head.Channel.Read)

	h.store.reads.Store(0)
	head, err = h.svc.Head(as(bob), RawAddress{EntityType: "ticket", Anchor: "TKT-2", Type: "room"})
	require.NoError(t, err)
	require.Nil(t, head.Channel.Read)
	require.Zero(t, h.store.reads.Load(), "an unstored channel makes no marker read")
}

// TestHead_ReadNotAboveLastSeq pins that the head never answers read.seq
// above last_seq: an own post that lands between Head and Reads raises
// both (TKT-2B2AIA AC 7).
func TestHead_ReadNotAboveLastSeq(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	h.post(t, bob, text("one"))
	h.store.beforeReads = func() {
		_, err := h.store.Store.Append(context.Background(), roomKey(), channels.NewMessage{
			Type: "text", Author: authorOf(bob.p), At: h.clock.Now(), Props: map[string]any{"body": "two"},
			MarkRead: true,
		})
		require.NoError(t, err)
	}
	head, err := h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Equal(t, &ChannelRead{Seq: 2}, head.Channel.Read)
	require.Equal(t, int64(2), head.Channel.LastSeq)
}

// TestMarkRead_Service pins Service.MarkRead (TKT-2B2AIA AC 8).
func TestMarkRead_Service(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice := user("alice")
	for i := range 3 {
		h.post(t, alice, text(strconv.Itoa(i)))
		h.clock.Add(2 * time.Second)
	}

	got := markRead(t, h, user("b1"), room, nil)
	require.Equal(t, MarkReadResult{Seq: 3, LastSeq: 3}, got, "no seq marks the head")
	got = markRead(t, h, user("b2"), room, seqPtr(99))
	require.Equal(t, MarkReadResult{Seq: 3, LastSeq: 3}, got, "clamped to the head")
	b3 := user("b3")
	got = markRead(t, h, b3, room, seqPtr(1))
	require.Equal(t, MarkReadResult{Seq: 1, Unread: 2, LastSeq: 3}, got)
	h.clock.Add(2 * time.Second)
	got = markRead(t, h, b3, room, seqPtr(0))
	require.Equal(t, MarkReadResult{Seq: 1, Unread: 2, LastSeq: 3}, got, "a marker never moves back")

	_, err := h.svc.MarkRead(as(user("b4")), room, seqPtr(-1))
	requireDenial(t, err, http.StatusBadRequest, TypeInvalid)

	h.store.setReads.Store(0)
	got = markRead(t, h, user("b5"), RawAddress{EntityType: "ticket", Anchor: "TKT-2", Type: "room"}, nil)
	require.Equal(t, MarkReadResult{}, got, "an unstored channel answers zeros")
	require.Zero(t, h.store.setReads.Load(), "and stores nothing")

	nobody := &fakeAccess{p: principal.Principal{User: principal.Unknown, Tool: principal.ToolDataEntry}, ceiling: true}
	_, err = h.svc.MarkRead(as(nobody), room, nil)
	requireDenial(t, err, http.StatusForbidden, TypeNoPrincipal)

	_, err = h.svc.MarkRead(as(alice), RawAddress{EntityType: "ticket", Anchor: "TKT-404", Type: "room"}, nil)
	require.ErrorIs(t, err, ErrAnchorNotFound)

	topic, err := h.svc.Create(as(user("reader", "chat.read")), "ticket", "TKT-1", "topics", CreateInput{})
	require.NoError(t, err)
	_, err = h.svc.MarkRead(as(user("b6")), RawAddress{
		EntityType: "ticket", Anchor: "TKT-1", Type: "topics", Name: topic.Name,
	}, nil)
	requireDenial(t, err, http.StatusForbidden, TypeReadDenied)

	burst := user("b7")
	for range 2 {
		markRead(t, h, burst, room, nil)
	}
	_, err = h.svc.MarkRead(as(burst), room, nil)
	d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	require.Positive(t, d.RetryAfter)
}

// TestMarkRead_OwnLimiter pins that marks never use up the post limiter
// (TKT-2B2AIA AC 8): marks on as many channels as one identity may hold
// post buckets leave the next post free.
func TestMarkRead_OwnLimiter(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	ctx := context.Background()
	n := realtimelimits.MaxRateBucketsPerIdentity
	for i := range n {
		id := fmt.Sprintf("TKT-%d", 100+i)
		h.anchors.mu.Lock()
		h.anchors.views[id] = AnchorView{ID: id, Type: "ticket"}
		h.anchors.mu.Unlock()
		k := channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"}
		_, err := h.store.Store.Append(ctx, k, channels.NewMessage{
			Type: "text", Author: authorOf(user("alice").p), At: h.clock.Now(), Props: map[string]any{"body": "x"},
		})
		require.NoError(t, err)
	}
	for i := range n {
		markRead(t, h, bob, RawAddress{EntityType: "ticket", Anchor: fmt.Sprintf("TKT-%d", 100+i), Type: "room"}, nil)
	}
	h.post(t, bob, text("still free to post"))
}

// TestMarkRead_WakesOnlyItsReader pins that a stored mark reaches the
// many-reader subscription tagged with its reader and no plain change
// set (TKT-2B2AIA AC 14). A mark that does not move reaches it too, so a
// first marker at 0 adds the channel to the reader's inbox window
// (TKT-9YRRM5 AC 10); it still reaches no plain change set.
func TestMarkRead_WakesOnlyItsReader(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice, bob := user("alice"), user("bob")
	h.post(t, alice, text("one"))
	hub, plain := h.svc.SubscribeLive(), h.svc.Subscribe()
	for _, s := range []*Subscription{hub, plain} {
		s.Take()
		s.TakeReads()
	}
	markRead(t, h, bob, room, nil)
	require.Equal(t, map[channels.Reader][]channels.Key{readerOf(bob): {roomKey()}}, hub.TakeReads())
	require.Empty(t, takeKeys(t, hub))
	require.Empty(t, takeKeys(t, plain))

	h.clock.Add(2 * time.Second)
	markRead(t, h, bob, room, nil)
	require.Equal(t, map[channels.Reader][]channels.Key{readerOf(bob): {roomKey()}}, hub.TakeReads(),
		"a mark that does not move still reaches the reader's own inbox")
	require.Empty(t, takeKeys(t, plain))
}

// TestLocal_MarkForScopesReader pins Local.markFor: an organization is
// part of the reader, plain subscriptions take nothing, and the zero
// reader marks nobody.
func TestLocal_MarkForScopesReader(t *testing.T) {
	t.Parallel()
	l, err := NewLocal(time.Now)
	require.NoError(t, err)
	t.Cleanup(l.Stop)
	a := channels.Reader{User: "alice"}
	b := channels.Reader{User: "alice", OrgID: "acme"}
	hub, plain := l.SubscribeLive(), l.Subscribe()
	l.markFor(a, keyOf("TKT-1"))
	l.markFor(b, keyOf("TKT-2"))
	require.Equal(t, map[channels.Reader][]channels.Key{
		a: {keyOf("TKT-1")}, b: {keyOf("TKT-2")},
	}, hub.TakeReads())
	require.Empty(t, takeKeys(t, plain))
	l.markFor(channels.Reader{}, keyOf("TKT-3"))
	require.Nil(t, hub.TakeReads())
	require.Empty(t, takeKeys(t, plain))
}

// TestLocal_SubscribeLiveReads pins the reads of the live hub's
// subscription: Take returns channel changes only, TakeReads every
// reader's marks tagged by reader, and an overflow of marks turns into one
// resync.
func TestLocal_SubscribeLiveReads(t *testing.T) {
	t.Parallel()
	l, err := NewLocal(time.Now)
	require.NoError(t, err)
	t.Cleanup(l.Stop)
	a := channels.Reader{User: "alice"}
	b := channels.Reader{User: "bob"}
	hub, plain := l.SubscribeLive(), l.Subscribe()
	l.markFor(a, keyOf("TKT-1"))
	l.markFor(b, keyOf("TKT-2"))
	l.mark(keyOf("TKT-3"))
	require.Equal(t, []channels.Key{keyOf("TKT-3")}, takeKeys(t, hub))
	require.Equal(t, map[channels.Reader][]channels.Key{
		a: {keyOf("TKT-1")}, b: {keyOf("TKT-2")},
	}, hub.TakeReads())
	require.Nil(t, hub.TakeReads(), "TakeReads empties the set")
	require.Nil(t, plain.TakeReads(), "a plain subscription takes no marks")
	require.Equal(t, []channels.Key{keyOf("TKT-3")}, takeKeys(t, plain))

	hub.max = 2
	l.markFor(a, keyOf("TKT-1"), keyOf("TKT-2"), keyOf("TKT-3"))
	require.Nil(t, hub.TakeReads(), "an overflow drops the marks")
	keys, resync := hub.Take()
	require.True(t, resync)
	require.Empty(t, keys)
}

// unreadSeed gives alice a marker on the case channel of each id by her
// post, then has bob post once in each of news, the later one newer.
func unreadSeed(t *testing.T, h *harness, ids []string, news ...string) {
	t.Helper()
	alice, bob := user("alice"), user("bob")
	for _, id := range ids {
		h.ticket(id, "zed")
		h.postTo(t, alice, caseAt(id), "chat", props("body", "mine"))
	}
	for _, id := range news {
		h.clock.Add(time.Second)
		h.postTo(t, bob, caseAt(id), "chat", props("body", "news"))
	}
}

// TestInbox_UnreadChannels pins the unread half of the inbox (TKT-2B2AIA
// AC 11): channels with unread messages, newest activity first, with the
// marker and the count; none on a cursor page.
func TestInbox_UnreadChannels(t *testing.T) {
	t.Parallel()
	h := newInboxHarness(t)
	unreadSeed(t, h, []string{"TKT-1", "TKT-2", "TKT-3"}, "TKT-3", "TKT-1")
	got := h.inbox(t, user("alice"), InboxRequest{})
	require.NotNil(t, got.UnreadCount)
	require.Equal(t, 2, *got.UnreadCount)
	require.Len(t, got.UnreadChannels, 2)
	require.Equal(t, "TKT-1", got.UnreadChannels[0].Channel.Anchor, "newest activity first")
	require.Equal(t, InboxUnread{
		Channel: InboxChannel{Anchor: "TKT-3", EntityType: "ticket", AnchorTitle: "Title of TKT-3", Type: "case",
			State: channels.StateOpen},
		Seq: 1, LastSeq: 2, Unread: 1,
	}, got.UnreadChannels[1])
	require.Len(t, got.WakeKeys, 3, "the wake keys are the whole gated window")

	for i := range 3 {
		h.ticket(fmt.Sprintf("TKT-W%d", i), "alice")
		h.stored(t, caseAt(fmt.Sprintf("TKT-W%d", i)))
	}
	first := h.inbox(t, user("alice"), InboxRequest{Limit: 1})
	require.NotEmpty(t, first.NextCursor)
	next := h.inbox(t, user("alice"), InboxRequest{Limit: 1, Cursor: first.NextCursor})
	require.Nil(t, next.UnreadCount, "a cursor page carries no unread fields")
	require.Nil(t, next.UnreadChannels)
}

// TestInbox_UnreadCap pins that the inbox lists at most MaxInboxUnread
// unread channels and counts them all.
func TestInbox_UnreadCap(t *testing.T) {
	t.Parallel()
	h := newInboxHarness(t)
	ids := make([]string, MaxInboxUnread+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("TKT-%d", 100+i)
	}
	unreadSeed(t, h, ids, ids...)
	got := h.inbox(t, user("alice"), InboxRequest{})
	require.Equal(t, MaxInboxUnread+1, *got.UnreadCount)
	require.Len(t, got.UnreadChannels, MaxInboxUnread)
}

// TestInbox_Kinds pins the kinds filter (TKT-2B2AIA AC 11).
func TestInbox_Kinds(t *testing.T) {
	t.Parallel()
	alice := user("alice")
	fresh := func(t *testing.T) *harness {
		t.Helper()
		h := newInboxHarness(t)
		unreadSeed(t, h, []string{"TKT-1"}, "TKT-1")
		h.ticket("TKT-9", "alice")
		h.stored(t, caseAt("TKT-9"))
		h.store.listReads.Store(0)
		h.store.reads.Store(0)
		h.store.waitingLists.Store(0)
		h.holders.calls.Store(0)
		return h
	}

	t.Run("waiting", func(t *testing.T) {
		t.Parallel()
		h := fresh(t)
		got := h.inbox(t, alice, InboxRequest{Kinds: []string{InboxKindWaiting}})
		require.Len(t, got.Entries, 1)
		require.Nil(t, got.UnreadCount)
		require.Zero(t, h.store.listReads.Load())
		require.Zero(t, h.store.reads.Load())
		require.NotContains(t, inboxJSON(t, got), "unread")
	})

	t.Run("unread", func(t *testing.T) {
		t.Parallel()
		h := fresh(t)
		got := h.inbox(t, alice, InboxRequest{Kinds: []string{InboxKindUnread}})
		require.Equal(t, 1, *got.UnreadCount)
		require.Zero(t, h.store.waitingLists.Load())
		require.Zero(t, h.holders.calls.Load())
		var raw map[string]any
		require.NoError(t, json.Unmarshal([]byte(inboxJSON(t, got)), &raw))
		for _, f := range []string{"entries", "count", "truncated", "next_cursor"} {
			require.NotContains(t, raw, f)
		}
		require.Contains(t, raw, "unread_channels")
		require.False(t, got.HasWaiting())
	})

	t.Run("unread zero is sent", func(t *testing.T) {
		t.Parallel()
		h := newInboxHarness(t)
		got := h.inbox(t, alice, InboxRequest{})
		require.Contains(t, inboxJSON(t, got), `"unread_count":0`)
	})

	t.Run("unknown kind", func(t *testing.T) {
		t.Parallel()
		h := fresh(t)
		_, err := h.svc.Inbox(as(alice), InboxRequest{Kinds: []string{"x"}})
		requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
		require.Error(t, CheckInboxKinds([]string{""}))
		require.NoError(t, CheckInboxKinds(nil))
	})

	t.Run("cursor with unread only", func(t *testing.T) {
		t.Parallel()
		h := fresh(t)
		cur := cursorOf(t, inboxCursor{V: 1, Since: h.clock.Now(), ID: "TKT-1", Type: "case"})
		_, err := h.svc.Inbox(as(alice), InboxRequest{Kinds: []string{InboxKindUnread}, Cursor: cur})
		requireDenial(t, err, http.StatusBadRequest, TypeInvalidCursor)
	})

	t.Run("degraded", func(t *testing.T) {
		t.Parallel()
		h := fresh(t)
		h.cfg.set(Snapshot{Config: h.cfg.Load().Config, Degraded: true})
		got := h.inbox(t, alice, InboxRequest{})
		require.Nil(t, got.UnreadCount)
		require.Zero(t, h.store.listReads.Load())
	})
}

// TestInbox_UnreadHiddenChangesNothing pins gate before count (TKT-2B2AIA
// AC 9): markers on channels whose anchor the caller cannot read, whose
// read role they fail, or whose face they cannot read change nothing in
// the answer.
func TestInbox_UnreadHiddenChangesNothing(t *testing.T) {
	t.Parallel()
	alice := user("alice")
	ctx := context.Background()
	plain := newInboxHarness(t)
	unreadSeed(t, plain, []string{"TKT-1"}, "TKT-1")
	mixed := newInboxHarness(t)
	unreadSeed(t, mixed, []string{"TKT-1", "TKT-5"}, "TKT-1", "TKT-5")

	// The anchor of TKT-5 is gone for alice.
	mixed.anchors.mu.Lock()
	delete(mixed.anchors.views, "TKT-5")
	mixed.anchors.mu.Unlock()

	// A secret channel alice cannot read, with a marker from before.
	secret := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "secret"}
	mixed.stored(t, secret)
	sk := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "secret"}
	_, _, err := mixed.store.SetRead(ctx, sk, readerOf(alice), 0, mixed.clock.Now())
	require.NoError(t, err)

	// A face channel on a face alice cannot read.
	mixed.page("PAGE-1", "zed", "zed")
	draft := RawAddress{EntityType: "page", Anchor: "PAGE-1@draft", Type: "facecase"}
	mixed.postTo(t, alice, draft, "chat", props("body", "mine"))
	mixed.postTo(t, user("bob"), draft, "chat", props("body", "news"))
	mixed.anchors.mu.Lock()
	delete(mixed.anchors.views, "PAGE-1@draft")
	mixed.anchors.mu.Unlock()

	base := plain.inbox(t, alice, InboxRequest{})
	withHidden := mixed.inbox(t, alice, InboxRequest{})
	require.Equal(t, inboxJSON(t, base), inboxJSON(t, withHidden))
	require.Equal(t, 1, *withHidden.UnreadCount)
}

// TestInbox_UnreadBudget pins the cost of the unread half (TKT-2B2AIA
// AC 10): one read window listing and one marker read beside the waiting
// half's calls, with one shared anchor batch and one role prime, the same
// at 3 and 12 channels.
func TestInbox_UnreadBudget(t *testing.T) {
	t.Parallel()
	for _, n := range []int{3, 12} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			h := newInboxHarness(t)
			h.seedVisible(t, n)
			ids := make([]string, n)
			for i := range ids {
				ids[i] = fmt.Sprintf("TKT-%d", 300+i)
			}
			unreadSeed(t, h, ids, ids...)
			alice := user("alice")
			h.anchors.anchorsCalls.Store(0)
			h.anchors.calls.Store(0)
			h.store.listReads.Store(0)
			h.store.reads.Store(0)
			h.store.heads.Store(0)
			got := h.inbox(t, alice, InboxRequest{})
			require.Len(t, got.Entries, n)
			require.Equal(t, n, *got.UnreadCount)
			require.Equal(t, int64(1), h.anchors.anchorsCalls.Load())
			require.Zero(t, h.anchors.calls.Load())
			require.Equal(t, int64(1), alice.primeCalls.Load())
			require.Equal(t, int64(1), h.store.listReads.Load())
			require.Equal(t, int64(1), h.store.reads.Load())
			require.Zero(t, h.store.heads.Load())
		})
	}
}

// TestInbox_UnreadSkipsNoUnread pins that messages of a type with
// `unread: false` leave every unread count alone (TKT-O6RC8Z AC 5): three
// progress posts above alice's marker give 0 on the channel list, the
// head, the mark answer and the inbox; one counting post then gives 1.
func TestInbox_UnreadSkipsNoUnread(t *testing.T) {
	t.Parallel()
	yaml := strings.Replace(inboxYAML("asked"),
		"      answer: { properties: { note: { type: string } }, post: [lead, watcher] }\n",
		"      answer: { properties: { note: { type: string } }, post: [lead, watcher] }\n"+
			"      progress: { properties: { body: { type: string } }, post: [member], unread: false }\n", 1)
	h := newHarness(t, withYAML(yaml))
	h.anchors.views = map[string]AnchorView{}
	alice, runner := user("alice"), user("runner")
	unreadSeed(t, h, []string{"TKT-1"})
	for range 3 {
		h.clock.Add(time.Second)
		h.postTo(t, runner, caseAt("TKT-1"), "progress", props("body", "step"))
	}

	check := func(want int) {
		t.Helper()
		list, err := h.svc.ListAnchor(as(alice), "ticket", "TKT-1", "")
		require.NoError(t, err)
		var listed *ChannelRead
		for _, tc := range list.Types {
			for _, c := range tc.Channels {
				if c.Type == "case" {
					listed = c.Read
				}
			}
		}
		require.Equal(t, &ChannelRead{Seq: 1, Unread: want}, listed, "channel list")
		head, err := h.svc.Head(as(alice), caseAt("TKT-1"))
		require.NoError(t, err)
		require.Equal(t, &ChannelRead{Seq: 1, Unread: want}, head.Channel.Read, "head")
		require.Equal(t, want, markRead(t, h, alice, caseAt("TKT-1"), seqPtr(1)).Unread, "mark read")
		got := h.inbox(t, alice, InboxRequest{})
		require.Equal(t, want, *got.UnreadCount, "unread_count")
		require.Len(t, got.UnreadChannels, want, "unread_channels")
		require.Equal(t, want, got.Summary().UnreadChannels, "summary")
	}
	check(0)
	h.clock.Add(time.Second)
	h.postTo(t, runner, caseAt("TKT-1"), "chat", props("body", "ask"))
	check(1)
}
