package realtime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// remindOpts are the knobs of [remindRealtimeWith].
type remindOpts struct {
	every, first, max string
	// after is the after: block of the contained state, "" for none.
	after string
	// extra is appended to the reminders block.
	extra string
}

// remindRealtimeWith declares breach-response with the status-update
// reminder of realtimecfg's testdata/valid.yaml, with o's interval, first
// slot and max; notified is not final, so reopened leads back into the
// reminder's span. pair has two reminders of one state. tasks is a
// cardinality-many type with a one-minute reminder, and deadline one with
// a two-minute timer and no reminder. alice owns TKT-1, so she holds lead.
func remindRealtimeWith(o remindOpts) string {
	after := ""
	if o.after != "" {
		after = "\n          after: " + o.after
	}
	maxLine := ""
	if o.max != "" {
		maxLine = "\n          max: " + o.max
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
      mod:    { permission: chat.moderate, label: moderator }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead] }
      notified:  { properties: { reference: { type: string } }, post: [lead] }
      reopened:  { properties: { note: { type: string } }, post: [lead] }
      wrapped:   { properties: { note: { type: string } }, post: [lead] }
      reminder:
        properties: { body: { type: markdown, required: true }, level: { type: risk-level } }
        post: [member]
      escalate-stalled: { properties: { note: { type: string } }, post: [lead] }
    message_ops:
      edit_own:   { roles: [member] }
      delete_own: { roles: [member] }
      remove:     { roles: [mod] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      initial: detected
      states:
        detected:
          on:
            contained: { to: contained }
            notified:  { to: notified }
        contained:
          on:
            assessed: { to: assessed }
            notified: { to: notified }
            escalate-stalled: { to: overdue, when: "session.reminders['status-update'] >= 3" }` + after + `
        assessed: { on: { notified: { to: notified } } }
        overdue:  { on: { notified: { to: notified } } }
        notified: { on: { reopened: { to: detected }, wrapped: { to: done } } }
        done: { final: true }
      reminders:
        status-update:
          label: Status update
          in: [detected, contained, assessed, overdue]
          every: ` + o.every + `
          first: ` + o.first + maxLine + `
          post:
            type: reminder
            props: { body: "Post a status update", level: high }` + o.extra + `
  pair:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown, required: true } }, post: [member] }
      ping: { properties: { body: { type: markdown, required: true } }, post: [member] }
    session:
      initial: live
      states:
        live: { on: { text: { to: over } } }
        over: { final: true }
      reminders:
        alpha: { in: [live], every: 1h, first: 1h, post: { type: ping, props: { body: a } } }
        beta:  { in: [live], every: 1h, first: 1h, post: { type: ping, props: { body: b } } }
  tasks:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      done: { properties: { note: { type: string } }, post: [member] }
      poke: { properties: { body: { type: markdown, required: true } }, post: [member] }
    session:
      initial: open-task
      states:
        open-task: { on: { done: { to: finished } } }
        finished: { final: true }
      reminders:
        poke: { in: [open-task], every: 1m, first: 1m, post: { type: poke, props: { body: poke } } }
  deadline:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown, required: true } }, post: [member] }
    session:
      initial: waiting
      states:
        waiting: { after: { due: { period: 2m, to: late } } }
        late: { final: true }
