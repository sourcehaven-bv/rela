package channelstest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// addressedTests are the addressed row tests (TKT-6RXAC0), registered in
// [RunAll].
var addressedTests = []conformanceTest{
	{"AppendAddressed", testAppendAddressed},
	{"AppendAddressedRefused", testAppendAddressedRefused},
	{"EditRemoveAddressed", testEditRemoveAddressed},
	{"ListAddressed", testListAddressed},
	{"ListAddressedScan", testListAddressedScan},
	{"ListAddressedKind", testListAddressedKind},
	{"ListAddressedKindScan", testListAddressedKindScan},
	{"AddressedTargetCap", testAddressedTargetCap},
	{"AddressedErased", testAddressedErased},
}

// login is a login mention of target.
func login(target string) channels.Mention {
	return channels.Mention{Target: target, Form: channels.FormLogin}
}

// mentioning is text(body) by author mentioning targets as logins.
func mentioning(author channels.Author, body string, targets ...string) channels.NewMessage {
	m := text(body)
	m.Author = author
	for _, tg := range targets {
		m.Mentions = append(m.Mentions, login(tg))
	}
	return m
}

// addressed returns r's addressed rows, failing on an error.
func addressed(t *testing.T, s channels.Store, r channels.Reader) []channels.Addressed {
	t.Helper()
	got, err := s.ListAddressed(context.Background(), r, channels.AddressAny, channels.MaxAddressedScan)
	require.NoError(t, err)
	return got
}

// rowKey is the (seq, kind) of an addressed row.
type rowKey struct {
	Seq  int64
	Kind channels.AddressKind
}

// rowKeys returns the (seq, kind) of each row of as, in order.
func rowKeys(as []channels.Addressed) []rowKey {
	out := make([]rowKey, len(as))
	for i, a := range as {
		out[i] = rowKey{a.Seq, a.Kind}
	}
	return out
}

// testAppendAddressed pins the rows [channels.Store.Append] writes: one
// per mention target with the message's author and type, the targets kept
// on the message; a reply row for the parent author under
// [channels.ReplyTarget] and none for a reply to one's own message, a
// trigger post, an unknown or rela author, or the same user in another
// organization; a mention and a reply row for a reply that also mentions
// the parent author.
func testAppendAddressed(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	carolR := channels.Reader{User: "carol"}

	user := channels.Mention{Target: "USR-7", Form: channels.FormUser, Span: "usr-7"}
	nm := mentioning(Alice, "hello", "carol")
	nm.Mentions = append(nm.Mentions, user, login("carol"))
	first := mustAppend(t, s, k, nm)
	require.Equal(t, []channels.Mention{user, login("carol")}, first.Mentions, "deduplicated and sorted")
	got, err := s.Get(ctx, k, first.Seq)
	require.NoError(t, err)
	require.Equal(t, first.Mentions, got.Mentions, "the targets are stored with their form")
	rows := addressed(t, s, carolR)
	require.Len(t, rows, 1)
	require.Equal(t, channels.Addressed{
		Channel: rows[0].Channel, Seq: 1, At: channels.NormTime(T0), Kind: channels.AddressMention,
		Author: Alice, Type: "text",
	}, rows[0])
	require.Equal(t, k, rows[0].Channel.Key)
	require.Len(t, addressed(t, s, channels.Reader{User: "USR-7"}), 1, "the user target is its canonical id")
	require.Empty(t, addressed(t, s, channels.Reader{User: "usr-7"}), "the span text is no target")

	own := text("own reply")
	own.ReplyTo = first.Seq
	ownReply := mustAppend(t, s, k, own)
	require.Empty(t, ownReply.ReplyTarget)
	require.Empty(t, addressed(t, s, readerAlice), "a reply to one's own message addresses no one")

	reply := fromBob("bob replies")
	reply.ReplyTo = first.Seq
	bobReply := mustAppend(t, s, k, reply)
	require.Equal(t, "alice", bobReply.ReplyTarget)
	rows = addressed(t, s, readerAlice)
	require.Equal(t, []rowKey{{bobReply.Seq, channels.AddressReply}}, rowKeys(rows))
	require.Equal(t, first.Seq, rows[0].ReplyTo)
	require.Equal(t, Bob, rows[0].Author)

	trigger := text("trigger post")
	trigger.TriggeredBy = "auto:room/x"
	triggered := mustAppend(t, s, k, trigger)
	for _, parent := range []channels.NewMessage{
		{Type: "text", Author: channels.Author{User: "unknown"}, At: T0},
		{Type: "text", Author: channels.Author{User: "system:realtime"}, At: T0},
		{Type: "text", Author: channels.Author{User: "carol", OrgID: "org-a"}, At: T0},
	} {
		p := mustAppend(t, s, k, parent)
		r := channels.NewMessage{Type: "text", Author: channels.Author{User: "carol", OrgID: "org-b"}, At: T0,
			ReplyTo: p.Seq}
		require.Empty(t, mustAppend(t, s, k, r).ReplyTarget, parent.Author.User)
	}
	r := fromBob("reply to a trigger post")
	r.ReplyTo = triggered.Seq
	require.Empty(t, mustAppend(t, s, k, r).ReplyTarget)
	for _, u := range []string{"unknown", "system:realtime"} {
		require.Empty(t, addressed(t, s, channels.Reader{User: u}), u)
	}
	require.Len(t, addressed(t, s, carolR), 1, "the same user in another org gets no reply row")

	both := mentioning(Bob, "reply and mention", "alice")
	both.ReplyTo = first.Seq
	bothMsg := mustAppend(t, s, k, both)
	require.Equal(t, []rowKey{{bobReply.Seq, channels.AddressReply}, {bothMsg.Seq, channels.AddressMention},
		{bothMsg.Seq, channels.AddressReply}}, rowKeys(addressed(t, s, readerAlice)), "equal times order by seq")
}

