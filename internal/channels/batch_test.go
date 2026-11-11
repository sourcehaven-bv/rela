package channels_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

func TestCheckAsked(t *testing.T) {
	t.Parallel()
	over := map[int64]channels.Asked{}
	for i := range channels.MaxAsked + 1 {
		over[int64(i+1)] = channels.Asked{AnswerType: "answer"}
	}
	for _, tc := range []struct {
		name string
		in   map[int64]channels.Asked
		ok   bool
	}{
		{"none", nil, true},
		{"one", map[int64]channels.Asked{1: {AnswerType: "answer", Answers: 2}}, true},
		{"over the cap stays readable", over, true},
		{"seq zero", map[int64]channels.Asked{0: {AnswerType: "answer"}}, false},
		{"bad type", map[int64]channels.Asked{1: {AnswerType: "a b"}}, false},
		{"negative", map[int64]channels.Asked{1: {AnswerType: "answer", Answers: -1}}, false},
	} {
		err := channels.CheckAsked(tc.in)
		if tc.ok {
			require.NoError(t, err, tc.name)
		} else {
			require.ErrorIs(t, err, channels.ErrInvalidMessage, tc.name)
		}
	}
}

func TestUnaskRemoved(t *testing.T) {
	t.Parallel()
	a := map[int64]channels.Asked{1: {AnswerType: "answer", Answers: 1}, 2: {AnswerType: "answer"}}
	for _, tc := range []struct {
		name    string
		m       channels.Message
		want    map[int64]channels.Asked
		changed bool
	}{
		{"a question", channels.Message{Seq: 2}, map[int64]channels.Asked{1: {AnswerType: "answer", Answers: 1}}, true},
		{"an answer", channels.Message{Seq: 5, ReplyTo: 1, Type: "answer"},
			map[int64]channels.Asked{1: {AnswerType: "answer"}, 2: {AnswerType: "answer"}}, true},
		{"another type", channels.Message{Seq: 5, ReplyTo: 1, Type: "text"}, a, false},
		{"an uncounted question", channels.Message{Seq: 5, ReplyTo: 2, Type: "answer"}, a, false},
		{"top level", channels.Message{Seq: 5}, a, false},
	} {
		got, changed := channels.UnaskRemoved(a, tc.m)
		require.Equal(t, tc.changed, changed, tc.name)
		require.Equal(t, tc.want, got, tc.name)
	}
	require.Equal(t, 1, a[1].Answers, "the input is never modified")
	got, _ := channels.UnaskRemoved(map[int64]channels.Asked{3: {AnswerType: "answer"}}, channels.Message{Seq: 3})
	require.Nil(t, got, "an empty set is nil")
}

func TestOpenQuestions(t *testing.T) {
	t.Parallel()
	c := channels.Channel{Asked: map[int64]channels.Asked{
		1: {AnswerType: "answer", Answers: 1}, 2: {AnswerType: "answer"}, 3: {AnswerType: "answer"},
	}}
	require.Equal(t, 2, c.OpenQuestions())
	require.Zero(t, channels.Channel{}.OpenQuestions())
}

var batchAt = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

func batchMsg(typ string) channels.NewMessage {
	return channels.NewMessage{Type: typ, Author: channels.Author{User: "bob"}, At: batchAt,
		Props: map[string]any{"body": "x"}}
}

func batchQuestion() channels.NewMessage {
	m := batchMsg("ask")
	m.Asks = "answer"
	return m
}

func TestPrepareBatch(t *testing.T) {
	t.Parallel()
	tooMany := make([]channels.NewMessage, channels.MaxBatch+1)
	for i := range tooMany {
		tooMany[i] = batchMsg("text")
	}
	reply := batchMsg("text")
	reply.ReplyTo = 1
	other := batchMsg("text")
	other.Author = channels.Author{User: "carol"}
	badAsk := batchMsg("text")
	badAsk.Asks = "a b"
	withFile := batchMsg("text")
	withFile.Files = []channels.NewFile{{Property: "image", Name: "a.txt", MIME: "text/plain", Data: []byte("a")}}
	replyAsk := batchQuestion()
	replyAsk.ReplyTo = 1
	for _, tc := range []struct {
		name  string
		batch []channels.NewMessage
		want  error
	}{
		{"empty", nil, channels.ErrInvalidMessage},
		{"over the cap", tooMany, channels.ErrTooLarge},
		{"a reply among several", []channels.NewMessage{batchMsg("text"), reply}, channels.ErrInvalidReply},
		{"files among several", []channels.NewMessage{batchMsg("text"), withFile}, channels.ErrInvalidMessage},
		{"two authors", []channels.NewMessage{batchMsg("text"), other}, channels.ErrInvalidMessage},
		{"an invalid answer type", []channels.NewMessage{badAsk}, channels.ErrInvalidMessage},
		{"a reply that asks", []channels.NewMessage{replyAsk}, channels.ErrInvalidMessage},
		{"an invalid message", []channels.NewMessage{{Type: "text"}}, channels.ErrInvalidMessage},
	} {
		_, err := channels.PrepareBatch(tc.batch)
		require.ErrorIs(t, err, tc.want, tc.name)
	}
	got, err := channels.PrepareBatch([]channels.NewMessage{batchQuestion(), batchMsg("text")})
	require.NoError(t, err)
	require.Len(t, got, 2)
	one, err := channels.PrepareBatch([]channels.NewMessage{reply})
	require.NoError(t, err, "a batch of one may be a reply")
	require.Equal(t, int64(1), one[0].ReplyTo)
}

