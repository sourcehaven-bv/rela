package dataentryconfig

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ganttMetamodel builds a fixture with the shapes a gantt has to handle:
// a self-referential containment relation (project contains project), a mixed
// hierarchy (project has-epic, epic has-ticket), per-type date properties
// under different names, a list-valued date, and a type with no display
// property.
func ganttMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Entities: map[string]metamodel.EntityDef{
			"project": {
				Label:           "Project",
				IDPrefix:        "PRJ-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":         {Type: metamodel.PropertyTypeString, Required: true},
					"planned_start": {Type: metamodel.PropertyTypeDate},
					"planned_end":   {Type: metamodel.PropertyTypeDate},
					"target_date":   {Type: metamodel.PropertyTypeDate},
					"status":        {Type: metamodel.PropertyTypeString},
				},
			},
			"epic": {
				Label:           "Epic",
				IDPrefix:        "EPIC-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":      {Type: metamodel.PropertyTypeString, Required: true},
					"start":      {Type: metamodel.PropertyTypeDate},
					"end":        {Type: metamodel.PropertyTypeDate},
					"milestones": {Type: metamodel.PropertyTypeDate, List: true},
				},
			},
			"anon": {
				Label:    "Anonymous",
				IDPrefix: "ANON-",
				Properties: map[string]metamodel.PropertyDef{
					"when": {Type: metamodel.PropertyTypeDate},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"contains": {Label: "contains", From: []string{"project"}, To: []string{"project"}},
			"has-epic": {Label: "has epic", From: []string{"project"}, To: []string{"epic"}},
		},
	}
}

// ganttCfg builds a Config carrying one gantt under the key "plan".
func ganttCfg(g Gantt) *Config {
	return &Config{Gantts: map[string]Gantt{"plan": g}}
}

// validGantt is a minimal gantt that must pass validation, so each test case
// can introduce exactly one defect.
func validGantt() Gantt {
	return Gantt{
		Title:     "Delivery",
		Hierarchy: []string{"contains", "has-epic"},
		Sources: map[string]GanttSource{
			"project": {Start: "planned_start", End: "planned_end", Committed: "target_date"},
			"epic":    {Start: "start", End: "end"},
		},
	}
}

// withGantt returns a valid gantt mutated by fn.
func withGantt(fn func(*Gantt)) Gantt {
	g := validGantt()
	fn(&g)
	return g
}

