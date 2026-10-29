package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// reviewYAML adds review, a session type that waits for its lead. The
// fixture's requirement has no owner property, so lead reads priority;
// vault is a session type only bob, who holds chat.read, may read. A
// handoff re-enters waiting and sets priority to the named user; the
// state has no timers, so the re-entry loads.
var reviewYAML = strings.Replace(channelTestYAML, "  chat:\n", `  review:
    attach_to: [requirement]
    read: { from: attached }
    roles:
      lead:   { from: entity.priority, label: the lead }
      member: { from: attached }
    message_types:
      text:    { properties: { body: { type: string } }, post: [member] }
      handoff: { properties: { to: { type: string, required: true } }, post: [lead] }
      done:    { properties: { note: { type: string } }, post: [lead] }
      closing: { properties: { note: { type: string } }, post: [lead], in: [finished] }
    session:
      initial: waiting
      states:
        waiting:
          waiting_for: [lead]
          on:
            handoff:
              to: waiting
              roles: [lead]
              outputs: [ { set: { properties: { priority: "{{message.to}}" } } } ]
            done: { to: finished, roles: [lead] }
        finished: { final: true }
  vault:
    attach_to: [requirement]
    read: { permission: chat.read }
    roles:
      member: { from: attached }
    message_types:
      done: { properties: { note: { type: string } }, post: [member] }
    session:
      initial: sealed
      states:
        sealed: { on: { done: { to: opened } } }
        opened: { final: true }
  chat:
`, 1)

// patchOutputs writes a set through the fixture's entity manager, as the
// poster, so a handoff changes the anchor.
type patchOutputs struct {
	fakeOutputs
	mgr func() entityPatcher
}

// entityPatcher is the entity manager method patchOutputs calls.
type entityPatcher interface {
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
}

func (p *patchOutputs) Write(ctx context.Context, r realtime.OutputRequest) (realtime.OutputWritten, error) {
	if r.Target.Kind != realtimecfg.OutputSet {
		return p.fakeOutputs.Write(ctx, r)
	}
	_, err := p.mgr().PatchEntity(ctx, r.Target.AnchorID, entity.Patch{Properties: r.Props})
	return realtime.OutputWritten{}, err
}

