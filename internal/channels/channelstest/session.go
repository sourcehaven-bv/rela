package channelstest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// noticeAuthor is the author the suite gives session notices.
var noticeAuthor = channels.Author{User: "system:realtime", Tool: "realtime", PrincipalType: "system"}

// sessionStep returns a step with decide and the suite's notice fields.
func sessionStep(decide func(channels.Channel) (channels.SessionMove, bool)) *channels.SessionStep {
	return &channels.SessionStep{
		Decide: decide, NoticeType: "session", NoticeBy: noticeAuthor, TriggeredBy: "session:room",
		Outcome: &channels.SessionOutcome{}, Due: noDue,
	}
}

// noDue is the schedule of a session without timers.
func noDue(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{} }

// moveFrom returns a decider that always moves from to to.
func moveFrom(from, to string) func(channels.Channel) (channels.SessionMove, bool) {
	return func(channels.Channel) (channels.SessionMove, bool) {
		return channels.SessionMove{From: from, To: to}, true
	}
}

// fromUnset returns a decider that moves an unset session to contained.
func fromUnset() func(channels.Channel) (channels.SessionMove, bool) {
	return func(c channels.Channel) (channels.SessionMove, bool) {
		if !c.Session.IsZero() {
			return channels.SessionMove{}, false
		}
		return channels.SessionMove{From: "detected", To: "contained"}, true
	}
}

// stepped is text(body) at at with step.
func stepped(body string, at time.Time, step *channels.SessionStep) channels.NewMessage {
	m := text(body)
	m.At, m.Session = at, step
	return m
}

// sessionRecords returns the session records of TKT-1's audit trail.
func sessionRecords(t *testing.T, s channels.Store) []channels.AuditRecord {
	t.Helper()
	trail, err := s.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	var out []channels.AuditRecord
	for _, r := range trail {
		if r.Op == channels.AuditSession {
			out = append(out, r)
		}
	}
	return out
}

// testSessionStep pins a move: the message at seq n, the notice at n+1,
// the session, the rev, the audit record and the outcome, and the session
// on every read of the channel.
func testSessionStep(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	at := T0.Add(time.Minute)
	step := sessionStep(fromUnset())
	step.AuditTriggeredBy = "trigger:escalate"
	msg := mustAppend(t, h.Store, k, stepped("move", at, step))
	require.Equal(t, int64(2), msg.Seq)

	want := channels.Session{State: "contained", Generation: 1, EnteredAt: channels.NormTime(at)}
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, want, head.Session)
	require.Equal(t, int64(3), head.LastSeq)
	require.Equal(t, msg.Rev+1, head.Rev, "the notice takes the next rev")
	require.Equal(t, channels.SessionOutcome{
		Session: want, CreatedAt: head.CreatedAt, Moved: true, Move: channels.SessionMove{From: "detected", To: "contained"},
		Notice: 3,
	}, *step.Outcome)

	page, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Len(t, page, 3)
	notice := page[2]
	require.Equal(t, int64(3), notice.Seq)
	require.Equal(t, head.Rev, notice.Rev)
	require.Equal(t, "session", notice.Type)
	require.Equal(t, noticeAuthor, notice.Author)
	require.Equal(t, "session:room", notice.TriggeredBy)
	require.Equal(t, channels.NormTime(at), notice.At)
	require.Equal(t, map[string]any{
		"from": "detected", "to": "contained", "generation": float64(1), "cause": float64(2),
	}, notice.Props)
	require.Nil(t, notice.Action, "a notice without a mark reads with no outputs, as a slice-4 notice does")
	stale, err := h.Store.StalePending(ctx, at.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, stale, "a notice without a mark is not pending")

	recs := sessionRecords(t, h.Store)
	require.Len(t, recs, 1)
	rec := recs[0]
	require.Equal(t, "detected", rec.From)
	require.Equal(t, notice.Props["from"], rec.From, "the notice and the audit name the same from")
	require.Equal(t, "contained", rec.To)
	require.Equal(t, int64(1), rec.Generation)
	require.Equal(t, int64(2), rec.Seq)
	require.Equal(t, Alice, rec.By)
	require.Equal(t, "trigger:escalate", rec.TriggeredBy)
	require.Equal(t, "room", rec.Type)

	listed, err := h.Store.List(ctx, k.Anchor, "room", "")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, want, listed[0].Session)
	byEntity, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, byEntity, 1)
	require.Equal(t, want, byEntity[0].Session)
	byState, err := h.Store.ListByLifecycle(ctx, "room", channels.StateOpen, T0.Add(24*time.Hour))
	require.NoError(t, err)
	require.Len(t, byState, 1)
	require.Equal(t, want, byState[0].Session)
}

