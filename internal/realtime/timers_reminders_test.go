package realtime

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

var (
	pairAddr     = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "pair"}
	deadlineAddr = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "deadline"}
)

func newRemindHarness(t *testing.T, yaml string) *timerHarness {
	t.Helper()
	h := newHarness(t, withYAML(yaml))
	return &timerHarness{harness: h, tm: h.timers(t)}
}

// reminders returns the reminder messages of k.
func (th *timerHarness) reminders(t *testing.T, k channels.Key) []channels.Message {
	t.Helper()
	msgs, err := th.store.Store.Messages(context.Background(), k, channels.Page{Limit: channels.MaxPageLimit})
	require.NoError(t, err)
	var out []channels.Message
	for _, m := range msgs {
		if isReminder(m) {
			out = append(out, m)
		}
	}
	return out
}

// count is the status-update count stored on k.
func (th *timerHarness) count(t *testing.T, k channels.Key) int64 {
	t.Helper()
	return th.head(t, k).Session.Reminders["status-update"].Count
}

// seedReminders stores recs as k's reminder records, keeping its state.
func (th *timerHarness) seedReminders(t *testing.T, k channels.Key, recs map[string]channels.ReminderRecord) {
	t.Helper()
	_, err := th.store.Store.SetSession(context.Background(), k, th.clock.Now(), channels.SessionStep{
		Decide:     func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(), TriggeredBy: channels.SessionMarker + k.Type,
		Due: func(c channels.Channel) channels.SessionSchedule {
			return channels.SessionSchedule{NextDueAt: c.Session.NextDueAt, Reminders: recs}
		},
	})
	require.NoError(t, err)
}

// view is the session of k as a reader of type typ sees it now.
func (th *timerHarness) view(t *testing.T, k channels.Key) *SessionView {
	t.Helper()
	snap := th.cfg.Load()
	ct, ok := snap.Config.ChannelType(k.Type)
	require.True(t, ok)
	h := th.head(t, k)
	return sessionView(ct, &h, th.clock.Now(), snap.Degraded)
}

func TestTimers_RemindPosts(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := breachKey("TKT-1")
	th.open(t, "TKT-1")
	require.Equal(t, t0.Add(10*time.Minute+realtimelimits.ReminderSlack), th.head(t, k).Session.NextDueAt)
	sub := th.local.Subscribe()
	defer sub.Close()

	th.at(10*time.Minute - time.Second)
	require.Zero(t, th.round(t).Reminded)
	for i, at := range []time.Duration{10 * time.Minute, 40 * time.Minute, 70 * time.Minute} {
		th.at(at)
		rep := th.round(t)
		require.Equal(t, 1, rep.Reminded, "slot %d", i)
		require.False(t, rep.More)
		require.Zero(t, th.round(t).Reminded, "a slot posts once")
		require.Equal(t, int64(i+1), th.count(t, k))
	}
	msgs := th.reminders(t, k)
	require.Len(t, msgs, 3)
	m := msgs[0]
	require.Equal(t, "reminder", m.Type)
	require.Equal(t, "Post a status update", m.Props["body"])
	require.Equal(t, systemAuthor(), m.Author)
	require.Equal(t, "reminder:breach-response/status-update", m.TriggeredBy)
	require.Equal(t, t0.Add(10*time.Minute), m.At)
	v := th.view(t, k)
	require.Equal(t, "detected", v.State)
	require.Zero(t, v.Generation)
	require.Empty(t, th.notices(t, k))
	require.Empty(t, th.sessionAudit(t), "a reminder writes no audit record")
	require.Contains(t, takeKeys(t, sub), k)
}

