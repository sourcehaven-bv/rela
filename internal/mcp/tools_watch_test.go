package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const (
	watchShowREQ1 = `{"tool":"show_entity","args":{"id":"REQ-001"}}`
	watchListDEC  = `{"tool":"list_entities","args":{"type":"decision"}}`
)

// watchServer builds a server through NewServer over the shared fixture and
// returns it with its store, for writes, and a connected client.
func watchServer(t *testing.T) (*mcpgo.ClientSession, *memstore.MemStore) {
	t.Helper()
	meta, st := makeTestFixture(t)
	srv, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return connect(t, srv), st
}

// callWatch calls watch with args (a JSON object) and decodes the result.
// A tool error is returned as an error whose text is the tool's message.
func callWatch(ctx context.Context, cs *mcpgo.ClientSession, args string) (watchResult, error) {
	res, err := cs.CallTool(ctx, &mcpgo.CallToolParams{Name: "watch", Arguments: json.RawMessage(args)})
	if err != nil {
		return watchResult{}, err
	}
	text := res.Content[0].(*mcpgo.TextContent).Text
	if res.IsError {
		return watchResult{}, toolError(text)
	}
	var out watchResult
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return watchResult{}, err
	}
	return out, nil
}

type toolError string

func (e toolError) Error() string { return string(e) }

func watchArgs(cursor string, timeout float64, targets ...string) string {
	b, _ := json.Marshal(cursor)
	t, _ := json.Marshal(timeout)
	return `{"targets":[` + strings.Join(targets, ",") + `],"cursor":` + string(b) + `,"timeout":` + string(t) + `}`
}

func mustWatch(t *testing.T, cs *mcpgo.ClientSession, args string) watchResult {
	t.Helper()
	got, err := callWatch(context.Background(), cs, args)
	if err != nil {
		t.Fatalf("watch %s: %v", args, err)
	}
	return got
}

func retitle(t *testing.T, st store.Store, id, typ, title string) {
	t.Helper()
	if err := setTitle(context.Background(), st, id, typ, title); err != nil {
		t.Fatal(err)
	}
}

// later runs write after d on another goroutine, reporting a failure with
// t.Errorf (t.Fatal must stay on the test goroutine).
func later(t *testing.T, d time.Duration, write func() error) {
	t.Helper()
	go func() {
		time.Sleep(d)
		if err := write(); err != nil {
			t.Errorf("write: %v", err)
		}
	}()
}

func setTitle(ctx context.Context, st store.Store, id, typ, title string) error {
	e := entity.New(id, typ)
	e.SetString("title", title)
	if err := st.UpdateEntity(ctx, e); err != nil {
		return fmt.Errorf("update %s: %w", id, err)
	}
	return nil
}

// AC1: without a cursor, watch answers at once with a baseline cursor.
func TestWatch_NoCursorReturnsBaseline(t *testing.T) {
	t.Parallel()
	cs, _ := watchServer(t)
	start := time.Now()
	got := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`]}`)
	if got.Cursor == "" || len(got.Changed) != 0 || got.TimedOut {
		t.Fatalf("got %+v, want a cursor, no changes, not timed out", got)
	}
	if d := time.Since(start); d > watchTick {
		t.Fatalf("took %v, want under one tick", d)
	}
}

// AC2: a watch with a cursor blocks until a target changes and reports it.
func TestWatch_BlocksUntilChange(t *testing.T) {
	t.Parallel()
	cs, st := watchServer(t)
	base := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`,`+watchListDEC+`]}`)

	const delay = 300 * time.Millisecond
	later(t, delay, func() error {
		return setTitle(context.Background(), st, "REQ-001", "requirement", "Changed title")
	})
	start := time.Now()
	got := mustWatch(t, cs, watchArgs(base.Cursor, 10, watchShowREQ1, watchListDEC))
	if got.TimedOut || len(got.Changed) != 1 || got.Changed[0] != 0 {
		t.Fatalf("got %+v, want changed [0]", got)
	}
	if d := time.Since(start); d < delay {
		t.Fatalf("returned after %v, before the write at %v", d, delay)
	}

	again := mustWatch(t, cs, watchArgs(got.Cursor, 0.6, watchShowREQ1, watchListDEC))
	if !again.TimedOut || len(again.Changed) != 0 {
		t.Fatalf("with the new cursor got %+v, want a timeout", again)
	}
}

