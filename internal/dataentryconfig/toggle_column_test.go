package dataentryconfig

import (
	"maps"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// toggleMetamodel is testMetamodel with a boolean and a two-value enum on
// ticket, so every toggle shape has a property to point at.
func toggleMetamodel() *metamodel.Metamodel {
	meta := testMetamodel()
	def := meta.Entities["ticket"]
	props := make(map[string]metamodel.PropertyDef, len(def.Properties)+3)
	maps.Copy(props, def.Properties)
	props["flagged"] = metamodel.PropertyDef{Type: "boolean"}
	props["done"] = metamodel.PropertyDef{Type: "enum", Values: []string{"yes", "no"}}
	props["auto"] = metamodel.PropertyDef{Type: "boolean", Computed: "true"}
	def.Properties = props
	meta.Entities["ticket"] = def
	return meta
}

// TestValidateConfig_ToggleColumn pins the load check of a list toggle
// column (TKT-M0AT73B).
func TestValidateConfig_ToggleColumn(t *testing.T) {
	title := ListColumn{Property: "title"}
	tests := []struct {
		name string
		col  ListColumn
		want string // "" means the config loads
	}{
		{"boolean", ListColumn{Property: "flagged", Widget: "toggle"}, ""},
		{"boolean switch", ListColumn{Property: "flagged", Widget: "switch"}, ""},
		{"two-value enum", ListColumn{Property: "done", Widget: "toggle", On: "yes", Off: "no"}, ""},
		{"wider enum with other", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open",
			Other: "disabled"}, ""},
		{"other hidden", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open", Other: "hidden"}, ""},
		{"other on", ListColumn{Property: "status", Widget: "switch", On: "closed", Off: "open", Other: "on"}, ""},
		{"wider enum without other", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open"},
			"set other: disabled, hidden or on"},
		{"other on a two-value enum", ListColumn{Property: "done", Widget: "toggle", On: "yes", Off: "no",
			Other: "hidden"}, "other does nothing"},
		{"unknown widget", ListColumn{Property: "flagged", Widget: "slider"}, `invalid widget "slider"`},
		{"on without widget", ListColumn{Property: "done", On: "yes"}, "need widget: toggle"},
		{"bad other", ListColumn{Property: "status", Widget: "toggle", On: "closed", Off: "open", Other: "off"},
			`invalid other "off"`},
		{"on off on a boolean", ListColumn{Property: "flagged", Widget: "toggle", On: "x"}, "takes no on, off or other"},
		{"enum without off", ListColumn{Property: "done", Widget: "toggle", On: "yes"}, "needs on and off"},
		{"value not declared", ListColumn{Property: "done", Widget: "toggle", On: "yes", Off: "maybe"},
			`"maybe" is not a value of property "done"`},
		{"on equals off", ListColumn{Property: "done", Widget: "toggle", On: "yes", Off: "yes"}, "on and off must differ"},
		{"string property", ListColumn{Property: "assignee", Widget: "toggle"}, "needs a boolean or enum property"},
		{"list property", ListColumn{Property: "tags", Widget: "toggle"}, "which is a list or computed"},
		{"computed property", ListColumn{Property: "auto", Widget: "toggle"}, "which is a list or computed"},
		{"with link", ListColumn{Property: "flagged", Widget: "toggle", Link: "entity"}, "without relation, face, busy or link"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Lists: map[string]List{"all": {EntityType: "ticket", Columns: []ListColumn{title, tc.col}}}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, toggleMetamodel())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("want load, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

// TestValidateConfig_ToggleColumnPlacement pins that a toggle is refused
// as the title column and outside a list.
func TestValidateConfig_ToggleColumnPlacement(t *testing.T) {
	toggle := ListColumn{Property: "flagged", Widget: "toggle"}
	lists := map[string]List{"all": {EntityType: "ticket", Columns: []ListColumn{{Property: "title"}}}}
	card := DashboardCard{Title: "T", List: "all", Display: "table", Columns: []ListColumn{{Property: "title"}, toggle}}
	tableView := map[string]ViewConfig{"v": {
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions"}},
		Sections: []ViewSection{{Source: "mentions", Display: "table", Columns: []ListColumn{{Property: "title"}, toggle}}},
	}}
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"title column", &Config{Lists: map[string]List{"all": {EntityType: "ticket", Columns: []ListColumn{toggle}}}},
			"the first column is the row title"},
		{"dashboard card", &Config{Lists: lists, Dashboard: &DashboardConfig{Cards: []DashboardCard{card}}},
			"widget, on, off and other are only supported on list columns"},
		{"table section", &Config{Views: tableView},
			"widget, on, off and other are only supported on list columns"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig([]byte(`version: "1.0"`), tc.cfg, toggleMetamodel())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}
