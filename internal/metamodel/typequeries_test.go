package metamodel

import (
	"slices"
	"testing"
)

func TestExpandTypeNames(t *testing.T) {
	m := mustLoadNS(t)
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil stays nil", nil, nil},
		{"empty stays empty", []string{}, []string{}},
		{"concrete kept", []string{"person"}, []string{"person"}},
		{"abstract expands in place", []string{"person", "task"}, []string{"person", "isms.task", "projects.task"}},
		{"duplicates dropped", []string{"isms.task", "task"}, []string{"isms.task", "projects.task"}},
		{"unknown kept", []string{"nope"}, []string{"nope"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandTypeNames(m, tc.in)
			if !slices.Equal(got, tc.want) || (got == nil) != (tc.want == nil) {
				t.Errorf("ExpandTypeNames(%v) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
	if got := ExpandTypeNames(nil, []string{"task"}); !slices.Equal(got, []string{"task"}) {
		t.Errorf("nil metamodel: got %v", got)
	}
}

func TestFaceQueries(t *testing.T) {
	m := mustLoadNS(t)
	tests := []struct {
		typ, face       string
		declares, faces bool
	}{
		{"isms.task", "draft", true, true},
		{"isms.task", "archived", false, true},
		{"person", "draft", false, false},
		{"nope", "draft", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.typ+"@"+tc.face, func(t *testing.T) {
			if got := DeclaresFace(m, tc.typ, tc.face); got != tc.declares {
				t.Errorf("DeclaresFace = %v, want %v", got, tc.declares)
			}
			if got := HasFaces(m, tc.typ); got != tc.faces {
				t.Errorf("HasFaces = %v, want %v", got, tc.faces)
			}
		})
	}
	if DeclaresFace(nil, "isms.task", "draft") || HasFaces(nil, "isms.task") {
		t.Error("nil metamodel declares a face")
	}
}

func TestEntityTypeNames(t *testing.T) {
	m := mustLoadNS(t)
	got := EntityTypeNames(m)
	if !slices.IsSorted(got) || !slices.Contains(got, "person") || !slices.Contains(got, "isms.task") {
		t.Errorf("EntityTypeNames = %v", got)
	}
	if EntityTypeNames(nil) != nil {
		t.Error("nil metamodel: want nil")
	}
}

func TestCopyTargetOf(t *testing.T) {
	m := &Metamodel{Copies: map[string]CopyDef{
		"publish": {To: "page@published"},
		"broken":  {To: ""},
	}}
	tests := []struct {
		name         string
		want         CopyTarget
		declared, ok bool
	}{
		{"publish", CopyTarget{Type: "page", Face: "published"}, true, true},
		{"broken", CopyTarget{}, true, false},
		{"missing", CopyTarget{}, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, declared, ok := CopyTargetOf(m, tc.name)
			if got != tc.want || declared != tc.declared || ok != tc.ok {
				t.Errorf("CopyTargetOf = %+v, %v, %v; want %+v, %v, %v",
					got, declared, ok, tc.want, tc.declared, tc.ok)
			}
		})
	}
	if _, declared, _ := CopyTargetOf(nil, "publish"); declared {
		t.Error("nil metamodel declares a copy")
	}
}
