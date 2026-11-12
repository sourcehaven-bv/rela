package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestValidateConfig_RollupMisuse pins that a list may show, sort, filter
// and group on a roll-up (TKT-6I2OPG), and a kanban may not filter on one.
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
	listCases := []struct {
		name string
		mut  func(*List)
	}{
		{"column", func(*List) {}},
		{"sortable column", func(l *List) { l.Columns[1].Sortable = true }},
		{"sort", func(l *List) { l.Sort = []SortSpec{{Property: "plans"}} }},
		{"filter", func(l *List) {
			l.Filters = []FilterConfig{{Property: "plans", Operator: "=", Value: "1"}}
		}},
		{"group_by", func(l *List) { l.GroupBy = &ListGroupBy{Property: "plans"} }},
	}
	for _, tc := range listCases {
		t.Run("list "+tc.name, func(t *testing.T) {
			list := List{EntityType: "risk", Columns: []ListColumn{{Property: "title"}, {Property: "plans"}}}
			tc.mut(&list)
			cfg := &Config{App: AppConfig{Name: "t"}, Lists: map[string]List{"risks": list}}
			if err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta); err != nil {
				t.Fatalf("ValidateConfig: %v", err)
			}
		})
	}
	t.Run("kanban filter", func(t *testing.T) {
		cfg := &Config{App: AppConfig{Name: "t"}, Kanbans: map[string]Kanban{"board": {
			EntityType: "risk", ColumnProperty: "title",
			Filters: []FilterConfig{{Property: "plans", Operator: "=", Value: "1"}},
		}}}
		err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
		if err == nil || !strings.Contains(err.Error(), `filters[0] uses roll-up property "plans"`) {
			t.Fatalf("ValidateConfig error = %v, want the kanban filter refused", err)
		}
	})
}
