package metamodel

import (
	"strings"
	"testing"
)

const enforceBaseYAML = `
version: "1.0"
entities:
  note:
    label: Note
    id_prefix: "N-"
    id_type: sequential
    properties:
      title: {type: string}
relations:
  refines:
    from: [note]
    to: [note]
`

// TestValidationEnforce_Load pins which rules may declare `enforce: write`
// (TKT-5CESLHD): only property conditions, never Lua or graph reads.
func TestValidationEnforce_Load(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want string // "" means the load succeeds
	}{
		{name: "filter conditions", rule: `
    enforce: write
    when: ["title=x"]
    then: ["title!="]`},
		{name: "expression condition with severity error", rule: `
    enforce: write
    severity: error
    then_condition: "entity.title ~= nil"`},
		{name: "not enforced may use lua", rule: `
    lua: "return true"`},
		{name: "unknown enforce value", rule: `
    enforce: always
    then: ["title!="]`, want: "`enforce: always` is not supported"},
		{name: "lua", rule: `
    enforce: write
    then: ["title!="]
    lua: "return true"`, want: "cannot be combined with `lua:`"},
		{name: "lua_file", rule: `
    enforce: write
    then: ["title!="]
    lua_file: check.lua`, want: "cannot be combined with `lua_file:`"},
		{name: "relations", rule: `
    enforce: write
    then: ["title!="]
    relations:
      refines: {min: 1}`, want: "cannot be combined with `relations:`"},
		{name: "content", rule: `
    enforce: write
    then: ["title!="]
    content:
      min_length: 3`, want: "cannot be combined with `content:`"},
		{name: "warning severity", rule: `
    enforce: write
    severity: warning
    then: ["title!="]`, want: "needs `severity: error`"},
		{name: "no then", rule: `
    enforce: write
    when: ["title=x"]`, want: "the rule would check nothing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := enforceBaseYAML + `
validations:
  - name: r
    entity_type: note` + tc.rule + "\n"
			_, err := Parse([]byte(yaml))
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

func TestValidationRule_SeverityDefault(t *testing.T) {
	tests := []struct {
		rule ValidationRule
		want string
	}{
		{ValidationRule{}, "warning"},
		{ValidationRule{Enforce: EnforceWrite}, "error"},
		{ValidationRule{Severity: "warning"}, "warning"},
		{ValidationRule{Severity: "error"}, "error"},
	}
	for _, tc := range tests {
		if got := tc.rule.GetSeverity(); got != tc.want {
			t.Errorf("GetSeverity(%+v) = %q, want %q", tc.rule, got, tc.want)
		}
	}
}
