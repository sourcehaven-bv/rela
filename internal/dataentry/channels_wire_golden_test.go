package dataentry

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
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
)

// channelWireTypes are the wire types the SPA mirrors, by TypeScript name.
var channelWireTypes = map[string]any{
	"ChannelAuthor":            v1.ChannelAuthor{},
	"Channel":                  v1.Channel{},
	"ChannelTypeList":          v1.ChannelTypeList{},
	"ChannelList":              v1.ChannelList{},
	"ChannelActions":           v1.ChannelActions{},
	"ChannelHead":              v1.ChannelHead{},
	"RefProp":                  v1.RefProp{},
	"MessageRemoval":           v1.MessageRemoval{},
	"MessageAction":            v1.MessageAction{},
	"Message":                  v1.Message{},
	"MessageActionResponse":    v1.MessageActionResponse{},
	"MessagePage":              v1.MessagePage{},
	"EphemeralMessage":         v1.EphemeralMessage{},
	"EphemeralAccepted":        v1.EphemeralAccepted{},
	"PostMessageRequest":       v1.PostMessageRequest{},
	"EditMessageRequest":       v1.EditMessageRequest{},
	"RemoveMessageRequest":     v1.RemoveMessageRequest{},
	"CreateChannelRequest":     v1.CreateChannelRequest{},
	"ChannelSchema":            v1.ChannelSchema{},
	"ChannelTypeSchema":        v1.ChannelTypeSchema{},
	"ChannelLifecycleSchema":   v1.ChannelLifecycleSchema{},
	"ChannelMessageTypeSchema": v1.ChannelMessageTypeSchema{},
	"ChannelPropertySchema":    v1.ChannelPropertySchema{},
	"ChannelActionSchema":      v1.ChannelActionSchema{},
}

// jsonFields lists the JSON names of v's struct fields, sorted.
func jsonFields(v any) []string {
	rt := reflect.TypeOf(v)
	out := make([]string, 0, rt.NumField())
	for f := range rt.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" || !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, name)
	}
	slices.Sort(out)
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
		for k, val := range x {
			if s, ok := val.(string); ok && s != "" && (k == "at" || strings.HasSuffix(k, "_at")) {
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

func TestChannelWireGolden(t *testing.T) {
	fields := map[string][]string{}
	for name, v := range channelWireTypes {
		fields[name] = jsonFields(v)
	}
	compareOrWrite(t, channelWireFieldsFile, fields)

	env := newChannelEnv(t)
	env.postText(t, "alice", roomPath, "hello")
	samples := map[string]any{}
	for name, path := range map[string]string{
		"list":     "/api/v1/tickets/TKT-1/_channels",
		"head":     roomPath,
		"messages": roomPath + "/messages",
		"schema":   channelSchemaPath,
	} {
		code, b := env.get(t, "alice", path)
		require.Equal(t, http.StatusOK, code, "%s: %s", path, b)
		var v any
		require.NoError(t, json.Unmarshal(b, &v))
		samples[name] = normalizeWire(v)
	}
	compareOrWrite(t, channelWireFile, samples)
}