`
}

// remindDefaults is the status-update reminder of testdata/valid.yaml.
var remindDefaults = remindOpts{every: "30m", first: "10m", max: "48"}

var remindRealtime = remindRealtimeWith(remindDefaults)

// remindSession returns the breach-response session of yaml and its
// status-update reminder.
func remindSession(t *testing.T, yaml string) (*realtimecfg.Session, realtimecfg.SessionReminder) {
	t.Helper()
	ct, ok := testCompiled(t, yaml).ChannelType("breach-response")
	require.True(t, ok)
	s := ct.Session()
	r, ok := s.Reminder("status-update")
	require.True(t, ok)
	return s, r
}

func inState(state string, gen int64, entered time.Time, recs map[string]channels.ReminderRecord) channels.Channel {
	return channels.Channel{CreatedAt: t0, Session: channels.Session{
		State: state, Generation: gen, EnteredAt: entered, Reminders: recs,
	}}
}

// rec is a status-update record whose span started at t0, with its last
// post at t0+last (none for a negative last) and count posts.
func rec(last time.Duration, count int64) map[string]channels.ReminderRecord {
	r := channels.ReminderRecord{Since: t0, Count: count}
	if last >= 0 {
		r.Last = t0.Add(last)
	}
	return map[string]channels.ReminderRecord{"status-update": r}
}

func TestNextSlot(t *testing.T) {
	t.Parallel()
	s, r := remindSession(t, remindRealtime)
	_, long := remindSession(t, remindRealtimeWith(remindOpts{every: "30m", first: "1m"}))
	_, short := remindSession(t, remindRealtimeWith(remindOpts{every: "7m", first: "10m"}))
	_, shifted := remindSession(t, remindRealtimeWith(remindOpts{every: "30m", first: "25m"}))
	_, minute := remindSession(t, remindRealtimeWith(remindOpts{every: "1m", first: "1m"}))
	_, later := remindSession(t, remindRealtimeWith(remindOpts{every: "30m", first: "5h"}))
	_ = s
	tests := []struct {
		name string
		r    realtimecfg.SessionReminder
		ch   channels.Channel
		want time.Duration
		none bool
	}{
		{name: "unset session counts from created_at", r: r, ch: channels.Channel{CreatedAt: t0}, want: 10 * time.Minute},
		{
			name: "a move into the span counts from the move", r: r,
			ch: inState("detected", 2, t0.Add(time.Hour), nil), want: time.Hour + 10*time.Minute,
		},
		{name: "zero last gives since plus first", r: r, ch: inState("contained", 1, t0, rec(-1, 0)), want: 10 * time.Minute},
		{
			name: "missed slots are skipped", r: r,
			ch: inState("contained", 1, t0, rec(2*time.Hour+40*time.Minute, 3)), want: 3*time.Hour + 10*time.Minute,
		},
		{
			name: "a post on a slot gives the next", r: r,
			ch: inState("contained", 1, t0, rec(40*time.Minute, 2)), want: 70 * time.Minute,
		},
		{name: "max reached", r: r, ch: inState("contained", 1, t0, rec(time.Hour, 48)), none: true},
		{name: "not active in the state", r: r, ch: inState("notified", 3, t0, nil), none: true},
		{name: "stranded", r: r, ch: inState("ghost", 3, t0, rec(-1, 0)), none: true},
		{
			name: "every lengthened after 100 posts stays within one every", r: long,
			ch: inState("detected", 0, t0, rec(101*time.Minute, 100)), want: 121 * time.Minute,
		},
		{
			name: "every shortened gives the first new slot after last", r: short,
			ch: inState("detected", 0, t0, rec(70*time.Minute, 3)), want: 73 * time.Minute,
		},
		{
			name: "a changed first moves the grid", r: shifted,
			ch: inState("detected", 0, t0, rec(70*time.Minute, 3)), want: 85 * time.Minute,
		},
		{
			// Posts at 10m, 40m and 70m under first 10m; first is now 5h. The
			// grid 5h - k*30m holds 90m, the first slot after the last post.
			name: "first lengthened from 10m to 5h stays within one every", r: later,
			ch: inState("detected", 0, t0, rec(70*time.Minute, 3)), want: 90 * time.Minute,
		},
		{
			name: "a last post on a grid point before first gives the next", r: later,
			ch: inState("detected", 0, t0, rec(time.Hour, 2)), want: 90 * time.Minute,
		},
		{
			name: "a minute grid", r: minute,
			ch: inState("detected", 0, t0, rec(5*time.Minute, 5)), want: 6 * time.Minute,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess := s
			got, ok := nextSlot(tc.r, currentSession(sess, tc.ch), tc.ch)
			if tc.none {
				require.False(t, ok, got)
				return
			}
			require.True(t, ok)
			require.Equal(t, t0.Add(tc.want), got)
		})
	}
}

func TestNextDue_Both(t *testing.T) {
	t.Parallel()
	slack := realtimelimits.ReminderSlack
	tests := []struct {
		name  string
		after string
		ch    channels.Channel
		want  time.Time
	}{
		{name: "reminder only", ch: channels.Channel{CreatedAt: t0}, want: t0.Add(10*time.Minute + slack)},
		{
			name: "timer first", after: "{ t: { period: 5m, to: overdue } }",
			ch: inState("contained", 1, t0, nil), want: t0.Add(5 * time.Minute),
		},
		{
			name: "reminder first", after: "{ t: { period: 1h, to: overdue } }",
			ch: inState("contained", 1, t0, nil), want: t0.Add(10*time.Minute + slack),
		},
		{
			name: "tie", after: "{ t: { period: 630s, to: overdue } }",
			ch: inState("contained", 1, t0, nil), want: t0.Add(10*time.Minute + slack),
		},
		{
			name: "normalized", ch: inState("detected", 0, t0.Add(1500*time.Nanosecond), nil),
			want: channels.NormTime(t0.Add(1500*time.Nanosecond + 10*time.Minute + slack)),
		},
		{name: "neither", ch: inState("notified", 2, t0, nil)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := remindDefaults
			o.after = tc.after
			s, _ := remindSession(t, remindRealtimeWith(o))
			require.Equal(t, tc.want, nextDue(s, tc.ch))
		})
	}
}

func TestStepScheduleReminders(t *testing.T) {
	t.Parallel()
	o := remindDefaults
	o.after = "{ notify-deadline: { period: 72h, since: session, to: overdue } }"
	s, _ := remindSession(t, remindRealtimeWith(o))
	moved := func(from channels.Channel, to string, at time.Duration) channels.Channel {
		next := from
		next.Session = channels.Session{State: to, Generation: from.Session.Generation + 1, EnteredAt: t0.Add(at)}
		return next
	}
	t.Run("a move between in states keeps the record", func(t *testing.T) {
		t.Parallel()
		from := inState("contained", 1, t0, rec(40*time.Minute, 2))
		got := stepSchedule(s, from, moved(from, "assessed", time.Hour))
		require.Equal(t, rec(40*time.Minute, 2), got.Reminders)
	})
	t.Run("an implicit record is stored explicitly", func(t *testing.T) {
		t.Parallel()
		from := channels.Channel{CreatedAt: t0}
		got := stepSchedule(s, from, moved(from, "contained", 25*time.Minute))
		require.Equal(t, rec(-1, 0), got.Reminders)
		require.Equal(t, t0.Add(10*time.Minute+realtimelimits.ReminderSlack), got.NextDueAt)
	})
	t.Run("a move out of in drops the record", func(t *testing.T) {
		t.Parallel()
		from := inState("contained", 1, t0, rec(40*time.Minute, 2))
		got := stepSchedule(s, from, moved(from, "notified", time.Hour))
		require.NotNil(t, got.Reminders)
		require.Empty(t, got.Reminders)
	})
	t.Run("a move into in starts a new span", func(t *testing.T) {
		t.Parallel()
		from := inState("notified", 2, t0, nil)
		next := moved(from, "detected", 5*time.Hour)
		got := stepSchedule(s, from, next)
		require.Empty(t, got.Reminders)
		next.Session.Reminders = got.Reminders
		slot, ok := nextSlot(mustReminder(t, s), currentSession(s, next), next)
		require.True(t, ok)
		require.Equal(t, t0.Add(5*time.Hour+10*time.Minute), slot)
	})
	t.Run("a re-entry keeps the record", func(t *testing.T) {
		t.Parallel()
		from := inState("contained", 1, t0, rec(40*time.Minute, 2))
		next := moved(from, "contained", time.Hour)
		require.Equal(t, rec(40*time.Minute, 2), stepSchedule(s, from, next).Reminders)
	})
	t.Run("no move keeps the records", func(t *testing.T) {
		t.Parallel()
		from := inState("contained", 1, t0, rec(40*time.Minute, 2))
		got := stepSchedule(s, from, from)
		require.Equal(t, rec(40*time.Minute, 2), got.Reminders)
		require.Nil(t, got.Restarts)
		require.Nil(t, stepSchedule(s, channels.Channel{CreatedAt: t0}, channels.Channel{CreatedAt: t0}).Reminders,
			"nil keeps an empty store")
	})
	t.Run("restart anchors carry alongside", func(t *testing.T) {
		t.Parallel()
		from := inState("contained", 1, t0, rec(40*time.Minute, 2))
		from.Session.Restarts = map[string]time.Time{"notify-deadline": t0.Add(time.Hour)}
		s2 := testCompiled(t, strings.Replace(remindRealtimeWith(o),
			"assessed: { on: { notified: { to: notified } } }",
			"assessed: { on: { notified: { to: notified } }, after: "+
				"{ notify-deadline: { period: 72h, since: session, to: overdue } } }", 1))
		ct, _ := s2.ChannelType("breach-response")
		got := stepSchedule(ct.Session(), from, moved(from, "assessed", 2*time.Hour))
		require.Equal(t, map[string]time.Time{"notify-deadline": t0.Add(time.Hour)}, got.Restarts)
		require.Equal(t, rec(40*time.Minute, 2), got.Reminders)
	})
}

func mustReminder(t *testing.T, s *realtimecfg.Session) realtimecfg.SessionReminder {
	t.Helper()
	r, ok := s.Reminder("status-update")
	require.True(t, ok)
	return r
}

func TestResumeScheduleReminders(t *testing.T) {
	t.Parallel()
	o := remindDefaults
	o.after = "{ notify-deadline: { period: 72h, since: session, to: overdue } }"
	s, _ := remindSession(t, remindRealtimeWith(o))
	t.Run("a due reminder restarts at the resume", func(t *testing.T) {
		t.Parallel()
		ch := inState("contained", 1, t0, rec(40*time.Minute, 2))
		got := resumeSession(s, ch, t0.Add(5*time.Hour))
		require.Equal(t, map[string]channels.ReminderRecord{
			"status-update": {Since: t0.Add(5 * time.Hour), Count: 2},
		}, got.Reminders)
		require.Equal(t, t0.Add(5*time.Hour+10*time.Minute+realtimelimits.ReminderSlack), got.NextDueAt)
	})
	t.Run("a reminder not yet due keeps its record", func(t *testing.T) {
		t.Parallel()
		ch := inState("contained", 1, t0, rec(40*time.Minute, 2))
		got := resumeSession(s, ch, t0.Add(50*time.Minute))
		require.Equal(t, rec(40*time.Minute, 2), got.Reminders)
	})
	t.Run("two cycles keep a slot not due at the second", func(t *testing.T) {
		t.Parallel()
		ch := inState("contained", 1, t0, rec(40*time.Minute, 2))
		first := resumeSession(s, ch, t0.Add(5*time.Hour))
		ch.Session.Reminders, ch.Session.Restarts = first.Reminders, first.Restarts
		got := resumeSession(s, ch, t0.Add(5*time.Hour+5*time.Minute))
		require.Equal(t, first.Reminders, got.Reminders)
	})
	t.Run("timer anchors resume in the same call", func(t *testing.T) {
		t.Parallel()
		ch := inState("contained", 1, t0, rec(40*time.Minute, 2))
		got := resumeSession(s, ch, t0.Add(100*time.Hour))
		require.Equal(t, map[string]time.Time{"notify-deadline": t0.Add(100 * time.Hour)}, got.Restarts)
		require.Equal(t, t0.Add(100*time.Hour), got.Reminders["status-update"].Since)
		require.Equal(t, t0.Add(100*time.Hour+10*time.Minute+realtimelimits.ReminderSlack), got.NextDueAt)
	})
}

func TestSchedulesPruneUndeclared(t *testing.T) {
	t.Parallel()
	s, r := remindSession(t, remindRealtime)
	stale := map[string]channels.ReminderRecord{"status-update": {Since: t0, Count: 1}}
	for i := range channels.MaxSessionReminders - 1 {
		stale[fmt.Sprintf("old-%d", i)] = channels.ReminderRecord{Since: t0, Count: 4}
	}
	ch := inState("contained", 1, t0, stale)
	want := map[string]channels.ReminderRecord{"status-update": {Since: t0, Count: 1}}

	moved := ch
	moved.Session = channels.Session{State: "assessed", Generation: 2, EnteredAt: t0.Add(time.Minute)}
	require.Equal(t, want, stepSchedule(s, ch, moved).Reminders, "a move")
	require.Equal(t, want, resumeSession(s, ch, t0.Add(time.Minute)).Reminders, "a resume")
	step := reminderStep(testChannelType(t, remindRealtime), s, r, stale["status-update"], nil, t0)
	require.Equal(t, want, step.Due(ch).Reminders, "a reminder step")

	t.Run("renamed reminders post without error", func(t *testing.T) {
		t.Parallel()
		renamed := strings.Replace(remindRealtime, "status-update:", "status-update-2:", 1)
		renamed = strings.Replace(renamed, "session.reminders['status-update']", "session.reminders['status-update-2']", 1)
		th := newRemindHarness(t, renamed)
		th.open(t, "TKT-1")
		full := map[string]channels.ReminderRecord{}
		for i := range channels.MaxSessionReminders {
			full[fmt.Sprintf("old-%d", i)] = channels.ReminderRecord{Since: t0, Count: 2}
		}
		th.seedReminders(t, breachKey("TKT-1"), full)
		th.at(10 * time.Minute)
		rep := th.round(t)
		require.Equal(t, 1, rep.Reminded, "%+v", rep)
		require.Zero(t, rep.Failed)
		head := th.head(t, breachKey("TKT-1"))
		require.LessOrEqual(t, len(head.Session.Reminders), channels.MaxSessionReminders)
		require.Equal(t, map[string]channels.ReminderRecord{
			"status-update-2": {Since: t0, Last: t0.Add(10 * time.Minute), Count: 1},
		}, head.Session.Reminders)
	})
}

func testChannelType(t *testing.T, yaml string) realtimecfg.ChannelType {
	t.Helper()
	ct, ok := testCompiled(t, yaml).ChannelType("breach-response")
	require.True(t, ok)
	return ct
}