// newReviewFixture is the channel fixture with reviewYAML, outputs that
// patch the anchor, and REQ-001's lead set to alice.
func newReviewFixture(t *testing.T) *channelFixture {
	t.Helper()
	f := newChannelFixture(t)
	compiled, err := realtimecfg.Parse([]byte(reviewYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
	w := &patchOutputs{}
	b := appbuildtest.New(f.meta, appbuildtest.WithStore(f.st), appbuildtest.WithDeclarative(f.d),
		appbuildtest.WithChannels(f.cs, testChannelConfig{p: f.cfg}), appbuildtest.WithChannelOutputs(w))
	t.Cleanup(func() { _ = b.Close() })
	w.mgr = func() entityPatcher { return b.EntityManager() }
	f.bndl, f.svc = b, appbuild.ChannelService(b)
	f.setLead(t, "alice")
	return f
}

// setLead sets REQ-001's priority, the review lead, as bob.
func (f *channelFixture) setLead(t *testing.T, user string) {
	t.Helper()
	_, err := f.bndl.EntityManager().PatchEntity(f.asBob(t), "REQ-001",
		entity.Patch{Properties: map[string]any{"priority": user}})
	if err != nil {
		t.Fatal(err)
	}
}

var reviewArgs = map[string]any{"id": "REQ-001", "channel_type": "review"}

// sessionRead is read_session's result.
type sessionRead struct {
	realtime.SessionRead
}

// TestReadSession pins AC 22: the result shape, the no_session answer and
// the read refusal.
func TestReadSession(t *testing.T) {
	t.Parallel()
	t.Run("shape", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		cs := f.client(t, "alice")
		mustCall(t, cs, "post_message", with(reviewArgs, map[string]any{
			"message_type": "text", "props": map[string]any{"body": "hi"},
		}), nil)
		var got sessionRead
		text := mustCall(t, cs, toolReadSession, reviewArgs, &got)
		if !noticeFirst(text) {
			t.Errorf("result does not lead with the notice: %s", text)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"rev", "last_seq", "messages", "next_after_rev"} {
			if _, ok := raw[key]; ok {
				t.Errorf("result carries %s: %s", key, text)
			}
		}
		if len(raw) != 5 {
			t.Errorf("result keys = %d, want notice, channel, session, transitions, entries: %s", len(raw), text)
		}
		s := got.Session
		want := realtime.SessionChannel{Anchor: "REQ-001", Type: "review", State: realtime.StateOpen}
		if got.Channel != want || s == nil || s.Status != "input_required" || !s.WaitingOnMe ||
			len(s.WaitingFor) != 1 || s.WaitingFor[0].Holders[0] != "alice" {

			t.Fatalf("read_session = %s", text)
		}
		if strings.Join(got.Transitions, ",") != "done,handoff" {
			t.Errorf("transitions = %v, want done and handoff", got.Transitions)
		}
		var carol sessionRead
		mustCall(t, f.client(t, "carol"), toolReadSession, reviewArgs, &carol)
		if carol.Session.WaitingOnMe || len(carol.Transitions) != 0 {
			t.Errorf("carol = %+v, want not waited on and no transitions", carol)
		}
	})
	t.Run("session carries a button", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		y := strings.Replace(reviewYAML, "done: { to: finished, roles: [lead] }",
			"done: { to: finished, roles: [lead], button: { label: Finish, props: { note: shipped } } }", 1)
		compiled, err := realtimecfg.Parse([]byte(y), f.meta, nil)
		if err != nil {
			t.Fatal(err)
		}
		f.cfg.Store(compiled)
		for _, tool := range []string{toolReadSession, toolReadChannel} {
			text := mustCall(t, f.client(t, "alice"), tool, reviewArgs, nil)
			if !strings.Contains(text, `{"message_type":"done","to":"finished","entry":0,`+
				`"button":{"label":"Finish","props":{"note":"shipped"}}}`) {

				t.Errorf("%s does not carry the button: %s", tool, text)
			}
			if !strings.Contains(text, `{"message_type":"done","entry":0}`) {
				t.Errorf("%s does not list the entry for the lead: %s", tool, text)
			}
		}
		var carol sessionRead
		mustCall(t, f.client(t, "carol"), toolReadSession, reviewArgs, &carol)
		if carol.Entries == nil || len(carol.Entries) != 0 {
			t.Errorf("carol's entries = %#v, want an empty list", carol.Entries)
		}
	})
	t.Run("no session", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		cs := f.client(t, "alice")
		var texts []string
		for _, id := range []string{"REQ-001", "REQ-404", "DEC-001"} {
			text, isErr := call(t, cs, toolReadSession, map[string]any{"id": id, "channel_type": "room"})
			if !isErr {
				t.Fatalf("read_session on room of %s = %s, want an error", id, text)
			}
			texts = append(texts, text)
		}
		if !strings.Contains(texts[0], "channel type room has no session") {
			t.Errorf("error = %q, want it to name the type", texts[0])
		}
		if texts[0] != texts[1] || texts[0] != texts[2] {
			t.Errorf("no_session differs by anchor: %q", texts)
		}
	})
	t.Run("may not read", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		cs := f.client(t, "alice")
		args := map[string]any{"id": "REQ-001", "channel_type": "vault"}
		text, isErr := call(t, cs, toolReadSession, args)
		want, wantErr := call(t, cs, toolReadChannel, args)
		if !isErr || !wantErr || text != want {
			t.Errorf("read_session = %q (error %v); read_channel = %q (error %v)", text, isErr, want, wantErr)
		}
	})
}