func moveWhen(ok func(channels.Channel) bool, outcome *channels.SessionOutcome) *channels.SessionStep {
	return &channels.SessionStep{
		Decide: func(c channels.Channel) (channels.SessionMove, bool) {
			return channels.SessionMove{From: "asking", To: "answered"}, ok(c)
		},
		Outcome: outcome,
	}
}

func TestPlanBatch(t *testing.T) {
	t.Parallel()
	cur := channels.Channel{LastSeq: 4, Asked: map[int64]channels.Asked{2: {AnswerType: "answer"}}}

	answer := batchMsg("answer")
	answer.ReplyTo = 2
	var seen []int
	answer.Session = moveWhen(func(c channels.Channel) bool {
		seen = append(seen, c.OpenQuestions())
		return c.OpenQuestions() == 0
	}, nil)
	after := batchMsg("text")
	after.Session = moveWhen(func(channels.Channel) bool { t.Fatal("decided after the move"); return false }, nil)
	p, err := channels.PlanBatch(cur, []channels.NewMessage{answer, batchQuestion(), after})
	require.NoError(t, err)
	require.Equal(t, []int{0}, seen, "the decider sees the answer counted")
	require.Equal(t, 0, p.MoveAt)
	require.True(t, p.Moved(0))
	require.Equal(t, []int64{5, 7, 8}, p.Seqs, "the notice takes seq 6")
	require.Equal(t, int64(5), p.Batch())
	require.Equal(t, map[int64]channels.Asked{7: {AnswerType: "answer"}}, p.Asked,
		"the move cleared the set; the batch's question joins the new span")
	require.Same(t, answer.Session, p.Due([]channels.NewMessage{answer, batchQuestion(), after}))

	quiet, err := channels.PlanBatch(channels.Channel{}, []channels.NewMessage{batchMsg("text")})
	require.NoError(t, err)
	require.Equal(t, -1, quiet.MoveAt)
	require.Zero(t, quiet.Batch(), "a batch of one has no batch id")
	require.Nil(t, quiet.Asked)
	require.Nil(t, quiet.Due([]channels.NewMessage{batchMsg("text")}))

	stepped := batchMsg("text")
	stepped.Session = moveWhen(func(channels.Channel) bool { return false }, nil)
	noMove, err := channels.PlanBatch(channels.Channel{}, []channels.NewMessage{stepped, batchMsg("text")})
	require.NoError(t, err)
	require.Same(t, stepped.Session, noMove.Due([]channels.NewMessage{stepped, batchMsg("text")}),
		"without a move, the last step is due")
}

func TestPlanBatch_Refusals(t *testing.T) {
	t.Parallel()
	refused := errors.New("refused")
	admit := batchMsg("text")
	admit.Session = &channels.SessionStep{Admit: func(channels.Channel) error { return refused }}
	_, err := channels.PlanBatch(channels.Channel{}, []channels.NewMessage{batchMsg("text"), admit})
	require.ErrorIs(t, err, refused)

	bad := batchMsg("text")
	bad.Session = &channels.SessionStep{Decide: func(channels.Channel) (channels.SessionMove, bool) {
		return channels.SessionMove{From: "a b", To: "c"}, true
	}}
	_, err = channels.PlanBatch(channels.Channel{}, []channels.NewMessage{bad})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)

	full := channels.Channel{LastSeq: channels.MaxAsked, Asked: map[int64]channels.Asked{}}
	for i := range channels.MaxAsked {
		full.Asked[int64(i+1)] = channels.Asked{AnswerType: "answer"}
	}
	_, err = channels.PlanBatch(full, []channels.NewMessage{batchQuestion()})
	require.ErrorIs(t, err, channels.ErrTooManyAsked)

	full.Asked[3] = channels.Asked{AnswerType: "answer", Answers: 1}
	full.Asked[2] = channels.Asked{AnswerType: "answer", Answers: 1}
	p, err := channels.PlanBatch(full, []channels.NewMessage{batchQuestion()})
	require.NoError(t, err)
	require.Len(t, p.Asked, channels.MaxAsked)
	require.NotContains(t, p.Asked, int64(2), "the oldest answered question leaves")
	require.Contains(t, p.Asked, int64(3))
}

func TestFillBatchOutcome(t *testing.T) {
	t.Parallel()
	var first, second channels.SessionOutcome
	a, b, c := batchMsg("text"), batchMsg("text"), batchMsg("text")
	a.Session = moveWhen(func(channels.Channel) bool { return false }, &first)
	b.Session = moveWhen(func(channels.Channel) bool { return true }, &second)
	c.Session = &channels.SessionStep{}
	batch := []channels.NewMessage{a, b, c}
	p, err := channels.PlanBatch(channels.Channel{}, batch)
	require.NoError(t, err)
	committed := channels.Channel{Session: channels.Session{State: "answered", Generation: 1}, CreatedAt: batchAt}
	channels.FillBatchOutcome(batch, p, committed)
	require.Equal(t, channels.SessionOutcome{Session: committed.Session, CreatedAt: batchAt}, first)
	require.True(t, second.Moved)
	require.Equal(t, p.Seqs[1]+1, second.Notice)
	require.Equal(t, "answered", second.Move.To)
}
