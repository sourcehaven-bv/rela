package projectsetup_test

import (
	"path/filepath"
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

func TestValidateWithFS_ValidatesScheduledMailReferences(t *testing.T) {
	t.Parallel()
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := fs.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("schema.yaml", `entities:
  person:
    label: Person
    id_type: manual
    properties:
      email: {type: string}
`)
	write("mail-templates.yaml", `mail_templates:
  digest:
    subject: Digest
    address_property: email
`)
	write("schedules.yaml", `tasks:
  - name: digest
    template: missing
    every: day
    for_each: {entity_type: person}
`)
	result, err := projectsetup.ValidateWithFS(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchedulesError == nil {
		t.Fatalf("expected unknown template error; metamodel=%v mail=%v", result.MetamodelError, result.MailTemplatesError)
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
		{"mail err", projectsetup.ValidateResult{MailTemplatesError: errExample}, true},
		{"schedules err", projectsetup.ValidateResult{SchedulesError: errExample}, true},
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
