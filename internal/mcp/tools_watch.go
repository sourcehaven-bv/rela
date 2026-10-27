package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The watch tool lets an agent wait for the result of read-tool calls to
// change instead of polling them (TKT-FMS8BS, FEAT-GWF8YH). It applies the
// live-view idea of the data-entry SSE stream to MCP: a store event pokes the
// watch, the watch replays each target as the caller and compares a digest of
// the gated result. It is unrelated to the file watcher in [Deps.Watcher].
//
// Hidden writes cannot end a wait: every replay reads through [Deps.Store],
// so a hidden entity reads as not found before and after its writes. That is
// also why search_entities is not watchable — its ranking uses index-wide
// term statistics, so a hidden write can reorder visible hits. Replays run
// on a fixed clock, so when a watch returns does not depend on hidden writes
// either.
const (
	watchMaxTargets      = 16
	watchDefaultTimeout  = 30 * time.Second
	watchMaxTimeout      = 50 * time.Second
	watchTick            = 500 * time.Millisecond
	watchMaxPerServer    = 32
	watchMaxPerIdentity  = 4
	watchParallel        = 4
	watchReplayTimeout   = 5 * time.Second
	watchPassBudget      = 10 * time.Second
	watchCursorMaxLen    = 4096
	watchEventBuffer     = 64
	watchDigestBytes     = 16
	watchCursorVersion   = 1
	watchTooManyMessage  = "too many concurrent watches; retry later"
	watchInvalidCursor   = "invalid cursor: pass the cursor a previous watch returned, or none"
	watchNoAccess        = "could not resolve read access for this principal"
	watchIncomplete      = "could not read every target in time; retry later"
	watchTargetsArgument = "targets"
)

// Channel targets. A read_channel follower (after_rev or after) replays at
// most watchChannelLimit messages: a waiting follower's page is empty (see
// [watchRun.pass]), so a short page sees every change and keeps a poll
// cheap. A page target replays its own limit.
const (
	watchChannelPoll  = 2 * time.Second
	watchChannelLimit = 5
)

// watchOrder lists the read tools a watch target may name, in the order the
// watch tool describes them. Only those registered on the server count.
var watchOrder = []string{
	"show_entity", "list_entities", "list_relations", toolReadChannel, toolReadSession, toolListInbox,
}