func TestValidateGantts_Valid(t *testing.T) {
	meta := ganttMetamodel()
	tests := []struct {
		name string
		g    Gantt
	}{
		{"minimal", validGantt()},
		{"self-referential hierarchy only", Gantt{
			Hierarchy: []string{"contains"},
			Sources:   map[string]GanttSource{"project": {Start: "planned_start", End: "planned_end"}},
		}},
		{"source with no date roles is a pure roll-up mapping", withGantt(func(g *Gantt) {
			g.Sources["project"] = GanttSource{}
		})},
		{"committed alone is a milestone target", withGantt(func(g *Gantt) {
			g.Sources["project"] = GanttSource{Committed: "target_date"}
		})},
		{"explicit policies", withGantt(func(g *Gantt) {
			g.MultiParent = "error"
			g.OnCycle = "prune"
		})},
		{"where clauses parse and resolve", withGantt(func(g *Gantt) {
			g.Sources["project"] = GanttSource{Start: "planned_start", End: "planned_end",
				Where: []string{"status=active"}}
		})},
		{"filter control resolving on one source type", withGantt(func(g *Gantt) {
			g.FilterControls = []FilterControl{{Property: "status"}}
		})},
		{"tooltip property on one source type", withGantt(func(g *Gantt) {
			g.Tooltip = GanttTooltip{Fields: []KanbanCardField{{Property: "status"}}}
		})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errs := validateGantts(ganttCfg(tt.g), meta); len(errs) > 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

func TestValidateGantts_Invalid(t *testing.T) {
	meta := ganttMetamodel()
	tests := []struct {
		name string
		g    Gantt
		want string
	}{
		{
			name: "no sources",
			g:    Gantt{Hierarchy: []string{"contains"}},
			want: "must declare at least one source",
		},
		{
			name: "empty hierarchy",
			g:    withGantt(func(g *Gantt) { g.Hierarchy = nil }),
			want: "'hierarchy' must list at least one relation type",
		},
		{
			name: "unknown relation type in hierarchy",
			g:    withGantt(func(g *Gantt) { g.Hierarchy = []string{"contains", "ghost-rel"} }),
			want: `hierarchy[1] references unknown relation type "ghost-rel"`,
		},
		{
			name: "unknown entity type as source key",
			g: withGantt(func(g *Gantt) {
				g.Sources["ghost"] = GanttSource{Start: "x"}
			}),
			want: `unknown entity type "ghost"`,
		},
		{
			name: "start property not on the type",
			g: withGantt(func(g *Gantt) {
				g.Sources["project"] = GanttSource{Start: "nope"}
			}),
			want: `start property "nope" not in metamodel`,
		},
		{
			name: "end property wrong type",
			g: withGantt(func(g *Gantt) {
				g.Sources["project"] = GanttSource{End: "status"}
			}),
			want: "must be date- or datetime-typed",
		},
		{
			name: "committed property is a list",
			g: withGantt(func(g *Gantt) {
				g.Sources["epic"] = GanttSource{Committed: "milestones"}
			}),
			want: "is a list; a gantt needs a single date per role",
		},
		{
			name: "multi_parent duplicate gets a dedicated message",
			g:    withGantt(func(g *Gantt) { g.MultiParent = "duplicate" }),
			want: "double-counts every roll-up",
		},
		{
			name: "multi_parent unknown value",
			g:    withGantt(func(g *Gantt) { g.MultiParent = "both" }),
			want: `multi_parent "both" is not valid`,
		},
		{
			name: "on_cycle unknown value",
			g:    withGantt(func(g *Gantt) { g.OnCycle = "ignore" }),
			want: `on_cycle "ignore" is not valid`,
		},
		{
			name: "duplicate hierarchy entry",
			g:    withGantt(func(g *Gantt) { g.Hierarchy = []string{"contains", "has-epic", "contains"} }),
			want: `hierarchy[2] duplicates hierarchy[0] ("contains")`,
		},
		{
			name: "default_depth exceeding max_depth",
			g:    withGantt(func(g *Gantt) { g.DefaultDepth = 20; g.MaxDepth = 3 }),
			want: "default_depth (20) exceeds max_depth (3)",
		},
		{
			name: "default_depth exceeding the DEFAULT max_depth",
			g:    withGantt(func(g *Gantt) { g.DefaultDepth = 20 }),
			want: "exceeds max_depth (10)",
		},
		{
			name: "negative max_depth",
			g:    withGantt(func(g *Gantt) { g.MaxDepth = -1 }),
			want: "max_depth must not be negative",
		},
		{
			name: "negative max_nodes",
			g:    withGantt(func(g *Gantt) { g.MaxNodes = -1 }),
			want: "max_nodes must not be negative",
		},
		{
			name: "negative default_depth",
			g:    withGantt(func(g *Gantt) { g.DefaultDepth = -1 }),
			want: "default_depth must not be negative",
		},
		{
			name: "unparseable where clause is a load error",
			g: withGantt(func(g *Gantt) {
				// No operator at all — the one shape filter.Parse rejects.
				g.Sources["project"] = GanttSource{Where: []string{"status active"}}
			}),
			want: "where[0]",
		},
		{
			name: "where clause referencing unknown property",
			g: withGantt(func(g *Gantt) {
				g.Sources["project"] = GanttSource{Where: []string{"ghost=1"}}
			}),
			want: `where[0] references unknown property "ghost"`,
		},
		{
			name: "label property not on the type",
			g: withGantt(func(g *Gantt) {
				g.Sources["project"] = GanttSource{Label: "nope"}
			}),
			want: `label property "nope" not in metamodel`,
		},
		{
			name: "label omitted with no display property fallback",
			g: withGantt(func(g *Gantt) {
				g.Sources["anon"] = GanttSource{Start: "when"}
			}),
			want: "no display property to fall back to",
		},
		{
			name: "invalid color token",
			g: withGantt(func(g *Gantt) {
				g.Sources["project"] = GanttSource{Color: "#ff0000"}
			}),
			want: "color",
		},
		{
			name: "filter control resolving on no source type",
			g: withGantt(func(g *Gantt) {
				g.FilterControls = []FilterControl{{Property: "ghost"}}
			}),
			want: `not present on any source type`,
		},
		{
			name: "filter control with neither property nor relation",
			g: withGantt(func(g *Gantt) {
				g.FilterControls = []FilterControl{{}}
			}),
			want: "must specify either property or relation",
		},
		{
			name: "tooltip relation field rejected",
			g: withGantt(func(g *Gantt) {
				g.Tooltip = GanttTooltip{Fields: []KanbanCardField{{Relation: "contains"}}}
			}),
			want: "relation fields are not supported on gantt tooltips",
		},
		{
			name: "tooltip field with no property",
			g: withGantt(func(g *Gantt) {
				g.Tooltip = GanttTooltip{Fields: []KanbanCardField{{}}}
			}),
			want: "tooltip.fields[0]: must specify a property",
		},
		{
			name: "tooltip property on no source type",
			g: withGantt(func(g *Gantt) {
				g.Tooltip = GanttTooltip{Fields: []KanbanCardField{{Property: "ghost"}}}
			}),
			want: `tooltip.fields[0]: property "ghost" not present on any source type`,
		},
		{
			name: "filter control unknown relation",
			g: withGantt(func(g *Gantt) {
				g.FilterControls = []FilterControl{{Relation: "ghost-rel"}}
			}),
			want: `references unknown relation "ghost-rel"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateGantts(ganttCfg(tt.g), meta)
			if !containsSubstring(errs, tt.want) {
				t.Errorf("expected an error containing %q, got: %v", tt.want, errs)
			}
		})
	}
}

func TestNormalizeGantts(t *testing.T) {
	cfg := ganttCfg(validGantt())
	NormalizeGantts(cfg)
	g := cfg.Gantts["plan"]

	if g.MultiParent != "first" {
		t.Errorf("MultiParent = %q, want first", g.MultiParent)
	}
	if g.OnCycle != "error" {
		t.Errorf("OnCycle = %q, want error", g.OnCycle)
	}
	if g.DefaultDepth != 2 {
		t.Errorf("DefaultDepth = %d, want 2", g.DefaultDepth)
	}
	if g.MaxDepth != 10 {
		t.Errorf("MaxDepth = %d, want 10", g.MaxDepth)
	}
	if g.MaxNodes != 2000 {
		t.Errorf("MaxNodes = %d, want 2000", g.MaxNodes)
	}

	// Authored values survive normalization.
	cfg2 := ganttCfg(withGantt(func(g *Gantt) {
		g.MultiParent = "error"
		g.OnCycle = "prune"
		g.DefaultDepth = 3
		g.MaxDepth = 5
		g.MaxNodes = 100
	}))
	NormalizeGantts(cfg2)
	g2 := cfg2.Gantts["plan"]
	if g2.MultiParent != "error" || g2.OnCycle != "prune" || g2.DefaultDepth != 3 ||
		g2.MaxDepth != 5 || g2.MaxNodes != 100 {

		t.Errorf("authored values overwritten: %+v", g2)
	}
}

// TestValidateGantts_NavReference pins that a navigation entry naming an
// unknown gantt fails the load.
func TestValidateGantts_NavReference(t *testing.T) {
	cfg := ganttCfg(validGantt())
	cfg.Navigation = []NavigationEntry{{Label: "Plan", Gantt: "ghost"}}

	errs := validateNavigation(cfg)
	if !containsSubstring(errs, `unknown gantt "ghost"`) {
		t.Errorf("expected unknown-gantt error, got: %v", errs)
	}
}