func TestTimers_RemindAcrossStates(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := breachKey("TKT-1")
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	th.at(25 * time.Minute)
	th.postTo(t, user("alice"), breachAddr, "contained", props("note", "x"))
	head := th.head(t, k)
	require.Equal(t, "contained", head.Session.State)
	require.Equal(t, t0.Add(40*time.Minute+realtimelimits.ReminderSlack), head.Session.NextDueAt)
	th.at(40*time.Minute - time.Second)
	require.Zero(t, th.round(t).Reminded)
	th.at(40 * time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	require.Equal(t, int64(2), th.count(t, k))
}

func TestTimers_RemindStopsOutsideIn(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := breachKey("TKT-1")
	alice := user("alice")
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	th.at(15 * time.Minute)
	th.postTo(t, alice, breachAddr, "notified", props("reference", "AP-1"))
	head := th.head(t, k)
	require.Empty(t, head.Session.Reminders)
	require.True(t, head.Session.NextDueAt.IsZero())
	ct := testChannelType(t, remindRealtime)
	s := ct.Session()
	counters, ok := reminderCounters(s, currentSession(s, head), head).(predicate.Record)
	require.True(t, ok)
	n, ok := counters.Get("status-update")
	require.True(t, ok)
	require.Equal(t, predicate.NewInt(0), n)
	require.Empty(t, th.view(t, k).Reminders)
	th.at(2 * time.Hour)
	require.Zero(t, th.round(t).Reminded)

	th.postTo(t, alice, breachAddr, "reopened", props("note", "back"))
	require.Zero(t, th.count(t, k))
	th.at(2*time.Hour + 10*time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	require.Equal(t, int64(1), th.count(t, k), "a new span counts from 0")
}

func TestTimers_RemindMax(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtimeWith(remindOpts{every: "30m", first: "10m", max: "2"}))
	k := breachKey("TKT-1")
	th.open(t, "TKT-1")
	for _, at := range []time.Duration{10 * time.Minute, 40 * time.Minute} {
		th.at(at)
		require.Equal(t, 1, th.round(t).Reminded)
	}
	require.True(t, th.head(t, k).Session.NextDueAt.IsZero())
	th.at(5 * time.Hour)
	require.Zero(t, th.round(t).Reminded)
	v := th.view(t, k)
	require.Len(t, v.Reminders, 1)
	require.Equal(t, int64(2), v.Reminders[0].Count)
	require.Equal(t, int64(2), v.Reminders[0].Max)
	require.Nil(t, v.Reminders[0].DueAt)
}

func TestTimers_RemindSkipsMissed(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := breachKey("TKT-1")
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	th.at(2*time.Hour + 20*time.Minute)
	rep := th.round(t)
	require.Equal(t, 1, rep.Reminded)
	require.False(t, rep.More)
	require.Zero(t, th.round(t).Reminded)
	require.Len(t, th.reminders(t, k), 2)
	require.Equal(t, t0.Add(2*time.Hour+40*time.Minute+realtimelimits.ReminderSlack), th.head(t, k).Session.NextDueAt,
		"the next slot stays on the grid")
}

func TestTimers_RemindTwoDue(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "pair"}
	th.postTo(t, user("alice"), pairAddr, "ping", props("body", "hi"))
	th.at(time.Hour)
	rep := th.round(t)
	require.Equal(t, 1, rep.Reminded)
	require.True(t, rep.More, "the second reminder is due work")
	require.Equal(t, realtimelimits.TimerFloor, NextTimerRound(rep, th.clock.Now()).Delay(th.clock.Now()))
	th.at(time.Hour + realtimelimits.TimerFloor)
	rep = th.round(t)
	require.Equal(t, 1, rep.Reminded)
	require.False(t, rep.More)
	msgs := th.reminders(t, k)
	require.Len(t, msgs, 2)
	require.Equal(t, "reminder:pair/alpha", msgs[0].TriggeredBy)
	require.Equal(t, "reminder:pair/beta", msgs[1].TriggeredBy)
}

