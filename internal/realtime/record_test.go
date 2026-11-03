package realtime

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
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

// TestRecord_ConfigChangedDuringWrite pins that a write refuses when a
// reload between its start and its record changed the shape of its channel
// type, or turned realtime off, and writes nothing.
func TestRecord_ConfigChangedDuringWrite(t *testing.T) {
	t.Parallel()
	plain := strings.Replace(testRealtime, "about: { type: ref, to: [ticket] }", "about: { type: string }", 1)
	require.NotEqual(t, testRealtime, plain)
	for name, next := range map[string]*realtimecfg.Compiled{
		"ref turned plain": testCompiled(t, plain),
		"realtime off":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.store.onRecord = func() { h.cfg.set(Snapshot{Config: next}) }
			_, err := h.svc.Post(as(user("bob")), room, text("x"))
			requireDenial(t, err, http.StatusConflict, TypeConfigChanged)
			require.Zero(t, h.store.appends.Load())
		})
	}
}

// TestRecord_NoOpReloadDuringWrite pins that a reload which publishes a
// new config with the same shape for the type does not refuse the write.
func TestRecord_NoOpReloadDuringWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	next := testCompiled(t, testRealtime)
	require.NotSame(t, h.cfg.Load().Config, next)
	h.store.onRecord = func() { h.cfg.set(Snapshot{Config: next}) }
	h.post(t, user("bob"), text("x"))
	require.EqualValues(t, 1, h.store.appends.Load())
}

// TestRender_RecordedRefWinsOverConfig pins that a property the store
// recorded as a ref renders as a ref after a config declares it another
// type: an entity the reader cannot read shows as restricted, without its
// id. The recorded refs are read once per config, not once per message.
func TestRender_RecordedRefWinsOverConfig(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": 1, "about": "TKT-2"}})
	plain := strings.Replace(testRealtime, "about: { type: ref, to: [ticket] }", "about: { type: string }", 1)
	require.NotEqual(t, testRealtime, plain)
	h.cfg.set(Snapshot{Config: testCompiled(t, plain)})
	h.refs.mu.Lock()
	delete(h.refs.views, "TKT-2") // the reader can no longer read it
	h.refs.mu.Unlock()
	st := &recordReadCounter{Store: h.store}
	h.svc.store = st
	for range 2 {
		page, err := h.svc.Read(as(user("bob")), room, ReadRequest{})
		require.NoError(t, err)
		require.Len(t, page.Messages, 1)
		require.Equal(t, &RefValue{Restricted: true}, page.Messages[0].Props["about"])
	}
	require.EqualValues(t, 1, st.reads.Load(), "the recorded refs are cached per config")
}

// recordReadCounter counts TypeRecords reads.
type recordReadCounter struct {
	channels.Store
	reads atomic.Int64
}

func (s *recordReadCounter) TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error) {
	s.reads.Add(1)
	return s.Store.TypeRecords(ctx)
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
	page, err := h.svc.Read(as(user("bob")), room, ReadRequest{})
	require.NoError(t, err)
	require.Len(t, page.Messages, 2)
	require.Equal(t, map[string]any{"level": float64(1)}, page.Messages[0].Props,
		"the undeclared property is omitted")
	require.Nil(t, page.Messages[1].Props, "a message of an undeclared type shows no properties")
}
