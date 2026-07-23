package metamodel

import (
	"strings"
	"testing"
)

const transformsBaseEntities = `
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title:
        type: string
`

func TestParse_Transforms(t *testing.T) {
	tests := []struct {
		name       string
		transforms string
		wantErr    bool
		wantErrHas string
	}{
		{
			name: "valid pdf transform",
			transforms: `
transforms:
  pdf:
    from: markdown
    command: ["pandoc", "-o", "{out}", "{in}"]
    produces: application/pdf
`,
			wantErr: false,
		},
		{
			name: "from defaults to markdown when omitted",
			transforms: `
transforms:
  pdf:
    command: ["pandoc"]
    produces: application/pdf
`,
			wantErr: false,
		},
		{
			name: "empty command rejected",
			transforms: `
transforms:
  pdf:
    from: markdown
    command: []
    produces: application/pdf
`,
			wantErr:    true,
			wantErrHas: "command is required",
		},
		{
			name: "missing produces rejected",
			transforms: `
transforms:
  pdf:
    command: ["pandoc"]
`,
			wantErr:    true,
			wantErrHas: "produces (content-type) is required",
		},
		{
			name: "malformed produces rejected",
			transforms: `
transforms:
  pdf:
    command: ["pandoc"]
    produces: "not a media type at all"
`,
			wantErr:    true,
			wantErrHas: "invalid produces",
		},
		{
			name:       "CRLF in produces rejected",
			transforms: "\ntransforms:\n  pdf:\n    command: [\"pandoc\"]\n    produces: \"application/pdf\\r\\nX-Evil: 1\"\n",
			wantErr:    true,
			wantErrHas: "invalid produces",
		},
		{
			name: "unsupported from rejected",
			transforms: `
transforms:
  weird:
    from: html
    command: ["x"]
    produces: text/html
`,
			wantErr:    true,
			wantErrHas: "unsupported from",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(transformsBaseEntities + tc.transforms))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrHas != "" && !strings.Contains(err.Error(), tc.wantErrHas) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErrHas)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParse_Transforms_Parsed(t *testing.T) {
	m, err := Parse([]byte(transformsBaseEntities + `
transforms:
  pdf:
    from: markdown
    command: ["pandoc", "-o", "{out}", "{in}"]
    produces: application/pdf
`))
	if err != nil {
		t.Fatal(err)
	}
	def, ok := m.Transforms["pdf"]
	if !ok {
		t.Fatal("pdf transform not parsed")
	}
	if def.Produces != "application/pdf" {
		t.Errorf("produces = %q", def.Produces)
	}
	if len(def.Command) != 4 || def.Command[0] != "pandoc" {
		t.Errorf("command = %v", def.Command)
	}
}
