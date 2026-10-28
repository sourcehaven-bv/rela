package channelstest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// readsTests are the read marker tests (TKT-2B2AIA), registered in
// [RunAll].
var readsTests = []conformanceTest{
	{"SetRead", testSetRead},
	{"SetReadConcurrent", testSetReadConcurrent},
	{"SetReadErased", testSetReadErased},
	{"ReadsCount", testReadsCount},
	{"ReadsChunks", testReadsChunks},
	{"ReadsCarried", testReadsCarried},
	{"ListReads", testListReads},
	{"ListReadsWindow", testListReadsWindow},
	{"ListReadsLastAt", testListReadsLastAt},
	{"AppendMarksRead", testAppendMarksRead},
}

// Readers of the read marker tests. Carol and Carol2 are one user in two
// organizations.
var (
	readerAlice = channels.ReaderOf(Alice)
	readerBob   = channels.ReaderOf(Bob)
	carol       = channels.Reader{User: "carol", OrgID: "org-a"}
	carol2      = channels.Reader{User: "carol", OrgID: "org-b"}
)

// setRead marks k read for r up to seq at at and returns the state and
// whether it moved, failing on an error.
func setRead(
	t *testing.T, s channels.Store, k channels.Key, r channels.Reader, seq int64, at time.Time,
) (channels.ReadState, bool) {
	t.Helper()
	st, moved, err := s.SetRead(context.Background(), k, r, seq, at)
	require.NoError(t, err)
	return st, moved
}

// reads returns r's states for keys, failing on an error.
func reads(
	t *testing.T, s channels.Store, r channels.Reader, keys ...channels.Key,
) map[channels.Key]channels.ReadState {
	t.Helper()
	got, err := s.Reads(context.Background(), r, keys)
	require.NoError(t, err)
	return got
}

// fromBob is text(body) by Bob.
func fromBob(body string) channels.NewMessage {
	m := text(body)
	m.Author = Bob
	return m
}

// testSetRead pins [channels.Store.SetRead]: create, raise, lower and
// equal are no-ops with at unchanged, beyond the head clamps, an empty
// named channel takes 0, closed and archived channels accept it, rev and
// audit are unchanged, readers and organizations keep separate markers,
// and the refusals.
func testSetRead(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	for i := range 3 {
		mustAppend(t, s, k, fromBob(fmt.Sprint("m", i)))
	}
	before, err := s.Head(ctx, k)
	require.NoError(t, err)
	trail, err := s.Audit(ctx, "TKT-1")
	require.NoError(t, err)

	st, moved := setRead(t, s, k, readerAlice, 1, T0)
	require.True(t, moved, "a missing marker is created")
	require.Equal(t, channels.ReadState{Seq: 1, At: channels.NormTime(T0), LastSeq: 3, Unread: 2}, st)

	later := T0.Add(time.Hour)
	st, moved = setRead(t, s, k, readerAlice, 1, later)
	require.False(t, moved, "an equal seq does not move the marker")
	require.Equal(t, channels.NormTime(T0), st.At, "at changes only on a move")
	st, moved = setRead(t, s, k, readerAlice, 0, later)
	require.False(t, moved, "a marker never moves back")
	require.Equal(t, int64(1), st.Seq)

	st, moved = setRead(t, s, k, readerAlice, 99, later)
	require.True(t, moved)
	require.Equal(t, channels.ReadState{Seq: 3, At: channels.NormTime(later), LastSeq: 3}, st, "clamped to the head")
	mustAppend(t, s, k, fromBob("after the clamp"))
	require.Equal(t, 1, reads(t, s, readerAlice, k)[k].Unread, "a later message counts")

	after, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, before.Rev+1, after.Rev, "only the append raised the rev")
	trailAfter, err := s.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, trailAfter, len(trail), "a mark writes no audit record")

	setRead(t, s, k, readerBob, 2, T0)
	setRead(t, s, k, carol, 1, T0)
	setRead(t, s, k, carol2, 4, T0)
	got := map[channels.Reader]int64{}
	for _, r := range []channels.Reader{readerAlice, readerBob, carol, carol2} {
		got[r] = reads(t, s, r, k)[k].Seq
	}
	require.Equal(t, map[channels.Reader]int64{readerAlice: 3, readerBob: 2, carol: 1, carol2: 4}, got)

	named, err := s.CreateMany(ctx, channels.Anchor{ID: "TKT-1"}, channels.NewChannel{Type: "topic", By: Bob, At: T0})
	require.NoError(t, err)
	st, moved = setRead(t, s, named.Key, readerAlice, 0, T0)
	require.True(t, moved, "an empty named channel takes a marker at 0")
	require.Equal(t, channels.ReadState{At: channels.NormTime(T0)}, st)

	setState(t, s, k, channels.StateClosed)
	_, _, err = s.SetRead(ctx, k, readerBob, 3, T0)
	require.NoError(t, err, "a closed channel accepts a mark")
	setState(t, s, named.Key, channels.StateClosed, channels.StateArchived)
	_, _, err = s.SetRead(ctx, named.Key, readerBob, 0, T0)
	require.NoError(t, err, "an archived channel accepts a mark")

	_, _, err = s.SetRead(ctx, key("TKT-2", "room"), readerAlice, 0, T0)
	require.ErrorIs(t, err, channels.ErrNotFound)
	for _, c := range []struct {
		r    channels.Reader
		seq  int64
		want error
	}{
		{channels.Reader{}, 0, channels.ErrInvalidMessage},
		{channels.Reader{User: "a\x00b"}, 0, channels.ErrInvalidMessage},
		{readerAlice, -1, channels.ErrInvalidMessage},
	} {
		_, _, err = s.SetRead(ctx, k, c.r, c.seq, T0)
		require.ErrorIs(t, err, c.want)
	}
	_, _, err = s.SetRead(ctx, key("TKT-1", "Bad Type"), readerAlice, 0, T0)
	require.ErrorIs(t, err, channels.ErrInvalidKey)

	tombstone(t, s, "TKT-1", "", T0)
	_, _, err = s.SetRead(ctx, k, readerAlice, 0, T0)
	require.ErrorIs(t, err, channels.ErrNotFound, "a tombstoned channel takes no mark")
}

