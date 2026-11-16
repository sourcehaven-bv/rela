package appbuild_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const bootTriggerSchema = `version: "1.0"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title: {type: string}
      state: {type: string}
`

// TestDiscover_RefusesBadTriggers pins that a project does not assemble when
// its flows.yaml declares a trigger the schema cannot honor, or when its
// schema.yaml still declares the `automations:` that moved to flows.yaml.
func TestDiscover_RefusesBadTriggers(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		flows   string
		wantErr string
	}{
		{
			name:   "trigger condition does not compile",
			schema: bootTriggerSchema,
			flows: `triggers:
  close:
    on: {entity: note, created: true, when: "entity.state =="}
    outputs:
      - set: {properties: {state: closed}}
`,
			wantErr: "trigger \"close\": `when`",
		},
		{
			name:   "trigger fails the schema checks",
			schema: bootTriggerSchema,
			flows: `triggers:
  close:
    on: {entity: note, updated: true, property: title}
    outputs:
      - set: {properties: {state: closed}}
`,
			wantErr: `flows.yaml: triggers:`,
		},
		{
			name: "automations left in schema.yaml",
			schema: bootTriggerSchema + `automations:
  - name: close
    on: {entity: note, created: true}
    do:
      - set: state
        value: closed
`,
			wantErr: "automations: are declared in flows.yaml under triggers:",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(tc.schema), 0o600))
			if tc.flows != "" {
				require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), []byte(tc.flows), 0o600))
			}
			svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
			if err == nil {
				_ = svc.Close()
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
