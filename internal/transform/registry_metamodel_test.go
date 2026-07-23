package transform

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestRegistryFromMetamodel(t *testing.T) {
	m := &metamodel.Metamodel{
		Transforms: map[string]metamodel.TransformDef{
			"pdf":  {From: "markdown", Command: []string{"pandoc", "{in}", "{out}"}, Produces: "application/pdf"},
			"bare": {Command: []string{"x"}, Produces: "text/plain"}, // from omitted -> markdown
		},
	}
	reg := RegistryFromMetamodel(m)
	if len(reg) != 2 {
		t.Fatalf("want 2 entries, got %d", len(reg))
	}
	if reg["pdf"].Produces != "application/pdf" || reg["pdf"].From != FormatMarkdown {
		t.Errorf("pdf = %+v", reg["pdf"])
	}
	if reg["bare"].From != FormatMarkdown {
		t.Errorf("bare.From = %q, want defaulted to markdown", reg["bare"].From)
	}
	// FromMarkdown surfaces both, sorted.
	fm := reg.FromMarkdown()
	if len(fm) != 2 || fm[0].Name != "bare" || fm[1].Name != "pdf" {
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
