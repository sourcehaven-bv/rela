package storeutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The \xc8 byte starts a two-byte UTF-8 sequence and '0' is not a valid
// continuation byte, so "\n\xc80" is the fuzzer's original crashing payload
// from BUG-X7ICNM. Every container shape a property value can take must be
// walked, because the fix for the sibling BUG-B1RA3J initially covered only
// top-level strings and nested values slipped through.
func TestValidateProperties_RejectsInvalidUTF8AtEveryNesting(t *testing.T) {
	const bad = "\n\xc80"
	cases := []struct {
		name  string
		props map[string]any
		want  string
	}{
		{"top-level string", map[string]any{"p": bad}, `property "p"`},
		{"string slice", map[string]any{"p": []string{"ok", bad}}, `property "p[1]"`},
		{"any slice", map[string]any{"p": []any{1, bad}}, `property "p[1]"`},
		{"nested map", map[string]any{"p": map[string]any{"v": bad}}, `property "p.v"`},
		{"string map", map[string]any{"p": map[string]string{"v": bad}}, `property "p.v"`},
		{"map in slice", map[string]any{"p": []any{map[string]any{"v": bad}}}, `property "p[0].v"`},
		{"slice in map", map[string]any{"p": map[string]any{"v": []any{bad}}}, `property "p.v[0]"`},
		{"any-keyed map", map[string]any{"p": map[any]any{80: bad}}, `property "p.80"`},
		{"any-keyed map key", map[string]any{"p": map[any]any{bad: "ok"}}, `key`},
		{"property key", map[string]any{bad: "ok"}, `property key`},
		{"nested key", map[string]any{"p": map[string]any{bad: "ok"}}, `key`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProperties(tc.props)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid UTF-8")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// Postgres cannot hold U+0000 in text or jsonb, so pgstore alone refused NUL
// while the other backends stored it. The shared rule makes all four refuse.
func TestValidateProperties_RejectsNUL(t *testing.T) {
	for name, props := range map[string]map[string]any{
		"string":     {"p": "a\x00b"},
		"lone":       {"p": "\x00"},
		"nested":     {"p": map[string]any{"v": []any{"\x00"}}},
		"key":        {"a\x00b": "ok"},
		"nested key": {"p": map[string]string{"\x00": "ok"}},
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidateProperties(props)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "contains NUL")
		})
	}
}

func TestValidateProperties_AcceptsValidText(t *testing.T) {
	props := map[string]any{
		"ascii":    "hello",
		"unicode":  "héllo ☃ 日本語",
		"newline":  "\n0",
		"empty":    "",
		"number":   42,
		"float":    3.5,
		"bool":     true,
		"nil":      nil,
		"strings":  []string{"a", "ü"},
		"anys":     []any{"a", 1, nil, map[string]any{"v": "☃"}},
		"nested":   map[string]any{"v": "ok", "w": []any{"x"}},
		"strmap":   map[string]string{"k": "v"},
		"emptymap": map[string]any{},
		"anykeys":  map[any]any{80: "http", "name": []any{"ok"}},
	}
	assert.NoError(t, ValidateProperties(props))
	assert.NoError(t, ValidateProperties(nil))
}
