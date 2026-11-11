package channelstest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// batchTests are the [channels.Store.AppendBatch] and [channels.Asked]
// tests of [RunAll].
var batchTests = []conformanceTest{
	{"BatchAppend", testBatchAppend},
	{"BatchMovesOnce", testBatchMovesOnce},
	{"BatchRefusals", testBatchRefusals},
	{"AskedAnswers", testAskedAnswers},
	{"AskedRemove", testAskedRemove},
	{"AskedCap", testAskedCap},
	{"AskedTimerMove", testAskedTimerMove},
}

// question is text(body) asking for replies of type answer.
func question(body string) channels.NewMessage {
	m := text(body)
	m.Asks = "answer"
	return m
}

// answer is a reply of type answer to seq, with step.
func answer(seq int64, step *channels.SessionStep) channels.NewMessage {
	return channels.NewMessage{
		Type: "answer", Author: Bob, At: T0.Add(time.Minute), Props: map[string]any{"body": "yes"},
		ReplyTo: seq, Session: step,
	}
}

func mustBatch(t *testing.T, s channels.Store, k channels.Key, batch ...channels.NewMessage) []channels.Message {
	t.Helper()
	got, err := s.AppendBatch(context.Background(), k, batch)
	require.NoError(t, err)
	return got
}

func mustHead(t *testing.T, s channels.Store, k channels.Key) channels.Channel {
	t.Helper()
	head, err := s.Head(context.Background(), k)
	require.NoError(t, err)
	return head
}

// testBatchAppend pins a batch without a session: ascending seqs, the
// batch id on every message, the implicit create on the first only, one
// read marker move, and the stored messages as returned.
func testBatchAppend(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "room")
	a, b, c := text("a"), text("b"), text("c")
	c.MarkRead = true
	got := mustBatch(t, h.Store, k, a, b, c)
	require.Len(t, got, 3)
	for i, m := range got {
		require.Equal(t, int64(i+1), m.Seq)
		require.Equal(t, int64(1), m.Batch)
		require.Equal(t, i == 0, m.CreatedChannel)
	}
	head := mustHead(t, h.Store, k)
	require.Equal(t, int64(3), head.LastSeq)
	require.Equal(t, got[2].Rev, head.Rev)
	page, err := h.Store.Messages(context.Background(), k, channels.Page{})
	require.NoError(t, err)
	require.Len(t, page, 3)
	for i, m := range page {
		require.Equal(t, int64(1), m.Batch, "seq %d", m.Seq)
		require.Equal(t, got[i].Props, m.Props)
	}
	one := mustAppend(t, h.Store, k, text("d"))
	require.Zero(t, one.Batch, "a batch of one carries no batch id")
	require.Equal(t, int64(3), reads(t, h.Store, readerAlice, k)[k].Seq,
		"the batch's MarkRead moved the marker to its last seq")
}