// testSetReadConcurrent pins that concurrent marks end at their maximum.
func testSetReadConcurrent(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	k := key("TKT-1", "room")
	for i := range 30 {
		mustAppend(t, s, k, fromBob(fmt.Sprint("m", i)))
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var highest int64
	for i := range 20 {
		seq := int64(i*7%30) + 1
		mu.Lock()
		highest = max(highest, seq)
		mu.Unlock()
		wg.Go(func() {
			_, _, err := s.SetRead(context.Background(), k, readerAlice, seq, T0)
			require.NoError(t, err)
		})
	}
	wg.Wait()
	require.Equal(t, highest, reads(t, s, readerAlice, k)[k].Seq)
}

// testSetReadErased pins that Delete and Purge erase markers: a deleted
// channel that a later post recreates starts without markers, and the
// reader's name is gone from storage.
func testSetReadErased(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	gone := channels.Reader{User: "reader-erased-7f3"}
	k := key("TKT-1", "room")
	mustAppend(t, s, k, text("one"))
	setRead(t, s, k, gone, 1, T0)
	head, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, k, channels.Deletion{Expect: head.State, Rev: head.Rev, By: Bob, At: T0}))
	require.NotContains(t, string(h.Raw(t)), gone.User, "a deleted channel's markers are gone")
	mustAppend(t, s, k, text("recreated"))
	require.Empty(t, reads(t, s, gone, k), "a recreated channel starts without markers")

	purged := key("TKT-2", "room")
	mustAppend(t, s, purged, text("one"))
	setRead(t, s, purged, gone, 1, T0)
	b := fileBatch(t, s, "TKT-2", "", T0, channels.Filing{Policy: channels.PolicyDelete})
	require.NoError(t, s.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)))
	require.NotContains(t, string(h.Raw(t)), gone.User, "a purged channel's markers are gone")
}

