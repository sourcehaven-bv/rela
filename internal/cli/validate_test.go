package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func TestParseChecks(t *testing.T) {
	// Create a minimal metamodel for testing
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-"},
			"bug":    {Label: "Bug", IDPrefix: "BUG-"},
		},
		Validations: []metamodel.ValidationRule{
			{Name: "rule-one", EntityType: "ticket"},
			{Name: "rule-two", EntityType: "bug"},
			{Name: "rule-all"},
		},
	}

	tests := []struct {
		name       string
		checks     []string
		wantCard   bool
		wantProps  bool
		wantVals   bool
		wantFilter int // number of validation filters
		wantErr    string
	}{
		{
			name:     "empty checks",
			checks:   []string{},
			wantCard: false, wantProps: false, wantVals: false,
		},
		{
			name:     "all",
			checks:   []string{"all"},
			wantCard: true, wantProps: true, wantVals: true,
		},
		{
			name:     "cardinality only",
			checks:   []string{"cardinality"},
			wantCard: true, wantProps: false, wantVals: false,
		},
		{
			name:     "properties only",
			checks:   []string{"properties"},
			wantCard: false, wantProps: true, wantVals: false,
		},
		{
			name:     "validations only",
			checks:   []string{"validations"},
			wantCard: false, wantProps: false, wantVals: true,
		},
		{
			name:       "validation with rule filter",
			checks:     []string{"validations:rule-one"},
			wantVals:   true,
			wantFilter: 1,
		},
		{
			name:       "validation with entity type filter",
			checks:     []string{"validations:@ticket"},
			wantVals:   true,
			wantFilter: 1,
		},
		{
			name:       "multiple validation filters",
			checks:     []string{"validations:rule-one", "validations:@bug"},
			wantVals:   true,
			wantFilter: 2,
		},
		{
			name:     "combined checks",
			checks:   []string{"cardinality", "properties"},
			wantCard: true, wantProps: true, wantVals: false,
		},
		{
			name:    "unknown check type",
			checks:  []string{"unknown"},
			wantErr: "unknown check type",
		},
		{
			name:    "unknown validation rule",
			checks:  []string{"validations:nonexistent"},
			wantErr: "unknown validation rule",
		},
		{
			name:    "unknown entity type",
			checks:  []string{"validations:@nonexistent"},
			wantErr: "unknown entity type",
		},
		{
			name:    "empty validation filter",
			checks:  []string{"validations:"},
			wantErr: "empty validation filter",
		},
		{
			name:    "empty entity type filter",
			checks:  []string{"validations:@"},
			wantErr: "empty entity type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseChecks(tt.checks, meta)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.cardinality != tt.wantCard {
				t.Errorf("cardinality = %v, want %v", result.cardinality, tt.wantCard)
			}
			if result.properties != tt.wantProps {
				t.Errorf("properties = %v, want %v", result.properties, tt.wantProps)
			}
			if result.validations != tt.wantVals {
				t.Errorf("validations = %v, want %v", result.validations, tt.wantVals)
			}
			if len(result.validationFilters) != tt.wantFilter {
				t.Errorf("validationFilters count = %d, want %d", len(result.validationFilters), tt.wantFilter)
			}
		})
	}
}

func TestRunValidationChecks_Cardinality(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	// Setup test graph with cardinality violations
	minOne := 1
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement", IDPrefix: "REQ-"},
			"feature":     {Label: "Feature", IDPrefix: "FEAT-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label:       "Implements",
				From:        []string{"feature"},
				To:          []string{"requirement"},
				MinOutgoing: &minOne,
			},
		},
	}
	meta.InitAliases()

	// Add a feature without any implements relation (violation)
	g.AddNode(testutil.EntityFor(meta, "feature").ID("FEAT-001").Build())
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").Build())

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	validateChecks = []string{"cardinality"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasErrors {
		t.Error("expected hasErrors=true for cardinality violations")
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "FEAT-001") {
		t.Errorf("output should contain FEAT-001 violation, got: %s", outputStr)
	}
}

func TestRunValidationChecks_Properties(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	// Setup test graph with property errors
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "status"},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {Values: []string{"open", "closed"}},
		},
	}
	meta.InitAliases()

	// Add ticket with invalid status value
	ticket := testutil.EntityFor(meta, "ticket").
		ID("TKT-001").
		WithProperty("status", "invalid").
		Build()
	g.AddNode(ticket)

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	validateChecks = []string{"properties"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasErrors {
		t.Error("expected hasErrors=true for property errors")
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "TKT-001") {
		t.Errorf("output should contain TKT-001 error, got: %s", outputStr)
	}
}

func TestRunValidationChecks_Validations(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	// Setup test graph with custom validation rule violations
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "string"},
					"assignee": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "in-progress-needs-assignee",
				Description: "In-progress tickets must have an assignee",
				EntityType:  "ticket",
				When:        []string{"status=in-progress"},
				Then:        []string{"assignee!="},
				Severity:    "error",
			},
		},
	}
	meta.InitAliases()

	// Add ticket that violates the rule (in-progress but no assignee)
	ticket := testutil.EntityFor(meta, "ticket").
		ID("TKT-001").
		WithProperty("status", "in-progress").
		Build()
	g.AddNode(ticket)

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	validateChecks = []string{"validations"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasErrors {
		t.Error("expected hasErrors=true for validation errors")
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "TKT-001") {
		t.Errorf("output should contain TKT-001 error, got: %s", outputStr)
	}
}

