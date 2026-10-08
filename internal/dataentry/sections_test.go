package dataentry

import (
	"reflect"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestPropertyToStrings(t *testing.T) {
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	moment := time.Date(2026, 9, 26, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name     string
		in       any
		propType string
		want     []string
	}{
		{"nil", nil, "", nil},
		{"empty string", "", "", nil},
		{"scalar string", "bug", "", []string{"bug"}},
		{"scalar int", 42, "", []string{"42"}},
		{"[]string", []string{"bug", "ui"}, "", []string{"bug", "ui"}},
		{"[]string with empty", []string{"bug", "", "ui"}, "", []string{"bug", "ui"}},
		{"[]string empty", []string{}, "", []string{}},
		{"[]any", []any{"bug", "ui"}, "", []string{"bug", "ui"}},
		{"[]any with mixed", []any{"bug", 42, true}, "", []string{"bug", "42", "true"}},
		{"[]any empty", []any{}, "", []string{}},
		// The fs backend decodes an unquoted YAML date to time.Time.
		{"date", day, metamodel.PropertyTypeDate, []string{"2026-09-26"}},
		{"datetime", moment, metamodel.PropertyTypeDatetime, []string{"2026-09-26T10:30:00Z"}},
		{"date as RFC 3339 string", "2026-09-26T00:00:00Z", metamodel.PropertyTypeDate, []string{"2026-09-26"}},
		{"datetime string kept", "2026-09-26T10:30:00Z", metamodel.PropertyTypeDatetime,
			[]string{"2026-09-26T10:30:00Z"}},
		{"external ref shows its id", map[string]any{"id": "42", "url": "https://x.test/42"},
			metamodel.PropertyTypeExternalRef, []string{"42"}},
		{"[]any dates", []any{day, day.AddDate(0, 0, 1)}, metamodel.PropertyTypeDate,
			[]string{"2026-09-26", "2026-09-27"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := propertyToStrings(tt.in, tt.propType)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("propertyToStrings(%#v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
