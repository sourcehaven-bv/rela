package entitymanager

import (
	"testing"
	"time"
)

func TestTaskItemValueText(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{
		{"string", "apollo", "apollo"},
		{"date", time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC), "2026-10-21"},
		{"list", []any{"a", "", "b"}, "a, b"},
		{"strings", []string{"a", "b"}, "a, b"},
		{"large float", float64(1000000), "1000000"},
		{"fraction", 2.5, "2.5"},
		{"integer", 42, "42"},
		{"boolean", true, "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := taskItemValueText(tc.value); got != tc.want {
				t.Fatalf("taskItemValueText(%v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