// watchNames returns the tools of watchOrder that reads holds.
func watchNames(reads map[string]mcpgo.ToolHandler) []string {
	var out []string
	for _, name := range watchOrder {
		if _, ok := reads[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

// ChangeSource pokes a watch when the store changes. It has the method set of
// [store.Watcher]; it is declared here so MCP depends on the one method it
// calls.
type ChangeSource interface {
	Subscribe(bufSize int) (events <-chan store.Event, cancel func())
}

// toolWatch describes the watch tool for the watchable tools names.
func toolWatch(names []string) *mcpgo.Tool {
	enumJSON, err := json.Marshal(names)
	if err != nil { // coverage-ignore: a string slice always marshals
		panic("mcp: marshal watch tool names: " + err.Error())
	}
	return newTool("watch",
		withDescription("Wait until the result of read-tool calls changes. Call it first without cursor, "+
			"then read, then call it again with the returned cursor; it returns when a target's result "+
			"changes for you, or at the timeout. Result: {cursor,changed,timed_out,incomplete}, changed being "+
			"indices into targets. incomplete lists the targets whose latest read did not finish in time, "+
			"so their result is unknown rather than unchanged. Re-read the changed targets yourself."),
		withArray(watchTargetsArgument, required(), description(
			"Up to 16 read calls: [{tool, args}], tool one of "+strings.Join(names, ", ")+"; "+
				"args is the object that tool takes"),
			func(p *schemaProperty, _ *toolSpec, _ string) {
				p.Items = json.RawMessage(`{"type":"object","properties":{` +
					`"tool":{"type":"string","enum":` + string(enumJSON) + `},` +
					`"args":{"type":"object"}},"required":["tool"]}`)
			}),
		withString("cursor", description("Cursor from a previous watch")),
		withNumber("timeout", description("Seconds to wait (default 30, max 50; 0 checks once)")),
	)
}

// watchHandler serves the watch tool. [NewServer] builds one per server, so
// both transports share its limits.
type watchHandler struct {
	s *Server
	// reads maps each watchable tool to its registered handler, wrapped by
	// addTool so a target's unknown arguments are refused the same way.
	// Never written after construction.
	reads map[string]mcpgo.ToolHandler
	// names lists the keys of reads in watchOrder.
	names []string
	// key keys the cursor digests. It is per process: a cursor from another
	// process reports every target changed once, which is safe.
	key   []byte
	slots chan struct{}
	// tick, budget, replayTimeout, inboxEvery and inboxGap are the
	// constants above; tests shorten them.
	tick          time.Duration
	budget        time.Duration
	replayTimeout time.Duration
	inboxEvery    time.Duration
	inboxGap      time.Duration

	mu     sync.Mutex
	active []*watchEntry
}

// watchEntry is one running watch. Watches of one identity share slot, so an
// identity holds at most one replay slot at a time.
type watchEntry struct {
	p    principal.Principal
	slot chan struct{}
}

type watchTarget struct {
	tool    string
	args    json.RawMessage
	key     string
	handler mcpgo.ToolHandler
	// channel is set for a read_channel, read_session or list_inbox
	// target.
	channel *watchChannel
}

// watchChannel is the channel a read_channel or read_session target
// reads, as its arguments name it. A read_session target never follows. A
// wake for another channel skips the target.
type watchChannel struct {
	// inbox is set for a list_inbox target: a change to a channel of a
	// type in types, or to a channel in keys (the unread window), may
	// change it. Until a replay has reported them, every channel may.
	inbox bool
	types map[string]bool
	keys  map[realtime.Key]bool
	known bool

	anchor string // entity id, without a face
	typ    string
	name   string
	// follows is set when the target follows the channel, by after_rev or
	// by after.
	follows bool
}

// matches reports whether a change to the channel (id, typ, name) may
// change the target's result. A target without a name matches every name,
// so a wake is never missed; an extra wake costs one replay.
func (c *watchChannel) matches(id, typ, name string) bool {
	if c.inbox {
		return !c.known || c.types[typ]
	}
	return c.anchor == id && c.typ == typ && (c.name == "" || c.name == name)
}

// matchesKey reports whether a change to the channel k may change the
// unread half of a list_inbox target: k is in the window its last replay
// reported.
func (c *watchChannel) matchesKey(k realtime.Key) bool {
	return c.inbox && c.keys[k]
}

// learnInbox records the waiting types and the window keys a list_inbox
// replay reported.
func (c *watchChannel) learnInbox(n inboxNote) {
	c.types, c.known = make(map[string]bool, len(n.types)), true
	for _, t := range n.types {
		c.types[t] = true
	}
	c.keys = make(map[realtime.Key]bool, len(n.keys))
	for _, k := range n.keys {
		c.keys[k] = true
	}
}

type watchResult struct {
	Cursor   string `json:"cursor"`
	Changed  []int  `json:"changed"`
	TimedOut bool   `json:"timed_out"`
	// Incomplete are the targets whose latest replay did not finish.
	Incomplete []int `json:"incomplete,omitempty"`
}

type watchCursor struct {
	V int               `json:"v"`
	H map[string]string `json:"h"`
}

func newWatchHandler(s *Server, reads map[string]mcpgo.ToolHandler) *watchHandler {
	key := make([]byte, sha256.Size)
	_, _ = rand.Read(key) // never fails (crypto/rand)
	return &watchHandler{
		s: s, reads: reads, names: watchNames(reads), key: key,
		slots:  make(chan struct{}, watchParallel),
		tick:   watchTick,
		budget: watchPassBudget, replayTimeout: watchReplayTimeout, inboxEvery: inboxPollEvery,
		inboxGap: inboxWakeGap,
	}
}

func (w *watchHandler) handle(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	d := w.s.deps()
	tr := newToolRequest(req)
	targets, err := w.parseTargets(tr.GetArguments()[watchTargetsArgument])
	if err != nil {
		return errorResult(err.Error()), nil
	}
	timeout, err := parseWatchTimeout(tr.GetArguments()["timeout"])
	if err != nil {
		return errorResult(err.Error()), nil
	}
	cursor := tr.GetString("cursor", "")
	var prev map[string]string
	if cursor != "" {
		if prev, err = decodeWatchCursor(cursor); err != nil {
			return errorResult(watchInvalidCursor), nil
		}
	}

	p := principal.From(ctx)
	entry, ok := w.acquire(p)
	if !ok {
		return errorResult(watchTooManyMessage), nil
	}
	defer w.release(entry)

	// Subscribe before the first pass, so a change during it still wakes
	// the loop.
	events, cancel := d.Watch.Changes.Subscribe(watchEventBuffer)
	defer cancel()
	// A lease change shows in show_entity without a store event. Without
	// leases the channel stays nil and never fires.
	var leaseWake <-chan struct{}
	if d.Leases != nil {
		wake, cancelWake := d.Leases.Subscribe()
		defer cancelWake()
		leaseWake = wake
	}
	chans := newChannelWake(d, targets)
	defer chans.close()

	run := &watchRun{w: w, d: d, entry: entry, targets: targets, shareKey: p.ShareKey(),
		digests:    make(map[string]string, len(targets)),
		inboxEvery: w.inboxEvery, inboxGap: w.inboxGap}
	for _, t := range targets {
		if dg, ok := prev[t.key]; ok {
			run.digests[t.key] = dg
		}
	}

	// Only a waiting call is bounded by its timeout; a baseline or a single
	// check gets one full pass.
	waits := cursor != "" && timeout > 0
	deadline := time.Now().Add(timeout)
	if !waits {
		deadline = time.Now().Add(w.budget)
	}
	changed, incomplete, err := run.pass(ctx, deadline, passAll)
	switch {
	case err != nil:
		return errorResult(watchNoAccess), nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case len(changed) > 0:
		return run.result(changed, false)
	case !waits:
		return run.result(nil, cursor != "")
	}
	return run.wait(ctx, deadline, watchWakes{events: events, lease: leaseWake, chans: chans}, incomplete)
}

// watchRun is the state of one watch call.
type watchRun struct {
	w        *watchHandler
	d        Deps
	entry    *watchEntry
	targets  []watchTarget
	shareKey string
	// digests holds the latest finished digest per target key, starting
	// from the cursor's.
	digests map[string]string
	// next is the target the next pass starts at, after a pass ran out of
	// budget.
	next int
	// failed holds the targets whose latest replay did not finish; nil is
	// none.
	failed map[string]bool
	// The list_inbox pacing, see [watchRun.inboxDue]. inboxEvery and
	// inboxNext pace the poll; a replay that does not finish doubles
	// inboxEvery and holds the target back on every path until inboxNext
	// (inboxHeld). inboxGap and inboxLast pace a wake or a store event,
	// and inboxPending records one that the gap held back.
	inboxEvery   time.Duration
	inboxNext    time.Time
	inboxHeld    bool
	inboxGap     time.Duration
	inboxLast    time.Time
	inboxPending bool
}

// passKind says what made a pass run.
type passKind int

const (
	// passAll replays every target: the first pass, or after a store or
	// lease event.
	passAll passKind = iota
	// passWake replays the channel targets after an in-process channel
	// wake.
	passWake
	// passPoll replays the channel targets on the channel poll.
	passPoll
	// passInbox replays only the list_inbox target, once the gap that held
	// back a wake for it has passed.
	passInbox
)

// watchWakes are the wake sources of one watch call. A nil channel never
// fires.
type watchWakes struct {
	events <-chan store.Event
	lease  <-chan struct{}
	chans  *channelWake
}

// wait replays the targets on the tick after a wake until one changes or
// deadline passes. dirty starts it with a full pass due.
func (r *watchRun) wait(
	ctx context.Context, deadline time.Time, wakes watchWakes, dirty bool,
) (*mcpgo.CallToolResult, error) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	ticker := time.NewTicker(r.w.tick)
	defer ticker.Stop()
	// A pass that skipped or did not finish a target runs again at the next
	// tick: the event that woke it may have been about that target. chanDirty
	// replays only the channel targets; polled says only the poll set it.
	chanDirty, polled := false, false
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return r.result(nil, true)
		case _, open := <-wakes.events:
			if !open {
				wakes.events = nil
			}
			dirty = true
		case <-wakes.lease:
			dirty = true
		case <-wakes.chans.wake:
			if wakes.chans.take() {
				chanDirty, polled = true, false
			}
		case <-wakes.chans.poll:
			polled = polled || !chanDirty
			chanDirty = true
		case <-ticker.C:
			inboxReady := r.inboxPending && r.inboxDue(passInbox)
			if !dirty && !chanDirty && !inboxReady {
				continue
			}
			kind := passAll
			switch {
			case dirty:
			case chanDirty && polled:
				kind = passPoll
			case chanDirty:
				kind = passWake
			default:
				kind = passInbox
			}
			changed, incomplete, err := r.pass(ctx, deadline, kind)
			switch {
			case err != nil:
				return errorResult(watchNoAccess), nil
			case ctx.Err() != nil:
				return nil, ctx.Err()
			case len(changed) > 0:
				return r.result(changed, false)
			case !time.Now().Before(deadline):
				return r.result(nil, true)
			}
			switch {
			case kind == passInbox:
			case dirty:
				dirty, chanDirty = incomplete, false
			default:
				chanDirty = incomplete
			}
			if kind != passInbox {
				polled = false
			}
		}
	}
}

