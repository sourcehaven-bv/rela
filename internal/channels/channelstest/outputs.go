package channelstest

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// outputsTests are the conformance tests of the outputs mark on a session
// notice (TKT-98TR9A).
var outputsTests = []conformanceTest{
	{"OutputsMarkOnAppendMove", testOutputsMarkOnAppendMove},
	{"OutputsClaimCAS", testOutputsClaimCAS},
	{"OutputsProgress", testOutputsProgress},
	{"OutputsEligibility", testOutputsEligibility},
	{"OutputsOnClosedChannel", testOutputsOnClosedChannel},
	{"OutputsRefusals", testOutputsRefusals},
	{"OutputsSurviveMoves", testOutputsSurviveMoves},
}

// claimTTL is the claim TTL the suite uses; the store knows none.
const claimTTL = 90 * time.Second

// markingStep is a step that moves an unset session and marks the move,
// claimed at at by token "a" and eligible at at+claimTTL.
func markingStep(at time.Time) *channels.SessionStep {
	step := sessionStep(fromUnset())
	step.Mark = func(channels.SessionMove) *channels.Action {
		return &channels.Action{
			PendingAt: at.Add(claimTTL),
			Outputs: &channels.OutputMark{
				Writer: Alice, Roles: []string{"dpo"}, Attempt: 1, Claim: "a", ClaimedAt: at, Close: true,
			},
		}
	}
	return step
}

// markedNotice appends a moving message to k at at with [markingStep] and
// returns the notice.
func markedNotice(t *testing.T, s channels.Store, k channels.Key, at time.Time) channels.Message {
	t.Helper()
	step := markingStep(at)
	mustAppend(t, s, k, stepped("move", at, step))
	require.True(t, step.Outcome.Moved)
	n, err := s.Get(context.Background(), k, step.Outcome.Notice)
	require.NoError(t, err)
	return n
}

// next returns the notice's action as the next write: the mark copied,
// with change applied, at at.
func next(n channels.Message, at time.Time, change func(a *channels.Action)) channels.Action {
	a := *n.Action
	a.Outputs = n.Action.Outputs.Clone()
	a.At = at
	change(&a)
	return a
}

func testOutputsMarkOnAppendMove(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	at := T0.Add(time.Minute)
	n := markedNotice(t, h.Store, k, at)
	require.Equal(t, int64(3), n.Seq)
	require.NotNil(t, n.Action)
	require.Equal(t, channels.OutputsAction, n.Action.Name)
	require.Equal(t, channels.ActionPending, n.Action.Status)
	require.Equal(t, Alice, n.Action.By)
	require.Equal(t, channels.NormTime(at.Add(claimTTL)), n.Action.PendingAt)
	require.Equal(t, &channels.OutputMark{
		Writer: Alice, Roles: []string{"dpo"}, Gen: 1, From: "detected", To: "contained", Cause: 2,
		Attempt: 1, Claim: "a", ClaimedAt: channels.NormTime(at), Close: true,
	}, n.Action.Outputs)
	page, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Equal(t, n, page[2], "a page reads the mark as Get does")

	// A step whose decider does not move, or whose Mark returns nil,
	// stores no mark.
	k2 := key("TKT-2", "room")
	still := sessionStep(func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false })
	still.Mark = markingStep(at).Mark
	mustAppend(t, h.Store, k2, stepped("stay", at, still))
	nilMark := sessionStep(fromUnset())
	nilMark.Mark = func(channels.SessionMove) *channels.Action { return nil }
	mustAppend(t, h.Store, k2, stepped("move", at, nilMark))
	page, err = h.Store.Messages(ctx, k2, channels.Page{})
	require.NoError(t, err)
	require.Len(t, page, 3)
	for _, m := range page {
		require.Nil(t, m.Action, "message %d", m.Seq)
	}
	stale, err := h.Store.StalePending(ctx, at.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, stale, 1, "only the marked notice is pending")
}

func testOutputsClaimCAS(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	stores := []channels.Store{h.Store, h.Store}
	if h.Second != nil {
		stores[1] = h.Second(t)
	}
	k := key("TKT-1", "room")
	n := markedNotice(t, h.Store, k, T0)
	claimAt := T0.Add(claimTTL + time.Second)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, s := range stores {
		wg.Go(func() {
			claim := next(n, claimAt, func(a *channels.Action) {
				a.Outputs.Attempt, a.Outputs.Claim, a.Outputs.ClaimedAt = 2, []string{"b", "c"}[i], claimAt
				a.PendingAt = claimAt.Add(claimTTL)
			})
			_, errs[i] = s.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, claim)
		})
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
			continue
		}
		require.ErrorIs(t, err, channels.ErrConflict)
	}
	require.Equal(t, 1, won, "exactly one claim wins")
	got, err := h.Store.Get(ctx, k, n.Seq)
	require.NoError(t, err)
	require.Equal(t, 2, got.Action.Outputs.Attempt)
}

