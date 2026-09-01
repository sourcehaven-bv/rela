package markdown

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// yaml.v3 emits a block scalar it cannot read back for strings beginning with a
// newline (BUG-B1RA3J / issue #993). ValueToNode quotes those instead.
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
func TestValueToNode_RoundTrips(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"leading newline then digit", "\n0"},
		{"leading newline then letter", "\nx"},
		{"lone newline", "\n"},
		{"interior newline", "a\nb"},
		{"trailing newline", "0\n"},
		{"leading space", " x"},
		{"plain", "hello"},
		{"list with leading newline", []string{"\n0", "ok"}},
		{"list of any with leading newline", []any{"\nx", "ok"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ValueToNode(tc.val)
			if err != nil {
				t.Fatalf("ValueToNode(%#v) = error %v", tc.val, err)
			}
			raw, err := yaml.Marshal(node)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			switch want := tc.val.(type) {
			case string:
				var got string
				if err := yaml.Unmarshal(raw, &got); err != nil {
					t.Fatalf("emitted %q, cannot read back: %v", raw, err)
				}
				if got != want {
					t.Errorf("round trip changed the value: emitted %q, got %q, want %q", raw, got, want)
				}
			case []string:
				var got []string
				if err := yaml.Unmarshal(raw, &got); err != nil {
					t.Fatalf("emitted %q, cannot read back: %v", raw, err)
				}
				if len(got) != len(want) {
					t.Fatalf("round trip changed length: got %q, want %q", got, want)
				}
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("element %d: got %q, want %q (emitted %q)", i, got[i], want[i], raw)
					}
				}
			case []any:
				var got []any
				if err := yaml.Unmarshal(raw, &got); err != nil {
					t.Fatalf("emitted %q, cannot read back: %v", raw, err)
				}
				if len(got) != len(want) {
					t.Fatalf("round trip changed length: got %v, want %v", got, want)
				}
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("element %d: got %v, want %v (emitted %q)", i, got[i], want[i], raw)
					}
				}
			}
		})
	}
}

// Values that do NOT trip the emitter must keep block style, so the fix does
// not reflow files it need not touch. Without this, quoting every multi-line
// string would churn the on-disk formatting of every existing entity whose
// property happens to span lines.
func TestValueToNode_LeavesOrdinaryMultilineAlone(t *testing.T) {
	node, err := ValueToNode("a\nb\nc")
	if err != nil {
		t.Fatalf("ValueToNode: %v", err)
	}
	if node.Style == yaml.DoubleQuotedStyle {
		t.Error("an ordinary multi-line string was quoted; only strings that break the emitter should be")
	}
}
