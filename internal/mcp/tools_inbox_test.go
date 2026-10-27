package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// inboxTestYAML is channelTestYAML plus ask: one per requirement, whose
// asked state waits for the lead. The test metamodel has no owner
// property, so priority holds the lead's login. An ask moves the session
// to asked; an answer by the lead moves it back; text moves nothing.
var inboxTestYAML = channelTestYAML + `  ask:
    attach_to: [requirement]
    read: { from: attached }
    roles:
      lead:   { from: entity.priority, label: the lead }
      member: { from: attached }
    message_types:
      text:   { properties: { body: { type: string } }, post: [member] }
      ask:    { properties: { note: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: working
      states:
        working: { on: { ask: { to: asked } } }
        asked:
          label: Asked
          waiting_for: [lead]
          on:
            answer: { to: working }
`

// useInbox swaps in inboxTestYAML.
func (f *channelFixture) useInbox(t *testing.T) {
	t.Helper()
	compiled, err := realtimecfg.Parse([]byte(inboxTestYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
}

// setPriority makes login the lead of a requirement, through the store.
func (f *channelFixture) setPriority(t *testing.T, id, login string) {
	t.Helper()
	ctx := context.Background()
	e, err := f.st.GetEntity(ctx, entity.Ref{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	e.SetString("priority", login)
	if err := f.st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
}

// askAsBob posts typ in the ask channel of anchor as bob through svc.
func (f *channelFixture) askAsBob(t *testing.T, svc *realtime.Service, anchor, typ string) {
	t.Helper()
	_, err := svc.Post(f.asBob(t), realtime.RawAddress{Anchor: anchor, Type: "ask"},
		realtime.PostInput{Type: typ, Props: map[string]any{}})
	if err != nil {
		t.Fatalf("post %s in the ask channel of %s as bob: %v", typ, anchor, err)
	}
}

// answerAsAlice posts an answer in the ask channel of anchor as alice.
func (f *channelFixture) answerAsAlice(t *testing.T, anchor string) {
	t.Helper()
	alice := acltest.Bind(principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolMCP}), t, f.d)
	_, err := f.svc.Post(alice, realtime.RawAddress{Anchor: anchor, Type: "ask"},
		realtime.PostInput{Type: "answer", Props: map[string]any{}})
	if err != nil {
		t.Fatalf("answer in the ask channel of %s as alice: %v", anchor, err)
	}
}

// inboxTarget is a watch target on list_inbox.
const inboxTarget = `{"tool":"list_inbox","args":{}}`

// inboxWatchClient connects user to a server whose channel poll runs every
// poll and replays list_inbox on the poll every inboxEvery. noWaker turns
// the in-process subscription off.
func inboxWatchClient(t *testing.T, f *channelFixture, user string, poll, inboxEvery time.Duration, noWaker bool,
) *Server {
	t.Helper()
	d := f.deps(t, false)
	d.Watch.ChannelPoll = poll
	if noWaker {
		d.Channels.Waker = nil
	}
	srv, err := NewServer(d, "test", WithPrincipal(principal.Principal{User: user, Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	srv.watch.inboxEvery = inboxEvery
	srv.watch.inboxGap = 50 * time.Millisecond
	return srv
}

// TestListInbox_Answer pins AC 14 for the answer: the service's object
// without links, notice first, and the refusals.
func TestListInbox_Answer(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	f.setPriority(t, "REQ-001", "alice")
	f.askAsBob(t, f.svc, "REQ-001", "ask")
	cs := f.client(t, "alice")

	text := mustCall(t, cs, toolListInbox, map[string]any{}, nil)
	if !noticeFirst(text) {
		t.Errorf("answer does not start with the notice: %s", text)
	}
	if strings.Contains(text, `"links"`) {
		t.Errorf("answer carries links: %s", text)
	}
	var got realtime.InboxView
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || len(got.Entries) != 1 {
		t.Fatalf("inbox = %s, want one entry", text)
	}
	e := got.Entries[0]
	if e.Channel.Anchor != "REQ-001" || e.Channel.Type != "ask" || e.Session.State != "asked" ||
		len(e.Roles) != 1 || e.Roles[0] != "lead" || len(e.Transitions) != 1 || e.Transitions[0] != "answer" {

		t.Errorf("entry = %+v", e)
	}

	if text, isErr := call(t, cs, toolListInbox, map[string]any{"limit": 0}); !isErr {
		t.Errorf("limit 0 = %s, want an error", text)
	}
	if text, isErr := call(t, cs, toolListInbox, map[string]any{"limit": 201}); !isErr ||
		strings.Contains(text, channelFailedText) {

		t.Errorf("limit 201 = %s, want the limit error", text)
	}
	if text, isErr := call(t, cs, toolListInbox, map[string]any{"cursor": "nope"}); !isErr ||
		strings.Contains(text, channelFailedText) {

		t.Errorf("forged cursor = %s, want the cursor error", text)
	}
	if bob := mustCall(t, f.client(t, "bob"), toolListInbox, map[string]any{}, &got); got.Count != 0 {
		t.Errorf("bob's inbox = %s, want empty", bob)
	}

	d := f.deps(t, false)
	d.Channels.Inbox = nil
	srv, err := NewServer(d, "test", WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	if toolNames(t, connect(t, srv))[toolListInbox] != nil {
		t.Error("list_inbox registered without an inbox reader")
	}

	d = f.deps(t, false)
	d.Channels = UnavailableChannels("channels are unavailable: test")
	d.Channels.Reader = f.svc // reports realtime on, so the tools register
	srv, err = NewServer(d, "test", WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	if text, isErr := call(t, connect(t, srv), toolListInbox, map[string]any{}); !isErr ||
		text != "channels are unavailable: test" {

		t.Errorf("unavailable = %q", text)
	}

	f.cfg.Store(nil)
	if text, isErr := call(t, cs, toolListInbox, map[string]any{}); !isErr || strings.Contains(text, channelFailedText) {
		t.Errorf("realtime off = %q, want the off text", text)
	}
}

// waitInbox waits up to 15 s for a watch to report the inbox target.
func waitInbox(t *testing.T, done <-chan watchResult, what string) watchResult {
	t.Helper()
	select {
	case got := <-done:
		if len(got.Changed) != 1 || got.Changed[0] != 0 {
			t.Fatalf("%s: watch = %+v, want the inbox changed", what, got)
		}
		return got
	case <-time.After(15 * time.Second):
		t.Fatalf("%s: watch did not return", what)
	}
	return watchResult{}
}

// TestWatch_ListInboxWakes pins AC 14: a watch on list_inbox wakes when a
// session starts waiting on the caller by a post in this process, by an
// entity edit through the store, and by a post in another process, which
// only the poll sees.
func TestWatch_ListInboxWakes(t *testing.T) {
	t.Parallel()
	t.Run("in-process post", func(t *testing.T) {
		t.Parallel()
		f := newChannelFixture(t)
		f.useInbox(t)
		f.setPriority(t, "REQ-001", "alice")
		cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
		base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
		done := waitChanged(cs, watchArgs(base.Cursor, 20, inboxTarget))
		time.Sleep(200 * time.Millisecond)
		f.askAsBob(t, f.svc, "REQ-001", "ask")
		waitInbox(t, done, "post")
	})
	t.Run("entity edit", func(t *testing.T) {
		t.Parallel()
		f := newChannelFixture(t)
		f.useInbox(t)
		f.setPriority(t, "REQ-001", "carol")
		f.askAsBob(t, f.svc, "REQ-001", "ask")
		cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
		base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
		done := waitChanged(cs, watchArgs(base.Cursor, 20, inboxTarget))
		time.Sleep(200 * time.Millisecond)
		f.setPriority(t, "REQ-001", "alice")
		waitInbox(t, done, "handoff")
	})
	t.Run("poll", func(t *testing.T) {
		t.Parallel()
		f := newChannelFixture(t)
		f.useInbox(t)
		f.setPriority(t, "REQ-001", "alice")
		_, other := f.instance(t)
		cs := connect(t, inboxWatchClient(t, f, "alice", 50*time.Millisecond, 200*time.Millisecond, true))
		base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
		done := waitChanged(cs, watchArgs(base.Cursor, 20, inboxTarget))
		time.Sleep(200 * time.Millisecond)
		f.askAsBob(t, other, "REQ-001", "ask")
		waitInbox(t, done, "post elsewhere")
	})
}

// TestWatch_ListInboxNoWakeOnChat pins AC 14: a message that does not move
// a waiting session does not end the watch.
func TestWatch_ListInboxNoWakeOnChat(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	f.setPriority(t, "REQ-001", "alice")
	f.askAsBob(t, f.svc, "REQ-001", "ask")
	cs := connect(t, inboxWatchClient(t, f, "alice", 50*time.Millisecond, 100*time.Millisecond, false))
	base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
	done := waitChanged(cs, watchArgs(base.Cursor, 3, inboxTarget))
	time.Sleep(200 * time.Millisecond)
	f.askAsBob(t, f.svc, "REQ-001", "text")
	if got := <-done; !got.TimedOut || len(got.Changed) != 0 {
		t.Errorf("watch after a chat = %+v, want a timeout", got)
	}
}

// TestWatch_ListInboxDigestPerCaller pins AC 14: the digest is keyed to
// the caller, so another principal's cursor reports the inbox changed.
func TestWatch_ListInboxDigestPerCaller(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	alice := f.client(t, "alice")
	base := mustWatch(t, alice, watchArgs("", 0, inboxTarget))
	if same := mustWatch(t, alice, watchArgs(base.Cursor, 0, inboxTarget)); len(same.Changed) != 0 {
		t.Errorf("alice's own cursor = %+v, want no change", same)
	}
	if other := mustWatch(t, f.client(t, "carol"), watchArgs(base.Cursor, 0, inboxTarget)); len(other.Changed) != 1 {
		t.Errorf("carol with alice's cursor = %+v, want the inbox changed", other)
	}
}

// TestWatch_ListInboxOneTarget pins AC 14: a watch holds one list_inbox
// target.
func TestWatch_ListInboxOneTarget(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useInbox(t)
	cs := f.client(t, "alice")
	other := `{"tool":"list_inbox","args":{"limit":5}}`
	_, err := callWatch(context.Background(), cs, watchArgs("", 0, inboxTarget, other))
	if err == nil || !strings.Contains(err.Error(), "at most one list_inbox") {
		t.Errorf("two inbox targets: err = %v", err)
	}
	if _, err := callWatch(context.Background(), cs, watchArgs("", 0, inboxTarget, watchShowREQ1)); err != nil {
		t.Errorf("an inbox target with another target: %v", err)
	}
}

// slowInbox answers at once until slow is set, then waits for ctx.
type slowInbox struct {
	slow  atomic.Bool
	calls atomic.Int64
}

func (s *slowInbox) Inbox(ctx context.Context, _ realtime.InboxRequest) (realtime.InboxView, error) {
	s.calls.Add(1)
	if s.slow.Load() {
		<-ctx.Done()
		return realtime.InboxView{}, ctx.Err()
	}
	return realtime.InboxView{Notice: realtime.UntrustedNotice, Entries: []realtime.InboxEntry{}}, nil
}

// TestWatch_ListInboxSlowReplayDoesNotSpin pins AC 14: a list_inbox replay
// slower than the replay timeout doubles the poll interval instead of
// rerunning at the next tick.
func TestWatch_ListInboxSlowReplayDoesNotSpin(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	inbox := &slowInbox{}
	d := f.deps(t, false)
	d.Watch.ChannelPoll = 20 * time.Millisecond
	d.Channels.Inbox = inbox
	srv, err := NewServer(d, "test", WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	srv.watch.tick = 10 * time.Millisecond
	srv.watch.replayTimeout = 50 * time.Millisecond
	srv.watch.inboxEvery = 200 * time.Millisecond
	cs := connect(t, srv)
	base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))

	inbox.slow.Store(true)
	inbox.calls.Store(0)
	got, err := callWatch(context.Background(), cs, watchArgs(base.Cursor, 2, inboxTarget))
	if err != nil || !got.TimedOut {
		t.Fatalf("watch = %+v, %v; want a timeout", got, err)
	}
	// Replays at about 0, 0.4 s and 1.2 s; at the 10 ms tick without the
	// backoff it would be about 30.
	if n := inbox.calls.Load(); n < 2 || n > 4 {
		t.Errorf("slow inbox replayed %d times in 2 s, want 2 to 4", n)
	}
	if !slices.Equal(got.Incomplete, []int{0}) {
		t.Errorf("incomplete = %v, want the inbox target, whose result is unknown", got.Incomplete)
	}

	inbox.slow.Store(false)
	got, err = callWatch(context.Background(), cs, watchArgs(base.Cursor, 0, inboxTarget))
	if err != nil || len(got.Incomplete) != 0 {
		t.Errorf("watch after recovery = %+v, %v; want nothing incomplete", got, err)
	}
}

// countingInbox counts the list_inbox replays of a watch.
type countingInbox struct {
	InboxReader
	calls atomic.Int64
}

func (c *countingInbox) Inbox(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error) {
	c.calls.Add(1)
	return c.InboxReader.Inbox(ctx, req)
}

// countingInboxServer is a watch server for alice whose list_inbox replays
// are counted, with no channel poll, wakes paced by gap.
func countingInboxServer(t *testing.T, f *channelFixture, gap time.Duration) (*Server, *countingInbox) {
	t.Helper()
	inbox := &countingInbox{InboxReader: f.svc}
	d := f.deps(t, false)
	d.Watch.ChannelPoll = time.Hour
	d.Channels.Inbox = inbox
	srv, err := NewServer(d, "test", WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	srv.watch.tick = 10 * time.Millisecond
	srv.watch.inboxEvery = time.Hour
	srv.watch.inboxGap = gap
	return srv, inbox
}

// TestWatch_ListInboxWakePacing pins RR rows 6 and 13: a wake for a
// channel type without a waiting state replays no inbox, and wakes for a
// waiting type replay it at most once per gap, the last one held back
// until the gap has passed rather than dropped.
func TestWatch_ListInboxWakePacing(t *testing.T) {
	t.Parallel()
	t.Run("type without waiting state", func(t *testing.T) {
		t.Parallel()
		f := newChannelFixture(t)
		f.useInbox(t)
		f.setPriority(t, "REQ-001", "alice")
		srv, inbox := countingInboxServer(t, f, 0)
		cs := connect(t, srv)
		base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
		done := waitChanged(cs, watchArgs(base.Cursor, 1, inboxTarget))
		time.Sleep(100 * time.Millisecond)
		before := inbox.calls.Load()
		for range 5 {
			f.postAsBob(t, f.svc, "REQ-001", "hi")
			time.Sleep(20 * time.Millisecond)
		}
		if got := <-done; !got.TimedOut {
			t.Errorf("watch = %+v, want a timeout", got)
		}
		if n := inbox.calls.Load() - before; n != 0 {
			t.Errorf("room posts replayed the inbox %d times, want none", n)
		}
	})
	t.Run("gap", func(t *testing.T) {
		t.Parallel()
		f := newChannelFixture(t)
		f.useInbox(t)
		f.setPriority(t, "REQ-001", "alice")
		f.askAsBob(t, f.svc, "REQ-001", "ask")
		srv, inbox := countingInboxServer(t, f, 400*time.Millisecond)
		cs := connect(t, srv)
		base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
		done := waitChanged(cs, watchArgs(base.Cursor, 3, inboxTarget))
		time.Sleep(100 * time.Millisecond)
		before := inbox.calls.Load()
		// Text in the ask channel wakes the watch but moves no session.
		for range 20 {
			f.askAsBob(t, f.svc, "REQ-001", "text")
			time.Sleep(25 * time.Millisecond)
		}
		// The answer comes inside a gap; the held-back replay still sees it.
		f.answerAsAlice(t, "REQ-001")
		waitInbox(t, done, "answer")
		// About 0.6 s of wakes at a 0.4 s gap, plus the held-back replay;
		// at the 10 ms tick without the gap it would be about 20.
		if n := inbox.calls.Load() - before; n < 1 || n > 4 {
			t.Errorf("wakes replayed the inbox %d times, want 1 to 4", n)
		}
	})
}
