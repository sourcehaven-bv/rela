package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// channelWatchClient connects alice to a server whose channel poll runs
// every poll.
func channelWatchClient(t *testing.T, f *channelFixture, user string, poll time.Duration) *mcpgo.ClientSession {
	t.Helper()
	d := f.deps(t, false)
	d.Watch.ChannelPoll = poll
	srv, err := NewServer(d, "test", WithPrincipal(principal.Principal{User: user, Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	return connect(t, srv)
}

// readChannelTarget is a watch target on read_channel.
func readChannelTarget(args map[string]any) string {
	raw, _ := json.Marshal(map[string]any{"tool": toolReadChannel, "args": args})
	return string(raw)
}

// headRev reads the channel and returns its next_after_rev.
func headRev(t *testing.T, cs *mcpgo.ClientSession, id, typ string) int64 {
	t.Helper()
	var read struct{ realtime.ReadView }
	mustCall(t, cs, toolReadChannel, map[string]any{"id": id, "channel_type": typ}, &read)
	return *read.NextAfterRev
}

// waitChanged runs a waiting watch in the background and returns its
// result channel.
func waitChanged(cs *mcpgo.ClientSession, args string) <-chan watchResult {
	out := make(chan watchResult, 1)
	go func() {
		got, err := callWatch(context.Background(), cs, args)
		if err != nil {
			got = watchResult{Cursor: "error: " + err.Error()}
		}
		out <- got
	}()
	return out
}

// TestWatch_ReadChannelReturnsOnPost pins AC 7 for a post in the same
// process: the subscription wakes the watch, with the poll out of play.
func TestWatch_ReadChannelReturnsOnPost(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := channelWatchClient(t, f, "alice", time.Hour)
	target := readChannelTarget(map[string]any{
		"id": "REQ-001", "channel_type": "room", "after_rev": headRev(t, cs, "REQ-001", "room"),
	})
	base := mustWatch(t, cs, watchArgs("", 0, target))

	done := waitChanged(cs, watchArgs(base.Cursor, 20, target))
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	postAsBob(t, f.svc, "REQ-001", "hello")
	select {
	case got := <-done:
		if len(got.Changed) != 1 || got.Changed[0] != 0 {
			t.Fatalf("watch = %+v, want target 0 changed", got)
		}
		if waited := time.Since(start); waited > 3*time.Second {
			t.Errorf("watch returned %v after the post", waited)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("watch did not return on a post")
	}
}

// TestWatch_ReadChannelCrossInstancePoll pins AC 7 for a post by another
// service instance over the same store: only the poll sees it.
func TestWatch_ReadChannelCrossInstancePoll(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	_, other := f.instance(t)
	cs := channelWatchClient(t, f, "alice", 100*time.Millisecond)
	target := readChannelTarget(map[string]any{
		"id": "REQ-001", "channel_type": "room", "after_rev": headRev(t, cs, "REQ-001", "room"),
	})
	base := mustWatch(t, cs, watchArgs("", 0, target))

	done := waitChanged(cs, watchArgs(base.Cursor, 20, target))
	time.Sleep(200 * time.Millisecond)
	postAsBob(t, other, "REQ-001", "from elsewhere")
	select {
	case got := <-done:
		if len(got.Changed) != 1 {
			t.Fatalf("watch = %+v, want target 0 changed", got)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("watch did not see a post by another instance")
	}
}

// TestWatch_ReadChannelBaselineNotEmpty pins that a target that follows by
// after_rev, with messages already past its after_rev at baseline, is
// reported at once: a post between the agent's read and its watch is not
// lost.
func TestWatch_ReadChannelBaselineNotEmpty(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := channelWatchClient(t, f, "alice", time.Hour)
	rev := headRev(t, cs, "REQ-001", "room")
	postAsBob(t, f.svc, "REQ-001", "between read and watch")

	target := readChannelTarget(map[string]any{"id": "REQ-001", "channel_type": "room", "after_rev": rev})
	got := mustWatch(t, cs, watchArgs("", 0, target))
	if len(got.Changed) != 1 {
		t.Fatalf("baseline = %+v, want the target reported", got)
	}
	again := mustWatch(t, cs, watchArgs(got.Cursor, 0, target))
	if len(again.Changed) != 0 {
		t.Errorf("the same target with its cursor = %+v, want unchanged", again)
	}

	newest := readChannelTarget(map[string]any{"id": "REQ-001", "channel_type": "room"})
	if got := mustWatch(t, cs, watchArgs("", 0, newest)); len(got.Changed) != 0 {
		t.Errorf("a newest-page target at baseline = %+v, want unchanged", got)
	}
}

// TestWatch_ReadChannelHiddenActivity pins AC 8: watches on a hidden
// anchor, on a read-denied type and on a named channel of a hidden anchor
// time out unchanged, with equal digests, while bob posts, edits and
// removes there.
func TestWatch_ReadChannelHiddenActivity(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	thread, err := f.svc.Create(asBob(), "", "DEC-001", "threads", realtime.CreateInput{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	cs := channelWatchClient(t, f, "alice", 100*time.Millisecond)
	targets := []string{
		readChannelTarget(map[string]any{"id": "DEC-001", "channel_type": "room", "after_rev": 0}),
		readChannelTarget(map[string]any{"id": "REQ-001", "channel_type": "secret", "after_rev": 0}),
		readChannelTarget(map[string]any{"id": "DEC-001", "channel_type": "threads", "name": thread.Name}),
	}
	base := mustWatch(t, cs, watchArgs("", 0, targets...))
	if len(base.Changed) != 0 {
		t.Fatalf("baseline = %+v", base)
	}

	done := waitChanged(cs, watchArgs(base.Cursor, 2, targets...))
	for _, at := range []struct{ id, typ, name string }{
		{"DEC-001", "room", ""}, {"REQ-001", "secret", ""}, {"DEC-001", "threads", thread.Name},
	} {
		addr := realtime.RawAddress{Anchor: at.id, Type: at.typ, Name: at.name}
		got, err := f.svc.Post(asBob(), addr, realtime.PostInput{Type: "text", Props: map[string]any{"body": "x"}})
		if err != nil {
			t.Fatalf("post %v: %v", addr, err)
		}
		if at.typ == "room" {
			if _, err := f.svc.Edit(asBob(), addr, got.Message.Seq, map[string]any{"body": "y"}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.Delete(asBob(), addr, got.Message.Seq, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	got := <-done
	if !got.TimedOut || len(got.Changed) != 0 || got.Cursor != base.Cursor {
		t.Errorf("watch on hidden channels = %+v, want a timeout with the baseline cursor %s", got, base.Cursor)
	}
}

// TestReadChannel_Deterministic pins that read_channel is a function of
// visible state: two reads with nothing changed in between, a second
// apart, answer the same bytes, live signal included.
func TestReadChannel_Deterministic(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	postAsBob(t, f.svc, "REQ-001", "hello")
	if _, err := f.svc.Post(asBob(), realtime.RawAddress{Anchor: "REQ-001", Type: "room"},
		realtime.PostInput{Type: "typing"}); err != nil {
		t.Fatal(err)
	}
	cs := f.client(t, "alice")
	args := map[string]any{"id": "REQ-001", "channel_type": "room", "signals": true}
	first := mustCall(t, cs, toolReadChannel, args, nil)
	time.Sleep(1100 * time.Millisecond)
	if second := mustCall(t, cs, toolReadChannel, args, nil); second != first {
		t.Errorf("read_channel moved with nothing changed:\n%s\n%s", first, second)
	}
}

// TestWatch_ChannelTargetsCountTowardCaps pins AC 9: read_channel targets
// count toward the target cap.
func TestWatch_ChannelTargetsCountTowardCaps(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	targets := make([]string, 0, watchMaxTargets+1)
	for i := range watchMaxTargets + 1 {
		targets = append(targets, readChannelTarget(map[string]any{
			"id": "REQ-001", "channel_type": "room", "after_rev": i,
		}))
	}
	_, err := callWatch(context.Background(), cs, watchArgs("", 0, targets...))
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("at most %d", watchMaxTargets)) {
		t.Errorf("17 channel targets = %v, want the cap", err)
	}
}

// TestWatch_PollReplaysOnlyChannelTargets pins AC 9: a channel-only pass
// replays the read_channel targets and skips the others.
func TestWatch_PollReplaysOnlyChannelTargets(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	srv := f.server(t, "alice", false)
	w := newWatchHandler(srv, nil)
	var entityReplays, channelReplays atomic.Int64
	counting := func(n *atomic.Int64) mcpgo.ToolHandler {
		return func(context.Context, *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			n.Add(1)
			return textResult("same"), nil
		}
	}
	p := principal.Principal{User: "alice", Tool: principal.ToolMCP}
	e, _ := w.acquire(p)
	run := &watchRun{w: w, d: srv.deps(), entry: e, shareKey: p.ShareKey(), digests: map[string]string{},
		targets: []watchTarget{
			{tool: "list_entities", key: "a", handler: counting(&entityReplays)},
			{tool: toolReadChannel, key: "b", handler: counting(&channelReplays), channel: &watchChannel{}},
		}}
	ctx := principal.With(context.Background(), p)
	if _, _, err := run.pass(ctx, time.Now().Add(time.Minute), true); err != nil {
		t.Fatal(err)
	}
	if entityReplays.Load() != 0 || channelReplays.Load() != 1 {
		t.Errorf("channel pass replayed %d entity and %d channel targets, want 0 and 1",
			entityReplays.Load(), channelReplays.Load())
	}
}

// TestWatch_NoPollWithoutChannelTarget pins AC 9: without a read_channel
// target a watch neither subscribes to channels nor polls.
func TestWatch_NoPollWithoutChannelTarget(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	d := f.deps(t, false)
	none := newChannelWake(d, []watchTarget{{tool: "show_entity"}})
	defer none.close()
	if none.sub != nil || none.poll != nil || none.wake != nil {
		t.Error("a watch without a channel target subscribes or polls")
	}
	some := newChannelWake(d, []watchTarget{{tool: toolReadChannel, channel: &watchChannel{anchor: "REQ-001"}}})
	defer some.close()
	if some.sub == nil || some.poll == nil {
		t.Error("a watch with a channel target neither subscribes nor polls")
	}
}

// TestWatchChannel_TargetAndWakeFilter pins the target parsing and the
// key filter of a channel wake.
func TestWatchChannel_TargetAndWakeFilter(t *testing.T) {
	t.Parallel()
	c, replay := channelTarget(map[string]any{
		"id": " REQ-001@draft ", "channel_type": "room", "after_rev": float64(0), "limit": float64(40),
	})
	if c.anchor != "REQ-001" || c.typ != "room" || !c.follows || replay["limit"] != float64(watchChannelLimit) {
		t.Errorf("channel target = %+v, replay args %v", c, replay)
	}
	if _, replay := channelTarget(map[string]any{"limit": float64(2)}); replay["limit"] != float64(2) {
		t.Errorf("a small limit was raised: %v", replay)
	}
	if _, replay := channelTarget(map[string]any{"limit": "x"}); replay["limit"] != "x" {
		t.Errorf("a mistyped limit was replaced: %v", replay)
	}
	named := &watchChannel{anchor: "REQ-001", typ: "threads", name: "a"}
	for _, tc := range []struct {
		id, typ, name string
		want          bool
	}{
		{"REQ-001", "threads", "a", true},
		{"REQ-001", "threads", "b", false},
		{"REQ-002", "threads", "a", false},
		{"REQ-001", "room", "a", false},
	} {
		if got := named.matches(tc.id, tc.typ, tc.name); got != tc.want {
			t.Errorf("matches(%s, %s, %s) = %v", tc.id, tc.typ, tc.name, got)
		}
	}
	if !(&watchChannel{anchor: "REQ-001", typ: "threads"}).matches("REQ-001", "threads", "x") {
		t.Error("a target without a name misses a named channel's wake")
	}
}
