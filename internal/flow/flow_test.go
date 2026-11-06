package flow_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func period(t *testing.T, s string) flow.Period {
	t.Helper()
	p, err := flow.ParsePeriod(s)
	require.NoError(t, err)
	return p
}

// review is a definition with three states: open (two timers and a
// reminder), waiting (a since: session timer and the same reminder) and
// done (final).
func review(t *testing.T) *flow.Definition {
	t.Helper()
	nudge := flow.NewReminder("nudge", []string{"open", "waiting"})
	nudge.Every, nudge.First, nudge.Max = time.Hour, 30*time.Minute, 3
	return flow.NewDefinition("open",
		[]flow.State{
			flow.NewState("open", false, []flow.Timer{
				{Name: "escalate", To: "waiting", Period: period(t, "2h"), Since: flow.SinceState},
				{Name: "expire", To: "done", Period: period(t, "1d"), Since: flow.SinceSession},
			}),
			flow.NewState("waiting", false, []flow.Timer{
				{Name: "expire", To: "done", Period: period(t, "1d"), Since: flow.SinceSession},
			}),
			flow.NewState("done", true, nil),
		},
		[]flow.Reminder{nudge})
}

func TestRead(t *testing.T) {
	t.Parallel()
	d := review(t)
	rec := map[string]flow.ReminderRecord{"nudge": {Since: t0, Count: 1}}
	restarts := map[string]time.Time{"expire": t0.Add(time.Hour)}

	t.Run("unset reads as initial with stored schedule", func(t *testing.T) {
		t.Parallel()
		cur := flow.Read(d, flow.Instance{Reminders: rec, Restarts: restarts}, t0)
		require.Equal(t, "open", cur.State)
		require.Equal(t, t0, cur.EnteredAt)
		require.Equal(t, t0, cur.CreatedAt)
		require.Equal(t, rec, cur.Reminders, "a reminder post before the first move is kept")
		require.Equal(t, restarts, cur.Restarts, "a resume before the first move is kept")
		require.False(t, cur.Stranded)
	})
	t.Run("declared", func(t *testing.T) {
		t.Parallel()
		inst := flow.Instance{State: "waiting", Generation: 2, EnteredAt: t0.Add(time.Hour)}
		cur := flow.Read(d, inst, t0)
		require.Equal(t, inst, cur.Instance)
		require.False(t, cur.Stranded)
	})
	t.Run("stranded", func(t *testing.T) {
		t.Parallel()
		cur := flow.Read(d, flow.Instance{State: "ghost", Generation: 1}, t0)
		require.True(t, cur.Stranded)
		_, _, ok := flow.NextTimer(d, cur)
		require.False(t, ok, "a stranded state has no timer")
		_, ok = flow.NextSlot(mustReminder(t, d), cur)
		require.False(t, ok, "a stranded state has no slot")
		require.Equal(t, flow.Schedule{}, flow.Resume(d, cur, t0))
	})
}

func mustReminder(t *testing.T, d *flow.Definition) flow.Reminder {
	t.Helper()
	r, ok := d.Reminder("nudge")
	require.True(t, ok)
	return r
}

func TestTimerDue(t *testing.T) {
	t.Parallel()
	d := review(t)
	moved := flow.Instance{State: "open", Generation: 1, EnteredAt: t0.Add(3 * time.Hour)}
	cur := flow.Read(d, moved, t0)
	escalate := flow.Timer{Name: "escalate", Period: period(t, "2h"), Since: flow.SinceState}
	expire := flow.Timer{Name: "expire", Period: period(t, "1d"), Since: flow.SinceSession}

	require.Equal(t, t0.Add(5*time.Hour), flow.TimerDue(escalate, cur), "since: state counts from the entry")
	require.Equal(t, t0.Add(24*time.Hour), flow.TimerDue(expire, cur), "since: session counts from creation")

	moved.Restarts = map[string]time.Time{"escalate": t0.Add(4 * time.Hour), "expire": t0.Add(-time.Hour)}
	cur = flow.Read(d, moved, t0)
	require.Equal(t, t0.Add(6*time.Hour), flow.TimerDue(escalate, cur), "a later restart anchor wins")
	require.Equal(t, t0.Add(24*time.Hour), flow.TimerDue(expire, cur), "an earlier one does not")

	tm, due, ok := flow.NextTimer(d, cur)
	require.True(t, ok)
	require.Equal(t, "escalate", tm.Name)
	require.Equal(t, t0.Add(6*time.Hour), due)

	_, _, ok = flow.NextTimer(d, flow.Read(d, flow.Instance{State: "done", Generation: 2}, t0))
	require.False(t, ok, "a final state has no timer")
}

