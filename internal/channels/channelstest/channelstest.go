// Package channelstest is the conformance suite every [channels.Store]
// backend must pass. The contract it pins is the one the channels package
// doc lists; asserting it once here keeps three backends from each behaving
// slightly differently.
package channelstest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Harness is what a backend hands the suite for one test.
type Harness struct {
	// Store is a fresh, empty store.
	Store channels.Store
	// Raw returns every byte the backend holds for channels: the files
	// under its directory, or its rows rendered as text. The erasure tests
	// search it for content that must be gone.
	Raw func(t *testing.T) []byte
	// Second opens another Store over the same storage, standing in for a
	// second process. Nil for a backend that admits one process only.
	Second func(t *testing.T) channels.Store
	// SeedLegacyType writes a type record the way a release before
	// cardinality was kept left it: the anchor mode only. Nil skips the
	// legacy-record test.
	SeedLegacyType func(t *testing.T, typ string, anchor channels.AnchorMode)
}

// T0 is the suite's base time. It has a sub-microsecond part on purpose:
// every backend must return the time normalized by [channels.NormTime].
var T0 = time.Date(2026, 10, 12, 9, 30, 0, 123456789, time.UTC)

// Alice and Bob are the suite's authors.
var (
	Alice = channels.Author{User: "alice", Tool: "data-entry"}
	Bob   = channels.Author{User: "bob", Tool: "mcp", PrincipalType: "agent"}
)

// RunAll runs every conformance test against stores from newHarness.
func RunAll(t *testing.T, newHarness func(t *testing.T) Harness) {
	t.Helper()
	tests := []conformanceTest{
		{"ImplicitChannelOnFirstAppend", testImplicitCreate},
		{"AppendRequireOpen", testAppendRequireOpen},
		{"NamedChannelMustExist", testNamedMustExist},
		{"GaplessSeqUnderConcurrency", testGapless},
		{"Paging", testPaging},
		{"PageLimitIsClamped", testPageClamp},
		{"AfterRevSeesEdits", testAfterRev},
		{"Edit", testEdit},
		{"RemoveErasesContent", testRemoveErases},
		{"EditKeepsNoHistory", testEditErases},
		{"Replies", testReplies},
		{"Actions", testActions},
		{"ActionGuards", testActionGuards},
		{"Lifecycle", testLifecycle},
		{"CreateMany", testCreateMany},
		{"CreateManyCap", testCreateManyCap},
		{"CreateManyMax", testCreateManyMax},
		{"ListAndListEntity", testList},
		{"DeleteKeepsAudit", testDelete},
		{"DeleteCompareAndSet", testDeleteCAS},
		{"DeleteRacesAppend", testDeleteRacesAppend},
		{"Rekey", testRekey},
		{"RekeyConflict", testRekeyConflict},
		{"TombstoneEntity", testTombstoneEntity},
		{"TombstoneFace", testTombstoneFace},
		{"Caps", testCaps},
		{"InvalidKeys", testInvalidKeys},
		{"Keys", testKeys},
		{"TimesRoundTrip", testTimes},
		{"SecondInstance", testSecondInstance},
		{"TypeRecords", testTypeRecords},
		{"TypeRecordConflict", testTypeRecordConflict},
		{"TypeRecordRefs", testTypeRecordRefs},
		{"TypeRecordConcurrent", testTypeRecordConcurrent},
		{"LegacyTypeRecord", testLegacyTypeRecord},
		{"UntombstoneRestoresItsBatch", testUntombstoneBatch},
		{"UntombstoneFace", testUntombstoneFace},
		{"UntombstoneOccupied", testUntombstoneOccupied},
		{"TombstoneHandleRoundTrips", testHandleRoundTrip},
		{"Incarnation", testIncarnation},
		{"ArchiveFromOpen", testArchiveFromOpen},
		{"AfterRevPagesInRevOrder", testAfterRevPaging},
		{"ManyReplies", testManyReplies},
		{"LongestAnchor", testLongestAnchor},
		{"TombstonesList", testTombstonesList},
		{"FilerSeesMovedChannels", testFilerSeesMovedChannels},
		{"FilerRefuses", testFilerRefuses},
		{"DueTombstones", testDueTombstones},
		{"ConfirmTombstone", testConfirmTombstone},
		{"ConfirmTombstones", testConfirmTombstones},
		{"Purge", testPurge},
		{"PurgeTypes", testPurgeTypes},
		{"PurgeFace", testPurgeFace},
		{"PurgeAfterUntombstone", testPurgeAfterUntombstone},
		{"DropTypeRecord", testDropTypeRecord},
		{"SessionStep", testSessionStep},
		{"SessionNoMove", testSessionNoMove},
		{"SessionDeciderSeesCurrent", testSessionDeciderSeesCurrent},
		{"SessionImplicitCreate", testSessionImplicitCreate},
		{"SessionInvalidMove", testSessionInvalidMove},
		{"SessionSurvivesMoves", testSessionSurvivesMoves},
		{"SessionConcurrent", testSessionConcurrent},
		{"SessionNoticeMarker", testSessionNoticeMarker},
		{"SessionStateNames", testSessionStateNames},
		{"SessionReply", testSessionReply},
	}
	for _, list := range [][]conformanceTest{tests, timerTests, reminderTests, outputsTests, waitingTests, readsTests} {
		for _, tc := range list {
			t.Run(tc.name, func(t *testing.T) {
				tc.fn(t, newHarness(t))
			})
		}
	}
}

// conformanceTest is one named test of [RunAll].
type conformanceTest = struct {
	name string
	fn   func(t *testing.T, h Harness)
}

// testAppendRequireOpen pins [channels.NewMessage.RequireOpen]: an append
// that requires an open channel creates an implicit one, appends to an open
// one, and is refused by a closed or archived one, which an append without
// it still reaches.
func testAppendRequireOpen(t *testing.T, h Harness) {
	t.Helper()
	s, ctx := h.Store, context.Background()
	k := key("TKT-1", "room")
	m := text("first")
	m.RequireOpen = true
	mustAppend(t, s, k, m)
	mustAppend(t, s, k, m)
	_, err := s.SetLifecycle(ctx, k, channels.StateOpen, channels.Transition{To: channels.StateClosed, By: Bob, At: T0})
	require.NoError(t, err)
	_, err = s.Append(ctx, k, m)
	require.ErrorIs(t, err, channels.ErrNotOpen)
	_, err = s.SetLifecycle(ctx, k, channels.StateClosed,
		channels.Transition{To: channels.StateArchived, By: Bob, At: T0})
	require.NoError(t, err)
	_, err = s.Append(ctx, k, m)
	require.ErrorIs(t, err, channels.ErrNotOpen)
	got := mustAppend(t, s, k, text("anyway"))
	require.Equal(t, int64(3), got.Seq, "a refused append takes no seq")
}

func key(id, typ string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id}, Type: typ}
}

func text(body string) channels.NewMessage {
	return channels.NewMessage{Type: "text", Author: Alice, At: T0, Props: map[string]any{"body": body}}
}

func mustAppend(t *testing.T, s channels.Store, k channels.Key, m channels.NewMessage) channels.Message {
	t.Helper()
	got, err := s.Append(context.Background(), k, m)
	require.NoError(t, err)
	return got
}

// tombstone tombstones id@face and returns the handle.
func tombstone(t *testing.T, s channels.Store, id string, face entity.Face, at time.Time) channels.TombstoneHandle {
	t.Helper()
	h, err := s.Tombstone(context.Background(), id, face, at, nil)
	require.NoError(t, err)
	return h
}

// requireTombstoneErr returns a check that a refused Tombstone returned
// want and no handle.
func requireTombstoneErr(t *testing.T, want error) func(channels.TombstoneHandle, error) {
	t.Helper()
	return func(h channels.TombstoneHandle, err error) {
		t.Helper()
		require.ErrorIs(t, err, want)
		require.True(t, h.IsZero())
	}
}

func seqs(ms []channels.Message) []int64 {
	out := make([]int64, len(ms))
	for i, m := range ms {
		out[i] = m.Seq
	}
	return out
}

func testImplicitCreate(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	_, err := h.Store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.Messages(ctx, k, channels.Page{})
	require.ErrorIs(t, err, channels.ErrNotFound)

	m := mustAppend(t, h.Store, k, text("hello"))
	require.Equal(t, int64(1), m.Seq)
	require.Positive(t, m.Rev)
	require.Equal(t, "hello", m.Props["body"])
	require.Equal(t, Alice, m.Author)

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, k, head.Key)
	require.Equal(t, channels.StateOpen, head.State)
	require.Equal(t, int64(1), head.LastSeq)
	require.Equal(t, m.Rev, head.Rev)
	require.Equal(t, Alice, head.CreatedBy)
	require.Equal(t, channels.NormTime(T0), head.CreatedAt)

	got, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.Equal(t, m, got)
	_, err = h.Store.Get(ctx, k, 2)
	require.ErrorIs(t, err, channels.ErrNotFound)

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, audit, 1)
	require.Equal(t, channels.AuditCreate, audit[0].Op)
	require.Equal(t, "room", audit[0].Type)
}