// testReadsCount pins the counting rule of [channels.Store.Reads]: a
// removed message and a session notice do not count; a timer notice is a
// session notice; a reminder and a reply count; an edit below the marker
// does not count again; the count stops at the cap; and the refusals.
func testReadsCount(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	contained(t, s, k, dueAt(T0.Add(time.Hour))) // seq 1 and its notice at 2
	edited := mustAppend(t, s, k, fromBob("edited below the marker"))
	setRead(t, s, k, readerAlice, edited.Seq, T0)
	_, err := s.Edit(ctx, k, edited.Seq, map[string]any{"body": "changed"}, nil, T0)
	require.NoError(t, err)
	require.Zero(t, reads(t, s, readerAlice, k)[k].Unread, "an edit below the marker does not count")

	mustAppend(t, s, k, fromBob("counts"))             // +1
	removed := mustAppend(t, s, k, fromBob("removed")) // removed: no
	_, err = s.Remove(ctx, k, removed.Seq, channels.Removal{By: Bob, At: T0})
	require.NoError(t, err)
	reply := fromBob("reply")
	reply.ReplyTo = edited.Seq
	mustAppend(t, s, k, reply) // +1
	_, err = s.SetSession(ctx, k, T0.Add(time.Hour),
		timerStep(containedToOverdue, "overdue", dueAt(time.Time{}))) // timer notice: no
	require.NoError(t, err)
	rec := channels.ReminderRecord{Since: T0, Last: T0, Count: 1}
	_, err = s.SetSession(ctx, k, T0.Add(2*time.Hour), remindStep(
		func(channels.Channel) (channels.ReminderPost, bool) { return reminderPost(rec), true }, dueAt(time.Time{})))
	require.NoError(t, err) // reminder: +1
	step := sessionStep(moveFrom("overdue", "resolved"))
	mustAppend(t, s, k, stepped("resolved", T0, step)) // +1, and its notice: no
	head, err := s.Head(ctx, k)
	require.NoError(t, err)
	got := reads(t, s, readerAlice, k)
	require.Equal(t, channels.ReadState{Seq: edited.Seq, At: channels.NormTime(T0), LastSeq: head.LastSeq, Unread: 4},
		got[k])

	many := key("TKT-2", "room")
	mustAppend(t, s, many, fromBob("read"))
	setRead(t, s, many, readerAlice, 1, T0)
	for i := range 150 {
		mustAppend(t, s, many, fromBob(fmt.Sprint("m", i)))
	}
	require.Equal(t, channels.UnreadCap, reads(t, s, readerAlice, many)[many].Unread, "the count stops at the cap")

	got = reads(t, s, readerBob, k, many, key("TKT-3", "room"))
	require.Empty(t, got, "keys without a marker and unknown keys are absent")
	got, err = s.Reads(ctx, readerAlice, nil)
	require.NoError(t, err)
	require.Empty(t, got)

	tooMany := make([]channels.Key, channels.MaxReadKeys+1)
	for i := range tooMany {
		tooMany[i] = key(fmt.Sprintf("TKT-%d", i+1), "room")
	}
	_, err = s.Reads(ctx, readerAlice, tooMany)
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = s.Reads(ctx, readerAlice, []channels.Key{key("TKT-1", "Bad Type")})
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = s.Reads(ctx, channels.Reader{}, []channels.Key{k})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
}

// testReadsChunks pins a Reads call over more keys than one database
// statement names.
func testReadsChunks(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	const n = 1200
	keys := make([]channels.Key, n)
	for i := range keys {
		keys[i] = key(fmt.Sprintf("TKT-%d", i+1), "room")
	}
	marked := map[channels.Key]bool{}
	for _, i := range []int{0, 499, 500, 999, 1000, n - 1} {
		k := keys[i]
		mustAppend(t, s, k, fromBob("one"))
		mustAppend(t, s, k, fromBob("two"))
		setRead(t, s, k, readerAlice, 1, T0)
		marked[k] = true
	}
	got := reads(t, s, readerAlice, keys...)
	require.Len(t, got, len(marked))
	for k := range marked {
		require.Equal(t, 1, got[k].Unread, k.String())
	}
}

// testReadsCarried pins that Rekey, Tombstone and Untombstone carry a
// marker: present under the new id, absent while tombstoned, present
// again after Untombstone.
func testReadsCarried(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	old := key("R-OLD", "room")
	mustAppend(t, s, old, fromBob("one"))
	setRead(t, s, old, readerAlice, 1, T0)
	require.NoError(t, s.Rekey(ctx, "R-OLD", "R-NEW", T0))
	moved := key("R-NEW", "room")
	require.Empty(t, reads(t, s, readerAlice, old))
	require.Equal(t, int64(1), reads(t, s, readerAlice, moved)[moved].Seq, "the marker follows the rename")

	batch := tombstone(t, s, "R-NEW", "", T0)
	require.Empty(t, reads(t, s, readerAlice, moved), "a tombstoned channel is absent")
	_, _, err := s.SetRead(ctx, moved, readerAlice, 1, T0)
	require.ErrorIs(t, err, channels.ErrNotFound)
	require.NoError(t, s.Untombstone(ctx, batch, T0.Add(time.Minute), nil))
	require.Equal(t, int64(1), reads(t, s, readerAlice, moved)[moved].Seq, "Untombstone brings the marker back")
}

