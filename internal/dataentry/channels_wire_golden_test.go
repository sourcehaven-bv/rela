package dataentry

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// The SPA mirrors the channel wire types by hand (frontend/src/types/
// channels.ts). TestChannelWireGolden writes the JSON field set of each
// wire type, and sample answers, to testdata; wireFields.test.ts compares
// the TypeScript key lists with the field sets, so a field added or
// renamed on one side alone fails a test. Rewrite with
// `go test ./internal/dataentry -run TestChannelWireGolden -update`.
const (
	channelWireFieldsFile = "testdata/channel_wire_fields.json"
	channelWireFile       = "testdata/channel_wire.json"
	// channelMessageOpsFile holds the message operation names, which the
	// SPA's MESSAGE_OPS mirrors.
	channelMessageOpsFile = "testdata/channel_message_ops.json"
)

// channelWireTypes are the wire types the SPA mirrors, by TypeScript name.
var channelWireTypes = map[string]any{
	"ChannelAuthor":              v1.ChannelAuthor{},
	"Channel":                    v1.Channel{},
	"ChannelRead":                v1.ChannelRead{},
	"ChannelMarkRead":            v1.ChannelMarkRead{},
	"MarkReadRequest":            v1.MarkReadRequest{},
	"ChannelSession":             v1.ChannelSession{},
	"ChannelSessionTransition":   v1.ChannelSessionTransition{},
	"ChannelSessionTimer":        v1.ChannelSessionTimer{},
	"ChannelSessionWaiting":      v1.ChannelSessionWaiting{},
	"ChannelSessionReminder":     v1.ChannelSessionReminder{},
	"ChannelTypeList":            v1.ChannelTypeList{},
	"ChannelList":                v1.ChannelList{},
	"ChannelActions":             v1.ChannelActions{},
	"ChannelHead":                v1.ChannelHead{},
	"TypingEntry":                v1.TypingEntry{},
	"TypingAccepted":             v1.TypingAccepted{},
	"RefProp":                    v1.RefProp{},
	"MessageRemoval":             v1.MessageRemoval{},
	"MessageAction":              v1.MessageAction{},
	"Message":                    v1.Message{},
	"ChannelMessageOutputs":      v1.ChannelMessageOutputs{},
	"ChannelMessageOutputResult": v1.ChannelMessageOutputResult{},
	"MessageActionResponse":      v1.MessageActionResponse{},
	"MessagePage":                v1.MessagePage{},
	"EphemeralMessage":           v1.EphemeralMessage{},
	"EphemeralAccepted":          v1.EphemeralAccepted{},
	"PostMessageRequest":         v1.PostMessageRequest{},
	"EditMessageRequest":         v1.EditMessageRequest{},
	"RemoveMessageRequest":       v1.RemoveMessageRequest{},
	"CreateChannelRequest":       v1.CreateChannelRequest{},
	"ChannelSchema":              v1.ChannelSchema{},
	"ChannelTypeSchema":          v1.ChannelTypeSchema{},
	"ChannelLifecycleSchema":     v1.ChannelLifecycleSchema{},
	"ChannelMessageTypeSchema":   v1.ChannelMessageTypeSchema{},
	"ChannelPropertySchema":      v1.ChannelPropertySchema{},
	"ChannelActionSchema":        v1.ChannelActionSchema{},
	"ChannelSessionSchema":       v1.ChannelSessionSchema{},
	"ChannelSessionStateSchema":  v1.ChannelSessionStateSchema{},
	"Inbox":                      v1.Inbox{},
	"InboxEntry":                 v1.InboxEntry{},
	"InboxChannel":               v1.InboxChannel{},
	"InboxSession":               v1.InboxSession{},
	"InboxLinks":                 v1.InboxLinks{},
	"InboxUnread":                v1.InboxUnread{},
	"InboxMention":               v1.InboxMention{},
	"InboxSummary":               v1.InboxSummary{},
	"MessageMention":             v1.MessageMention{},
}

// jsonFields lists the JSON names of v's struct fields, sorted. An
// embedded struct without a JSON name contributes its own fields, as
// encoding/json flattens it.
func jsonFields(v any) []string {
	out := structFields(reflect.TypeOf(v))
	slices.Sort(out)
	return out
}

func structFields(rt reflect.Type) []string {
	var out []string
	for f := range rt.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if f.Anonymous && name == "" && ft.Kind() == reflect.Struct {
			out = append(out, structFields(ft)...)
			continue
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, name)
	}
	return out
}

// updatingGoldens reports whether the test binary runs with -update, the
// flag the golden parity corpus declares in this binary.
func updatingGoldens() bool {
	f := flag.Lookup("update")
	return f != nil && f.Value.String() == "true"
}

