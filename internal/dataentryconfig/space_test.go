package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateSpaces(t *testing.T) {
	meta := testMetamodel()
	lists := map[string]List{"tickets": {EntityType: "ticket"}}
	space := func(mut func(*Space)) []Space {
		sp := Space{ID: "crm", Label: "CRM", Navigation: []NavigationEntry{{Label: "Tickets", List: "tickets"}}}
		if mut != nil {
			mut(&sp)
		}
		return []Space{sp}
	}

	cases := []struct {
		name    string
		spaces  []Space
		nav     []NavigationEntry
		wantErr string // substring; "" means expect success
	}{
		{"valid", space(func(sp *Space) {
			sp.Icon = "folder"
			sp.Permission = "crm:use"
			sp.Home = &NavigationEntry{List: "tickets"}
			sp.Create = []string{"ticket", "category"}
		}), nil, ""},
		{"home is a dashboard", space(func(sp *Space) { sp.Home = &NavigationEntry{Dashboard: true} }), nil, ""},
		{"missing id", space(func(sp *Space) { sp.ID = "" }), nil, "spaces[0]: id is required"},
		{"id with uppercase", space(func(sp *Space) { sp.ID = "Crm" }), nil, `spaces[0]: invalid id "Crm"`},
		{"id starting with a digit", space(func(sp *Space) { sp.ID = "1crm" }), nil, `spaces[0]: invalid id "1crm"`},
		{"id with a colon", space(func(sp *Space) { sp.ID = "a:b" }), nil, `spaces[0]: invalid id "a:b"`},
		{"id too long", space(func(sp *Space) { sp.ID = "a" + strings.Repeat("b", 32) }), nil, "spaces[0]: invalid id"},
		{
			"duplicate id",
			append(space(nil), Space{ID: "crm", Label: "Again"}),
			nil,
			"spaces[crm]: duplicate id",
		},
		{"missing label", space(func(sp *Space) { sp.Label = " " }), nil, "spaces[crm]: label is required"},
		{"unknown icon", space(func(sp *Space) { sp.Icon = "nope" }), nil, `spaces[crm]: unknown icon "nope"`},
		{
			"unknown create type",
			space(func(sp *Space) { sp.Create = []string{"contact"} }),
			nil,
			`spaces[crm].create: unknown entity type "contact"`,
		},
		{
			"create type listed twice",
			space(func(sp *Space) { sp.Create = []string{"ticket", "ticket"} }),
			nil,
			`spaces[crm].create: entity type "ticket" is listed twice`,
		},
		{
			"home is a group",
			space(func(sp *Space) {
				sp.Home = &NavigationEntry{Group: "G", Items: []NavigationEntry{{List: "tickets"}}}
			}),
			nil,
			"spaces[crm].home: must be a destination, not a group",
		},
		{
			"home has status",
			space(func(sp *Space) {
				sp.Home = &NavigationEntry{List: "tickets", Status: []NavStatusRule{{Tone: "new", Label: "x"}}}
			}),
			nil,
			"spaces[crm].home: status is not supported",
		},
		{
			"home has open",
			space(func(sp *Space) { sp.Home = &NavigationEntry{List: "tickets", Open: NavOpenFlyout} }),
			nil,
			"spaces[crm].home: open is not supported",
		},
		{
			"home has permission",
			space(func(sp *Space) { sp.Home = &NavigationEntry{List: "tickets", Permission: "x"} }),
			nil,
			"spaces[crm].home: permission is not supported",
		},
		{
			"home is an action",
			space(func(sp *Space) { sp.Home = &NavigationEntry{Action: "a"} }),
			nil,
			"spaces[crm].home: an action is not a destination",
		},
		{
			"home names nothing",
			space(func(sp *Space) { sp.Home = &NavigationEntry{Label: "Home"} }),
			nil,
			"spaces[crm].home: names no destination",
		},
		{
			"home references an unknown list",
			space(func(sp *Space) { sp.Home = &NavigationEntry{List: "nope"} }),
			nil,
			`spaces[crm].home: references unknown list "nope"`,
		},
		{
			"spaces and navigation both set",
			space(nil),
			[]NavigationEntry{{Label: "Tickets", List: "tickets"}},
			"navigation and spaces are both set: move the top-level navigation into a space",
		},
		{
			"space nav entry references an unknown list",
			space(func(sp *Space) { sp.Navigation = []NavigationEntry{{Label: "X", List: "nope"}} }),
			nil,
			`spaces[crm]: navigation: references unknown list "nope"`,
		},
		{
			"space nav group has a permission",
			space(func(sp *Space) {
				sp.Navigation = []NavigationEntry{{Group: "G", Permission: "p",
					Items: []NavigationEntry{{Label: "T", List: "tickets"}}}}
			}),
			nil,
			`spaces[crm]: navigation: group "G" cannot have a permission`,
		},
		{
			"space nav flyout on a non-list entry",
			space(func(sp *Space) {
				sp.Navigation = []NavigationEntry{{Label: "S", Search: true, Open: NavOpenFlyout}}
			}),
			nil,
			`spaces[crm]: navigation "S": open: flyout is only supported on a list entry`,
		},
		{
			"space nav status with an unknown tone",
			space(func(sp *Space) {
				sp.Navigation = []NavigationEntry{{Label: "T", List: "tickets",
					Status: []NavStatusRule{{Tone: "danger", Label: "x"}}}}
			}),
			nil,
			`spaces[crm]: navigation "T": status[0]: unknown tone "danger"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Lists:      lists,
				Actions:    map[string]Action{"a": {Label: "A", Set: map[string]string{"status": "open"}}},
				Navigation: tc.nav,
				Spaces:     tc.spaces,
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

// `spaces:` is a known top-level key, and a list in the YAML keeps its order.
func TestSpaces_ParseFromYAML(t *testing.T) {
	data := []byte(`version: "1.0"
lists:
  tickets: {entity_type: ticket}
spaces:
  - id: projects
    label: Projects
    home: {list: tickets}
    create: [ticket]
    navigation:
      - label: Tickets
        list: tickets
  - id: isms
    label: ISMS
    permission: isms:use
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(data, &cfg, testMetamodel()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(cfg.Spaces) != 2 || cfg.Spaces[0].ID != "projects" || cfg.Spaces[1].ID != "isms" {
		t.Fatalf("spaces = %+v", cfg.Spaces)
	}
	if cfg.Spaces[0].Home == nil || cfg.Spaces[0].Home.List != "tickets" {
		t.Errorf("home = %+v", cfg.Spaces[0].Home)
	}
}

// Entries inside a space get keys prefixed with the space id; top-level keys
// keep their shape.
func TestNavStatusEntries_SpaceKeys(t *testing.T) {
	status := []NavStatusRule{{Tone: "new", Label: "x"}}
	cfg := &Config{Spaces: []Space{
		{ID: "crm", Navigation: []NavigationEntry{
			{Label: "Home", Dashboard: true},
			{Label: "Top", List: "a", Status: status},
			{Group: "Work", Items: []NavigationEntry{
				{Label: "Plain", List: "b"},
				{Label: "Mine", List: "c", Status: status},
			}},
		}},
		{ID: "isms", Navigation: []NavigationEntry{{Label: "Risks", List: "d", Status: status}}},
	}}

	var keys []string
	for _, e := range NavStatusEntries(cfg) {
		keys = append(keys, e.Space+"|"+e.Key+"="+e.Entry.Label)
	}
	want := []string{"crm|crm:1=Top", "crm|crm:2.1=Mine", "isms|isms:0=Risks"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if id := NavStatusConditionID("crm:2.1", 0); id != "crm:2.1#0" {
		t.Errorf("condition id = %q", id)
	}
}