func testNamedMustExist(t *testing.T, h Harness) {
	t.Helper()
	k := key("TKT-1", "topic")
	k.Name = "abc"
	_, err := h.Store.Append(context.Background(), k, text("x"))
	require.ErrorIs(t, err, channels.ErrNotFound)
	stepped := text("x")
	stepped.Session = sessionStep(moveFrom("detected", "contained"))
	_, err = h.Store.Append(context.Background(), k, stepped)
	require.ErrorIs(t, err, channels.ErrNotFound, "a step does not create a named channel")
}

// testGapless is the concurrency contract: 50 concurrent appends to one
// channel get seqs 1..50 exactly once each. With a second instance, half go
// through it, standing in for a second process.
func testGapless(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	stores := []channels.Store{h.Store}
	if h.Second != nil {
		stores = append(stores, h.Second(t))
	}
	k := key("TKT-1", "room")
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	got := make(chan int64, n)
	for i := range n {
		s := stores[i%len(stores)]
		wg.Go(func() {
			m, err := s.Append(ctx, k, text(fmt.Sprintf("m%d", i)))
			if err != nil {
				errs <- err
				return
			}
			got <- m.Seq
		})
	}
	wg.Wait()
	close(errs)
	close(got)
	for err := range errs {
		require.NoError(t, err)
	}
	seen := map[int64]bool{}
	for s := range got {
		require.False(t, seen[s], "seq %d issued twice", s)
		seen[s] = true
	}
	require.Len(t, seen, n)
	for i := int64(1); i <= n; i++ {
		require.True(t, seen[i], "seq %d missing", i)
	}

	page, err := h.Store.Messages(ctx, k, channels.Page{Limit: channels.MaxPageLimit})
	require.NoError(t, err)
	require.Len(t, page, n)
	for i, m := range page {
		require.Equal(t, int64(i+1), m.Seq)
	}
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(n), head.LastSeq)

	revs := map[int64]bool{}
	for _, m := range page {
		require.False(t, revs[m.Rev], "rev %d issued twice", m.Rev)
		revs[m.Rev] = true
		require.LessOrEqual(t, m.Rev, head.Rev)
	}
}

func testPaging(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	for i := range 7 {
		mustAppend(t, h.Store, k, text(fmt.Sprintf("m%d", i+1)))
	}
	cases := []struct {
		name string
		page channels.Page
		want []int64
	}{
		{"tail", channels.Page{Limit: 3}, []int64{5, 6, 7}},
		{"all", channels.Page{}, []int64{1, 2, 3, 4, 5, 6, 7}},
		{"after", channels.Page{AfterSeq: 2, Limit: 3}, []int64{3, 4, 5}},
		{"after end", channels.Page{AfterSeq: 7}, []int64{}},
		{"before", channels.Page{BeforeSeq: 6, Limit: 2}, []int64{4, 5}},
		{"before start", channels.Page{BeforeSeq: 1}, []int64{}},
		{"before far", channels.Page{BeforeSeq: 100, Limit: 2}, []int64{6, 7}},
		{"after rev", channels.Page{AfterRev: 0, Limit: 2}, []int64{6, 7}},
		{"by rev from zero", channels.Page{ByRev: true, Limit: 2}, []int64{1, 2}},
		{"by rev after", channels.Page{ByRev: true, AfterRev: 5}, []int64{6, 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := h.Store.Messages(ctx, k, tc.page)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.want, seqs(got))
		})
	}
	_, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: 1, BeforeSeq: 3})
	require.ErrorIs(t, err, channels.ErrInvalidPage)
	_, err = h.Store.Messages(ctx, k, channels.Page{AfterSeq: -1})
	require.ErrorIs(t, err, channels.ErrInvalidPage)
	_, err = h.Store.Messages(ctx, k, channels.Page{ByRev: true, AfterSeq: 1})
	require.ErrorIs(t, err, channels.ErrInvalidPage)
}

func testPageClamp(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	for range channels.MaxPageLimit + 5 {
		mustAppend(t, h.Store, k, text("x"))
	}
	got, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: 0, Limit: 10_000})
	require.NoError(t, err)
	require.Len(t, got, channels.MaxPageLimit)
	require.Equal(t, int64(6), got[0].Seq)
	got, err = h.Store.Messages(ctx, k, channels.Page{AfterSeq: 1, Limit: 10_000})
	require.NoError(t, err)
	require.Len(t, got, channels.MaxPageLimit)
	require.Equal(t, int64(2), got[0].Seq)
}

func testAfterRev(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	for i := range 3 {
		mustAppend(t, h.Store, k, text(fmt.Sprintf("m%d", i+1)))
	}
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)

	edited, err := h.Store.Edit(ctx, k, 1, map[string]any{"body": "m1 edited"}, T0.Add(time.Minute))
	require.NoError(t, err)
	require.Greater(t, edited.Rev, head.Rev)
	removed, err := h.Store.Remove(ctx, k, 2, channels.Removal{By: Bob, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)
	require.Greater(t, removed.Rev, edited.Rev)

	got, err := h.Store.Messages(ctx, k, channels.Page{AfterRev: head.Rev})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, seqs(got), "ordered by rev: the edit, then the removal")
	require.Equal(t, "m1 edited", got[0].Props["body"])
	require.NotNil(t, got[1].Removed)

	after, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, removed.Rev, after.Rev)
	got, err = h.Store.Messages(ctx, k, channels.Page{AfterRev: after.Rev})
	require.NoError(t, err)
	require.Empty(t, got)
}

func testEdit(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	orig := mustAppend(t, h.Store, k, text("first"))
	at := T0.Add(time.Minute)
	got, err := h.Store.Edit(ctx, k, 1, map[string]any{"body": "second", "n": 2}, at)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"body": "second", "n": float64(2)}, got.Props)
	require.Equal(t, channels.NormTime(at), got.EditedAt)
	require.Equal(t, orig.Author, got.Author)
	require.Equal(t, orig.At, got.At)
	read, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.Equal(t, got, read)

	_, err = h.Store.Edit(ctx, k, 9, map[string]any{"body": "x"}, at)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.Edit(ctx, key("TKT-2", "room"), 1, map[string]any{"body": "x"}, at)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.Remove(ctx, k, 1, channels.Removal{By: Alice, At: at})
	require.NoError(t, err)
	_, err = h.Store.Edit(ctx, k, 1, map[string]any{"body": "x"}, at)
	require.ErrorIs(t, err, channels.ErrRemoved)
	_, err = h.Store.Remove(ctx, k, 1, channels.Removal{By: Alice, At: at})
	require.ErrorIs(t, err, channels.ErrRemoved)
}

// testRemoveErases is the erasure contract: after a removal, the payload is
// gone from the backend's raw bytes, not only from reads.
func testRemoveErases(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	const payload = "erase-me-7f3a9c"
	const reason = "spam-reason-b41e"
	mustAppend(t, h.Store, k, text("keep"))
	secret := text(payload)
	secret.Author = channels.Author{User: "author-9d2e"}
	mustAppend(t, h.Store, k, secret)
	mustAppend(t, h.Store, k, text("after"))
	parent := mustAppend(t, h.Store, k, text("parent"))
	reply := text("reply")
	reply.ReplyTo = parent.Seq
	mustAppend(t, h.Store, k, reply)
	require.Contains(t, string(h.Raw(t)), payload)

	at := T0.Add(time.Hour)
	got, err := h.Store.Remove(ctx, k, 2, channels.Removal{By: Bob, At: at, Reason: reason})
	require.NoError(t, err)
	require.Equal(t, int64(2), got.Seq)
	require.Equal(t, &channels.Removal{By: Bob, At: channels.NormTime(at), Reason: reason}, got.Removed)
	require.Empty(t, got.Type)
	require.Empty(t, got.Props)
	require.True(t, got.Author.IsZero())
	require.True(t, got.At.IsZero())

	raw := string(h.Raw(t))
	require.NotContains(t, raw, payload, "removed content must be gone from storage")
	require.NotContains(t, raw, "author-9d2e", "the removed message's author must be gone from storage")

	// The parent of a thread keeps its place and its reply count.
	got, err = h.Store.Remove(ctx, k, parent.Seq, channels.Removal{By: Bob, At: at})
	require.NoError(t, err)
	require.Equal(t, 1, got.ReplyCount)

	page, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3, 4, 5}, seqs(page), "a removed message keeps its seq")
	require.Equal(t, "keep", page[0].Props["body"])

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	var removes int
	for _, r := range audit {
		if r.Op == channels.AuditRemove {
			removes++
			require.Equal(t, Bob, r.By)
		}
	}
	require.Equal(t, 2, removes)
	enc, err := json.Marshal(audit)
	require.NoError(t, err)
	require.NotContains(t, string(enc), reason, "the audit trail holds no content")
	require.NotContains(t, string(enc), payload)
}