// AC4: a timeout keeps the digests, and baselines a target the cursor did
// not have, so a change after that call is reported by the next one.
func TestWatch_TimeoutBaselinesNewTargets(t *testing.T) {
	t.Parallel()
	cs, st := watchServer(t)
	base := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`]}`)

	same := mustWatch(t, cs, watchArgs(base.Cursor, 0.6, watchShowREQ1))
	if !same.TimedOut || same.Cursor != base.Cursor {
		t.Fatalf("got %+v, want a timeout with the same cursor", same)
	}

	showDEC := `{"tool":"show_entity","args":{"id":"DEC-001"}}`
	added := mustWatch(t, cs, watchArgs(base.Cursor, 0.6, watchShowREQ1, showDEC))
	if !added.TimedOut || len(added.Changed) != 0 {
		t.Fatalf("adding a target got %+v, want a timeout", added)
	}
	retitle(t, st, "DEC-001", "decision", "Changed decision")
	// REQ-001's view lists DEC-001's title too, so both may be reported;
	// the added target must be.
	got := mustWatch(t, cs, watchArgs(added.Cursor, 5, watchShowREQ1, showDEC))
	if !slices.Contains(got.Changed, 1) {
		t.Fatalf("got %+v, want target 1 reported", got)
	}
}

// AC5: a change between two calls is reported at once by the second.
func TestWatch_ChangeBetweenCallsReportedAtOnce(t *testing.T) {
	t.Parallel()
	cs, st := watchServer(t)
	base := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`,`+watchShowREQ1+`]}`)
	retitle(t, st, "REQ-001", "requirement", "Between calls")

	start := time.Now()
	got := mustWatch(t, cs, watchArgs(base.Cursor, 10, watchShowREQ1, watchShowREQ1))
	if len(got.Changed) != 2 {
		t.Fatalf("got %+v, want both duplicate targets reported", got)
	}
	if d := time.Since(start); d > watchTick {
		t.Fatalf("took %v, want an immediate answer", d)
	}
}

// AC6: invalid input is a tool error, never a wait.
func TestWatch_InvalidInput(t *testing.T) {
	t.Parallel()
	cs, _ := watchServer(t)
	targets16 := strings.Repeat(watchShowREQ1+",", 16) + watchShowREQ1
	cases := []struct{ name, args, want string }{
		{"no targets", `{"targets":[]}`, "non-empty array"},
		{"targets not an array", `{"targets":"x"}`, "non-empty array"},
		{"17 targets", `{"targets":[` + targets16 + `]}`, "at most 16"},
		{"target not an object", `{"targets":["show_entity"]}`, "must be an object"},
		{"write tool", `{"targets":[{"tool":"update_entity","args":{"id":"REQ-001"}}]}`, "tool must be one of"},
		{"search", `{"targets":[{"tool":"search_entities","args":{"query":"x"}}]}`, "tool must be one of"},
		{"watch itself", `{"targets":[{"tool":"watch","args":{}}]}`, "tool must be one of"},
		{"unknown target field", `{"targets":[{"tool":"show_entity","id":"REQ-001"}]}`, "unknown field"},
		{"args not an object", `{"targets":[{"tool":"show_entity","args":"REQ-001"}]}`, "args must be an object"},
		{"negative timeout", `{"targets":[` + watchShowREQ1 + `],"timeout":-1}`, "timeout"},
		{"timeout over 50", `{"targets":[` + watchShowREQ1 + `],"timeout":51}`, "timeout"},
		{"huge timeout", `{"targets":[` + watchShowREQ1 + `],"timeout":1e300}`, "timeout"},
		{"cursor not base64", `{"targets":[` + watchShowREQ1 + `],"cursor":"***"}`, "invalid cursor"},
		{"cursor bad json", `{"targets":[` + watchShowREQ1 + `],"cursor":"bm90IGpzb24"}`, "invalid cursor"},
		{"cursor version 2", `{"targets":[` + watchShowREQ1 + `],"cursor":"eyJ2IjoyLCJoIjp7fX0"}`, "invalid cursor"},
		{"cursor too long", `{"targets":[` + watchShowREQ1 + `],"cursor":"` + strings.Repeat("A", 4097) + `"}`,
			"invalid cursor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := callWatch(context.Background(), cs, tc.args)
			var te toolError
			if !errors.As(err, &te) || !strings.Contains(string(te), tc.want) {
				t.Fatalf("got %v, want a tool error containing %q", err, tc.want)
			}
		})
	}
}

