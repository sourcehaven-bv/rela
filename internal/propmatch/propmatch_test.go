package propmatch_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/propmatch"
)

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want bool
	}{
		{"nil is empty", nil, true},
		{"empty string is empty", "", true},
		{"absent key reads as nil", map[string]any{}["missing"], true},
		{"non-empty string", "doing", false},
		{"whitespace is NOT empty", " ", false},
		{"zero int is not empty", 0, false},
		{"false is not empty", false, false},
		{"empty []string is empty", []string{}, true},
		{"nil []string is empty", []string(nil), true},
		{"populated []string", []string{"a"}, false},
		{"empty []any is empty", []any{}, true},
		{"populated []any", []any{"a"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := propmatch.IsEmpty(tc.val); got != tc.want {
				t.Errorf("IsEmpty(%#v) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestDecide_EmptySemantics pins the four-line rule documented in
// internal/filter/match.go. These four cases are the contract; if one
// changes, the store and filter layers have diverged.
func TestDecide_EmptySemantics(t *testing.T) {
	for _, empty := range []any{nil, "", []string{}} {
		tests := []struct {
			name   string
			op     propmatch.Op
			target string
			want   propmatch.Result
		}{
			{"property=value -> NoMatch", propmatch.OpEqual, "doing", propmatch.NoMatch},
			{"property!=value -> NoMatch", propmatch.OpNotEqual, "doing", propmatch.NoMatch},
			{"property= -> Match", propmatch.OpEqual, "", propmatch.Match},
			{"property!= -> NoMatch", propmatch.OpNotEqual, "", propmatch.NoMatch},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				if got := propmatch.Decide(empty, tc.op, tc.target); got != tc.want {
					t.Errorf("Decide(%#v, op=%v, %q) = %v, want %v",
						empty, tc.op, tc.target, got, tc.want)
				}
			})
		}
	}
}

func TestDecide_NonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		val    any
		op     propmatch.Op
		target string
		want   propmatch.Result
	}{
		{"equal matches", "doing", propmatch.OpEqual, "doing", propmatch.Match},
		{"unequal does not", "doing", propmatch.OpEqual, "todo", propmatch.NoMatch},
		{"not-equal on different", "doing", propmatch.OpNotEqual, "todo", propmatch.Match},
		{"not-equal on same", "doing", propmatch.OpNotEqual, "doing", propmatch.NoMatch},

		// Existence checks against a populated value.
		{"is-empty on populated", "doing", propmatch.OpEqual, "", propmatch.NoMatch},
		{"is-not-empty on populated", "doing", propmatch.OpNotEqual, "", propmatch.Match},

		// Multi-select: any element matching is a match.
		{"list any-match", []string{"a", "b"}, propmatch.OpEqual, "b", propmatch.Match},
		{"list no-match", []string{"a", "b"}, propmatch.OpEqual, "c", propmatch.NoMatch},
		{"list not-equal excludes member", []string{"a", "b"}, propmatch.OpNotEqual, "b", propmatch.NoMatch},
		{"list not-equal on absent", []string{"a", "b"}, propmatch.OpNotEqual, "c", propmatch.Match},
		{"[]any any-match", []any{"a", "b"}, propmatch.OpEqual, "b", propmatch.Match},

		// Non-string scalars compare by string form.
		{"int equal", 3, propmatch.OpEqual, "3", propmatch.Match},
		{"int unequal", 3, propmatch.OpEqual, "4", propmatch.NoMatch},
		{"bool equal", true, propmatch.OpEqual, "true", propmatch.Match},
		{"zero int is not empty", 0, propmatch.OpNotEqual, "", propmatch.Match},
		{"false is not empty", false, propmatch.OpNotEqual, "", propmatch.Match},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := propmatch.Decide(tc.val, tc.op, tc.target); got != tc.want {
				t.Errorf("Decide(%#v, op=%v, %q) = %v, want %v",
					tc.val, tc.op, tc.target, got, tc.want)
			}
		})
	}
}

// TestDecide_ExclusionDoesNotWiden guards the asymmetry that is easiest
// to "fix" wrongly: an unset property must NOT satisfy `prop!=value`,
// or every exclusion filter silently grows to include unset rows.
func TestDecide_ExclusionDoesNotWiden(t *testing.T) {
	if got := propmatch.Decide(nil, propmatch.OpNotEqual, "doing"); got != propmatch.NoMatch {
		t.Fatalf("unset property matched an exclusion filter: got %v, want NoMatch", got)
	}
}

func TestStringify(t *testing.T) {
	tests := []struct {
		val  any
		want string
	}{
		{nil, ""},
		{"s", "s"},
		{"", ""},
		{3, "3"},
		{true, "true"},
	}
	for _, tc := range tests {
		if got := propmatch.Stringify(tc.val); got != tc.want {
			t.Errorf("Stringify(%#v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}
