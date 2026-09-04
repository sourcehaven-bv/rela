package markdown

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// yaml.v3 emits a block scalar it cannot read back for strings beginning with a
// newline, beginning with a tab and spanning lines, or — under a sequence —
// beginning with a space and spanning lines (BUG-B1RA3J / issue #993).
// ValueToNode quotes those instead, wherever they sit in a value.
//
// These assert a ROUND TRIP — marshal, unmarshal, compare — not merely "no
// error on write". That distinction is the whole point: before the fix, `"\n0"`
// FAILED to write, which was the safe outcome. A change that stopped the error
// while still emitting unreadable YAML would have turned a loud failure into a
// corrupt file on disk, and a write-only assertion would have called that a
// pass.
//
// The `"\n"` case is the one that motivates the phrasing above: it never
// errored. It emitted `|4+` and read back as `""`, losing the value silently —
// worse than the reported bug, and found only because the test compares values.
//
// The nested cases exist because the first version of this fix handled only
// top-level strings and flat lists. A `map[string]any{"v": "\n0"}` — the exact
// shape the store fuzz target generates — still wrote `"0"` to disk without an
// error. Review caught it; the table now covers every container shape a
// property value can take.
func TestValueToNode_RoundTrips(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"leading newline then digit", "\n0"},
		{"leading newline then letter", "\nx"},
		{"lone newline", "\n"},
		{"two newlines", "\n\n"},
		{"newline then tab", "\n\t"},
		{"tab-led multi-line", "\tx\ny"},
		{"tab then newline", "\t\nx"},
		{"tab-led every line", "\ta\n\tb"},
		{"interior newline", "a\nb"},
		{"trailing newline", "0\n"},
		{"leading space", " x"},
		{"leading space multi-line", " x\ny"},
		{"leading space multi-line in list", []string{" x\ny", "ok"}},
		{"leading space multi-line in map under list", []any{map[string]any{"v": " x\ny"}}},
		{"tab on a later line", "a\n\tb"},
		{"plain", "hello"},
		{"list with leading newline", []string{"\n0", "ok"}},
		{"list of any with leading newline", []any{"\nx", "ok"}},
		{"list of any mixed with non-strings", []any{"\nx", 1, true, nil, "ok"}},
		{"nested map", map[string]any{"v": "\n0"}},
		{"nested map lone newline", map[string]any{"v": "\n"}},
		{"nested map tab-led", map[string]any{"v": "\tx\ny"}},
		{"string map", map[string]string{"v": "\n0", "w": "ok"}},
		{"map in list", []any{map[string]any{"v": "\n0"}}},
		{"list in list", []any{[]any{"\n0"}}},
		{"map with breaking string beside ordinary values", map[string]any{
			"a": "\n0", "b": 1, "c": "x\ny", "d": []any{"ok"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := roundTrip(t, tc.val)
			want := normalize(tc.val)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("round trip changed the value: got %#v, want %#v", got, want)
			}
		})
	}
}

// Every value that contains no breaking string must emit byte-for-byte what
// yaml.v3 would have emitted on its own, so the fix does not reflow files it
// need not touch. Without this, quoting every multi-line string would churn the
// on-disk formatting of every existing entity whose property spans lines.
//
// The map with "a2"/"a10" keys pins yaml.v3's numeric-aware key order; the
// hand-built mapping path must reproduce it, and the way to know that it does
// is to see the same bytes here first.
func TestValueToNode_MatchesEncodeWhenNothingBreaks(t *testing.T) {
	values := []any{
		"a\nb\nc", "0\n", " x\ny", "a\n\tb", "\tx", " \nx", "123", "true", "", nil,
		42, 3.5, false,
		[]string{"a\nb", "c"}, []any{"a", 1, nil},
		map[string]any{"a10": "x\ny", "a2": 1, "a9": []any{"z"}},
		map[string]string{"k": "v\nw"},
	}
	for _, val := range values {
		var direct yaml.Node
		if err := direct.Encode(val); err != nil {
			t.Fatalf("Encode(%#v): %v", val, err)
		}
		want, err := yaml.Marshal(&direct)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		node, err := ValueToNode(val)
		if err != nil {
			t.Fatalf("ValueToNode(%#v): %v", val, err)
		}
		got, err := yaml.Marshal(node)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("ValueToNode(%#v) emitted %q, Encode emits %q", val, got, want)
		}
	}
}