func testOutputsProgress(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	n := markedNotice(t, h.Store, k, T0)
	step := T0.Add(10 * time.Second)
	progress := next(n, step, func(a *channels.Action) {
		a.Outputs.ClaimedAt, a.PendingAt = step, step.Add(claimTTL)
		a.Outputs.Results = []channels.OutputResult{{Kind: channels.OutputKindSet, Status: channels.OutputDone,
			Props: []string{"status"}}}
	})
	got, err := h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, progress)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(step), got.Action.Outputs.ClaimedAt)
	require.Equal(t, channels.NormTime(step.Add(claimTTL)), got.Action.PendingAt)
	require.Equal(t, progress.Outputs.Results, got.Action.Outputs.Results)

	for name, change := range map[string]func(a *channels.Action){
		"another token":   func(a *channels.Action) { a.Outputs.Claim = "z" },
		"another attempt": func(a *channels.Action) { a.Outputs.Attempt = 3 },
	} {
		lost := next(got, step, change)
		_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, lost)
		require.ErrorIs(t, err, channels.ErrConflict, name)
	}

	doneAt := step.Add(time.Second)
	done := next(got, doneAt, func(a *channels.Action) {
		a.Status, a.Outputs.Close = channels.ActionDone, false
		a.Outputs.Results = append(a.Outputs.Results, channels.OutputResult{
			Kind: channels.OutputKindCreate, Status: channels.OutputDone,
			Entity: &channels.Promoted{ID: "TL-1", Type: "timeline-entry"},
		})
	})
	got, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, done)
	require.NoError(t, err)
	require.Equal(t, channels.ActionDone, got.ActionStatus())
	require.Equal(t, "TL-1", got.Action.Outputs.Results[1].Entity.ID)
	read, err := h.Store.Get(ctx, k, n.Seq)
	require.NoError(t, err)
	require.Equal(t, got, read)
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	var recs []channels.AuditRecord
	for _, r := range trail {
		if r.Op == channels.AuditAction {
			recs = append(recs, r)
		}
	}
	require.Len(t, recs, 1, "only the done write is audited")
	require.Equal(t, channels.OutputsAction, recs[0].Action)
	require.Equal(t, Alice, recs[0].By)
	require.Equal(t, n.Seq, recs[0].Seq)
	stale, err := h.Store.StalePending(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, stale)
}

func testOutputsEligibility(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	late := key("TKT-1", "room")
	early := key("TKT-2", "room")
	nLate := markedNotice(t, h.Store, late, T0.Add(time.Minute))
	nEarly := markedNotice(t, h.Store, early, T0)
	stale, err := h.Store.StalePending(ctx, T0.Add(claimTTL))
	require.NoError(t, err)
	require.Empty(t, stale, "a mark is not listed before its PendingAt has passed")
	stale, err = h.Store.StalePending(ctx, T0.Add(claimTTL+time.Second))
	require.NoError(t, err)
	require.Equal(t, []channels.PendingAction{{
		Key: early, Seq: nEarly.Seq, Name: channels.OutputsAction, PendingAt: channels.NormTime(T0.Add(claimTTL)),
	}}, stale)
	stale, err = h.Store.StalePending(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, stale, 2)
	require.Equal(t, early, stale[0].Key, "oldest first, whatever the key order")
	require.Equal(t, late, stale[1].Key)
	require.Equal(t, nLate.Seq, stale[1].Seq)
}

func testOutputsOnClosedChannel(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	n := markedNotice(t, h.Store, k, T0)
	_, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Second)})
	require.NoError(t, err)
	at := T0.Add(2 * time.Second)
	progress := next(n, at, func(a *channels.Action) { a.Outputs.ClaimedAt = at })
	got, err := h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, progress)
	require.NoError(t, err, "progress on a closed channel")
	done := next(got, at, func(a *channels.Action) { a.Status = channels.ActionDone })
	_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, done)
	require.NoError(t, err, "done on a closed channel")
}

func testOutputsRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("plain"))
	n := markedNotice(t, h.Store, k, T0)
	claimAt := T0.Add(claimTTL + time.Second)

	// An outputs action on a message that is not a notice.
	forged := channels.Action{Status: channels.ActionPending, By: Alice, At: T0,
		Outputs: &channels.OutputMark{Writer: Alice, Gen: 1, From: "a", To: "b", Cause: 1}}
	_, err := h.Store.SetAction(ctx, k, 1, channels.OutputsAction, channels.ActionNone, forged)
	require.ErrorIs(t, err, channels.ErrInvalidMessage, "an outputs mark taken outside a session move")
	forged.Outputs = nil
	_, err = h.Store.SetAction(ctx, k, 1, "accept", channels.ActionNone, channels.Action{
		Status: channels.ActionPending, By: Alice, At: T0, Outputs: n.Action.Outputs,
	})
	require.ErrorIs(t, err, channels.ErrInvalidMessage, "another action carries a mark")

	// A pending-to-pending write on another action name.
	_, err = h.Store.SetAction(ctx, k, 1, "accept", channels.ActionNone, forged)
	require.NoError(t, err)
	_, err = h.Store.SetAction(ctx, k, 1, "accept", channels.ActionPending, forged)
	require.ErrorIs(t, err, channels.ErrInvalidMessage, "pending to pending on another action")

	claim := func(change func(m *channels.OutputMark)) channels.Action {
		return next(n, claimAt, func(a *channels.Action) {
			a.Outputs.Attempt, a.Outputs.Claim, a.Outputs.ClaimedAt = 2, "b", claimAt
			change(a.Outputs)
		})
	}
	for name, change := range map[string]func(m *channels.OutputMark){
		"writer":     func(m *channels.OutputMark) { m.Writer = Bob },
		"roles":      func(m *channels.OutputMark) { m.Roles = []string{"admin"} },
		"generation": func(m *channels.OutputMark) { m.Gen = 7 },
		"from":       func(m *channels.OutputMark) { m.From = "other" },
		"to":         func(m *channels.OutputMark) { m.To = "other" },
		"cause":      func(m *channels.OutputMark) { m.Cause = 1 },
	} {
		_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, claim(change))
		require.ErrorIs(t, err, channels.ErrInvalidMessage, "a claim that changes the %s", name)
		progress := next(n, T0.Add(time.Second), func(a *channels.Action) { change(a.Outputs) })
		_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, progress)
		require.ErrorIs(t, err, channels.ErrInvalidMessage, "a progress write that changes the %s", name)
	}
	tooMany := claim(func(m *channels.OutputMark) {
		m.Results = make([]channels.OutputResult, channels.MaxOutputResults+1)
		for i := range m.Results {
			m.Results[i] = channels.OutputResult{Kind: channels.OutputKindSet, Status: channels.OutputDone}
		}
	})
	_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, tooMany)
	require.ErrorIs(t, err, channels.ErrTooLarge, "more than five results")
	oversized := claim(func(m *channels.OutputMark) {
		m.Results = []channels.OutputResult{{Kind: channels.OutputKindSet, Status: channels.OutputFailed,
			Props: []string{strings.Repeat("p", channels.MaxPropsBytes)}}}
	})
	_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, oversized)
	require.ErrorIs(t, err, channels.ErrTooLarge, "an oversized mark")
	_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending,
		channels.Action{By: Bob, At: claimAt})
	require.ErrorIs(t, err, channels.ErrInvalidMessage, "a release of the mark")
	early := next(n, T0.Add(time.Second), func(a *channels.Action) { a.Outputs.Attempt, a.Outputs.Claim = 2, "b" })
	_, err = h.Store.SetAction(ctx, k, n.Seq, channels.OutputsAction, channels.ActionPending, early)
	require.ErrorIs(t, err, channels.ErrConflict, "a claim before the mark is eligible")

	got, err := h.Store.Get(ctx, k, n.Seq)
	require.NoError(t, err)
	require.Equal(t, n, got, "no refused write stored anything")
}

func testOutputsSurviveMoves(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("OLD-1", "room")
	n := markedNotice(t, h.Store, k, T0)
	want := n.Action
	_, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Minute)})
	require.NoError(t, err)
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed,
		channels.Transition{To: channels.StateOpen, By: Bob, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)
	got, err := h.Store.Get(ctx, k, n.Seq)
	require.NoError(t, err)
	require.Equal(t, want, got.Action, "close and reopen")

	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(3*time.Minute)))
	moved := key("NEW-1", "room")
	got, err = h.Store.Get(ctx, moved, n.Seq)
	require.NoError(t, err)
	require.Equal(t, want, got.Action, "rekey")

	th := tombstone(t, h.Store, "NEW-1", "", T0.Add(4*time.Minute))
	require.NoError(t, h.Store.Untombstone(ctx, th, T0.Add(5*time.Minute), nil))
	got, err = h.Store.Get(ctx, moved, n.Seq)
	require.NoError(t, err)
	require.Equal(t, want, got.Action, "tombstone and untombstone")
	stale, err := h.Store.StalePending(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, stale, 1, "the mark is still pending after the moves")
	require.Equal(t, moved, stale[0].Key)
}