// compareOrWrite compares got, as indented JSON, with the file at path, or
// rewrites the file under -update.
func compareOrWrite(t *testing.T, path string, got any) {
	t.Helper()
	b, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	b = append(b, '\n')
	if updatingGoldens() {
		require.NoError(t, os.WriteFile(path, b, 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with -update to write %s", path)
	require.JSONEq(t, string(want), string(b), "%s is stale; run with -update", path)
}

// normalizeWire replaces the time values in a decoded answer, so the
// samples do not change from run to run.
func normalizeWire(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if _, signal := x["expires_at"]; signal && x["id"] != nil {
			x["id"] = "signal-id" // minted at random
		}
		for k, val := range x {
			if s, ok := val.(string); ok && s != "" && isWireTimeKey(k) {
				x[k] = "2026-01-02T03:04:05Z"
				continue
			}
			x[k] = normalizeWire(val)
		}
	case []any:
		for i := range x {
			x[i] = normalizeWire(x[i])
		}
	}
	return v
}

// isWireTimeKey reports whether k names a time value on the channel wire.
func isWireTimeKey(k string) bool {
	return k == "at" || strings.HasSuffix(k, "_at") || k == "edit_until" || k == "release_after" || k == "since"
}

// TestJSONFields_FlattensEmbedded pins that an embedded struct contributes
// its fields, not its type name.
func TestJSONFields_FlattensEmbedded(t *testing.T) {
	type inner struct {
		A string `json:"a"`
	}
	type outer struct {
		inner
		*v1.RefProp
		B string `json:"b"`
		c string
	}
	require.Equal(t, []string{"a", "b", "id", "restricted", "title", "type"}, jsonFields(outer{c: ""}))
}

func TestChannelWireGolden(t *testing.T) {
	fields := map[string][]string{}
	for name, v := range channelWireTypes {
		fields[name] = jsonFields(v)
	}
	compareOrWrite(t, channelWireFieldsFile, fields)
	compareOrWrite(t, channelMessageOpsFile, realtime.MessageOpNames())

	env := newChannelEnv(t)
	// The samples cover every message shape the SPA reads: alice's own
	// message with edit and its window, a ref she cannot read, an action
	// done, a pending one she may release later, a removed message and an
	// ephemeral signal.
	env.postText(t, "alice", roomPath, "hello")
	resp, b := env.do(t, "dave", http.MethodPost, roomPath+"/messages",
		`{"type":"text","props":{"body":"see","about":"FEAT-1"}}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	done := env.postText(t, "alice", roomPath, "acked")
	resp, b = env.do(t, "alice", http.MethodPost, fmt.Sprintf("%s/messages/%d/_action/ack", roomPath, done.Seq), "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
	pending := env.postText(t, "alice", roomPath, "pending")
	env.pend(t, pending.Seq, time.Now())
	gone := env.postText(t, "alice", roomPath, "gone")
	resp, b = env.do(t, "alice", http.MethodDelete, fmt.Sprintf("%s/messages/%d", roomPath, gone.Seq), "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
	resp, b = env.do(t, "alice", http.MethodPost, roomPath+"/messages", `{"type":"typing"}`, nil)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
	// A session channel whose session moved: the head carries the session
	// and the page a notice.
	resp, b = env.do(t, "alice", http.MethodPost, incidentPath+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", b)
	// bob types in the chat: alice's head lists him in typing_now.
	resp, b = env.do(t, "bob", http.MethodPost, chatPath+"/_typing", "", nil)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
	var accepted any
	require.NoError(t, json.Unmarshal(b, &accepted))
	samples := map[string]any{"typing_accepted": accepted}
	for name, path := range map[string]string{
		"list":             "/api/v1/tickets/TKT-1/_channels",
		"head":             roomPath,
		"messages":         roomPath + "/messages",
		"schema":           channelSchemaPath,
		"session_head":     incidentPath,
		"session_messages": incidentPath + "/messages",
		"typing_head":      chatPath,
		"inbox_summary":    inboxSummaryPath,
	} {
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s: %s", path, b)
		var v any
		require.NoError(t, json.Unmarshal(b, &v))
		samples[name] = normalizeWire(v)
	}

	// The breach-response example: its descriptor entry, and a head in
	// contained, whose transition is guarded. The lead role is swapped for
	// member so alice can move the session; the descriptor names no role,
	// so it is the example's.
	breach := strings.Replace(breachRealtime, "contained: { to: contained, roles: [lead] }",
		"contained: { to: contained, roles: [member] }", 1)
	env.ch.cfg.Store(env.ch.parse(t, breach))
	const breachPath = "/api/v1/tickets/TKT-1/_channels/breach-response"
	resp, posted := env.do(t, "alice", http.MethodPost, breachPath+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", posted)
	// The leader's timer round, an hour on, posts the first reminder.
	runTimerRound(t, env, time.Now().Add(time.Hour))
	for name, path := range map[string]string{
		"session_schema": channelSchemaPath, "guarded_head": breachPath, "reminder_messages": breachPath + "/messages",
	} {
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s: %s", path, b)
		var v any
		require.NoError(t, json.Unmarshal(b, &v))
		samples[name] = normalizeWire(v)
	}

	// Session outputs: TKT-1's incident, in contained, offers a note
	// transition with one output; TKT-2's contained notice records its
	// outputs done, with a created entity alice cannot read, and TKT-1's
	// note notice records a failed set.
	env.ch.cfg.Store(env.ch.parse(t, outputsRealtime))
	const otherIncident = "/api/v1/tickets/TKT-2/_channels/incident"
	code, head := env.get(t, "alice", incidentPath)
	require.Equal(t, http.StatusOK, code, "%s", head)
	var v any
	require.NoError(t, json.Unmarshal(head, &v))
	samples["outputs_head"] = normalizeWire(v)
	resp, posted = env.do(t, "alice", http.MethodPost, otherIncident+"/messages", `{"type":"contained"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", posted)
	resp, posted = env.do(t, "alice", http.MethodPost, incidentPath+"/messages",
		`{"type":"note","props":{"body":"done"}}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", posted)
	for name, path := range map[string]string{
		"outputs_messages": otherIncident + "/messages", "failed_outputs_messages": incidentPath + "/messages",
	} {
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s: %s", path, b)
		var v any
		require.NoError(t, json.Unmarshal(b, &v))
		samples[name] = normalizeWire(v)
	}
	compareOrWrite(t, channelWireFile, samples)
}
