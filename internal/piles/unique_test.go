package piles

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestFirstUnique(t *testing.T) {
	a, b, c := entity.Ref{ID: "a"}, entity.Ref{ID: "b"}, entity.Ref{ID: "c"}
	tests := []struct {
		name  string
		refs  []entity.Ref
		limit int
		want  []entity.Ref
	}{
		{"empty", nil, 3, []entity.Ref{}},
		{"dedupes keeping first", []entity.Ref{a, b, a, c}, 5, []entity.Ref{a, b, c}},
		{"caps at limit", []entity.Ref{a, a, b, c}, 2, []entity.Ref{a, b}},
		{"zero limit", []entity.Ref{a}, 0, []entity.Ref{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FirstUnique(tc.refs, tc.limit); !slices.Equal(got, tc.want) {
				t.Fatalf("FirstUnique = %v, want %v", got, tc.want)
			}
		})
	}
}