func testEditErases(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	const old = "before-edit-5c1d"
	mustAppend(t, h.Store, k, text(old))
	mustAppend(t, h.Store, k, text("other"))
	_, err := h.Store.Edit(ctx, k, 1, map[string]any{"body": "after"}, T0.Add(time.Minute))
	require.NoError(t, err)
	require.NotContains(t, string(h.Raw(t)), old, "an edit keeps no history")
}

func testReplies(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	parent := mustAppend(t, h.Store, k, text("parent"))
	reply := text("reply")
	reply.ReplyTo = 1
	r1 := mustAppend(t, h.Store, k, reply)
	require.Equal(t, int64(1), r1.ReplyTo)
	r2 := mustAppend(t, h.Store, k, reply)

	got, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.Equal(t, 2, got.ReplyCount)
	require.Greater(t, got.Rev, parent.Rev, "a reply bumps the parent's rev")
	require.NotEqual(t, got.Rev, r2.Rev)

	changed, err := h.Store.Messages(ctx, k, channels.Page{AfterRev: r1.Rev})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{1, 3}, seqs(changed))

	nested := text("nested")
	nested.ReplyTo = r1.Seq
	_, err = h.Store.Append(ctx, k, nested)
	require.ErrorIs(t, err, channels.ErrInvalidReply)
	missing := text("missing")
	missing.ReplyTo = 99
	_, err = h.Store.Append(ctx, k, missing)
	require.ErrorIs(t, err, channels.ErrInvalidReply)

	mustAppend(t, h.Store, k, text("will go"))
	_, err = h.Store.Remove(ctx, k, 4, channels.Removal{By: Alice, At: T0})
	require.NoError(t, err)
	toRemoved := text("to removed")
	toRemoved.ReplyTo = 4
	_, err = h.Store.Append(ctx, k, toRemoved)
	require.ErrorIs(t, err, channels.ErrInvalidReply)

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(4), head.LastSeq, "a refused reply takes no seq")
}

func testActions(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("proposal"))
	mustAppend(t, h.Store, k, text("other"))
	at := T0.Add(time.Minute)

	pending := channels.Action{Status: channels.ActionPending, By: Alice, At: at}
	got, err := h.Store.SetAction(ctx, k, 1, "accept", channels.ActionNone, pending)
	require.NoError(t, err)
	require.Equal(t, channels.ActionPending, got.ActionStatus())
	require.Equal(t, "accept", got.Action.Name)
	require.Equal(t, channels.NormTime(at), got.Action.PendingAt)
	require.Equal(t, "proposal", got.Props["body"], "an action leaves the content alone")

	_, err = h.Store.SetAction(ctx, k, 1, "accept", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrActionPending, "a second accept loses to the pending one")
	_, err = h.Store.SetAction(ctx, k, 1, "reject", channels.ActionPending, pending)
	require.ErrorIs(t, err, channels.ErrConflict, "another action name loses")

	stale, err := h.Store.StalePending(ctx, at.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, []channels.PendingAction{{Key: k, Seq: 1, Name: "accept", PendingAt: channels.NormTime(at)}}, stale)
	stale, err = h.Store.StalePending(ctx, at)
	require.NoError(t, err)
	require.Empty(t, stale)

	done := channels.Action{
		Status: channels.ActionDone, By: Alice, At: at.Add(time.Second),
		PendingAt: at, Promoted: &channels.Promoted{ID: "DEC-1", Type: "decision"},
	}
	got, err = h.Store.SetAction(ctx, k, 1, "accept", channels.ActionPending, done)
	require.NoError(t, err)
	require.Equal(t, channels.ActionDone, got.ActionStatus())
	require.Equal(t, &channels.Promoted{ID: "DEC-1", Type: "decision"}, got.Action.Promoted)
	read, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.Equal(t, got, read)

	stale, err = h.Store.StalePending(ctx, at.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, stale)

	// A failed side effect, or a moderator, releases the pending action.
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionNone, pending)
	require.NoError(t, err)
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionPending, channels.Action{})
	require.ErrorIs(t, err, channels.ErrInvalidMessage, "a release names who released it")
	releasedAt := at.Add(2 * time.Second)
	got, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionPending,
		channels.Action{By: Bob, At: releasedAt})
	require.NoError(t, err)
	require.Nil(t, got.Action)
	// A promote that wrote nothing reverts its own pending action.
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionNone, pending)
	require.NoError(t, err)
	got, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionPending,
		channels.Action{By: Alice, At: releasedAt, Revert: true})
	require.NoError(t, err)
	require.Nil(t, got.Action)
	stale, err = h.Store.StalePending(ctx, at.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, stale)

	assertActionAudit(t, h, releasedAt)
	_, err = h.Store.SetAction(ctx, k, 9, "accept", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.SetAction(ctx, k, 1, "Bad Name", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	_, err = h.Store.Remove(ctx, k, 2, channels.Removal{By: Alice, At: at})
	require.NoError(t, err)
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrRemoved)
}

// testActionGuards checks that a pending action freezes its message and
// that an action can only be taken in an open channel.
func testActionGuards(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("one"))
	mustAppend(t, h.Store, k, text("two"))
	at := T0.Add(time.Minute)
	pending := channels.Action{Status: channels.ActionPending, By: Alice, At: at}

	_, err := h.Store.SetAction(ctx, k, 1, "accept", channels.ActionNone, pending)
	require.NoError(t, err)
	_, err = h.Store.Edit(ctx, k, 1, map[string]any{"body": "changed"}, at)
	require.ErrorIs(t, err, channels.ErrActionPending)
	_, err = h.Store.Remove(ctx, k, 1, channels.Removal{By: Bob, At: at})
	require.ErrorIs(t, err, channels.ErrActionPending)
	got, err := h.Store.Get(ctx, k, 1)
	require.NoError(t, err)
	require.Equal(t, "one", got.Props["body"], "a refused edit changes nothing")

	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: at})
	require.NoError(t, err)
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrNotOpen, "no new action in a closed channel")
	done := channels.Action{Status: channels.ActionDone, By: Alice, At: at.Add(time.Second), PendingAt: at}
	_, err = h.Store.SetAction(ctx, k, 1, "accept", channels.ActionPending, done)
	require.NoError(t, err, "a pending action may finish after the close")
	_, err = h.Store.Edit(ctx, k, 1, map[string]any{"body": "after"}, at)
	require.NoError(t, err, "a done action no longer freezes the message")
}

// assertActionAudit checks the action records testActions leaves.
func assertActionAudit(t *testing.T, h Harness, releasedAt time.Time) {
	t.Helper()
	audit, err := h.Store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	var actions []channels.AuditRecord
	for _, r := range audit {
		if r.Op == channels.AuditAction {
			actions = append(actions, r)
		}
	}
	require.Len(t, actions, 3, "a completed, a released and a reverted action are audited; going pending is not")
	require.Equal(t, "accept", actions[0].Action)
	require.Equal(t, int64(1), actions[0].Seq)
	require.Equal(t, &channels.Promoted{ID: "DEC-1", Type: "decision"}, actions[0].Promoted)
	require.Empty(t, actions[0].Status)
	require.Equal(t, channels.AuditRecord{
		At: channels.NormTime(releasedAt), Op: channels.AuditAction, Type: "room", Seq: 2, By: Bob,
		Action: "accept", Status: channels.AuditReleased,
	}, actions[1])
	require.Equal(t, channels.AuditReverted, actions[2].Status)
	require.Equal(t, Alice, actions[2].By)
}

