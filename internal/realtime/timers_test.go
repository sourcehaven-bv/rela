package realtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// noContainment is the timer of the detected state in timerRealtime.
const noContainment = `{ no-containment: { period: 24h, to: overdue, label: Not contained within a day } }`

// timerRealtimeWith declares breach-response, the breach session of
// realtimecfg's testdata/valid.yaml on tickets, with detectedAfter as the
// timers of its detected state ("" for none); cases, a cardinality-many
// type whose open state times out into a final one; and chain, whose
// second timer counts from the session's start. alice owns TKT-1, so she
// holds lead.
func timerRealtimeWith(detectedAfter string) string {
	after := ""
	if detectedAfter != "" {
		after = "\n          after: " + detectedAfter
	}
	return `
version: 1
channel_types:
  breach-response:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead] }
      notified:  { properties: { reference: { type: string } }, post: [lead] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      initial: detected
      states:
        detected:
          on:
            contained: { to: contained }
            notified:  { to: notified }` + after + `
        contained:
          on: { assessed: { to: assessed } }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, label: 72 hours since detection }
        assessed:
          on: { notified: { to: notified } }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, label: 72 hours since detection }
        overdue:
          on: { notified: { to: notified } }
        notified: { final: true }
  cases:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      done: { properties: { note: { type: string } }, post: [member] }
    session:
      initial: open-case
      states:
        open-case:
          on: { done: { to: finished } }
          after: { stale: { period: 1h, to: finished } }
        finished: { final: true }
  chain:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown, required: true } }, post: [member] }
    session:
      initial: first
      states:
        first:  { after: { one: { period: 1h, to: second } } }
        second: { after: { two: { period: 1h, since: session, to: third } } }
        third:  { final: true }
  dual:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown, required: true } }, post: [member] }
    session:
      initial: early
      states:
        early: { after: { x-late: { period: 3h, to: done }, a-early: { period: 1h, to: done } } }
        tie:   { after: { beta: { period: 1h, to: done }, alpha: { period: 1h, to: done } } }
        done:  { final: true }
`
}

var (
	timerRealtime = timerRealtimeWith(noContainment)
	breachAddr    = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "breach-response"}
	chainAddr     = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "chain"}
)

func breachKey(id string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id}, Type: "breach-response"}
}

// t0 is the harness clock's start.
var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// timerHarness is a harness with a timer loop over its store.
type timerHarness struct {
	*harness
	tm *Timers
}

func newTimerHarness(t *testing.T) *timerHarness {
	t.Helper()
	h := newHarness(t, withYAML(timerRealtime))
	return &timerHarness{harness: h, tm: h.timers(t)}
}

// timers returns a timer loop over the harness's store, with no jitter.
func (h *harness) timers(t *testing.T) *Timers {
	t.Helper()
	tm, err := NewTimers(TimerDeps{Store: h.store, Config: h.cfg, Marker: h.local, Clock: h.clock.Now})
	require.NoError(t, err)
	tm.jitter = func(d time.Duration) time.Duration { return d }
	return tm
}

func (th *timerHarness) round(t *testing.T) TimerReport {
	t.Helper()
	rep, err := th.tm.RunRound(context.Background())
	require.NoError(t, err)
	return rep
}

// at sets the clock to t0 plus d.
func (th *timerHarness) at(d time.Duration) {
	th.clock.mu.Lock()
	defer th.clock.mu.Unlock()
	th.clock.now = t0.Add(d)
}

func (th *timerHarness) head(t *testing.T, k channels.Key) channels.Channel {
	t.Helper()
	c, err := th.store.Store.Head(context.Background(), k)
	require.NoError(t, err)
	return c
}

// open posts a text to the breachAddr channel of id, which creates it.
func (th *timerHarness) open(t *testing.T, id string) {
	t.Helper()
	addr := breachAddr
	addr.Anchor = id
	th.postTo(t, user("alice"), addr, "text", props("body", "hi"))
}

// notices returns the session notices of k.
func (th *timerHarness) notices(t *testing.T, k channels.Key) []channels.Message {
	t.Helper()
	msgs, err := th.store.Store.Messages(context.Background(), k, channels.Page{})
	require.NoError(t, err)
	var out []channels.Message
	for _, m := range msgs {
		if m.Type == realtimecfg.SessionNoticeType {
			out = append(out, m)
		}
	}
	return out
}

func (th *timerHarness) lifecycle(t *testing.T, op string) {
	t.Helper()
	_, err := th.svc.Lifecycle(as(user("alice")), breachAddr, op)
	require.NoError(t, err)
}

// session returns the compiled session of channel type typ.
func testSession(t *testing.T, typ string) *realtimecfg.Session {
	t.Helper()
	ct, ok := testCompiled(t, timerRealtime).ChannelType(typ)
	require.True(t, ok)
	require.NotNil(t, ct.Session())
	return ct.Session()
}