// testAppendAddressedRefused pins that a refused append writes no row,
// and that invalid or too many targets refuse the append.
func testAppendAddressedRefused(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	dave := channels.Reader{User: "dave"}
	mustAppend(t, s, k, text("first"))

	life := key("TKT-2", "room")
	nm := text("life one")
	nm.Incarnation = "life-1"
	mustAppend(t, s, life, nm)
	stale := mentioning(Alice, "stale", "dave")
	stale.Incarnation = "life-2"
	_, err := s.Append(ctx, life, stale)
	require.ErrorIs(t, err, channels.ErrStaleAnchor)
	closed := key("TKT-3", "room")
	mustAppend(t, s, closed, text("one"))
	_, err = s.SetLifecycle(ctx, closed, channels.StateOpen, channels.Transition{To: channels.StateClosed, By: Bob,
		At: T0})
	require.NoError(t, err)
	open := mentioning(Alice, "needs open", "dave")
	open.RequireOpen = true
	_, err = s.Append(ctx, closed, open)
	require.ErrorIs(t, err, channels.ErrNotOpen)
	bad := mentioning(Alice, "bad reply", "dave")
	bad.ReplyTo = 999
	_, err = s.Append(ctx, k, bad)
	require.ErrorIs(t, err, channels.ErrInvalidReply)
	require.Empty(t, addressed(t, s, dave), "a refused append writes no row")

	for _, m := range []channels.Mention{
		{Target: "", Form: channels.FormLogin},
		{Target: "a\x00b", Form: channels.FormLogin},
		{Target: strings.Repeat("a", channels.MaxTextBytes+1), Form: channels.FormLogin},
		{Target: "x", Form: "group"},
		{Target: " x", Form: channels.FormLogin},
		{Target: "x ", Form: channels.FormLogin},
		{Target: "\xff", Form: channels.FormLogin},
	} {
		nm := text("invalid")
		nm.Mentions = []channels.Mention{m}
		_, appendErr := s.Append(ctx, k, nm)
		require.ErrorIs(t, appendErr, channels.ErrInvalidMessage, "%q", m.Target)
	}
	many := text("too many")
	for i := range channels.MaxMentions + 1 {
		many.Mentions = append(many.Mentions, login(fmt.Sprint("user-", i)))
	}
	_, err = s.Append(ctx, k, many)
	require.ErrorIs(t, err, channels.ErrTooLarge)
}