func testLifecycle(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("x"))
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)

	closedAt := T0.Add(time.Hour)
	c, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: closedAt})
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, c.State)
	require.Equal(t, channels.NormTime(closedAt), c.ClosedAt)
	require.Greater(t, c.Rev, before.Rev, "a lifecycle change bumps the rev")
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, c, head)

	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: closedAt})
	require.ErrorIs(t, err, channels.ErrConflict)
	_, err = h.Store.SetLifecycle(ctx, key("TKT-9", "room"), channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: closedAt})
	require.ErrorIs(t, err, channels.ErrNotFound)

	assertClosedList(t, h, k, closedAt)

	auditLog, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Empty(t, auditLog[len(auditLog)-1].TriggeredBy, "a requested change names no trigger")
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: closedAt, TriggeredBy: "auto:room/reopen",
	})
	require.NoError(t, err)
	auditLog, err = h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Equal(t, "auto:room/reopen", auditLog[len(auditLog)-1].TriggeredBy)
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: closedAt})
	require.NoError(t, err)

	archivedAt := closedAt.Add(time.Hour)
	c, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed,
		channels.Transition{To: channels.StateArchived, By: Bob, At: archivedAt})
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(archivedAt), c.ArchivedAt)
	require.Equal(t, channels.NormTime(closedAt), c.ClosedAt)

	c, err = h.Store.SetLifecycle(ctx, k, channels.StateArchived,
		channels.Transition{To: channels.StateOpen, By: Alice, At: archivedAt.Add(time.Hour)})
	require.NoError(t, err)
	require.True(t, c.ClosedAt.IsZero())
	require.True(t, c.ArchivedAt.IsZero())
	reopenedAt := channels.NormTime(archivedAt.Add(time.Hour))
	require.Equal(t, reopenedAt, c.ReopenedAt)
	require.Equal(t, reopenedAt, c.StateSince(), "an open channel counts from its reopen")
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, c, head)
	got, err := h.Store.ListByLifecycle(ctx, "room", channels.StateOpen, reopenedAt)
	require.NoError(t, err)
	require.Empty(t, got, "a reopened channel has been open since its reopen, not its creation")
	got, err = h.Store.ListByLifecycle(ctx, "room", channels.StateOpen, reopenedAt.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, got, 1)

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	var trail []string
	for _, r := range audit {
		if r.Op == channels.AuditLifecycle {
			trail = append(trail, r.From+">"+r.To)
		}
	}
	require.Equal(t, []string{"open>closed", "closed>open", "open>closed", "closed>archived", "archived>open"}, trail)
}

// assertClosedList checks ListByLifecycle against k, the one channel,
// closed at closedAt.
func assertClosedList(t *testing.T, h Harness, k channels.Key, closedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	got, err := h.Store.ListByLifecycle(ctx, "room", channels.StateClosed, closedAt.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, k, got[0].Key)
	got, err = h.Store.ListByLifecycle(ctx, "other", channels.StateClosed, closedAt.Add(time.Second))
	require.NoError(t, err)
	require.Empty(t, got, "the type narrows the list")
	got, err = h.Store.ListByLifecycle(ctx, "room", channels.StateClosed, closedAt)
	require.NoError(t, err)
	require.Empty(t, got)
	got, err = h.Store.ListByLifecycle(ctx, "room", channels.StateOpen, closedAt.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, got)
	_, err = h.Store.ListByLifecycle(ctx, "", channels.StateOpen, closedAt)
	require.ErrorIs(t, err, channels.ErrInvalidKey, "the type is required")
}

func testCreateMany(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	anchor := channels.Anchor{ID: "TKT-1"}
	c, err := h.Store.CreateMany(ctx, anchor, channels.NewChannel{
		Type: "topic", Title: "Release plan", By: Alice, At: T0,
	})
	require.NoError(t, err)
	require.NoError(t, c.Key.Validate())
	require.NotEmpty(t, c.Key.Name)
	require.Equal(t, "Release plan", c.Title)
	require.Equal(t, channels.StateOpen, c.State)
	require.Equal(t, int64(0), c.LastSeq)

	head, err := h.Store.Head(ctx, c.Key)
	require.NoError(t, err)
	require.Equal(t, c, head)
	m := mustAppend(t, h.Store, c.Key, text("first"))
	require.Equal(t, int64(1), m.Seq)

	other, err := h.Store.CreateMany(ctx, anchor, channels.NewChannel{Type: "topic", Title: "", By: Alice, At: T0})
	require.NoError(t, err)
	require.NotEqual(t, c.Key.Name, other.Key.Name)

	_, err = h.Store.CreateMany(ctx, anchor, channels.NewChannel{
		Type: "topic", Title: strings.Repeat("é", channels.MaxTitleRunes+1), By: Alice, At: T0,
	})
	require.ErrorIs(t, err, channels.ErrTooLarge)
	_, err = h.Store.CreateMany(ctx, anchor, channels.NewChannel{
		Type: "topic", Title: strings.Repeat("é", channels.MaxTitleRunes), By: Alice, At: T0,
	})
	require.NoError(t, err, "the cap counts characters, not bytes")
	_, err = h.Store.CreateMany(ctx, anchor, channels.NewChannel{Type: "Bad", Title: "", By: Alice, At: T0})
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = h.Store.CreateMany(ctx, anchor, channels.NewChannel{
		Type: "topic", Title: "", By: channels.Author{}, At: T0,
	})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
}