// An unknown argument inside a target is refused by the replayed tool, as
// a direct call would be: the target's result is that error.
func TestWatch_TargetArgumentsCheckedLikeACall(t *testing.T) {
	t.Parallel()
	cs, _ := watchServer(t)
	text, isError := callToolOn(t, cs, "show_entity", `{"id":"REQ-001","bogus":1}`)
	if !isError || !strings.Contains(text, "unknown argument") {
		t.Fatalf("direct call: %q, want an unknown-argument error", text)
	}
	if _, err := callWatch(context.Background(), cs,
		`{"targets":[{"tool":"show_entity","args":{"id":"REQ-001","bogus":1}}]}`); err != nil {
		t.Fatalf("watch: %v; the target's error is its result, not a watch error", err)
	}
}

func callToolOn(t *testing.T, cs *mcpgo.ClientSession, name, args string) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpgo.CallToolParams{Name: name, Arguments: json.RawMessage(args)})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res.Content[0].(*mcpgo.TextContent).Text, res.IsError
}

// AC7: an identity runs at most four watches at once; the fifth is told to
// retry, and a slot frees when a watch ends.
func TestWatch_PerIdentityLimit(t *testing.T) {
	t.Parallel()
	src := &countingSource{}
	srv, _ := serverWithSource(t, src)
	cs := connect(t, srv)
	base := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`]}`)
	baselineSubs, _ := src.counts()

	cancels := make([]context.CancelFunc, watchMaxPerIdentity)
	done := make(chan error, watchMaxPerIdentity)
	for i := range cancels {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		go func() {
			_, err := callWatch(ctx, cs, watchArgs(base.Cursor, 30, watchShowREQ1))
			done <- err
		}()
	}
	t.Cleanup(func() {
		for _, c := range cancels {
			c()
		}
	})
	waitFor(t, func() bool { subs, _ := src.counts(); return subs == baselineSubs+watchMaxPerIdentity })

	_, err := callWatch(context.Background(), cs, watchArgs(base.Cursor, 0, watchShowREQ1))
	if err == nil || !strings.Contains(err.Error(), watchTooManyMessage) {
		t.Fatalf("fifth watch: %v, want %q", err, watchTooManyMessage)
	}
	cancels[0]()
	<-done
	waitFor(t, func() bool {
		_, err := callWatch(context.Background(), cs, watchArgs(base.Cursor, 0, watchShowREQ1))
		return err == nil
	})
}

// waitFor polls cond until it holds, failing after 5 s.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 5 s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The server-wide limit counts every identity.
func TestWatchHandler_ServerLimit(t *testing.T) {
	t.Parallel()
	w := newWatchHandler(nil, nil)
	for i := range watchMaxPerServer {
		if _, ok := w.acquire(principal.Principal{User: string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Tool: principal.ToolMCP}); !ok {
			t.Fatalf("watch %d refused under the limit", i)
		}
	}
	if _, ok := w.acquire(principal.Principal{User: "late", Tool: principal.ToolMCP}); ok {
		t.Fatal("a watch over the server limit was admitted")
	}
}

// countingSource is a ChangeSource that records subscriptions and lets a
// test push events.
type countingSource struct {
	mu        sync.Mutex
	subs      int
	cancelled int
	chans     []chan store.Event
}

