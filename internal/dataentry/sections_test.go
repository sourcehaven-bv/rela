package dataentry

import (
	"reflect"
	"testing"
)

func TestPropertyToStrings(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, nil},
		{"empty string", "", nil},
		{"scalar string", "bug", []string{"bug"}},
		{"scalar int", 42, []string{"42"}},
		{"[]string", []string{"bug", "ui"}, []string{"bug", "ui"}},
		{"[]string with empty", []string{"bug", "", "ui"}, []string{"bug", "ui"}},
		{"[]string empty", []string{}, []string{}},
		{"[]any", []any{"bug", "ui"}, []string{"bug", "ui"}},
		{"[]any with mixed", []any{"bug", 42, true}, []string{"bug", "42", "true"}},
		{"[]any empty", []any{}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := propertyToStrings(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("propertyToStrings(%#v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