func TestCarriedRestarts(t *testing.T) {
	t.Parallel()
	breach, chain := testSession(t, "breach-response"), testSession(t, "chain")
	r := t0.Add(100 * time.Hour)
	in := func(state string, anchors map[string]time.Time) channels.Channel {
		return channels.Channel{CreatedAt: t0, Session: channels.Session{
			State: state, Generation: 1, EnteredAt: t0, Restarts: anchors,
		}}
	}
	cases := []struct {
		name string
		s    *realtimecfg.Session
		from channels.Channel
		to   string
		want map[string]time.Time
	}{
		{
			"since session in both states", breach,
			in("contained", map[string]time.Time{"notify-deadline": r}), "assessed",
			map[string]time.Time{"notify-deadline": r},
		},
		{"target lacks the timer", breach, in("contained", map[string]time.Time{"notify-deadline": r}), "overdue", nil},
		{"left state lacks the timer", breach, in("detected", map[string]time.Time{"notify-deadline": r}), "contained", nil},
		{"since state anchor resets", breach, in("detected", map[string]time.Time{"no-containment": r}), "contained", nil},
		{"other name", chain, in("first", map[string]time.Time{"one": r}), "second", nil},
		{"no anchors", breach, in("contained", nil), "assessed", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, carriedRestarts(tc.s, tc.from, tc.to))
		})
	}
}

func TestMaxSessionRestarts(t *testing.T) {
	t.Parallel()
	require.Equal(t, realtimelimits.MaxSessionTimers, channels.MaxSessionRestarts,
		"a state may restart each of its timers")
}

func TestTimerDue(t *testing.T) {
	t.Parallel()
	breachS := testSession(t, "breach-response")
	dual := testSession(t, "dual")
	sess := func(state string, entered time.Duration) channels.Session {
		return channels.Session{State: state, Generation: 1, EnteredAt: t0.Add(entered)}
	}
	cases := []struct {
		name  string
		s     *realtimecfg.Session
		ch    channels.Channel
		timer string
		due   time.Time
	}{
		{"since state counts from entered_at", breachS,
			channels.Channel{CreatedAt: t0, Session: channels.Session{
				State: "detected", Generation: 2, EnteredAt: t0.Add(5 * time.Hour),
			}}, "no-containment", t0.Add(29 * time.Hour)},
		{"never moved counts from created_at", breachS,
			channels.Channel{CreatedAt: t0}, "no-containment", t0.Add(24 * time.Hour)},
		{"since session counts from created_at", breachS,
			channels.Channel{CreatedAt: t0, Session: sess("assessed", 30*time.Hour)}, "notify-deadline",
			t0.Add(72 * time.Hour)},
		{"a later restart anchor wins", breachS,
			channels.Channel{CreatedAt: t0, Session: channels.Session{
				State: "contained", Generation: 1, EnteredAt: t0,
				Restarts: map[string]time.Time{"notify-deadline": t0.Add(100 * time.Hour)},
			}}, "notify-deadline", t0.Add(172 * time.Hour)},
		{"an earlier restart anchor loses", breachS,
			channels.Channel{CreatedAt: t0, Session: channels.Session{
				State: "contained", Generation: 1, EnteredAt: t0.Add(10 * time.Hour),
				Restarts: map[string]time.Time{"notify-deadline": t0.Add(-time.Hour)},
			}}, "notify-deadline", t0.Add(72 * time.Hour)},
		{"the earliest timer wins", dual, channels.Channel{CreatedAt: t0}, "a-early", t0.Add(time.Hour)},
		{"a tie goes to the first name", dual,
			channels.Channel{CreatedAt: t0, Session: sess("tie", 0)}, "alpha", t0.Add(time.Hour)},
		{"a final state has none", breachS,
			channels.Channel{CreatedAt: t0, Session: sess("notified", 0)}, "", time.Time{}},
		{"a state without timers has none", breachS,
			channels.Channel{CreatedAt: t0, Session: sess("overdue", 0)}, "", time.Time{}},
		{"a stranded state has none", breachS,
			channels.Channel{CreatedAt: t0, Session: sess("ghost", 0)}, "", time.Time{}},
		{"normalized to microseconds", breachS,
			channels.Channel{CreatedAt: t0.Add(1500 * time.Nanosecond)}, "no-containment",
			t0.Add(24*time.Hour + time.Microsecond)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tm, due, ok := nextTimer(tc.s, tc.ch)
			require.Equal(t, tc.timer != "", ok)
			require.Equal(t, tc.timer, tm.Name)
			require.True(t, tc.due.Equal(due), "due %v, want %v", due, tc.due)
		})
	}
}