func (c *countingSource) Subscribe(n int) (events <-chan store.Event, cancel func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs++
	ch := make(chan store.Event, n)
	c.chans = append(c.chans, ch)
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.cancelled++
		})
	}
}

func (c *countingSource) counts() (subs, cancelled int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subs, c.cancelled
}

func (c *countingSource) poke() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.chans {
		select {
		case ch <- store.Event{}:
		default:
		}
	}
}

func serverWithSource(t *testing.T, src ChangeSource) (srv *Server, st *memstore.MemStore) {
	t.Helper()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	deps.Watch.Changes = src
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv, st
}

// AC8: canceling the call ends the wait and releases the subscription.
func TestWatch_CancelReleasesSubscription(t *testing.T) {
	t.Parallel()
	src := &countingSource{}
	srv, _ := serverWithSource(t, src)
	cs := connect(t, srv)
	base := mustWatch(t, cs, `{"targets":[`+watchShowREQ1+`]}`)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := callWatch(ctx, cs, watchArgs(base.Cursor, 30, watchShowREQ1))
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a cancelled watch returned a result")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled watch did not return")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		subs, cancelled := src.counts()
		if subs == 2 && cancelled == 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscriptions = %d, cancelled = %d; want 2 and 2", subs, cancelled)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// fakeWatch returns a function that calls a watch handler over src, as the
// tester, whose show_entity target runs show.
func fakeWatch(
	t *testing.T, src ChangeSource, show mcpgo.ToolHandler,
) func(ctx context.Context, args string) (watchResult, error) {
	t.Helper()
	srv, _ := serverWithSource(t, src)
	w := newWatchHandler(srv, map[string]mcpgo.ToolHandler{"show_entity": show})
	tester := principal.Principal{User: "tester", Tool: principal.ToolMCP}
	return func(ctx context.Context, args string) (watchResult, error) {
		res, err := w.handle(principal.With(ctx, tester), &mcpgo.CallToolRequest{Params: &mcpgo.CallToolParamsRaw{
			Name: "watch", Arguments: json.RawMessage(args),
		}})
		if err != nil {
			return watchResult{}, err
		}
		text := res.Content[0].(*mcpgo.TextContent).Text
		if res.IsError {
			return watchResult{}, toolError(text)
		}
		var out watchResult
		return out, json.Unmarshal([]byte(text), &out)
	}
}

// varying is a show_entity stand-in whose result is body, and which fails
// while fail is positive, counting down.
type varying struct {
	body    atomic.Value
	fail    atomic.Int64
	replays atomic.Int64
}

func (v *varying) handle(context.Context, *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v.replays.Add(1)
	if v.fail.Add(-1) >= 0 {
		return nil, errors.New("busy")
	}
	body, _ := v.body.Load().(string)
	return textResult(body), nil
}

// A burst of store events runs one replay pass per tick, not one per event.
func TestWatch_BurstReplaysOncePerTick(t *testing.T) {
	t.Parallel()
	src := &countingSource{}
	v := &varying{}
	call := fakeWatch(t, src, v.handle)
	base, err := call(context.Background(), `{"targets":[`+watchShowREQ1+`]}`)
	if err != nil {
		t.Fatal(err)
	}
	v.replays.Store(0)

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				src.poke()
			}
		}
	}()
	_, err = call(context.Background(), watchArgs(base.Cursor, 1.2, watchShowREQ1))
	close(stop)
	if err != nil {
		t.Fatal(err)
	}
	// 1 initial pass plus at most one per tick within 1.2 s.
	if got := v.replays.Load(); got > 1+int64(1200*time.Millisecond/watchTick)+1 {
		t.Fatalf("replays = %d during a burst, want one per tick", got)
	}
}

