package metamodel

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse_Checkers(t *testing.T) {
	tests := []struct {
		name       string
		entities   string
		checkers   string
		wantErrHas string
	}{
		{
			name: "valid vale checker",
			checkers: `
checkers:
  style:
    command: ["vale", "--output=JSON", "{in}"]
    format: vale
    exit_codes: [0, 1]
    timeout: 5s
    analyze: true
`,
		},
		{
			name: "empty command rejected",
			checkers: `
checkers:
  style:
    command: []
    format: rela
`,
			wantErrHas: `checker "style": command is required`,
		},
		{
			name: "empty binary rejected",
			checkers: `
checkers:
  style:
    command: [" ", "{in}"]
    format: rela
`,
			wantErrHas: "command[0] (the binary) is empty",
		},
		{
			name: "missing format rejected",
			checkers: `
checkers:
  style:
    command: ["x"]
`,
			wantErrHas: "format is required",
		},
		{
			name: "unknown format rejected",
			checkers: `
checkers:
  style:
    command: ["x"]
    format: languagetool
`,
			wantErrHas: `unsupported format "languagetool"`,
		},
		{
			name: "exit code out of range rejected",
			checkers: `
checkers:
  style:
    command: ["x"]
    format: rela
    exit_codes: [256]
`,
			wantErrHas: "exit code 256 is out of range",
		},
		{
			name: "unparseable timeout rejected",
			checkers: `
checkers:
  style:
    command: ["x"]
    format: rela
    timeout: soon
`,
			wantErrHas: "invalid timeout",
		},
		{
			name: "timeout above cap rejected",
			checkers: `
checkers:
  style:
    command: ["x"]
    format: rela
    timeout: 5m
`,
			wantErrHas: "must be above 0 and at most",
		},
		{
			name: "unsafe type name rejected when checkers are declared",
			entities: `
  "review response":
    label: Review response
    id_prefix: "RR-"
    id_type: sequential
    properties:
      title:
        type: string
`,
			checkers: `
checkers:
  style:
    command: ["x"]
    format: rela
`,
			wantErrHas: `entity type "review response": with checkers declared`,
		},
		{
			name: "unsafe type name allowed without checkers",
			entities: `
  "review response":
    label: Review response
    id_prefix: "RR-"
    id_type: sequential
    properties:
      title:
        type: string
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(transformsBaseEntities + tc.entities + tc.checkers))
			if tc.wantErrHas == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrHas) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErrHas)
			}
		})
	}
}

func TestCheckableTypeName(t *testing.T) {
	for _, ok := range []string{"procedure", "review-response", "task_2", "A1"} {
		if !CheckableTypeName.MatchString(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", ".hidden", "-x", "a/b", "a b", "a*", "a?", "a[b]", "a{b}", "ä", "a.b"} {
		if CheckableTypeName.MatchString(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCheckerDef_Defaults(t *testing.T) {
	m, err := Parse([]byte(transformsBaseEntities + `
checkers:
  style:
    command: ["x", "{in}"]
    format: rela
`))
	if err != nil {
		t.Fatal(err)
	}
	def := m.Checkers["style"]
	if got := def.TimeoutDuration(); got != DefaultCheckerTimeout {
		t.Errorf("timeout = %s, want %s", got, DefaultCheckerTimeout)
	}
	if got := def.AcceptedExitCodes(); len(got) != 1 || got[0] != 0 {
		t.Errorf("exit codes = %v, want [0]", got)
	}
	if def.Analyze {
		t.Error("analyze defaults to false")
	}

	m, err = Parse([]byte(transformsBaseEntities + `
checkers:
  style:
    command: ["x"]
    format: vale
    exit_codes: [0, 1]
    timeout: 3s
`))
	if err != nil {
		t.Fatal(err)
	}
	def = m.Checkers["style"]
	if got := def.TimeoutDuration(); got != 3*time.Second {
		t.Errorf("timeout = %s", got)
	}
	if got := def.AcceptedExitCodes(); len(got) != 2 || got[1] != 1 {
		t.Errorf("exit codes = %v", got)
	}
}

func TestLoadWithIncludes_IncludeHasCheckers(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - bad.yaml
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "bad.yaml"), `
checkers:
  style:
    command: ["vale", "{in}"]
    format: vale
`)

	_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var rootFieldErr *IncludeHasRootFieldError
	if !errors.As(err, &rootFieldErr) {
		t.Fatalf("expected IncludeHasRootFieldError, got %T: %v", err, err)
	}
	assertEqual(t, rootFieldErr.Field, "checkers")
}
