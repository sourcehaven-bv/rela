package channels

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testStep returns a valid session step around decide.
func testStep(decide func(Channel) (SessionMove, bool)) *SessionStep {
	return &SessionStep{
		Decide: decide, NoticeType: "session", NoticeBy: Author{User: "system:realtime"},
		TriggeredBy: SessionMarker + "room", Outcome: &SessionOutcome{},
		Due: func(Channel) SessionSchedule { return SessionSchedule{} },
	}
}

func move(from, to string) func(Channel) (SessionMove, bool) {
	return func(Channel) (SessionMove, bool) { return SessionMove{From: from, To: to}, true }
}

func stay(Channel) (SessionMove, bool) { return SessionMove{}, false }

func TestSessionIsZero(t *testing.T) {
	require.True(t, Session{}.IsZero())
	require.True(t, Session{EnteredAt: t0}.IsZero(), "a creation time alone is still unset")
	require.False(t, Session{State: "open"}.IsZero())
	require.False(t, Session{Generation: 1}.IsZero())
}

func TestValidStateName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"open", true},
		{"risk-assessed", true},
		{"aux", true},
		{"a1", true},
		{"", false},
		{"Open", false},
		{"1open", false},
		{"has space", false},
		{"under_score", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.ok, ValidStateName(tc.name))
		})
	}
}

func TestPrepareSessionStep(t *testing.T) {
	long := strings.Repeat("x", MaxTextBytes+1)
	cases := []struct {
		name string
		edit func(*SessionStep)
		want error
	}{
		{"valid", func(*SessionStep) {}, nil},
		{"no decider", func(s *SessionStep) { s.Decide = nil }, ErrInvalidMessage},
		{"bad notice type", func(s *SessionStep) { s.NoticeType = "Not Valid" }, ErrInvalidMessage},
		{"no notice author", func(s *SessionStep) { s.NoticeBy = Author{} }, ErrInvalidMessage},
		{"no marker", func(s *SessionStep) { s.TriggeredBy = "trigger:x" }, ErrInvalidMessage},
		{"long triggered_by", func(s *SessionStep) { s.TriggeredBy = SessionMarker + long }, ErrTooLarge},
		{"long audit triggered_by", func(s *SessionStep) { s.AuditTriggeredBy = long }, ErrTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := testStep(stay)
			tc.edit(s)
			err := PrepareSessionStep(s)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
	require.NoError(t, PrepareSessionStep(nil), "an append without a step")
}

func TestCheckSessionMove(t *testing.T) {
	require.NoError(t, CheckSessionMove(SessionMove{From: "a", To: "a"}))
	require.ErrorIs(t, CheckSessionMove(SessionMove{From: "", To: "b"}), ErrInvalidMessage)
	require.ErrorIs(t, CheckSessionMove(SessionMove{From: "a", To: "B"}), ErrInvalidMessage)
}

func TestDecideSession(t *testing.T) {
	cases := []struct {
		name    string
		step    *SessionStep
		moved   bool
		move    SessionMove
		wantErr bool
	}{
		{"no step", nil, false, SessionMove{}, false},
		{"no move", testStep(stay), false, SessionMove{}, false},
		{"move", testStep(move("open", "done")), true, SessionMove{From: "open", To: "done"}, false},
		{"invalid move", testStep(move("open", "Not A State")), false, SessionMove{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, moved, err := DecideSession(tc.step, Channel{State: StateOpen})
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidMessage)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.moved, moved)
			require.Equal(t, tc.move, got)
		})
	}
}

func TestDecideSession_SeesChannel(t *testing.T) {
	var seen Channel
	step := testStep(func(c Channel) (SessionMove, bool) {
		seen = c
		return SessionMove{}, false
	})
	cur := Channel{State: StateClosed, Session: Session{State: "open", Generation: 2}}
	_, _, err := DecideSession(step, cur)
	require.NoError(t, err)
	require.Equal(t, cur, seen)
}

func TestApplySession(t *testing.T) {
	c := Channel{Session: Session{State: "open", Generation: 2}}
	ApplySession(&c, SessionMove{From: "open", To: "done"}, t0)
	require.Equal(t, Session{State: "done", Generation: 3, EnteredAt: NormTime(t0)}, c.Session)
	require.Equal(t, time.UTC, c.Session.EnteredAt.Location())
}

func TestSessionNoticeAndAudit(t *testing.T) {
	step := testStep(stay)
	step.AuditTriggeredBy = "trigger:escalate"
	cause := Message{Seq: 7, Rev: 11, Type: "text", Author: alice, At: NormTime(t0), MovedSession: true}
	mv := SessionMove{From: "open", To: "done"}

	notice, raw, err := SessionNotice(step, cause, mv, 3, 13)
	require.NoError(t, err)
	require.Equal(t, int64(8), notice.Seq)
	require.Equal(t, int64(13), notice.Rev)
	require.Equal(t, "session", notice.Type)
	require.Equal(t, step.NoticeBy, notice.Author)
	require.Equal(t, cause.At, notice.At)
	require.Equal(t, step.TriggeredBy, notice.TriggeredBy)
	require.False(t, notice.MovedSession, "the notice records the move; it did not cause it")
	require.Equal(t, "open", notice.Props["from"])
	require.Equal(t, "done", notice.Props["to"])
	require.EqualValues(t, 3, notice.Props["generation"])
	require.EqualValues(t, 7, notice.Props["cause"])
	require.JSONEq(t, `{"from":"open","to":"done","generation":3,"cause":7}`, string(raw))

	k := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	rec := SessionAudit(k, step, cause, mv, 3)
	require.Equal(t, AuditSession, rec.Op)
	require.Equal(t, int64(7), rec.Seq)
	require.Equal(t, alice, rec.By)
	require.Equal(t, "open", rec.From)
	require.Equal(t, "done", rec.To)
	require.Equal(t, int64(3), rec.Generation)
	require.Equal(t, "trigger:escalate", rec.TriggeredBy)
	require.Equal(t, cause.At, rec.At)
}

func TestFillSessionOutcome(t *testing.T) {
	c := Channel{CreatedAt: NormTime(t0), Session: Session{State: "done", Generation: 1, EnteredAt: NormTime(t0)}}
	mv := SessionMove{From: "open", To: "done"}

	FillSessionOutcome(nil, c, mv, true) // no step: nothing to fill, no panic
	noOutcome := testStep(stay)
	noOutcome.Outcome = nil
	FillSessionOutcome(noOutcome, c, mv, true)

	step := testStep(stay)
	FillSessionOutcome(step, c, mv, true)
	require.Equal(t, SessionOutcome{Session: c.Session, CreatedAt: c.CreatedAt, Moved: true, Move: mv}, *step.Outcome)

	FillSessionOutcome(step, Channel{CreatedAt: c.CreatedAt}, SessionMove{}, false)
	require.Equal(t, SessionOutcome{CreatedAt: c.CreatedAt}, *step.Outcome)
}

// TestPrepareMessage_RefusesSessionMarker pins that only the store writes
// a notice: an append whose TriggeredBy starts with the marker is refused.
func TestPrepareMessage_RefusesSessionMarker(t *testing.T) {
	_, _, err := PrepareMessage(NewMessage{Type: "text", Author: alice, At: t0, TriggeredBy: SessionMarker + "room"})
	require.ErrorIs(t, err, ErrInvalidMessage)
	_, _, err = PrepareMessage(NewMessage{Type: "text", Author: alice, At: t0, TriggeredBy: "trigger:x"})
	require.NoError(t, err)
}
