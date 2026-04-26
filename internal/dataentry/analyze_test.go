package dataentry

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// newTestApp creates a minimal App with the given fixture and metamodel for testing.
func newTestApp(f *fixture, meta *metamodel.Metamodel) *App {
	return newAppFromParts(&Config{}, meta, f)
}

func TestAnalyzeOrphans(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Orphan"}})
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "Connected A"}})
	g.AddNode(&entity.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{"title": "Connected B"}})
	g.AddEdge(&entity.Relation{From: "T-002", Type: "blocks", To: "T-003"})

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
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{}})
	g.AddEdge(&entity.Relation{From: "T-001", Type: "blocks", To: "T-002"})

	app := newTestApp(g, meta)
	section := app.analyzeOrphans()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(section.Issues))
	}
}

func TestAnalyzeDuplicates(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Setup CI"}})
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "setup ci"}})
	g.AddNode(&entity.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{"title": "Unique"}})

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
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
			}},
		},
	}

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Alpha"}})
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "Beta"}})

	app := newTestApp(g, meta)
	section := app.analyzeDuplicates()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 duplicate issues, got %d", len(section.Issues))
	}
}

func TestAnalyzeGaps(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {IDPrefix: "T-", Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})
	// T-002 missing
	g.AddNode(&entity.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{}})

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
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"component": {IDType: "manual", IDPrefix: "C-", Properties: map[string]metamodel.PropertyDef{}},
		},
	}

	g.AddNode(&entity.Entity{ID: "C-001", Type: "component", Properties: map[string]interface{}{}})
	g.AddNode(&entity.Entity{ID: "C-005", Type: "component", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	section := app.analyzeGaps()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 gap issues for manual ID type, got %d", len(section.Issues))
	}
}

func TestAnalyzeCardinality(t *testing.T) {
	g := newFixture()
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

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "A"}})
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "B"}})
	g.AddNode(&entity.Entity{ID: "C-001", Type: "component", Properties: map[string]interface{}{"name": "Auth"}})
	// Only T-001 implements C-001; T-002 has no implements relation
	g.AddEdge(&entity.Relation{From: "T-001", Type: "implements", To: "C-001"})

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
	g := newFixture()
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

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})
	g.AddNode(&entity.Entity{ID: "C-001", Type: "component", Properties: map[string]interface{}{}})
	g.AddEdge(&entity.Relation{From: "T-001", Type: "implements", To: "C-001"})

	app := newTestApp(g, meta)
	section := app.analyzeCardinality()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 violations, got %d", len(section.Issues))
	}
}

func TestAnalyzeProperties(t *testing.T) {
	g := newFixture()
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
	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"status": "invalid"}})
	// Valid entity
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"title": "Good", "status": "open"}})

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
	g := newFixture()
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

	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"title": "Valid"}})

	app := newTestApp(g, meta)
	section := app.analyzeProperties()

	if len(section.Issues) != 0 {
		t.Errorf("expected 0 property issues, got %d", len(section.Issues))
	}
}

func TestAnalyzeValidations(t *testing.T) {
	g := newFixture()
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
	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{"status": "accepted"}})
	// Accepted with priority — OK
	g.AddNode(&entity.Entity{ID: "T-002", Type: "ticket", Properties: map[string]interface{}{"status": "accepted", "priority": "high"}})
	// Draft — rule doesn't apply
	g.AddNode(&entity.Entity{ID: "T-003", Type: "ticket", Properties: map[string]interface{}{"status": "draft"}})

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

// TestAnalyzeValidations_SurfacesScriptError covers RR-MG0LG: a Lua
// rule that fails to compile must appear as an error issue in the
// analyze view rather than vanishing silently. The data-entry web
// surface uses CheckRuleFull so ScriptErrors and LoadErrors no
// longer disappear on the way through GenericValidator.
func TestAnalyzeValidations_SurfacesScriptError(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "broken-rule",
				EntityType: "ticket",
				Lua:        `if oops invalid`,
			},
		},
	}
	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) == 0 {
		t.Fatal("expected the script error to surface as an analysis issue, got 0 issues")
	}
	var foundScriptError bool
	for _, issue := range section.Issues {
		if issue.Severity != "error" {
			continue
		}
		if issue.Title == "broken-rule" || strings.Contains(issue.Message, "Validation script failed") {
			foundScriptError = true
		}
	}
	if !foundScriptError {
		t.Errorf("expected an issue tagged with the broken rule; got %+v", section.Issues)
	}
}

