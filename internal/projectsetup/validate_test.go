package projectsetup_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestValidateWithFS_NoProject(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := projectsetup.ValidateWithFS(context.Background(), "/missing", fs)
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
	result, err := projectsetup.ValidateWithFS(context.Background(), root, fs)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchedulesError == nil {
		t.Fatalf("expected unknown template error; metamodel=%v mail=%v", result.MetamodelError, result.MailTemplatesError)
	}
}

func TestValidateWithFS_ComputedCycle(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}
	const schema = `version: "1.0"
entities:
  item:
    label: Item
    id_prefix: ITEM-
    properties:
      a: {type: integer, computed: "entity.b + 1"}
      b: {type: integer, computed: "entity.a + 1"}
`
	if err := fs.WriteFile(filepath.Join(root, "schema.yaml"), []byte(schema), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	result, err := projectsetup.ValidateWithFS(context.Background(), root, fs)
	if err != nil {
		t.Fatalf("ValidateWithFS: %v", err)
	}
	if result.MetamodelError == nil || !strings.Contains(result.MetamodelError.Error(), "cycle") {
		t.Fatalf("MetamodelError = %v, want computed cycle", result.MetamodelError)
	}
}

func TestValidateWithFS_ValidProject(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := projectsetup.ValidateWithFS(context.Background(), root, fs)
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

// rela validate refuses a connections.yaml that assembly would refuse, and
// accepts a project without one (TKT-01KZSO).
func TestValidateWithFS_Connections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		body    string // "" leaves the file out
		wantErr string
	}{
		{name: "absent"},
		{name: "valid", body: `connections:
  svc:
    token_url: https://auth.example.com/token
    client_id_secret: svc_id
    client_secret_secret: svc_secret
    style: rfc6749
    user_agent: "test (ops@example.com)"
`},
		{name: "unknown key", body: "connections:\n  svc:\n    tokn_url: x\n", wantErr: "tokn_url"},
		{name: "http token url", body: `connections:
  svc:
    token_url: http://auth.example.com/token
    client_id_secret: svc_id
    client_secret_secret: svc_secret
    style: rfc6749
    user_agent: "test (ops@example.com)"
`, wantErr: "https"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := storage.NewMemFS()
			root := "/proj"
			if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
				t.Fatal(err)
			}
			if tc.body != "" {
				if err := fs.WriteFile(filepath.Join(root, "connections.yaml"), []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := projectsetup.ValidateWithFS(context.Background(), root, fs)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantErr == "" {
				if result.ConnectionsError != nil {
					t.Fatalf("ConnectionsError = %v, want nil", result.ConnectionsError)
				}
				return
			}
			if result.ConnectionsError == nil || !strings.Contains(result.ConnectionsError.Error(), tc.wantErr) {
				t.Fatalf("ConnectionsError = %v, want one naming %q", result.ConnectionsError, tc.wantErr)
			}
		})
	}
}

