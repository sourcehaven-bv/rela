package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestBuild_V1ToV2(t *testing.T) {
	spec := Spec{
		Description: "Status rework",
		From:        metamodel.NewShapeProjection(metaV1()),
		To:          metamodel.NewShapeProjection(metaV2()),
		Steps: []SpecStep{
			{RenameProperty: &RenamePropertySpec{Entity: "task", From: "status", To: "state"}},
			{MapValues: &MapValuesSpec{Entity: "task", Property: "state", Mapping: map[string]string{"open": "todo", "wip": "doing"}}},
			{Convert: &ConvertSpec{Entity: "task", Property: "due", ToType: "date"}},
		},
	}
	d, err := Build(spec, testNow())
	if err != nil {
		t.Fatal(err)
	}
	f := mustParse(t, d.FileName, d.Content)
	if len(f.Steps) != 3 || f.Description != "Status rework" {
		t.Fatalf("parsed %d steps, description %q", len(f.Steps), f.Description)
	}
	if !strings.HasSuffix(d.FileName, "-status-rework.yaml") {
		t.Errorf("file name %q", d.FileName)
	}
}

func TestBuild_Refuses(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{"step for a missing property", Spec{
			Description: "x", From: metamodel.NewShapeProjection(metaV1()), To: metamodel.NewShapeProjection(metaV1()),
			Steps: []SpecStep{{RenameProperty: &RenamePropertySpec{Entity: "task", From: "nope", To: "x"}}},
		}, "nope"},
		{"no description", Spec{From: metamodel.NewShapeProjection(metaV1()), To: metamodel.NewShapeProjection(metaV2())}, "description"},
		{"two kinds in one step", Spec{
			Description: "x", From: metamodel.NewShapeProjection(metaV1()), To: metamodel.NewShapeProjection(metaV1()),
			Steps: []SpecStep{{
				RenameProperty: &RenamePropertySpec{Entity: "task", From: "due", To: "x"},
				SetDefault:     &SetDefaultSpec{Entity: "task", Property: "due", Value: "x"},
			}},
		}, "exactly one"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.spec, testNow())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestBuild_NothingToMigrate(t *testing.T) {
	p := metamodel.NewShapeProjection(metaV1())
	d, err := Build(Spec{Description: "x", From: p, To: p}, testNow())
	if err != nil || d != nil {
		t.Fatalf("got %v, %v", d, err)
	}
}
