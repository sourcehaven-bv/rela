package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
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
