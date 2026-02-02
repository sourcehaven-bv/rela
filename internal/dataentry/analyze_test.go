package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// newTestApp creates a minimal App with the given graph and metamodel for testing.
func newTestApp(g *graph.Graph, meta *metamodel.Metamodel) *App {
	return &App{
		g:    g,
		meta: meta,
		Cfg:  &Config{},
	}
}

func TestAnalyzeOrphans(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Orphan"}})
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "Connected A"}})
	g.AddNode(&model.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{"title": "Connected B"}})
	g.AddEdge(&model.Relation{From: "T-002", Type: "blocks", To: "T-003"})

	app := newTestApp(g, meta)
	section := app.analyzeOrphans()

	if section.Name != "Orphans" {
		t.Errorf("expected section name 'Orphans', got %q", section.Name)
	}
	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(section.Issues))
	}
	if section.Issues[0].EntityID != "T-001" {
		t.Errorf("expected orphan T-001, got %s", section.Issues[0].EntityID)
	}
	if section.Issues[0].Severity != "warning" {
		t.Errorf("expected severity 'warning', got %q", section.Issues[0].Severity)
	}
}

func TestAnalyzeOrphans_NoOrphans(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{}})
	g.AddEdge(&model.Relation{From: "T-001", Type: "blocks", To: "T-002"})

	app := newTestApp(g, meta)
	section := app.analyzeOrphans()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(section.Issues))
	}
}

func TestAnalyzeDuplicates(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Setup CI"}})
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "setup ci"}})
	g.AddNode(&model.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{"title": "Unique"}})

	app := newTestApp(g, meta)
	section := app.analyzeDuplicates()

	if len(section.Issues) != 2 {
		t.Fatalf("expected 2 duplicate issues (for the pair), got %d", len(section.Issues))
	}
	// Both should reference the same title group
	if section.Issues[0].EntityID != "T-001" || section.Issues[1].EntityID != "T-002" {
		t.Errorf("expected T-001 and T-002, got %s and %s",
			section.Issues[0].EntityID, section.Issues[1].EntityID)
	}
}

func TestAnalyzeDuplicates_NoDuplicates(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
			}},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Alpha"}})
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "Beta"}})

	app := newTestApp(g, meta)
	section := app.analyzeDuplicates()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 duplicate issues, got %d", len(section.Issues))
	}
}

func TestAnalyzeGaps(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {IDPrefix: "T-", Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})
	// T-002 missing
	g.AddNode(&model.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	section := app.analyzeGaps()

	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 gap issue, got %d", len(section.Issues))
	}
	if section.Issues[0].Message != "Missing ID: T-002" {
		t.Errorf("expected 'Missing ID: T-002', got %q", section.Issues[0].Message)
	}
}

func TestAnalyzeGaps_ManualIDsSkipped(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"component": {IDType: "manual", IDPrefix: "C-", Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	g.AddNode(&model.Entity{ID: "C-001", Type: "component", Properties: map[string]interface{}{}})
	g.AddNode(&model.Entity{ID: "C-005", Type: "component", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	section := app.analyzeGaps()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 gap issues for manual ID type, got %d", len(section.Issues))
	}
}

func TestAnalyzeCardinality(t *testing.T) {
	g := graph.New()
	min1 := 1
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
			}},
			"component": {Properties: map[string]metamodel.PropertyDef{
				"name": {Type: "string"},
			}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				From:        []string{"ticket"},
				To:          []string{"component"},
				MinOutgoing: &min1, // Each ticket must implement at least 1 component
			},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "B"}})
	g.AddNode(&model.Entity{ID: "C-001", Type: "component", Properties: map[string]interface{}{"name": "Auth"}})
	// Only T-001 implements C-001; T-002 has no implements relation
	g.AddEdge(&model.Relation{From: "T-001", Type: "implements", To: "C-001"})

	app := newTestApp(g, meta)
	section := app.analyzeCardinality()

	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 cardinality violation, got %d", len(section.Issues))
	}
	if section.Issues[0].EntityID != "T-002" {
		t.Errorf("expected violation on T-002, got %s", section.Issues[0].EntityID)
	}
	if section.Issues[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", section.Issues[0].Severity)
	}
}

func TestAnalyzeCardinality_AllSatisfied(t *testing.T) {
	g := graph.New()
	min1 := 1
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":    {Properties: map[string]metamodel.PropertyDef{}},
			"component": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				From:        []string{"ticket"},
				To:          []string{"component"},
				MinOutgoing: &min1,
			},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})
	g.AddNode(&model.Entity{ID: "C-001", Type: "component", Properties: map[string]interface{}{}})
	g.AddEdge(&model.Relation{From: "T-001", Type: "implements", To: "C-001"})

	app := newTestApp(g, meta)
	section := app.analyzeCardinality()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 violations, got %d", len(section.Issues))
	}
}