func TestResumeSchedule(t *testing.T) {
	t.Parallel()
	s := testSession(t, "breach-response")
	dual := testSession(t, "dual")
	contained := channels.Channel{CreatedAt: t0, Session: channels.Session{State: "contained", Generation: 1, EnteredAt: t0}}

	t.Run("a due timer restarts at the resume", func(t *testing.T) {
		t.Parallel()
		got := resumeSession(s, contained, t0.Add(100*time.Hour))
		require.Equal(t, t0.Add(172*time.Hour), got.NextDueAt)
		require.Equal(t, map[string]time.Time{"notify-deadline": t0.Add(100 * time.Hour)}, got.Restarts)
	})
	t.Run("a timer not yet due keeps its deadline", func(t *testing.T) {
		t.Parallel()
		got := resumeSession(s, contained, t0.Add(10*time.Hour))
		require.Equal(t, t0.Add(72*time.Hour), got.NextDueAt)
		require.Empty(t, got.Restarts)
	})
	t.Run("two cycles keep a restarted deadline", func(t *testing.T) {
		t.Parallel()
		first := resumeSession(s, contained, t0.Add(100*time.Hour))
		again := contained
		again.Session.Restarts = first.Restarts
		again.Session.NextDueAt = first.NextDueAt
		got := resumeSession(s, again, t0.Add(120*time.Hour))
		require.Equal(t, t0.Add(172*time.Hour), got.NextDueAt)
		require.Equal(t, first.Restarts, got.Restarts)
	})
	t.Run("each timer keeps its own anchor", func(t *testing.T) {
		t.Parallel()
		ch := channels.Channel{CreatedAt: t0}
		got := resumeSession(dual, ch, t0.Add(2*time.Hour))
		require.Equal(t, map[string]time.Time{"a-early": t0.Add(2 * time.Hour)}, got.Restarts)
		require.Equal(t, t0.Add(3*time.Hour), got.NextDueAt, "a-early restarted ties x-late")
		ch.Session.Restarts = got.Restarts
		got = resumeSession(dual, ch, t0.Add(4*time.Hour))
		require.Equal(t, map[string]time.Time{
			"a-early": t0.Add(4 * time.Hour), "x-late": t0.Add(4 * time.Hour),
		}, got.Restarts)
		require.Equal(t, t0.Add(5*time.Hour), got.NextDueAt)
	})
	t.Run("a final state has no schedule", func(t *testing.T) {
		t.Parallel()
		final := channels.Channel{CreatedAt: t0, Session: channels.Session{State: "notified", Generation: 1}}
		require.Equal(t, channels.SessionSchedule{}, resumeSession(s, final, t0))
	})
	t.Run("another type keeps its schedule", func(t *testing.T) {
		t.Parallel()
		cfg := testCompiled(t, timerRealtime)
		ch := channels.Channel{Key: channels.Key{Type: "unknown"}, Session: channels.Session{NextDueAt: t0}}
		require.Equal(t, t0, ResumeSchedule(cfg, t0.Add(time.Hour))(ch).NextDueAt)
		require.Equal(t, t0, ResumeSchedule(nil, t0.Add(time.Hour))(ch).NextDueAt)
		ch.Key.Type, ch.CreatedAt = "breach-response", t0
		require.Equal(t, t0.Add(49*time.Hour), ResumeSchedule(cfg, t0.Add(25*time.Hour))(ch).NextDueAt)
	})
}

func TestTimers_Fires(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	k := breachKey("TKT-1")
	require.Equal(t, t0.Add(24*time.Hour), th.head(t, k).Session.NextDueAt)
	sub := th.local.Subscribe()
	defer sub.Close()

	th.at(24*time.Hour - time.Second)
	rep := th.round(t)
	require.Zero(t, rep.Fired)
	require.Equal(t, t0.Add(24*time.Hour), rep.Next)

	th.at(24 * time.Hour)
	rep = th.round(t)
	require.Equal(t, 1, rep.Fired)
	c := th.head(t, k)
	require.Equal(t, "overdue", c.Session.State)
	require.Equal(t, int64(1), c.Session.Generation)
	require.True(t, c.Session.NextDueAt.IsZero(), "overdue has no timers")

	notices := th.notices(t, k)
	require.Len(t, notices, 1)
	require.Equal(t, "no-containment", notices[0].Props["timer"])
	require.Equal(t, "detected", notices[0].Props["from"])
	require.Equal(t, "overdue", notices[0].Props["to"])
	audit := th.sessionAudit(t)
	require.Len(t, audit, 1)
	require.Equal(t, systemAuthor(), audit[0].By)
	require.Equal(t, "timer:breach-response/detected/no-containment", audit[0].TriggeredBy)
	require.Contains(t, takeKeys(t, sub), k)

	require.Zero(t, th.round(t).Fired, "a fired timer does not fire again")
}

func TestTimers_FromSession(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	alice := user("alice")
	th.postTo(t, alice, breachAddr, "contained", props("note", "x"))
	th.at(30 * time.Hour)
	th.postTo(t, alice, breachAddr, "assessed", props("risk", "high"))
	k := breachKey("TKT-1")
	require.Equal(t, "assessed", th.head(t, k).Session.State)
	require.Equal(t, t0.Add(72*time.Hour), th.head(t, k).Session.NextDueAt)

	th.at(72*time.Hour - time.Second)
	require.Zero(t, th.round(t).Fired)
	th.at(72 * time.Hour)
	require.Equal(t, 1, th.round(t).Fired)
	require.Equal(t, "overdue", th.head(t, k).Session.State)
}

func TestTimers_MessageWins(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	th.at(24 * time.Hour)
	var once sync.Once
	th.store.onSetSession = func(channels.Key) error {
		once.Do(func() { th.postTo(t, user("alice"), breachAddr, "notified", props("reference", "AP-1")) })
		return nil
	}
	rep := th.round(t)
	require.Zero(t, rep.Fired)
	k := breachKey("TKT-1")
	require.Equal(t, "notified", th.head(t, k).Session.State)
	require.Len(t, th.notices(t, k), 1, "only the message moved the session")
}

