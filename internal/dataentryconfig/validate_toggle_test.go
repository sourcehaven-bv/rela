package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func toggleMetamodel() *metamodel.Metamodel {
	m := testMetamodel()
	def := m.Entities["ticket"]
	def.Properties["done"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}
	def.Properties["flag"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum, Values: []string{"yes", "no"}}
	m.Entities["ticket"] = def
	return m
}

// TestValidateConfig_ToggleColumn pins the load rules for widget: toggle.
func TestValidateConfig_ToggleColumn(t *testing.T) {
	tests := []struct {
		name string
		col  ListColumn
		want string // empty: loads
	}{
		{"bool", ListColumn{Property: "done", Widget: "toggle", On: "true", Off: "false"}, ""},
		{"enum custom type with other", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open",
			Other: "disabled"}, ""},
		{"two-value enum", ListColumn{Property: "flag", Widget: "toggle", On: "yes", Off: "no"}, ""},
		{"unknown widget", ListColumn{Property: "done", Widget: "slider"}, `unknown widget "slider"`},
		{"on without widget", ListColumn{Property: "done", On: "true"}, "on needs widget: toggle"},
		{"relation column", ListColumn{Relation: "blocks", Widget: "toggle", On: "a", Off: "b"}, "takes no relation"},
		{"string property", ListColumn{Property: "assignee", Widget: "toggle", On: "a", Off: "b"},
			"is not a boolean or enum"},
		{"missing off", ListColumn{Property: "done", Widget: "toggle", On: "true"}, "needs off"},
		{"equal", ListColumn{Property: "flag", Widget: "toggle", On: "yes", Off: "yes"}, "on and off are both"},
		{"bool bad value", ListColumn{Property: "done", Widget: "toggle", On: "yes", Off: "false"},
			"on of a boolean toggle must be true or false"},
		{"bool other", ListColumn{Property: "done", Widget: "toggle", On: "true", Off: "false", Other: "on"},
			"other is not needed on a boolean"},
		{"undeclared value", ListColumn{Property: "status", Widget: "toggle", On: "done", Off: "open",
			Other: "on"}, `on "done" is not a value`},
		{"other missing", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open"},
			"other is required"},
		{"other not needed", ListColumn{Property: "flag", Widget: "toggle", On: "yes", Off: "no", Other: "on"},
			"only two values"},
		{"unknown other", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open",
			Other: "maybe"}, `unknown other "maybe"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lists := map[string]List{"all": {EntityType: "ticket", Columns: []ListColumn{{Property: "title"}, tc.col}}}
			err := ValidateConfig([]byte(`version: "1.0"`), &Config{Lists: lists}, toggleMetamodel())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("must load: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

// TestValidateConfig_ToggleRefusedOffLists pins that dashboard card and view
// section columns refuse the toggle keys.
func TestValidateConfig_ToggleRefusedOffLists(t *testing.T) {
	toggle := ListColumn{Property: "done", Widget: "toggle", On: "true", Off: "false"}
	lists := map[string]List{"all": {EntityType: "ticket", Columns: []ListColumn{{Property: "title"}}}}
	card := DashboardCard{Title: "T", List: "all", Display: "table", Columns: []ListColumn{{Property: "title"}, toggle}}
	tableView := map[string]ViewConfig{"v": {
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions"}},
		Sections: []ViewSection{{Source: "mentions", Display: "table", Columns: []ListColumn{{Property: "title"}, toggle}}},
	}}
	for name, cfg := range map[string]*Config{
		"dashboard": {Lists: lists, Dashboard: &DashboardConfig{Cards: []DashboardCard{card}}},
		"section":   {Views: tableView},
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, toggleMetamodel())
			if err == nil || !strings.Contains(err.Error(), "only supported on list columns") {
				t.Fatalf("want refusal, got %v", err)
			}
		})
	}
}