// TestAnalyzeValidations_SurfacesLoadError covers RR-MG0LG for the
// load-error path: a `lua_file:` rule whose script does not exist
// must appear as an error issue rather than silently dropping.
func TestAnalyzeValidations_SurfacesLoadError(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "missing-script",
				EntityType: "ticket",
				LuaFile:    "no-such-file.lua",
			},
		},
	}
	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) == 0 {
		t.Fatal("expected the load error to surface as an analysis issue, got 0 issues")
	}
	var foundLoadError bool
	for _, issue := range section.Issues {
		if issue.Severity == "error" && issue.Title == "missing-script" {
			foundLoadError = true
			if !strings.Contains(issue.Message, "load failed") {
				t.Errorf("expected message to mention load failed, got %q", issue.Message)
			}
		}
	}
	if !foundLoadError {
		t.Errorf("expected an issue tagged with rule name 'missing-script'; got %+v", section.Issues)
	}
}

func TestAnalyzeValidations_NoRules(t *testing.T) {
	g := newFixture()
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
	g := newFixture()
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
	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})

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
	g := newFixture()
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
	g := newFixture()
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
	g.AddNode(&entity.Entity{ID: "T-001", Type: "ticket", Properties: map[string]interface{}{}})

	app := newTestApp(g, meta)
	errors, warnings := app.analysisIssueCounts()

	if errors < 1 {
		t.Errorf("expected at least 1 error, got %d", errors)
	}
	if warnings < 1 {
		t.Errorf("expected at least 1 warning, got %d", warnings)
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

func TestAnalyzeValidationsWithContentRules(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"decision": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: "string"},
				"status": {Type: "string"},
			}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "decision-needs-context",
				Description: "Decisions must have Context section",
				EntityType:  "decision",
				When:        []string{"status=accepted"},
				Content: &metamodel.ContentRule{
					RequiredHeaders: []metamodel.HeaderCheck{
						{Header: "## Context"},
					},
				},
				Severity: "error",
			},
		},
	}

	// Accepted without Context section — violation
	g.AddNode(&entity.Entity{
		ID:         "DEC-001",
		Type:       "decision",
		Properties: map[string]interface{}{"title": "Auth", "status": "accepted"},
		Content:    "# Decision\nSome text without context section",
	})
	// Accepted with Context section — OK
	g.AddNode(&entity.Entity{
		ID:         "DEC-002",
		Type:       "decision",
		Properties: map[string]interface{}{"title": "Database", "status": "accepted"},
		Content:    "# Decision\n## Context\nWe need to decide...",
	})
	// Draft — rule doesn't apply
	g.AddNode(&entity.Entity{
		ID:         "DEC-003",
		Type:       "decision",
		Properties: map[string]interface{}{"title": "Draft", "status": "draft"},
		Content:    "# Draft decision",
	})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 content validation issue, got %d", len(section.Issues))
	}
	if section.Issues[0].EntityID != "DEC-001" {
		t.Errorf("expected violation on DEC-001, got %s", section.Issues[0].EntityID)
	}
	if section.Issues[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", section.Issues[0].Severity)
	}
}