// TestPostMessage_SessionWrongState pins TKT-QYFLRH AC 4 over MCP: a type
// limited by in: is not in read_channel's actions.post outside its states,
// and post_message answers session_wrong_state with the allowed states
// and the rule. In its state it is offered and posts.
func TestPostMessage_SessionWrongState(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t)
	cs := f.client(t, "alice")
	offered := func() bool {
		t.Helper()
		text := mustCall(t, cs, toolReadChannel, with(reviewArgs, map[string]any{"limit": 1}), nil)
		var v struct {
			Actions struct {
				Post []string `json:"post"`
			} `json:"actions"`
		}
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			t.Fatal(err)
		}
		return slices.Contains(v.Actions.Post, "closing")
	}
	if offered() {
		t.Error("closing is offered in waiting")
	}
	closing := with(reviewArgs, map[string]any{"message_type": "closing", "props": map[string]any{"note": "bye"}})
	text, isErr := call(t, cs, "post_message", closing)
	want := "session_wrong_state: closing may be posted only while the session is in finished; it is in waiting " +
		"(rule: channel_types.review.message_types.closing.in)"
	if !isErr || text != want {
		t.Fatalf("post_message = %q (error %v); want %q", text, isErr, want)
	}
	mustCall(t, cs, "post_message", with(reviewArgs, map[string]any{"message_type": "done"}), nil)
	if !offered() {
		t.Error("closing is not offered in finished")
	}
	mustCall(t, cs, "post_message", closing, nil)
}

// TestReadChannel_UnstoredSessionStable pins the read_channel row of AC
// 14: an unstored channel reads the same bytes at two clock times, with
// no entered_at.
func TestReadChannel_UnstoredSessionStable(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	var mu sync.Mutex
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	local, err := realtime.NewLocal(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	})
	if err != nil {
		t.Fatal(err)
	}
	b := appbuildtest.New(f.meta, appbuildtest.WithStore(f.st), appbuildtest.WithDeclarative(f.d),
		appbuildtest.WithChannelsLocal(f.cs, testChannelConfig{p: f.cfg}, local),
		appbuildtest.WithChannelOutputs(fakeOutputs{}))
	t.Cleanup(func() { _ = b.Close() })
	f.bndl, f.svc = b, appbuild.ChannelService(b)
	cs := f.client(t, "alice")
	args := map[string]any{"id": "REQ-002", "channel_type": "incident"}
	first := mustCall(t, cs, toolReadChannel, args, nil)
	mu.Lock()
	now = now.Add(time.Hour)
	mu.Unlock()
	if second := mustCall(t, cs, toolReadChannel, args, nil); second != first {
		t.Errorf("read_channel of an unstored channel moved:\n%s\n%s", first, second)
	}
	if strings.Contains(first, "entered_at") {
		t.Errorf("unstored channel carries entered_at: %s", first)
	}
}

// readSessionTarget is a watch target on read_session.
func readSessionTarget(args map[string]any) string {
	raw, _ := json.Marshal(map[string]any{"tool": toolReadSession, "args": args})
	return string(raw)
}

