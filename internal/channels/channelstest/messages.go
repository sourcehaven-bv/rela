package channelstest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// getMessagesTests are the batch message read tests (TKT-V05706X),
// registered in [RunAll].
var getMessagesTests = []conformanceTest{
	{"GetMessages", testGetMessages},
	{"GetMessagesRefusals", testGetMessagesRefusals},
	{"GetMessagesMax", testGetMessagesMax},
}

// testGetMessages pins [channels.Store.GetMessages] (AC 2): every named
// message of a live channel, equal to Get; a removed message as its stub;
// an edit's current revision; missing channels, seqs and tombstoned
// channels absent; a duplicate read once; no refs is an empty answer.
func testGetMessages(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	a, b := key("TKT-1", "room"), key("TKT-2", "room")
	face := key("TKT-1", "notes")
	face.Face = "draft"
	for i := range 3 {
		mustAppend(t, h.Store, a, text(fmt.Sprintf("a%d", i+1)))
		mustAppend(t, h.Store, b, text(fmt.Sprintf("b%d", i+1)))
	}
	mustAppend(t, h.Store, face, text("draft"))
	_, err := h.Store.Edit(ctx, a, 2, map[string]any{"body": "a2 edited"}, nil, T0.Add(time.Minute))
	require.NoError(t, err)
	_, err = h.Store.Remove(ctx, b, 3, channels.Removal{By: Bob, At: T0.Add(time.Minute), Reason: "spam"})
	require.NoError(t, err)
	gone := key("TKT-3", "room")
	mustAppend(t, h.Store, gone, text("filed"))
	tombstone(t, h.Store, "TKT-3", "", T0.Add(time.Hour))

	refs := []channels.MessageRef{
		{Key: a, Seq: 2}, {Key: b, Seq: 3}, {Key: a, Seq: 1}, {Key: face, Seq: 1},
		{Key: a, Seq: 2},                    // duplicate
		{Key: a, Seq: 99},                   // missing seq
		{Key: key("TKT-9", "room"), Seq: 1}, // missing channel
		{Key: gone, Seq: 1},                 // tombstoned channel
		{Key: channels.Key{Anchor: a.Anchor, Type: "room", Name: "side"}, Seq: 1}, // missing named channel
	}
	got, err := h.Store.GetMessages(ctx, refs)
	require.NoError(t, err)
	want := map[channels.MessageRef]channels.Message{}
	for _, r := range []channels.MessageRef{{Key: a, Seq: 2}, {Key: b, Seq: 3}, {Key: a, Seq: 1}, {Key: face, Seq: 1}} {
		m, getErr := h.Store.Get(ctx, r.Key, r.Seq)
		require.NoError(t, getErr)
		want[r] = m
	}
	require.Equal(t, want, got)
	require.Equal(t, "a2 edited", got[channels.MessageRef{Key: a, Seq: 2}].Props["body"])
	require.NotNil(t, got[channels.MessageRef{Key: b, Seq: 3}].Removed, "a removed message is its stub")

	empty, err := h.Store.GetMessages(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}

// testGetMessagesRefusals pins the refused refs: an invalid key, a seq of
// 0 or below, and more than [channels.MaxGetMessages] refs.
func testGetMessagesRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("x"))
	for name, refs := range map[string][]channels.MessageRef{
		"invalid key": {{Key: key("../x", "room"), Seq: 1}},
		"seq 0":       {{Key: k, Seq: 0}},
		"negative":    {{Key: k, Seq: -1}},
		"too many":    make([]channels.MessageRef, channels.MaxGetMessages+1),
	} {
		if name == "too many" {
			for i := range refs {
				refs[i] = channels.MessageRef{Key: k, Seq: 1}
			}
		}
		_, err := h.Store.GetMessages(ctx, refs)
		require.ErrorIs(t, err, channels.ErrInvalidKey, name)
	}
}

// testGetMessagesMax pins a full call: [channels.MaxGetMessages] refs over
// as many channels, each answered.
func testGetMessagesMax(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	refs := make([]channels.MessageRef, 0, channels.MaxGetMessages)
	for i := range channels.MaxGetMessages {
		k := key(fmt.Sprintf("TKT-%d", i+1), "room")
		m := mustAppend(t, h.Store, k, text("q"))
		refs = append(refs, channels.MessageRef{Key: k, Seq: m.Seq})
	}
	got, err := h.Store.GetMessages(ctx, refs)
	require.NoError(t, err)
	require.Len(t, got, channels.MaxGetMessages)
}