func TestTimers_LeaderOverlap(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	other := th.timers(t)
	th.open(t, "TKT-1")
	th.at(24 * time.Hour)
	var wg sync.WaitGroup
	reps := make([]TimerReport, 2)
	for i, tm := range []*Timers{th.tm, other} {
		wg.Go(func() {
			rep, err := tm.RunRound(context.Background())
			require.NoError(t, err)
			reps[i] = rep
		})
	}
	wg.Wait()
	require.Equal(t, 1, reps[0].Fired+reps[1].Fired)
	require.Len(t, th.notices(t, breachKey("TKT-1")), 1)
	require.Len(t, th.sessionAudit(t), 1)
}

func TestTimers_ClosedFreezes(t *testing.T) {
	t.Parallel()
	k := breachKey("TKT-1")
	t.Run("reopened after the due time", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.open(t, "TKT-1")
		th.at(10 * time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleClose)
		th.at(30 * time.Hour)
		require.Zero(t, th.round(t).Fired, "a closed channel does not fire")
		th.at(100 * time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleReopen)
		require.Equal(t, t0.Add(124*time.Hour), th.head(t, k).Session.NextDueAt)
		require.Zero(t, th.round(t).Fired)
		th.at(124*time.Hour - time.Second)
		require.Zero(t, th.round(t).Fired)
		th.at(124 * time.Hour)
		require.Equal(t, 1, th.round(t).Fired)
	})
	t.Run("reopened before the due time", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.open(t, "TKT-1")
		th.at(time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleClose)
		th.at(2 * time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleReopen)
		require.Equal(t, t0.Add(24*time.Hour), th.head(t, k).Session.NextDueAt)
		th.at(24 * time.Hour)
		require.Equal(t, 1, th.round(t).Fired)
	})
	t.Run("two cycles keep the restarted deadline", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.postTo(t, user("alice"), breachAddr, "contained", props("note", "x"))
		for _, cycle := range [][2]time.Duration{{10 * time.Hour, 100 * time.Hour}, {110 * time.Hour, 120 * time.Hour}} {
			th.at(cycle[0])
			th.lifecycle(t, realtimecfg.LifecycleClose)
			th.at(cycle[1])
			th.lifecycle(t, realtimecfg.LifecycleReopen)
			require.Equal(t, t0.Add(172*time.Hour), th.head(t, k).Session.NextDueAt)
		}
		th.at(172*time.Hour - time.Second)
		require.Zero(t, th.round(t).Fired)
		th.at(172 * time.Hour)
		require.Equal(t, 1, th.round(t).Fired)
	})
	t.Run("a since session anchor survives a move", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		alice := user("alice")
		th.postTo(t, alice, breachAddr, "contained", props("note", "x"))
		th.at(10 * time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleClose)
		th.at(100 * time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleReopen)
		th.at(110 * time.Hour)
		th.postTo(t, alice, breachAddr, "assessed", props("risk", "high"))
		head := th.head(t, k)
		require.Equal(t, "assessed", head.Session.State)
		require.Equal(t, t0.Add(172*time.Hour), head.Session.NextDueAt, "the restarted deadline carries over")
		require.Zero(t, th.round(t).Fired, "the move does not make the session overdue")
		th.at(172*time.Hour - time.Second)
		require.Zero(t, th.round(t).Fired)
		require.Equal(t, "assessed", th.head(t, k).Session.State)
		th.at(172 * time.Hour)
		require.Equal(t, 1, th.round(t).Fired)
		require.Equal(t, "overdue", th.head(t, k).Session.State)
	})
}

func TestTimers_RestoreFromTombstone(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	ctx := context.Background()
	th.open(t, "TKT-1")
	th.at(10 * time.Hour)
	h, err := th.store.Tombstone(ctx, "TKT-1", "", th.clock.Now(),
		channels.FileAs(channels.Filing{Policy: channels.PolicyDetach, PurgeAfter: t0.Add(1000 * time.Hour), Pending: true}))
	require.NoError(t, err)
	th.at(30 * time.Hour)
	require.Zero(t, th.round(t).Fired, "a tombstoned channel does not fire")
	th.at(40 * time.Hour)
	at := th.clock.Now()
	require.NoError(t, th.store.Untombstone(ctx, h, at, ResumeSchedule(th.cfg.Load().Config, at)))
	require.Zero(t, th.round(t).Fired, "the restore counts as a reopen")
	k := breachKey("TKT-1")
	require.Equal(t, t0.Add(64*time.Hour), th.head(t, k).Session.NextDueAt)
	th.at(64 * time.Hour)
	require.Equal(t, 1, th.round(t).Fired)
}

