package realtime

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// TestRecord_RefsRecorded pins that a stored write records the type's
// anchor, cardinality and ref properties.
func TestRecord_RefsRecorded(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), text("x"))
	recs, err := h.store.TypeRecords(t.Context())
	require.NoError(t, err)
	require.Equal(t, channels.TypeRecord{
		Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne,
		Refs: []string{"cursor.at", "note.about", "note.also"},
	}, recs["room"])
}

// TestRecord_Conflict pins that a write to a channel type whose store
// record has another shape is refused with 409 channel_type_conflict and
// writes nothing, and that it stays refused.
func TestRecord_Conflict(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	require.NoError(t, h.store.RecordTypes(t.Context(), map[string]channels.TypeRecord{
		"room":   {Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne},
		"topics": {Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne},
	}))
	for range 2 {
		_, err := h.svc.Post(as(user("bob")), room, text("x"))
		d := requireDenial(t, err, http.StatusConflict, TypeChannelTypeConflict)
		require.Equal(t, "channel_types.room", d.Rule)
	}
	require.Zero(t, h.store.appends.Load())
	_, err := h.svc.Create(as(user("bob", "chat.read")), "ticket", "TKT-1", "topics", CreateInput{})
	requireDenial(t, err, http.StatusConflict, TypeChannelTypeConflict)
	_, err = h.svc.Post(as(user("bob")), room, typing())
	require.NoError(t, err, "a signal is never stored, so it records nothing")
}

// TestRecord_RefTurnedPlain pins that once a type is recorded with a ref,
// a configuration declaring that property as another type cannot write to
// the type: the stored ids would render as plain text.
func TestRecord_RefTurnedPlain(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": 1, "about": "TKT-2"}})
	plain := strings.Replace(testRealtime, "about: { type: ref, to: [ticket] }", "about: { type: string }", 1)
	require.NotEqual(t, testRealtime, plain)
	h.cfg.set(Snapshot{Config: testCompiled(t, plain)})
	_, err := h.svc.Post(as(user("bob")), room, text("y"))
	requireDenial(t, err, http.StatusConflict, TypeChannelTypeConflict)
	require.EqualValues(t, 1, h.store.appends.Load())
}

// TestRecord_ConfigChangedDuringWrite pins that a write refuses when
// realtime.yaml was reloaded between its start and its record, and writes
// nothing.
func TestRecord_ConfigChangedDuringWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	next := testCompiled(t, testRealtime)
	h.store.onRecord = func() { h.cfg.set(Snapshot{Config: next}) }
	_, err := h.svc.Post(as(user("bob")), room, text("x"))
	requireDenial(t, err, http.StatusConflict, TypeConfigChanged)
	require.Zero(t, h.store.appends.Load())
	h.store.onRecord = nil
	h.post(t, user("bob"), text("x"))
}

// TestRender_UndeclaredOmitted pins that rendering is fail-safe: a stored
// property that the current configuration does not declare, or whose
// message type it does not declare, is left out rather than shown raw.
func TestRender_UndeclaredOmitted(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": 1, "about": "TKT-2"}})
	h.post(t, user("bob", "chat.post"), PostInput{Type: "perm-text", Props: map[string]any{"body": "x"}})
	noAbout := strings.Replace(testRealtime, "          about: { type: ref, to: [ticket] }\n", "", 1)
	require.NotEqual(t, testRealtime, noAbout)
	h.cfg.set(Snapshot{Config: testCompiled(t, withoutType(noAbout, "perm-text"))})
	page, err := h.svc.Messages(as(user("bob")), room, pageAll())
	require.NoError(t, err)
	require.Len(t, page.Messages, 2)
	require.Equal(t, map[string]any{"level": float64(1)}, page.Messages[0].Props,
		"the undeclared property is omitted")
	require.Nil(t, page.Messages[1].Props, "a message of an undeclared type shows no properties")
}