func TestTimers_RemindClosedFreezes(t *testing.T) {
	t.Parallel()
	k := breachKey("TKT-1")
	setup := func(t *testing.T) *timerHarness {
		t.Helper()
		th := newRemindHarness(t, remindRealtime)
		th.open(t, "TKT-1")
		th.at(10 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		th.at(20 * time.Minute)
		th.lifecycle(t, realtimecfg.LifecycleClose)
		return th
	}
	t.Run("reopened after the slot", func(t *testing.T) {
		t.Parallel()
		th := setup(t)
		th.at(45 * time.Minute)
		require.Zero(t, th.round(t).Reminded, "a closed channel gets no reminder")
		th.at(2 * time.Hour)
		th.lifecycle(t, realtimecfg.LifecycleReopen)
		require.Equal(t, t0.Add(2*time.Hour+10*time.Minute+realtimelimits.ReminderSlack), th.head(t, k).Session.NextDueAt)
		th.at(2*time.Hour + 10*time.Minute - time.Second)
		require.Zero(t, th.round(t).Reminded)
		th.at(2*time.Hour + 10*time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		require.Equal(t, int64(2), th.count(t, k), "the counter is kept")
	})
	t.Run("reopened before the slot", func(t *testing.T) {
		t.Parallel()
		th := setup(t)
		th.at(30 * time.Minute)
		th.lifecycle(t, realtimecfg.LifecycleReopen)
		th.at(40 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		require.Equal(t, int64(2), th.count(t, k))
	})
}

func TestTimers_RemindRestoreFromTombstone(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	ctx := context.Background()
	k := breachKey("TKT-1")
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	th.at(20 * time.Minute)
	h, err := th.store.Tombstone(ctx, "TKT-1", "", th.clock.Now(),
		channels.FileAs(channels.Filing{Policy: channels.PolicyDetach, PurgeAfter: t0.Add(1000 * time.Hour), Pending: true}))
	require.NoError(t, err)
	th.at(time.Hour)
	require.Zero(t, th.round(t).Reminded)
	th.at(2 * time.Hour)
	at := th.clock.Now()
	require.NoError(t, th.store.Untombstone(ctx, h, at, ResumeSchedule(th.cfg.Load().Config, at)))
	require.Zero(t, th.round(t).Reminded, "the restore counts as a reopen")
	th.at(2*time.Hour + 10*time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	require.Equal(t, int64(2), th.count(t, k))
}

func TestTimers_RemindAndTimerSameRound(t *testing.T) {
	t.Parallel()
	k := breachKey("TKT-1")
	for _, tc := range []struct {
		to   string
		post int
	}{{to: "overdue", post: 1}, {to: "notified", post: 0}} {
		t.Run(tc.to, func(t *testing.T) {
			t.Parallel()
			o := remindDefaults
			o.after = "{ t: { period: 30m, to: " + tc.to + " } }"
			th := newRemindHarness(t, remindRealtimeWith(o))
			th.postTo(t, user("alice"), breachAddr, "contained", props("note", "x"))
			th.at(30 * time.Minute)
			rep := th.round(t)
			require.Equal(t, 1, rep.Fired)
			require.Zero(t, rep.Reminded, "the timer fires first")
			require.Equal(t, tc.post == 1, rep.More)
			require.Equal(t, tc.to, th.head(t, k).Session.State)
			rep = th.round(t)
			require.Equal(t, tc.post, rep.Reminded)
			require.Len(t, th.reminders(t, k), tc.post)
		})
	}
}

func TestTimers_RemindLeaderOverlap(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	other := th.timers(t)
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
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
	require.Equal(t, 1, reps[0].Reminded+reps[1].Reminded)
	require.Len(t, th.reminders(t, breachKey("TKT-1")), 1)
}

func TestTimers_RemindMessageRace(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := breachKey("TKT-1")
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	var once sync.Once
	th.store.onSetSession = func(channels.Key) error {
		once.Do(func() { th.postTo(t, user("alice"), breachAddr, "notified", props("reference", "AP-1")) })
		return nil
	}
	rep := th.round(t)
	require.Zero(t, rep.Reminded)
	require.Equal(t, "notified", th.head(t, k).Session.State)
	require.Empty(t, th.reminders(t, k))
}

func TestTimers_RemindReload(t *testing.T) {
	t.Parallel()
	k := breachKey("TKT-1")
	slack := realtimelimits.ReminderSlack
	t.Run("a shorter every reschedules", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, remindRealtime)
		th.open(t, "TKT-1")
		th.at(10 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		th.at(11 * time.Minute)
		th.cfg.set(Snapshot{Config: testCompiled(t, remindRealtimeWith(remindOpts{every: "5m", first: "10m"}))})
		rep := th.round(t)
		require.Equal(t, 1, rep.Rescheduled)
		require.Equal(t, t0.Add(15*time.Minute+slack), th.head(t, k).Session.NextDueAt)
	})
	t.Run("a longer every posts within one new every", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, remindRealtimeWith(remindOpts{every: "1m", first: "1m"}))
		th.open(t, "TKT-1")
		for i := 1; i <= 20; i++ {
			th.at(time.Duration(i) * time.Minute)
			require.Equal(t, 1, th.round(t).Reminded, "minute %d", i)
		}
		th.cfg.set(Snapshot{Config: testCompiled(t, remindRealtimeWith(remindOpts{every: "30m", first: "1m"}))})
		th.round(t)
		require.Equal(t, t0.Add(31*time.Minute+slack), th.head(t, k).Session.NextDueAt)
		th.at(31 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
	})
	t.Run("a changed first keeps at most one every", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, remindRealtime)
		th.open(t, "TKT-1")
		th.at(10 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		th.cfg.set(Snapshot{Config: testCompiled(t, remindRealtimeWith(remindOpts{every: "30m", first: "25m"}))})
		th.round(t)
		require.Equal(t, t0.Add(25*time.Minute+slack), th.head(t, k).Session.NextDueAt)
	})
	moved := strings.NewReplacer(
		"status-update:", "renamed:",
		"session.reminders['status-update']", "session.reminders['renamed']",
		"in: [detected, contained, assessed, overdue]", "in: [assessed]",
	).Replace(remindRealtime)
	t.Run("a removed reminder clears its due time and record", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, remindRealtime)
		th.open(t, "TKT-1")
		th.at(10 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		th.cfg.set(Snapshot{Config: testCompiled(t, moved)})
		require.Equal(t, 1, th.round(t).Rescheduled)
		head := th.head(t, k)
		require.True(t, head.Session.NextDueAt.IsZero())
		require.Empty(t, head.Session.Reminders)
	})
	t.Run("an added reminder posts once, then follows the grid", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, moved)
		th.open(t, "TKT-1")
		th.at(2 * time.Hour)
		require.Zero(t, th.round(t).Reminded)
		th.cfg.set(Snapshot{Config: testCompiled(t, remindRealtime)})
		require.Equal(t, 1, th.round(t).Reminded)
		require.Zero(t, th.round(t).Reminded)
		require.Equal(t, t0.Add(2*time.Hour+10*time.Minute+slack), th.head(t, k).Session.NextDueAt)
	})
}