// testEditRemoveAddressed pins [channels.Store.Edit] and
// [channels.Store.Remove] on rows: an edit keeps the rows of kept targets
// with their time, writes added targets at the edit time, drops the rest
// and keeps the reply row; an edit with no targets drops every mention
// row; a removal drops every row of the message and clears its targets;
// removing a parent keeps the reply rows of its replies.
func testEditRemoveAddressed(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	carolR, dave, erin := channels.Reader{User: "carol"}, channels.Reader{User: "dave"}, channels.Reader{User: "erin"}
	parent := mustAppend(t, s, k, fromBob("parent"))
	nm := mentioning(Alice, "reply", "carol", "dave")
	nm.ReplyTo = parent.Seq
	msg := mustAppend(t, s, k, nm)

	t1 := T0.Add(time.Minute)
	edited, err := s.Edit(ctx, k, msg.Seq, map[string]any{"body": "edited"},
		[]channels.Mention{login("erin"), login("carol")}, t1)
	require.NoError(t, err)
	require.Equal(t, []channels.Mention{login("carol"), login("erin")}, edited.Mentions)
	got, err := s.Get(ctx, k, msg.Seq)
	require.NoError(t, err)
	require.Equal(t, edited.Mentions, got.Mentions, "the edit replaced the stored targets")
	require.Equal(t, channels.NormTime(T0), addressed(t, s, carolR)[0].At, "a kept row keeps its time")
	require.Equal(t, channels.NormTime(t1), addressed(t, s, erin)[0].At, "an added row takes the edit time")
	require.Empty(t, addressed(t, s, dave), "a dropped target's row goes")
	require.Len(t, addressed(t, s, readerBob), 1, "the reply row stays")

	_, err = s.Edit(ctx, k, msg.Seq, map[string]any{"body": "no mentions"}, nil, t1)
	require.NoError(t, err)
	require.Empty(t, addressed(t, s, carolR), "an edit that removes the only mentions drops their rows")
	require.Empty(t, addressed(t, s, erin))
	require.Len(t, addressed(t, s, readerBob), 1, "the reply row stays")

	t2 := T0.Add(2 * time.Minute)
	_, err = s.Edit(ctx, k, msg.Seq, map[string]any{"body": "again"}, []channels.Mention{login("carol")}, t2)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(t2), addressed(t, s, carolR)[0].At)

	removed, err := s.Remove(ctx, k, msg.Seq, channels.Removal{By: Alice, At: t2})
	require.NoError(t, err)
	require.Nil(t, removed.Mentions)
	got, err = s.Get(ctx, k, msg.Seq)
	require.NoError(t, err)
	require.Nil(t, got.Mentions, "a removal clears the targets")
	require.Empty(t, addressed(t, s, carolR), "a removal drops the mention rows")
	require.Empty(t, addressed(t, s, readerBob), "a removal drops the reply row")

	r := text("second reply")
	r.ReplyTo = parent.Seq
	second := mustAppend(t, s, k, r)
	_, err = s.Remove(ctx, k, parent.Seq, channels.Removal{By: Bob, At: t2})
	require.NoError(t, err)
	require.Equal(t, []rowKey{{second.Seq, channels.AddressReply}}, rowKeys(addressed(t, s, readerBob)),
		"removing a parent keeps its replies' reply rows")
}

