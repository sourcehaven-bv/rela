package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// A busy list as a live shape (TKT-W5ZSYJ): a channel move on a row of
// the page replays a busy list subscription, a message-only write does
// not, and a move from another process reaches it on its safety turn.

const (
	busyRunBoard = "/api/v1/tickets?busy=run"
	busyRunPath1 = "/api/v1/tickets/TKT-1/_channels/run"
)

// busyLiveEnv is a live env over a busy env, with the round clock driven
// by hand and every principal in the last safety slice, so the safety
// replay runs on a known tick only.
//
// Tick budget: the first safety turn is tick safetyPeriod-inboxGrid (24 on
// [LiveInboxGrid], 16 on [LiveInboxGridScan]), and it replays every board
// with a wake. A test that asserts which boards a tick replayed, and is
// not about the safety replay, must stay below it: call
// requireBeforeSafety before such an assertion.
type busyLiveEnv struct {
	*liveEnv
	e     *busyEnv
	local *realtime.Local
	ticks *liveStepper
}

// newBusyLiveEnv seeds the busy env under policy with seed, on the inbox
// grid grid, and starts its live worker on a manual clock.
func newBusyLiveEnv(t *testing.T, policy *acl.Policy, seed func(app *App), grid int) *busyLiveEnv {
	t.Helper()
	return newBusyLiveEnvWith(t, policy, seed, grid, nil)
}

// newBusyLiveEnvWith is newBusyLiveEnv with setup run on the app before
// the router is built and the worker starts. Nil: no setup.
func newBusyLiveEnvWith(
	t *testing.T, policy *acl.Policy, seed func(app *App), grid int, setup func(app *App),
) *busyLiveEnv {
	t.Helper()
	local, err := realtime.NewLocal(time.Now)
	require.NoError(t, err)
	t.Cleanup(local.Stop)
	e := newBusyEnvLocal(t, policy, seed, nil, local)
	require.NoError(t, SetInboxGrid(e.app, grid))
	last := e.app.live.safetySlices() - 1
	e.app.live.safetySliceOf = func(string) uint64 { return last }
	env := &busyLiveEnv{e: e, local: local}
	env.liveEnv = newLiveEnvFor(t, e.app, func(app *App) {
		env.ticks = manualClock(app)
		if setup != nil {
			setup(app)
		}
	})
	return env
}

// firstSafety is the first tick that runs a safety turn for the env's
// principals, who all sit in the last slice.
func (b *busyLiveEnv) firstSafety() uint64 {
	h := b.app.live
	return h.safetyPeriod() - h.inboxGrid
}

// requireBeforeSafety fails the test when the next tick would be a safety
// turn: a replay count taken there would include the safety replay.
func (b *busyLiveEnv) requireBeforeSafety(t *testing.T) {
	t.Helper()
	require.Less(t, b.ticks.next(), b.firstSafety(),
		"the test reached the first safety tick; its replay counts would include the safety replay")
}

// stepUntil runs one tick at a time until cond holds. It is for a graph
// write, which reaches the worker through the change feed's pump, off the
// test goroutine: a fixed number of ticks may all run before it arrives.
// It fails at the first safety tick, or after ten seconds.
func (b *busyLiveEnv) stepUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		b.requireBeforeSafety(t)
		b.ticks.step(t, 1)
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no tick before the deadline: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// newBusyTickets is newBusyLiveEnv under busyPolicy, with TKT-1 and TKT-2
// owned by alice and run by the runner.
func newBusyTickets(t *testing.T) *busyLiveEnv {
	t.Helper()
	return newBusyLiveEnv(t, busyPolicy(), func(app *App) {
		seedEntity(app, ownedTicket("TKT-1"))
		seedEntity(app, ownedTicket("TKT-2"))
	}, LiveInboxGrid)
}

// ctxOf is user's context with the read gate attached, as a request has.
func (b *busyLiveEnv) ctxOf(t *testing.T, user string) context.Context {
	t.Helper()
	return gateCtxFor(principalCtx(user), t, b.e.d)
}

// ticketChannel is the unnamed channel of chtype on ticket id.
func ticketChannel(id, chtype string) realtime.RawAddress {
	return realtime.RawAddress{EntityType: "ticket", Anchor: id, Type: chtype}
}

// postOn posts typ with props to the chtype channel of ticket id as user,
// through the realtime service, and returns the message's seq.
func (b *busyLiveEnv) postOn(t *testing.T, user, id, chtype, typ string, props map[string]any) int64 {
	t.Helper()
	res, err := b.e.ch.svc.Post(b.ctxOf(t, user), ticketChannel(id, chtype), realtime.PostInput{Type: typ, Props: props})
	require.NoError(t, err)
	require.NotNil(t, res.Message)
	return res.Message.Seq
}

// lifecycle runs op on the chtype channel of ticket id as user.
func (b *busyLiveEnv) lifecycle(t *testing.T, user, id, chtype, op string) {
	t.Helper()
	_, err := b.e.ch.svc.Lifecycle(b.ctxOf(t, user), ticketChannel(id, chtype), op)
	require.NoError(t, err)
}

// replayURL is the URL the hub replays for the subscription path p.
func (b *busyLiveEnv) replayURL(t *testing.T, p string) string {
	t.Helper()
	lp, err := parseLivePath(p, b.app.Meta())
	require.NoError(t, err)
	return lp.String()
}

