package channels

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testMark is a valid outputs mark of a move from detected to contained.
func testMark() *OutputMark {
	return &OutputMark{
		Writer: alice, Roles: []string{"dpo"}, Gen: 2, From: "detected", To: "contained", Cause: 4,
		Attempt: 1, Claim: "a", ClaimedAt: t0,
		Results: []OutputResult{{
			Kind: OutputKindCreate, Status: OutputDone, Props: []string{"title"}, Warnings: []string{"w"},
			Entity: &Promoted{ID: "DOC-1", Type: "doc"},
		}},
	}
}

func TestOutputMark_Clone(t *testing.T) {
	var none *OutputMark
	require.Nil(t, none.Clone())

	m := testMark()
	c := m.Clone()
	require.Equal(t, m, c)
	c.Roles[0], c.Results[0].Props[0], c.Results[0].Warnings[0], c.Results[0].Entity.ID = "x", "x", "x", "x"
	require.Equal(t, testMark(), m, "the clone shares no slice or pointer with m")

	require.Nil(t, (&OutputMark{Writer: alice}).Clone().Results, "no results stay nil")
}

func TestPrepareAction_OutputMark(t *testing.T) {
	pending := func(m *OutputMark) Action {
		return Action{Status: ActionPending, By: alice, At: t0, Outputs: m}
	}
	got, err := PrepareAction(OutputsAction, pending(testMark()))
	require.NoError(t, err)
	require.Equal(t, time.UTC, got.Outputs.ClaimedAt.Location(), "the claim time is normalized")

	tooMany := testMark()
	tooMany.Results = make([]OutputResult, MaxOutputResults+1)
	huge := testMark()
	huge.Roles = []string{strings.Repeat("r", MaxPropsBytes)}
	noWriter := testMark()
	noWriter.Writer = Author{}
	badMove := testMark()
	badMove.Gen = 0
	badEntity := testMark()
	badEntity.Results[0].Entity = &Promoted{ID: "", Type: "doc"}
	both := testMark()
	both.Timer, both.Job = "late", true
	neither := testMark()
	neither.Cause = 0
	timerNotJob := testMark()
	timerNotJob.Cause, timerNotJob.Timer = 0, "late"
	timer := testMark()
	timer.Cause, timer.Timer, timer.Job = 0, "late", true
	_, err = PrepareAction(OutputsAction, pending(timer))
	require.NoError(t, err, "a timer's job mark names no cause")

	tests := []struct {
		name   string
		action string
		next   Action
		want   error
	}{
		{"outputs without a mark", OutputsAction, pending(nil), ErrInvalidMessage},
		{"another action with a mark", "accept", pending(testMark()), ErrInvalidMessage},
		{"no writer", OutputsAction, pending(noWriter), ErrInvalidMessage},
		{"too many results", OutputsAction, pending(tooMany), ErrTooLarge},
		{"no generation", OutputsAction, pending(badMove), ErrInvalidMessage},
		{"invalid created entity", OutputsAction, pending(badEntity), ErrInvalidMessage},
		{"too large", OutputsAction, pending(huge), ErrTooLarge},
		{"cause and timer", OutputsAction, pending(both), ErrInvalidMessage},
		{"neither cause nor timer", OutputsAction, pending(neither), ErrInvalidMessage},
		{"timer without job", OutputsAction, pending(timerNotJob), ErrInvalidMessage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, perr := PrepareAction(tc.action, tc.next)
			require.ErrorIs(t, perr, tc.want)
		})
	}

	_, err = PrepareAction("accept", pending(nil))
	require.NoError(t, err, "another action without a mark")
}