// A map that DOES contain a breaking string still has to come out with
// yaml.v3's key order, not bytewise order: "a2" before "a10".
func TestValueToNode_KeepsEncodeKeyOrderWhenQuoting(t *testing.T) {
	node, err := ValueToNode(map[string]any{"a10": "\n0", "a2": "x", "a9": "q"})
	if err != nil {
		t.Fatalf("ValueToNode: %v", err)
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := "a2: x\na9: q\na10: \"\\n0\"\n"
	if string(raw) != want {
		t.Errorf("emitted %q, want %q", raw, want)
	}
}

// The on-disk shape is the contract, not the node's style flag: an ordinary
// multi-line string must still be written as a block scalar.
func TestValueToNode_LeavesOrdinaryMultilineAlone(t *testing.T) {
	node, err := ValueToNode("a\nb\nc")
	if err != nil {
		t.Fatalf("ValueToNode: %v", err)
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.HasPrefix(string(raw), "|") {
		t.Errorf("an ordinary multi-line string was not written as a block scalar: %q", raw)
	}
}

// FuzzValueToNode checks that any valid string round-trips through ValueToNode
// at every position a property value can put it. It is what characterized
// [needsQuoting]; a new failure here means yaml.v3 has a fourth shape and
// needsQuoting must learn it.
func FuzzValueToNode(f *testing.F) {
	for _, s := range []string{"\n0", "\n", "\tx\ny", "a\n\tb", " x\ny", "x", "#\nx", "- x\ny", "\r\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip("invalid UTF-8 is unrepresentable in YAML and refused upstream; see BUG-X7ICNM")
		}
		for _, val := range []any{
			s,
			[]string{s, "ok"},
			map[string]any{"v": s},
			[]any{map[string]any{"v": s}, []any{s}},
		} {
			got := roundTrip(t, val)
			if want := normalize(val); !reflect.DeepEqual(got, want) {
				t.Errorf("round trip changed %#v: got %#v", val, got)
			}
		}
	})
}

// roundTrip marshals val through ValueToNode and reads it back as a generic
// value, failing the test if either direction errors.
func roundTrip(t *testing.T, val any) any {
	t.Helper()
	node, err := ValueToNode(val)
	if err != nil {
		t.Fatalf("ValueToNode(%#v) = error %v", val, err)
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got any
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("emitted %q, cannot read back: %v", raw, err)
	}
	return got
}

// normalize rewrites val into the shapes yaml.v3 decodes into `any`, so a
// typed input can be compared against its generic read-back.
func normalize(val any) any {
	switch v := val.(type) {
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = normalize(e)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(v))
		for k, s := range v {
			out[k] = s
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = normalize(e)
		}
		return out
	}
	return val
}

// Keys pass through the same emitter as values, so a key that starts with a
// newline hit the same defect: written as a block scalar, read back as "",
// which silently RENAMES the property. Ordinary keys, including ones that
// look like other scalar types, must keep being written plain.
func TestMarshalOrdered_KeysRoundTrip(t *testing.T) {
	data := map[string]any{"\n": "0", "\tx\ny": "1", "plain": "2", "123": "3", "y": "4"}
	raw, err := marshalOrdered(data, nil)
	if err != nil {
		t.Fatalf("marshalOrdered: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("emitted %q, cannot read back: %v", raw, err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Errorf("keys changed in round trip: emitted %q, got %#v", raw, got)
	}
	for _, plain := range []string{"\nplain: ", "\n123: ", "\ny: "} {
		if !strings.Contains(string(raw), plain) {
			t.Errorf("ordinary key was not written plain: want %q in %q", plain, raw)
		}
	}
}
