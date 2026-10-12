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
	tests := []struct {
		name string
		fn   func(t *testing.T, h Harness)
	}{
		{"ImplicitChannelOnFirstAppend", testImplicitCreate},
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
		{"Lifecycle", testLifecycle},
		{"CreateMany", testCreateMany},
		{"CreateManyCap", testCreateManyCap},
		{"ListAndListEntity", testList},
		{"DeleteKeepsAudit", testDelete},
		{"Rekey", testRekey},
		{"RekeyConflict", testRekeyConflict},
		{"TombstoneEntity", testTombstoneEntity},
		{"TombstoneFace", testTombstoneFace},
		{"Caps", testCaps},
		{"InvalidKeys", testInvalidKeys},
		{"Keys", testKeys},
		{"TimesRoundTrip", testTimes},
		{"SecondInstance", testSecondInstance},
		{"AnchorModes", testAnchorModes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t, newHarness(t))
		})
	}
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
	require.ErrorIs(t, err, channels.ErrConflict, "a second accept loses")
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

	// A failed side effect reverts to no action.
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionNone, pending)
	require.NoError(t, err)
	got, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionPending, channels.Action{})
	require.NoError(t, err)
	require.Nil(t, got.Action)
	stale, err = h.Store.StalePending(ctx, at.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, stale)

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	var actions []channels.AuditRecord
	for _, r := range audit {
		if r.Op == channels.AuditAction {
			actions = append(actions, r)
		}
	}
	require.Len(t, actions, 1, "only a completed action is audited")
	require.Equal(t, "accept", actions[0].Action)
	require.Equal(t, int64(1), actions[0].Seq)
	require.Equal(t, &channels.Promoted{ID: "DEC-1", Type: "decision"}, actions[0].Promoted)

	_, err = h.Store.SetAction(ctx, k, 9, "accept", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.SetAction(ctx, k, 1, "Bad Name", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	_, err = h.Store.Remove(ctx, k, 2, channels.Removal{By: Alice, At: at})
	require.NoError(t, err)
	_, err = h.Store.SetAction(ctx, k, 2, "accept", channels.ActionNone, pending)
	require.ErrorIs(t, err, channels.ErrRemoved)
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

	got, err := h.Store.ListByLifecycle(ctx, channels.StateClosed, closedAt.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, k, got[0].Key)
	got, err = h.Store.ListByLifecycle(ctx, channels.StateClosed, closedAt)
	require.NoError(t, err)
	require.Empty(t, got)
	got, err = h.Store.ListByLifecycle(ctx, channels.StateOpen, closedAt.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, got)

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

	audit, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	var trail []string
	for _, r := range audit {
		if r.Op == channels.AuditLifecycle {
			trail = append(trail, r.From+">"+r.To)
		}
	}
	require.Equal(t, []string{"open>closed", "closed>archived", "archived>open"}, trail)
}

func testCreateMany(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	anchor := channels.Anchor{ID: "TKT-1"}
	c, err := h.Store.CreateMany(ctx, anchor, "topic", "Release plan", Alice, T0)
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

	other, err := h.Store.CreateMany(ctx, anchor, "topic", "", Alice, T0)
	require.NoError(t, err)
	require.NotEqual(t, c.Key.Name, other.Key.Name)

	_, err = h.Store.CreateMany(ctx, anchor, "topic", strings.Repeat("é", channels.MaxTitleRunes+1), Alice, T0)
	require.ErrorIs(t, err, channels.ErrTooLarge)
	_, err = h.Store.CreateMany(ctx, anchor, "topic", strings.Repeat("é", channels.MaxTitleRunes), Alice, T0)
	require.NoError(t, err, "the cap counts characters, not bytes")
	_, err = h.Store.CreateMany(ctx, anchor, "Bad", "", Alice, T0)
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = h.Store.CreateMany(ctx, anchor, "topic", "", channels.Author{}, T0)
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
			_, err := h.Store.CreateMany(ctx, anchor, "topic", "", Alice, T0)
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
	_, err := h.Store.CreateMany(ctx, anchor, "other", "", Alice, T0)
	require.NoError(t, err)
	_, err = h.Store.CreateMany(ctx, channels.Anchor{ID: "TKT-1", Face: "draft"}, "topic", "", Alice, T0)
	require.NoError(t, err)

	list, err := h.Store.List(ctx, anchor, "topic", "")
	require.NoError(t, err)
	require.Len(t, list, channels.MaxChannelsPerType+1)
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
	topic, err := h.Store.CreateMany(ctx, fam, "topic", "t", Alice, T0)
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

	require.NoError(t, h.Store.Delete(ctx, k, Bob, T0.Add(time.Hour)))
	_, err := h.Store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.Messages(ctx, k, channels.Page{})
	require.ErrorIs(t, err, channels.ErrNotFound)
	require.NotContains(t, string(h.Raw(t)), payload, "a deleted channel's content is gone from storage")
	require.ErrorIs(t, h.Store.Delete(ctx, k, Bob, T0), channels.ErrNotFound)

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

func testRekey(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("OLD-1", "room")
	face := channels.Key{Anchor: channels.Anchor{ID: "OLD-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("one"))
	mustAppend(t, h.Store, fam, text("two"))
	mustAppend(t, h.Store, face, text("draft"))
	topic, err := h.Store.CreateMany(ctx, fam.Anchor, "topic", "t", Alice, T0)
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
	require.NoError(t, h.Store.Tombstone(ctx, "B-1", "", T0))
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

	require.NoError(t, h.Store.Tombstone(ctx, "TKT-1", "", T0.Add(time.Hour)))
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
	require.NoError(t, h.Store.Tombstone(ctx, "TKT-1", "", T0.Add(time.Hour)))
	require.NoError(t, h.Store.Tombstone(ctx, "TKT-1", "", T0.Add(time.Hour)))
	require.NoError(t, h.Store.Tombstone(ctx, "NONE-1", "", T0))
	require.ErrorIs(t, h.Store.Tombstone(ctx, "../x", "", T0), channels.ErrInvalidKey)
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

	require.NoError(t, h.Store.Tombstone(ctx, "TKT-1", "draft", T0.Add(time.Hour)))
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
	require.ErrorIs(t, h.Store.Tombstone(ctx, "TKT-1", "Bad Face", T0), channels.ErrInvalidKey)
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
	topic, err := h.Store.CreateMany(ctx, channels.Anchor{ID: "B-2"}, "topic", "", Alice, T0)
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

// testAnchorModes checks that the recorded anchor modes persist, that a
// record replaces only the types it names, that invalid input records
// nothing, and that channel deletion and tombstones leave the record alone.
func testAnchorModes(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	got, err := h.Store.AnchorModes(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)

	require.NoError(t, h.Store.RecordAnchorModes(ctx, map[string]channels.AnchorMode{
		"room": channels.AnchorFamily, "review": channels.AnchorFace,
	}))
	require.NoError(t, h.Store.RecordAnchorModes(ctx, map[string]channels.AnchorMode{
		"room": channels.AnchorFace, "alerts": channels.AnchorFamily,
	}))
	want := map[string]channels.AnchorMode{
		"room": channels.AnchorFace, "review": channels.AnchorFace, "alerts": channels.AnchorFamily,
	}
	got, err = h.Store.AnchorModes(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got, "a record replaces only the types it names")

	err = h.Store.RecordAnchorModes(ctx, map[string]channels.AnchorMode{"topic": "both"})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	err = h.Store.RecordAnchorModes(ctx, map[string]channels.AnchorMode{
		"topic": channels.AnchorFace, "Bad Type": channels.AnchorFace,
	})
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	require.NoError(t, h.Store.RecordAnchorModes(ctx, nil))

	mustAppend(t, h.Store, key("TKT-1", "room"), text("x"))
	require.NoError(t, h.Store.Tombstone(ctx, "TKT-1", "", T0))
	got, err = h.Store.AnchorModes(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got, "invalid input records nothing; tombstones leave the record")
	keys, err := h.Store.Keys(ctx)
	require.NoError(t, err)
	require.Empty(t, keys, "the record is not a channel")

	if h.Second != nil {
		got, err = h.Second(t).AnchorModes(ctx)
		require.NoError(t, err)
		require.Equal(t, want, got, "another instance reads the same record")
	}
}

// Contains reports whether raw holds needle; a helper for backend tests
// that inspect raw bytes themselves.
func Contains(raw []byte, needle string) bool { return bytes.Contains(raw, []byte(needle)) }