// TestSweep_RestoreNotesDueWake pins that the retention sweep, putting
// back a pending batch whose entity exists, notes the resumed due time on
// its DueWake, so the timer round runs then rather than at the ceiling.
func TestSweep_RestoreNotesDueWake(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	ctx := context.Background()
	th.open(t, "TKT-1")
	th.at(10 * time.Hour)
	_, err := th.store.Tombstone(ctx, "TKT-1", "", th.clock.Now(),
		channels.FileAs(channels.Filing{Policy: channels.PolicyDetach, PurgeAfter: t0.Add(1000 * time.Hour), Pending: true}))
	require.NoError(t, err)
	anchors := &fakeExistence{faces: map[string][]entity.Face{}}
	anchors.set("TKT-1")
	wake := NewDueWake()
	sw, err := NewSweeper(SweepDeps{
		Store: th.store, Config: th.cfg, Anchors: anchors, Marker: &recordingMarker{}, Clock: th.clock.Now,
		DueWake: wake,
	})
	require.NoError(t, err)
	th.at(10*time.Hour + realtimelimits.OrphanGrace + time.Minute)
	rep, err := sw.RunRetention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Restored)
	due := th.head(t, breachKey("TKT-1")).Session.NextDueAt
	require.False(t, due.IsZero())
	require.Equal(t, due, wake.Take())
}

func TestResumption(t *testing.T) {
	t.Parallel()
	cfg := testCompiled(t, timerRealtime)
	at := t0.Add(100 * time.Hour)
	r := NewResumption(cfg, at)
	r.Note(nil) // a nil wake is accepted
	wake := NewDueWake()
	r.Note(wake)
	require.True(t, wake.Take().IsZero(), "nothing scheduled, nothing noted")
	ch := func(created time.Time) channels.Channel {
		return channels.Channel{
			Key: breachKey("TKT-1"), State: channels.StateOpen, CreatedAt: created,
			Session: channels.Session{State: "contained", Generation: 1, EnteredAt: created},
		}
	}
	restarted := r.Schedule(ch(t0)) // came due before at: restarts from at
	kept := r.Schedule(ch(t0.Add(90 * time.Hour)))
	require.Equal(t, at.Add(72*time.Hour), restarted.NextDueAt)
	require.Equal(t, t0.Add(162*time.Hour), kept.NextDueAt)
	r.Note(wake)
	require.Equal(t, kept.NextDueAt, wake.Take(), "the earliest due time is noted")
}

func TestTimers_Stranded(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	ctx := context.Background()
	th.open(t, "TKT-1")
	k := breachKey("TKT-1")
	_, err := th.store.Store.SetSession(ctx, k, t0, channels.SessionStep{
		Decide: func(channels.Channel) (channels.SessionMove, bool) {
			return channels.SessionMove{From: "detected", To: "ghost"}, true
		},
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(), Timer: "gone",
		TriggeredBy: channels.SessionMarker + "breach-response",
		Due: func(channels.Channel) channels.SessionSchedule {
			return channels.SessionSchedule{NextDueAt: t0.Add(time.Hour)}
		},
	})
	require.NoError(t, err)

	th.at(2 * time.Hour)
	rep := th.round(t)
	require.Zero(t, rep.Fired)
	require.Equal(t, 1, rep.Stranded)
	c := th.head(t, k)
	require.Equal(t, "ghost", c.Session.State)
	require.True(t, c.Session.NextDueAt.IsZero(), "the recompute clears the due time")
	require.True(t, th.tm.warned[[2]string{"breach-response", "ghost"}], "logged once for this config")
	th.at(3 * time.Hour)
	require.Zero(t, th.round(t).Fired)
}

// TestTimerRound_StrandCountsOnce pins that a stranded session the
// recompute and the due listing both see in one round counts once.
func TestTimerRound_StrandCountsOnce(t *testing.T) {
	t.Parallel()
	var r timerRound
	r.strand(breachKey("TKT-1"))
	r.strand(breachKey("TKT-1"))
	r.strand(breachKey("TKT-2"))
	require.Equal(t, 2, r.rep.Stranded)
}

func TestTimers_FinalNoTimers(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	v, err := th.svc.Create(as(user("alice")), "ticket", "TKT-1", "cases", CreateInput{})
	require.NoError(t, err)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "cases", Name: v.Name}
	th.at(time.Hour)
	require.Equal(t, 1, th.round(t).Fired)
	c := th.head(t, k)
	require.Equal(t, "finished", c.Session.State)
	require.True(t, c.Session.NextDueAt.IsZero())
}

func TestTimers_ReloadChangesPeriod(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	th.at(2 * time.Hour)
	require.Zero(t, th.round(t).Fired)

	th.cfg.set(Snapshot{Config: testCompiled(t, timerRealtimeWith(
		strings.Replace(noContainment, "24h", "1h", 1)))})
	rep := th.round(t)
	require.Equal(t, 1, rep.Rescheduled)
	require.Equal(t, 1, rep.Fired, "the shortened timer fires in the same round")
	require.Equal(t, "overdue", th.head(t, breachKey("TKT-1")).Session.State)

	th.open(t, "TKT-2")
	k2 := breachKey("TKT-2")
	require.Equal(t, t0.Add(3*time.Hour), th.head(t, k2).Session.NextDueAt)
	th.cfg.set(Snapshot{Config: testCompiled(t, timerRealtimeWith(""))})
	th.round(t)
	require.True(t, th.head(t, k2).Session.NextDueAt.IsZero(), "a removed timer clears the due time")
}