// replaysOf is how often got replayed path as user.
func (b *busyLiveEnv) replaysOf(t *testing.T, got map[string]int, user, path string) int {
	t.Helper()
	return got[user+" "+b.replayURL(t, path)]
}

// stream opens a stream as user with subs and waits for their baselines.
func (b *busyLiveEnv) stream(t *testing.T, user string, subs ...liveSubEntry) *sseClient {
	t.Helper()
	c, conn := b.open(t, user)
	b.subscribe(t, user, conn, subs...)
	return c
}

// isPending reports whether alice's subscription under key waits for a
// pending replay.
func (h *liveHub) isPending(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.conns {
		if c.owner.User != "alice" {
			continue
		}
		c.mu.Lock()
		s := c.subs[key]
		c.mu.Unlock()
		if s != nil {
			_, ok := h.pending[s]
			return ok
		}
	}
	return false
}

// anchorsOf returns the sorted anchors of w, or nil for no wake.
func anchorsOf(w *busyWake) []string {
	if w == nil {
		return nil
	}
	out := make([]string, 0, len(w.anchors))
	for a := range w.anchors {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

// TestRecordBusy pins AC 2 and AC 10: a family row adds its id, a face row
// its id and its id@face address, a full page holds at most two keys per
// row, and a request without a probe or an empty page records nothing.
func TestRecordBusy(t *testing.T) {
	t.Run("family and face rows", func(t *testing.T) {
		ctx, p := withTopicProbe(context.Background())
		recordBusy(ctx, "run", []*entity.Entity{
			{ID: "TKT-1", Type: "ticket"},
			{ID: "PAGE-1", Type: "page", Face: "draft"},
		})
		require.NotNil(t, p.busy)
		require.Equal(t, "run", p.busy.typ)
		require.Equal(t, []string{"PAGE-1", "PAGE-1@draft", "TKT-1"}, anchorsOf(p.busy))
	})
	t.Run("full page", func(t *testing.T) {
		ctx, p := withTopicProbe(context.Background())
		rows := make([]*entity.Entity, 100)
		for i := range rows {
			rows[i] = &entity.Entity{ID: fmt.Sprintf("PAGE-%d", i), Type: "page", Face: "draft"}
		}
		recordBusy(ctx, "facerun", rows)
		require.Len(t, p.busy.anchors, 200)
	})
	t.Run("empty page", func(t *testing.T) {
		ctx, p := withTopicProbe(context.Background())
		recordBusy(ctx, "run", nil)
		require.Nil(t, p.busy)
	})
	t.Run("no probe", func(t *testing.T) {
		require.NotPanics(t, func() {
			recordBusy(context.Background(), "run", []*entity.Entity{{ID: "TKT-1", Type: "ticket"}})
		})
	})
}

// TestRoundFilter_SelectsBusy pins AC 3: a busy subscription is selected
// by a moved key of its wake's type on an anchor of its page, or by a
// resync when it has a wake, and by nothing else.
func TestRoundFilter_SelectsBusy(t *testing.T) {
	key := func(id string, face entity.Face, typ string) channels.Key {
		return channels.Key{Anchor: channels.Anchor{ID: id, Face: face}, Type: typ}
	}
	wake := &busyWake{typ: "run", anchors: map[string]bool{
		"TKT-1": true, "PAGE-1": true, "PAGE-1@published": true,
	}}
	for _, tc := range []struct {
		name    string
		wake    *busyWake
		moved   []channels.Key
		durable []channels.Key
		resync  bool
		want    bool
	}{
		{name: "family key on a row", wake: wake, moved: []channels.Key{key("TKT-1", "", "run")}, want: true},
		{name: "family key on a face row", wake: wake, moved: []channels.Key{key("PAGE-1", "", "run")}, want: true},
		{
			name: "face key on the row's face", wake: wake,
			moved: []channels.Key{key("PAGE-1", "published", "run")}, want: true,
		},
		{name: "face key on another face", wake: wake, moved: []channels.Key{key("PAGE-1", "draft", "run")}},
		{name: "anchor not on the page", wake: wake, moved: []channels.Key{key("TKT-2", "", "run")}},
		{name: "another type", wake: wake, moved: []channels.Key{key("TKT-1", "", "chat")}},
		{name: "durable key not moved", wake: wake, durable: []channels.Key{key("TKT-1", "", "run")}},
		{name: "resync", wake: wake, resync: true, want: true},
		{name: "resync without a wake", resync: true},
		{name: "moved key without a wake", moved: []channels.Key{key("TKT-1", "", "run")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keys := slices.Concat(tc.moved, tc.durable)
			f := newRoundFilter(nil, keys, tc.resync)
			f.addInbox(keys, nil)
			f.addMoved(tc.moved)
			s := &liveSub{url: busyRunBoard, busy: "run", busyWake: tc.wake}
			require.Equal(t, tc.want, f.selects(s))
		})
	}
}

// failingBusy is a busyReader whose read fails with a store error.
type failingBusy struct{}

func (failingBusy) Busy(context.Context, string, []realtime.BusyAnchor) (
	rows map[entity.Address][]realtime.BusyView, ok bool, err error,
) {
	return nil, false, errors.New("channel store unavailable")
}

// TestLive_BusyWakeSetFromReplay pins AC 2: a 200 replay whose busy read
// was served teaches the wake, and so does one whose busy read failed with
// a store error; an empty page, a read that was not served and a refused
// replay leave none; an unfinished replay keeps it.
func TestLive_BusyWakeSetFromReplay(t *testing.T) {
	t.Run("served", func(t *testing.T) {
		env := newBusyTickets(t)
		env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
		w := env.app.live.busyWakeOf("alice", "board")
		require.NotNil(t, w)
		require.Equal(t, "run", w.typ)
		require.Equal(t, []string{"TKT-1", "TKT-2"}, anchorsOf(w))
	})
	t.Run("busy read fails", func(t *testing.T) {
		env := newBusyLiveEnvWith(t, busyPolicy(), func(app *App) {
			seedEntity(app, ownedTicket("TKT-1"))
			seedEntity(app, ownedTicket("TKT-2"))
		}, LiveInboxGrid, func(app *App) { app.channels.busy = failingBusy{} })
		env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
		code, body := env.get(t, "alice", busyRunBoard)
		require.Equal(t, http.StatusOK, code)
		require.NotContains(t, string(body), `"_busy"`, "a failed busy read omits the field")
		w := env.app.live.busyWakeOf("alice", "board")
		require.NotNil(t, w, "a store error keeps the board in the live wakes")
		require.Equal(t, []string{"TKT-1", "TKT-2"}, anchorsOf(w))

		replays := countPrincipalReplays(t, env.app.live)
		env.local.Mark(channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "run"})
		env.ticks.step(t, 1)
		got := replays.take()
		require.Equal(t, 1, env.replaysOf(t, got, "alice", busyRunBoard), "%v", got)
	})
	for _, tc := range []struct {
		name  string
		path  string
		setup func(env *busyLiveEnv)
	}{
		{name: "empty page", path: busyRunBoard + "&filter[title]=nothing"},
		{name: "realtime off", path: busyRunBoard, setup: func(env *busyLiveEnv) { env.e.ch.cfg.Store(nil) }},
		{name: "unknown type", path: "/api/v1/tickets?busy=nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newBusyTickets(t)
			if tc.setup != nil {
				tc.setup(env)
			}
			env.stream(t, "alice", liveSubEntry{"board", tc.path})
			require.NotEmpty(t, env.app.live.hashOf("alice"), "the list answered")
			require.Nil(t, env.app.live.busyWakeOf("alice", "board"))
		})
	}
	t.Run("membership removed empties the page", func(t *testing.T) {
		// alice reads tickets only through her membership of readers.
		policy := busyPolicy()
		delete(policy.Assignments, "alice")
		policy.Assignments["readers"] = "reader"
		env := newBusyLiveEnv(t, policy, func(app *App) {
			seedEntity(app, ownedTicket("TKT-1"))
			seedEntity(app, &entity.Entity{ID: "readers", Type: "person", Properties: map[string]any{"title": "r"}})
			seedRelation(app, entity.NewRelation("alice", "member-of", "readers"))
		}, LiveInboxGrid)
		c := env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
		env.ticks.step(t, 1)
		require.NotNil(t, env.app.live.busyWakeOf("alice", "board"))

		replays := countPrincipalReplays(t, env.app.live)
		k := entity.RelationKey{From: "alice", Type: "member-of", To: "readers"}
		require.NoError(t, env.app.store.DeleteRelation(context.Background(), k))
		env.stepUntil(t, "the full round of the graph write", func() bool {
			return replays.count("alice "+env.replayURL(t, busyRunBoard)) > 0
		})
		requireChanged(t, c, "board")
		// Without a role the list answers 200 with no rows, so the wake
		// clears through the empty page; a refused replay is the unit case
		// below.
		code, body := env.get(t, "alice", busyRunBoard)
		require.Equal(t, http.StatusOK, code)
		require.Contains(t, string(body), `"data":[]`)
		require.Nil(t, env.app.live.busyWakeOf("alice", "board"))
	})
	t.Run("type removed from realtime.yaml", func(t *testing.T) {
		env := newBusyTickets(t)
		c := env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
		env.ticks.step(t, 1)
		require.NotNil(t, env.app.live.busyWakeOf("alice", "board"))

		renamed := strings.Replace(busyRowsRealtime, "\n  run:\n", "\n  run2:\n", 1)
		require.NotEqual(t, busyRowsRealtime, renamed, "the run type was not renamed")
		env.e.ch.cfg.Store(env.e.ch.parse(t, renamed))
		env.app.live.policyChanged()
		// The reload and the tick reach the worker's select together, so
		// the full round may run a tick later.
		env.stepUntil(t, "the full round of the reload", func() bool {
			return env.app.live.busyWakeOf("alice", "board") == nil
		})
		requireChanged(t, c, "board")

		replays := countPrincipalReplays(t, env.app.live)
		env.local.Mark(channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "run"})
		env.ticks.step(t, 1)
		env.requireBeforeSafety(t)
		got := replays.take()
		require.Zero(t, env.replaysOf(t, got, "alice", busyRunBoard), "%v", got)
		liveQuiet(t, c)
	})
	t.Run("unfinished replay keeps it", func(t *testing.T) {
		w := &busyWake{typ: "run"}
		s := &liveSub{busy: "run", busyWake: w}
		applyTopic(s, replayResult{})
		require.Same(t, w, s.busyWake)
		applyTopic(s, replayResult{ok: true, status: http.StatusOK})
		require.Nil(t, s.busyWake, "a 200 whose busy read was not served clears it")
		s.busyWake = w
		applyTopic(s, replayResult{ok: true, status: http.StatusNotFound})
		require.Nil(t, s.busyWake)
	})
	t.Run("plain list holds none", func(t *testing.T) {
		s := &liveSub{url: "/api/v1/tickets"}
		applyTopic(s, replayResult{ok: true, status: http.StatusOK, busy: &busyWake{typ: "run"}})
		require.Nil(t, s.busyWake)
	})
}