// A replay that did not finish runs again at the next tick without another
// store event: the event that woke the pass may have been about it.
func TestWatch_UnfinishedReplayRetried(t *testing.T) {
	t.Parallel()
	src := &countingSource{}
	v := &varying{}
	v.body.Store("old")
	call := fakeWatch(t, src, v.handle)
	base, err := call(context.Background(), `{"targets":[`+watchShowREQ1+`]}`)
	if err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		res watchResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := call(context.Background(), watchArgs(base.Cursor, 5, watchShowREQ1))
		done <- outcome{res, err}
	}()
	waitFor(t, func() bool { subs, _ := src.counts(); return subs == 2 })
	time.Sleep(50 * time.Millisecond) // let the first pass finish
	v.body.Store("new")
	v.fail.Store(1)
	src.poke()

	got := <-done
	if got.err != nil || len(got.res.Changed) != 1 {
		t.Fatalf("got %+v, %v; want the change reported after the retry", got.res, got.err)
	}
}

// A pass ends at the call's deadline: a replay that hangs does not hold the
// answer past the timeout.
func TestWatch_PassBoundedByTimeout(t *testing.T) {
	t.Parallel()
	src := &countingSource{}
	var hang atomic.Bool
	call := fakeWatch(t, src, func(ctx context.Context, _ *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		if hang.Load() {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return textResult("same"), nil
	})
	base, err := call(context.Background(), `{"targets":[`+watchShowREQ1+`]}`)
	if err != nil {
		t.Fatal(err)
	}
	hang.Store(true)
	go func() {
		time.Sleep(100 * time.Millisecond)
		src.poke()
	}()
	start := time.Now()
	got, err := call(context.Background(), watchArgs(base.Cursor, 0.8, watchShowREQ1))
	if err != nil || !got.TimedOut {
		t.Fatalf("got %+v, %v; want a timeout", got, err)
	}
	if d := time.Since(start); d > 800*time.Millisecond+watchTick {
		t.Fatalf("returned after %v, want close to the 0.8 s timeout", d)
	}
}

// A baseline that could not read every target is an error: a cursor
// without one would baseline it silently next time and lose a change.
func TestWatch_IncompleteBaselineIsAnError(t *testing.T) {
	t.Parallel()
	v := &varying{}
	v.fail.Store(1)
	call := fakeWatch(t, &countingSource{}, v.handle)
	_, err := call(context.Background(), `{"targets":[`+watchShowREQ1+`]}`)
	if err == nil || !strings.Contains(err.Error(), watchIncomplete) {
		t.Fatalf("got %v, want %q", err, watchIncomplete)
	}
}

// Running out of budget skips the rest of a pass; the next pass starts at
// the first skipped target, so every target is read in turn.
func TestWatchRun_BudgetRotates(t *testing.T) {
	t.Parallel()
	srv, _ := serverWithSource(t, &countingSource{})
	w := newWatchHandler(srv, nil)
	w.budget = 30 * time.Millisecond
	var order []string
	slow := func(id string) watchTarget {
		return watchTarget{tool: "show_entity", key: id, handler: func(
			context.Context, *mcpgo.CallToolRequest,
		) (*mcpgo.CallToolResult, error) {
			order = append(order, id)
			time.Sleep(40 * time.Millisecond)
			return textResult(id), nil
		}}
	}
	p := principal.Principal{User: "tester", Tool: principal.ToolMCP}
	e, _ := w.acquire(p)
	run := &watchRun{w: w, d: srv.deps(), entry: e, shareKey: p.ShareKey(),
		targets: []watchTarget{slow("a"), slow("b"), slow("c")}, digests: map[string]string{}}
	ctx := principal.With(context.Background(), p)
	far := time.Now().Add(time.Minute)
	for range 3 {
		if _, incomplete, err := run.pass(ctx, far); err != nil || !incomplete {
			t.Fatalf("pass: incomplete=%v err=%v; want an incomplete pass", incomplete, err)
		}
	}
	if !slices.Equal(order, []string{"a", "b", "c"}) {
		t.Fatalf("replay order = %v, want each target once in turn", order)
	}
}

// A failure to resolve read access is a tool error, not a silent timeout.
func TestWatch_RebindFailureIsAnError(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	deps.Watch.Rebind = func(context.Context) (context.Context, error) { return nil, errors.New("unstamped") }
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	_, err = callWatch(context.Background(), connect(t, srv), `{"targets":[`+watchShowREQ1+`]}`)
	if err == nil || !strings.Contains(err.Error(), watchNoAccess) {
		t.Fatalf("got %v, want %q", err, watchNoAccess)
	}
}

// A panicking replay is not finished: the target keeps its digest.
func TestWatchHandler_PanickingReplay(t *testing.T) {
	t.Parallel()
	w := newWatchHandler(nil, nil)
	e, _ := w.acquire(principal.Principal{User: "u", Tool: principal.ToolMCP})
	_, ok := w.replay(context.Background(), e, "k", watchTarget{
		tool: "show_entity",
		handler: func(context.Context, *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			panic("boom")
		},
	})
	if ok {
		t.Fatal("a panicking replay counted as finished")
	}
}

// A cursor binds the result to the principal and the process key.
func TestWatchHandler_DigestKeyedAndBound(t *testing.T) {
	t.Parallel()
	res := textResult("same body")
	a, b := newWatchHandler(nil, nil), newWatchHandler(nil, nil)
	if a.digest("alice", "t", res) == a.digest("bob", "t", res) {
		t.Error("two principals got the same digest for the same result")
	}
	if a.digest("alice", "t", res) == b.digest("alice", "t", res) {
		t.Error("two processes got the same digest")
	}
	if a.digest("alice", "t", res) == a.digest("alice", "t", errorResult("same body")) {
		t.Error("an error result digests like a success with the same text")
	}
}

// A cursor from another process reports every target changed once.
func TestWatch_ForeignCursorReportsChanged(t *testing.T) {
	t.Parallel()
	csA, _ := watchServer(t)
	csB, _ := watchServer(t)
	base := mustWatch(t, csA, `{"targets":[`+watchShowREQ1+`]}`)
	got := mustWatch(t, csB, watchArgs(base.Cursor, 5, watchShowREQ1))
	if len(got.Changed) != 1 {
		t.Fatalf("got %+v, want the target reported changed", got)
	}
}

// --- ACL ---------------------------------------------------------------

// watchPolicyServer builds a server under a policy where group `readers`
// (alice is a member) reads tickets, and nobody reads features. Every call
// runs as alice with an acl.Request attached up front.
func watchPolicyServer(t *testing.T) (*mcpgo.ClientSession, store.Store) {
	t.Helper()
	manual := func(label, prefix string) metamodel.EntityDef {
		return metamodel.EntityDef{
			Label: label, IDPrefix: prefix, IDType: metamodel.IDTypeManual, DisplayProperty: "title",
			Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
		}
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  manual("Ticket", "TKT"),
			"feature": manual("Feature", "FEAT"),
			"person":  manual("Person", ""),
			"group":   manual("Group", ""),
		},
		Relations: map[string]metamodel.RelationDef{
			"member-of": {Label: "member of", From: []string{"person"}, To: []string{"group"}},
			"relates":   {Label: "relates to", From: []string{"ticket"}, To: []string{"feature"}},
		},
	}
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		newEntity("TKT-1", "ticket", "visible ticket"),
		newEntity("FEAT-1", "feature", "hidden feature"),
		newEntity("alice", "person", "Alice"),
		newEntity("readers", "group", "Readers"),
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "alice", Type: "member-of", To: "readers"}, nil); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"readers": "viewer"},
	}, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	reads := svc.GatedReads()
	deps := Deps{
		Store:         reads.Reader,
		Traversals:    reads.Traversals,
		Meta:          meta,
		Tracer:        reads.Tracer,
		Searcher:      reads.Searcher,
		Validator:     reads.Validator,
		EntityManager: svc.EntityManager(),
		Config:        svc.Config(),
		Watcher:       nopWatcher{},
		Watch:         WatchDeps{Changes: st, Rebind: reads.Rebind},
		ProjectRoot:   t.TempDir(),
		Attachments:   testAttachmentDeps(t, svc, meta),
		World:         store.TrivialScope(),
		Families:      store.TrivialScope(),
	}
	alice := principal.Principal{User: "alice", Tool: principal.ToolMCP}
	srv, err := NewServer(deps, "test", WithPrincipal(alice))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	// Attach one acl.Request per call, as attachACLRequest does on remote
	// MCP. It caches alice's roles for the whole call.
	srv.mcp.AddReceivingMiddleware(func(next mcpgo.MethodHandler) mcpgo.MethodHandler {
		return func(ctx context.Context, method string, req mcpgo.Request) (mcpgo.Result, error) {
			r, err := d.ForPrincipal(alice)
			if err != nil {
				return nil, err
			}
			return next(acl.WithRequest(ctx, r), method, req)
		}
	})
	return connect(t, srv), st
}