// channelWake holds the channel wake sources of one watch call: the
// in-process channel subscription and the channel poll. Both stay nil, and
// never fire, without a channel target.
type channelWake struct {
	sub     *realtime.Subscription
	ticker  *time.Ticker
	wake    <-chan struct{}
	poll    <-chan time.Time
	targets []*watchChannel
}

func newChannelWake(d Deps, targets []watchTarget) *channelWake {
	c := &channelWake{}
	for _, t := range targets {
		if t.channel != nil {
			c.targets = append(c.targets, t.channel)
		}
	}
	if len(c.targets) == 0 {
		return c
	}
	if d.Channels.Waker != nil {
		c.sub = d.Channels.Waker.Subscribe()
		c.wake = c.sub.Wake()
	}
	// A post made in another process wakes no subscription here; only the
	// poll sees it.
	every := d.Watch.ChannelPoll
	if every <= 0 {
		every = watchChannelPoll
	}
	c.ticker = time.NewTicker(every)
	c.poll = c.ticker.C
	return c
}

// take drains the subscription and reports whether a changed channel is
// one a target reads.
func (c *channelWake) take() bool {
	keys, resync := c.sub.Take()
	if resync {
		return true
	}
	for _, k := range keys {
		for _, t := range c.targets {
			if t.matches(k.ID, k.Type, k.Name) || t.matchesKey(k) {
				return true
			}
		}
	}
	return false
}