func testCreateManyCap(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	anchor := channels.Anchor{ID: "TKT-1"}
	// The implicit channel of the same type does not count.
	mustAppend(t, h.Store, key("TKT-1", "topic"), text("x"))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var made, refused int
	errs := make(chan error, channels.MaxChannelsPerType+10)
	for range channels.MaxChannelsPerType + 10 {
		wg.Go(func() {
			_, err := h.Store.CreateMany(ctx, anchor, channels.NewChannel{Type: "topic", Title: "", By: Alice, At: T0})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				made++
			case errors.Is(err, channels.ErrTooManyChannels):
				refused++
			default:
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, channels.MaxChannelsPerType, made)
	require.Equal(t, 10, refused)

	// Another type or anchor has its own budget.
	_, err := h.Store.CreateMany(ctx, anchor, channels.NewChannel{Type: "other", Title: "", By: Alice, At: T0})
	require.NoError(t, err)
	_, err = h.Store.CreateMany(ctx, channels.Anchor{ID: "TKT-1", Face: "draft"}, channels.NewChannel{
		Type: "topic", Title: "", By: Alice, At: T0,
	})
	require.NoError(t, err)

	list, err := h.Store.List(ctx, anchor, "topic", "")
	require.NoError(t, err)
	require.Len(t, list, channels.MaxChannelsPerType+1)
}

// testCreateManyMax checks that a caller's lower cap holds under
// concurrent creates and counts only the channels of the type.
func testCreateManyMax(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	anchor := channels.Anchor{ID: "TKT-1"}
	_, err := h.Store.CreateMany(ctx, anchor, channels.NewChannel{Type: "other", By: Alice, At: T0, Max: 3})
	require.NoError(t, err)
	const maxTopics, tries = 3, 12
	var wg sync.WaitGroup
	var made, refused atomic.Int64
	errs := make(chan error, tries)
	for range tries {
		wg.Go(func() {
			_, err := h.Store.CreateMany(ctx, anchor, channels.NewChannel{
				Type: "topic", By: Alice, At: T0, Max: maxTopics,
			})
			switch {
			case err == nil:
				made.Add(1)
			case errors.Is(err, channels.ErrTooManyChannels):
				refused.Add(1)
			default:
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(maxTopics), made.Load(), "the other type does not count")
	require.Equal(t, int64(tries-maxTopics), refused.Load())
	require.Equal(t, channels.MaxChannelsPerType, channels.NewChannel{Max: channels.MaxChannelsPerType + 1}.Cap())
	require.Equal(t, channels.MaxChannelsPerType, channels.NewChannel{}.Cap())
}

func testList(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := channels.Anchor{ID: "TKT-1"}
	draft := channels.Anchor{ID: "TKT-1", Face: "draft"}
	mustAppend(t, h.Store, channels.Key{Anchor: fam, Type: "room"}, text("a"))
	mustAppend(t, h.Store, channels.Key{Anchor: fam, Type: "alerts"}, text("b"))
	mustAppend(t, h.Store, channels.Key{Anchor: draft, Type: "room"}, text("c"))
	mustAppend(t, h.Store, key("TKT-2", "room"), text("d"))
	topic, err := h.Store.CreateMany(ctx, fam, channels.NewChannel{Type: "topic", Title: "t", By: Alice, At: T0})
	require.NoError(t, err)
	_, err = h.Store.SetLifecycle(ctx, channels.Key{Anchor: fam, Type: "alerts"}, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Alice, At: T0})
	require.NoError(t, err)

	keysOf := func(cs []channels.Channel) []string {
		out := make([]string, len(cs))
		for i, c := range cs {
			out[i] = c.Key.String()
		}
		return out
	}
	got, err := h.Store.List(ctx, fam, "", "")
	require.NoError(t, err)
	require.Equal(t, []string{"TKT-1/alerts", "TKT-1/room", topic.Key.String()}, keysOf(got))
	got, err = h.Store.List(ctx, fam, "room", "")
	require.NoError(t, err)
	require.Equal(t, []string{"TKT-1/room"}, keysOf(got))
	got, err = h.Store.List(ctx, fam, "", channels.StateClosed)
	require.NoError(t, err)
	require.Equal(t, []string{"TKT-1/alerts"}, keysOf(got))
	got, err = h.Store.List(ctx, draft, "", "")
	require.NoError(t, err)
	require.Equal(t, []string{"TKT-1@draft/room"}, keysOf(got))
	got, err = h.Store.List(ctx, channels.Anchor{ID: "TKT-404"}, "", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)

	got, err = h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Equal(t, []string{"TKT-1/alerts", "TKT-1/room", topic.Key.String(), "TKT-1@draft/room"}, keysOf(got))
	for _, c := range got {
		if c.Key.Type == "room" && c.Key.Face == "" {
			require.Equal(t, int64(1), c.LastSeq)
		}
	}
}

func testDelete(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	const payload = "deleted-payload-e8b2"
	mustAppend(t, h.Store, k, text(payload))
	mustAppend(t, h.Store, k, text("more"))
	keep := key("TKT-1", "other")
	mustAppend(t, h.Store, keep, text("stays"))

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	del := channels.Deletion{Expect: head.State, Rev: head.Rev, By: Bob, At: T0.Add(time.Hour)}
	require.NoError(t, h.Store.Delete(ctx, k, del))
	_, err = h.Store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.Messages(ctx, k, channels.Page{})
	require.ErrorIs(t, err, channels.ErrNotFound)
	require.NotContains(t, string(h.Raw(t)), payload, "a deleted channel's content is gone from storage")
	require.ErrorIs(t, h.Store.Delete(ctx, k, del), channels.ErrNotFound)

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	last := audit[len(audit)-1]
	require.Equal(t, channels.AuditDelete, last.Op)
	require.Equal(t, "room", last.Type)
	require.Equal(t, Bob, last.By)

	m := mustAppend(t, h.Store, k, text("again"))
	require.Equal(t, int64(1), m.Seq, "a channel re-created after a delete starts over")
	got, err := h.Store.Messages(ctx, keep, channels.Page{})
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// testDeleteCAS pins that a delete compares the state and rev the caller
// read: a lifecycle change or a new message since refuses it, and a refused
// delete deletes and records nothing.
func testDeleteCAS(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("one"))
	read, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	at := T0.Add(time.Hour)

	mustAppend(t, h.Store, k, text("two"))
	require.ErrorIs(t, h.Store.Delete(ctx, k, channels.Deletion{Expect: read.State, Rev: read.Rev, By: Bob, At: at}),
		channels.ErrConflict, "a new message bumps the rev")

	cur, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.ErrorIs(t, h.Store.Delete(ctx, k,
		channels.Deletion{Expect: channels.StateClosed, Rev: cur.Rev, By: Bob, At: at}), channels.ErrConflict)

	closed, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Minute)})
	require.NoError(t, err)
	require.ErrorIs(t, h.Store.Delete(ctx, k,
		channels.Deletion{Expect: channels.StateOpen, Rev: cur.Rev, By: Bob, At: at}),
		channels.ErrConflict, "a channel closed since the read")

	_, err = h.Store.Head(ctx, k)
	require.NoError(t, err, "a refused delete deletes nothing")
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	for _, r := range trail {
		require.NotEqual(t, channels.AuditDelete, r.Op, "a refused delete records nothing")
	}

	// Reopened and closed again: the state is as read, only the rev moved.
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed,
		channels.Transition{To: channels.StateOpen, By: Bob, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)
	reclosed, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(3 * time.Minute)})
	require.NoError(t, err)
	require.Equal(t, closed.State, reclosed.State)
	require.ErrorIs(t, h.Store.Delete(ctx, k,
		channels.Deletion{Expect: closed.State, Rev: closed.Rev, By: Bob, At: at}),
		channels.ErrConflict, "a channel reopened and closed since the read, at the same state")

	require.ErrorIs(t, h.Store.Delete(ctx, k, channels.Deletion{Expect: "gone", By: Bob, At: at}),
		channels.ErrInvalidMessage)
	require.ErrorIs(t, h.Store.Delete(ctx, k, channels.Deletion{Expect: reclosed.State, Rev: reclosed.Rev, At: at}),
		channels.ErrInvalidMessage, "a delete needs an author")
	require.NoError(t, h.Store.Delete(ctx, k,
		channels.Deletion{Expect: reclosed.State, Rev: reclosed.Rev, By: Bob, At: at}))
}

// testDeleteRacesAppend races a delete against an append, through a second
// instance where the backend has one. Either the append lands first and
// the delete is refused, or the delete lands first and the append starts
// the channel over. The append is never lost to the delete.
func testDeleteRacesAppend(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	other := h.Store
	if h.Second != nil {
		other = h.Second(t)
	}
	const rounds = 20
	for i := range rounds {
		k := key(fmt.Sprintf("TKT-%d", i+1), "room")
		mustAppend(t, h.Store, k, text("first"))
		read, err := h.Store.Head(ctx, k)
		require.NoError(t, err)
		var delErr, appErr error
		var wg sync.WaitGroup
		wg.Go(func() {
			delErr = h.Store.Delete(ctx, k,
				channels.Deletion{Expect: read.State, Rev: read.Rev, By: Bob, At: T0.Add(time.Hour)})
		})
		wg.Go(func() {
			_, appErr = other.Append(ctx, k, text("racing"))
		})
		wg.Wait()
		require.NoError(t, appErr)
		head, err := h.Store.Head(ctx, k)
		require.NoError(t, err, "the append is never lost to the delete")
		if delErr == nil {
			require.Equal(t, int64(1), head.LastSeq, "a delete that won leaves only the later append")
			continue
		}
		require.ErrorIs(t, delErr, channels.ErrConflict)
		require.Equal(t, int64(2), head.LastSeq, "a refused delete deletes nothing")
	}
}

func testRekey(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("OLD-1", "room")
	face := channels.Key{Anchor: channels.Anchor{ID: "OLD-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("one"))
	mustAppend(t, h.Store, fam, text("two"))
	mustAppend(t, h.Store, face, text("draft"))
	topic, err := h.Store.CreateMany(ctx, fam.Anchor, channels.NewChannel{Type: "topic", Title: "t", By: Alice, At: T0})
	require.NoError(t, err)
	mustAppend(t, h.Store, key("OLD-10", "room"), text("prefix neighbor"))

	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(time.Hour)))

	got, err := h.Store.ListEntity(ctx, "OLD-1")
	require.NoError(t, err)
	require.Empty(t, got, "nothing is found under the old id")
	_, err = h.Store.Head(ctx, fam)
	require.ErrorIs(t, err, channels.ErrNotFound)

	got, err = h.Store.ListEntity(ctx, "NEW-1")
	require.NoError(t, err)
	require.Len(t, got, 3)
	for _, c := range got {
		require.Equal(t, "NEW-1", c.Key.ID)
	}
	moved := key("NEW-1", "room")
	msgs, err := h.Store.Messages(ctx, moved, channels.Page{})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, seqs(msgs))
	m := mustAppend(t, h.Store, moved, text("three"))
	require.Equal(t, int64(3), m.Seq, "seq continues across a rename")
	movedTopic := topic.Key
	movedTopic.ID = "NEW-1"
	_, err = h.Store.Head(ctx, movedTopic)
	require.NoError(t, err)

	neighbor, err := h.Store.ListEntity(ctx, "OLD-10")
	require.NoError(t, err)
	require.Len(t, neighbor, 1, "an id that merely starts with the old id is untouched")

	audit, err := h.Store.Audit(ctx, "NEW-1")
	require.NoError(t, err)
	require.Equal(t, channels.AuditCreate, audit[0].Op, "the audit trail moves with the channels")
	last := audit[len(audit)-1]
	require.Equal(t, channels.AuditRekey, last.Op)
	require.Equal(t, "OLD-1", last.From)
	require.Equal(t, "NEW-1", last.To)
	old, err := h.Store.Audit(ctx, "OLD-1")
	require.NoError(t, err)
	require.Empty(t, old)

	require.NoError(t, h.Store.Rekey(ctx, "NONE-1", "NONE-2", T0), "an id without channels is not an error")
	require.ErrorIs(t, h.Store.Rekey(ctx, "NEW-1", "NEW-1", T0), channels.ErrInvalidKey)
	require.ErrorIs(t, h.Store.Rekey(ctx, "../x", "NEW-2", T0), channels.ErrInvalidKey)
}

