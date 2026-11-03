package ingestspec

import "testing"

func TestMappingCreateType(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    Mapping
		want string
	}{
		{"no create", Mapping{Find: &Find{Type: "a"}}, ""},
		{"explicit type", Mapping{Find: &Find{Type: "a"}, CreateIfMissing: &Create{Type: "b"}}, "b"},
		{"falls back to find", Mapping{Find: &Find{Type: "a"}, CreateIfMissing: &Create{}}, "a"},
		{"nothing to fall back to", Mapping{CreateIfMissing: &Create{}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.CreateType(); got != tc.want {
				t.Errorf("CreateType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMappingNeedsBody(t *testing.T) {
	set := Step{Set: map[string]string{"a": "b"}}
	appendStep := Step{AppendSection: &AppendSection{Section: "Log", Content: "x"}}
	for _, tc := range []struct {
		name string
		then []Step
		want bool
	}{
		{"no steps", nil, false},
		{"set only", []Step{set}, false},
		{"append", []Step{set, appendStep}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Mapping{Then: tc.then}).NeedsBody(); got != tc.want {
				t.Errorf("NeedsBody() = %v, want %v", got, tc.want)
			}
		})
	}
}
