package metamodel

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestFacesOf(t *testing.T) {
	m := &Metamodel{Entities: map[string]EntityDef{
		"doc":   {Faces: map[string]FaceDef{"draft": {}, "approved": {}}},
		"plain": {},
	}}
	tests := []struct {
		name string
		meta *Metamodel
		typ  string
		want []entity.Face
	}{
		{"faced type sorted without recorded order", m, "doc", []entity.Face{"approved", "draft"}},
		{"faceless type", m, "plain", nil},
		{"unknown type", m, "nope", nil},
		{"nil metamodel", nil, "doc", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FacesOf(tc.meta, tc.typ); !slices.Equal(got, tc.want) {
				t.Fatalf("FacesOf = %v, want %v", got, tc.want)
			}
		})
	}
}
