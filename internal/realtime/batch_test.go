package realtime

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// questionRealtime is a room whose session waits in asking until every
// question of the span has an answer, and a plain chat without a session.
const questionRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
      mod:    { permission: chat.moderate }
    message_types:
      ask:
        properties:
          question: { type: string, required: true }
          options:  { type: string, list: true }
        post: [member]
        choices: { from: options, answer: answer, into: body }
      answer:
        properties: { body: { type: string, required: true } }
        post: [member]
      slow:
        properties: { body: { type: string, required: true } }
        post: [member]
        rate: 3/m
      text:
        properties: { body: { type: markdown, required: true } }
        post: [member]
      ping:
        persist: ephemeral
        post: [member]
    message_ops:
      edit_own:   { roles: [member] }
      delete_own: { roles: [member] }
      remove:     { roles: [mod] }
    session:
      initial: asking
      states:
        asking:   { on: { answer: { to: answered, when: "session.open_questions == 0" } } }
        answered: { on: { text: { to: asking } } }
  chat:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      ask:
        properties:
          question: { type: string, required: true }
          options:  { type: string, list: true }
        post: [member]
        choices: { from: options, answer: answer, into: body }
      answer:
        properties: { body: { type: string, required: true } }
        post: [member]
    message_ops:
      edit_own: { roles: [member] }
