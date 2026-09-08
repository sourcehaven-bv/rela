package appbuild

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestDeclaresFaces pins the gate on the coarse boot-time state probe
// (TKT-WAV8XP).
//
// Under TKT-DOFYR1 no face was declarable, so any state row was
// stranded data and the two-COUNT probe was unconditional. Once a project
// can declare content states, that probe would warn about perfectly
// declared drafts on every boot — and warning fatigue on a boot
// diagnostic is how the genuinely stranded case stops being noticed. So
// the coarse probe steps aside for `analyze states`, which subtracts the
// declared set per type.
func TestDeclaresFaces(t *testing.T) {
	tests := []struct {
		name string
		meta *metamodel.Metamodel
		want bool
	}{
		{
			name: "nil metamodel",
			meta: nil,
			want: false,
		},
		{
			name: "no entities",
			meta: &metamodel.Metamodel{},
			want: false,
		},
		{
			name: "faceless project keeps the DOFYR1 probe",
			meta: &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
				"ticket": {Label: "Ticket"},
				"page":   {Label: "Page"},
			}},
			want: false,
		},
		{
			name: "one type declaring states silences the coarse probe",
			meta: &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
				"ticket": {Label: "Ticket"},
				"page": {Label: "Page", BareFace: "draft",
					Faces: map[string]metamodel.FaceDef{
						"draft": {},
					}},
			}},
			want: true,
		},
		{
			name: "an empty faces map is not a declaration",
			meta: &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
				"page": {Label: "Page", Faces: map[string]metamodel.FaceDef{}},
			}},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := declaresFaces(tc.meta); got != tc.want {
				t.Errorf("declaresFaces() = %v, want %v", got, tc.want)
			}
		})
	}
}
