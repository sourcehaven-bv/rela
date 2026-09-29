package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A face column exports the face's declared label, falls back to its name,
// and stays empty for a row served outside any face.
func TestExport_ListFaceColumn(t *testing.T) {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"beleid": {Faces: map[string]metamodel.FaceDef{
			"vastgesteld": {Label: "Vastgesteld"},
			"concept":     {},
		}},
	}}
	col := dataentryconfig.ListColumn{Face: true}
	tests := []struct {
		name string
		face entityPkg.Face
		want string
	}{
		{"declared label", "vastgesteld", "Vastgesteld"},
		{"no label falls back to the name", "concept", "concept"},
		{"no face", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &entityPkg.Entity{ID: "POLICY-1", Type: "beleid", Face: tc.face}
			if got := columnCell(meta, e, col, nil); got != tc.want {
				t.Errorf("columnCell = %q, want %q", got, tc.want)
			}
		})
	}
	if got := columnLabel(col); got != "Face" {
		t.Errorf("columnLabel = %q, want Face", got)
	}
}