// TestLive_BusyListWakesOnSessionMove pins AC 4: a session move, a channel
// create, a lifecycle move and a Local.Mark made in this process replay
// the board within one tick. The replay frames only when `_busy` moved.
func TestLive_BusyListWakesOnSessionMove(t *testing.T) {
	env := newBusyTickets(t)
	const jobBoard, taskBoard = "/api/v1/tickets?busy=job", "/api/v1/tickets?busy=task"
	c := env.stream(t, "alice", liveSubEntry{"board", busyRunBoard}, liveSubEntry{"run", busyRunPath1})
	cJob := env.stream(t, "alice", liveSubEntry{"job", jobBoard})
	cTask := env.stream(t, "alice", liveSubEntry{"task", taskBoard})
	env.ticks.step(t, 1)
	replays := countPrincipalReplays(t, env.app.live)
	// The steps share the env and build on each other, so they are not
	// subtests: one run alone would test the wrong thing.
	step := func(name string, f func()) {
		t.Helper()
		t.Log(name)
		f()
	}
	busyOn := func(path string) []string {
		t.Helper()
		var out []string
		for id, vs := range liveBusy(t, env.liveEnv, "alice", path) {
			if len(vs) > 0 {
				out = append(out, id)
			}
		}
		slices.Sort(out)
		return out
	}

	step("move that changes nothing", func() {
		env.postOn(t, "alice", "TKT-1", "run", "go", nil)
		env.ticks.step(t, 1)
		requireChanged(t, c, "run")
		liveQuiet(t, c)
		require.Equal(t, 1, env.replaysOf(t, replays.take(), "alice", busyRunBoard))
	})
	step("claim", func() {
		env.postOn(t, "runner", "TKT-1", "run", "claim", map[string]any{"worktree": "wt"})
		env.ticks.step(t, 1)
		requireChanged(t, c, "board", "run")
		require.Equal(t, []string{"TKT-1"}, busyOn(busyRunBoard))
	})
	step("plan", func() {
		env.postOn(t, "runner", "TKT-1", "run", "plan", map[string]any{"plan": "do it"})
		env.ticks.step(t, 1)
		requireChanged(t, c, "board", "run")
		require.Empty(t, busyOn(busyRunBoard))
	})
	step("first post starts working", func() {
		env.postOn(t, "alice", "TKT-1", "job", "log", map[string]any{"body": "start"})
		env.ticks.step(t, 1)
		requireChanged(t, cJob, "job")
		require.Equal(t, []string{"TKT-1"}, busyOn(jobBoard))
	})
	step("create", func() {
		_, err := env.e.ch.svc.Create(env.ctxOf(t, "alice"), "ticket", "TKT-1", "task", realtime.CreateInput{})
		require.NoError(t, err)
		env.ticks.step(t, 1)
		requireChanged(t, cTask, "task")
		require.Equal(t, []string{"TKT-1"}, busyOn(taskBoard))
	})
	for _, tc := range []struct {
		op   string
		busy []string
	}{
		{realtimecfg.LifecycleClose, nil},
		{realtimecfg.LifecycleReopen, []string{"TKT-1"}},
		{realtimecfg.LifecycleDelete, nil},
	} {
		step(tc.op, func() {
			env.lifecycle(t, "alice", "TKT-1", "job", tc.op)
			env.ticks.step(t, 1)
			requireChanged(t, cJob, "job")
			require.Equal(t, tc.busy, busyOn(jobBoard))
		})
	}
	step("Local.Mark", func() {
		env.e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "job", time.Now())
		env.ticks.step(t, 1)
		liveQuiet(t, cJob)
		env.local.Mark(channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "job"})
		env.ticks.step(t, 1)
		requireChanged(t, cJob, "job")
		require.Equal(t, []string{"TKT-1"}, busyOn(jobBoard))
	})
	liveQuiet(t, cTask)
}

