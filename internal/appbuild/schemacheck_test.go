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

// rollupBase adds a roll-up (TKT-6I2OPG) to checkBase: a task counts the
// tasks below it.
var rollupBase = strings.Replace(checkBase, "      state:\n        type: status\n",
	"      state:\n        type: status\n      subtasks:\n        type: integer\n"+
		"        rollup: {via: parentOf, count: true}\n", 1) + `relations:
  child-of:
    from: [task]
    to: [task]
    inverse: parentOf
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
		// Roll-ups are read surfaces only (TKT-6I2OPG): a write-time
		// expression that names one fails the load.
		{"automation reads a roll-up", rollupBase + `automations:
  - name: a
    on: {entity: task, condition: "entity.subtasks > 0"}
    do: [{set: state, value: done}]
`, "automations"},
		{"validation reads a roll-up", rollupBase + `validations:
  - name: v
    entity_type: task
    then_condition: "entity.subtasks == 0"
`, `validation "v": condition`},
		{"roll-up over an unknown relation", strings.Replace(rollupBase, "via: parentOf", "via: nope", 1),
			"roll-up properties"},
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
