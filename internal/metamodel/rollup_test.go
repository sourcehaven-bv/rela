package metamodel_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const rollupShapeSchema = `version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "RIS-"
    properties:
      name: {type: string}
      PROP
  plan:
    label: Plan
    id_prefix: "PLN-"
    properties:
      done: {type: boolean}
relations:
  treats:
    from: [plan]
    to: [risk]
    inverse: treatedBy
`

func TestRollupShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		prop string
		want string // "" means the schema loads
	}{
		{"count loads", `n: {type: integer, rollup: {via: treatedBy, count: true}}`, ""},
		{"exists loads", `any: {type: boolean, rollup: {via: treatedBy, exists: true}}`, ""},
		{"no via", `n: {type: integer, rollup: {count: true}}`, "needs via"},
		{"no function", `n: {type: integer, rollup: {via: treatedBy}}`, "exactly one of"},
		{"two functions", `n: {type: integer, rollup: {via: treatedBy, count: true, max: done}}`, "exactly one of"},
		{"count not integer", `n: {type: string, rollup: {via: treatedBy, count: true}}`, "count needs type integer"},
		{"exists not boolean", `n: {type: integer, rollup: {via: treatedBy, exists: true}}`, "exists needs type boolean"},
		{"latest_by without pick", `n: {type: date, rollup: {via: treatedBy, latest_by: d}}`, "latest_by needs pick"},
		{"required", `n: {type: integer, required: true, rollup: {via: treatedBy, count: true}}`,
			"cannot be combined with required"},
		{"list", `n: {type: integer, list: true, rollup: {via: treatedBy, count: true}}`,
			"cannot be combined with list"},
		{"computed", `n: {type: integer, computed: "1", rollup: {via: treatedBy, count: true}}`,
			"cannot be combined with computed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := metamodel.Parse([]byte(strings.Replace(rollupShapeSchema, "PROP", tc.prop, 1)))
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRollupNames(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(strings.Replace(rollupShapeSchema, "PROP",
		`n: {type: integer, rollup: {via: treatedBy, count: true}}`, 1)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := meta.RollupNames("risk"); len(got) != 1 || got[0] != "n" {
		t.Errorf("RollupNames(risk) = %v, want [n]", got)
	}
	if got := meta.RollupNames("plan"); len(got) != 0 {
		t.Errorf("RollupNames(plan) = %v, want none", got)
	}
	if got := meta.RollupNames("nope"); got != nil {
		t.Errorf("RollupNames(nope) = %v, want nil", got)
	}
}
