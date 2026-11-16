package appbuild

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
		{"valid", checkBase + `validations:
  - name: v
    entity_type: task
    when_condition: "entity.state == 'done'"
    then_condition: "entity.title ~= nil"
`, ""},
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

// checkMeta parses schema for the trigger tests.
func checkMeta(t *testing.T, schema string) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	require.NoError(t, err)
	return meta
}

// TestLoadTriggers pins that flows.yaml triggers load in document order, that
// an absent file or block yields none, and that an invalid rule is an error.
func TestLoadTriggers(t *testing.T) {
	meta := checkMeta(t, checkBase)
	cases := []struct {
		name    string
		files   mapLoader
		want    []string // rule names, in order
		wantErr string   // substring; empty means no error
	}{
		{name: "absent file", files: mapLoader{}},
		{name: "no triggers block", files: mapLoader{"flows.yaml": "flows: {}\n"}},
		{name: "valid rules", files: mapLoader{"flows.yaml": `triggers:
  close:
    on: {entity: task, property: title}
    outputs:
      - set: {properties: {state: done}}
  reopen:
    on: {entity: task, created: true}
    outputs:
      - set: {properties: {state: open}}
`}, want: []string{"close", "reopen"}},
		{name: "invalid rule", files: mapLoader{"flows.yaml": `triggers:
  bad:
    on: {entity: task}
    outputs:
      - set: {properties: {state: done}}
        relate: {type: x, to: y}
`}, wantErr: "flows.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := LoadTriggers(context.Background(), tc.files, meta)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			var names []string
			for _, r := range rules {
				names = append(names, r.Name)
			}
			require.Equal(t, tc.want, names)
		})
	}
}

// TestCheckTriggers pins that a trigger the schema cannot honor is reported
// with the file name first, with and without a store, and that a valid one
// is not.
func TestCheckTriggers(t *testing.T) {
	cases := []struct {
		name   string
		schema string
		flows  string
		bad    bool
	}{
		{name: "no flows.yaml", schema: checkBase},
		{name: "valid", schema: checkBase, flows: `triggers:
  a:
    on: {entity: task, created: true, when: "entity.state == 'open'"}
    outputs: [{set: {properties: {state: done}}}]
`},
		{name: "condition names an unknown property", schema: checkBase, flows: `triggers:
  a:
    on: {entity: task, created: true, when: "entity.nope == 1"}
    outputs: [{set: {properties: {state: done}}}]
`, bad: true},
		{name: "condition does not compile", schema: checkBase, flows: `triggers:
  a:
    on: {entity: task, created: true, when: "entity.state =="}
    outputs: [{set: {properties: {state: done}}}]
`, bad: true},
		// Roll-ups are read surfaces only (TKT-6I2OPG): a write-time
		// expression that names one fails the load.
		{name: "condition reads a roll-up", schema: rollupBase, flows: `triggers:
  a:
    on: {entity: task, created: true, when: "entity.subtasks > 0"}
    outputs: [{set: {properties: {state: done}}}]
`, bad: true},
	}
	stores := []struct {
		name string
		st   store.Store
	}{
		{"nil store", nil},
		{"memstore", memstore.New()},
	}
	for _, tc := range cases {
		for _, sc := range stores {
			t.Run(tc.name+"/"+sc.name, func(t *testing.T) {
				files := mapLoader{}
				if tc.flows != "" {
					files["flows.yaml"] = tc.flows
				}
				err := CheckTriggers(context.Background(), files, checkMeta(t, tc.schema), sc.st)
				if !tc.bad {
					require.NoError(t, err)
					return
				}
				require.Error(t, err)
				require.True(t, strings.HasPrefix(err.Error(), "flows.yaml"), "error = %q", err)
			})
		}
	}
}