func TestTimers_RemindDegraded(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	cfg := th.cfg.Load().Config
	th.cfg.set(Snapshot{Config: cfg, Degraded: true})
	require.Equal(t, TimerReport{}, th.round(t))
	require.Empty(t, th.reminders(t, breachKey("TKT-1")))
	th.cfg.set(Snapshot{Config: cfg})
	require.Equal(t, 1, th.round(t).Reminded)
}

// breakProps makes th's props check of reminder name fail while *broken is
// true, as when the metamodel lost a value the reminder posts.
func breakProps(th *timerHarness, name string, mu *sync.Mutex, broken *bool) {
	th.tm.props = func(
		ctx context.Context, cfg *realtimecfg.Compiled, ct realtimecfg.ChannelType, r realtimecfg.SessionReminder,
		now time.Time,
	) (map[string]any, error) {
		mu.Lock()
		defer mu.Unlock()
		if *broken && r.Name == name {
			return nil, errors.New("the metamodel lost the value the reminder posts")
		}
		return reminderProps(ctx, cfg, ct, r, now)
	}
}

func TestTimers_RemindPostFails(t *testing.T) {
	t.Parallel()
	o := remindDefaults
	o.after = "{ late: { period: 30m, to: overdue } }"
	th := newRemindHarness(t, remindRealtimeWith(o))
	k := breachKey("TKT-1")
	th.postTo(t, user("alice"), breachAddr, "contained", props("note", "x"))
	var mu sync.Mutex
	broken := true
	breakProps(th, "status-update", &mu, &broken)

	th.at(10 * time.Minute)
	rep, err := th.tm.RunRound(context.Background())
	require.Error(t, err, "the paused reminder is logged")
	require.Equal(t, 1, rep.Failed)
	require.False(t, rep.More)
	require.Empty(t, th.reminders(t, k))
	require.Equal(t, t0.Add(30*time.Minute), th.head(t, k).Session.NextDueAt,
		"the due time skips the paused reminder")
	require.Empty(t, th.tm.backoff, "a config failure puts no channel in backoff")

	th.at(11 * time.Minute)
	rep, err = th.tm.RunRound(context.Background())
	require.NoError(t, err, "logged once per configuration")
	require.Zero(t, rep.Failed)

	th.at(30 * time.Minute)
	rep = th.round(t)
	require.Equal(t, 1, rep.Fired, "the deadline timer fires on time")
	require.Equal(t, "overdue", th.head(t, k).Session.State)
	require.Empty(t, th.reminders(t, k))

	mu.Lock()
	broken = false
	mu.Unlock()
	require.Zero(t, th.round(t).Reminded, "paused until the next configuration")
	th.tm.Reset()
	require.Equal(t, 1, th.round(t).Reminded)
}

