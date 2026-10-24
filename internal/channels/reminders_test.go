package channels

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplySchedule_Reminders(t *testing.T) {
	zone := time.FixedZone("CEST", 2*60*60)
	stored := map[string]ReminderRecord{"old": {Since: t0, Count: 1}}
	many := map[string]ReminderRecord{}
	for i := range MaxSessionReminders + 1 {
		many[fmt.Sprintf("r%d", i)] = ReminderRecord{Since: t0}
	}
	cases := []struct {
		name    string
		in      map[string]ReminderRecord
		wantErr bool
		want    map[string]ReminderRecord
	}{
		{name: "nil keeps the stored records", in: nil, want: stored},
		{name: "empty stores none", in: map[string]ReminderRecord{}, want: nil},
		{
			name: "normalized",
			in:   map[string]ReminderRecord{"status-update": {Since: t0.Add(time.Nanosecond).In(zone), Count: 2}},
			want: map[string]ReminderRecord{"status-update": {Since: NormTime(t0.Add(time.Nanosecond)), Count: 2}},
		},
		{name: "too many", in: many, wantErr: true, want: stored},
		{name: "invalid name", in: map[string]ReminderRecord{"Bad Name": {Since: t0}}, wantErr: true, want: stored},
		{name: "negative count", in: map[string]ReminderRecord{"x": {Since: t0, Count: -1}}, wantErr: true, want: stored},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Channel{Session: Session{NextDueAt: t0, Reminders: stored}}
			err := ApplySchedule(&c, func(Channel) SessionSchedule {
				return SessionSchedule{NextDueAt: t0.Add(time.Hour), Reminders: tc.in}
			})
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidMessage)
				require.Equal(t, t0, c.Session.NextDueAt, "a refused schedule changes nothing")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, c.Session.Reminders)
		})
	}
}

func TestCloneReminders(t *testing.T) {
	require.Nil(t, CloneReminders(nil))
	require.Nil(t, CloneReminders(map[string]ReminderRecord{}))
	in := map[string]ReminderRecord{"a": {Since: t0, Count: 1}}
	out := CloneReminders(in)
	out["a"] = ReminderRecord{}
	require.Equal(t, int64(1), in["a"].Count, "the clone shares nothing")
}

func validPost() ReminderPost {
	return ReminderPost{
		Name: "status-update", Record: ReminderRecord{Since: t0, Last: t0.Add(time.Minute), Count: 1},
		Type: "reminder", Props: map[string]any{"body": "hi"}, TriggeredBy: ReminderMarker + "room/status-update",
	}
}

func TestPrepareReminderPost(t *testing.T) {
	at := t0.Add(time.Minute)
	cases := []struct {
		name   string
		mutate func(*ReminderPost)
		want   error
	}{
		{name: "valid", mutate: func(*ReminderPost) {}},
		{name: "no marker", mutate: func(p *ReminderPost) { p.TriggeredBy = "trigger:x" }, want: ErrInvalidMessage},
		{
			name:   "triggered_by too long",
			mutate: func(p *ReminderPost) { p.TriggeredBy = ReminderMarker + strings.Repeat("x", MaxTextBytes) },
			want:   ErrTooLarge,
		},
		{name: "bad type", mutate: func(p *ReminderPost) { p.Type = "Bad Type" }, want: ErrInvalidMessage},
		{name: "bad name", mutate: func(p *ReminderPost) { p.Name = "Bad Name" }, want: ErrInvalidMessage},
		{name: "negative count", mutate: func(p *ReminderPost) { p.Record.Count = -1 }, want: ErrInvalidMessage},
		{
			name:   "last after now",
			mutate: func(p *ReminderPost) { p.Record.Last = at.Add(time.Second) },
			want:   ErrInvalidMessage,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validPost()
			tc.mutate(&p)
			err := PrepareReminderPost(p, at)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestDecideAndApplyReminder(t *testing.T) {
	at := t0.Add(time.Minute)
	cur := Channel{Session: Session{Reminders: map[string]ReminderRecord{"other": {Since: t0}}}}

	post, ok, err := DecideReminder(&SessionStep{}, cur, at)
	require.NoError(t, err)
	require.False(t, ok, "a step without Remind posts nothing")
	require.Equal(t, ReminderPost{}, post)

	none := &SessionStep{Remind: func(Channel) (ReminderPost, bool) { return ReminderPost{}, false }}
	_, ok, err = DecideReminder(none, cur, at)
	require.NoError(t, err)
	require.False(t, ok)

	bad := &SessionStep{Remind: func(Channel) (ReminderPost, bool) {
		p := validPost()
		p.TriggeredBy = "forged"
		return p, true
	}}
	_, ok, err = DecideReminder(bad, cur, at)
	require.ErrorIs(t, err, ErrInvalidMessage)
	require.False(t, ok)

	step := &SessionStep{
		Remind:   func(Channel) (ReminderPost, bool) { return validPost(), true },
		NoticeBy: Author{User: "system:realtime"}, Outcome: &SessionOutcome{},
	}
	post, ok, err = DecideReminder(step, cur, at)
	require.NoError(t, err)
	require.True(t, ok)

	after := cur
	ApplyReminder(&after, post)
	require.Len(t, after.Session.Reminders, 2)
	require.Len(t, cur.Session.Reminders, 1, "the channel copied from keeps its own records")
	require.Equal(t, int64(1), after.Session.Reminders["status-update"].Count)

	var empty Channel
	ApplyReminder(&empty, post)
	require.Len(t, empty.Session.Reminders, 1)

	m, raw, err := ReminderMessage(step, post, 5, at, 9)
	require.NoError(t, err)
	require.JSONEq(t, `{"body":"hi"}`, string(raw))
	require.Equal(t, int64(5), m.Seq)
	require.Equal(t, int64(9), m.Rev)
	require.Equal(t, "reminder", m.Type)
	require.Equal(t, post.TriggeredBy, m.TriggeredBy)
	require.Equal(t, "system:realtime", m.Author.User)

	big := post
	big.Props = map[string]any{"body": strings.Repeat("x", MaxPropsBytes+1)}
	_, _, err = ReminderMessage(step, big, 5, at, 9)
	require.ErrorIs(t, err, ErrTooLarge)

	FillReminderOutcome(step, after)
	require.True(t, step.Outcome.Reminded)
	require.False(t, step.Outcome.Moved)
	FillReminderOutcome(&SessionStep{}, after)
}

func TestPrepareAppendStep_RefusesRemind(t *testing.T) {
	step := &SessionStep{
		Decide: func(Channel) (SessionMove, bool) { return SessionMove{}, false },
		Remind: func(Channel) (ReminderPost, bool) { return ReminderPost{}, false },
	}
	require.ErrorIs(t, PrepareAppendStep(step), ErrInvalidMessage)
}
