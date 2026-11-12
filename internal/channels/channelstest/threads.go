package channelstest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// threadTests pin the thread filters of [channels.Page], [channels.Thread.Inline]
// and [channels.Thread.LastReplyAt] (TKT-ETL233O).
var threadTests = []conformanceTest{
	{"ThreadPages", testThreadPages},
	{"ThreadRemoval", testThreadRemoval},
	{"ThreadPageRefusals", testThreadPageRefusals},
	{"LastReplyAtNeverBack", testLastReplyAtNeverBack},
}

// seedThreads appends, at T0 plus one minute per seq:
//
//	1 ask          top-level
//	2 text         top-level
//	3 text         reply to 1
//	4 answer       reply to 1, answering ask: inline
//	5 answer       reply to 2, whose type it does not answer
//	6 text         top-level
//	7 text         reply to 1
func seedThreads(t *testing.T, s channels.Store, k channels.Key) map[int64]channels.Message {
	t.Helper()
	rows := []struct {
		typ     string
		replyTo int64
	}{{"ask", 0}, {"text", 0}, {"text", 1}, {"answer", 1}, {"answer", 2}, {"text", 0}, {"text", 1}}
	out := map[int64]channels.Message{}
	for i, r := range rows {
		nm := text("m")
		nm.Type, nm.ReplyTo, nm.At = r.typ, r.replyTo, T0.Add(time.Duration(i+1)*time.Minute)
		if r.typ == "answer" {
			nm.InlineUnder = []string{"ask"}
		}
		m := mustAppend(t, s, k, nm)
		out[m.Seq] = m
	}
	return out
}

func page(t *testing.T, s channels.Store, k channels.Key, p channels.Page) []int64 {
	t.Helper()
	got, err := s.Messages(context.Background(), k, p)
	require.NoError(t, err)
	return seqs(got)
}

func testThreadPages(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	sent := seedThreads(t, h.Store, k)
	require.True(t, sent[4].Inline, "an answer to its question's type is inline")
	require.False(t, sent[5].Inline, "an answer under another type is not")
	require.False(t, sent[3].Inline)
	require.False(t, sent[1].Inline, "a top-level message is never inline")

	require.Equal(t, []int64{1, 3, 4, 7}, page(t, h.Store, k, channels.Page{Thread: 1}))
	require.Equal(t, []int64{2, 5}, page(t, h.Store, k, channels.Page{Thread: 2}))
	require.Empty(t, page(t, h.Store, k, channels.Page{Thread: 3}), "a reply names no thread")
	require.Empty(t, page(t, h.Store, k, channels.Page{Thread: 99}))
	require.Equal(t, []int64{3, 4}, page(t, h.Store, k, channels.Page{Thread: 1, AfterSeq: 1, Limit: 2}))
	require.Equal(t, []int64{4, 7}, page(t, h.Store, k, channels.Page{Thread: 1, Limit: 2}),
		"the newest page of a thread")
	require.Equal(t, []int64{1, 3}, page(t, h.Store, k, channels.Page{Thread: 1, BeforeSeq: 4}))

	require.Equal(t, []int64{1, 2, 4, 6}, page(t, h.Store, k, channels.Page{Stream: true}))
	require.Equal(t, []int64{4, 6}, page(t, h.Store, k, channels.Page{Stream: true, Limit: 2}),
		"hidden replies never thin a full page")
	require.Equal(t, []int64{2, 4}, page(t, h.Store, k, channels.Page{Stream: true, BeforeSeq: 6, Limit: 2}))
	require.Equal(t, []int64{4, 6}, page(t, h.Store, k, channels.Page{Stream: true, AfterSeq: 2}))

	parent, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.Equal(t, 3, parent.ReplyCount)
	require.Equal(t, channels.NormTime(T0.Add(7*time.Minute)), parent.LastReplyAt)
	other, err := h.Store.Get(ctx, k, 6)
	require.NoError(t, err)
	require.True(t, other.LastReplyAt.IsZero(), "a message without replies has no reply time")

	// The reply at seq 7 bumped the parent: a rev page of the thread
	// after the reply at seq 4 holds the parent and the later reply.
	changed := page(t, h.Store, k, channels.Page{Thread: 1, AfterRev: sent[4].Rev})
	require.ElementsMatch(t, []int64{1, 7}, changed)
	require.Equal(t, []int64{2, 6, 1}, page(t, h.Store, k, channels.Page{Stream: true, AfterRev: sent[5].Rev}),
		"in rev order: the parent of seq 5, seq 6, the parent of seq 7; no hidden reply")

	got, err := h.Store.GetMessages(ctx, []channels.MessageRef{{Key: k, Seq: 4}, {Key: k, Seq: 1}})
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, m := range got {
		require.Equal(t, m.Seq == 4, m.Inline, "seq %d", m.Seq)
		if m.Seq == 1 {
			require.Equal(t, parent.LastReplyAt, m.LastReplyAt)
		}
	}
}

// testThreadRemoval pins that a removal keeps a message's place in its
// thread and in the stream, and changes neither the count nor the reply
// time of its parent.
func testThreadRemoval(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	seedThreads(t, h.Store, k)
	before, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	for _, seq := range []int64{4, 7, 1} {
		_, rmErr := h.Store.Remove(ctx, k, seq, channels.Removal{By: Alice, At: T0.Add(time.Hour)})
		require.NoError(t, rmErr)
	}
	stub, err := h.Store.Get(ctx, k, 4)
	require.NoError(t, err)
	require.NotNil(t, stub.Removed)
	require.True(t, stub.Inline, "a removed inline reply keeps its place in the stream")
	parent, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.NotNil(t, parent.Removed)
	require.Equal(t, before.ReplyCount, parent.ReplyCount)
	require.Equal(t, before.LastReplyAt, parent.LastReplyAt)

	require.Equal(t, []int64{1, 3, 4, 7}, page(t, h.Store, k, channels.Page{Thread: 1}),
		"the thread of a removed parent stays readable")
	require.Equal(t, []int64{1, 2, 4, 6}, page(t, h.Store, k, channels.Page{Stream: true}))
}

func testThreadPageRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("one"))
	for _, p := range []channels.Page{{Thread: -1}, {Thread: 1, Stream: true}} {
		_, err := h.Store.Messages(ctx, k, p)
		require.ErrorIs(t, err, channels.ErrInvalidPage, "%+v", p)
	}
}

// testLastReplyAtNeverBack pins that a reply with an earlier clock, as
// from another process, does not lower its parent's reply time.
func testLastReplyAtNeverBack(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "room")
	parent := mustAppend(t, h.Store, k, text("p"))
	late := text("late")
	late.ReplyTo, late.At = parent.Seq, T0.Add(time.Hour)
	mustAppend(t, h.Store, k, late)
	early := text("early")
	early.ReplyTo, early.At = parent.Seq, T0.Add(time.Minute)
	mustAppend(t, h.Store, k, early)
	got, err := h.Store.Get(context.Background(), k, parent.Seq)
	require.NoError(t, err)
	require.Equal(t, 2, got.ReplyCount)
	require.Equal(t, channels.NormTime(late.At), got.LastReplyAt)
}