// TestLive_BusyListIgnoresMessageOnly pins AC 5: a message-only write, a
// move of another channel type and a move on an anchor off the page do not
// replay the board. Each case steps one tick in which alice's TKT-1 run
// channel subscription is replayed, so the round ran.
func TestLive_BusyListIgnoresMessageOnly(t *testing.T) {
	env := newBusyTickets(t)
	const board = busyRunBoard + "&per_page=1"
	at := time.Now()
	env.e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", at)
	env.postOn(t, "alice", "TKT-1", "run", "go", nil)
	env.stream(t, "alice", liveSubEntry{"board", board}, liveSubEntry{"run", busyRunPath1})
	env.ticks.step(t, 1)
	require.Equal(t, []string{"TKT-1"}, anchorsOf(env.app.live.busyWakeOf("alice", "board")),
		"the board shows TKT-1 only")
	replays := countPrincipalReplays(t, env.app.live)
	var seq int64
	control := func(t *testing.T) {
		t.Helper()
		seq = env.postOn(t, "alice", "TKT-1", "run", "go", map[string]any{"note": "still queued"})
	}
	for _, tc := range []struct {
		name string
		op   func(t *testing.T)
	}{
		{"post in run that moves nothing", control},
		{"edit", func(t *testing.T) {
			t.Helper()
			_, err := env.e.ch.svc.Edit(env.ctxOf(t, "alice"), ticketChannel("TKT-1", "run"), seq,
				map[string]any{"note": "edited"})
			require.NoError(t, err)
		}},
		{"removal", func(t *testing.T) {
			t.Helper()
			_, err := env.e.ch.svc.Delete(env.ctxOf(t, "alice"), ticketChannel("TKT-1", "run"), seq, "")
			require.NoError(t, err)
		}},
		{"typing", func(t *testing.T) {
			t.Helper()
			_, err := env.e.ch.svc.Typing(env.ctxOf(t, "alice"), ticketChannel("TKT-1", "run"))
			require.NoError(t, err)
		}},
		{"move in chat", func(t *testing.T) {
			t.Helper()
			env.postOn(t, "alice", "TKT-1", "chat", "note", map[string]any{"body": "x"})
			control(t)
		}},
		{"move in instant", func(t *testing.T) {
			t.Helper()
			env.postOn(t, "alice", "TKT-1", "instant", "note", map[string]any{"body": "x"})
			control(t)
		}},
		{"run move off the page", func(t *testing.T) {
			t.Helper()
			env.postOn(t, "alice", "TKT-2", "run", "go", nil)
			control(t)
		}},
	} {
		// The cases share the env, and edit and removal need the seq of
		// the post before them, so they are not subtests.
		t.Log(tc.name)
		tc.op(t)
		env.ticks.step(t, 1)
		env.requireBeforeSafety(t)
		got := replays.take()
		require.Positive(t, env.replaysOf(t, got, "alice", busyRunPath1), "%s: the round ran: %v", tc.name, got)
		require.Zero(t, env.replaysOf(t, got, "alice", board), "%s: %v", tc.name, got)
	}
}