func TestAnalyzeValidationsWithCombinedRules(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"decision": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: "string"},
				"status": {Type: "string"},
				"owner":  {Type: "string"},
			}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "accepted-decision-complete",
				Description: "Accepted decisions need owner and Context section",
				EntityType:  "decision",
				When:        []string{"status=accepted"},
				Then:        []string{"owner!="},
				Content: &metamodel.ContentRule{
					RequiredHeaders: []metamodel.HeaderCheck{
						{Header: "## Context"},
					},
				},
				Severity: "error",
			},
		},
	}

	// Missing owner — then fails
	g.AddNode(&entity.Entity{
		ID:         "DEC-001",
		Type:       "decision",
		Properties: map[string]interface{}{"title": "No owner", "status": "accepted"},
		Content:    "# Decision\n## Context\nHas context",
	})
	// Has owner but missing Context — content fails
	g.AddNode(&entity.Entity{
		ID:         "DEC-002",
		Type:       "decision",
		Properties: map[string]interface{}{"title": "No context", "status": "accepted", "owner": "Alice"},
		Content:    "# Decision\nNo context section",
	})
	// Has both — OK
	g.AddNode(&entity.Entity{
		ID:         "DEC-003",
		Type:       "decision",
		Properties: map[string]interface{}{"title": "Complete", "status": "accepted", "owner": "Bob"},
		Content:    "# Decision\n## Context\nAll good",
	})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) != 2 {
		t.Fatalf("expected 2 violations (DEC-001 and DEC-002), got %d", len(section.Issues))
	}

	// Check both violations are present
	ids := make(map[string]bool)
	for _, issue := range section.Issues {
		ids[issue.EntityID] = true
	}
	if !ids["DEC-001"] {
		t.Error("expected violation on DEC-001")
	}
	if !ids["DEC-002"] {
		t.Error("expected violation on DEC-002")
	}
}

func TestAnalyzeValidationsWithChecklistRule(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"checklist": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: "string"},
				"status": {Type: "string"},
			}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "done-checklist-complete",
				Description: "Done checklists must have all items checked",
				EntityType:  "checklist",
				When:        []string{"status=done"},
				Content: &metamodel.ContentRule{
					Checklist: &metamodel.ChecklistRule{
						AllChecked: true,
					},
				},
				Severity: "error",
			},
		},
	}

	// Done with unchecked items — violation
	g.AddNode(&entity.Entity{
		ID:         "CHK-001",
		Type:       "checklist",
		Properties: map[string]interface{}{"title": "Incomplete", "status": "done"},
		Content:    "- [x] Done item\n- [ ] Not done item",
	})
	// Done with all checked — OK
	g.AddNode(&entity.Entity{
		ID:         "CHK-002",
		Type:       "checklist",
		Properties: map[string]interface{}{"title": "Complete", "status": "done"},
		Content:    "- [x] Done 1\n- [x] Done 2",
	})
	// In-progress — rule doesn't apply
	g.AddNode(&entity.Entity{
		ID:         "CHK-003",
		Type:       "checklist",
		Properties: map[string]interface{}{"title": "WIP", "status": "in-progress"},
		Content:    "- [ ] Not started",
	})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 checklist validation issue, got %d", len(section.Issues))
	}
	if section.Issues[0].EntityID != "CHK-001" {
		t.Errorf("expected violation on CHK-001, got %s", section.Issues[0].EntityID)
	}
	if section.Issues[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", section.Issues[0].Severity)
	}
}

func TestAnalyzeValidationsWithChecklistAllowSkipped(t *testing.T) {
	g := newFixture()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"checklist": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: "string"},
				"status": {Type: "string"},
			}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "done-checklist-allow-skipped",
				Description: "Done checklists must have all items checked or skipped",
				EntityType:  "checklist",
				When:        []string{"status=done"},
				Content: &metamodel.ContentRule{
					Checklist: &metamodel.ChecklistRule{
						AllChecked:   true,
						AllowSkipped: true,
					},
				},
				Severity: "error",
			},
		},
	}

	// Done with skipped item — OK
	g.AddNode(&entity.Entity{
		ID:         "CHK-001",
		Type:       "checklist",
		Properties: map[string]interface{}{"title": "Skipped", "status": "done"},
		Content:    "- [x] Done item\n- [x] ~~Skipped item~~ (N/A: reason)",
	})
	// Done with unchecked non-skipped — violation
	g.AddNode(&entity.Entity{
		ID:         "CHK-002",
		Type:       "checklist",
		Properties: map[string]interface{}{"title": "Incomplete", "status": "done"},
		Content:    "- [x] Done item\n- [ ] Not done, not skipped",
	})

	app := newTestApp(g, meta)
	section := app.analyzeValidations()

	if len(section.Issues) != 1 {
		t.Fatalf("expected 1 checklist validation issue, got %d", len(section.Issues))
	}
	if section.Issues[0].EntityID != "CHK-002" {
		t.Errorf("expected violation on CHK-002, got %s", section.Issues[0].EntityID)
	}
}