func TestTimers_RemindPostFailsManyChannels(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	th.createTasks(t, 200, 0)
	th.postTo(t, user("alice"), deadlineAddr, "text", props("body", "hi"))
	dk := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "deadline"}
	var mu sync.Mutex
	broken := true
	breakProps(th, "poke", &mu, &broken)

	// 200 channels whose only reminder is broken sort ahead of a due
	// deadline. Each is rescheduled once, past the paused reminder; none is
	// held back, so the deadline fires within ceil(201/64) rounds and the
	// loop then rests.
	fired := -1
	for i := range 4 {
		th.at(2*time.Minute + time.Duration(i)*realtimelimits.TimerFloor)
		rep, err := th.tm.RunRound(context.Background())
		require.Equal(t, i == 0, err != nil, "round %d logs the pause once", i+1)
		if rep.Fired == 1 {
			fired = i
		}
	}
	require.GreaterOrEqual(t, fired, 0, "the deadline fired")
	require.Equal(t, "late", th.head(t, dk).Session.State)
	require.Empty(t, th.tm.backoff)
	th.at(2*time.Minute + 4*realtimelimits.TimerFloor)
	rep := th.round(t)
	require.False(t, rep.More, "no spinning on the broken channels")
	require.Zero(t, rep.Rescheduled+rep.Reminded+rep.Failed)
}

// createTasks creates n tasks channels on TKT-1, the i-th at the clock
// time plus i*step.
func (th *timerHarness) createTasks(t *testing.T, n int, step time.Duration) []channels.Key {
	t.Helper()
	ct, _ := th.cfg.Load().Config.ChannelType("tasks")
	start := th.clock.Now()
	keys := make([]channels.Key, 0, n)
	for i := range n {
		c, err := th.store.CreateMany(context.Background(), channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
			Type: "tasks", By: channels.Author{User: "alice"}, At: start.Add(time.Duration(i) * step),
			SessionDue: sessionDue(ct.Session()),
		})
		require.NoError(t, err)
		keys = append(keys, c.Key)
	}
	return keys
}