// testSessionNoMove pins that a decider returning no move appends only the
// message, as a plain append does.
func testSessionNoMove(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)

	step := sessionStep(func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false })
	msg := mustAppend(t, h.Store, k, stepped("stay", T0.Add(time.Minute), step))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, before.LastSeq+1, head.LastSeq)
	require.Equal(t, before.Rev+1, head.Rev)
	require.Equal(t, msg.Rev, head.Rev)
	require.True(t, head.Session.IsZero())
	require.Equal(t, channels.SessionOutcome{CreatedAt: head.CreatedAt}, *step.Outcome,
		"no move: the outcome reports the unset session and when the channel was created")
	after, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Equal(t, trail, after)
}

// testSessionDeciderSeesCurrent pins that the decider receives the channel
// as committed: its lifecycle state, session and rev, also after a move
// through another instance and after a close.
func testSessionDeciderSeesCurrent(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	mover := h.Store
	if h.Second != nil {
		mover = h.Second(t)
	}
	mustAppend(t, mover, k, stepped("move", T0.Add(time.Minute), sessionStep(fromUnset())))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)

	var seen channels.Channel
	capture := sessionStep(func(c channels.Channel) (channels.SessionMove, bool) {
		seen = c
		return channels.SessionMove{}, false
	})
	mustAppend(t, h.Store, k, stepped("look", T0.Add(2*time.Minute), capture))
	require.Equal(t, channels.StateOpen, seen.State)
	require.Equal(t, head.Session, seen.Session)
	require.Equal(t, head.Rev, seen.Rev)
	require.Equal(t, head.LastSeq, seen.LastSeq)

	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(3 * time.Minute)})
	require.NoError(t, err)
	mustAppend(t, h.Store, k, stepped("closed", T0.Add(4*time.Minute), capture))
	require.Equal(t, channels.StateClosed, seen.State)
	require.Equal(t, head.Session, seen.Session)
}

// testSessionImplicitCreate pins that the decider sees an open channel with
// an unset session for a cardinality-one channel that does not exist yet,
// and that a move creates and moves it in one step.
func testSessionImplicitCreate(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-2", "room")
	var seen channels.Channel
	step := sessionStep(func(c channels.Channel) (channels.SessionMove, bool) {
		seen = c
		return channels.SessionMove{From: "detected", To: "contained"}, true
	})
	mustAppend(t, h.Store, k, stepped("first", T0, step))
	require.Equal(t, channels.StateOpen, seen.State)
	require.True(t, seen.Session.IsZero())
	require.Equal(t, channels.NormTime(T0), seen.CreatedAt)

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, "contained", head.Session.State)
	require.Equal(t, int64(1), head.Session.Generation)
	require.Equal(t, int64(2), head.LastSeq)
	trail, err := h.Store.Audit(ctx, "TKT-2")
	require.NoError(t, err)
	require.Len(t, trail, 2)
	require.Equal(t, channels.AuditCreate, trail[0].Op)
	require.Equal(t, channels.AuditSession, trail[1].Op)
}

// testSessionInvalidMove pins that a move to a state name the grammar
// refuses stores nothing, and creates no implicit channel.
func testSessionInvalidMove(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	_, err = h.Store.Append(ctx, k, stepped("bad", T0, sessionStep(moveFrom("detected", "Not A State"))))
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, before, head)

	fresh := key("TKT-3", "room")
	_, err = h.Store.Append(ctx, fresh, stepped("bad", T0, sessionStep(moveFrom("", "contained"))))
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	_, err = h.Store.Head(ctx, fresh)
	require.ErrorIs(t, err, channels.ErrNotFound, "an invalid move created the channel")
	trail, err := h.Store.Audit(ctx, "TKT-3")
	require.NoError(t, err)
	require.Empty(t, trail)
}

