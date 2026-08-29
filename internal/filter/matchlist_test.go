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

	// Each case runs twice: once with a []string property and once with the
	// []any shape YAML frontmatter actually decodes to. They must agree — a
	// divergence there is invisible in production, since only the decoder
	// decides which one a given entity carries.
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
		anyList := make([]any, len(tc.list))
		for i, s := range tc.list {
			anyList[i] = s
		}
		shapes := map[string]any{"[]string": tc.list, "[]any": anyList}

		for shape, prop := range shapes {
			t.Run(tc.name+"/"+shape, func(t *testing.T) {
				rec := Record{Properties: map[string]any{"tags": prop}}
				f := &Filter{Property: "tags", Operator: tc.op, Value: tc.value}
				got, err := Match(rec, f, propDef, m)
				if err != nil {
					t.Fatalf("Match returned error: %v", err)
				}
				if got != tc.want {
					t.Errorf("%s %v %s %q: got %v, want %v", shape, tc.list, tc.op, tc.value, got, tc.want)
				}
			})
		}
	}
}