func TestTimers_ReminderBacklogBound(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	th.createTasks(t, 200, 0)
	th.postTo(t, user("alice"), deadlineAddr, "text", props("body", "hi"))
	dk := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "deadline"}
	require.Equal(t, t0.Add(2*time.Minute), th.head(t, dk).Session.NextDueAt)
	th.at(2 * time.Minute)
	for i, want := range []int{64, 64, 64, 8} {
		th.at(2*time.Minute + time.Duration(i)*realtimelimits.TimerFloor)
		rep := th.round(t)
		require.Equal(t, want, rep.Reminded, "round %d", i+1)
		require.Equal(t, i == 3, rep.Fired == 1, "round %d", i+1)
		require.Equal(t, i < 3, rep.More, "round %d", i+1)
	}
	require.Equal(t, "late", th.head(t, dk).Session.State)
}

func TestTimers_ReminderSlack(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	keys := th.createTasks(t, 1, 0)
	th.postTo(t, user("alice"), deadlineAddr, "text", props("body", "hi"))
	dk := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "deadline"}
	require.Equal(t, t0.Add(time.Minute+realtimelimits.ReminderSlack), th.head(t, keys[0]).Session.NextDueAt)
	require.Equal(t, t0.Add(2*time.Minute), th.head(t, dk).Session.NextDueAt, "a timer's hint is exact")
	th.at(40 * time.Second)
	require.Equal(t, t0.Add(time.Minute+realtimelimits.ReminderSlack), th.round(t).Next)
	th.at(time.Minute + 10*time.Second)
	require.Equal(t, 1, th.round(t).Reminded, "a round before the hint posts a passed slot")
}

func TestTimers_ReminderBudget(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	const n = 200
	step := 300 * time.Millisecond
	keys := th.createTasks(t, n, step)
	end := t0.Add(10 * time.Minute)
	rounds := map[int]int{}
	total := 0
	for now := t0; now.Before(end); {
		th.clock.mu.Lock()
		th.clock.now = now
		th.clock.mu.Unlock()
		rep := th.round(t)
		require.Zero(t, rep.Failed)
		rounds[int(now.Sub(t0)/time.Minute)]++
		total++
		now = now.Add(NextTimerRound(rep, now).Delay(now))
	}
	require.Equal(t, int64(total), th.store.dueLists.Load(), "one due listing per round")
	for minute, r := range rounds {
		require.LessOrEqual(t, r, 6, "minute %d", minute)
	}
	for i, k := range keys {
		created := t0.Add(time.Duration(i) * step)
		msgs := th.reminders(t, k)
		// Every slot whose hint (slot plus slack) passed before the end
		// has its reminder.
		served := int((end.Sub(created) - realtimelimits.ReminderSlack - realtimelimits.TimerFloor) / time.Minute)
		require.GreaterOrEqual(t, len(msgs), served, "channel %d", i)
		for j, m := range msgs {
			slot := created.Add(time.Duration(j+1) * time.Minute)
			require.False(t, m.At.Before(channels.NormTime(slot)), "channel %d reminder %d before its slot", i, j)
			require.True(t, m.At.Before(slot.Add(time.Minute)), "channel %d reminder %d skipped a slot", i, j)
		}
	}
}

