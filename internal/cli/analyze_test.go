package cli

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// analyze_test.go covers the CLI JSON output shape. The underlying
// analysis correctness (orphans, duplicates, cardinality, properties,
// validations) is exercised directly in internal/workspace/analysis_test.go.
// Keep only:
//   - one representative JSON-output test (so the CLI wiring is covered)
//   - the gaps test (not currently covered at the workspace layer)

// setupJSONTestOutput sets up JSON output writer and returns the buffer.
func setupJSONTestOutput() *bytes.Buffer {
	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)
	return &buf
}

// TestAnalyzeOrphansJSONOutput is the canonical CLI JSON-shape test:
// it runs one analysis command through the CLI and verifies the
// envelope (status, count) is correctly populated.
func TestAnalyzeOrphansJSONOutput(t *testing.T) {
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPrefix:   "REQ-",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "requirement").ID("REQ-003"))
	applySeeder(seeder)

	buf := setupJSONTestOutput()

	if err := analyzeOrphansCmd.RunE(nil, nil); err != nil {
		t.Fatalf("analyze orphans error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "warning" {
		t.Errorf("Expected status %q, got %q", "warning", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count %d, got %d", 1, result.Count)
	}
}

// TestAnalyzeValidations_NonZeroExitOnScriptError covers RR-3H1QC:
// `rela analyze validations` must return a non-zero exit when a Lua
// rule fails to compile/run, mirroring `rela validate --check
// validations`. Otherwise CI piping the command sees clean runs even
// when rules silently fail.
func TestAnalyzeValidations_NonZeroExitOnScriptError(t *testing.T) {
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{Name: "broken", EntityType: "ticket", Lua: `if oops invalid`},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "ticket").ID("TKT-001"))
	applySeeder(seeder)

	out = output.NewWithWriter(&bytes.Buffer{}, output.FormatTable)
	err := runValidations(context.Background(), workspace.AnalyzeOptions{})

	if err == nil {
		t.Fatal("expected non-zero exit error, got nil")
	}
	var exitErr *errors.ExitError
	if !stderrors.As(err, &exitErr) {
		t.Fatalf("expected *errors.ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.Code)
	}
}

// TestAnalyzeAll_JSONIncludesScriptAndLoadErrorCounts covers
// RR-NO4VF: `analyze all --output json` must include
// validation_script_errors and validation_load_errors so CI
// consumers parsing JSON see rule failures rather than silently
// reading 0s while text output shows failures.
func TestAnalyzeAll_JSONIncludesScriptAndLoadErrorCounts(t *testing.T) {
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{Name: "broken", EntityType: "ticket", Lua: `if oops invalid`},
			{Name: "missing", EntityType: "ticket", LuaFile: "no-such-file.lua"},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "ticket").ID("TKT-001"))
	applySeeder(seeder)

	buf := setupJSONTestOutput()
	analyzeAllCmd.SetContext(context.Background())
	if err := analyzeAllCmd.RunE(analyzeAllCmd, nil); err != nil {
		t.Fatalf("analyze all: %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	details, ok := result.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("Details is not a map: %T", result.Details)
	}
	scriptErrs, _ := details["validation_script_errors"].(float64)
	loadErrs, _ := details["validation_load_errors"].(float64)
	if scriptErrs != 1 {
		t.Errorf("validation_script_errors = %v, want 1", scriptErrs)
	}
	if loadErrs != 1 {
		t.Errorf("validation_load_errors = %v, want 1", loadErrs)
	}
	if result.Status != "error" {
		t.Errorf("Status = %q, want error (rule failures must trigger error status)", result.Status)
	}
}

// TestAnalyzeGaps covers the gaps analysis, which has no workspace-layer
// test today. Runs via the CLI so we simultaneously exercise the JSON
// output envelope.
func TestAnalyzeGaps(t *testing.T) {
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPrefix:   "REQ-",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "requirement").ID("REQ-001"))
	seeder.addEntity(testutil.EntityFor(meta, "requirement").ID("REQ-003"))
	applySeeder(seeder)

	buf := setupJSONTestOutput()

	if err := analyzeGapsCmd.RunE(nil, nil); err != nil {
		t.Fatalf("analyze gaps error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "warning" {
		t.Errorf("Expected status %q, got %q", "warning", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count %d, got %d", 1, result.Count)
	}
}