// TestLive_BusyListPagedWake pins AC 8 of TKT-TU3XH4: a list view's paged
// live path records a wake with its own page's rows only. A session move on
// a page-2 row replays it and, once the row turns busy, frames it; a move
// on a page-1 row does not select it. Each negative step has a positive
// control in the same round: alice's TKT-1 run channel subscription.
func TestLive_BusyListPagedWake(t *testing.T) {
	env := newBusyLiveEnv(t, busyPolicy(), func(app *App) {
		for _, id := range []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"} {
			seedEntity(app, ownedTicket(id))
		}
		app.State().Cfg.Lists = map[string]dataentryconfig.List{"all": {
			EntityType: "ticket",
			Columns:    []dataentryconfig.ListColumn{{Property: "title"}, {Busy: "run"}},
		}}
	}, LiveInboxGrid)
	const page2 = "/api/v1/tickets?list_id=all&page=2&per_page=2&busy=run"
	c := env.stream(t, "alice", liveSubEntry{"page2", page2}, liveSubEntry{"run", busyRunPath1})
	env.ticks.step(t, 1)
	require.Equal(t, []string{"TKT-3", "TKT-4"}, anchorsOf(env.app.live.busyWakeOf("alice", "page2")),
		"the wake holds the page-2 rows only")
	replays := countPrincipalReplays(t, env.app.live)

	t.Log("move on a page-1 row")
	env.postOn(t, "alice", "TKT-1", "run", "go", nil)
	env.ticks.step(t, 1)
	env.requireBeforeSafety(t)
	requireChanged(t, c, "run")
	got := replays.take()
	require.Positive(t, env.replaysOf(t, got, "alice", busyRunPath1), "the round ran: %v", got)
	require.Zero(t, env.replaysOf(t, got, "alice", page2), "%v", got)

	t.Log("move on a page-2 row that changes nothing")
	env.postOn(t, "alice", "TKT-3", "run", "go", nil)
	env.ticks.step(t, 1)
	env.requireBeforeSafety(t)
	liveQuiet(t, c)
	require.Equal(t, 1, env.replaysOf(t, replays.take(), "alice", page2))

	t.Log("claim on a page-2 row")
	env.postOn(t, "runner", "TKT-3", "run", "claim", map[string]any{"worktree": "wt"})
	env.ticks.step(t, 1)
	env.requireBeforeSafety(t)
	requireChanged(t, c, "page2")
	busy := liveBusy(t, env.liveEnv, "alice", page2)
	require.Len(t, busy["TKT-3"], 1)
	require.Empty(t, busy["TKT-4"])
}

