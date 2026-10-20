package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

const channelSchemaPath = "/api/v1/_schema/channels"

// TestChannelSchema_Lists pins the descriptor's shape: every channel type
// sorted, labels, lifecycle in order, properties required first, actions
// with their promote target.
func TestChannelSchema_Lists(t *testing.T) {
	env := newChannelEnv(t)
	code, b := env.get(t, "alice", channelSchemaPath)
	require.Equal(t, http.StatusOK, code, string(b))
	got := decodeAs[v1.ChannelSchema](t, b)
	var names []string
	for _, ct := range got.ChannelTypes {
		names = append(names, ct.Name)
	}
	require.Equal(t, []string{"room", "secret", "threads", "topics"}, names)

	room := got.ChannelTypes[0]
	require.Equal(t, "Room", room.Label)
	require.Equal(t, []string{"ticket", "feature"}, room.AttachTo)
	require.Equal(t, "one", room.Cardinality)
	require.Equal(t, []v1.ChannelLifecycleSchema{
		{Name: "close", Label: "close"}, {Name: "reopen", Label: "reopen"}, {Name: "delete", Label: "delete"},
	}, room.Lifecycle)
	require.False(t, room.RemoveReasonRequired)
	text := room.MessageTypes[1]
	require.Equal(t, "text", text.Name)
	require.True(t, text.Reply)
	require.Equal(t, []v1.ChannelPropertySchema{
		{Name: "body", Label: "body", Type: "string", Required: true},
		{Name: "about", Label: "about", Type: "ref", To: []string{"ticket", "feature"}},
	}, text.Properties)
	require.Equal(t, []v1.ChannelActionSchema{
		{Name: "accept", Label: "accept", PromoteTo: "ticket"}, {Name: "ack", Label: "ack"},
	}, text.Actions)
	typing := room.MessageTypes[2]
	require.Equal(t, "ephemeral", typing.Persist)
	require.Equal(t, 5, typing.TTLSeconds)
	require.NotNil(t, typing.Properties)
}

// TestChannelSchema_SameForEveryPrincipal pins that the descriptor does not
// depend on who asks: the configuration is not a secret.
func TestChannelSchema_SameForEveryPrincipal(t *testing.T) {
	env := newChannelEnv(t)
	_, want := env.get(t, "alice", channelSchemaPath)
	for _, user := range []string{"carol", "dave", "nobody"} {
		code, b := env.get(t, user, channelSchemaPath)
		require.Equal(t, http.StatusOK, code, user)
		require.JSONEq(t, string(want), string(b), user)
	}
}

// TestChannelSchema_Off pins that realtime off, and channels not wired,
// answer an empty list rather than an error.
func TestChannelSchema_Off(t *testing.T) {
	off := newChannelEnv(t)
	off.ch.cfg.Store(nil)
	plain := newLiveEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	for _, env := range []*liveEnv{off.liveEnv, plain} {
		code, b := env.get(t, "alice", channelSchemaPath)
		require.Equal(t, http.StatusOK, code)
		require.JSONEq(t, `{"channel_types":[]}`, string(b))
	}
	resp, _ := plain.do(t, "alice", http.MethodPost, channelSchemaPath, "", nil)
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

// TestConfig_ServesChannels pins that the `channels:` block reaches the
// SPA in `_config`, next to forms, in field order.
func TestConfig_ServesChannels(t *testing.T) {
	app := newTestAppV1(t)
	app.State().Cfg.Channels = map[string]dataentryconfig.ChannelPresentation{"room": {
		Messages: map[string]dataentryconfig.MessagePresentation{"text": {Fields: []dataentryconfig.FormField{
			{Property: "body", Label: "Message", Widget: dataentryconfig.WidgetTextarea},
			{Property: "about", Span: 6},
		}}},
	}}
	rec := httptest.NewRecorder()
	app.handleV1Config(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code)
	var raw struct {
		Channels json.RawMessage `json:"channels"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.JSONEq(t, `{"room":{"messages":{"text":{"fields":[
		{"property":"body","label":"Message","widget":"textarea"},
		{"property":"about","span":6}]}}}}`, string(raw.Channels))
}