// testSessionSurvivesMoves pins that the session is kept across a close
// and reopen, a rekey, and a tombstone and untombstone.
func testSessionSurvivesMoves(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("OLD-1", "room")
	mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	want := head.Session
	require.Equal(t, "contained", want.State)

	c, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Minute)})
	require.NoError(t, err)
	require.Equal(t, want, c.Session)
	c, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed,
		channels.Transition{To: channels.StateOpen, By: Bob, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)
	require.Equal(t, want, c.Session)

	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(3*time.Minute)))
	got, err := h.Store.ListEntity(ctx, "NEW-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want, got[0].Session)

	th := tombstone(t, h.Store, "NEW-1", "", T0.Add(4*time.Minute))
	require.NoError(t, h.Store.Untombstone(ctx, th, T0.Add(5*time.Minute), nil))
	got, err = h.Store.ListEntity(ctx, "NEW-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want, got[0].Session)
}

// testSessionConcurrent pins that concurrent appends whose decider moves
// only an unset session move it once: every message is stored, the log
// holds one notice, and it follows the lowest-seq message.
func testSessionConcurrent(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	stores := []channels.Store{h.Store}
	if h.Second != nil {
		stores = append(stores, h.Second(t))
	}
	k := key("TKT-1", "room")
	const n = 8
	var moved atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Go(func() {
			step := sessionStep(fromUnset())
			_, err := stores[i%len(stores)].Append(ctx, k, stepped("race", T0, step))
			if err != nil {
				errs <- err
				return
			}
			if step.Outcome.Moved {
				moved.Add(1)
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), moved.Load())
	page, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Len(t, page, n+1)
	var notices []channels.Message
	for _, m := range page {
		if m.Type == "session" {
			notices = append(notices, m)
		}
	}
	require.Len(t, notices, 1)
	require.Equal(t, int64(2), notices[0].Seq, "the notice follows the first message")
	require.EqualValues(t, 1, notices[0].Props["cause"])
}

// testSessionNoticeMarker pins that only the store writes the session
// marker: an append without a step that carries it is refused, and so is
// a step whose notice lacks it.
func testSessionNoticeMarker(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	forged := text("forged")
	forged.TriggeredBy = "session:room"
	_, err := h.Store.Append(ctx, k, forged)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	step := sessionStep(fromUnset())
	step.TriggeredBy = "trigger:x"
	_, err = h.Store.Append(ctx, k, stepped("x", T0, step))
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	_, err = h.Store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
}

// testSessionStateNames pins that a state named like a Windows device is a
// valid state on every backend, and that a re-entry raises the generation.
func testSessionStateNames(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, stepped("in", T0, sessionStep(moveFrom("detected", "aux"))))
	mustAppend(t, h.Store, k, stepped("again", T0.Add(time.Minute), sessionStep(moveFrom("aux", "aux"))))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, channels.Session{State: "aux", Generation: 2, EnteredAt: channels.NormTime(T0.Add(time.Minute))},
		head.Session)
}

// testSessionReply pins a reply that moves the session: the message, its
// parent's reply count and the notice take three revs in that order, an
// AfterRev page returns all three, and only the causing message carries
// MovedSession, which an edit keeps.
func testSessionReply(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	parent := mustAppend(t, h.Store, k, text("parent"))
	mustAppend(t, h.Store, k, text("other"))
	reply := stepped("reply", T0.Add(time.Minute), sessionStep(fromUnset()))
	reply.ReplyTo = parent.Seq
	msg := mustAppend(t, h.Store, k, reply)
	require.Equal(t, int64(3), msg.Seq)
	require.True(t, msg.MovedSession)

	page, err := h.Store.Messages(ctx, k, channels.Page{AfterRev: msg.Rev - 1})
	require.NoError(t, err)
	revs := map[int64]int64{}
	for _, m := range page {
		revs[m.Seq] = m.Rev
	}
	require.Equal(t, map[int64]int64{3: msg.Rev, 1: msg.Rev + 1, 4: msg.Rev + 2}, revs,
		"message, then parent, then notice")

	all, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Len(t, all, 4)
	require.Equal(t, 1, all[0].ReplyCount)
	for _, m := range all {
		require.Equal(t, m.Seq == 3, m.MovedSession, "seq %d", m.Seq)
	}
	edited, err := h.Store.Edit(ctx, k, 3, map[string]any{"body": "edited"}, nil, T0.Add(2*time.Minute))
	require.NoError(t, err)
	require.True(t, edited.MovedSession, "an edit keeps the mark")
	got, err := h.Store.Get(ctx, k, 3)
	require.NoError(t, err)
	require.True(t, got.MovedSession)
}

// testSessionAdmit pins [channels.SessionStep.Admit]: it sees the channel
// as committed, including the unset session of a channel the append would
// create; a refusal stores nothing (no message, no notice, no addressed
// row, no implicit channel, no audit) and comes back unchanged; and
// SetSession refuses a step that sets it.
func testSessionAdmit(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	carolR := channels.Reader{User: "carol"}
	rows := addressed(t, h.Store, carolR)

	var seen channels.Channel
	refusal := &channels.SessionStateError{State: "contained"}
	step := sessionStep(moveFrom("contained", "assessed"))
	step.Admit = func(c channels.Channel) error {
		seen = c
		return refusal
	}
	nm := stepped("refused", T0.Add(time.Minute), step)
	nm.Mentions = []channels.Mention{login("carol")}
	_, err = h.Store.Append(ctx, k, nm)
	require.ErrorIs(t, err, channels.ErrSessionState)
	got, ok := errors.AsType[*channels.SessionStateError](err)
	require.True(t, ok, "want the *SessionStateError back, got %v", err)
	require.Same(t, refusal, got, "the store returns Admit's error unchanged")
	require.Equal(t, before.Session, seen.Session, "Admit sees the committed session")
	require.Equal(t, before.LastSeq, seen.LastSeq)

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, before, head, "a refused append stored something")
	after, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Equal(t, trail, after)
	require.Equal(t, rows, addressed(t, h.Store, carolR), "a refused append wrote an addressed row")
	page, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Len(t, page, int(before.LastSeq))

	admitted := sessionStep(moveFrom("contained", "assessed"))
	admitted.Admit = func(channels.Channel) error { return nil }
	msg := mustAppend(t, h.Store, k, stepped("admitted", T0.Add(2*time.Minute), admitted))
	require.Equal(t, before.LastSeq+1, msg.Seq)
	require.True(t, admitted.Outcome.Moved, "an admitted append runs the decider")

	testSessionAdmitCreate(t, h)
	testSessionAdmitTimer(t, h, k)
}