// TestLive_BusyHiddenAnchorNoWake pins AC 6: a move on an anchor the
// subscriber cannot see, or on a face that is not on its page, replays
// none of its boards.
func TestLive_BusyHiddenAnchorNoWake(t *testing.T) {
	// bob reads TKT-1 only, through officer-of, and pages in the
	// published world.
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reader":  {Read: []string{"ticket", "page", "person"}, Worlds: []string{"published"}},
			"member":  {Read: []string{"page", "person"}, Worlds: []string{"published"}},
			"officer": {Read: []string{"ticket"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"officer-of": {Confers: "officer"}},
		Assignments:   map[string]string{"alice": "reader", "runner": "reader", "bob": "member"},
	}
	seed := func(app *App) {
		seedEntity(app, ownedTicket("TKT-1"))
		seedEntity(app, ownedTicket("TKT-2"))
		seedRelation(app, entity.NewRelation("bob", "officer-of", "TKT-1"))
		for _, f := range []entity.Face{"published", "draft"} {
			seedEntity(app, &entity.Entity{ID: "PAGE-1", Type: "page", Face: f,
				Properties: map[string]any{"title": "p", "owner": "bob"}})
		}
	}

	t.Run("hidden row", func(t *testing.T) {
		env := newBusyLiveEnv(t, policy, seed, LiveInboxGrid)
		ca := env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
		cb := env.stream(t, "bob", liveSubEntry{"board", busyRunBoard})
		env.postOn(t, "alice", "TKT-2", "run", "go", nil)
		env.ticks.step(t, 2)
		liveDrain(ca)
		replays := countPrincipalReplays(t, env.app.live)

		env.postOn(t, "runner", "TKT-2", "run", "claim", map[string]any{"worktree": "wt"})
		env.ticks.step(t, 1)
		env.requireBeforeSafety(t)
		requireChanged(t, ca, "board")
		liveQuiet(t, cb)
		got := replays.take()
		require.Equal(t, 1, env.replaysOf(t, got, "alice", busyRunBoard), "%v", got)
		require.Zero(t, env.replaysOf(t, got, "bob", busyRunBoard), "%v", got)
		require.Equal(t, []string{"TKT-1"}, anchorsOf(env.app.live.busyWakeOf("bob", "board")))
		require.Equal(t, []string{"TKT-1", "TKT-2"}, anchorsOf(env.app.live.busyWakeOf("alice", "board")))
	})

	t.Run("face not on the page", func(t *testing.T) {
		env := newBusyLiveEnv(t, policy, seed, LiveInboxGrid)
		const pages = "/api/v1/pages?busy=facerun&world=published"
		env.stream(t, "bob", liveSubEntry{"pages", pages}, liveSubEntry{"tickets", busyRunBoard})
		env.ticks.step(t, 1)
		require.Equal(t, []string{"PAGE-1", "PAGE-1@published"}, anchorsOf(env.app.live.busyWakeOf("bob", "pages")))
		replays := countPrincipalReplays(t, env.app.live)

		// No busy principal may post in the draft world, so the move is a
		// store write and the Local.Mark a timer or sweep would make.
		draft := channels.Anchor{ID: "PAGE-1", Face: "draft"}
		env.e.seedChannel(t, draft, "facerun", time.Now())
		env.local.Mark(channels.Key{Anchor: draft, Type: "facerun"})
		env.postOn(t, "alice", "TKT-1", "run", "go", nil)
		env.ticks.step(t, 1)
		env.requireBeforeSafety(t)
		got := replays.take()
		require.Equal(t, 1, env.replaysOf(t, got, "bob", busyRunBoard), "%v", got)
		require.Zero(t, env.replaysOf(t, got, "bob", pages), "%v", got)
	})
}

// TestLive_BusyWakePerPrincipal pins AC 7: a move on a channel only alice
// may read replays both boards as their own principals. Only alice's hash
// moves, so only she gets a frame.
func TestLive_BusyWakePerPrincipal(t *testing.T) {
	env := newBusyTickets(t)
	const board = "/api/v1/tickets?busy=instant"
	env.e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
	ca := env.stream(t, "alice", liveSubEntry{"board", board})
	cb := env.stream(t, "bob", liveSubEntry{"board", board})
	env.ticks.step(t, 1)
	require.Empty(t, liveBusy(t, env.liveEnv, "bob", board)["TKT-1"])
	before := [2]string{env.app.live.hashOf("alice"), env.app.live.hashOf("bob")}
	replays := countPrincipalReplays(t, env.app.live)

	env.postOn(t, "alice", "TKT-1", "instant", "note", map[string]any{"body": "done"})
	env.ticks.step(t, 1)
	requireChanged(t, ca, "board")
	liveQuiet(t, cb)
	got := replays.take()
	require.Equal(t, 1, env.replaysOf(t, got, "alice", board), "%v", got)
	require.Equal(t, 1, env.replaysOf(t, got, "bob", board), "%v", got)
	require.NotEqual(t, before[0], env.app.live.hashOf("alice"))
	require.Equal(t, before[1], env.app.live.hashOf("bob"))
	bob := liveBusy(t, env.liveEnv, "bob", board)["TKT-1"]
	require.NotNil(t, bob)
	require.Empty(t, bob)
}

