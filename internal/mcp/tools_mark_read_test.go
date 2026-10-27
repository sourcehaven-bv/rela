package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// roomOf is the address arguments of the room on id.
func roomOf(id string) map[string]any { return map[string]any{"id": id, chanTypeArg: "room"} }

// aliceRead returns alice's marker on the room of REQ-001, straight from the
// channel store.
func (f *channelFixture) aliceRead(t *testing.T) (channels.ReadState, bool) {
	t.Helper()
	k := channels.Key{Anchor: channels.Anchor{ID: "REQ-001"}, Type: "room"}
	got, err := f.cs.Reads(context.Background(), channels.Reader{User: "alice"}, []channels.Key{k})
	if err != nil {
		t.Fatal(err)
	}
	st, ok := got[k]
	return st, ok
}

// postAsAlice posts a text message in the room of anchor as alice.
func (f *channelFixture) postAsAlice(t *testing.T, anchor, body string) {
	t.Helper()
	alice := acltest.Bind(principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolMCP}), t, f.d)
	_, err := f.svc.Post(alice, realtime.RawAddress{Anchor: anchor, Type: "room"},
		realtime.PostInput{Type: "text", Props: map[string]any{"body": body}})
	if err != nil {
		t.Fatalf("post in the room of %s as alice: %v", anchor, err)
	}
}

// TestMarkRead_Tool pins TKT-2B2AIA AC 12: mark_read answers the stored
// marker, clamps, never moves back, and refuses a bad seq.
func TestMarkRead_Tool(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	for _, b := range []string{"one", "two", "three"} {
		f.postAsBob(t, f.svc, "REQ-001", b)
	}
	cs := f.client(t, "alice")
	var got realtime.MarkReadResult
	args := roomOf("REQ-001")
	args[chanSeqArg] = 1
	mustCall(t, cs, toolMarkRead, args, &got)
	if want := (realtime.MarkReadResult{Seq: 1, Unread: 2, LastSeq: 3}); got != want {
		t.Fatalf("mark_read seq 1 = %+v, want %+v", got, want)
	}
	mustCall(t, cs, toolMarkRead, roomOf("REQ-001"), &got)
	if want := (realtime.MarkReadResult{Seq: 3, LastSeq: 3}); got != want {
		t.Fatalf("mark_read without seq = %+v, want %+v", got, want)
	}
	args[chanSeqArg] = -1
	if text, isErr := call(t, cs, toolMarkRead, args); !isErr {
		t.Errorf("mark_read seq -1 = %q, want an error", text)
	}
	if text, isErr := call(t, cs, toolMarkRead, map[string]any{"id": "REQ-001", chanTypeArg: "secret"}); !isErr {
		t.Errorf("mark_read on a channel alice cannot read = %q, want an error", text)
	}
	list := mustCall(t, cs, "list_channels", map[string]any{"id": "REQ-001"}, nil)
	if !strings.Contains(list, `"read":{"seq":3,"unread":0}`) {
		t.Errorf("list_channels does not carry the marker: %s", list)
	}
}

// TestReadChannel_DoesNotMoveMarker pins TKT-2B2AIA AC 12: read_channel,
// also when a watch replays it, never moves a marker.
func TestReadChannel_DoesNotMoveMarker(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.postAsBob(t, f.svc, "REQ-001", "one")
	cs := f.client(t, "alice")
	mustCall(t, cs, toolReadChannel, roomOf("REQ-001"), nil)
	target := readChannelTarget(roomOf("REQ-001"))
	base := mustWatch(t, cs, watchArgs("", 0, target))
	mustWatch(t, cs, watchArgs(base.Cursor, 0, target))
	if st, ok := f.aliceRead(t); ok {
		t.Fatalf("a read moved alice's marker to %+v", st)
	}
}

// TestUnavailableChannels_MarkRead pins that the refusal deps refuse
// mark_read with their reason.
func TestUnavailableChannels_MarkRead(t *testing.T) {
	t.Parallel()
	u := UnavailableChannels("channels are unavailable: test")
	if u.Marker == nil {
		t.Fatal("UnavailableChannels has no Marker, so mark_read would not be registered")
	}
	_, err := u.Marker.MarkRead(context.Background(), realtime.RawAddress{}, nil)
	if err == nil || err.Error() != "channels are unavailable: test" {
		t.Errorf("MarkRead = %v, want the reason", err)
	}
}

// TestWatch_ListInboxWakesOnUnread pins TKT-2B2AIA AC 13: a post in a
// channel of the caller's unread window wakes a list_inbox watch.
func TestWatch_ListInboxWakesOnUnread(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	f.postAsAlice(t, "REQ-001", "mine")
	cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
	base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
	done := waitChanged(cs, watchArgs(base.Cursor, 20, inboxTarget))
	time.Sleep(200 * time.Millisecond)
	f.postAsBob(t, f.svc, "REQ-001", "news")
	waitInbox(t, done, "post in a marked channel")
}

// TestWatch_ListInboxNoWakeOutsideWindow pins TKT-2B2AIA AC 13: a post in
// a channel outside the window does not wake the watch, though the window
// holds a channel of the same type.
func TestWatch_ListInboxNoWakeOutsideWindow(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	f.postAsAlice(t, "REQ-001", "mine")
	cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
	base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
	done := waitChanged(cs, watchArgs(base.Cursor, 3, inboxTarget))
	time.Sleep(200 * time.Millisecond)
	f.postAsBob(t, f.svc, "REQ-002", "elsewhere")
	if got := <-done; !got.TimedOut || len(got.Changed) != 0 {
		t.Errorf("watch after a post outside the window = %+v, want a timeout", got)
	}
}

// TestWatch_ListInboxWaitingOnlyIgnoresChat pins TKT-2B2AIA AC 13: with
// kinds ["waiting"], a post in a marked channel does not wake the watch.
func TestWatch_ListInboxWaitingOnlyIgnoresChat(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	f.postAsAlice(t, "REQ-001", "mine")
	target := `{"tool":"list_inbox","args":{"kinds":["waiting"]}}`
	cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
	base := mustWatch(t, cs, watchArgs("", 0, target))
	done := waitChanged(cs, watchArgs(base.Cursor, 3, target))
	time.Sleep(200 * time.Millisecond)
	f.postAsBob(t, f.svc, "REQ-001", "news")
	if got := <-done; !got.TimedOut || len(got.Changed) != 0 {
		t.Errorf("waiting-only watch after a chat = %+v, want a timeout", got)
	}
}

// TestListInbox_Kinds pins the kinds argument of list_inbox.
func TestListInbox_Kinds(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	f.postAsAlice(t, "REQ-001", "mine")
	f.postAsBob(t, f.svc, "REQ-001", "news")
	cs := f.client(t, "alice")
	var got struct {
		UnreadCount *int             `json:"unread_count"`
		Unread      []map[string]any `json:"unread_channels"`
		Entries     []any            `json:"entries"`
	}
	mustCall(t, cs, toolListInbox, map[string]any{"kinds": []string{"unread"}}, &got)
	if got.UnreadCount == nil || *got.UnreadCount != 1 || len(got.Unread) != 1 || got.Entries != nil {
		t.Fatalf("list_inbox unread = %+v", got)
	}
	for _, bad := range []any{[]string{"mention"}, "unread", []any{1}} {
		if text, isErr := call(t, cs, toolListInbox, map[string]any{"kinds": bad}); !isErr {
			t.Errorf("kinds %v = %q, want an error", bad, text)
		}
	}
}
