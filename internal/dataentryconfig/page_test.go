package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidatePages(t *testing.T) {
	meta := testMetamodel()
	lists := map[string]List{"tickets": {EntityType: "ticket"}}
	pages := func(mut func(*Page)) map[string]Page {
		p := Page{Label: "Tickets", Tabs: []PageTab{
			{ID: "table", Label: "Table", List: "tickets"},
			{ID: "home", Label: "Home", Dashboard: true},
		}}
		if mut != nil {
			mut(&p)
		}
		return map[string]Page{"tickets": p}
	}
	tab := func(mut func(*PageTab)) func(*Page) {
		return func(p *Page) { mut(&p.Tabs[0]) }
	}

	cases := []struct {
		name    string
		pages   map[string]Page
		nav     []NavigationEntry
		spaces  []Space
		wantErr string // substring; "" means expect success
	}{
		{"valid", pages(func(p *Page) {
			p.Icon = "folder"
			p.Tabs[0].Icon = "list"
			p.Tabs[0].Permission = "tickets:read"
		}), []NavigationEntry{{Page: "tickets", Icon: "folder", Permission: "p"}}, nil, ""},
		{"nav entry without a label", pages(nil), []NavigationEntry{{Page: "tickets"}}, nil, ""},
		{"invalid page id", map[string]Page{"Tickets": pages(nil)["tickets"]}, nil, nil,
			`pages[Tickets]: invalid id "Tickets"`},
		{"missing label", pages(func(p *Page) { p.Label = " " }), nil, nil, "pages[tickets]: label is required"},
		{"unknown icon", pages(func(p *Page) { p.Icon = "nope" }), nil, nil, `pages[tickets]: unknown icon "nope"`},
		{"no tabs", pages(func(p *Page) { p.Tabs = nil }), nil, nil, "pages[tickets]: tabs is required"},
		{"tab without id", pages(tab(func(t *PageTab) { t.ID = "" })), nil, nil,
			"pages[tickets].tabs[0]: id is required"},
		{"tab id with a slash", pages(tab(func(t *PageTab) { t.ID = "a/b" })), nil, nil,
			`pages[tickets].tabs[0]: invalid id "a/b"`},
		{"duplicate tab id", pages(func(p *Page) { p.Tabs[1].ID = "table" }), nil, nil,
			"pages[tickets].tabs[table]: duplicate id"},
		{"tab without label", pages(tab(func(t *PageTab) { t.Label = "" })), nil, nil,
			"pages[tickets].tabs[table]: label is required"},
		{"tab with unknown icon", pages(tab(func(t *PageTab) { t.Icon = "nope" })), nil, nil,
			`pages[tickets].tabs[table]: unknown icon "nope"`},
		{"tab names no view", pages(tab(func(t *PageTab) { t.List = "" })), nil, nil,
			"pages[tickets].tabs[table]: names no view"},
		{"tab names two views", pages(tab(func(t *PageTab) { t.Dashboard = true })), nil, nil,
			"pages[tickets].tabs[table]: names more than one view"},
		{"unknown list", pages(tab(func(t *PageTab) { t.List = "nope" })), nil, nil,
			`pages[tickets].tabs[table]: references unknown list "nope"`},
		{"unknown kanban", pages(tab(func(t *PageTab) { t.List, t.Kanban = "", "nope" })), nil, nil,
			`pages[tickets].tabs[table]: references unknown kanban "nope"`},
		{"unknown gantt", pages(tab(func(t *PageTab) { t.List, t.Gantt = "", "nope" })), nil, nil,
			`pages[tickets].tabs[table]: references unknown gantt "nope"`},
		{"unknown calendar", pages(tab(func(t *PageTab) { t.List, t.Calendar = "", "nope" })), nil, nil,
			`pages[tickets].tabs[table]: references unknown calendar "nope"`},
		{"unknown document", pages(tab(func(t *PageTab) { t.List, t.Document = "", "nope" })), nil, nil,
			`pages[tickets].tabs[table]: references unknown document "nope"`},
		{"nav entry names an unknown page", pages(nil), []NavigationEntry{{Label: "X", Page: "nope"}}, nil,
			`navigation: references unknown page "nope"`},
		{"nav status counts the first tab", pages(nil), []NavigationEntry{{Page: "tickets",
			Status: []NavStatusRule{{Tone: "new", Label: "{count} new"}}}}, nil, ""},
		{"nav status needs a list as the first tab", pages(func(p *Page) { p.Tabs[0], p.Tabs[1] = p.Tabs[1], p.Tabs[0] }),
			[]NavigationEntry{{Label: "T", Page: "tickets", Status: []NavStatusRule{{Tone: "new", Label: "x"}}}}, nil,
			`navigation "T": status on a page entry needs the page's first tab to be a list`},
		{"flyout on a page entry", pages(nil), []NavigationEntry{{Label: "T", Page: "tickets", Open: NavOpenFlyout}}, nil,
			`navigation "T": open: flyout is only supported on a list entry`},
		{"space home is a page", pages(nil), nil,
			[]Space{{ID: "crm", Label: "CRM", Home: &NavigationEntry{Page: "tickets"}}}, ""},
		{"space home names an unknown page", pages(nil), nil,
			[]Space{{ID: "crm", Label: "CRM", Home: &NavigationEntry{Page: "nope"}}},
			`spaces[crm].home: references unknown page "nope"`},
		{"space navigation names an unknown page", pages(nil), nil,
			[]Space{{ID: "crm", Label: "CRM", Navigation: []NavigationEntry{{Page: "nope"}}}},
			`spaces[crm]: navigation: references unknown page "nope"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Lists: lists, Pages: tc.pages, Navigation: tc.nav, Spaces: tc.spaces}
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

// `pages:` is a known top-level key and a tab keeps its YAML order.
func TestPages_ParseFromYAML(t *testing.T) {
	data := []byte(`version: "1.0"
lists:
  tickets: {entity_type: ticket}
pages:
  tickets:
    label: Tickets
    icon: folder
    tabs:
      - { id: table, list: tickets, label: Table }
      - { id: home, dashboard: true, label: Home, permission: admin:read }
navigation:
  - page: tickets
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(data, &cfg, testMetamodel()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	page := cfg.Pages["tickets"]
	if len(page.Tabs) != 2 || page.Tabs[0].ID != "table" || page.Tabs[1].Permission != "admin:read" {
		t.Fatalf("tabs = %+v", page.Tabs)
	}
	if cfg.Navigation[0].Page != "tickets" {
		t.Errorf("navigation = %+v", cfg.Navigation)
	}
}

// A page entry's status counts over the list of the page's first tab.
func TestNavStatusEntries_PageList(t *testing.T) {
	status := []NavStatusRule{{Tone: "new", Label: "x"}}
	cfg := &Config{
		Pages: map[string]Page{"work": {Label: "Work", Tabs: []PageTab{{ID: "t", Label: "T", List: "tickets"}}}},
		Navigation: []NavigationEntry{
			{Label: "Plain", List: "bugs", Status: status},
			{Page: "work", Status: status},
		},
	}
	entries := NavStatusEntries(cfg)
	if len(entries) != 2 || entries[0].List != "bugs" || entries[1].List != "tickets" {
		t.Fatalf("entries = %+v", entries)
	}
	if got := cfg.NavEntryList(NavigationEntry{Page: "nope"}); got != "" {
		t.Errorf("unknown page list = %q", got)
	}
}

// An entity page: entity_type, badge and a scope on every tab.
func TestValidateEntityPages(t *testing.T) {
	meta := testMetamodel()
	base := func() *Config {
		return &Config{
			Lists:   map[string]List{"tickets": {EntityType: "ticket"}, "cats": {EntityType: "category"}},
			Kanbans: map[string]Kanban{"board": {EntityType: "ticket", ColumnProperty: "status"}},
			Gantts: map[string]Gantt{"plan": {
				Title: "Plan", Hierarchy: []string{"blocks"},
				Sources: map[string]GanttSource{"ticket": {Label: "title"}},
			}},
			Documents: map[string]DocumentConfig{"report": {Script: "report.lua"}},
		}
	}
	page := func(mut func(*Page)) map[string]Page {
		p := Page{Label: "Category", EntityType: "category", Badge: "name", Tabs: []PageTab{
			{ID: "table", Label: "Table", List: "tickets",
				Scope: &PageTabScope{Relation: "belongs-to", Direction: DirectionIncoming}},
			{ID: "board", Label: "Board", Kanban: "board", Scope: &PageTabScope{Relation: "belongs-to"}},
		}}
		if mut != nil {
			mut(&p)
		}
		return map[string]Page{"cat": p}
	}
	ganttPage := func(scope *PageTabScope) map[string]Page {
		return map[string]Page{"tk": {Label: "Ticket", EntityType: "ticket", Tabs: []PageTab{
			{ID: "plan", Label: "Plan", Gantt: "plan", Scope: scope},
		}}}
	}

	cases := []struct {
		name    string
		pages   map[string]Page
		mut     func(*Config)
		wantErr string // substring; "" means expect success
	}{
		{"valid", page(nil), nil, ""},
		{"gantt rooted at the anchor", ganttPage(&PageTabScope{Root: true}), nil, ""},
		{"unknown entity type", page(func(p *Page) { p.EntityType = "nope" }), nil,
			`pages[cat]: unknown entity_type "nope"`},
		{"badge not a property", page(func(p *Page) { p.Badge = "nope" }), nil,
			`pages[cat]: badge property "nope" not in metamodel`},
		{"badge without entity type", page(func(p *Page) { p.EntityType = ""; p.Tabs = p.Tabs[:0] }), nil,
			"pages[cat]: badge needs entity_type"},
		{"scope on a plain page", map[string]Page{"t": {Label: "T", Tabs: []PageTab{
			{ID: "a", Label: "A", List: "tickets", Scope: &PageTabScope{Relation: "belongs-to"}},
		}}}, nil, "pages[t].tabs[a]: scope needs entity_type"},
		{"tab without scope", page(func(p *Page) { p.Tabs[0].Scope = nil }), nil,
			"pages[cat].tabs[table]: scope is required on an entity page"},
		{"root on a list", page(func(p *Page) { p.Tabs[0].Scope = &PageTabScope{Root: true} }), nil,
			"pages[cat].tabs[table]: scope: root is only for a gantt tab"},
		{"unknown relation", page(func(p *Page) { p.Tabs[0].Scope.Relation = "nope" }), nil,
			`pages[cat].tabs[table]: scope.relation "nope" not in metamodel`},
		{"wrong direction", page(func(p *Page) { p.Tabs[0].Scope.Direction = DirectionOutgoing }), nil,
			`pages[cat].tabs[table]: scope.relation "belongs-to" does not connect "category" to "ticket" outgoing`},
		{"relation that misses the row type", page(func(p *Page) { p.Tabs[0].List = "cats" }), nil,
			`does not connect "category" to "category" incoming`},
		{"ambiguous direction", map[string]Page{"tk": {Label: "T", EntityType: "ticket", Tabs: []PageTab{
			{ID: "a", Label: "A", List: "tickets", Scope: &PageTabScope{Relation: "blocks"}},
		}}}, nil, "pages[tk].tabs[a]: scope needs an explicit `direction:`"},
		{"dashboard tab", page(func(p *Page) {
			p.Tabs = append(p.Tabs, PageTab{ID: "home", Label: "Home", Dashboard: true})
		}), nil, "pages[cat].tabs[home]: an entity page can only show list, kanban and gantt tabs"},
		{"document tab", page(func(p *Page) {
			p.Tabs = append(p.Tabs, PageTab{ID: "doc", Label: "Doc", Document: "report"})
		}), nil, "pages[cat].tabs[doc]: an entity page can only show list, kanban and gantt tabs"},
		{"gantt without root", ganttPage(&PageTabScope{Relation: "blocks"}), nil,
			"pages[tk].tabs[plan]: a gantt tab on an entity page needs scope: root"},
		{"gantt without a source for the anchor", ganttPage(&PageTabScope{Root: true}), func(c *Config) {
			g := c.Gantts["plan"]
			g.Sources = map[string]GanttSource{"category": {Label: "name"}}
			c.Gantts["plan"] = g
		}, `gantt "plan" has no source for "ticket"`},
		{"gantt hierarchy never leaves the anchor", ganttPage(&PageTabScope{Root: true}), func(c *Config) {
			g := c.Gantts["plan"]
			g.Hierarchy = []string{"belongs-to"}
			g.Sources = map[string]GanttSource{"ticket": {Label: "title"}, "category": {Label: "name"}}
			c.Gantts["plan"] = g
			c.Pages["tk"] = Page{Label: "C", EntityType: "category", Tabs: []PageTab{
				{ID: "plan", Label: "Plan", Gantt: "plan", Scope: &PageTabScope{Root: true}},
			}}
		}, `no hierarchy relation of gantt "plan" runs from "category"`},
		{"nav entry names an entity page", page(nil), func(c *Config) {
			c.Navigation = []NavigationEntry{{Label: "C", Page: "cat"}}
		}, `navigation: page "cat" has entity_type "category" so it cannot be a navigation destination`},
		{"space home names an entity page", page(nil), func(c *Config) {
			c.Spaces = []Space{{ID: "crm", Label: "CRM", Home: &NavigationEntry{Page: "cat"}}}
		}, `spaces[crm].home: page "cat" has entity_type "category"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			cfg.Pages = tc.pages
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

// `scope:` takes `root` or a relation mapping; anything else is a load error.
func TestPageTabScope_UnmarshalYAML(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		want    PageTabScope
		wantErr string
	}{
		{"root", `root`, PageTabScope{Root: true}, ""},
		{"relation", `{relation: bestaat_uit, direction: outgoing}`,
			PageTabScope{Relation: "bestaat_uit", Direction: DirectionOutgoing}, ""},
		{"relation without direction", `{relation: bestaat_uit}`, PageTabScope{Relation: "bestaat_uit"}, ""},
		{"other scalar", `all`, PageTabScope{}, `invalid scope at line 1: "all"`},
		{"unknown key", `{relation: x, dir: outgoing}`, PageTabScope{}, `unknown key "dir"`},
		{"bad direction", `{relation: x, direction: up}`, PageTabScope{}, `invalid direction "up"`},
		{"sequence", `[root]`, PageTabScope{}, "must be \"root\" or a mapping"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got PageTabScope
			err := yaml.Unmarshal([]byte(tc.yaml), &got)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