// staleHint clears the stored due time of k, as an older config would.
func (th *timerHarness) staleHint(t *testing.T, k channels.Key) {
	t.Helper()
	_, err := th.store.Store.SetSession(context.Background(), k, th.clock.Now(), channels.SessionStep{
		Decide:     func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
		TriggeredBy: channels.SessionMarker + k.Type,
		Due:         func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{} },
	})
	require.NoError(t, err)
	require.True(t, th.head(t, k).Session.NextDueAt.IsZero())
}

func TestTimers_RecomputeRepairsStaleHint(t *testing.T) {
	t.Parallel()
	k := breachKey("TKT-1")
	t.Run("periodic", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.open(t, "TKT-1")
		th.round(t)
		th.staleHint(t, k)
		th.at(5 * time.Minute)
		require.Zero(t, th.round(t).Rescheduled, "no recompute before its time")
		th.at(24 * time.Hour)
		rep := th.round(t)
		require.Equal(t, 1, rep.Rescheduled)
		require.Equal(t, 1, rep.Fired)
	})
	t.Run("second pass after a config change", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.open(t, "TKT-1")
		th.round(t)
		th.at(time.Minute)
		th.cfg.set(Snapshot{Config: testCompiled(t, timerRealtime)})
		th.round(t)
		th.staleHint(t, k)
		th.at(time.Minute + 30*time.Second)
		require.Zero(t, th.round(t).Rescheduled)
		th.at(2 * time.Minute)
		require.Equal(t, 1, th.round(t).Rescheduled)
		require.Equal(t, t0.Add(24*time.Hour), th.head(t, k).Session.NextDueAt)
	})
}

func TestTimers_ChainFiresWithoutCeiling(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.postTo(t, user("alice"), chainAddr, "text", props("body", "x"))
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "chain"}
	th.at(3 * time.Hour)
	rep := th.round(t)
	require.Equal(t, 1, rep.Fired)
	require.True(t, rep.More, "the next timer is already due")
	require.Equal(t, "second", th.head(t, k).Session.State)
	th.at(3*time.Hour + realtimelimits.TimerFloor)
	rep = th.round(t)
	require.Equal(t, 1, rep.Fired)
	require.False(t, rep.More)
	require.Equal(t, "third", th.head(t, k).Session.State)
}

func TestTimers_FailingRowIsolated(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) *timerHarness {
		t.Helper()
		th := newTimerHarness(t)
		th.open(t, "TKT-1")
		th.at(time.Minute)
		th.open(t, "TKT-2")
		var mu sync.Mutex
		failing := true
		th.store.onSetSession = func(k channels.Key) error {
			mu.Lock()
			defer mu.Unlock()
			if failing && k == breachKey("TKT-1") {
				return errors.New("disk on fire")
			}
			return nil
		}
		th.at(25 * time.Hour)
		rep, err := th.tm.RunRound(context.Background())
		require.Error(t, err)
		require.Equal(t, 1, rep.Failed)
		require.Equal(t, 1, rep.Fired, "the later channel still fires")
		require.Equal(t, "overdue", th.head(t, breachKey("TKT-2")).Session.State)
		mu.Lock()
		failing = false
		mu.Unlock()
		return th
	}
	t.Run("backoff", func(t *testing.T) {
		t.Parallel()
		th := setup(t)
		require.Zero(t, th.round(t).Fired, "the failing key waits out its backoff")
		th.at(25*time.Hour + realtimelimits.TimerBackoff)
		require.Equal(t, 1, th.round(t).Fired)
	})
	t.Run("reset clears the backoff", func(t *testing.T) {
		t.Parallel()
		th := setup(t)
		th.tm.Reset()
		require.Equal(t, 1, th.round(t).Fired)
	})
}

func TestTimers_Degraded(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	th.at(24 * time.Hour)
	cfg := th.cfg.Load().Config
	th.cfg.set(Snapshot{Config: cfg, Degraded: true})
	require.Equal(t, TimerReport{}, th.round(t))
	require.Zero(t, th.store.dueLists.Load())
	require.Zero(t, th.store.lifecycleLists.Load())
	require.Empty(t, th.sessionAudit(t))
	th.cfg.set(Snapshot{Config: cfg})
	require.Equal(t, 1, th.round(t).Fired)
}

// createCases creates n cases channels on TKT-1 at the clock time.
func (th *timerHarness) createCases(t *testing.T, n int) {
	t.Helper()
	ct, _ := th.cfg.Load().Config.ChannelType("cases")
	for range n {
		_, err := th.store.CreateMany(context.Background(), channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
			Type: "cases", By: channels.Author{User: "alice"}, At: th.clock.Now(), SessionDue: sessionDue(ct.Session()),
		})
		require.NoError(t, err)
	}
}

func TestTimers_RoundCap(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.createCases(t, realtimelimits.MaxTimerWritesPerRound+6)
	th.at(time.Hour)
	rep := th.round(t)
	require.Equal(t, realtimelimits.MaxTimerWritesPerRound, rep.Fired)
	require.True(t, rep.More)
	rep = th.round(t)
	require.Equal(t, 6, rep.Fired)
	require.False(t, rep.More)
}

