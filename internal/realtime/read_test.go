package realtime

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

func rev(n int64) *int64 { return &n }

// postN posts n text messages as bob, past the rate limit.
func (h *harness) postN(t *testing.T, n int) []MessageView {
	t.Helper()
	out := make([]MessageView, 0, n)
	for i := range n {
		h.local.limiter = newLimiter(defaultLimitCaps()) // keep the rate limit out of the test
		out = append(out, h.post(t, user("bob"), text(fmt.Sprintf("m%d", i+1))))
	}
	return out
}

func readSeqs(v ReadView) []int64 {
	out := []int64{}
	for _, m := range v.Messages {
		out = append(out, m.Seq)
	}
	return out
}

// TestRead_Cursors pins the cursor rule of every page kind.
func TestRead_Cursors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := as(user("bob"))

	empty, err := h.svc.Read(bob, room, ReadRequest{})
	require.NoError(t, err)
	require.Empty(t, empty.Messages)
	require.Equal(t, Cursors{NextAfterRev: rev(0)}, empty.Cursors, "an empty channel can be followed from rev 0")

	posted := h.postN(t, 3)
	headRev := posted[2].Rev

	tests := []struct {
		name    string
		req     ReadRequest
		seqs    []int64
		hasMore bool
		want    Cursors
	}{
		{"newest", ReadRequest{Limit: 2}, []int64{2, 3}, true, Cursors{NextBefore: 2, NextAfterRev: rev(headRev)}},
		{"before", ReadRequest{BeforeSeq: 2, Limit: 2}, []int64{1}, false, Cursors{NextBefore: 1}},
		{"before start", ReadRequest{BeforeSeq: 1}, []int64{}, false, Cursors{}},
		{"after", ReadRequest{AfterSeq: 1, Limit: 1}, []int64{2}, true, Cursors{NextAfter: 2}},
		{"after end", ReadRequest{AfterSeq: 3}, []int64{}, false, Cursors{NextAfter: 3}},
		{"after rev zero", ReadRequest{AfterRev: rev(0)}, []int64{1, 2, 3}, false, Cursors{NextAfterRev: rev(headRev)}},
		{
			"after rev full page", ReadRequest{AfterRev: rev(0), Limit: 1}, []int64{1}, true,
			Cursors{NextAfterRev: rev(posted[0].Rev)},
		},
		{"after rev at head", ReadRequest{AfterRev: rev(headRev)}, []int64{}, false, Cursors{NextAfterRev: rev(headRev)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := h.svc.Read(bob, room, tc.req)
			require.NoError(t, err)
			require.Equal(t, tc.seqs, readSeqs(got))
			require.Equal(t, tc.hasMore, got.HasMore)
			require.Equal(t, tc.want, got.Cursors)
		})
	}

	_, err = h.svc.Edit(bob, room, 1, map[string]any{"body": "edited"})
	require.NoError(t, err)
	got, err := h.svc.Read(bob, room, ReadRequest{AfterRev: rev(headRev)})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, readSeqs(got), "an edit is seen past the rev cursor")
	require.Equal(t, got.Messages[0].Rev, *got.NextAfterRev)

	_, err = h.svc.Read(bob, room, ReadRequest{AfterSeq: 1, AfterRev: rev(0)})
	requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
}

// TestRead_AfterRevMissesNothing pins that following a channel by
// after_rev sees a post that lands between the channel read and the page
// read, and one that lands after the page read.
func TestRead_AfterRevMissesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := as(user("bob"))
	posted := h.postN(t, 1)

	raw := h.store.Store
	var landed atomic.Bool
	h.store.onHead = func() {
		if landed.CompareAndSwap(false, true) {
			_, err := raw.Append(context.Background(), roomKey(), channels.NewMessage{
				Type: "text", Author: channels.Author{User: "carol"}, At: h.clock.Now(),
				Props: map[string]any{"body": "racing"},
			})
			require.NoError(t, err)
		}
	}
	got, err := h.svc.Read(bob, room, ReadRequest{AfterRev: rev(posted[0].Rev)})
	require.NoError(t, err)
	require.Equal(t, []int64{2}, readSeqs(got), "the racing post is in the page")
	h.store.onHead = nil

	h.postN(t, 1)
	next, err := h.svc.Read(bob, room, ReadRequest{AfterRev: got.NextAfterRev})
	require.NoError(t, err)
	require.Equal(t, []int64{3}, readSeqs(next), "a post after the page is seen by the next read")
}

// countingAccess counts the access captures of a service.
type countingAccess struct {
	AccessSource
	n atomic.Int64
}

func (a *countingAccess) For(ctx context.Context) (context.Context, Access, error) {
	a.n.Add(1)
	return a.AccessSource.For(ctx)
}

// countingConfig counts the config snapshots of a service.
type countingConfig struct {
	ConfigSource
	n atomic.Int64
}

func (c *countingConfig) Load() Snapshot {
	c.n.Add(1)
	return c.ConfigSource.Load()
}

