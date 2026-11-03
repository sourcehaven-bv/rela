package channels

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplySchedule(t *testing.T) {
	due := t0.Add(time.Hour)
	anchors := func(n int) map[string]time.Time {
		m := make(map[string]time.Time, n)
		for i := range n {
			m[fmt.Sprintf("t%d", i)] = t0
		}
		return m
	}
	old := map[string]time.Time{"old": t0}
	cases := []struct {
		name     string
		f        ScheduleFunc
		wantErr  bool
		wantDue  time.Time
		restarts map[string]time.Time
	}{
		{name: "nil", f: nil, wantDue: t0, restarts: old},
		{
			name:    "due only keeps anchors",
			f:       func(Channel) SessionSchedule { return SessionSchedule{NextDueAt: due} },
			wantDue: NormTime(due), restarts: old,
		},
		{
			name: "anchors normalized",
			f: func(Channel) SessionSchedule {
				return SessionSchedule{NextDueAt: due, Restarts: map[string]time.Time{"stale": t0}}
			},
			wantDue: NormTime(due), restarts: map[string]time.Time{"stale": NormTime(t0)},
		},
		{
			name: "empty anchors clear",
			f: func(Channel) SessionSchedule {
				return SessionSchedule{Restarts: map[string]time.Time{}}
			},
			restarts: nil,
		},
		{
			name: "too many anchors",
			f: func(Channel) SessionSchedule {
				return SessionSchedule{Restarts: anchors(MaxSessionRestarts + 1)}
			},
			wantErr: true, wantDue: t0, restarts: old,
		},
		{
			name: "bad anchor name",
			f: func(Channel) SessionSchedule {
				return SessionSchedule{Restarts: map[string]time.Time{"Bad Name": t0}}
			},
			wantErr: true, wantDue: t0, restarts: old,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Channel{Session: Session{State: "open", NextDueAt: t0, Restarts: old}}
			err := ApplySchedule(&c, tc.f)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidMessage)
			} else {
				require.NoError(t, err)
			}
			require.True(t, tc.wantDue.Equal(c.Session.NextDueAt), c.Session.NextDueAt)
			require.Equal(t, tc.restarts, c.Session.Restarts)
		})
	}
}

func TestPrepareTimerSteps(t *testing.T) {
	noDue := testStep(stay)
	noDue.Due = nil
	require.ErrorIs(t, PrepareSessionStep(noDue), ErrInvalidMessage)

	badTimer := testStep(stay)
	badTimer.Timer = "Bad Timer"
	require.ErrorIs(t, PrepareSessionStep(badTimer), ErrInvalidMessage)

	timed := testStep(stay)
	timed.Timer = "stale"
	require.NoError(t, PrepareTimerStep(*timed))
	require.ErrorIs(t, PrepareAppendStep(timed), ErrInvalidMessage, "an append cannot name a timer")
	require.NoError(t, PrepareAppendStep(testStep(stay)))
	require.NoError(t, PrepareAppendStep(nil))
}

func TestDecideTimer(t *testing.T) {
	unnamed := testStep(move("triage", "done"))
	_, _, err := DecideTimer(unnamed, Channel{})
	require.ErrorIs(t, err, ErrInvalidMessage, "a timer move must name its timer")

	named := testStep(move("triage", "done"))
	named.Timer = "stale"
	mv, moved, err := DecideTimer(named, Channel{})
	require.NoError(t, err)
	require.True(t, moved)
	require.Equal(t, SessionMove{From: "triage", To: "done"}, mv)

	stays := testStep(stay)
	_, moved, err = DecideTimer(stays, Channel{})
	require.NoError(t, err)
	require.False(t, moved)
}

func TestTimerNoticeAndAudit(t *testing.T) {
	step := testStep(stay)
	step.Timer, step.AuditTriggeredBy = "stale", SessionMarker+"room:stale"
	mv := SessionMove{From: "open", To: "done"}
	m, raw, err := TimerNotice(step, 7, t0, mv, 3, 9)
	require.NoError(t, err)
	require.Equal(t, int64(7), m.Seq)
	require.Equal(t, int64(9), m.Rev)
	require.Equal(t, step.NoticeType, m.Type)
	require.Equal(t, step.NoticeBy, m.Author)
	require.True(t, NormTime(t0).Equal(m.At))
	require.Equal(t, step.TriggeredBy, m.TriggeredBy)
	require.JSONEq(t, `{"from":"open","to":"done","generation":3,"timer":"stale"}`, string(raw))

	k := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	r := TimerAudit(k, step, m, mv, 3)
	require.Equal(t, AuditSession, r.Op)
	require.Equal(t, int64(7), r.Seq)
	require.Equal(t, step.NoticeBy, r.By)
	require.Equal(t, "open", r.From)
	require.Equal(t, "done", r.To)
	require.Equal(t, int64(3), r.Generation)
	require.Equal(t, step.AuditTriggeredBy, r.TriggeredBy)
}

func TestCheckDueQuery(t *testing.T) {
	require.ErrorIs(t, CheckDueQuery(nil, 1), ErrInvalidKey)
	require.ErrorIs(t, CheckDueQuery([]string{"Bad Type"}, 1), ErrInvalidKey)
	require.ErrorIs(t, CheckDueQuery([]string{"room"}, 0), ErrInvalidMessage)
	require.NoError(t, CheckDueQuery([]string{"room", "incident"}, 5))
}

func TestSortDue(t *testing.T) {
	ch := func(id string, due time.Time) Channel {
		return Channel{Key: Key{Anchor: Anchor{ID: id}, Type: "room"}, Session: Session{NextDueAt: due}}
	}
	cs := []Channel{ch("TKT-3", t0.Add(time.Hour)), ch("TKT-2", t0), ch("TKT-1", t0)}
	SortDue(cs)
	ids := make([]string, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.Key.ID)
	}
	require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3"}, ids)
}

func TestDamageWarner(t *testing.T) {
	var w DamageWarner
	k := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	require.True(t, w.due(k, t0))
	require.False(t, w.due(k, t0.Add(DamageWarnInterval-time.Second)), "once per interval")
	require.True(t, w.due(Key{Anchor: Anchor{ID: "TKT-2"}, Type: "room"}, t0), "per channel")
	require.True(t, w.due(k, t0.Add(DamageWarnInterval)), "again after the interval")
	w.Warn(k, t0.Add(2*DamageWarnInterval), "test: damaged", errors.New("bad"))
	require.False(t, w.due(k, t0.Add(2*DamageWarnInterval)), "Warn records the warning")
	for i := range maxDamageWarnKeys {
		w.due(Key{Anchor: Anchor{ID: fmt.Sprintf("X-%d", i)}, Type: "room"}, t0)
	}
	require.LessOrEqual(t, len(w.last), maxDamageWarnKeys, "the warner stays bounded")
}