func TestCheckOutputStep(t *testing.T) {
	held := t0.Add(time.Minute)
	cur := &Action{Name: OutputsAction, Status: ActionPending, PendingAt: held, Outputs: testMark()}
	step := func(status ActionStatus, at time.Time, edit func(*OutputMark)) Action {
		m := testMark()
		if edit != nil {
			edit(m)
		}
		return Action{Status: status, At: at, Outputs: m}
	}
	claim := func(m *OutputMark) { m.Attempt, m.Claim = 2, "b" }
	after := held.Add(time.Second)

	tests := []struct {
		name   string
		cur    *Action
		action string
		expect ActionStatus
		next   Action
		want   error
	}{
		{"progress", cur, OutputsAction, ActionPending, step(ActionPending, t0, nil), nil},
		{"done", cur, OutputsAction, ActionPending, step(ActionDone, t0, nil), nil},
		{"claim after the hold", cur, OutputsAction, ActionPending, step(ActionPending, after, claim), nil},
		{"another action", nil, "accept", ActionNone, Action{Status: ActionPending}, nil},
		{
			"another action pending to pending", nil, "accept", ActionPending,
			Action{Status: ActionPending}, ErrInvalidMessage,
		},
		{"taken from none", cur, OutputsAction, ActionNone, step(ActionPending, t0, nil), ErrInvalidMessage},
		{
			"on a message without a mark", &Action{Status: ActionPending}, OutputsAction, ActionPending,
			step(ActionPending, t0, nil), ErrInvalidMessage,
		},
		{"released", cur, OutputsAction, ActionPending, step(ActionNone, t0, nil), ErrInvalidMessage},
		{
			"move changed", cur, OutputsAction, ActionPending,
			step(ActionPending, t0, func(m *OutputMark) { m.To = "closed" }), ErrInvalidMessage,
		},
		{
			"roles changed", cur, OutputsAction, ActionPending,
			step(ActionPending, t0, func(m *OutputMark) { m.Roles = nil }), ErrInvalidMessage,
		},
		{
			"claim with the same token", cur, OutputsAction, ActionPending,
			step(ActionPending, after, func(m *OutputMark) { m.Attempt = 2 }), ErrConflict,
		},
		{"claim while held", cur, OutputsAction, ActionPending, step(ActionPending, t0, claim), ErrConflict},
		{"claim that is done", cur, OutputsAction, ActionPending, step(ActionDone, after, claim), ErrInvalidMessage},
		{
			"another claim's write", cur, OutputsAction, ActionPending,
			step(ActionPending, t0, func(m *OutputMark) { m.Claim = "c" }), ErrConflict,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckOutputStep(tc.cur, tc.action, tc.expect, tc.next)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestSessionMark(t *testing.T) {
	cause := Message{Seq: 4, At: t0}
	move := SessionMove{From: "detected", To: "contained"}

	got, err := sessionMark(&SessionStep{}, cause, move, 2)
	require.NoError(t, err)
	require.Nil(t, got, "a step without Mark marks nothing")

	got, err = sessionMark(&SessionStep{Mark: func(SessionMove) *Action { return nil }}, cause, move, 2)
	require.NoError(t, err)
	require.Nil(t, got, "a Mark that gives no action marks nothing")

	mark := func(SessionMove) *Action {
		return &Action{PendingAt: t0.Add(time.Minute), Outputs: &OutputMark{Writer: alice, Attempt: 1, Claim: "a"}}
	}
	got, err = sessionMark(&SessionStep{Mark: mark}, cause, move, 2)
	require.NoError(t, err)
	require.Equal(t, OutputsAction, got.Name)
	require.Equal(t, ActionPending, got.Status)
	require.Equal(t, alice, got.By, "the mark's writer takes the action")
	m := got.Outputs
	require.Equal(t, []any{int64(2), "detected", "contained", int64(4)}, []any{m.Gen, m.From, m.To, m.Cause},
		"the store fills in the move")

	invalid := func(SessionMove) *Action { return &Action{Outputs: &OutputMark{}} }
	_, err = sessionMark(&SessionStep{Mark: invalid}, cause, move, 2)
	require.ErrorIs(t, err, ErrInvalidMessage, "a mark without a writer")
}