// TestTimers_NextWake pins Next: the earliest due time after now among the
// sessions listed (those due within one ceiling) and the due times the
// round wrote; zero when there is none.
func TestTimers_NextWake(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	require.Zero(t, th.round(t).Next, "nothing due")
	th.open(t, "TKT-1")
	th.at(30 * time.Second)
	th.open(t, "TKT-2")
	th.at(2 * time.Hour)
	require.Zero(t, th.round(t).Next, "nothing due within a ceiling")
	th.at(24*time.Hour - 10*time.Second)
	require.Equal(t, t0.Add(24*time.Hour), th.round(t).Next)
	th.at(24 * time.Hour)
	rep := th.round(t)
	require.Equal(t, 1, rep.Fired)
	require.Equal(t, t0.Add(24*time.Hour+30*time.Second), rep.Next)

	// A write's due time counts too: a longer timer reschedules TKT-2.
	th.at(24*time.Hour + 5*time.Second)
	th.cfg.set(Snapshot{Config: testCompiled(t, timerRealtimeWith(
		strings.Replace(noContainment, "24h", "1441m", 1)))})
	rep = th.round(t)
	require.Equal(t, 1, rep.Rescheduled)
	require.Equal(t, t0.Add(24*time.Hour+90*time.Second), rep.Next)
}

// untimedType is a channel type whose session has no timers, to append to
// [timerRealtime].
const untimedType = `  untimed:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown, required: true } }, post: [member] }
    session:
      initial: waiting
      states:
        waiting: { on: { text: { to: finished } } }
        finished: { final: true }
`

func TestTimers_Budget(t *testing.T) {
	t.Parallel()
	t.Run("one due listing per round", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.open(t, "TKT-1")
		th.round(t)
		th.at(time.Minute)
		th.round(t)
		require.Equal(t, int64(2), th.store.dueLists.Load())
	})
	t.Run("no due listing without timers", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, withYAML(sessionRealtime("detected", sessionStates)))
		tm := h.timers(t)
		_, err := tm.RunRound(context.Background())
		require.NoError(t, err)
		require.Zero(t, h.store.dueLists.Load())
	})
	t.Run("recompute after a config change", func(t *testing.T) {
		t.Parallel()
		th := newTimerHarness(t)
		th.createCases(t, realtimelimits.MaxChannelsPerAnchor)
		th.round(t)
		th.at(time.Minute)
		require.Zero(t, th.round(t).Rescheduled)
		th.store.lifecycleLists.Store(0)
		th.store.setSessions.Store(0)
		th.round(t)
		require.Zero(t, th.store.lifecycleLists.Load(), "no recompute before its time")
		require.Zero(t, th.store.setSessions.Load(), "nothing to rewrite")

		th.cfg.set(Snapshot{Config: testCompiled(t, strings.Replace(timerRealtime+untimedType,
			"{ stale: { period: 1h,", "{ stale: { period: 2h,", 1))})
		total := 0
		for i := range 4 {
			rep := th.round(t)
			total += rep.Rescheduled
			require.Equal(t, i < 3, rep.More, "round %d", i)
		}
		require.Equal(t, realtimelimits.MaxChannelsPerAnchor, total)
		timerTypes, sessionTypes := int64(0), int64(0)
		for _, ct := range th.cfg.Load().Config.ChannelTypes() {
			if s := ct.Session(); s != nil {
				sessionTypes++
				if s.HasTimers() {
					timerTypes++
				}
			}
		}
		require.Less(t, timerTypes, sessionTypes, "the config has a session type without timers")
		require.Equal(t, timerTypes, th.store.lifecycleLists.Load(), "one listing per type with timers")
	})
}

func TestTimers_NoRewriteWhenUnchanged(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	th.open(t, "TKT-2")
	th.createCases(t, 3)
	th.round(t)
	th.at(20 * time.Minute)
	th.round(t)
	require.Zero(t, th.store.setSessions.Load())
}

func TestTimers_RejectsNil(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	full := TimerDeps{Store: th.store, Config: th.cfg, Marker: th.local, Clock: th.clock.Now}
	for name, d := range map[string]TimerDeps{
		"store":  {Config: full.Config, Marker: full.Marker, Clock: full.Clock},
		"config": {Store: full.Store, Marker: full.Marker, Clock: full.Clock},
		"marker": {Store: full.Store, Config: full.Config, Clock: full.Clock},
		"clock":  {Store: full.Store, Config: full.Config, Marker: full.Marker},
	} {
		_, err := NewTimers(d)
		require.Error(t, err, name)
	}
	require.Equal(t, TimerReport{}, func() TimerReport {
		th.cfg.set(Snapshot{})
		return th.round(t)
	}(), "realtime off")
}

func TestTenthJitter(t *testing.T) {
	t.Parallel()
	for range 50 {
		d := tenthJitter(10 * time.Minute)
		require.GreaterOrEqual(t, d, 9*time.Minute)
		require.Less(t, d, 11*time.Minute)
	}
}

func TestTimers_BackoffCap(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	for i := range realtimelimits.MaxTimerBackoffKeys + 2 {
		th.tm.holdBack(channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "cases", Name: string(rune('a'+i%26)) +
			string(rune('a'+i/26))}, t0)
	}
	require.Len(t, th.tm.backoff, realtimelimits.MaxTimerBackoffKeys)
	require.Len(t, th.tm.order, realtimelimits.MaxTimerBackoffKeys)
}