// AC3: writes the caller cannot see do not end the wait; a visible one does.
func TestWatch_HiddenChangesDoNotWake(t *testing.T) {
	t.Parallel()
	cs, st := watchPolicyServer(t)
	listTickets := `{"tool":"list_entities","args":{"type":"ticket"}}`
	showHidden := `{"tool":"show_entity","args":{"id":"FEAT-1"}}`
	listHidden := `{"tool":"list_entities","args":{"type":"feature"}}`
	relsToHidden := `{"tool":"list_relations","args":{"to":"FEAT-1"}}`
	allRels := `{"tool":"list_relations","args":{}}`
	targets := []string{listTickets, showHidden, listHidden, relsToHidden, allRels}
	base := mustWatch(t, cs, `{"targets":[`+strings.Join(targets, ",")+`]}`)

	ctx := context.Background()
	toHidden := entity.RelationKey{From: "TKT-1", Type: "relates", To: "FEAT-1"}
	later(t, 100*time.Millisecond, func() error {
		if err := setTitle(ctx, st, "FEAT-1", "feature", "secretly changed"); err != nil {
			return err
		}
		if err := st.CreateEntity(ctx, newEntity("FEAT-2", "feature", "new secret")); err != nil {
			return err
		}
		if _, err := st.CreateRelation(ctx, toHidden, nil); err != nil {
			return err
		}
		_, err := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "relates", To: "FEAT-2"}, nil)
		return err
	})
	got := mustWatch(t, cs, watchArgs(base.Cursor, 1.5, targets...))
	if !got.TimedOut || len(got.Changed) != 0 {
		t.Fatalf("got %+v after hidden writes, want a timeout", got)
	}
	later(t, 100*time.Millisecond, func() error { return st.DeleteRelation(ctx, toHidden) })
	got = mustWatch(t, cs, watchArgs(got.Cursor, 1, targets...))
	if !got.TimedOut || len(got.Changed) != 0 {
		t.Fatalf("got %+v after deleting a hidden relation, want a timeout", got)
	}

	later(t, 100*time.Millisecond, func() error { return setTitle(ctx, st, "TKT-1", "ticket", "visibly changed") })
	got = mustWatch(t, cs, watchArgs(got.Cursor, 5, targets...))
	if len(got.Changed) != 1 || got.Changed[0] != 0 {
		t.Fatalf("got %+v after a visible write, want changed [0]", got)
	}
}

// AC9: access is resolved again on every pass. Removing alice from the
// group mid-watch turns TKT-1 into not found, which is reported; without the
// fresh resolution the cached roles would keep the old result.
func TestWatch_AccessResolvedEveryPass(t *testing.T) {
	t.Parallel()
	cs, st := watchPolicyServer(t)
	showTicket := `{"tool":"show_entity","args":{"id":"TKT-1"}}`
	base := mustWatch(t, cs, `{"targets":[`+showTicket+`]}`)

	later(t, 100*time.Millisecond, func() error {
		return st.DeleteRelation(context.Background(), entity.RelationKey{From: "alice", Type: "member-of", To: "readers"})
	})
	got := mustWatch(t, cs, watchArgs(base.Cursor, 5, showTicket))
	if len(got.Changed) != 1 {
		t.Fatalf("got %+v, want the revoked read reported", got)
	}
	text, isError := callToolOn(t, cs, "show_entity", `{"id":"TKT-1"}`)
	if !isError || strings.Contains(text, "visible ticket") {
		t.Fatalf("after revocation show_entity = %q, want not found", text)
	}
}
