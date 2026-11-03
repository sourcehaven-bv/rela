package mcp

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// countingComments is a thread that grows when add is called. A comment
// write sends no store event, so only the poll can see it.
type countingComments struct{ n atomic.Int32 }

func (c *countingComments) add() { c.n.Add(1) }

func (c *countingComments) ListComments(context.Context, string) ([]comments.View, error) {
	out := make([]comments.View, c.n.Load())
	for i := range out {
		out[i] = comments.View{ID: string(rune('a' + i)), Author: "alice", Body: "remark"}
	}
	return out, nil
}

const watchCommentsREQ1 = `{"tool":"list_comments","args":{"id":"REQ-001"}}`

// TestWatch_ListCommentsPolled pins TKT-RQKKS9: a list_comments target
// reports a new comment on the poll, though no store event fires.
func TestWatch_ListCommentsPolled(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	thread := &countingComments{}
	deps.Comments.Reader = thread
	const poll = 50 * time.Millisecond
	deps.Watch.ChannelPoll = poll
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	cs := connect(t, srv)

	base := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`,`+watchCommentsREQ1+`]}`)
	const delay = 200 * time.Millisecond
	later(t, delay, func() error { thread.add(); return nil })
	start := time.Now()
	got := mustWatch(t, cs, watchArgs(base.Cursor, 10, watchShowREQ1, watchCommentsREQ1))
	if got.TimedOut || !slices.Equal(got.Changed, []int{1}) {
		t.Fatalf("got %+v, want changed [1]", got)
	}
	if d := time.Since(start); d < delay || d > delay+20*poll {
		t.Fatalf("returned after %v, want shortly after the comment at %v", d, delay)
	}

	again := mustWatch(t, cs, watchArgs(got.Cursor, 0.3, watchShowREQ1, watchCommentsREQ1))
	if !again.TimedOut || len(again.Changed) != 0 {
		t.Fatalf("with the new cursor got %+v, want a timeout", again)
	}
}

// TestWatch_ListCommentsTargetOnlyWhenRegistered: without a comments block,
// list_comments is not a watch target.
func TestWatch_ListCommentsTargetOnlyWhenRegistered(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	deps.Comments = CommentDeps{}
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if slices.Contains(srv.watch.names, toolListCommentsName) {
		t.Fatalf("watch names %v list list_comments without a comment reader", srv.watch.names)
	}
	_, err = callWatch(context.Background(), connect(t, srv), `{"targets":[`+watchCommentsREQ1+`]}`)
	if err == nil {
		t.Fatal("watch accepted a list_comments target with no comment tools")
	}
}

// TestWatchRun_CommentsDueSurvivesMergedPasses pins the review finding on
// TKT-RQKKS9: a channel wake and the poll merge into one pass, and a failed
// comment replay is retried by a passWake, so whether the comment targets
// replay cannot ride on the pass kind.
func TestWatchRun_CommentsDueSurvivesMergedPasses(t *testing.T) {
	t.Parallel()
	srv, _ := serverWithSource(t, &countingSource{})
	w := newWatchHandler(srv, nil)
	v := &varying{}
	v.body.Store("one")
	p := principal.Principal{User: "tester", Tool: principal.ToolMCP}
	e, _ := w.acquire(p)
	run := &watchRun{w: w, d: srv.deps(), entry: e, shareKey: p.ShareKey(), digests: map[string]string{},
		inboxEvery: w.inboxEvery, inboxGap: w.inboxGap,
		targets: []watchTarget{{tool: toolListCommentsName, key: "c", handler: v.handle, comments: true}}}
	ctx := principal.With(context.Background(), p)
	far := time.Now().Add(time.Minute)
	pass := func(kind passKind) {
		t.Helper()
		if _, _, err := run.pass(ctx, far, kind); err != nil {
			t.Fatalf("pass: %v", err)
		}
	}
	replays := func() int64 { return v.replays.Load() }

	pass(passAll)
	if replays() != 1 {
		t.Fatalf("passAll replayed comments %d times, want 1", replays())
	}
	pass(passWake)
	if replays() != 1 {
		t.Fatal("a wake with no poll due replayed the comments")
	}

	run.commentsDue = true // the poll fired; a wake merged the pass
	pass(passWake)
	if replays() != 2 || run.commentsDue {
		t.Fatalf("replays=%d due=%v; want the due comments replayed by the merged pass", replays(), run.commentsDue)
	}

	run.commentsDue = true
	v.fail.Store(1)
	pass(passPoll)
	if !run.commentsDue {
		t.Fatal("a comment replay that did not finish must stay due")
	}
	pass(passWake)
	if replays() != 4 || run.commentsDue {
		t.Fatalf("replays=%d due=%v; want the retry to replay the comments", replays(), run.commentsDue)
	}

	run.commentsDue = true
	pass(passInbox)
	if replays() != 4 || !run.commentsDue {
		t.Fatal("passInbox must leave the comments alone and due")
	}
}

// TestWatch_ListCommentsReplaysOnStoreEvent: an entity edit can detach an
// anchor, so a store event replays a list_comments target too.
func TestWatch_ListCommentsReplaysOnStoreEvent(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	thread := &countingComments{}
	deps.Comments.Reader = thread
	deps.Watch.ChannelPoll = time.Hour
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	cs := connect(t, srv)

	base := mustWatch(t, cs, `{"targets":[`+watchCommentsREQ1+`]}`)
	later(t, 200*time.Millisecond, func() error {
		thread.add()
		return setTitle(context.Background(), st, "REQ-001", "requirement", "Edited")
	})
	got := mustWatch(t, cs, watchArgs(base.Cursor, 10, watchCommentsREQ1))
	if got.TimedOut || !slices.Equal(got.Changed, []int{0}) {
		t.Fatalf("got %+v, want changed [0] from the store event, with the poll an hour away", got)
	}
}
