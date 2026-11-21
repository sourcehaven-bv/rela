package dataentryconfig

import (
	"strings"
	"testing"
)

// TestValidateConfig_PrimaryColumnOnlyOnLists pins that `primary` loads on a
// list column and is refused on dashboard card and view section columns,
// which never drop a column to fit.
func TestValidateConfig_PrimaryColumnOnlyOnLists(t *testing.T) {
	yes := true
	primary := ListColumn{Property: "title", Primary: &yes}
	lists := map[string]List{"all": {EntityType: "ticket", Columns: []ListColumn{{Property: "title"}, primary}}}
	if err := ValidateConfig([]byte(`version: "1.0"`), &Config{Lists: lists}, testMetamodel()); err != nil {
		t.Fatalf("primary on a list column must load: %v", err)
	}

	card := DashboardCard{Title: "T", List: "all", Display: "table", Columns: []ListColumn{{Property: "title"}, primary}}
	view := map[string]ViewConfig{"v": {
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions"}},
		Sections: []ViewSection{{Source: "mentions", Display: "table", Columns: []ListColumn{{Property: "title"}, primary}}},
	}}
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"dashboard card", &Config{Lists: lists, Dashboard: &DashboardConfig{Cards: []DashboardCard{card}}},
			`dashboard: card[0] "T" column[1]: primary is only supported on list columns`},
		{"view section", &Config{Views: view},
			`view "v": section[0] column[1]: primary is only supported on list columns`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig([]byte(`version: "1.0"`), tc.cfg, testMetamodel())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}