func TestSessionPost_ReminderGuard(t *testing.T) {
	t.Parallel()
	k := breachKey("TKT-1")
	alice := user("alice")
	t.Run("moves after the third reminder", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, remindRealtime)
		th.postTo(t, alice, breachAddr, "contained", props("note", "x"))
		for _, at := range []time.Duration{10 * time.Minute, 40 * time.Minute} {
			th.at(at)
			require.Equal(t, 1, th.round(t).Reminded)
		}
		res := th.postTo(t, alice, breachAddr, "escalate-stalled", props("note", "slow"))
		require.Equal(t, "contained", res.Session.State, "two reminders do not meet the guard")
		th.at(70 * time.Minute)
		require.Equal(t, 1, th.round(t).Reminded)
		res = th.postTo(t, alice, breachAddr, "escalate-stalled", props("note", "slow"))
		require.Equal(t, "overdue", res.Session.State)
		require.Equal(t, "overdue", th.head(t, k).Session.State)
	})
	t.Run("reads the committed counter", func(t *testing.T) {
		t.Parallel()
		th := newRemindHarness(t, remindRealtime)
		th.postTo(t, alice, breachAddr, "contained", props("note", "x"))
		for _, at := range []time.Duration{10 * time.Minute, 40 * time.Minute} {
			th.at(at)
			require.Equal(t, 1, th.round(t).Reminded)
		}
		th.at(70 * time.Minute)
		th.local.recMu.Lock()
		delete(th.local.recorded, "breach-response")
		th.local.recMu.Unlock()
		var once sync.Once
		th.store.onRecord = func() {
			once.Do(func() { require.Equal(t, 1, th.round(t).Reminded) })
		}
		res := th.postTo(t, alice, breachAddr, "escalate-stalled", props("note", "slow"))
		require.Equal(t, int64(3), th.count(t, k))
		require.Equal(t, "overdue", res.Session.State, "the third reminder landed after the head read")
	})
	t.Run("a reminder outside in reads 0", func(t *testing.T) {
		t.Parallel()
		narrowed := strings.Replace(remindRealtime,
			"in: [detected, contained, assessed, overdue]", "in: [detected, assessed]", 1)
		th := newRemindHarness(t, narrowed)
		th.postTo(t, alice, breachAddr, "contained", props("note", "x"))
		th.seedReminders(t, k, map[string]channels.ReminderRecord{"status-update": {Since: t0, Count: 5}})
		res := th.postTo(t, alice, breachAddr, "escalate-stalled", props("note", "slow"))
		require.Equal(t, "contained", res.Session.State)
	})
}

func TestReminderMessageProtected(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	k := breachKey("TKT-1")
	alice, mo := user("alice"), user("mo", "chat.moderate")
	th.open(t, "TKT-1")
	th.at(10 * time.Minute)
	require.Equal(t, 1, th.round(t).Reminded)
	seq := th.reminders(t, k)[0].Seq

	_, err := th.svc.Edit(as(alice), breachAddr, seq, props("body", "changed"))
	requireDenial(t, err, http.StatusConflict, TypeReminderMessage)
	_, err = th.svc.Delete(as(alice), breachAddr, seq, "")
	requireDenial(t, err, http.StatusConflict, TypeReminderMessage)
	require.NotContains(t, th.offeredOn(t, alice, breachAddr, seq), OpEdit)
	require.NotContains(t, th.offeredOn(t, alice, breachAddr, seq), OpDelete)
	require.Contains(t, th.offeredOn(t, mo, breachAddr, seq), OpRemove)

	got, err := th.svc.Delete(as(mo), breachAddr, seq, "too noisy")
	require.NoError(t, err)
	require.NotNil(t, got.Removed)
	recs, err := th.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	removals := 0
	for _, r := range recs {
		if r.Op == channels.AuditRemove {
			removals++
		}
	}
	require.Equal(t, 1, removals)
	require.Equal(t, int64(1), th.count(t, k), "a removal does not lower the counter")

	own := th.postTo(t, alice, breachAddr, "reminder", props("body", "mine"))
	_, err = th.svc.Edit(as(alice), breachAddr, own.Message.Seq, props("body", "mine, edited"))
	require.NoError(t, err, "a person's post of the type is an ordinary message")
	require.Equal(t, int64(1), th.count(t, k))
}

func TestSessionPost_NotesReminderWake(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	th.open(t, "TKT-1")
	require.Equal(t, t0.Add(10*time.Minute+realtimelimits.ReminderSlack), th.wake.Take())
}

func TestSessionCreate_SetsReminderDue(t *testing.T) {
	t.Parallel()
	th := newRemindHarness(t, remindRealtime)
	v, err := th.svc.Create(as(user("alice")), "ticket", "TKT-1", "tasks", CreateInput{})
	require.NoError(t, err)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "tasks", Name: v.Name}
	want := t0.Add(time.Minute + realtimelimits.ReminderSlack)
	require.Equal(t, want, th.head(t, k).Session.NextDueAt)
	require.Equal(t, want, th.wake.Take())
}