func TestValidateWithFS_Subscriptions(t *testing.T) {
	t.Parallel()
	const valid = `subscriptions:
  news:
    url: https://example.com/feed.xml
    every: 1h
    find: {type: requirement, match: [title], values: {title: "{{item.title}}"}}
    create_if_missing: {}
`
	for _, tc := range []struct{ name, body, wantErr string }{
		{name: "absent"},
		{name: "valid", body: valid},
		{name: "unknown type", body: strings.Replace(valid, "requirement", "post", 1), wantErr: `"post"`},
		{name: "every too short", body: strings.Replace(valid, "1h", "1m", 1), wantErr: "minimum"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := storage.NewMemFS()
			root := "/proj"
			if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
				t.Fatal(err)
			}
			if tc.body != "" {
				if err := fs.WriteFile(filepath.Join(root, "subscriptions.yaml"), []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := projectsetup.ValidateWithFS(context.Background(), root, fs)
			if err != nil {
				t.Fatal(err)
			}
			got := result.SubscriptionsError
			if tc.wantErr == "" && got != nil {
				t.Fatalf("SubscriptionsError = %v, want nil", got)
			}
			if tc.wantErr != "" && (got == nil || !strings.Contains(got.Error(), tc.wantErr)) {
				t.Fatalf("SubscriptionsError = %v, want one naming %q", got, tc.wantErr)
			}
		})
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
		{"connections err", projectsetup.ValidateResult{ConnectionsError: errExample}, true},
		{"subscriptions err", projectsetup.ValidateResult{SubscriptionsError: errExample}, true},
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

// TestValidateWithFS_FaceGrammar pins that `rela validate` checks the
// face NAME GRAMMAR, not just world structure (TKT-WAV8XP).
//
// The two halves live in different packages: the loader validates world
// structure, while the grammar is enforced by the compiler, because
// metamodel may not import entity under arch-lint. Every validation entry
// point therefore has to call both — and this is the command whose entire
// job is answering "is my schema valid?". Before the compiler call was
// added, `validate` reported "All configuration files are valid" on a
// schema that failed at startup for every other command, which is the
// worst possible direction for a pre-commit or CI gate.
func TestValidateWithFS_FaceGrammar(t *testing.T) {
	const schema = `version: "1.0"
entities:
  page:
    label: Page
    plural: pages
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title: {type: string}
    faces:
      %s: {default: true}
      published: {}
`

	validateOver := func(t *testing.T, faceName string) *projectsetup.ValidateResult {
		t.Helper()
		fs := storage.NewMemFS()
		root := "/proj"
		if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
			t.Fatalf("init: %v", err)
		}
		body := fmt.Sprintf(schema, faceName)
		if err := fs.WriteFile(root+"/schema.yaml", []byte(body), 0o644); err != nil {
			t.Fatalf("write schema: %v", err)
		}
		result, err := projectsetup.ValidateWithFS(context.Background(), root, fs)
		if err != nil {
			t.Fatalf("ValidateWithFS: %v", err)
		}
		return result
	}

	t.Run("a legal face name validates", func(t *testing.T) {
		result := validateOver(t, "draft")
		if !result.MetamodelValid {
			t.Fatalf("MetamodelValid = false, err = %v", result.MetamodelError)
		}
	})

	t.Run("an illegal face name fails validation", func(t *testing.T) {
		// `Draft` is rejected by the grammar (no uppercase). The loader's
		// structural checks pass it; only the compiler catches it.
		result := validateOver(t, "Draft")
		if result.MetamodelValid {
			t.Fatal("validate must reject an invalid face name")
		}
		if !result.HasErrors() {
			t.Error("HasErrors() = false, want true")
		}
		if result.MetamodelError == nil {
			t.Fatal("MetamodelError must be set")
		}
		if !strings.Contains(result.MetamodelError.Error(), "Draft") {
			t.Errorf("error must name the offending face, got: %v", result.MetamodelError)
		}
	})
}

// TestCheckChannels pins that only a non-empty channels: section or a
// kanban busy field is checked, and that realtime off makes it a warning.
func TestCheckChannels(t *testing.T) {
	for _, data := range []string{
		"", "version: \"1.0\"\n", "channels: {}\n", "channels: [broken",
		"kanbans:\n  board:\n    card:\n      fields: [{ property: title }]\n",
	} {
		if projectsetup.HasRealtimePresentation([]byte(data)) {
			t.Errorf("HasRealtimePresentation(%q) = true", data)
		}
		if errs, warnings := projectsetup.CheckChannels([]byte(data), nil); errs != nil || warnings != nil {
			t.Errorf("CheckChannels(%q) = %v, %v", data, errs, warnings)
		}
	}
	for _, data := range []string{
		"channels:\n  room: {}\n",
		"kanbans:\n  board:\n    entity_type: ticket\n    card:\n      fields: [{ busy: run }]\n",
	} {
		if !projectsetup.HasRealtimePresentation([]byte(data)) {
			t.Fatalf("HasRealtimePresentation missed %q", data)
		}
		errs, warnings := projectsetup.CheckChannels([]byte(data), nil)
		if len(errs) != 0 || len(warnings) != 1 {
			t.Fatalf("realtime off, %q: errs %v, warnings %v", data, errs, warnings)
		}
	}
}