func (c *channelWake) close() {
	if c.sub != nil {
		c.sub.Close()
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
}

// pass replays every target once with freshly resolved read access and
// returns the indices of the targets whose digest moved. A target missing
// from the cursor takes its first digest as the baseline and is not
// reported; a replay that does not finish keeps the target's previous
// digest, and makes the pass incomplete, as does running out of budget.
// A pass ends at its budget or at deadline, whichever is first. An error
// means read access could not be resolved for the caller. passWake and
// passPoll replay only the channel targets, passInbox only the list_inbox
// target.
//
// A list_inbox target is paced by [watchRun.inboxDue]. A replay of it that
// does not finish does not make the pass incomplete, so the next tick does
// not rerun it; its pacing retries it, and the result lists it as
// incomplete until a replay finishes.
//
// A read_channel target that follows by after_rev or after and is missing
// from the cursor is reported when its page is not empty. An agent that
// moves its cursor forward starts a new target, and a message posted
// between its read and that baseline would otherwise be part of the
// baseline, and be lost.
func (r *watchRun) pass(
	ctx context.Context, deadline time.Time, kind passKind,
) (changed []int, incomplete bool, err error) {
	rctx, err := r.d.Watch.Rebind(ctx)
	if err != nil {
		slog.Warn("mcp: watch: could not resolve read access", "err", err)
		return nil, false, err
	}
	end := time.Now().Add(r.w.budget)
	if deadline.Before(end) {
		end = deadline
	}
	rctx, cancel := context.WithDeadline(rctx, end)
	defer cancel()

	moved := map[string]bool{}
	done := map[string]bool{}
	n := len(r.targets)
	first := r.next
	r.next = 0
	for i := range n {
		idx := (first + i) % n
		t := r.targets[idx]
		inbox := t.channel != nil && t.channel.inbox
		if done[t.key] || r.skips(t, kind) {
			continue
		}
		// The clock, not only rctx: the deadline timer that cancels rctx can
		// fire late on a loaded machine, and the budget must hold anyway.
		if rctx.Err() != nil || !time.Now().Before(end) {
			r.next = idx
			return r.moved(moved), true, nil
		}
		done[t.key] = true
		dg, pending, ok := r.replay(rctx, t)
		if !ok {
			incomplete = incomplete || !inbox
			continue
		}
		if old, had := r.digests[t.key]; (had && old != dg) || (!had && pending) {
			moved[t.key] = true
		}
		r.digests[t.key] = dg
	}
	return r.moved(moved), incomplete, nil
}

// skips reports whether a pass of kind leaves t out. A list_inbox wake
// the gap holds back is recorded, and runs once the gap has passed; the
// poll and a held target keep their own schedule.
func (r *watchRun) skips(t watchTarget, kind passKind) bool {
	inbox := t.channel != nil && t.channel.inbox
	switch {
	case kind != passAll && t.channel == nil, kind == passInbox && !inbox:
		return true
	case inbox && !r.inboxDue(kind):
		r.inboxPending = r.inboxPending || (kind != passPoll && !r.inboxHeld)
		return true
	}
	return false
}

// replay replays t and records whether it finished, pacing a list_inbox
// target.
func (r *watchRun) replay(ctx context.Context, t watchTarget) (digest string, pending, ok bool) {
	inbox := t.channel != nil && t.channel.inbox
	if inbox {
		r.inboxLast, r.inboxPending = time.Now(), false
	}
	digest, pending, ok = r.w.replay(ctx, r.entry, r.shareKey, t)
	if inbox {
		r.paceInbox(ok)
	}
	if ok {
		delete(r.failed, t.key)
		return digest, pending, true
	}
	if r.failed == nil {
		r.failed = map[string]bool{}
	}
	r.failed[t.key] = true
	return "", false, false
}

// inboxDue reports whether a pass of kind replays the list_inbox target
// now. The poll, and every path while a replay of it did not finish,
// replays it no sooner than inboxNext. A wake or a store event replays it
// no sooner than inboxGap after the last replay began, so a busy server
// costs each inbox watcher at most one replay per gap.
func (r *watchRun) inboxDue(kind passKind) bool {
	now := time.Now()
	if kind == passPoll || r.inboxHeld {
		return !now.Before(r.inboxNext)
	}
	return r.inboxLast.IsZero() || now.Sub(r.inboxLast) >= r.inboxGap
}

// paceInbox sets when the poll may replay the list_inbox target next. A
// replay that did not finish doubles the interval, for the rest of the
// call.
func (r *watchRun) paceInbox(finished bool) {
	r.inboxHeld = !finished
	if !finished {
		r.inboxEvery *= 2
	}
	r.inboxNext = time.Now().Add(r.inboxEvery)
}

// moved returns the indices of the targets whose key is in keys.
func (r *watchRun) moved(keys map[string]bool) []int {
	var changed []int
	for i, t := range r.targets {
		if keys[t.key] {
			changed = append(changed, i)
		}
	}
	return changed
}

// result answers the call. A target without a digest has never been read
// in time; a cursor without it would baseline it silently on the next call
// and lose a change in between, so that is an error instead.
func (r *watchRun) result(changed []int, timedOut bool) (*mcpgo.CallToolResult, error) {
	c := watchCursor{V: watchCursorVersion, H: make(map[string]string, len(r.targets))}
	for _, t := range r.targets {
		dg, ok := r.digests[t.key]
		if !ok {
			return errorResult(watchIncomplete), nil
		}
		c.H[t.key] = dg
	}
	raw, err := json.Marshal(c)
	if err != nil { // coverage-ignore: a map of strings always marshals
		return errorResult("encode cursor: " + err.Error()), nil
	}
	if changed == nil {
		changed = []int{}
	}
	var incomplete []int
	for i, t := range r.targets {
		if r.failed[t.key] {
			incomplete = append(incomplete, i)
		}
	}
	text, err := marshalJSON(watchResult{
		Cursor:     base64.RawURLEncoding.EncodeToString(raw),
		Changed:    changed,
		TimedOut:   timedOut,
		Incomplete: incomplete,
	})
	if err != nil { // coverage-ignore: a struct of strings and ints always marshals
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

// replay runs t as the caller and returns the digest of its result. pending
// reports a read_channel follower (after_rev or after) whose page holds
// messages. ok is false when the replay did not finish: no slot before ctx
// ended, a panic, the replay timeout, or a cancelled ctx.
func (w *watchHandler) replay(
	ctx context.Context, e *watchEntry, shareKey string, t watchTarget,
) (digest string, pending, ok bool) {
	select {
	case e.slot <- struct{}{}:
	case <-ctx.Done():
		return "", false, false
	}
	defer func() { <-e.slot }()
	select {
	case w.slots <- struct{}{}:
	case <-ctx.Done():
		return "", false, false
	}
	defer func() { <-w.slots }()

	rctx, cancel := context.WithTimeout(ctx, w.replayTimeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			slog.Error("mcp: watch: replay panicked", "tool", t.tool, "panic", p)
			digest, pending, ok = "", false, false
		}
	}()
	var probe *readProbe
	if t.channel != nil && (t.channel.follows || t.channel.inbox) {
		rctx, probe = withReadProbe(rctx)
	}
	res, err := t.handler(rctx, &mcpgo.CallToolRequest{
		Params: &mcpgo.CallToolParamsRaw{Name: t.tool, Arguments: t.args},
	})
	if err != nil || res == nil || rctx.Err() != nil {
		return "", false, false
	}
	if probe != nil && probe.inbox != nil {
		t.channel.learnInbox(*probe.inbox)
	}
	return w.digest(shareKey, t.key, res), probe != nil && probe.messages > 0, true
}

// readProbe receives what a read_channel replay read, so the watch learns
// whether the page held messages without parsing the tool's text.
// A list_inbox replay reports the waiting types and the window keys on it
// instead.
type readProbe struct {
	messages int
	inbox    *inboxNote
}

// inboxNote is what a list_inbox replay reports to its watch.
type inboxNote struct {
	types []string
	keys  []realtime.Key
}

type readProbeKey struct{}

// withReadProbe returns ctx carrying a fresh probe.
func withReadProbe(ctx context.Context) (context.Context, *readProbe) {
	p := &readProbe{}
	return context.WithValue(ctx, readProbeKey{}, p), p
}

// noteRead records a read_channel page on the probe in ctx, if any.
func noteRead(ctx context.Context, v realtime.ReadView) {
	if p, ok := ctx.Value(readProbeKey{}).(*readProbe); ok {
		p.messages = len(v.Messages)
	}
}

// noteInbox records the waiting types and the window keys of a list_inbox
// answer on the probe in ctx, if any.
func noteInbox(ctx context.Context, v realtime.InboxView) {
	if p, ok := ctx.Value(readProbeKey{}).(*readProbe); ok {
		p.inbox = &inboxNote{types: slices.Clone(v.WaitingTypes), keys: slices.Clone(v.WakeKeys)}
	}
}

// digest keys a result to the caller and the target, so a cursor cannot be
// used to test guesses offline or by another principal.
func (w *watchHandler) digest(shareKey, targetKey string, res *mcpgo.CallToolResult) string {
	m := hmac.New(sha256.New, w.key)
	field := func(b []byte) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(b)))
		m.Write(n[:])
		m.Write(b)
	}
	field([]byte(shareKey))
	field([]byte(targetKey))
	if res.IsError {
		field([]byte{1})
	} else {
		field([]byte{0})
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcpgo.TextContent); ok {
			field([]byte(tc.Text))
		}
	}
	return hex.EncodeToString(m.Sum(nil)[:watchDigestBytes])
}

