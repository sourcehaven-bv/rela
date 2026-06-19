package canonical_test

import (
	"bytes"
	"encoding/json"
	"testing"

	yaml "gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestHashEntity_CrossBackendDecode is the linchpin guarantee of TKT-8FSBGB:
// the same logical entity must hash identically regardless of which storage
// backend reconstructed it.
//
// Rather than stand up both real stores (fsstore needs schema wiring, pgstore
// is build-tagged and DB-gated — those round-trips are asserted in each store's
// own suite when the hash is wired in), this test reproduces the exact decode
// paths that produce the divergent Go types:
//
//   - fsstore decodes YAML frontmatter (gopkg.in/yaml.v3).
//   - pgstore decodes a JSONB blob with json.Decoder.UseNumber, then runs
//     normalizeJSONNumbers to fold whole numbers back to int.
//
// If canonical hashing is correct, an entity whose properties came through the
// YAML path and the same entity whose properties came through the JSON path
// hash to the same value.
func TestHashEntity_CrossBackendDecode(t *testing.T) {
	cases := []struct {
		name  string
		props map[string]any
		body  string
	}{
		{
			name:  "scalars",
			props: map[string]any{"title": "Hello", "priority": 3, "done": true},
		},
		{
			name:  "whole and fractional numbers",
			props: map[string]any{"count": 42, "ratio": 1.5, "zero": 0},
		},
		{
			name:  "string list",
			props: map[string]any{"tags": []any{"alpha", "beta", "gamma"}},
		},
		{
			name:  "nested map",
			props: map[string]any{"meta": map[string]any{"a": 1, "b": "two", "c": []any{1, 2}}},
		},
		{
			name:  "unicode and multiline body",
			props: map[string]any{"title": "café ☕ — über"},
			body:  "# Heading\n\nA paragraph with some length so that wrapping behavior is exercised across the eighty column boundary.\n\n- a\n- b\n",
		},
		{
			name:  "empty props",
			props: map[string]any{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viaYAML := entity.Entity{
				ID: "E1", Type: "ticket",
				Properties: decodeViaYAML(t, tc.props),
				Content:    tc.body,
			}
			viaJSON := entity.Entity{
				ID: "E1", Type: "ticket",
				Properties: decodeViaJSON(t, tc.props),
				Content:    tc.body,
			}

			hy := canonical.HashEntity(viaYAML)
			hj := canonical.HashEntity(viaJSON)
			if hy != hj {
				t.Fatalf("cross-backend hash mismatch:\n yaml=%s (%#v)\n json=%s (%#v)",
					hy, viaYAML.Properties, hj, viaJSON.Properties)
			}
		})
	}
}

// decodeViaYAML round-trips a property map through YAML, reproducing fsstore's
// frontmatter decode (gopkg.in/yaml.v3) and the concrete Go types it yields.
func decodeViaYAML(t *testing.T, props map[string]any) map[string]any {
	t.Helper()
	raw, err := yaml.Marshal(props)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	var out map[string]any
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out
}

// decodeViaJSON round-trips a property map through JSON the way pgstore does:
// json.Decoder with UseNumber, then whole-number folding (a local copy of
// pgstore's normalizeJSONNumbers, kept here so the test does not depend on the
// build-tagged pgstore package).
func decodeViaJSON(t *testing.T, props map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("json.Decode: %v", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return normalizeJSONNumbers(out).(map[string]any)
}

// normalizeJSONNumbers mirrors pgstore's normalization: json.Number folds to int
// when whole, else float64.
func normalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	case map[string]any:
		for k, val := range t {
			t[k] = normalizeJSONNumbers(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = normalizeJSONNumbers(t[i])
		}
		return t
	default:
		return v
	}
}
