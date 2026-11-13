package realtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRead_Threads pins TKT-ETL233O at the service: an answer to its
// question's type is inline, any other reply is not; a thread page holds
// the parent and its replies; a stream page holds top-level messages and
// inline replies; the parent names its reply time; and a thread page of an
// unreadable anchor gets the same answer as any read of it.
func TestRead_Threads(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(listRealtime))
	bob, carol := user("bob"), user("carol")
	q := h.post(t, bob, PostInput{Type: "ask", Props: map[string]any{"question": "Which?", "options": []any{"A"}}})
	answer := h.post(t, carol, PostInput{Type: "answer", Props: map[string]any{"body": "A"}, ReplyTo: q.Seq})
	other := h.post(t, carol, PostInput{Type: "answer", Props: map[string]any{"body": "x"}})
	aside := h.post(t, carol, PostInput{Type: "answer", Props: map[string]any{"body": "y"}, ReplyTo: other.Seq})
	require.True(t, answer.Inline, "an answer to its question is inline")
	require.False(t, aside.Inline, "a reply to a type the answer does not answer is not")

	seqs := func(req ReadRequest) []int64 {
		t.Helper()
		v, err := h.svc.Read(as(bob), room, req)
		require.NoError(t, err)
		var out []int64
		for _, m := range v.Messages {
			out = append(out, m.Seq)
		}
		return out
	}
	require.Equal(t, []int64{q.Seq, answer.Seq}, seqs(ReadRequest{Thread: q.Seq}))
	require.Equal(t, []int64{other.Seq, aside.Seq}, seqs(ReadRequest{Thread: other.Seq}))
	require.Equal(t, []int64{q.Seq, answer.Seq, other.Seq}, seqs(ReadRequest{Stream: true}))
	require.Empty(t, seqs(ReadRequest{Thread: aside.Seq}), "a reply names no thread")

	v, err := h.svc.Read(as(bob), room, ReadRequest{Thread: other.Seq})
	require.NoError(t, err)
	require.Equal(t, aside.At, v.Messages[0].LastReplyAt)

	_, err = h.svc.Read(as(bob), room, ReadRequest{Thread: 1, Stream: true})
	require.Error(t, err)

	hidden := RawAddress{EntityType: "ticket", Anchor: "TKT-9", Type: "room"}
	_, plain := h.svc.Read(as(bob), hidden, ReadRequest{})
	_, thread := h.svc.Read(as(bob), hidden, ReadRequest{Thread: 1})
	require.ErrorIs(t, plain, ErrAnchorNotFound)
	require.Equal(t, plain.Error(), thread.Error())
}

// TestInlineUnder pins that a type is inline under every type whose
// choices answer with it, in name order.
func TestInlineUnder(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(listRealtime))
	ct, ok := h.svc.Config().ChannelType("room")
	require.True(t, ok)
	require.Equal(t, []string{"ask"}, inlineUnder(&ct, "answer"))
	require.Empty(t, inlineUnder(&ct, "ask"))
}
