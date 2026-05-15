package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// validate_test.go covers only the CLI concerns:
//   - flag parsing (parseChecks — --checks syntax)
//   - one representative JSON-output shape test
//
// Validation correctness (cardinality, properties, custom rules,
// filter-by-rule/filter-by-type, warning vs error severity) is
// exercised directly at the workspace layer in analysis_test.go —
// don't duplicate those assertions here.

func TestParseChecks(t *testing.T) {
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
		wantFilter int
		wantErr    string
	}{
		{name: "empty checks", checks: []string{}},
		{name: "all", checks: []string{"all"}, wantCard: true, wantProps: true, wantVals: true},
		{name: "cardinality only", checks: []string{"cardinality"}, wantCard: true},
		{name: "properties only", checks: []string{"properties"}, wantProps: true},
		{name: "validations only", checks: []string{"validations"}, wantVals: true},
		{name: "validation with rule filter", checks: []string{"validations:rule-one"}, wantVals: true, wantFilter: 1},
		{name: "validation with entity type filter", checks: []string{"validations:@ticket"}, wantVals: true, wantFilter: 1},
		{name: "multiple validation filters", checks: []string{"validations:rule-one", "validations:@bug"}, wantVals: true, wantFilter: 2},
		{name: "combined checks", checks: []string{"cardinality", "properties"}, wantCard: true, wantProps: true},
		{name: "unknown check type", checks: []string{"unknown"}, wantErr: "unknown check type"},
		{name: "unknown validation rule", checks: []string{"validations:nonexistent"}, wantErr: "unknown validation rule"},
		{name: "unknown entity type", checks: []string{"validations:@nonexistent"}, wantErr: "unknown entity type"},
		{name: "empty validation filter", checks: []string{"validations:"}, wantErr: "empty validation filter"},
		{name: "empty entity type filter", checks: []string{"validations:@"}, wantErr: "empty entity type"},
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

// seedWorkspace builds a workspace from a seeded memstore using the
// given metamodel and seed function. Used to feed runValidationChecks
// in CLI-layer tests — that function takes *workspace.Workspace
// directly because validate.go (the package-global-free validate
// command) constructs its own workspace for the --check pass.
func seedWorkspace(meta *metamodel.Metamodel, seed func(*storeSeeder)) *workspace.Workspace {
	seeder := newStoreSeeder(meta)
	if seed != nil {
		seed(seeder)
	}
	return workspace.NewForTest(meta, workspace.WithTestStore(seeder.s))
}

// TestRunValidationChecks_JSONOutput exercises the CLI JSON output
// envelope once, using a cardinality violation as the trigger. The
// actual cardinality, property, and custom-rule logic is covered in
// workspace/analysis_test.go; here we only verify the CLI shape.
func TestRunValidationChecks_JSONOutput(t *testing.T) {
	origOut, origChecks, origQuiet := out, validateChecks, quiet
	t.Cleanup(func() {
		out, validateChecks, quiet = origOut, origChecks, origQuiet
	})

	minOne := 1
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

	ws := seedWorkspace(meta, func(s *storeSeeder) {
		s.addEntity(testutil.EntityFor(meta, "feature").ID("FEAT-001"))
	})

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	validateChecks = []string{"cardinality"}

	hasErrors, err := runValidationChecks(context.Background(), ws, out, meta)
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
