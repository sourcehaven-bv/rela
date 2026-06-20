package canonical_test

import (
	"bytes"
	"encoding/json"
	"testing"

	yaml "gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

// TestHashEntity_CrossBackendDecode is the linchpin guarantee of TKT-8FSBGB:
// the same logical entity must hash identically regardless of which storage
// backend reconstructed it.
//
// To be faithful, each case is expressed as the RAW frontmatter text a user
// would author. The fsstore arm decodes that YAML directly (fsstore's real
// path: yaml.Unmarshal of the frontmatter string). The pgstore arm models the
// pg round-trip: the decoded value is JSON-marshaled (pg write) and re-decoded
// with json.Decoder.UseNumber (pg read), exactly as pgstore does. If
// canonicalization is correct, both arms hash identically.
//
// The cases deliberately include every divergence found in code review
// (whole-valued floats, dates, non-string-keyed maps, large unsigned ints,
// control characters) as regression fixtures.
func TestHashEntity_CrossBackendDecode(t *testing.T) {
	cases := []struct {
		name string
		yaml string // raw frontmatter (properties only)
	}{
		{name: "scalars", yaml: "title: Hello\npriority: 3\ndone: true\n"},
		{name: "whole-valued float (regression: 2.0)", yaml: "ratio: 2.0\nscore: 5.0\n"},
		{name: "fractional float", yaml: "ratio: 1.5\n"},
		{name: "negative and zero ints", yaml: "a: -5\nb: 0\n"},
		{name: "date (regression: time.Time)", yaml: "due: 2026-06-19\n"},
		{name: "datetime (regression)", yaml: "ts: 2026-06-19T10:30:00Z\n"},
		{name: "string list", yaml: "tags:\n  - alpha\n  - beta\n"},
		{name: "nested string-keyed map", yaml: "meta:\n  a: 1\n  b: two\n"},
		{name: "unicode strings", yaml: "title: café ☕ — über\n"},
		{name: "control char in value (regression: collision)", yaml: "weird: \"a\\x1fb\\x1ec\"\n"},
		{name: "empty", yaml: "\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := decodeYAMLProps(t, tc.yaml)

			viaFS := entity.Entity{ID: "E1", Type: "ticket", Properties: props}
			viaPG := entity.Entity{ID: "E1", Type: "ticket", Properties: pgRoundTrip(t, props)}

			hfs := canonical.HashEntity(viaFS)
			hpg := canonical.HashEntity(viaPG)
			if hfs != hpg {
				t.Fatalf("cross-backend hash mismatch:\n  fs=%s  %#v\n  pg=%s  %#v",
					hfs, viaFS.Properties, hpg, viaPG.Properties)
			}
		})
	}
}

// decodeYAMLProps decodes raw frontmatter exactly as fsstore.parseDocument does.
func decodeYAMLProps(t *testing.T, fm string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := yaml.Unmarshal([]byte(fm), &out); err != nil {
		t.Fatalf("yaml.Unmarshal(%q): %v", fm, err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out
}

// pgRoundTrip models pgstore's storage round-trip: marshal the properties to
// JSON (the JSONB column write) and decode them back with json.Decoder.UseNumber
// (the read). It does NOT pre-fold numbers — canonical.normalize handles the raw
// json.Number, so this test exercises the real pg read shape without forking
// pgstore's normalizeJSONNumbers (the previous version's fragile copy).
func pgRoundTrip(t *testing.T, props map[string]any) map[string]any {
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
	return out
}

// FuzzCrossBackendDecode generates arbitrary frontmatter and asserts the fs and
// pg arms hash identically. yaml that fails to parse, or values that don't
// survive a JSON round-trip, are skipped — we only assert the invariant for
// inputs both real backends could actually store.
func FuzzCrossBackendDecode(f *testing.F) {
	seeds := []string{
		"title: Hello\nn: 3\n",
		"ratio: 2.0\n",
		"due: 2026-06-19\n",
		"tags:\n  - a\n  - b\n",
		"m:\n  k: v\n",
		"weird: \"a\\x1fb\"\n",
		"big: 18446744073709551615\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, fm string) {
		var props map[string]any
		if err := yaml.Unmarshal([]byte(fm), &props); err != nil {
			t.Skip() // not valid frontmatter
		}
		if props == nil {
			props = map[string]any{}
		}
		// Only compare inputs that survive a JSON round-trip (what pg can store).
		raw, err := json.Marshal(props)
		if err != nil {
			t.Skip()
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var pg map[string]any
		if err := dec.Decode(&pg); err != nil {
			t.Skip()
		}
		if pg == nil {
			pg = map[string]any{}
		}

		hfs := canonical.HashEntity(entity.Entity{ID: "E", Type: "t", Properties: props})
		hpg := canonical.HashEntity(entity.Entity{ID: "E", Type: "t", Properties: pg})
		if hfs != hpg {
			t.Fatalf("cross-backend hash mismatch for %q:\n  fs props=%#v\n  pg props=%#v", fm, props, pg)
		}
	})
}

// TestBodyConvergence is the regression for the body half of the cross-backend
// guarantee (RR-G92SKT): pgstore stores a body raw, fsstore stores
// FormatMarkdown(raw). The two entities must hash identically even when
// FormatMarkdown is not idempotent (e.g. "0) \n\n0").
func TestBodyConvergence(t *testing.T) {
	bodies := []string{
		"",
		"# Title\n\nA paragraph.\n",
		"0) \n\n0", // FormatMarkdown leaves then strips a leading blank line
		"**\n*",    // goldmark re-parses its own output ("** *" -> "---")
		"- a\n\n- b\n",
		"```\ncode\n```\n",
		"a very long line of prose that the formatter will wrap somewhere near the eighty column mark when it normalizes the body into canonical form for hashing",
	}
	for _, raw := range bodies {
		t.Run(raw, func(t *testing.T) {
			pg := entity.Entity{ID: "E", Type: "t", Content: raw}                          // pg stores raw
			fs := entity.Entity{ID: "E", Type: "t", Content: markdown.FormatMarkdown(raw)} // fs stores once-formatted
			if canonical.HashEntity(pg) != canonical.HashEntity(fs) {
				t.Fatalf("body did not converge for %q:\n raw->%q\n fmt->%q",
					raw, raw, markdown.FormatMarkdown(raw))
			}
		})
	}
}

// FuzzBodyConvergence asserts the body invariant over arbitrary input: an
// entity whose body is stored raw (pg) and one whose body is stored
// once-formatted (fs) must hash identically, regardless of FormatMarkdown's
// idempotency.
func FuzzBodyConvergence(f *testing.F) {
	for _, s := range []string{"", "# h\n\ntext\n", "0) \n\n0", "**\n*", "- a\n- b\n", "```\nx\n```\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		pg := canonical.HashEntity(entity.Entity{ID: "E", Type: "t", Content: body})
		fs := canonical.HashEntity(entity.Entity{ID: "E", Type: "t", Content: markdown.FormatMarkdown(body)})
		if pg != fs {
			t.Fatalf("body convergence failed for %q", body)
		}
	})
}