// markerKeys returns the channel keys of ms in order.
func markerKeys(ms []channels.ReadMarker) []channels.Key {
	out := make([]channels.Key, len(ms))
	for i, m := range ms {
		out[i] = m.Channel.Key
	}
	return out
}

// listReads returns r's read window, failing on an error.
func listReads(t *testing.T, s channels.Store, r channels.Reader, limit int) []channels.ReadMarker {
	t.Helper()
	got, err := s.ListReads(context.Background(), r, limit)
	require.NoError(t, err)
	return got
}

// testListReads pins [channels.Store.ListReads]: window order (unread
// first, then last activity, then key), the full channel and LastAt, the
// limit, tombstoned and deleted channels and other readers absent, a
// Rekey keeping the marker, and the refusals.
func testListReads(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	post := func(k channels.Key, at time.Time) {
		m := fromBob("hi")
		m.At = at
		mustAppend(t, s, k, m)
	}
	quiet, recent, news, gone, tomb, ren := key("L-1", "room"), key("L-2", "room"), key("L-3", "room"),
		key("L-4", "room"), key("L-5", "room"), key("L-6", "room")
	post(quiet, T0)
	post(recent, T0.Add(2*time.Minute))
	post(news, T0.Add(time.Minute))
	post(gone, T0.Add(3*time.Minute))
	post(tomb, T0.Add(4*time.Minute))
	post(ren, T0.Add(30*time.Second))
	for _, k := range []channels.Key{quiet, recent, news, gone, tomb, ren} {
		setRead(t, s, k, readerAlice, 1, T0)
	}
	setRead(t, s, key("L-1", "room"), readerBob, 1, T0)
	post(news, T0.Add(time.Minute)) // news has an unread message, at its old time
	head, err := s.Head(ctx, gone)
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, gone, channels.Deletion{Expect: head.State, Rev: head.Rev, By: Bob, At: T0}))
	tombstone(t, s, "L-5", "", T0)
	require.NoError(t, s.Rekey(ctx, "L-6", "L-7", T0))
	renamed := key("L-7", "room")

	got := listReads(t, s, readerAlice, channels.MaxReadWindow)
	require.Equal(t, []channels.Key{news, recent, renamed, quiet}, markerKeys(got))
	first := got[0]
	require.Equal(t, int64(1), first.Seq)
	require.Equal(t, channels.NormTime(T0), first.At)
	require.Equal(t, int64(2), first.Channel.LastSeq)
	require.Equal(t, channels.NormTime(T0.Add(time.Minute)), first.LastAt)
	require.Equal(t, channels.StateOpen, first.Channel.State)
	require.Equal(t, Bob, first.Channel.CreatedBy)

	require.Equal(t, []channels.Key{news, recent}, markerKeys(listReads(t, s, readerAlice, 2)))
	require.Equal(t, []channels.Key{quiet}, markerKeys(listReads(t, s, readerBob, 10)))
	require.Empty(t, listReads(t, s, carol, 10))
	for _, c := range []struct {
		r     channels.Reader
		limit int
	}{{readerAlice, 0}, {readerAlice, channels.MaxReadWindow + 1}, {channels.Reader{}, 1}} {
		_, err := s.ListReads(ctx, c.r, c.limit)
		require.ErrorIs(t, err, channels.ErrInvalidMessage)
	}
}

// testListReadsWindow pins that a channel with news is never cut: among
// MaxReadWindow+1 markers, the one with the oldest activity is the only
// one with new messages, and it comes first.
func testListReadsWindow(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	n := channels.MaxReadWindow + 1
	keys := make([]channels.Key, n)
	for i := range keys {
		keys[i] = key(fmt.Sprintf("W-%d", i+1), "room")
		m := fromBob("hi")
		m.At = T0.Add(time.Duration(i) * time.Second)
		mustAppend(t, s, keys[i], m)
		setRead(t, s, keys[i], readerAlice, 1, T0)
	}
	mustAppend(t, s, keys[0], fromBob("news"))
	got := listReads(t, s, readerAlice, channels.MaxReadWindow)
	require.Len(t, got, channels.MaxReadWindow)
	require.Equal(t, keys[0], got[0].Channel.Key)
	require.Equal(t, keys[n-1], got[1].Channel.Key, "then the newest activity")
}