func TestNextSlot(t *testing.T) {
	t.Parallel()
	d := review(t)
	r := mustReminder(t, d)
	at := func(recs map[string]flow.ReminderRecord) flow.Current {
		return flow.Read(d, flow.Instance{State: "open", Generation: 1, EnteredAt: t0, Reminders: recs}, t0)
	}
	cases := []struct {
		name string
		recs map[string]flow.ReminderRecord
		want time.Time
		ok   bool
	}{
		{name: "no record: first after entry", want: t0.Add(30 * time.Minute), ok: true},
		{
			name: "after the last post",
			recs: map[string]flow.ReminderRecord{"nudge": {Since: t0, Last: t0.Add(95 * time.Minute), Count: 2}},
			want: t0.Add(150 * time.Minute), ok: true,
		},
		{
			name: "last post before first, after a reload",
			recs: map[string]flow.ReminderRecord{"nudge": {Since: t0, Last: t0.Add(10 * time.Minute), Count: 1}},
			want: t0.Add(30 * time.Minute), ok: true,
		},
		{
			name: "max reached",
			recs: map[string]flow.ReminderRecord{"nudge": {Since: t0, Last: t0.Add(3 * time.Hour), Count: 3}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := flow.NextSlot(r, at(tc.recs))
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}

	_, ok := flow.NextSlot(r, flow.Read(d, flow.Instance{State: "done", Generation: 3}, t0))
	require.False(t, ok, "not active in done")

	cur := at(nil)
	_, slot, ok := flow.NextReminder(d, cur)
	require.True(t, ok)
	require.Equal(t, t0.Add(30*time.Minute), slot)
	_, _, ok = flow.NextReminder(d.Pausing([]string{"nudge"}), cur)
	require.False(t, ok, "a paused reminder has no slot")
	require.False(t, d.Paused("nudge"), "Pausing leaves d alone")

	require.Equal(t, t0.Add(30*time.Minute+realtimelimits.ReminderSlack), flow.NextDue(d, cur),
		"the reminder slot plus slack comes before the timers")
	require.Equal(t, map[string]int64{"nudge": 0}, flow.ReminderCounts(d, cur))
	require.Equal(t, map[string]int64{"nudge": 2},
		flow.ReminderCounts(d, at(map[string]flow.ReminderRecord{"nudge": {Since: t0, Count: 2}})))
}

func TestStepSchedule(t *testing.T) {
	t.Parallel()
	d := review(t)
	from := flow.Read(d, flow.Instance{
		State: "open", Generation: 1, EnteredAt: t0,
		Restarts:  map[string]time.Time{"expire": t0.Add(time.Hour), "escalate": t0.Add(time.Hour)},
		Reminders: map[string]flow.ReminderRecord{"nudge": {Since: t0, Count: 1}, "gone": {Since: t0}},
	}, t0)

	t.Run("no move keeps pruned records", func(t *testing.T) {
		t.Parallel()
		sch := flow.StepSchedule(d, from, from)
		require.Nil(t, sch.Restarts)
		require.Equal(t, map[string]flow.ReminderRecord{"nudge": {Since: t0, Count: 1}}, sch.Reminders)
		require.Equal(t, flow.NextDue(d, from), sch.NextDueAt)
	})
	t.Run("move carries since: session anchors and shared reminders", func(t *testing.T) {
		t.Parallel()
		next := flow.Read(d, from.Enter(flow.Move{From: "open", To: "waiting"}, t0.Add(2*time.Hour), 0), t0)
		sch := flow.StepSchedule(d, from, next)
		require.Equal(t, map[string]time.Time{"expire": t0.Add(time.Hour)}, sch.Restarts)
		require.Equal(t, map[string]flow.ReminderRecord{"nudge": {Since: t0, Count: 1}}, sch.Reminders)
	})
	t.Run("move to final keeps nothing", func(t *testing.T) {
		t.Parallel()
		next := flow.Read(d, from.Enter(flow.Move{From: "open", To: "done"}, t0.Add(2*time.Hour), 0), t0)
		sch := flow.StepSchedule(d, from, next)
		require.Nil(t, sch.Restarts)
		require.Empty(t, sch.Reminders)
		require.NotNil(t, sch.Reminders, "an empty map replaces the stored records")
		require.True(t, sch.NextDueAt.IsZero())
	})
}

func TestResume(t *testing.T) {
	t.Parallel()
	d := review(t)
	cur := flow.Read(d, flow.Instance{State: "open", Generation: 1, EnteredAt: t0}, t0)
	at := t0.Add(3 * time.Hour)
	sch := flow.Resume(d, cur, at)
	require.Equal(t, map[string]time.Time{"escalate": at}, sch.Restarts, "only the timer that came due restarts")
	require.Equal(t, map[string]flow.ReminderRecord{"nudge": {Since: at}}, sch.Reminders)
	require.Equal(t, at.Add(30*time.Minute+realtimelimits.ReminderSlack), sch.NextDueAt)
}

func TestInstanceEnter(t *testing.T) {
	t.Parallel()
	i := flow.Instance{
		State: "open", Generation: 2, Previous: "draft", NextDueAt: t0,
		Restarts: map[string]time.Time{"x": t0}, Reminders: map[string]flow.ReminderRecord{"r": {}},
	}
	at := time.Date(2026, 10, 12, 10, 0, 0, 1500, time.FixedZone("x", 3600))
	got := i.Enter(flow.Move{From: "open", To: "review"}, at, 7)
	require.Equal(t, flow.Instance{
		State: "review", Generation: 3, EnteredAt: at.UTC().Truncate(time.Microsecond), Cause: 7, Previous: "open",
	}, got)
	again := got.Enter(flow.Move{From: "review", To: "review"}, at, 0)
	require.Equal(t, "open", again.Previous, "a re-entry keeps the previous state")
	require.True(t, flow.Instance{}.IsZero())
	require.False(t, got.IsZero())
}

type branch struct {
	to      string
	admit   bool
	holds   bool
	err     error
	blocked bool
}

func TestDecide(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	rules := flow.Rules[branch]{
		Admits: func(b branch) bool { return b.admit },
		Holds:  func(b branch) (bool, error) { return b.holds, b.err },
		Gate:   func(b branch) bool { return !b.blocked },
		Target: func(b branch) string { return b.to },
	}
	cases := []struct {
		name     string
		branches []branch
		outcome  flow.Outcome
		to       string
	}{
		{name: "none", outcome: flow.NoMove},
		{
			name:     "first admitted holding branch",
			branches: []branch{{to: "a", holds: true}, {to: "b", admit: true}, {to: "c", admit: true, holds: true}},
			outcome:  flow.Moved, to: "c",
		},
		{
			name:     "guard error stops",
			branches: []branch{{to: "a", admit: true, err: boom}, {to: "b", admit: true, holds: true}},
			outcome:  flow.GuardFailed, to: "a",
		},
		{
			name:     "gate stops",
			branches: []branch{{to: "a", admit: true, holds: true, blocked: true}, {to: "b", admit: true, holds: true}},
			outcome:  flow.Gated, to: "a",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := flow.Decide("s", tc.branches, rules)
			require.Equal(t, tc.outcome, d.Outcome)
			require.Equal(t, tc.to, d.Branch.to)
			if tc.outcome == flow.Moved {
				require.Equal(t, flow.Move{From: "s", To: tc.to}, d.Move)
			}
			if tc.outcome == flow.GuardFailed {
				require.ErrorIs(t, d.Err, boom)
			}
		})
	}
	noGate := rules
	noGate.Gate = nil
	d := flow.Decide("s", []branch{{to: "a", admit: true, holds: true, blocked: true}}, noGate)
	require.Equal(t, flow.Moved, d.Outcome, "no gate admits every branch")
}

func TestDefinition(t *testing.T) {
	t.Parallel()
	d := review(t)
	require.Equal(t, "open", d.Initial())
	st, ok := d.State("open")
	require.True(t, ok)
	require.Equal(t, "open", st.Name())
	require.False(t, st.Final())
	timers := st.Timers()
	timers[0].Name = "changed"
	require.Equal(t, "escalate", st.Timers()[0].Name, "Timers returns a copy")
	_, ok = d.State("ghost")
	require.False(t, ok)
	require.Len(t, d.Reminders(), 1)
	require.Empty(t, d.RemindersIn("done"))
	_, ok = d.Reminder("ghost")
	require.False(t, ok)
	r := mustReminder(t, d)
	in := r.In()
	in[0] = "changed"
	require.Equal(t, []string{"open", "waiting"}, r.In(), "In returns a copy")
	require.True(t, r.Active("waiting"))
	require.False(t, r.Active("done"))
	pruned := flow.PruneReminders(d, map[string]flow.ReminderRecord{"nudge": {}, "gone": {}})
	require.Equal(t, map[string]flow.ReminderRecord{"nudge": {}}, pruned)
}

func TestRecords(t *testing.T) {
	t.Parallel()
	require.Nil(t, flow.CloneReminders(nil))
	rs := map[string]flow.ReminderRecord{"a": {Count: 1}}
	c := flow.CloneReminders(rs)
	c["a"] = flow.ReminderRecord{Count: 2}
	require.Equal(t, int64(1), rs["a"].Count)
	local := time.Date(2026, 1, 1, 10, 0, 0, 1500, time.FixedZone("x", 3600))
	n := flow.NormReminder(flow.ReminderRecord{Since: local, Count: 4})
	require.Equal(t, local.UTC().Truncate(time.Microsecond), n.Since)
	require.True(t, n.Last.IsZero())
	require.True(t, flow.NormTime(time.Time{}).IsZero())
}

func TestStatusTerminal(t *testing.T) {
	t.Parallel()
	for _, s := range []flow.Status{flow.StatusCompleted, flow.StatusCancelled, flow.StatusFailed} {
		require.True(t, s.Terminal(), s)
	}
	for _, s := range []flow.Status{flow.StatusWorking, flow.StatusInputRequired} {
		require.False(t, s.Terminal(), s)
	}
}

func TestPeriod(t *testing.T) {
	t.Parallel()
	p := flow.FixedPeriod(30*24*time.Hour, "30d")
	d, fixed := p.Fixed()
	require.True(t, fixed)
	require.Equal(t, 30*24*time.Hour, d)
	require.Equal(t, "30d", p.String())
	require.Equal(t, period(t, "30d"), p)
}

func TestDueWake(t *testing.T) {
	t.Parallel()
	w := flow.NewDueWake()
	require.True(t, w.Take().IsZero())
	w.Note(time.Time{})
	select {
	case <-w.C():
		t.Fatal("a zero time pings")
	default:
	}
	w.Note(t0.Add(2 * time.Hour))
	w.Note(t0.Add(time.Hour))
	<-w.C()
	require.Equal(t, t0.Add(time.Hour), w.Take(), "the earliest is kept")
	require.True(t, w.Take().IsZero(), "Take clears it")
}