`

func ask(q string) BatchMessage {
	return BatchMessage{Type: "ask", Props: map[string]any{"question": q, "options": []any{"yes", "no"}}}
}

func (h *harness) askBatch(t *testing.T, a *fakeAccess, addr RawAddress, msgs ...BatchMessage) BatchResult {
	t.Helper()
	res, err := h.svc.PostBatch(as(a), addr, BatchInput{Messages: msgs})
	require.NoError(t, err)
	return res
}

func (h *harness) answer(t *testing.T, a *fakeAccess, q int64) PostResult {
	t.Helper()
	res, err := h.svc.Post(as(a), room, PostInput{Type: "answer", ReplyTo: q, Props: map[string]any{"body": "yes"}})
	require.NoError(t, err)
	return res
}

// TestPostBatch_QuestionsMoveOnLastAnswer pins the batch path end to end:
// the questions share a batch id and join the asked set, the head shows
// them, and the session moves on the answer that leaves none open.
func TestPostBatch_QuestionsMoveOnLastAnswer(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(questionRealtime))
	bob := user("bob")
	res := h.askBatch(t, bob, room, ask("Store?"), ask("Region?"))
	require.Len(t, res.Messages, 2)
	q1, q2 := res.Messages[0].Seq, res.Messages[1].Seq
	require.Equal(t, []int64{q1, q1}, []int64{res.Messages[0].Batch, res.Messages[1].Batch})
	require.Equal(t, "asking", res.Session.State)

	head, err := h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Equal(t, []AskedView{{Seq: q1, AnswerType: "answer"}, {Seq: q2, AnswerType: "answer"}},
		head.Channel.Asked)

	first := h.answer(t, bob, q1)
	require.Equal(t, "asking", first.Session.State, "one question is still open")
	last := h.answer(t, bob, q2)
	require.Equal(t, "answered", last.Session.State, "the last answer moves the session")

	head, err = h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Nil(t, head.Channel.Asked, "the move ended the span")
}

// TestPostBatch_Refusals pins what a batch refuses before it stores
// anything, and that a denial names the message it is about.
func TestPostBatch_Refusals(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(questionRealtime))
	bob := user("bob")
	tooMany := make([]BatchMessage, channels.MaxBatch+1)
	for i := range tooMany {
		tooMany[i] = ask("q")
	}
	for _, tc := range []struct {
		name  string
		msgs  []BatchMessage
		title string
	}{
		{"empty", nil, "a batch holds 1 to 10 messages, not 0"},
		{"over the cap", tooMany, "a batch holds 1 to 10 messages, not 11"},
		{"a signal", []BatchMessage{ask("q"), {Type: "ping"}}, "message 1: a batch posts log messages; ping is a signal"},
		{"unknown type", []BatchMessage{{Type: "nope"}}, `message 0: channel type room has no message type "nope"`},
		{"invalid props", []BatchMessage{ask("q"), {Type: "ask", Props: map[string]any{}}}, "message 1: "},
	} {
		_, err := h.svc.PostBatch(as(bob), room, BatchInput{Messages: tc.msgs})
		d := requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
		require.Contains(t, d.Title, tc.title, tc.name)
	}
	head, err := h.store.Head(context.Background(), roomKey())
	if err == nil {
		require.Zero(t, head.LastSeq, "a refused batch stores nothing")
	}
}

// TestPostBatch_ChargesOnlyWhenAccepted pins that a batch is charged one
// token per message in one step, after every message passed its checks: a
// refused batch spends nothing, and a batch over a type's rate is a 400
// rather than a 429 no wait could meet.
func TestPostBatch_ChargesOnlyWhenAccepted(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(questionRealtime))
	bob := user("bob")
	slow := func(body string) BatchMessage { return BatchMessage{Type: "slow", Props: map[string]any{"body": body}} }

	_, err := h.svc.PostBatch(as(bob), room, BatchInput{Messages: []BatchMessage{slow("a"), slow("b"), slow("c"), slow("d")}})
	d := requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
	require.Contains(t, d.Title, "at most 3 messages of type slow")

	for range 3 {
		_, err = h.svc.PostBatch(as(bob), room, BatchInput{Messages: []BatchMessage{slow("a"), {Type: "slow"}}})
		requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
	}
	res := h.askBatch(t, bob, room, slow("a"), slow("b"), slow("c"))
	require.Len(t, res.Messages, 3, "the refused batches spent no token")
	_, err = h.svc.PostBatch(as(bob), room, BatchInput{Messages: []BatchMessage{slow("d")}})
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
}

// TestPostBatch_AskedLock pins that the author can neither edit nor delete
// a question while it is asked, that a moderator may still remove it, and
// that the lock lifts when the session moves.
func TestPostBatch_AskedLock(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(questionRealtime))
	bob, mod := user("bob"), user("carol", "chat.moderate")
	res := h.askBatch(t, bob, room, ask("Store?"), ask("Region?"))
	q1, q2 := res.Messages[0].Seq, res.Messages[1].Seq

	_, err := h.svc.Edit(as(bob), room, q1, map[string]any{"question": "Changed?", "options": []any{"a"}})
	requireDenial(t, err, http.StatusConflict, TypeAskedQuestion)
	_, err = h.svc.Delete(as(bob), room, q1, "")
	requireDenial(t, err, http.StatusConflict, TypeAskedQuestion)

	_, err = h.svc.Delete(as(mod), room, q2, "off topic")
	require.NoError(t, err, "a moderator may remove an asked question")
	head, err := h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Equal(t, []AskedView{{Seq: q1, AnswerType: "answer"}}, head.Channel.Asked)

	require.Equal(t, "answered", h.answer(t, bob, q1).Session.State)
	_, err = h.svc.Edit(as(bob), room, q1, map[string]any{"question": "Changed?", "options": []any{"a"}})
	require.NoError(t, err, "the lock lifts when the session moves")
}

// TestPostBatch_RemovingLastOpenQuestionDoesNotMove pins that a removal
// runs no transition, even when it leaves no question open: a removal is
// not a message, so no `on:` applies. The next answer moves the session.
func TestPostBatch_RemovingLastOpenQuestionDoesNotMove(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(questionRealtime))
	bob, mod := user("bob"), user("carol", "chat.moderate")
	res := h.askBatch(t, bob, room, ask("Store?"), ask("Region?"))
	q1, q2 := res.Messages[0].Seq, res.Messages[1].Seq
	require.Equal(t, "asking", h.answer(t, bob, q1).Session.State)

	_, err := h.svc.Delete(as(mod), room, q2, "withdrawn")
	require.NoError(t, err)
	head, err := h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Equal(t, "asking", head.Channel.Session.State, "a removal moves nothing")
	require.Equal(t, []AskedView{{Seq: q1, AnswerType: "answer", Answers: 1}}, head.Channel.Asked)

	require.Equal(t, "answered", h.answer(t, bob, q1).Session.State, "the next answer moves it")
}

// TestPostBatch_NoSessionAsksNothing pins that a channel type without a
// session keeps no asked set: no move would ever end the span.
func TestPostBatch_NoSessionAsksNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(questionRealtime))
	bob := user("bob")
	chat := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "chat"}
	res := h.askBatch(t, bob, chat, ask("Store?"), ask("Region?"))
	require.Equal(t, res.Messages[0].Seq, res.Messages[1].Batch)
	head, err := h.svc.Head(as(bob), chat)
	require.NoError(t, err)
	require.Nil(t, head.Channel.Asked)
	_, err = h.svc.Edit(as(bob), chat, res.Messages[0].Seq, map[string]any{"question": "Changed?"})
	require.NoError(t, err)
}

// TestPostBatch_PostDenied pins that the batch path runs the post gate per
// message: a type the caller may not post refuses the whole batch with 403
// post_denied, names the message and the rule, and stores nothing.
func TestPostBatch_PostDenied(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	body := map[string]any{"body": "hi"}
	_, err := h.svc.PostBatch(as(bob), room, BatchInput{Messages: []BatchMessage{
		{Type: "text", Props: body}, {Type: "perm-text", Props: body},
	}})
	d := requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	require.Contains(t, d.Title, "message 1: ")
	require.Contains(t, d.Rule, ".message_types.perm-text.post")
	require.Zero(t, h.store.appends.Load(), "a denied batch must not reach the store")

	_, err = h.svc.PostBatch(as(readOnlyClient(user("bob", "chat.post"))), room,
		BatchInput{Messages: []BatchMessage{{Type: "perm-text", Props: body}}})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
}