// TestLive_BusyOverflowNoFallbackFrame pins AC 8: a busy board that an
// overflowed topic round skips gets no frame and goes to pending; the
// next tick replays it and frames it. A full round that overflows frames
// it, like any list.
func TestLive_BusyOverflowNoFallbackFrame(t *testing.T) {
	// setup subscribes alice's board and run channel on two streams, and
	// moves the run to queued, which nothing shows as busy.
	setup := func(t *testing.T) (env *busyLiveEnv, board, run *sseClient) {
		t.Helper()
		env = newBusyTickets(t)
		board = env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
		run = env.stream(t, "alice", liveSubEntry{"run", busyRunPath1})
		env.postOn(t, "alice", "TKT-1", "run", "go", nil)
		env.ticks.step(t, 2)
		liveDrain(run)
		return env, board, run
	}
	for _, tc := range []struct {
		name  string
		limit func(h *liveHub)
		// keepsRun is set when the limit keeps the run channel's pair.
		keepsRun bool
	}{
		{"round cap", func(h *liveHub) { h.roundCap = 1 }, true},
		{"round budget", func(h *liveHub) { h.roundBudget = -1 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, board, run := setup(t)
			// The worker waits for the next tick, which the stepper sends
			// after this write. The rotation starts at the first pair, and
			// the run channel's URL sorts before the board's ('/' < '?'),
			// so a cap of one keeps the run channel and cuts the board. A
			// budget below zero cuts both.
			tc.limit(env.app.live)
			env.app.live.presenceRotate = 0
			env.postOn(t, "runner", "TKT-1", "run", "claim", map[string]any{"worktree": "wt"})
			env.ticks.step(t, 1)
			require.True(t, env.app.live.isPending("board"), "the round cut the board")
			require.Equal(t, !tc.keepsRun, env.app.live.isPending("run"))
			requireChanged(t, run, "run")
			liveQuiet(t, board)

			env.app.live.roundCap, env.app.live.roundBudget = liveRoundCap, liveRoundBudget
			env.ticks.step(t, 1)
			requireChanged(t, board, "board")
		})
	}
	t.Run("full round frames", func(t *testing.T) {
		env, board, _ := setup(t)
		env.app.live.roundBudget = -1
		update(t, env.app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
		env.stepUntil(t, "the full round of the graph write", func() bool {
			return env.app.live.isPending("board")
		})
		requireChanged(t, board, "board")
	})
	t.Run("fallback rule", func(t *testing.T) {
		plain, presence := liveSub{url: "/api/v1/tickets"}, liveSub{url: "/api/v1/tickets/TKT-1", presenceID: "TKT-1"}
		for _, tc := range []struct {
			name    string
			sub     liveSub
			kind    roundKind
			dueOnly bool
			frame   bool
		}{
			{"plain list in a full round", plain, fullRound, false, true},
			{"plain list in a topic round", plain, topicRound, false, true},
			{"plain list a topic round took only as pending", plain, topicRound, true, false},
			{"presence a topic round took only as pending", presence, topicRound, true, false},
			{"busy list in a full round", liveSub{url: busyRunBoard, busy: "run"}, fullRound, false, true},
			{"busy list in a topic round", liveSub{url: busyRunBoard, busy: "run"}, topicRound, false, false},
			{"inbox in a full round", liveSub{url: inboxPath, inbox: true}, fullRound, false, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				h := newLiveHub(nil)
				c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
				s := tc.sub
				c.subs["k"] = &s
				ref := subRef{conn: c, key: "k", sub: &s, dueOnly: tc.dueOnly}
				h.fallback([]*replayPair{{subs: []subRef{ref}}}, tc.kind)
				require.Equal(t, tc.frame, len(c.takeChanged()) == 1)
				require.True(t, h.hasPending())
			})
		}
	})
}

// TestLive_BusyListRecheckWhileSyncing pins AC 9: a claim that lands while
// the board's PUT takes its baseline leaves the board pending, and the
// next tick replays and frames it.
func TestLive_BusyListRecheckWhileSyncing(t *testing.T) {
	env := newBusyTickets(t)
	env.postOn(t, "alice", "TKT-1", "run", "go", nil)
	env.ticks.step(t, 1)
	c, conn := env.open(t, "alice")
	ctx := env.ctxOf(t, "runner")
	var err error
	ticked := false
	hookBaseline(env.liveEnv, func() {
		_, err = env.e.ch.svc.Post(ctx, ticketChannel("TKT-1", "run"),
			realtime.PostInput{Type: "claim", Props: map[string]any{"worktree": "wt"}})
		ticked = env.ticks.tryStep()
	})
	if code, body := env.put(t, as("alice"), conn, liveSubEntry{"board", busyRunBoard}); code != http.StatusNoContent {
		t.Fatalf("PUT = %d %s", code, body)
	}
	require.NoError(t, err)
	require.True(t, ticked, "the live worker did not tick during the baseline")
	require.True(t, env.app.live.isPending("board"))

	env.ticks.step(t, 1)
	requireChanged(t, c, "board")
}

// TestLive_BusyWakeCoalesces pins AC 10: twenty moves on the board's rows
// within one tick replay it once.
func TestLive_BusyWakeCoalesces(t *testing.T) {
	env := newBusyTickets(t)
	env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
	env.ticks.step(t, 1)
	replays := countPrincipalReplays(t, env.app.live)
	for i := range 20 {
		id := []string{"TKT-1", "TKT-2"}[i%2]
		env.local.Mark(channels.Key{Anchor: channels.Anchor{ID: id}, Type: "run"})
	}
	env.ticks.step(t, 1)
	got := replays.take()
	require.Equal(t, 1, env.replaysOf(t, got, "alice", busyRunBoard), "%v", got)
}

