package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateNavItemsFrom(t *testing.T) {
	meta := testMetamodel()
	base := func() *Config {
		return &Config{
			Lists: map[string]List{"tickets": {EntityType: "ticket"}, "cats": {EntityType: "category"}},
			Pages: map[string]Page{
				"cat": {PageHeader: PageHeader{Label: "Category", EntityType: "category"}, Tabs: []PageTab{
					{ID: "table", Label: "Table", List: "tickets",
						Scope: &PageTabScope{Relation: "belongs-to", Direction: DirectionIncoming}},
				}},
				"plain": {PageHeader: PageHeader{Label: "Plain"}, Tabs: []PageTab{{ID: "t", Label: "T", List: "tickets"}}},
			},
		}
	}
	group := func(mut func(*NavItemsFrom)) []NavigationEntry {
		from := NavItemsFrom{List: "cats", Page: "cat", Limit: 10,
			Initial: &NavItemsInitial{Relation: "belongs-to", Direction: DirectionIncoming}}
		if mut != nil {
			mut(&from)
		}
		return []NavigationEntry{{Group: "Categories", ItemsFrom: &from}}
	}

	cases := []struct {
		name    string
		nav     []NavigationEntry
		mut     func(*Config)
		wantErr string // substring; "" means expect success
	}{
		{"valid", group(nil), nil, ""},
		{"no page, no initial, default limit", group(func(f *NavItemsFrom) {
			f.Page, f.Initial, f.Limit = "", nil, 0
		}), nil, ""},
		{"property initial", group(func(f *NavItemsFrom) { f.Initial = &NavItemsInitial{Property: "name"} }), nil, ""},
		{"inferred direction", group(func(f *NavItemsFrom) { f.Initial.Direction = "" }), nil, ""},
		{"children and status", group(func(f *NavItemsFrom) {
			f.Status = &NavItemsStatus{List: "tickets", Via: "belongs-to", Tone: "info", Label: "{count}"}
			f.Children = &NavItemsChildren{List: "tickets", Via: "belongs-to", Limit: 5,
				Status: &NavItemsStatus{List: "tickets", Via: "blocks", Direction: DirectionOutgoing,
					Tone: "info", Label: "{count}"}}
		}), nil, ""},
		{"children unknown list", group(func(f *NavItemsFrom) {
			f.Children = &NavItemsChildren{List: "nope", Via: "belongs-to"}
		}), nil, `items_from.children: references unknown list "nope"`},
		{"children no via", group(func(f *NavItemsFrom) {
			f.Children = &NavItemsChildren{List: "tickets"}
		}), nil, "items_from.children.via is required"},
		{"children via does not join", group(func(f *NavItemsFrom) {
			f.Children = &NavItemsChildren{List: "cats", Via: "belongs-to", Direction: DirectionOutgoing}
		}), nil, `items_from.children: relation "belongs-to" does not connect "category"`},
		{"children limit", group(func(f *NavItemsFrom) {
			f.Children = &NavItemsChildren{List: "tickets", Via: "belongs-to", Limit: 51}
		}), nil, "items_from.children.limit must be between 1 and 50"},
		{"children page for another type", group(func(f *NavItemsFrom) {
			f.Children = &NavItemsChildren{List: "tickets", Via: "belongs-to", Page: "cat"}
		}), nil, `page "cat" shows "category" but the list shows "ticket"`},
		{"status unknown tone", group(func(f *NavItemsFrom) {
			f.Status = &NavItemsStatus{List: "tickets", Via: "belongs-to", Tone: "loud", Label: "x"}
		}), nil, `items_from.status: unknown tone "loud"`},
		{"status no label", group(func(f *NavItemsFrom) {
			f.Status = &NavItemsStatus{List: "tickets", Via: "belongs-to", Tone: "info"}
		}), nil, "items_from.status: label is required"},
		{"status no list", group(func(f *NavItemsFrom) {
			f.Status = &NavItemsStatus{Via: "belongs-to", Tone: "info", Label: "x"}
		}), nil, "items_from.status.list is required"},
		{"status unknown relation", group(func(f *NavItemsFrom) {
			f.Status = &NavItemsStatus{List: "tickets", Via: "nope", Tone: "info", Label: "x"}
		}), nil, `items_from.status: relation "nope" not in metamodel`},
		{"on an item", []NavigationEntry{{Label: "Cats", List: "cats", ItemsFrom: &NavItemsFrom{List: "cats"}}}, nil,
			`navigation "Cats": items_from is only supported on a group`},
		{"on an item inside a group", []NavigationEntry{{Group: "G", Items: []NavigationEntry{
			{Label: "Cats", List: "cats", ItemsFrom: &NavItemsFrom{List: "cats"}},
		}}}, nil, `navigation "Cats": items_from is only supported on a group`},
		{"with static items", []NavigationEntry{{Group: "G", ItemsFrom: &NavItemsFrom{List: "cats"},
			Items: []NavigationEntry{{Label: "T", List: "tickets"}}}}, nil,
			`group "G": items_from and items cannot both be set`},
		{"no list", group(func(f *NavItemsFrom) { f.List = "" }), nil, "items_from.list is required"},
		{"unknown list", group(func(f *NavItemsFrom) { f.List = "nope" }), nil,
			`items_from: references unknown list "nope"`},
		{"limit too high", group(func(f *NavItemsFrom) { f.Limit = 51 }), nil,
			"items_from.limit must be between 1 and 50, got 51"},
		{"negative limit", group(func(f *NavItemsFrom) { f.Limit = -1 }), nil, "items_from.limit must be between"},
		{"unknown page", group(func(f *NavItemsFrom) { f.Page = "nope" }), nil,
			`items_from: references unknown page "nope"`},
		{"page is not an entity page", group(func(f *NavItemsFrom) { f.Page = "plain" }), nil,
			`page "plain" has no entity_type`},
		{"page for another type", group(func(f *NavItemsFrom) { f.List = "tickets"; f.Initial = nil }), nil,
			`page "cat" shows "category" but the list shows "ticket"`},
		{"initial with no source", group(func(f *NavItemsFrom) { f.Initial = &NavItemsInitial{} }), nil,
			"items_from.initial: set property or relation"},
		{"initial with both sources", group(func(f *NavItemsFrom) { f.Initial.Property = "name" }), nil,
			"items_from.initial: set property or relation, not both"},
		{"direction on a property", group(func(f *NavItemsFrom) {
			f.Initial = &NavItemsInitial{Property: "name", Direction: DirectionIncoming}
		}), nil, "direction only applies to a relation"},
		{"unknown property", group(func(f *NavItemsFrom) { f.Initial = &NavItemsInitial{Property: "nope"} }), nil,
			`items_from.initial: property "nope" not in metamodel for entity "category"`},
		{"unknown relation", group(func(f *NavItemsFrom) { f.Initial.Relation = "nope" }), nil,
			`items_from.initial: relation "nope" not in metamodel`},
		{"wrong direction", group(func(f *NavItemsFrom) { f.Initial.Direction = DirectionOutgoing }), nil,
			`relation "belongs-to" does not connect "category" outgoing`},
		{"ambiguous direction", group(func(f *NavItemsFrom) {
			f.List, f.Page = "tickets", ""
			f.Initial = &NavItemsInitial{Relation: "blocks"}
		}), nil, "items_from.initial needs an explicit `direction:`"},
		{"create with a form for the type", group(func(f *NavItemsFrom) { f.Create = true }),
			func(c *Config) { c.Forms = map[string]Form{"new_cat": {EntityType: "category"}} }, ""},
		{"create with no form", group(func(f *NavItemsFrom) { f.Create = true }), nil,
			`items_from.create: list "cats" has no create_form and no form creates "category"`},
		{"group with status", []NavigationEntry{{Group: "G", ItemsFrom: &NavItemsFrom{List: "cats"},
			Status: []NavStatusRule{{Tone: "new", Label: "{count}"}}}}, nil,
			"status is only supported on a list or page entry"},
		{"inside a space", nil, func(c *Config) {
			c.Spaces = []Space{{ID: "crm", Label: "CRM", Navigation: group(func(f *NavItemsFrom) { f.List = "nope" })}}
		}, `spaces[crm]: navigation: group "Categories": items_from: references unknown list "nope"`},
		{"space home", nil, func(c *Config) {
			c.Spaces = []Space{{ID: "crm", Label: "CRM",
				Home: &NavigationEntry{List: "cats", ItemsFrom: &NavItemsFrom{List: "cats"}}}}
		}, "spaces[crm].home: items_from is not supported"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			cfg.Navigation = tc.nav
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

// The keys are positional like status keys, so the sidebar and the items
// endpoint agree on them without either telling the other.
func TestNavItemsFromEntries_Keys(t *testing.T) {
	from := &NavItemsFrom{List: "cats"}
	cfg := &Config{
		Navigation: []NavigationEntry{
			{Label: "Home", Dashboard: DefaultDashboard},
			{Group: "Categories", ItemsFrom: from},
			{Group: "Static", Items: []NavigationEntry{{Label: "T", List: "tickets"}}},
		},
		Spaces: []Space{{ID: "crm", Navigation: []NavigationEntry{{Group: "Mine", ItemsFrom: from}}}},
	}
	got := NavItemsFromEntries(cfg)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}
	if got[0].Key != "1" || got[0].Space != "" || got[0].Entry.Group != "Categories" {
		t.Errorf("top-level entry = %+v", got[0])
	}
	if got[1].Key != "crm:0" || got[1].Space != "crm" || got[1].Entry.Group != "Mine" {
		t.Errorf("space entry = %+v", got[1])
	}
}

func TestNavItemsFrom_ParseFromYAML(t *testing.T) {
	var nav []NavigationEntry
	src := `
- group: Topics
  items_from:
    list: actieve_topics
    page: topic
    limit: 20
    initial: { relation: is_topic_eigenaar_van, direction: incoming }
`
	if err := yaml.Unmarshal([]byte(src), &nav); err != nil {
		t.Fatal(err)
	}
	from := nav[0].ItemsFrom
	if from == nil || from.List != "actieve_topics" || from.Page != "topic" || from.Limit != 20 {
		t.Fatalf("items_from = %+v", from)
	}
	if from.Initial == nil || from.Initial.Relation != "is_topic_eigenaar_van" || !from.Initial.Direction.IsIncoming() {
		t.Fatalf("initial = %+v", from.Initial)
	}
	if got := (NavItemsFrom{}).EffectiveLimit(); got != NavItemsDefaultLimit {
		t.Errorf("EffectiveLimit() of zero = %d, want %d", got, NavItemsDefaultLimit)
	}
}
