package appbuild

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const checkBase = `version: "1.0"
types:
  status:
    values: [open, done]
entities:
  task:
    label: Task
    id_prefix: TSK
    properties:
      title:
        type: string
      state:
        type: status
`

// TestCheckSchemaCompiles pins that a malformed expression in each place an
// operator writes one is reported, and that a valid one is not.
func TestCheckSchemaCompiles(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string // substring of the single problem; empty means none
	}{
		{"valid", checkBase + `automations:
  - name: a
    on: {entity: task, condition: "entity.state == 'open'"}
    do: [{set: state, value: done}]
validations:
  - name: v
    entity_type: task
    when_condition: "entity.state == 'done'"
    then_condition: "entity.title ~= nil"
`, ""},
		{"automation condition", checkBase + `automations:
  - name: a
    on: {entity: task, condition: "entity.state =="}
    do: [{set: state, value: done}]
`, "automations"},
		{"validation when_condition", checkBase + `validations:
  - name: v
    entity_type: task
    when_condition: "entity.state =="
`, `validation "v": condition`},
		{"validation unknown property", checkBase + `validations:
  - name: v
    entity_type: task
    then_condition: "entity.nope == 1"
`, `validation "v": condition`},
		{"query scope", strings.Replace(checkBase, "      state:\n        type: status\n",
			"      state:\n        type: status\n    query_scopes:\n      open: \"entity.state ==\"\n", 1),
			"query scopes"},
		{"unscoped validation", checkBase + `validations:
  - name: v
    then_condition: "("
`, `on "task"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := metamodel.Parse([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := CheckSchemaCompiles(meta, memstore.New())
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected problems: %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Fatalf("problems = %v, want one containing %q", got, tc.want)
			}
		})
	}
}
