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
	require.Equal(t, []string{"briefing", "chat", "incident", "room", "secret", "threads", "topics"}, names)

	room := got.ChannelTypes[3]
	require.False(t, room.Typing)
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

// TestChannelSchema_Typing pins AC 5: a type with typing: true says so,
// and typing is not one of its message types, so the composer never lists
// it.
func TestChannelSchema_Typing(t *testing.T) {
	env := newChannelEnv(t)
	code, b := env.get(t, "alice", channelSchemaPath)
	require.Equal(t, http.StatusOK, code, string(b))
	got := decodeAs[v1.ChannelSchema](t, b)
	chat := got.ChannelTypes[1]
	require.Equal(t, "chat", chat.Name)
	require.True(t, chat.Typing)
	var names []string
	for _, mt := range chat.MessageTypes {
		names = append(names, mt.Name)
	}
	require.Equal(t, []string{"notice", "status", "text"}, names)

	code, b = env.get(t, "alice", chatPath)
	require.Equal(t, http.StatusOK, code, string(b))
	head := decodeAs[v1.ChannelHead](t, b)
	require.Equal(t, []string{"status", "text"}, head.Actions.Post)
	require.True(t, head.Actions.Typing)
}

// TestChannelSchema_SameForEveryPrincipal pins that the descriptor does not
// depend on who asks: the configuration is not a secret. The breach
// configuration adds a session with roles on its transitions.
func TestChannelSchema_SameForEveryPrincipal(t *testing.T) {
	for _, yaml := range []string{channelTestRealtime, breachRealtime} {
		env := newChannelEnv(t)
		env.ch.cfg.Store(env.ch.parse(t, yaml))
		_, want := env.get(t, "alice", channelSchemaPath)
		require.Contains(t, string(want), `"session":`)
		for _, user := range []string{"bob", "carol", "dave", "nobody"} {
			code, b := env.get(t, user, channelSchemaPath)
			require.Equal(t, http.StatusOK, code, user)
			require.JSONEq(t, string(want), string(b), user)
		}
	}
}

// breachRealtime is the breach-response example of docs/realtime.md on
// tickets, and plain, a type without a session.
const breachRealtime = `
version: 1
channel_types:
  breach-response:
    label: Breach response
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.status }
      member: { from: attached }
      dpo:    { permission: privacy.officer }
    message_types:
      text:      { properties: { body: { type: string, required: true } }, post: [member] }
      contained: { properties: { note: { type: string } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: string, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
      dismiss:   { properties: { reason: { type: string, required: true } }, post: [lead] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
      reminder:  { properties: { body: { type: string, required: true } }, post: [lead] }
    session:
      label: GDPR breach response
      initial: detected
      states:
        detected:
          label: Detected
          on:
            contained: { to: contained, roles: [lead] }
            dismiss:   { to: dismissed }
            escalated: { to: contained, by: trigger }
        contained:
          label: Contained
          on:
            assessed: { to: assessed, when: "message.risk ~= 'none'" }
        assessed:
          label: Risk assessed
          on:
            notified: { to: notified }
        notified:  { label: AP notified, final: true }
        dismissed: { label: Dismissed, final: true }
      reminders:
        status-update:
          label: Status update
          in: [detected, contained, assessed]
          every: 30m
          first: 10m
          max: 48
          post: { type: reminder, props: { body: "Post a status update" } }
  plain:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
`

// TestChannelSchema_Session pins AC 2: a session type carries its label,
// initial state and ordered states, and no role, permission, guard or
// transition; a type without a session has no session key.
func TestChannelSchema_Session(t *testing.T) {
	env := newChannelEnv(t)
	env.ch.cfg.Store(env.ch.parse(t, breachRealtime))
	code, b := env.get(t, "alice", channelSchemaPath)
	require.Equal(t, http.StatusOK, code, string(b))
	var raw struct {
		ChannelTypes []map[string]json.RawMessage `json:"channel_types"`
	}
	require.NoError(t, json.Unmarshal(b, &raw))
	require.Len(t, raw.ChannelTypes, 2)
	require.JSONEq(t, `{
		"label": "GDPR breach response",
		"initial": "detected",
		"states": [
			{ "name": "detected", "label": "Detected" },
			{ "name": "contained", "label": "Contained" },
			{ "name": "assessed", "label": "Risk assessed" },
			{ "name": "notified", "label": "AP notified", "final": true },
			{ "name": "dismissed", "label": "Dismissed", "final": true }
		]
	}`, string(raw.ChannelTypes[0]["session"]))
	_, has := raw.ChannelTypes[1]["session"]
	require.False(t, has, "a type without a session has no session key")
	for _, leak := range []string{"lead", "dpo", "privacy.officer", "risk", "transitions", "escalated\":{"} {
		require.NotContains(t, string(raw.ChannelTypes[0]["session"]), leak)
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