// TestTimerDelay pins the wait between two timer rounds (TKT-INM3AZ AC
// 14): the time until Next, clamped to [TimerFloor, TimerCeiling]; More
// gives the floor; a zero Next gives the ceiling.
func TestTimerDelay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	cases := []struct {
		name string
		next time.Time
		more bool
		want time.Duration
	}{
		{"until next", now.Add(10 * time.Second), false, 10 * time.Second},
		{"next past is the floor", now.Add(-time.Second), false, realtimelimits.TimerFloor},
		{"next too close is the floor", now.Add(time.Millisecond), false, realtimelimits.TimerFloor},
		{"next far is the ceiling", now.Add(time.Hour), false, realtimelimits.TimerCeiling},
		{"zero next is the ceiling", time.Time{}, false, realtimelimits.TimerCeiling},
		{"more is the floor", now.Add(time.Hour), true, realtimelimits.TimerFloor},
		{"more without next is the floor", time.Time{}, true, realtimelimits.TimerFloor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, TimerDelay(tc.next, tc.more, now))
		})
	}
}

// TestTimerSchedule_Wake pins how a noted due time moves the next
// timer round: an earlier one shortens the sleep, but never to less than
// the floor after the last round; a later one, or none, starts no round.
func TestTimerSchedule_Wake(t *testing.T) {
	t.Parallel()
	last := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	fresh := func() TimerSchedule {
		return TimerSchedule{last: last, target: last.Add(realtimelimits.TimerCeiling)}
	}

	s := fresh()
	require.True(t, s.Wake(last.Add(10*time.Second)), "an earlier due time moves the target")
	require.Equal(t, 5*time.Second, s.Delay(last.Add(5*time.Second)))

	s = fresh()
	require.True(t, s.Wake(last.Add(-time.Hour)), "a past due time moves the target")
	require.Equal(t, last.Add(realtimelimits.TimerFloor), s.target, "never closer than the floor")

	s = fresh()
	require.False(t, s.Wake(last.Add(2*realtimelimits.TimerCeiling)), "a later due time starts no round")
	require.Equal(t, last.Add(realtimelimits.TimerCeiling), s.target)
	require.False(t, s.Wake(time.Time{}), "no due time starts no round")

	s = TimerSchedule{last: last, target: last.Add(realtimelimits.TimerFloor)}
	require.False(t, s.Wake(last), "a target already at the floor stays")

	require.Zero(t, fresh().Delay(last.Add(time.Hour)), "a passed target runs at once")
	require.Equal(t, TimerSchedule{last: last, target: last.Add(realtimelimits.TimerFloor)},
		NextTimerRound(TimerReport{More: true}, last))
}

func TestDueWake(t *testing.T) {
	t.Parallel()
	w := NewDueWake()
	require.True(t, w.Take().IsZero())
	w.Note(time.Time{})
	select {
	case <-w.C():
		t.Fatal("a zero time pings")
	default:
	}
	w.Note(t0.Add(2 * time.Hour))
	w.Note(t0.Add(time.Hour))
	w.Note(t0.Add(3 * time.Hour))
	<-w.C()
	select {
	case <-w.C():
		t.Fatal("pings are coalesced")
	default:
	}
	require.Equal(t, t0.Add(time.Hour), w.Take(), "the earliest is kept")
	require.True(t, w.Take().IsZero(), "Take clears it")
	noteDue(w, nil)
	noteDue(w, &channels.SessionStep{})
	require.True(t, w.Take().IsZero())
}

func TestSessionPost_SetsDue(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	require.Equal(t, t0.Add(24*time.Hour), th.head(t, breachKey("TKT-1")).Session.NextDueAt)
	require.Equal(t, t0.Add(24*time.Hour), th.wake.Take())
	th.at(time.Hour)
	th.postTo(t, user("alice"), breachAddr, "contained", props("note", "x"))
	require.Equal(t, t0.Add(72*time.Hour), th.head(t, breachKey("TKT-1")).Session.NextDueAt)
	require.Equal(t, t0.Add(72*time.Hour), th.wake.Take())
}

func TestSessionCreate_SetsDue(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	v, err := th.svc.Create(as(user("alice")), "ticket", "TKT-1", "cases", CreateInput{})
	require.NoError(t, err)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "cases", Name: v.Name}
	require.Equal(t, t0.Add(time.Hour), th.head(t, k).Session.NextDueAt)
	require.Equal(t, t0.Add(time.Hour), th.wake.Take())
}

func TestSessionReopen_Resumes(t *testing.T) {
	t.Parallel()
	th := newTimerHarness(t)
	th.open(t, "TKT-1")
	th.at(time.Hour)
	th.lifecycle(t, realtimecfg.LifecycleClose)
	th.wake.Take()
	th.at(30 * time.Hour)
	th.lifecycle(t, realtimecfg.LifecycleReopen)
	c := th.head(t, breachKey("TKT-1"))
	require.Equal(t, map[string]time.Time{"no-containment": t0.Add(30 * time.Hour)}, c.Session.Restarts)
	require.Equal(t, t0.Add(54*time.Hour), c.Session.NextDueAt)
	require.Equal(t, t0.Add(54*time.Hour), th.wake.Take())
}
