package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestValidateConfig_RollupMisuse pins that a list may show a roll-up
// (TKT-6I2OPG) as a column but not sort, filter or group on it.
func TestValidateConfig_RollupMisuse(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "RIS-"
    properties:
      title: {type: string}
      plans:
        type: integer
        rollup: {via: treatedBy, count: true}
  plan:
    label: Plan
    id_prefix: "PLN-"
    properties:
      title: {type: string}
relations:
  treats:
    from: [plan]
    to: [risk]
    inverse: treatedBy
`))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		mut  func(*List)
		want string
	}{
		{"column", func(*List) {}, ""},
		{"sortable column", func(l *List) { l.Columns[1].Sortable = true }, "column[1] (sortable)"},
		{"sort", func(l *List) { l.Sort = []SortSpec{{Property: "plans"}} }, "sort[0]"},
		{"filter", func(l *List) {
			l.Filters = []FilterConfig{{Property: "plans", Operator: "eq", Value: "1"}}
		}, "filters[0]"},
		{"group_by", func(l *List) { l.GroupBy = &ListGroupBy{Property: "plans"} }, "group_by"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := List{EntityType: "risk", Columns: []ListColumn{{Property: "title"}, {Property: "plans"}}}
			tc.mut(&list)
			cfg := &Config{App: AppConfig{Name: "t"}, Lists: map[string]List{"risks": list}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("ValidateConfig: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) ||
				!strings.Contains(err.Error(), `roll-up property "plans"`) {

				t.Fatalf("ValidateConfig error = %v, want %q on the roll-up", err, tc.want)
			}
		})
	}
}
