package entity

import "testing"

func TestIsReservedEntityKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"id", "id", true},
		{"type", "type", true},
		{"from is not an entity key", "from", false},
		{"relation is not an entity key", "relation", false},
		{"to is not an entity key", "to", false},
		{"ordinary property", "title", false},
		{"case sensitive", "ID", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsReservedEntityKey(tc.key); got != tc.want {
				t.Errorf("IsReservedEntityKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestIsReservedRelationKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"from", "from", true},
		{"relation", "relation", true},
		{"to", "to", true},
		{"type is not a relation frontmatter key", "type", false},
		{"id is not a relation key", "id", false},
		{"ordinary property", "since", false},
		{"case sensitive", "From", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsReservedRelationKey(tc.key); got != tc.want {
				t.Errorf("IsReservedRelationKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}
