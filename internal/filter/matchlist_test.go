package filter

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestMatchList pins the list-matching contract directly. matchList is the
// reference implementation the rest of the codebase is expected to agree with
// (see internal/dataentry propertyElements, added for BUG-AMK38R), but it had
// no direct test — the bug it would have caught shipped in a second copy.
func TestMatchList(t *testing.T) {
	m := &metamodel.Metamodel{}
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}

	tests := []struct {
		name  string
		list  []string
		op    Operator
		value string
		want  bool
	}{
		{"equal matches first element", []string{"a", "b"}, OpEqual, "a", true},
		{"equal matches later element", []string{"a", "b"}, OpEqual, "b", true},
		{"equal no element matches", []string{"a", "b"}, OpEqual, "c", false},
		{"equal does not match joined form", []string{"a", "b"}, OpEqual, "a b", false},
		{"not-equal false when an element matches", []string{"a", "b"}, OpNotEqual, "a", false},
		{"not-equal true when none match", []string{"a", "b"}, OpNotEqual, "c", true},
		{"single element list behaves like scalar", []string{"solo"}, OpEqual, "solo", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := Record{Properties: map[string]any{"tags": tc.list}}
			f := &Filter{Property: "tags", Operator: tc.op, Value: tc.value}
			got, err := Match(rec, f, propDef, m)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if got != tc.want {
				t.Errorf("list %v %s %q: got %v, want %v", tc.list, tc.op, tc.value, got, tc.want)
			}
		})
	}
}

// TestMatchList_AnyElementViaAnySlice pins that a []any property (the shape
// YAML frontmatter actually decodes to) is treated the same as []string.
func TestMatchList_AnyElementViaAnySlice(t *testing.T) {
	m := &metamodel.Metamodel{}
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}

	rec := Record{Properties: map[string]any{"tags": []any{"urgent", "blocker"}}}
	f := &Filter{Property: "tags", Operator: OpEqual, Value: "blocker"}

	got, err := Match(rec, f, propDef, m)
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !got {
		t.Error("[]any list should match an element, got false")
	}
}