// acquire registers a watch for p, or reports false when the server or p's
// identity is at its limit. Check and register happen under one lock.
func (w *watchHandler) acquire(p principal.Principal) (*watchEntry, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.active) >= watchMaxPerServer {
		return nil, false
	}
	var slot chan struct{}
	n := 0
	for _, e := range w.active {
		if e.p.SameIdentity(p) {
			n++
			slot = e.slot
		}
	}
	if n >= watchMaxPerIdentity {
		return nil, false
	}
	if slot == nil {
		slot = make(chan struct{}, 1)
	}
	e := &watchEntry{p: p, slot: slot}
	w.active = append(w.active, e)
	return e, true
}

func (w *watchHandler) release(e *watchEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.active = slices.DeleteFunc(w.active, func(x *watchEntry) bool { return x == e })
}

func (w *watchHandler) parseTargets(raw any) ([]watchTarget, error) {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, errors.New("targets: a non-empty array of {tool, args} is required")
	}
	if len(list) > watchMaxTargets {
		return nil, fmt.Errorf("targets: at most %d", watchMaxTargets)
	}
	out := make([]watchTarget, 0, len(list))
	inboxes := 0
	for i, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("targets[%d]: must be an object {tool, args}", i)
		}
		for k := range obj {
			if k != "tool" && k != "args" {
				return nil, fmt.Errorf("targets[%d]: unknown field %q; a target takes tool, args", i, k)
			}
		}
		tool, _ := obj["tool"].(string)
		h, ok := w.reads[tool]
		if !ok {
			return nil, fmt.Errorf("targets[%d]: tool must be one of %s", i, strings.Join(w.names, ", "))
		}
		args := map[string]any{}
		if a, present := obj["args"]; present && a != nil {
			if args, ok = a.(map[string]any); !ok {
				return nil, fmt.Errorf("targets[%d]: args must be an object", i)
			}
		}
		// encoding/json sorts map keys, so equal targets get equal keys.
		canon, err := json.Marshal(map[string]any{"tool": tool, "args": args})
		if err != nil { // coverage-ignore: decoded JSON always re-marshals
			return nil, fmt.Errorf("targets[%d]: %w", i, err)
		}
		var channel *watchChannel
		switch tool {
		case toolReadChannel, toolReadSession:
			channel, args = channelTarget(args)
		case toolListInbox:
			inboxes++
			if inboxes > 1 {
				return nil, fmt.Errorf("targets[%d]: at most one %s target per watch", i, toolListInbox)
			}
			channel = &watchChannel{inbox: true}
		}
		rawArgs, err := json.Marshal(args)
		if err != nil { // coverage-ignore: decoded JSON always re-marshals
			return nil, fmt.Errorf("targets[%d]: %w", i, err)
		}
		sum := sha256.Sum256(canon)
		out = append(out, watchTarget{
			tool: tool, args: rawArgs, handler: h, channel: channel,
			key: hex.EncodeToString(sum[:watchDigestBytes]),
		})
	}
	return out, nil
}

