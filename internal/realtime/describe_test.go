package realtime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// TestDescribe_ReusesMessageTypes pins that Describe serves the timing and
// reply rule a channel read serves, sorts properties required first, and
// carries labels and promote targets.
func TestDescribe_ReusesMessageTypes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	types := map[string]ChannelTypeView{}
	for _, ct := range h.svc.Describe() {
		types[ct.Name] = ct
	}
	roomType := types["room"]
	require.Equal(t, "Room", roomType.Label)
	require.Equal(t, string(realtimecfg.CardinalityOne), roomType.Cardinality)
	require.True(t, roomType.RemoveReasonRequired)
	require.Empty(t, roomType.Lifecycle)

	read, err := h.svc.Read(as(user("bob")), room, ReadRequest{Schemas: true})
	require.NoError(t, err)
	byName := map[string]MessageTypeDescription{}
	for _, mt := range roomType.MessageTypes {
		byName[mt.Name] = mt
	}
	require.Len(t, byName, len(read.MessageTypes))
	for _, want := range read.MessageTypes {
		got := byName[want.Name]
		require.Equal(t, want.TTLSeconds, got.TTLSeconds, want.Name)
		require.Equal(t, want.Reply, got.Reply, want.Name)
		require.Equal(t, want.Label, got.Label, want.Name)
		require.Equal(t, string(want.Persist), got.Persist, want.Name)
	}
	var names []string
	for _, p := range byName["note"].Properties {
		names = append(names, p.Name)
	}
	require.Equal(t, []string{"level", "about", "also", "at", "ok", "on", "pick"}, names)
	require.Equal(t, []string{"ticket"}, byName["note"].Properties[1].To)
	for _, p := range byName["note"].Properties {
		require.NotEmpty(t, p.Label, "an unauthored label is the name: %s", p.Name)
	}

	threads := types["threads"]
	require.Equal(t, string(realtimecfg.CardinalityMany), threads.Cardinality)

	a := newActionHarness(t, &fakePromoter{})
	for _, ct := range a.svc.Describe() {
		if ct.Name != "room" {
			continue
		}
		for _, mt := range ct.MessageTypes {
			if mt.Name == "proposal" {
				require.Equal(t, []ActionDescription{
					{Name: "accept", Label: "accept", PromoteTo: "doc"},
					{Name: "ack", Label: "ack"},
					{Name: "triage", Label: "triage"},
				}, mt.Actions)
			}
		}
	}

	l := newHarness(t, withLifecycleYAML())
	for _, ct := range l.svc.Describe() {
		if ct.Name == "room" {
			require.Equal(t, []LabeledName{
				{Name: "close", Label: "close"}, {Name: "reopen", Label: "reopen"},
				{Name: "archive", Label: "archive"}, {Name: "delete", Label: "delete"},
			}, ct.Lifecycle)
		}
	}
}

// TestDescribe_Off pins that realtime off describes no channel type.
func TestDescribe_Off(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.cfg.set(Snapshot{})
	require.NotNil(t, h.svc.Describe())
	require.Empty(t, h.svc.Describe())
}
