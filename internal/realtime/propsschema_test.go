package realtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// schemaAccepts validates props against ps the way an agent's validator
// would, and asserts the formats too, which jsonschema-go does not.
func schemaAccepts(t *testing.T, ps realtimecfg.PropsSchema, props map[string]any) bool {
	t.Helper()
	raw, err := json.Marshal(ps)
	require.NoError(t, err)
	var s jsonschema.Schema
	require.NoError(t, json.Unmarshal(raw, &s))
	rs, err := s.Resolve(nil)
	require.NoError(t, err)
	if rs.Validate(props) != nil {
		return false
	}
	for name, p := range ps.Properties {
		v, ok := props[name].(string)
		if !ok {
			continue
		}
		switch p.Format {
		case realtimecfg.FormatDate:
			if _, err := time.Parse(metamodel.DefaultDateFormat, v); err != nil {
				return false
			}
		case realtimecfg.FormatDateTime:
			if _, err := time.Parse(time.RFC3339, v); err != nil {
				return false
			}
		}
	}
	return true
}

// TestPropsSchema_AgreesWithValidation pins AC5: for the checks JSON
// Schema can express, the published props schema accepts exactly what the
// service accepts. It cannot express the type's `when:`, the property
// count, the encoded size, the entity id syntax of a ref and whether a ref
// names a readable entity of an allowed type; a null value, which the
// service reads as absent, is left out too.
func TestPropsSchema_AgreesWithValidation(t *testing.T) {
	t.Parallel()
	cfg := testCompiled(t, testRealtime)
	ct, ok := cfg.ChannelType("room")
	require.True(t, ok)
	cases := []struct {
		typ, props string
		valid      bool
	}{
		{"note", `{"level": 1}`, true},
		{"note", `{"level": 1, "on": "2026-01-02", "at": "2026-01-02T03:04:05Z", "ok": true,
			"pick": "low", "about": "TKT-2", "also": "DOC-1"}`, true},
		{"note", `{}`, false},
		{"note", `{"level": "1"}`, false},
		{"note", `{"level": 1.5}`, false},
		{"note", `{"level": 1, "extra": 1}`, false},
		{"note", `{"level": 1, "on": "tomorrow"}`, false},
		{"note", `{"level": 1, "at": "2026-01-02"}`, false},
		{"note", `{"level": 1, "ok": "yes"}`, false},
		{"note", `{"level": 1, "pick": "mid"}`, false},
		{"note", `{"level": 1, "about": 7}`, false},
		{"text", `{"body": "` + strings.Repeat("a", 100) + `"}`, true},
		{"text", `{"body": "` + strings.Repeat("é", 100) + `"}`, true},
		{"text", `{"body": "` + strings.Repeat("a", 101) + `"}`, false},
		{"text", `{}`, false},
		{"typing", `{}`, true},
		{"typing", `{"x": 1}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.typ+" "+tc.props, func(t *testing.T) {
			t.Parallel()
			mt := ct.MessageTypes[tc.typ]
			var props map[string]any
			require.NoError(t, json.Unmarshal([]byte(tc.props), &props))
			_, err := validateProps(mt, props, realtimelimits.MaxMessageBytes)
			require.Equal(t, tc.valid, err == nil, "service: %v", err)
			require.Equal(t, tc.valid, schemaAccepts(t, mt.PropsSchema(), props), "schema")
		})
	}
}