// testListAddressed pins [channels.Store.ListAddressed]: newest first, a
// marker at the seq hides a row and one below keeps it, each organization
// has its own marker, a tombstoned channel is absent until Untombstone, a
// Rekey carries the rows, another target's rows are absent, and the
// refusals.
func testListAddressed(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	a, b := key("L-1", "room"), key("L-2", "room")
	m1 := mentioning(Alice, "one", "carol")
	mustAppend(t, s, a, m1)
	m2 := mentioning(Alice, "two", "carol", "dave")
	m2.At = T0.Add(time.Minute)
	mustAppend(t, s, b, m2)
	m3 := mentioning(Bob, "three", "carol")
	m3.At = T0.Add(2 * time.Minute)
	mustAppend(t, s, a, m3)

	got := addressed(t, s, carol)
	require.Len(t, got, 3)
	require.Equal(t, []channels.Key{a, b, a}, []channels.Key{got[0].Channel.Key, got[1].Channel.Key,
		got[2].Channel.Key}, "newest first")
	require.Equal(t, []int64{2, 1, 1}, []int64{got[0].Seq, got[1].Seq, got[2].Seq})

	setRead(t, s, a, carol, 1, T0)
	got = addressed(t, s, carol)
	require.Equal(t, []rowKey{{2, channels.AddressMention}, {1, channels.AddressMention}}, rowKeys(got),
		"a marker at the seq hides the row; one below keeps the later row")
	require.Len(t, addressed(t, s, carol2), 3, "the same user in another org has its own marker")
	require.Len(t, addressed(t, s, channels.Reader{User: "dave"}), 1, "another target's rows are separate")
	require.Empty(t, addressed(t, s, readerAlice))

	batch := tombstone(t, s, "L-1", "", T0)
	require.Len(t, addressed(t, s, carol2), 1, "a tombstoned channel is absent")
	require.NoError(t, s.Untombstone(ctx, batch, T0.Add(time.Minute), nil))
	require.Len(t, addressed(t, s, carol2), 3, "Untombstone brings the rows back")

	require.NoError(t, s.Rekey(ctx, "L-2", "L-9", T0))
	moved := 0
	for _, r := range addressed(t, s, carol2) {
		require.NotEqual(t, "L-2", r.Channel.Key.ID)
		if r.Channel.Key.ID == "L-9" {
			moved++
		}
	}
	require.Equal(t, 1, moved, "a rename carries the rows")

	limited, err := s.ListAddressed(ctx, carol2, channels.AddressAny, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	for _, bad := range []struct {
		r     channels.Reader
		kind  channels.AddressKind
		limit int
	}{
		{carol, channels.AddressAny, 0}, {carol, channels.AddressAny, channels.MaxAddressedScan + 1},
		{channels.Reader{}, channels.AddressAny, 1}, {carol, "group", 1},
	} {
		_, err := s.ListAddressed(ctx, bad.r, bad.kind, bad.limit)
		require.ErrorIs(t, err, channels.ErrInvalidMessage)
	}
}

// testListAddressedScan pins the scan window of
// [channels.Store.ListAddressed]: one unread row older than
// [channels.MaxAddressedScan] read rows of the same target is outside the
// scan and absent, as documented.
func testListAddressedScan(t *testing.T, h Harness) {
	t.Helper()
	if h.SkipScale {
		t.Skip("the harness runs the scale tests elsewhere")
	}
	s := h.Store
	target := channels.Reader{User: "scan-target"}
	mustAppend(t, s, key("S-0", "room"), mentioning(Alice, "old unread", target.User))
	n := channels.MaxAddressedScan + 1
	per := channels.MaxAddressedPerTarget
	for i := range n {
		k := key(fmt.Sprintf("S-%d", 1+i/per), "room")
		m := mentioning(Alice, "newer", target.User)
		m.At = T0.Add(time.Duration(i+1) * time.Second)
		mustAppend(t, s, k, m)
	}
	for c := 1; c <= (n+per-1)/per; c++ {
		setRead(t, s, key(fmt.Sprintf("S-%d", c), "room"), target, int64(per), T0)
	}
	require.Empty(t, addressed(t, s, target), "the unread row is outside the scan window")
}

// testListAddressedKind pins the kind filter of
// [channels.Store.ListAddressed]: a kind lists only its rows, and
// [channels.AddressAny] lists both.
func testListAddressedKind(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	k := key("K-1", "room")
	target := channels.Reader{User: "kind-target"}
	parent := text("question")
	parent.Author = channels.Author{User: target.User}
	q := mustAppend(t, s, k, parent)
	mention := mustAppend(t, s, k, mentioning(Alice, "a mention", target.User))
	reply := fromBob("a reply")
	reply.ReplyTo = q.Seq
	r := mustAppend(t, s, k, reply)
	list := func(kind channels.AddressKind) []rowKey {
		got, err := s.ListAddressed(context.Background(), target, kind, channels.MaxAddressedScan)
		require.NoError(t, err)
		return rowKeys(got)
	}
	require.Equal(t, []rowKey{{mention.Seq, channels.AddressMention}}, list(channels.AddressMention))
	require.Equal(t, []rowKey{{r.Seq, channels.AddressReply}}, list(channels.AddressReply))
	require.ElementsMatch(t, []rowKey{{mention.Seq, channels.AddressMention}, {r.Seq, channels.AddressReply}},
		list(channels.AddressAny))
}

// testListAddressedKindScan pins that the kind filter runs inside the
// scan of [channels.MaxAddressedScan] rows, and that a call returns every
// unread row the scan finds: an unread reply older than that many unread
// mentions is outside the scan of every kind, yet listed for
// [channels.AddressReply], and the mentions all come back, more than the
// inbox ever lists, so the caller's gate sees each of them.
func testListAddressedKindScan(t *testing.T, h Harness) {
	t.Helper()
	if h.SkipScale {
		t.Skip("the harness runs the scale tests elsewhere")
	}
	s := h.Store
	target := channels.Reader{User: "kind-scan-target"}
	k0 := key("KS-0", "room")
	parent := text("question")
	parent.Author = channels.Author{User: target.User}
	q := mustAppend(t, s, k0, parent)
	reply := fromBob("old reply")
	reply.ReplyTo = q.Seq
	r := mustAppend(t, s, k0, reply)
	n := channels.MaxAddressedScan
	per := channels.MaxAddressedPerTarget
	for i := range n {
		m := mentioning(Alice, "newer", target.User)
		m.At = T0.Add(time.Duration(i+1) * time.Second)
		mustAppend(t, s, key(fmt.Sprintf("KS-%d", 1+i/per), "room"), m)
	}
	ctx := context.Background()
	all, err := s.ListAddressed(ctx, target, channels.AddressAny, channels.MaxAddressedScan)
	require.NoError(t, err)
	require.Len(t, all, n, "every unread row the scan finds comes back")
	for _, a := range all {
		require.Equal(t, channels.AddressMention, a.Kind, "the old reply is outside the scan of every kind")
	}
	replies, err := s.ListAddressed(ctx, target, channels.AddressReply, channels.MaxAddressedScan)
	require.NoError(t, err)
	require.Equal(t, []rowKey{{r.Seq, channels.AddressReply}}, rowKeys(replies),
		"the reply scan does not count the mention rows")
}

// testAddressedTargetCap pins [channels.MaxAddressedPerTarget] and the
// eviction order of [channels.Evict]: a set full of one poster's unread
// rows skips another poster's row; once the target reads, the next row
// evicts the oldest read row; a poster at the cap with their own unread
// rows evicts their own oldest.
func testAddressedTargetCap(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	k := key("C-1", "room")
	for range channels.MaxAddressedPerTarget {
		mustAppend(t, s, k, mentioning(Alice, "a", "carol"))
	}
	skipped := mustAppend(t, s, k, mentioning(Bob, "b", "carol"))
	rows := addressed(t, s, carol)
	require.Len(t, rows, channels.MaxAddressedPerTarget)
	for _, r := range rows {
		require.Equal(t, Alice, r.Author, "another poster's row is skipped, never evicting")
		require.NotEqual(t, skipped.Seq, r.Seq)
	}

	setRead(t, s, k, carol, 50, T0)
	next := mustAppend(t, s, k, mentioning(Bob, "b2", "carol"))
	rows = addressed(t, s, carol2) // no marker in org-b: sees every row
	require.Len(t, rows, channels.MaxAddressedPerTarget)
	seen := map[int64]bool{}
	for _, r := range rows {
		seen[r.Seq] = true
	}
	require.False(t, seen[1], "the oldest read row was evicted")
	require.True(t, seen[2])
	require.True(t, seen[next.Seq], "the new row was written")

	own := key("C-2", "room")
	for range channels.MaxAddressedPerTarget {
		mustAppend(t, s, own, mentioning(Alice, "a", "dave"))
	}
	last := mustAppend(t, s, own, mentioning(Alice, "a again", "dave"))
	rows = addressed(t, s, channels.Reader{User: "dave"})
	require.Len(t, rows, channels.MaxAddressedPerTarget)
	seen = map[int64]bool{}
	for _, r := range rows {
		seen[r.Seq] = true
	}
	require.False(t, seen[1], "the poster's own oldest unread row was evicted")
	require.True(t, seen[last.Seq])

	_, err := s.Edit(context.Background(), own, 1, map[string]any{"body": "kept"},
		[]channels.Mention{login("dave")}, T0.Add(time.Hour))
	require.NoError(t, err)
	for _, r := range addressed(t, s, channels.Reader{User: "dave"}) {
		require.NotEqual(t, int64(1), r.Seq, "an edit that keeps the target does not bring an evicted row back")
	}
}

// testAddressedErased pins that Delete and Purge erase the addressed rows:
// the target's name is gone from storage, and a recreated channel holds no
// rows.
func testAddressedErased(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	gone := channels.Reader{User: "target-erased-9q2"}
	k := key("TKT-1", "room")
	mustAppend(t, s, k, mentioning(Alice, "one", gone.User))
	require.Len(t, addressed(t, s, gone), 1)
	head, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, k, channels.Deletion{Expect: head.State, Rev: head.Rev, By: Bob, At: T0}))
	require.NotContains(t, string(h.Raw(t)), gone.User, "a deleted channel's rows are gone")
	mustAppend(t, s, k, text("recreated"))
	require.Empty(t, addressed(t, s, gone), "a recreated channel holds no rows")

	purged := key("TKT-2", "room")
	mustAppend(t, s, purged, mentioning(Alice, "one", gone.User))
	b := fileBatch(t, s, "TKT-2", "", T0, channels.Filing{Policy: channels.PolicyDelete})
	require.NoError(t, s.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)))
	require.NotContains(t, string(h.Raw(t)), gone.User, "a purged channel's rows are gone")
}
