package projectsetup_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestValidateWithFS_NoProject(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := projectsetup.ValidateWithFS("/missing", fs)
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
}

func TestValidateWithFS_ValidProject(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := projectsetup.ValidateWithFS(root, fs)
	if err != nil {
		t.Fatalf("ValidateWithFS: %v", err)
	}
	if !result.MetamodelValid {
		t.Errorf("MetamodelValid = false, err = %v", result.MetamodelError)
	}
	if result.HasErrors() {
		t.Errorf("HasErrors() = true, want false")
	}
}

func TestValidateResult_HasErrors(t *testing.T) {
	cases := []struct {
		name string
		r    projectsetup.ValidateResult
		want bool
	}{
		{"clean", projectsetup.ValidateResult{}, false},
		{"metamodel err", projectsetup.ValidateResult{MetamodelError: errExample}, true},
		{"data-entry err", projectsetup.ValidateResult{DataEntryError: errExample}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.HasErrors(); got != tc.want {
				t.Errorf("HasErrors() = %v, want %v", got, tc.want)
			}
		})
	}
}

type sentinel struct{ msg string }

func (s sentinel) Error() string { return s.msg }

var errExample = sentinel{msg: "x"}

// TestValidateWithFS_PointerGrammar pins that `rela validate` checks the
// pointer NAME GRAMMAR, not just world structure (TKT-WAV8XP).
//
// The two halves live in different packages: the loader validates world
// structure, while the grammar is enforced by the compiler, because
// metamodel may not import entity under arch-lint. Every validation entry
// point therefore has to call both — and this is the command whose entire
// job is answering "is my schema valid?". Before the compiler call was
// added, `validate` reported "All configuration files are valid" on a
// schema that failed at startup for every other command, which is the
// worst possible direction for a pre-commit or CI gate.
func TestValidateWithFS_PointerGrammar(t *testing.T) {
	const schema = `version: "1.0"
entities:
  page:
    label: Page
    plural: pages
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title: {type: string}
    pointers:
      %s: {default: true}
      published: {}
`

	validateOver := func(t *testing.T, pointerName string) *projectsetup.ValidateResult {
		t.Helper()
		fs := storage.NewMemFS()
		root := "/proj"
		if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
			t.Fatalf("init: %v", err)
		}
		body := fmt.Sprintf(schema, pointerName)
		if err := fs.WriteFile(root+"/schema.yaml", []byte(body), 0o644); err != nil {
			t.Fatalf("write schema: %v", err)
		}
		result, err := projectsetup.ValidateWithFS(root, fs)
		if err != nil {
			t.Fatalf("ValidateWithFS: %v", err)
		}
		return result
	}

	t.Run("a legal pointer name validates", func(t *testing.T) {
		result := validateOver(t, "draft")
		if !result.MetamodelValid {
			t.Fatalf("MetamodelValid = false, err = %v", result.MetamodelError)
		}
	})

	t.Run("an illegal pointer name fails validation", func(t *testing.T) {
		// `Draft` is rejected by the grammar (no uppercase). The loader's
		// structural checks pass it; only the compiler catches it.
		result := validateOver(t, "Draft")
		if result.MetamodelValid {
			t.Fatal("validate must reject an invalid pointer name")
		}
		if !result.HasErrors() {
			t.Error("HasErrors() = false, want true")
		}
		if result.MetamodelError == nil {
			t.Fatal("MetamodelError must be set")
		}
		if !strings.Contains(result.MetamodelError.Error(), "Draft") {
			t.Errorf("error must name the offending pointer, got: %v", result.MetamodelError)
		}
	})
}
