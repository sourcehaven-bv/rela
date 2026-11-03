package acl

import (
	"testing"
)

// TestFieldCeilingHides pins both ceiling spellings.
func TestFieldCeilingHides(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ceiling FieldCeiling
		field   string
		want    bool
	}{
		{"no ceiling", FieldCeiling{}, "salary", false},
		{"redact names it", FieldCeiling{Redact: []string{"salary"}}, "salary", true},
		{"redact names another", FieldCeiling{Redact: []string{"bsn"}}, "salary", false},
		{"visible omits it", FieldCeiling{Visible: []string{"name"}}, "salary", true},
		{"visible names it", FieldCeiling{Visible: []string{"salary"}}, "salary", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ceiling.Hides(tc.field); got != tc.want {
				t.Errorf("Hides(%q) = %v, want %v", tc.field, got, tc.want)
			}
		})
	}
}