func TestRunValidationChecks_ValidationFilter_RuleName(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "rule-to-run",
				Description: "Should run",
				EntityType:  "ticket",
				When:        []string{"status=bad"},
				Then:        []string{"status!=bad"}, // Always fails for status=bad
				Severity:    "error",
			},
			{
				Name:        "rule-to-skip",
				Description: "Should not run",
				EntityType:  "ticket",
				When:        []string{"status=bad"},
				Then:        []string{"status!=bad"},
				Severity:    "error",
			},
		},
	}
	meta.InitAliases()

	// Add ticket that would violate both rules
	ticket := testutil.EntityFor(meta, "ticket").
		ID("TKT-001").
		WithProperty("status", "bad").
		Build()
	g.AddNode(ticket)

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	// Only run "rule-to-run"
	validateChecks = []string{"validations:rule-to-run"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasErrors {
		t.Error("expected hasErrors=true")
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "Should run") {
		t.Errorf("output should contain 'Should run' rule, got: %s", outputStr)
	}
	if strings.Contains(outputStr, "Should not run") {
		t.Errorf("output should NOT contain 'Should not run' rule, got: %s", outputStr)
	}
}

func TestRunValidationChecks_ValidationFilter_EntityType(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
			},
			"bug": {
				Label:    "Bug",
				IDPrefix: "BUG-",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "ticket-rule",
				Description: "Ticket rule",
				EntityType:  "ticket",
				When:        []string{"status=bad"},
				Then:        []string{"status!=bad"},
				Severity:    "error",
			},
			{
				Name:        "bug-rule",
				Description: "Bug rule",
				EntityType:  "bug",
				When:        []string{"status=bad"},
				Then:        []string{"status!=bad"},
				Severity:    "error",
			},
		},
	}
	meta.InitAliases()

	// Add both ticket and bug that would violate their respective rules
	g.AddNode(testutil.EntityFor(meta, "ticket").ID("TKT-001").WithProperty("status", "bad").Build())
	g.AddNode(testutil.EntityFor(meta, "bug").ID("BUG-001").WithProperty("status", "bad").Build())

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	// Only run ticket rules
	validateChecks = []string{"validations:@ticket"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasErrors {
		t.Error("expected hasErrors=true")
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "Ticket rule") {
		t.Errorf("output should contain 'Ticket rule', got: %s", outputStr)
	}
	if strings.Contains(outputStr, "Bug rule") {
		t.Errorf("output should NOT contain 'Bug rule', got: %s", outputStr)
	}
}

func TestRunValidationChecks_JSONOutput(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	minOne := 1
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"feature": {Label: "Feature", IDPrefix: "FEAT-"},
			"concept": {Label: "Concept", IDPrefix: "CON-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"requires": {
				Label:       "Requires",
				From:        []string{"feature"},
				To:          []string{"concept"},
				MinOutgoing: &minOne,
			},
		},
	}
	meta.InitAliases()

	g.AddNode(testutil.EntityFor(meta, "feature").ID("FEAT-001").Build())

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	validateChecks = []string{"cardinality"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasErrors {
		t.Error("expected hasErrors=true")
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v, raw: %s", err, buf.String())
	}

	if result.Status != "error" {
		t.Errorf("status = %q, want 'error'", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("count = %d, want 1", result.Count)
	}
}

func TestRunValidationChecks_WarningsOnly(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	// Setup with warning-severity rule only
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"description": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "tickets-should-have-description",
				Description: "Tickets should have a description",
				EntityType:  "ticket",
				Then:        []string{"description!="},
				Severity:    "warning", // Warning, not error
			},
		},
	}
	meta.InitAliases()

	// Add ticket without description
	g.AddNode(testutil.EntityFor(meta, "ticket").ID("TKT-001").Build())

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	validateChecks = []string{"validations"}

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Warnings should not cause hasErrors=true
	if hasErrors {
		t.Error("expected hasErrors=false for warnings only")
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "warning") || !strings.Contains(outputStr, "TKT-001") {
		t.Errorf("output should contain warning for TKT-001, got: %s", outputStr)
	}
}

func TestRunValidationChecks_NoViolations(t *testing.T) {
	// Save and restore global state
	origWs, origOut, origChecks, origQuiet := ws, out, validateChecks, quiet
	t.Cleanup(func() {
		ws, out, validateChecks, quiet = origWs, origOut, origChecks, origQuiet
	})

	// Setup without violations
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-"},
		},
	}
	meta.InitAliases()

	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	validateChecks = []string{"all"}
	quiet = false

	hasErrors, err := runValidationChecks(ws, out, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hasErrors {
		t.Error("expected hasErrors=false when no violations")
	}
}