// sessionWatchClient connects user to a server over f with a channel poll
// every poll; without waker the watch sees channel changes by poll only.
func sessionWatchClient(
	t *testing.T, f *channelFixture, user string, poll time.Duration, waker bool,
) *mcpgo.ClientSession {
	t.Helper()
	d := f.deps(t, false)
	d.Watch.ChannelPoll = poll
	if !waker {
		d.Channels.Waker = nil
	}
	srv, err := NewServer(d, "test", WithPrincipal(principal.Principal{User: user, Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	return connect(t, srv)
}

// postAs posts typ with props in REQ-001's review as user through svc.
func (f *channelFixture) postAs(t *testing.T, svc *realtime.Service, user, typ string, props map[string]any) {
	t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolMCP})
	ctx = acltest.Bind(ctx, t, f.d)
	if _, err := svc.Post(ctx, realtime.RawAddress{Anchor: "REQ-001", Type: "review"},
		realtime.PostInput{Type: typ, Props: props}); err != nil {
		t.Fatalf("post %s as %s: %v", typ, user, err)
	}
}

// TestWatch_ReadSession pins AC 23.
func TestWatch_ReadSession(t *testing.T) {
	t.Parallel()
	changes := []struct {
		name string
		act  func(t *testing.T, f *channelFixture)
		want func(t *testing.T, s *realtime.SessionView)
	}{
		{"transition", func(t *testing.T, f *channelFixture) {
			t.Helper()
			f.postAs(t, f.svc, "alice", "done", nil)
		}, func(t *testing.T, s *realtime.SessionView) {
			t.Helper()
			if s.Status != "completed" {
				t.Errorf("status = %s, want completed", s.Status)
			}
		}},
		{"handoff", func(t *testing.T, f *channelFixture) {
			t.Helper()
			f.postAs(t, f.svc, "alice", "handoff", map[string]any{"to": "carol"})
		}, func(t *testing.T, s *realtime.SessionView) {
			t.Helper()
			if s.WaitingOnMe || s.WaitingFor[0].Holders[0] != "carol" {
				t.Errorf("session = %+v, want carol waited on", s)
			}
		}},
		{"entity edit", func(t *testing.T, f *channelFixture) {
			t.Helper()
			f.setLead(t, "carol")
		}, func(t *testing.T, s *realtime.SessionView) {
			t.Helper()
			if s.WaitingOnMe {
				t.Errorf("session = %+v, want alice no longer waited on", s)
			}
		}},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newReviewFixture(t)
			cs := sessionWatchClient(t, f, "alice", time.Hour, true)
			f.postAs(t, f.svc, "alice", "text", map[string]any{"body": "stored"})
			target := readSessionTarget(reviewArgs)
			base := mustWatch(t, cs, watchArgs("", 0, target))
			done := waitChanged(cs, watchArgs(base.Cursor, 20, target))
			time.Sleep(200 * time.Millisecond)
			tc.act(t, f)
			select {
			case got := <-done:
				if len(got.Changed) != 1 || got.Changed[0] != 0 {
					t.Fatalf("watch = %+v, want target 0 changed", got)
				}
			case <-time.After(15 * time.Second):
				t.Fatal("watch did not return")
			}
			var read sessionRead
			mustCall(t, cs, toolReadSession, reviewArgs, &read)
			tc.want(t, read.Session)
		})
	}
	t.Run("text post does not wake", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		cs := sessionWatchClient(t, f, "alice", 100*time.Millisecond, true)
		f.postAs(t, f.svc, "alice", "text", map[string]any{"body": "stored"})
		target := readSessionTarget(reviewArgs)
		base := mustWatch(t, cs, watchArgs("", 0, target))
		done := waitChanged(cs, watchArgs(base.Cursor, 1.5, target))
		time.Sleep(200 * time.Millisecond)
		f.postAs(t, f.svc, "alice", "text", map[string]any{"body": "chatter"})
		got := <-done
		if !got.TimedOut || len(got.Changed) != 0 {
			t.Errorf("watch = %+v, want a timeout", got)
		}
	})
	t.Run("unstored channel times out", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		cs := sessionWatchClient(t, f, "alice", 100*time.Millisecond, true)
		target := readSessionTarget(map[string]any{"id": "REQ-002", "channel_type": "review"})
		base := mustWatch(t, cs, watchArgs("", 0, target))
		got := mustWatch(t, cs, watchArgs(base.Cursor, 1, target))
		if !got.TimedOut || len(got.Changed) != 0 {
			t.Errorf("watch = %+v, want a timeout across several polls", got)
		}
	})
	t.Run("another process", func(t *testing.T) {
		t.Parallel()
		f := newReviewFixture(t)
		_, other := f.instance(t)
		cs := sessionWatchClient(t, f, "alice", 100*time.Millisecond, false)
		f.postAs(t, f.svc, "alice", "text", map[string]any{"body": "stored"})
		target := readSessionTarget(reviewArgs)
		base := mustWatch(t, cs, watchArgs("", 0, target))
		done := waitChanged(cs, watchArgs(base.Cursor, 20, target))
		time.Sleep(200 * time.Millisecond)
		f.postAs(t, other, "alice", "done", nil)
		select {
		case got := <-done:
			if len(got.Changed) != 1 {
				t.Fatalf("watch = %+v, want target 0 changed", got)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("watch did not see a move by another instance")
		}
	})
}
