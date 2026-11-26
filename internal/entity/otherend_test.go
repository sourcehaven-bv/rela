package entity

import "testing"

func TestRelationOtherEnd(t *testing.T) {
	tests := []struct {
		name string
		rel  Relation
		id   string
		want string
	}{
		{"from side", Relation{From: "a", To: "b"}, "a", "b"},
		{"to side", Relation{From: "a", To: "b"}, "b", "a"},
		{"self loop", Relation{From: "a", To: "a"}, "a", "a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rel.OtherEnd(tc.id); got != tc.want {
				t.Fatalf("OtherEnd(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}