func testRekeyConflict(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	mustAppend(t, h.Store, key("A-1", "room"), text("a"))
	mustAppend(t, h.Store, key("B-1", "other"), text("b"))
	err := h.Store.Rekey(ctx, "A-1", "B-1", T0)
	require.ErrorIs(t, err, channels.ErrRekeyConflict)
	a, err := h.Store.ListEntity(ctx, "A-1")
	require.NoError(t, err)
	require.Len(t, a, 1, "a refused rekey moves nothing")
	b, err := h.Store.ListEntity(ctx, "B-1")
	require.NoError(t, err)
	require.Len(t, b, 1)

	// The occupant tombstoned first, the rekey goes through.
	tombstone(t, h.Store, "B-1", "", T0)
	require.NoError(t, h.Store.Rekey(ctx, "A-1", "B-1", T0))
	b, err = h.Store.ListEntity(ctx, "B-1")
	require.NoError(t, err)
	require.Len(t, b, 1)
	require.Equal(t, "room", b[0].Key.Type)
}

func testTombstoneEntity(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("TKT-1", "room")
	face := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("a"))
	mustAppend(t, h.Store, fam, text("b"))
	mustAppend(t, h.Store, face, text("c"))
	mustAppend(t, h.Store, key("TKT-2", "room"), text("other"))

	tombstone(t, h.Store, "TKT-1", "", T0.Add(time.Hour))
	got, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Empty(t, got)
	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Empty(t, audit, "a reused id starts with a clean audit trail")
	keys, err := h.Store.Keys(ctx)
	require.NoError(t, err)
	require.Equal(t, []channels.Key{key("TKT-2", "room")}, keys)

	m := mustAppend(t, h.Store, fam, text("new entity"))
	require.Equal(t, int64(1), m.Seq, "a new entity on the id gets a fresh channel")

	// Twice in a row, and on an id without channels, is not an error.
	tombstone(t, h.Store, "TKT-1", "", T0.Add(time.Hour))
	tombstone(t, h.Store, "TKT-1", "", T0.Add(time.Hour))
	tombstone(t, h.Store, "NONE-1", "", T0)
	requireTombstoneErr(t, channels.ErrInvalidKey)(h.Store.Tombstone(ctx, "../x", "", T0, nil))
}

func testTombstoneFace(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("TKT-1", "room")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	live := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "live"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("family"))
	mustAppend(t, h.Store, draft, text("draft"))
	mustAppend(t, h.Store, live, text("live"))

	tombstone(t, h.Store, "TKT-1", "draft", T0.Add(time.Hour))
	got, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, fam, got[0].Key)
	require.Equal(t, live, got[1].Key)
	_, err = h.Store.Head(ctx, draft)
	require.ErrorIs(t, err, channels.ErrNotFound)

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	last := audit[len(audit)-1]
	require.Equal(t, channels.AuditTombstone, last.Op)
	require.Equal(t, entity.Face("draft"), last.Face)
	requireTombstoneErr(t, channels.ErrInvalidKey)(h.Store.Tombstone(ctx, "TKT-1", "Bad Face", T0, nil))
}

func testCaps(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	big := text(strings.Repeat("x", channels.MaxPropsBytes))
	_, err := h.Store.Append(ctx, k, big)
	require.ErrorIs(t, err, channels.ErrTooLarge)

	many := map[string]any{}
	for i := range channels.MaxProps + 1 {
		many[fmt.Sprintf("p%d", i)] = i
	}
	_, err = h.Store.Append(ctx, k, channels.NewMessage{Type: "text", Author: Alice, At: T0, Props: many})
	require.ErrorIs(t, err, channels.ErrTooLarge)
	delete(many, "p0")
	_, err = h.Store.Append(ctx, k, channels.NewMessage{Type: "text", Author: Alice, At: T0, Props: many})
	require.NoError(t, err, "exactly the cap is allowed")

	bad := text("x")
	bad.Type = "Not A Name"
	_, err = h.Store.Append(ctx, k, bad)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	anon := text("x")
	anon.Author = channels.Author{}
	_, err = h.Store.Append(ctx, k, anon)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	untimed := text("x")
	untimed.At = time.Time{}
	_, err = h.Store.Append(ctx, k, untimed)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)

	_, err = h.Store.Edit(ctx, k, 1, many, T0)
	require.NoError(t, err)
	_, err = h.Store.Edit(ctx, k, 1, map[string]any{"body": strings.Repeat("x", channels.MaxPropsBytes)}, T0)
	require.ErrorIs(t, err, channels.ErrTooLarge)

	_, err = h.Store.Remove(ctx, k, 1, channels.Removal{
		By: Alice, At: T0, Reason: strings.Repeat("é", channels.MaxReasonRunes+1),
	})
	require.ErrorIs(t, err, channels.ErrTooLarge)
	_, err = h.Store.Remove(ctx, k, 1, channels.Removal{At: T0})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.LastSeq, "refused appends take no seq")
}

func testInvalidKeys(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	bad := []channels.Key{
		key("../etc", "room"),
		key("", "room"),
		key("TKT-1", ""),
		key("TKT-1", "Room"),
		key("TKT-1", "con"),
		key("TKT-1", "a/b"),
		{Anchor: channels.Anchor{ID: "TKT-1", Face: "Bad"}, Type: "room"},
		{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room", Name: "../x"},
		{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room", Name: "UPPER"},
	}
	for _, k := range bad {
		t.Run(k.String(), func(t *testing.T) {
			_, err := h.Store.Append(ctx, k, text("x"))
			require.ErrorIs(t, err, channels.ErrInvalidKey)
			_, err = h.Store.Head(ctx, k)
			require.ErrorIs(t, err, channels.ErrInvalidKey)
			_, err = h.Store.Messages(ctx, k, channels.Page{})
			require.ErrorIs(t, err, channels.ErrInvalidKey)
		})
	}
	_, err := h.Store.List(ctx, channels.Anchor{ID: "../x"}, "", "")
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = h.Store.ListEntity(ctx, "a b")
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = h.Store.Audit(ctx, "")
	require.ErrorIs(t, err, channels.ErrInvalidKey)
}

func testKeys(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	keys, err := h.Store.Keys(ctx)
	require.NoError(t, err)
	require.NotNil(t, keys)
	require.Empty(t, keys)

	want := make([]channels.Key, 0, 4)
	want = append(want,
		key("A-1", "room"),
		channels.Key{Anchor: channels.Anchor{ID: "A-1", Face: "draft"}, Type: "room"},
		key("B-2", "alerts"),
	)
	for _, k := range want {
		mustAppend(t, h.Store, k, text("x"))
	}
	topic, err := h.Store.CreateMany(ctx, channels.Anchor{ID: "B-2"}, channels.NewChannel{
		Type: "topic", Title: "", By: Alice, At: T0,
	})
	require.NoError(t, err)
	want = append(want, topic.Key)
	channels.SortKeys(want)
	keys, err = h.Store.Keys(ctx)
	require.NoError(t, err)
	require.Equal(t, want, keys)
}

func testTimes(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	local := time.Date(2026, 1, 2, 3, 4, 5, 999999999, time.FixedZone("CET", 3600))
	m := text("x")
	m.At = local
	got := mustAppend(t, h.Store, k, m)
	require.Equal(t, channels.NormTime(local), got.At)
	read, err := h.Store.Get(ctx, k, got.Seq)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(local), read.At)
	require.Equal(t, time.UTC, read.At.Location())
}

func testSecondInstance(t *testing.T, h Harness) {
	t.Helper()
	if h.Second == nil {
		t.Skip("backend admits one process only")
	}
	ctx := context.Background()
	other := h.Second(t)
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("from one"))
	head, err := other.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.LastSeq, "the second instance sees the first's write")
	m := mustAppend(t, other, k, text("from two"))
	require.Equal(t, int64(2), m.Seq, "the second instance never trusts a cached seq")
	got, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, seqs(got))
}

// testTypeRecords checks that the type records persist, that a record
// replaces only the types it names, that invalid input records nothing, and
// that channel deletion and tombstones leave the records alone.
func testTypeRecords(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	got, err := h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)

	famOne := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne}
	faceOne := channels.TypeRecord{Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne}
	famMany := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityMany}
	require.NoError(t, h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{
		"room": famOne, "review": faceOne,
	}))
	require.NoError(t, h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{
		"room": famOne, "alerts": famMany, "review": faceOne,
	}))
	want := map[string]channels.TypeRecord{"room": famOne, "review": faceOne, "alerts": famMany}
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got, "a record adds the types it names and keeps the others")

	err = h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{
		"topic": {Anchor: "both", Cardinality: channels.CardinalityOne},
	})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	err = h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{
		"topic": {Anchor: channels.AnchorFace, Cardinality: "few"},
	})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	err = h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"topic": faceOne, "Bad Type": faceOne})
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	require.NoError(t, h.Store.RecordTypes(ctx, nil))

	mustAppend(t, h.Store, key("TKT-1", "room"), text("x"))
	tombstone(t, h.Store, "TKT-1", "", T0)
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got, "invalid input records nothing; tombstones leave the records")
	keys, err := h.Store.Keys(ctx)
	require.NoError(t, err)
	require.Empty(t, keys, "a record is not a channel")

	if h.Second != nil {
		got, err = h.Second(t).TypeRecords(ctx)
		require.NoError(t, err)
		require.Equal(t, want, got, "another instance reads the same records")
	}
}