// testListReadsLastAt pins LastAt ([channels.LastActivity]) on every
// backend: a removed last message keeps its time, a closed channel
// with no message takes the close time, and a close after a message
// takes the later of the two.
func testListReadsLastAt(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	removed, closedLater := key("A-1", "room"), key("A-3", "room")

	m := fromBob("first")
	m.At = T0
	mustAppend(t, s, removed, m)
	m = fromBob("last")
	m.At = T0.Add(time.Minute)
	last := mustAppend(t, s, removed, m)
	setRead(t, s, removed, readerAlice, 2, T0)
	_, err := s.Remove(ctx, removed, last.Seq, channels.Removal{By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)

	c, err := s.CreateMany(ctx, channels.Anchor{ID: "A-2"}, channels.NewChannel{Type: "topic", By: Alice, At: T0})
	require.NoError(t, err)
	closedEmpty := c.Key
	setRead(t, s, closedEmpty, readerAlice, 0, T0)
	_, err = s.SetLifecycle(ctx, closedEmpty, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Alice, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)

	m = fromBob("hi")
	m.At = T0
	mustAppend(t, s, closedLater, m)
	setRead(t, s, closedLater, readerAlice, 1, T0)
	_, err = s.SetLifecycle(ctx, closedLater, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Alice, At: T0.Add(3 * time.Minute)})
	require.NoError(t, err)

	got := map[channels.Key]time.Time{}
	for _, mk := range listReads(t, s, readerAlice, channels.MaxReadWindow) {
		got[mk.Channel.Key] = mk.LastAt
	}
	require.Equal(t, map[channels.Key]time.Time{
		removed:     channels.NormTime(T0.Add(time.Minute)),
		closedEmpty: channels.NormTime(T0.Add(2 * time.Minute)),
		closedLater: channels.NormTime(T0.Add(3 * time.Minute)),
	}, got)
}

// testAppendMarksRead pins [channels.NewMessage.MarkRead] and
// [channels.NewChannel.MarkRead]: a post moves the author's marker to the
// last seq the append wrote; a session move takes it to the
// notice; the first append creates channel and marker; an append without
// MarkRead and a reminder touch no marker; a named channel's creator gets
// a marker at 0.
func testAppendMarksRead(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	m := text("first")
	m.MarkRead = true
	mustAppend(t, s, k, m)
	setRead(t, s, k, readerBob, 1, T0)
	got := reads(t, s, readerAlice, k)
	require.Equal(t, channels.ReadState{Seq: 1, At: channels.NormTime(T0), LastSeq: 1}, got[k],
		"the first append creates channel and marker")

	mustAppend(t, s, k, text("second"))
	require.Equal(t, int64(1), reads(t, s, readerAlice, k)[k].Seq, "an append without MarkRead touches no marker")
	setRead(t, s, k, readerAlice, 2, T0)
	mustAppend(t, s, k, fromBob("third"))
	rec := channels.ReminderRecord{Since: T0, Last: T0, Count: 1}
	_, err := s.SetSession(ctx, k, T0, remindStep(
		func(channels.Channel) (channels.ReminderPost, bool) { return reminderPost(rec), true }, dueAt(time.Time{})))
	require.NoError(t, err)
	require.Equal(t, int64(2), reads(t, s, readerAlice, k)[k].Seq, "a reminder touches no marker")

	step := stepped("moves", T0, sessionStep(fromUnset()))
	step.MarkRead = true
	msg := mustAppend(t, s, k, step)
	got = reads(t, s, readerAlice, k)
	require.Equal(t, msg.Seq+1, got[k].Seq, "a session move takes the marker to the notice")
	require.Zero(t, got[k].Unread)
	require.Equal(t, 4, reads(t, s, readerBob, k)[k].Unread, "another reader at the previous head sees the post")

	named, err := s.CreateMany(ctx, channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
		Type: "topic", By: Alice, At: T0, MarkRead: true,
	})
	require.NoError(t, err)
	require.Equal(t, channels.ReadState{At: channels.NormTime(T0)}, reads(t, s, readerAlice, named.Key)[named.Key],
		"the creator's marker is at 0")
	none, err := s.CreateMany(ctx, channels.Anchor{ID: "TKT-1"}, channels.NewChannel{Type: "topic", By: Alice, At: T0})
	require.NoError(t, err)
	require.Empty(t, reads(t, s, readerAlice, none.Key))
}
