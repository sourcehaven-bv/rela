package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func placementConfig() *Config {
	c := validNextActionConfig()
	c.Lists = map[string]List{"open": {EntityType: "quip"}}
	c.Kanbans = map[string]Kanban{"board": {}}
	return c
}

func TestNextActionsEnabled(t *testing.T) {
	on, off := true, false
	tests := []struct {
		name    string
		mutate  func(*Config)
		surface NextActionSurface
		view    string
		want    bool
	}{
		{"every data surface defaults on", nil, SurfaceList, "open", true},
		{"other defaults off", nil, SurfaceOther, "", false},
		{"surface switch off", func(c *Config) {
			c.NextActionSurfaces = map[NextActionSurface]bool{SurfaceList: false}
		}, SurfaceList, "open", false},
		{"surface switch leaves other kinds alone", func(c *Config) {
			c.NextActionSurfaces = map[NextActionSurface]bool{SurfaceList: false}
		}, SurfaceKanban, "board", true},
		{"view on beats surface off", func(c *Config) {
			c.NextActionSurfaces = map[NextActionSurface]bool{SurfaceList: false}
			l := c.Lists["open"]
			l.Suggestions = &on
			c.Lists["open"] = l
		}, SurfaceList, "open", true},
		{"view off beats the default", func(c *Config) {
			k := c.Kanbans["board"]
			k.Suggestions = &off
			c.Kanbans["board"] = k
		}, SurfaceKanban, "board", false},
		{"dashboard setting", func(c *Config) {
			c.Dashboard = &DashboardConfig{Suggestions: &off}
		}, SurfaceDashboard, "", false},
		{"named dashboard has its own setting", func(c *Config) {
			c.Dashboards = map[string]DashboardConfig{"ops": {Suggestions: &off}}
		}, SurfaceDashboard, "ops", false},
		{"named dashboard ignores the default dashboard's setting", func(c *Config) {
			c.Dashboard = &DashboardConfig{Suggestions: &off}
			c.Dashboards = map[string]DashboardConfig{"ops": {}}
		}, SurfaceDashboard, "ops", true},
		{"entity page follows its type's view", func(c *Config) {
			c.Views = map[string]ViewConfig{"quip-view": {Entry: ViewEntry{Type: "quip"}, Suggestions: &off}}
		}, SurfaceEntity, "quip", false},
		{"entity page without a view uses the surface", nil, SurfaceEntity, "quip", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := placementConfig()
			if tc.mutate != nil {
				tc.mutate(c)
			}
			if got := c.NextActionsEnabled(tc.surface, tc.view); got != tc.want {
				t.Fatalf("NextActionsEnabled(%s, %q) = %v, want %v", tc.surface, tc.view, got, tc.want)
			}
		})
	}
}

func TestNextActionSource_VisibleOnSurface(t *testing.T) {
	src := NextActionSource{VisibleOn: []string{"dashboard", "list:open", "entity:quip"}}
	tests := []struct {
		surface NextActionSurface
		view    string
		want    bool
	}{
		{SurfaceDashboard, "", true},
		{SurfaceList, "open", true},
		{SurfaceList, "closed", false},
		{SurfaceEntity, "quip", true},
		{SurfaceEntity, "task", false},
		{SurfaceSearch, "", false},
	}
	for _, tc := range tests {
		if got := src.VisibleOnSurface(tc.surface, tc.view); got != tc.want {
			t.Errorf("VisibleOnSurface(%s, %q) = %v, want %v", tc.surface, tc.view, got, tc.want)
		}
	}
	if !(NextActionSource{}).VisibleOnSurface(SurfaceForm, "x") {
		t.Error("an absent visible_on must match every page")
	}
}

func TestValidateNextActionPlacement(t *testing.T) {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{"quip": {}}}
	setVisibleOn := func(v []string) func(*Config) {
		return func(c *Config) { s := c.NextActions["quip"]; s.VisibleOn = v; c.NextActions["quip"] = s }
	}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantSub string
	}{
		{"surface kind and scoped views are accepted", setVisibleOn([]string{"list", "list:open", "entity:quip", "kanban:board"}), ""},
		{"unknown surface key", func(c *Config) {
			c.NextActionSurfaces = map[NextActionSurface]bool{"sidebar": false}
		}, `next_action_surfaces: unknown surface "sidebar"`},
		{"empty visible_on", setVisibleOn([]string{}), "visible_on is empty"},
		{"unknown surface in visible_on", setVisibleOn([]string{"lists"}), `visible_on[0]: unknown surface "lists"`},
		{"unknown list", setVisibleOn([]string{"list:gone"}), `"gone" names no list view`},
		{"empty view after colon", setVisibleOn([]string{"list:"}), `"" names no list view`},
		{"unknown entity type", setVisibleOn([]string{"entity:task"}), `"task" names no entity type`},
		{"a view on a surface without views", setVisibleOn([]string{"dashboard:x"}), `"x" names no dashboard view`},
		{"a context source placed only where no page is about its type", func(c *Config) {
			s := c.NextActions["quip"]
			s.Context, s.Query, s.VisibleOn = "quip", "", []string{"list", "kanban:board"}
			c.NextActions["quip"] = s
		}, "names no page about a quip"},
		{"a context source on its entity page", func(c *Config) {
			s := c.NextActions["quip"]
			s.Context, s.Query, s.VisibleOn = "quip", "", []string{"list", "entity:quip"}
			c.NextActions["quip"] = s
		}, ""},
		{"a context source on a document about its type", func(c *Config) {
			c.Documents = map[string]DocumentConfig{"brief": {EntityType: "quip"}}
			s := c.NextActions["quip"]
			s.Context, s.Query, s.VisibleOn = "quip", "", []string{"document"}
			c.NextActions["quip"] = s
		}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := placementConfig()
			tc.mutate(c)
			errs := validateNextActions(c, meta)
			if tc.wantSub == "" {
				if len(errs) > 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				return
			}
			if !strings.Contains(strings.Join(errs, "\n"), tc.wantSub) {
				t.Fatalf("errors %v do not mention %q", errs, tc.wantSub)
			}
		})
	}
}

// suggestions is a bool. A number or a string must fail the load rather than
// read as "unset", which would silently keep suggestions on.
func TestSuggestions_RejectsNonBool(t *testing.T) {
	var l List
	if err := yaml.Unmarshal([]byte("entity_type: quip\nsuggestions: 3\n"), &l); err == nil {
		t.Fatal("suggestions: 3 must not parse")
	}
	if err := yaml.Unmarshal([]byte("entity_type: quip\nsuggestions: false\n"), &l); err != nil {
		t.Fatal(err)
	}
	if l.Suggestions == nil || *l.Suggestions {
		t.Fatalf("suggestions: false parsed as %v", l.Suggestions)
	}
}