// testTypeRecordConflict checks that a record never replaces a held anchor
// mode or cardinality: the whole call is refused with
// [channels.ErrTypeShapeConflict] and records nothing.
func testTypeRecordConflict(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	famOne := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne}
	faceOne := channels.TypeRecord{Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne}
	famMany := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityMany}
	require.NoError(t, h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"room": famOne}))
	for _, other := range []channels.TypeRecord{faceOne, famMany} {
		err := h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"room": other, "alerts": famOne})
		require.ErrorIs(t, err, channels.ErrTypeShapeConflict)
	}
	got, err := h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]channels.TypeRecord{"room": famOne}, got,
		"a conflict records nothing, not even the other types of the call")
}

// testTypeRecordRefs checks that refs only grow, and that a record declaring
// a held ref as plain is refused.
func testTypeRecordRefs(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	rec := func(refs, plain []string) map[string]channels.TypeRecord {
		return map[string]channels.TypeRecord{"room": {
			Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne, Refs: refs, Plain: plain,
		}}
	}
	require.NoError(t, h.Store.RecordTypes(ctx, rec([]string{"note.about"}, []string{"text.body"})))
	require.NoError(t, h.Store.RecordTypes(ctx, rec([]string{"note.also", "note.about"}, nil)))
	require.NoError(t, h.Store.RecordTypes(ctx, rec(nil, []string{"text.body"})),
		"a record that drops a ref keeps it")
	got, err := h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	want := channels.TypeRecord{
		Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne, Refs: []string{"note.about", "note.also"},
	}
	require.Equal(t, want, got["room"], "refs grow, sorted; plain is not stored")

	err = h.Store.RecordTypes(ctx, rec([]string{"note.about"}, []string{"note.also"}))
	require.ErrorIs(t, err, channels.ErrTypeShapeConflict)
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got["room"])
	if h.Second != nil {
		got, err = h.Second(t).TypeRecords(ctx)
		require.NoError(t, err)
		require.Equal(t, want, got["room"], "another instance reads the same refs")
	}
}

// testTypeRecordConcurrent checks that of concurrent records with two
// different shapes, one shape wins and every record of the other conflicts.
func testTypeRecordConcurrent(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	recs := []channels.TypeRecord{
		{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne},
		{Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne},
	}
	const tries = 8
	var wg sync.WaitGroup
	var conflicts atomic.Int64
	errs := make(chan error, tries)
	for i := range tries {
		wg.Go(func() {
			err := h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"room": recs[i%2]})
			switch {
			case errors.Is(err, channels.ErrTypeShapeConflict):
				conflicts.Add(1)
			case err != nil:
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(tries/2), conflicts.Load(), "every record of the losing shape conflicts")
	got, err := h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Contains(t, recs, got["room"])
}

// testLegacyTypeRecord checks a record written before cardinality was kept.
// It reads with an unknown cardinality and no refs, and it matches any
// configured cardinality but not another anchor. The next RecordTypes for
// the type fills the cardinality in and adds its refs; a record with
// another anchor still conflicts and records nothing. Once filled in, a
// different cardinality conflicts again.
func testLegacyTypeRecord(t *testing.T, h Harness) {
	t.Helper()
	if h.SeedLegacyType == nil {
		t.Skip("backend has no legacy type record")
	}
	ctx := context.Background()
	h.SeedLegacyType(t, "room", channels.AnchorFamily)
	h.SeedLegacyType(t, "review", channels.AnchorFace)

	legacyRoom := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityUnknown}
	legacyReview := channels.TypeRecord{Anchor: channels.AnchorFace, Cardinality: channels.CardinalityUnknown}
	legacy := map[string]channels.TypeRecord{"room": legacyRoom, "review": legacyReview}
	got, err := h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, legacy, got)

	famMany := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityMany}
	famOne := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne}
	faceOne := channels.TypeRecord{Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne}
	require.Empty(t, channels.TypeRecordChanges(got, map[string]channels.TypeRecord{"room": famMany}),
		"an unknown cardinality matches any configured one")
	require.Empty(t, channels.TypeRecordChanges(got, map[string]channels.TypeRecord{"room": famOne}),
		"an unknown cardinality matches any configured one")
	require.Len(t, channels.TypeRecordChanges(got, map[string]channels.TypeRecord{"review": famOne}), 1,
		"the anchor mode is still compared")

	err = h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"room": famMany, "review": famOne})
	require.ErrorIs(t, err, channels.ErrTypeShapeConflict, "an unknown cardinality does not excuse another anchor")
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, legacy, got, "a conflict fills in nothing, not even the other types of the call")

	roomRefs := famMany
	roomRefs.Refs = []string{"note.about"}
	roomRefs.Plain = []string{"text.body"}
	require.NoError(t, h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"room": roomRefs}))
	filledRoom := famMany
	filledRoom.Refs = []string{"note.about"}
	want := map[string]channels.TypeRecord{"room": filledRoom, "review": legacyReview}
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got, "RecordTypes fills in the cardinality and refs of the type it names only")
	require.Len(t, channels.TypeRecordChanges(got, map[string]channels.TypeRecord{"room": famOne}), 1,
		"a recorded cardinality conflicts with another")
	err = h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"room": famOne})
	require.ErrorIs(t, err, channels.ErrTypeShapeConflict, "a filled-in cardinality is held")
	if h.Second != nil {
		got, err = h.Second(t).TypeRecords(ctx)
		require.NoError(t, err)
		require.Equal(t, want, got, "another instance reads the filled-in record")
	}

	require.NoError(t, h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{"review": faceOne}))
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]channels.TypeRecord{"room": filledRoom, "review": faceOne}, got,
		"a fill without refs still writes the cardinality")
}

// Contains reports whether raw holds needle; a helper for backend tests
// that inspect raw bytes themselves.
func Contains(raw []byte, needle string) bool { return bytes.Contains(raw, []byte(needle)) }

// testIncarnation pins the id-reuse guard: a channel records the
// incarnation it was created in, keeps it across a rekey, and refuses an
// append from another incarnation without writing anything.
func testIncarnation(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	life1, life2 := text("one"), text("two")
	life1.Incarnation, life2.Incarnation = "life-1", "life-2"

	mustAppend(t, h.Store, k, life1)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, "life-1", head.Incarnation)
	require.NoError(t, channels.CheckIncarnation(head, "life-1"))
	require.ErrorIs(t, channels.CheckIncarnation(head, "life-2"), channels.ErrNotFound)

	_, err = h.Store.Append(ctx, k, life2)
	require.ErrorIs(t, err, channels.ErrStaleAnchor)
	_, err = h.Store.Append(ctx, k, text("no incarnation"))
	require.ErrorIs(t, err, channels.ErrStaleAnchor)
	after, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, head, after, "a refused append writes nothing")
	mustAppend(t, h.Store, k, life1)

	topic, err := h.Store.CreateMany(ctx, k.Anchor, channels.NewChannel{
		Type: "topic", Incarnation: "life-1", By: Alice, At: T0,
	})
	require.NoError(t, err)
	require.Equal(t, "life-1", topic.Incarnation)
	listed, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	for _, c := range listed {
		require.Equal(t, "life-1", c.Incarnation, c.Key.String())
	}

	require.NoError(t, h.Store.Rekey(ctx, "TKT-1", "TKT-2", T0))
	moved, err := h.Store.Head(ctx, key("TKT-2", "room"))
	require.NoError(t, err)
	require.Equal(t, "life-1", moved.Incarnation, "a rename keeps the entity, so it keeps the incarnation")

	// A new entity on the id starts a new channel once the leftover is
	// tombstoned.
	tombstone(t, h.Store, "TKT-2", "", T0)
	fresh := mustAppend(t, h.Store, key("TKT-2", "room"), life2)
	require.Equal(t, int64(1), fresh.Seq)
	head, err = h.Store.Head(ctx, key("TKT-2", "room"))
	require.NoError(t, err)
	require.Equal(t, "life-2", head.Incarnation)

	long := text("x")
	long.Incarnation = strings.Repeat("i", channels.MaxTextBytes+1)
	_, err = h.Store.Append(ctx, key("TKT-3", "room"), long)
	require.ErrorIs(t, err, channels.ErrTooLarge)
}

