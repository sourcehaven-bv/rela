package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// An entity dashboard: entity_type under dashboards:, and a relation scope on
// every card from the anchor type to the card list's type.
func TestValidateEntityDashboards(t *testing.T) {
	meta := testMetamodel()
	card := func(mut func(*DashboardCard)) DashboardCard {
		c := DashboardCard{
			Title: "Tickets", List: "tickets", Display: "count",
			Scope: &PageTabScope{Relation: "belongs-to", Direction: DirectionIncoming},
		}
		if mut != nil {
			mut(&c)
		}
		return c
	}
	cases := []struct {
		name    string
		mut     func(*Config)
		wantErr string // substring; "" means expect success
	}{
		{"valid", nil, ""},
		{"direction inferred", func(c *Config) {
			c.Dashboards["cat"].Cards[0].Scope.Direction = ""
		}, ""},
		{"unknown entity type", func(c *Config) {
			d := c.Dashboards["cat"]
			d.EntityType = "nope"
			c.Dashboards["cat"] = d
		}, `dashboards[cat]: unknown entity_type "nope"`},
		{"entity type on the default dashboard", func(c *Config) {
			c.Dashboard = &DashboardConfig{Title: "Home", EntityType: "category"}
		}, "dashboard: entity_type is only allowed on a dashboard under dashboards:"},
		{"card without scope", func(c *Config) {
			c.Dashboards["cat"].Cards[0] = card(func(c *DashboardCard) { c.Scope = nil })
		}, `dashboards[cat]: card[0] "Tickets": scope is required on an entity dashboard`},
		{"card with root scope", func(c *Config) {
			c.Dashboards["cat"].Cards[0] = card(func(c *DashboardCard) { c.Scope = &PageTabScope{Root: true} })
		}, `card[0] "Tickets": scope: root is not allowed on a card`},
		{"card without relation", func(c *Config) {
			c.Dashboards["cat"].Cards[0] = card(func(c *DashboardCard) { c.Scope = &PageTabScope{} })
		}, `card[0] "Tickets": scope.relation is required`},
		{"query card", func(c *Config) {
			c.Dashboards["cat"].Cards[0] = card(func(c *DashboardCard) { c.List = ""; c.Query = "type:ticket" })
		}, `card[0] "Tickets": a card of an entity dashboard needs list:`},
		{"unknown list", func(c *Config) {
			c.Dashboards["cat"].Cards[0] = card(func(c *DashboardCard) { c.List = "nope" })
		}, `card[0] "Tickets"`},
		{"unknown relation", func(c *Config) {
			c.Dashboards["cat"].Cards[0].Scope.Relation = "nope"
		}, `card[0] "Tickets": scope.relation "nope" not in metamodel`},
		{"wrong direction", func(c *Config) {
			c.Dashboards["cat"].Cards[0].Scope.Direction = DirectionOutgoing
		}, `scope.relation "belongs-to" does not connect "category" to "ticket" outgoing`},
		{"scope on a plain dashboard", func(c *Config) {
			c.Dashboards["plain"] = DashboardConfig{Title: "Plain", Cards: []DashboardCard{card(nil)}}
		}, `dashboards[plain]: card[0] "Tickets": scope needs entity_type on the dashboard`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Lists: map[string]List{"tickets": {EntityType: "ticket"}},
				Dashboards: map[string]DashboardConfig{
					"cat": {Title: "Category", EntityType: "category", Cards: []DashboardCard{card(nil)}},
				},
			}
			if tc.mut != nil {
				tc.mut(cfg)
			}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("expected success, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// A card's scope is read from YAML in the relation form a list tab uses.
func TestDashboardCard_ScopeYAML(t *testing.T) {
	data := []byte(`
version: "1.0"
lists:
  tickets: { entity_type: ticket }
dashboards:
  cat:
    title: Category
    entity_type: category
    cards:
      - title: Tickets
        list: tickets
        display: count
        scope: { relation: belongs-to, direction: incoming }
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(data, &cfg, testMetamodel()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	d := cfg.Dashboards["cat"]
	sc := d.Cards[0].Scope
	if !d.IsEntityDashboard() || sc == nil || sc.Relation != "belongs-to" || sc.Direction != DirectionIncoming {
		t.Fatalf("dashboard = %+v", d)
	}
}