// testBatchMovesOnce pins the session logic of a batch: Admit sees the
// channel before the batch, the decider runs in order until one moves,
// the notice follows the moving message, and only its step reports the
// move.
func testBatchMovesOnce(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	var admitted, decided []int64
	steps := make([]*channels.SessionStep, 3)
	batch := make([]channels.NewMessage, 3)
	for i := range batch {
		steps[i] = sessionStep(func(c channels.Channel) (channels.SessionMove, bool) {
			decided = append(decided, c.LastSeq)
			return channels.SessionMove{From: "detected", To: "contained"}, c.LastSeq == 2
		})
		steps[i].Admit = func(c channels.Channel) error {
			admitted = append(admitted, c.LastSeq)
			return nil
		}
		batch[i] = stepped("m", T0.Add(time.Minute), steps[i])
	}
	got := mustBatch(t, h.Store, k, batch...)
	require.Equal(t, []int64{1, 1, 1}, admitted, "every Admit sees the channel before the batch")
	require.Equal(t, []int64{1, 2}, decided, "the decider runs until the first move")
	require.Equal(t, []int64{2, 3, 5}, []int64{got[0].Seq, got[1].Seq, got[2].Seq}, "the notice takes seq 4")
	require.Equal(t, []bool{false, true, false},
		[]bool{got[0].MovedSession, got[1].MovedSession, got[2].MovedSession})
	head := mustHead(t, h.Store, k)
	require.Equal(t, int64(5), head.LastSeq)
	require.Equal(t, int64(1), head.Session.Generation)
	require.Equal(t, int64(3), head.Session.Cause)
	require.False(t, steps[0].Outcome.Moved)
	require.True(t, steps[1].Outcome.Moved)
	require.Equal(t, int64(4), steps[1].Outcome.Notice)
	require.Equal(t, head.Session, steps[2].Outcome.Session)
	notice, err := h.Store.Get(context.Background(), k, 4)
	require.NoError(t, err)
	require.Equal(t, "session", notice.Type)
	require.Zero(t, notice.Batch, "a notice belongs to no batch")
	recs := sessionRecords(t, h.Store)
	require.Len(t, recs, 1)
	require.Equal(t, int64(3), recs[0].Seq)
}

// testBatchRefusals pins what a batch refuses, and that a refusal stores
// nothing.
func testBatchRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	parent := mustAppend(t, h.Store, k, text("parent"))
	reply := text("reply")
	reply.ReplyTo = parent.Seq
	other := text("other")
	other.Author = Bob
	askReply := question("q")
	askReply.ReplyTo = parent.Seq
	withFile := text("file")
	withFile.Files = []channels.NewFile{{Property: "image", Name: "a.txt", MIME: "text/plain", Data: []byte("a")}}
	refusing := stepped("refused", T0, sessionStep(fromUnset()))
	refusing.Session.Admit = func(channels.Channel) error { return channels.ErrSessionState }
	tooMany := make([]channels.NewMessage, channels.MaxBatch+1)
	for i := range tooMany {
		tooMany[i] = text("x")
	}
	for _, tc := range []struct {
		name  string
		batch []channels.NewMessage
		want  error
	}{
		{"empty", nil, channels.ErrInvalidMessage},
		{"over the cap", tooMany, channels.ErrTooLarge},
		{"a reply among several", []channels.NewMessage{text("a"), reply}, channels.ErrInvalidReply},
		{"two authors", []channels.NewMessage{text("a"), other}, channels.ErrInvalidMessage},
		{"files among several", []channels.NewMessage{text("a"), withFile}, channels.ErrInvalidMessage},
		{"a reply that asks", []channels.NewMessage{askReply}, channels.ErrInvalidMessage},
		{"an Admit refusal", []channels.NewMessage{text("a"), refusing}, channels.ErrSessionState},
	} {
		_, err := h.Store.AppendBatch(ctx, k, tc.batch)
		require.ErrorIs(t, err, tc.want, tc.name)
	}
	head := mustHead(t, h.Store, k)
	require.Equal(t, int64(1), head.LastSeq, "a refused batch stores nothing")
}