// testSessionAdmitCreate pins that Admit sees the unset session of a
// channel the append would create, and that its refusal creates nothing.
func testSessionAdmitCreate(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fresh := key("TKT-3", "room")
	var unset channels.Channel
	create := sessionStep(fromUnset())
	create.Admit = func(c channels.Channel) error {
		unset = c
		return &channels.SessionStateError{State: "detected"}
	}
	_, err := h.Store.Append(ctx, fresh, stepped("first", T0, create))
	require.ErrorIs(t, err, channels.ErrSessionState)
	require.Equal(t, channels.StateOpen, unset.State)
	require.True(t, unset.Session.IsZero(), "Admit sees the unset session of a channel the append creates")
	_, err = h.Store.Head(ctx, fresh)
	require.ErrorIs(t, err, channels.ErrNotFound, "a refused append created the channel")
	freshTrail, err := h.Store.Audit(ctx, "TKT-3")
	require.NoError(t, err)
	require.Empty(t, freshTrail)
}

// testSessionAdmitTimer pins that a timer step with Admit is refused and
// stores nothing.
func testSessionAdmitTimer(t *testing.T, h Harness, k channels.Key) {
	t.Helper()
	ctx := context.Background()
	timer := timerStep(moveFrom("assessed", "overdue"), "late", dueAt(T0))
	timer.Admit = func(channels.Channel) error { return nil }
	headBefore, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	_, err = h.Store.SetSession(ctx, k, T0.Add(3*time.Minute), timer)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	headAfter, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, headBefore, headAfter, "a refused SetSession stored something")
}

// expectGeneration is an Admit that refuses a channel whose session is
// not at generation want, as a post with expect_generation does.
func expectGeneration(want int64) func(channels.Channel) error {
	return func(c channels.Channel) error {
		if c.Session.Generation != want {
			return &channels.SessionGenerationError{Want: want, Got: c.Session.Generation, State: c.Session.State}
		}
		return nil
	}
}

// testSessionAdmitGeneration pins the generation precondition: an Admit
// that returns a [*channels.SessionGenerationError] gets it back unchanged
// with nothing stored, and of concurrent appends that all expect the same
// generation and all move, exactly one is stored.
func testSessionAdmitGeneration(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(1), before.Session.Generation)

	stale := sessionStep(moveFrom("contained", "assessed"))
	stale.Admit = expectGeneration(0)
	_, err = h.Store.Append(ctx, k, stepped("stale", T0.Add(time.Minute), stale))
	require.ErrorIs(t, err, channels.ErrSessionState)
	got, ok := errors.AsType[*channels.SessionGenerationError](err)
	require.True(t, ok, "want the *SessionGenerationError back, got %v", err)
	require.Equal(t, channels.SessionGenerationError{Want: 0, Got: 1, State: "contained"}, *got)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, before, head, "a refused append stored something")

	stores := []channels.Store{h.Store}
	if h.Second != nil {
		stores = append(stores, h.Second(t))
	}
	const n = 8
	var stored atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Go(func() {
			step := sessionStep(moveFrom("contained", "contained"))
			step.Admit = expectGeneration(1)
			_, appendErr := stores[i%len(stores)].Append(ctx, k, stepped("race", T0.Add(2*time.Minute), step))
			if appendErr != nil {
				errs <- appendErr
				return
			}
			stored.Add(1)
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		_, ok := errors.AsType[*channels.SessionGenerationError](err)
		require.True(t, ok, "a losing append is refused by the precondition, got %v", err)
	}
	require.Equal(t, int32(1), stored.Load(), "exactly one append expecting generation 1 is stored")
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(2), head.Session.Generation)
	require.Equal(t, before.LastSeq+2, head.LastSeq, "one message and its notice")
}