func TestAnalyzeProperties(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefix: "T-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "enum", Values: []string{"open", "closed"}},
				},
			},
		},
	}

	// Missing required title, invalid enum value
	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"status": "invalid"}})
	// Valid entity
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "Good", "status": "open"}})

	app := newTestApp(g, meta)
	section := app.analyzeProperties()

	if len(section.Issues) < 2 {
		t.Fatalf("expected at least 2 property errors (missing title + invalid enum), got %d", len(section.Issues))
	}
	// All issues for T-001
	for _, issue := range section.Issues {
		if issue.EntityID != "T-001" {
			t.Errorf("expected all issues for T-001, got issue for %s", issue.EntityID)
		}
		if issue.Severity != "error" {
			t.Errorf("expected severity 'error', got %q", issue.Severity)
		}
	}
}

func TestAnalyzeProperties_AllValid(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefix: "T-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}

	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Valid"}})

	app := newTestApp(g, meta)
	section := app.analyzeProperties()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 property issues, got %d", len(section.Issues))
	}
}

func TestAnalyzeValidations(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"status":   {Type: "string"},
				"priority": {Type: "string"},
			}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "accepted-needs-priority",
				Description: "Accepted tickets must have priority",
				EntityType:  "ticket",
				When:        []string{"status=accepted"},
				Then:        []string{"priority!="},
				Severity:    "error",
			},
		},
	}

	// Accepted without priority — violation
	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"status": "accepted"}})
	// Accepted with priority — OK
	g.AddNode(&model.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"status": "accepted", "priority": "high"}})
	// Draft — rule doesn't apply
	g.AddNode(&model.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{"status": "draft"}})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 validation issue, got %d", len(section.Issues))
	}
	if section.Issues[0].EntityID != "T-001" {
		t.Errorf("expected violation on T-001, got %s", section.Issues[0].EntityID)
	}
	if section.Issues[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", section.Issues[0].Severity)
	}
}

func TestAnalyzeValidations_NoRules(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 issues with no rules, got %d", len(section.Issues))
	}
}

func TestRunAnalysis(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefix: "T-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}

	// Orphan with missing required property
	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	result := app.runAnalysis()

	if len(result.Sections) != 6 {
		t.Errorf("expected 6 sections, got %d", len(result.Sections))
	}
	// Should have at least 1 error (missing required property) and 1 warning (orphan)
	if result.ErrorCount < 1 {
		t.Errorf("expected at least 1 error, got %d", result.ErrorCount)
	}
	if result.WarningCount < 1 {
		t.Errorf("expected at least 1 warning, got %d", result.WarningCount)
	}
}

func TestRunAnalysis_EmptyGraph(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}

	app := newTestApp(g, meta)
	result := app.runAnalysis()

	if result.ErrorCount != 0 {
		t.Errorf("expected 0 errors for empty graph, got %d", result.ErrorCount)
	}
	if result.WarningCount != 0 {
		t.Errorf("expected 0 warnings for empty graph, got %d", result.WarningCount)
	}
}

func TestAnalysisSectionCounts(t *testing.T) {
	section := AnalysisSection{
		Issues: []AnalysisIssue{
			{Severity: "error"},
			{Severity: "warning"},
			{Severity: "error"},
			{Severity: "warning"},
			{Severity: "warning"},
		},
	}

	if section.ErrorCount() != 2 {
		t.Errorf("expected 2 errors, got %d", section.ErrorCount())
	}
	if section.WarningCount() != 3 {
		t.Errorf("expected 3 warnings, got %d", section.WarningCount())
	}
}

func TestAnalysisIssueCounts(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefix: "T-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
	}

	// Orphan (warning) + missing required title (error)
	g.AddNode(&model.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	errors, warnings := app.analysisIssueCounts()

	if errors < 1 {
		t.Errorf("expected at least 1 error, got %d", errors)
	}
	if warnings < 1 {
		t.Errorf("expected at least 1 warning, got %d", warnings)
	}
}

func TestCountEdgesByType(t *testing.T) {
	edges := []*model.Relation{
		{From: "A", Type: "blocks", To: "B"},
		{From: "A", Type: "implements", To: "C"},
		{From: "A", Type: "blocks", To: "D"},
	}

	if got := countEdgesByType(edges, "blocks"); got != 2 {
		t.Errorf("expected 2 'blocks' edges, got %d", got)
	}
	if got := countEdgesByType(edges, "implements"); got != 1 {
		t.Errorf("expected 1 'implements' edge, got %d", got)
	}
	if got := countEdgesByType(edges, "unknown"); got != 0 {
		t.Errorf("expected 0 'unknown' edges, got %d", got)
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello world"},
		{"  Hello   World  ", "hello world"},
		{"", ""},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := normalizeTitle(tt.input)
		if got != tt.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