// testArchiveFromOpen: archiving an open channel closes it in the same
// instant, so the retention sweep that reads the close time sees one.
func testArchiveFromOpen(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("x"))
	at := T0.Add(time.Hour)
	c, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateArchived, By: Bob, At: at})
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(at), c.ClosedAt)
	require.Equal(t, channels.NormTime(at), c.ArchivedAt)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, c, head)
}

// testAfterRevPaging walks a channel by rev in small pages after changes to
// old and new messages, and must see every change once, in rev order.
func testAfterRevPaging(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	for i := range 12 {
		mustAppend(t, h.Store, k, text(fmt.Sprintf("m%d", i+1)))
	}
	start, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	for _, seq := range []int64{11, 2, 7, 1} {
		_, err = h.Store.Edit(ctx, k, seq, map[string]any{"body": "edited"}, T0)
		require.NoError(t, err)
	}
	mustAppend(t, h.Store, k, text("m13"))

	var walked []int64
	cursor := start.Rev
	for {
		page, pageErr := h.Store.Messages(ctx, k, channels.Page{AfterRev: cursor, Limit: 2})
		require.NoError(t, pageErr)
		if len(page) == 0 {
			break
		}
		require.LessOrEqual(t, len(page), 2)
		for _, m := range page {
			require.Greater(t, m.Rev, cursor)
			cursor = m.Rev
			walked = append(walked, m.Seq)
		}
	}
	require.Equal(t, []int64{11, 2, 7, 1, 13}, walked)
}

// testManyReplies sends enough replies to one message that a backend which
// patches the parent in place must compact, and checks the count and the
// rev cursor survive it.
func testManyReplies(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	parent := mustAppend(t, h.Store, k, text("parent"))
	const n = 30
	for range n {
		reply := text("reply")
		reply.ReplyTo = parent.Seq
		mustAppend(t, h.Store, k, reply)
	}
	got, err := h.Store.Get(ctx, k, parent.Seq)
	require.NoError(t, err)
	require.Equal(t, n, got.ReplyCount)
	require.Equal(t, "parent", got.Props["body"])
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(n+1), head.LastSeq)
	require.Equal(t, head.Rev, got.Rev, "the last reply's patch is the newest change")
	page, err := h.Store.Messages(ctx, k, channels.Page{AfterRev: head.Rev - 1})
	require.NoError(t, err)
	require.Equal(t, []int64{parent.Seq}, seqs(page))
}

// testLongestAnchor uses the longest id and face the caps admit, through
// every operation that names them, including the tombstone, whose stored
// name is the longest.
func testLongestAnchor(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	id := strings.Repeat("A", channels.MaxAnchorIDBytes)
	face := entity.Face("f" + strings.Repeat("a", channels.MaxFaceBytes-1))
	k := channels.Key{Anchor: channels.Anchor{ID: id, Face: face}, Type: strings.Repeat("t", 63)}
	mustAppend(t, h.Store, k, text("x"))
	_, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	tombstone(t, h.Store, id, face, T0)
	mustAppend(t, h.Store, k, text("y"))
	tombstone(t, h.Store, id, "", T0)
	left, err := h.Store.ListEntity(ctx, id)
	require.NoError(t, err)
	require.Empty(t, left)
}

// testUntombstoneBatch tombstones one id twice, each time over different
// channels, and checks that each handle restores its own batch: the
// messages, the audit trail filed with it, and a record of the restore.
func testUntombstoneBatch(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	first := key("TKT-1", "room")
	second := key("TKT-1", "notes")
	_, err := h.Store.CreateMany(ctx, first.Anchor, channels.NewChannel{Type: "topic", By: Alice, At: T0})
	require.NoError(t, err)
	mustAppend(t, h.Store, first, text("first life"))
	h1 := tombstone(t, h.Store, "TKT-1", "", T0.Add(time.Minute))
	require.False(t, h1.IsZero())

	mustAppend(t, h.Store, second, text("second life"))
	h2 := tombstone(t, h.Store, "TKT-1", "", T0.Add(2*time.Minute))
	require.False(t, h2.IsZero())
	require.NotEqual(t, h1, h2)

	// The first batch comes back alone.
	require.NoError(t, h.Store.Untombstone(ctx, h1, T0.Add(3*time.Minute), nil))
	got, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, c := range got {
		require.NotEqual(t, "notes", c.Key.Type, "the second batch stays tombstoned")
	}
	msgs, err := h.Store.Messages(ctx, first, channels.Page{})
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "first life", msgs[0].Props["body"])

	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	ops := make([]channels.AuditOp, len(trail))
	for i, r := range trail {
		ops[i] = r.Op
	}
	require.Equal(t, []channels.AuditOp{
		channels.AuditCreate, channels.AuditCreate, channels.AuditTombstone, channels.AuditUntombstone,
	}, ops, "the trail filed with the batch comes back, and the restore is recorded")

	// A batch is restored once.
	require.ErrorIs(t, h.Store.Untombstone(ctx, h1, T0, nil), channels.ErrNotFound)
	// The zero handle is a no-op.
	require.NoError(t, h.Store.Untombstone(ctx, channels.TombstoneHandle{}, T0, nil))
	empty := tombstone(t, h.Store, "NONE-1", "", T0)
	require.True(t, empty.IsZero(), "an id without channels files no batch")
}

// testUntombstoneFace restores a face batch next to the family channel,
// which never moved.
func testUntombstoneFace(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("TKT-1", "room")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("family"))
	mustAppend(t, h.Store, draft, text("draft"))
	hd := tombstone(t, h.Store, "TKT-1", "draft", T0.Add(time.Minute))
	require.Equal(t, channels.Anchor{ID: "TKT-1", Face: "draft"}, hd.Anchor())
	none := tombstone(t, h.Store, "TKT-1", "live", T0)
	require.True(t, none.IsZero(), "a face without channels files no batch")

	require.NoError(t, h.Store.Untombstone(ctx, hd, T0.Add(2*time.Minute), nil))
	got, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, fam, got[0].Key)
	require.Equal(t, draft, got[1].Key)
	msgs, err := h.Store.Messages(ctx, draft, channels.Page{})
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	last := trail[len(trail)-1]
	require.Equal(t, channels.AuditUntombstone, last.Op)
	require.Equal(t, entity.Face("draft"), last.Face)
}

// testUntombstoneOccupied checks that a restore onto an anchor that holds
// live channels again is refused and changes nothing, on the whole id and
// on one face.
func testUntombstoneOccupied(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("TKT-1", "room")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("old"))
	hf := tombstone(t, h.Store, "TKT-1", "", T0)
	mustAppend(t, h.Store, key("TKT-1", "other"), text("new occupant"))

	err := h.Store.Untombstone(ctx, hf, T0, nil)
	var occ *channels.OccupiedError
	require.ErrorAs(t, err, &occ)
	require.Equal(t, channels.Anchor{ID: "TKT-1"}, occ.Anchor)
	got, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, got, 1, "a refused restore moves nothing")
	require.Equal(t, "other", got[0].Key.Type)

	mustAppend(t, h.Store, draft, text("draft old"))
	hd := tombstone(t, h.Store, "TKT-1", "draft", T0)
	mustAppend(t, h.Store, draft, text("draft new"))
	err = h.Store.Untombstone(ctx, hd, T0, nil)
	require.ErrorAs(t, err, &occ)
	require.Equal(t, entity.Face("draft"), occ.Anchor.Face)
	msgs, err := h.Store.Messages(ctx, draft, channels.Page{})
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "draft new", msgs[0].Props["body"], "the live face channel is untouched")

	// Once the occupant is gone, the same handle restores.
	tombstone(t, h.Store, "TKT-1", "", T0)
	require.NoError(t, h.Store.Untombstone(ctx, hf, T0, nil))
	head, err := h.Store.Head(ctx, fam)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.LastSeq)
}

// testHandleRoundTrip restores a batch through the text form of its handle,
// and refuses text that is not a handle.
func testHandleRoundTrip(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	face := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, face, text("x"))
	orig := tombstone(t, h.Store, "TKT-1", "draft", T0)

	parsed, err := channels.ParseTombstoneHandle(orig.String())
	require.NoError(t, err)
	require.Equal(t, orig, parsed)
	require.NoError(t, h.Store.Untombstone(ctx, parsed, T0, nil))
	_, err = h.Store.Head(ctx, face)
	require.NoError(t, err)

	zero, err := channels.ParseTombstoneHandle("")
	require.NoError(t, err)
	require.True(t, zero.IsZero())
	for _, bad := range []string{"x", "t1:a:b", "t2:" + strings.TrimPrefix(orig.String(), "t1:"),
		strings.Replace(orig.String(), orig.Stamp(), "../../etc", 1)} {
		_, err = channels.ParseTombstoneHandle(bad)
		require.ErrorIs(t, err, channels.ErrInvalidKey, bad)
	}
}