// channelTarget returns the channel a read_channel or read_session target
// names, and the arguments its replay passes: the target's own, with the
// page limit of a follower capped at watchChannelLimit. A page target keeps its own limit,
// which the tool bounds, so a change to any message on its page is seen.
// Arguments of the wrong type are kept as given, so the replay answers the
// same error the tool does.
func channelTarget(args map[string]any) (c *watchChannel, replay map[string]any) {
	c = &watchChannel{}
	if id, ok := args["id"].(string); ok {
		c.anchor, _, _ = strings.Cut(trimID(id), "@")
	}
	c.typ, _ = args[chanTypeArg].(string)
	c.name, _ = args[chanNameArg].(string)
	c.follows = args[chanAfterRevArg] != nil || args[chanAfterArg] != nil
	if !c.follows {
		return c, args
	}
	replay = maps.Clone(args)
	switch limit := args["limit"].(type) {
	case nil:
		replay["limit"] = float64(watchChannelLimit)
	case float64:
		if limit > watchChannelLimit {
			replay["limit"] = float64(watchChannelLimit)
		}
	}
	return c, replay
}

func parseWatchTimeout(raw any) (time.Duration, error) {
	if raw == nil {
		return watchDefaultTimeout, nil
	}
	f, ok := raw.(float64)
	// Compare as a float: converting a huge value to a Duration overflows.
	if !ok || !(f >= 0 && f <= watchMaxTimeout.Seconds()) {
		return 0, fmt.Errorf("timeout: seconds from 0 to %d", int(watchMaxTimeout/time.Second))
	}
	return time.Duration(f * float64(time.Second)), nil
}

// decodeWatchCursor returns the digests in a cursor a watch returned.
func decodeWatchCursor(cursor string) (map[string]string, error) {
	if len(cursor) > watchCursorMaxLen {
		return nil, errors.New("cursor too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, err
	}
	var c watchCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.V != watchCursorVersion || c.H == nil {
		return nil, errors.New("unsupported cursor")
	}
	return c.H, nil
}