// testAskedAnswers pins the asked set: a batch's questions join it, a
// reply of the answer type counts and one of another type does not, the
// decider sees the count after the reply, and the move clears the set.
func testAskedAnswers(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "room")
	qs := mustBatch(t, h.Store, k, question("q1"), question("q2"), text("note"))
	head := mustHead(t, h.Store, k)
	require.Equal(t, map[int64]channels.Asked{
		qs[0].Seq: {AnswerType: "answer"}, qs[1].Seq: {AnswerType: "answer"},
	}, head.Asked)
	require.Equal(t, 2, head.OpenQuestions())

	whenAnswered := func(c channels.Channel) (channels.SessionMove, bool) {
		return channels.SessionMove{From: "asking", To: "answered"}, c.OpenQuestions() == 0
	}
	wrongType := answer(qs[0].Seq, nil)
	wrongType.Type = "text"
	mustAppend(t, h.Store, k, wrongType)
	require.Equal(t, 2, mustHead(t, h.Store, k).OpenQuestions(), "a reply of another type answers nothing")

	first := mustAppend(t, h.Store, k, answer(qs[0].Seq, sessionStep(whenAnswered)))
	require.False(t, first.MovedSession)
	head = mustHead(t, h.Store, k)
	require.Equal(t, 1, head.Asked[qs[0].Seq].Answers)
	require.Equal(t, 1, head.OpenQuestions())

	last := mustAppend(t, h.Store, k, answer(qs[1].Seq, sessionStep(whenAnswered)))
	require.True(t, last.MovedSession, "the decider saw the last question answered")
	require.Nil(t, mustHead(t, h.Store, k).Asked, "the move cleared the set")

	again := mustBatch(t, h.Store, k, question("q3"), stepped("move", T0, sessionStep(moveFrom("answered", "asking"))))
	require.True(t, again[1].MovedSession)
	require.Equal(t, map[int64]channels.Asked{again[0].Seq: {AnswerType: "answer"}}, mustHead(t, h.Store, k).Asked,
		"a question joins the span its batch moved into")
}

// testAskedRemove pins removal: a removed answer is uncounted, and a
// removed question leaves the set.
func testAskedRemove(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	qs := mustBatch(t, h.Store, k, question("q1"), question("q2"))
	a := mustAppend(t, h.Store, k, answer(qs[0].Seq, nil))
	require.Equal(t, 1, mustHead(t, h.Store, k).OpenQuestions())
	_, err := h.Store.Remove(ctx, k, a.Seq, channels.Removal{By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, 2, mustHead(t, h.Store, k).OpenQuestions(), "a removed answer is uncounted")
	_, err = h.Store.Remove(ctx, k, qs[1].Seq, channels.Removal{By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, map[int64]channels.Asked{qs[0].Seq: {AnswerType: "answer"}}, mustHead(t, h.Store, k).Asked,
		"a removed question leaves the set")
}

// testAskedCap pins [channels.MaxAsked] across batches of one span: a
// question over the cap evicts the oldest answered one, and is refused when
// every question is unanswered.
func testAskedCap(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "room")
	full := make([]channels.NewMessage, channels.MaxBatch)
	for i := range full {
		full[i] = question("q")
	}
	for range channels.MaxAsked / channels.MaxBatch {
		mustBatch(t, h.Store, k, full...)
	}
	_, err := h.Store.Append(context.Background(), k, question("one too many"))
	require.ErrorIs(t, err, channels.ErrTooManyAsked)
	head := mustHead(t, h.Store, k)
	require.Len(t, head.Asked, channels.MaxAsked)
	require.Equal(t, int64(channels.MaxAsked), head.LastSeq)

	mustAppend(t, h.Store, k, answer(3, nil))
	mustAppend(t, h.Store, k, answer(2, nil))
	q := mustAppend(t, h.Store, k, question("evicts the oldest answered"))
	head = mustHead(t, h.Store, k)
	require.Len(t, head.Asked, channels.MaxAsked)
	require.NotContains(t, head.Asked, int64(2), "the oldest answered question left")
	require.Contains(t, head.Asked, int64(3))
	require.Contains(t, head.Asked, q.Seq)
}

// testAskedTimerMove pins that a timer move clears the asked set too.
func testAskedTimerMove(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "room")
	contained(t, h.Store, k, noDue)
	mustAppend(t, h.Store, k, question("q"))
	require.Len(t, mustHead(t, h.Store, k).Asked, 1)
	_, err := h.Store.SetSession(context.Background(), k, T0.Add(time.Hour), timerStep(containedToOverdue, "late", noDue))
	require.NoError(t, err)
	require.Nil(t, mustHead(t, h.Store, k).Asked)
}
