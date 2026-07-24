package transform

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestRegistryFromMetamodel(t *testing.T) {
	// The metamodel loader canonicalizes `from` (an omitted value is written
	// back as markdown at load), so the projection is a pure shape conversion
	// of already-resolved values.
	m := &metamodel.Metamodel{
		Transforms: map[string]metamodel.TransformDef{
			"pdf":  {From: "markdown", Command: []string{"pandoc", "{in}", "{out}"}, Produces: "application/pdf"},
			"docx": {From: "markdown", Command: []string{"x"}, Produces: "text/plain"},
		},
	}
	reg := RegistryFromMetamodel(m)
	if len(reg) != 2 {
		t.Fatalf("want 2 entries, got %d", len(reg))
	}
	if reg["pdf"].Produces != "application/pdf" || reg["pdf"].From != FormatMarkdown {
		t.Errorf("pdf = %+v", reg["pdf"])
	}
	// FromMarkdown surfaces both, sorted.
	fm := reg.FromMarkdown()
	if len(fm) != 2 || fm[0].Name != "docx" || fm[1].Name != "pdf" {
		t.Errorf("FromMarkdown = %+v", fm)
	}
}

func TestRegistryFromMetamodel_Empty(t *testing.T) {
	reg := RegistryFromMetamodel(&metamodel.Metamodel{})
	if reg == nil {
		t.Fatal("registry should be non-nil")
	}
	if len(reg) != 0 {
		t.Errorf("want empty, got %d", len(reg))
	}
}