// TestLive_BusyMovedKeyAloneStartsRound pins the lost-wake fix: a tick
// whose channel changes carry a moved key and nothing else still runs a
// topic round and replays the board. With one Take call per set, a mark
// that landed between Take and TakeMoved gave a tick exactly that, and
// the moved key was dropped. TakeLive now takes every set at once
// (realtime.TestTakeLive_OneCriticalSection); this pins that the hub also
// starts the round on such a tick.
func TestLive_BusyMovedKeyAloneStartsRound(t *testing.T) {
	fake := &fakeChanges{wake: make(chan struct{}, 1)}
	env := newBusyLiveEnvWith(t, busyPolicy(), func(app *App) {
		seedEntity(app, ownedTicket("TKT-1"))
		seedEntity(app, ownedTicket("TKT-2"))
	}, LiveInboxGrid, func(app *App) {
		app.channels.changes = func() channelChanges { return fake }
	})
	env.stream(t, "alice", liveSubEntry{"board", busyRunBoard})
	env.ticks.step(t, 1)
	require.NotNil(t, env.app.live.busyWakeOf("alice", "board"))
	replays := countPrincipalReplays(t, env.app.live)

	fake.send(realtime.LiveChanges{Moved: []channels.Key{{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "run"}}})
	env.ticks.step(t, 1)
	env.requireBeforeSafety(t)
	got := replays.take()
	require.Equal(t, 1, env.replaysOf(t, got, "alice", busyRunBoard), "%v", got)
}

// TestLiveHub_CollectMarksDueOnly pins which pairs a topic round took only
// because they were pending: those the round's changes and the safety turn
// did not select. A full round marks none.
func TestLiveHub_CollectMarksDueOnly(t *testing.T) {
	h := newLiveHub(nil)
	c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	list := &liveSub{url: "/api/v1/tickets", hash: "h"}
	board := &liveSub{url: busyRunBoard, busy: "run", hash: "h", busyWake: &busyWake{
		typ: "run", anchors: map[string]bool{"TKT-1": true},
	}}
	c.subs["list"], c.subs["board"] = list, board
	pend := func() {
		h.mu.Lock()
		h.addPendingLocked(list)
		h.addPendingLocked(board)
		h.mu.Unlock()
	}
	dueOnly := func(pairs []*replayPair) map[string]bool {
		out := map[string]bool{}
		for _, p := range pairs {
			for _, ref := range p.subs {
				out[ref.key] = ref.dueOnly
			}
		}
		return out
	}

	pend()
	f := newRoundFilter(nil, nil, false)
	f.addMoved([]channels.Key{{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "run"}})
	require.Equal(t, map[string]bool{"list": true, "board": false}, dueOnly(h.collect(true, f)),
		"the moved key selected the board; the list came only from pending")

	pend()
	require.Equal(t, map[string]bool{"list": false, "board": false}, dueOnly(h.collect(true, nil)),
		"a full round marks none")
}

// TestLive_BusySafetyReplay pins AC 12: a move made straight in the
// channel store, as another process makes it, reaches the board on its
// principal's safety turn, and the next safety turn replays it again with
// no frame. A busy subscription without a wake is not taken.
func TestLive_BusySafetyReplay(t *testing.T) {
	for _, tc := range []struct{ grid, safety int }{{LiveInboxGrid, 28}, {LiveInboxGridScan, 24}} {
		t.Run(fmt.Sprintf("grid %d", tc.grid), func(t *testing.T) {
			env := newBusyLiveEnv(t, busyPolicy(), func(app *App) { seedEntity(app, ownedTicket("TKT-1")) }, tc.grid)
			require.Equal(t, uint64(tc.safety), env.app.live.safetyPeriod())
			const board, unserved = "/api/v1/tickets?busy=instant", "/api/v1/tickets?busy=nope"
			c := env.stream(t, "alice", liveSubEntry{"board", board}, liveSubEntry{"unserved", unserved})
			env.ticks.step(t, 1)
			replays := countPrincipalReplays(t, env.app.live)

			env.e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
			at := uint64(tc.safety - tc.grid) // the last slice's first grid tick
			env.ticks.through(t, at-1)
			liveQuiet(t, c)
			got := replays.take()
			require.Zero(t, env.replaysOf(t, got, "alice", board), "%v", got)

			env.ticks.step(t, 1) // tick at, alice's safety turn
			requireChanged(t, c, "board")
			got = replays.take()
			require.Equal(t, 1, env.replaysOf(t, got, "alice", board), "%v", got)
			require.Zero(t, env.replaysOf(t, got, "alice", unserved), "a subscription without a wake: %v", got)

			env.ticks.through(t, at+uint64(tc.safety)-1)
			got = replays.take()
			require.Zero(t, env.replaysOf(t, got, "alice", board), "%v", got)
			env.ticks.step(t, 1) // the next safety turn
			liveQuiet(t, c)
			got = replays.take()
			require.Equal(t, 1, env.replaysOf(t, got, "alice", board), "%v", got)
			require.Zero(t, env.replaysOf(t, got, "alice", unserved), "%v", got)
		})
	}
}