// TestRead_OneCaptureFlatCost pins that a read captures its state once and
// that its store, anchor and ref calls do not grow with the page.
func TestRead_OneCaptureFlatCost(t *testing.T) {
	t.Parallel()
	for _, n := range []int{10, 50} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			for i := range n {
				h.local.limiter = newLimiter(defaultLimitCaps())
				h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": i + 1, "also": "DOC-1"}})
			}
			_, err := h.svc.Post(as(user("bob")), room, PostInput{Type: "typing"})
			require.NoError(t, err)

			acc := &countingAccess{AccessSource: fakeAccessSource{}}
			cfg := &countingConfig{ConfigSource: h.cfg}
			svc, err := New(Deps{Store: h.store, Config: cfg, Access: acc, Anchors: h.anchors, Refs: h.refs, Local: h.local})
			require.NoError(t, err)
			h.store.heads.Store(0)
			h.store.messages.Store(0)
			h.store.typeRecs.Store(0)
			h.anchors.calls.Store(0)
			h.refs.calls.Store(0)

			got, err := svc.Read(as(user("bob")), room, ReadRequest{Signals: true, Schemas: true})
			require.NoError(t, err)
			require.Len(t, got.Messages, n)
			require.Len(t, got.Signals, 1)
			require.EqualValues(t, 1, acc.n.Load(), "access captures")
			require.EqualValues(t, 1, cfg.n.Load(), "config snapshots")
			require.EqualValues(t, 1, h.store.heads.Load(), "channel reads")
			require.EqualValues(t, 1, h.store.messages.Load(), "page reads")
			require.LessOrEqual(t, h.store.typeRecs.Load(), int64(1), "type record reads")
			require.EqualValues(t, 1, h.anchors.calls.Load(), "anchor reads")
			require.EqualValues(t, 1, h.refs.calls.Load(), "ref batches")
		})
	}
}

// TestRead_SchemasSignalsAndLogOnly pins the message type descriptions,
// the signals switch, and that a log-only transport offers no signal type.
func TestRead_SchemasSignalsAndLogOnly(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := as(user("bob"))
	_, err := h.svc.Post(bob, room, PostInput{Type: "typing"})
	require.NoError(t, err)

	types := func(v ReadView) map[string]MessageTypeView {
		out := map[string]MessageTypeView{}
		for _, mt := range v.MessageTypes {
			out[mt.Name] = mt
		}
		return out
	}

	full, err := h.svc.Read(bob, room, ReadRequest{Signals: true, Schemas: true})
	require.NoError(t, err)
	require.Len(t, full.Signals, 1)
	mts := types(full)
	require.Equal(t, MessageTypeView{
		Name: "typing", Label: "typing", Persist: realtimecfg.PersistEphemeral, TTLSeconds: 5, CanPost: true,
		Props: realtimecfg.PropsSchema{Type: "object", Properties: map[string]realtimecfg.PropSchema{}},
	}, mts["typing"])
	require.True(t, mts["text"].CanPost)
	require.True(t, mts["text"].Reply)
	require.False(t, mts["perm-text"].CanPost, "bob holds no post role of perm-text")
	require.Equal(t, []string{"body"}, mts["text"].Props.Required)
	require.Contains(t, full.Actions.Post, "typing")

	bare, err := h.svc.Read(bob, room, ReadRequest{})
	require.NoError(t, err)
	require.Empty(t, bare.Signals)
	require.Empty(t, bare.MessageTypes)

	logOnly, err := h.svc.Read(bob, room, ReadRequest{Schemas: true, LogOnly: true})
	require.NoError(t, err)
	mts = types(logOnly)
	require.False(t, mts["typing"].CanPost)
	require.False(t, mts["cursor"].CanPost)
	require.True(t, mts["text"].CanPost)
	require.Equal(t, []string{"note", "text"}, logOnly.Actions.Post)

	ro, err := h.svc.Read(as(readOnlyClient(user("bob"))), room, ReadRequest{Schemas: true})
	require.NoError(t, err)
	for _, mt := range ro.MessageTypes {
		require.False(t, mt.CanPost, mt.Name)
	}
}

// TestPost_LogOnlyRefusesSignals pins that a log-only transport cannot
// post a signal, and that authorization answers first.
func TestPost_LogOnlyRefusesSignals(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	typing := PostInput{Type: "typing", LogOnly: true}

	_, err := h.svc.Post(as(user("bob")), room, typing)
	d := requireDenial(t, err, http.StatusUnprocessableEntity, TypeSignalsNotKept)
	require.Contains(t, d.Title, "typing")

	_, err = h.svc.Post(as(readOnlyClient(user("bob"))), room, typing)
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	_, err = h.svc.Post(as(user("")), room, typing)
	requireDenial(t, err, http.StatusForbidden, TypeNoPrincipal)
	_, err = h.svc.Post(as(user("bob")), RawAddress{EntityType: "ticket", Anchor: "TKT-9", Type: "room"}, typing)
	require.ErrorIs(t, err, ErrAnchorNotFound)

	logged := text("kept")
	logged.LogOnly = true
	h.post(t, user("bob"), logged)
	_, err = h.svc.Post(as(user("bob")), room, PostInput{Type: "typing"})
	require.NoError(t, err, "the server path still takes the signal")
	require.Len(t, h.local.liveSignals(roomKey()), 1)
}

// TestRead_UntypedRouteOutsideAttachTo pins that a route naming no entity
// type answers unknown_channel_type for a readable entity whose type the
// channel type does not attach to, and the uniform anchor miss for an
// entity the caller cannot read.
func TestRead_UntypedRouteOutsideAttachTo(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, err := h.svc.Read(as(user("bob")), RawAddress{Anchor: "DOC-1", Type: "room"}, ReadRequest{})
	d := requireDenial(t, err, http.StatusNotFound, TypeUnknownChannelType)
	require.Contains(t, d.Title, "doc")
	_, err = h.svc.Read(as(user("bob")), RawAddress{Anchor: "DOC-9", Type: "room"}, ReadRequest{})
	require.ErrorIs(t, err, ErrAnchorNotFound)
}
