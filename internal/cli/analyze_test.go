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

func setupAnalyzeTestGraph() {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPrefix:   "REQ-",
				Properties: map[string]metamodel.PropertyDef{},
			},
			"decision": {
				Label:      "Decision",
				IDPrefix:   "DEC-",
				Properties: map[string]metamodel.PropertyDef{},
			},
			"component": {
				Label:      "Component",
				IDPrefix:   "CMP-",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "Implements",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
			"uses": {
				Label: "Uses",
				From:  []string{"component"},
				To:    []string{"component"},
			},
		},
	}
	ws = workspace.NewForTest(g, meta)
	out = output.New(output.FormatTable)

	// Add test entities
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-002").
		Build())

	g.AddNode(testutil.EntityFor(meta, "decision").
		ID("DEC-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "component").
		ID("CMP-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "component").
		ID("CMP-002").
		Build())

	// Add relations
	g.AddEdge(testutil.NewRelation("DEC-001", "implements", "REQ-001").Build())
	g.AddEdge(testutil.NewRelation("DEC-001", "implements", "REQ-002").Build())
	g.AddEdge(testutil.NewRelation("CMP-001", "uses", "CMP-002").Build())
}

// setupJSONTestOutput sets up JSON output writer and returns the buffer
func setupJSONTestOutput() *bytes.Buffer {
	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)
	return &buf
}

// runJSONTest is a helper that runs an analyze command and verifies the JSON output
func runJSONTest(t *testing.T, name string, setup func(), run func() error, wantStatus string, wantCount int) {
	t.Helper()
	setup()
	buf := setupJSONTestOutput()

	// Reset cached options for each test
	resetAnalyzeOptsCache()

	if err := run(); err != nil {
		t.Fatalf("%s error = %v", name, err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != wantStatus {
		t.Errorf("Expected status %q, got %q", wantStatus, result.Status)
	}
	if result.Count != wantCount {
		t.Errorf("Expected count %d, got %d", wantCount, result.Count)
	}
}

func TestAnalyzeJSONOutput(t *testing.T) {
	minThree := 3

	tests := []struct {
		name       string
		setup      func()
		run        func() error
		wantStatus string
		wantCount  int
	}{
		{
			name: "orphans with issues",
			setup: func() {
				g = graph.New()
				meta = &metamodel.Metamodel{
					Entities: map[string]metamodel.EntityDef{
						"requirement": {
							Label:      "Requirement",
							IDPrefix:   "REQ-",
							Properties: map[string]metamodel.PropertyDef{},
						},
					},
				}
				ws = workspace.NewForTest(g, meta)
				g.AddNode(testutil.EntityFor(meta, "requirement").
					ID("REQ-003").
					Build())
			},
			run:        func() error { return analyzeOrphansCmd.RunE(nil, nil) },
			wantStatus: "warning",
			wantCount:  1,
		},
		{
			name:       "orphans empty",
			setup:      setupAnalyzeTestGraph,
			run:        func() error { return analyzeOrphansCmd.RunE(nil, nil) },
			wantStatus: "success",
			wantCount:  0,
		},
		{
			name: "duplicates with issues",
			setup: func() {
				g = graph.New()
				meta = &metamodel.Metamodel{
					Entities: map[string]metamodel.EntityDef{
						"requirement": {
							Label:      "Requirement",
							IDPrefix:   "REQ-",
							Properties: map[string]metamodel.PropertyDef{},
						},
					},
				}
				ws = workspace.NewForTest(g, meta)
				g.AddNode(testutil.EntityFor(meta, "requirement").
					ID("REQ-001").
					With("title", "Same Title").
					Build())
				g.AddNode(testutil.EntityFor(meta, "requirement").
					ID("REQ-002").
					With("title", "Same Title").
					Build())
			},
			run:        func() error { return analyzeDuplicatesCmd.RunE(nil, nil) },
			wantStatus: "warning",
			wantCount:  1,
		},
		{
			name: "gaps with issues",
			setup: func() {
				g = graph.New()
				meta = &metamodel.Metamodel{
					Entities: map[string]metamodel.EntityDef{
						"requirement": {
							Label:      "Requirement",
							IDPrefix:   "REQ-",
							Properties: map[string]metamodel.PropertyDef{},
						},
					},
				}
				ws = workspace.NewForTest(g, meta)
				g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").Build())
				g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-003").Build())
			},
			run:        func() error { return analyzeGapsCmd.RunE(nil, nil) },
			wantStatus: "warning",
			wantCount:  1,
		},
		{
			name: "cardinality with violations",
			setup: func() {
				setupAnalyzeTestGraph()
				meta.Relations["implements"] = metamodel.RelationDef{
					Label:       "Implements",
					From:        []string{"decision"},
					To:          []string{"requirement"},
					MinOutgoing: &minThree,
				}
			},
			run:        func() error { return analyzeCardinalityCmd.RunE(nil, nil) },
			wantStatus: "warning",
			wantCount:  1,
		},
		{
			name: "validations with errors",
			setup: func() {
				g = graph.New()
				meta = &metamodel.Metamodel{
					Entities: map[string]metamodel.EntityDef{
						"requirement": {
							Label:    "Requirement",
							IDPrefix: "REQ-",
							Properties: map[string]metamodel.PropertyDef{
								"status":   {Type: "string"},
								"priority": {Type: "string"},
							},
						},
					},
					Validations: []metamodel.ValidationRule{
						{
							Name:        "accepted-needs-priority",
							Description: "Accepted requirements must have priority",
							EntityType:  "requirement",
							When:        []string{"status=accepted"},
							Then:        []string{"priority!="},
							Severity:    "error",
						},
					},
				}
				ws = workspace.NewForTest(g, meta)
				g.AddNode(testutil.EntityFor(meta, "requirement").
					ID("REQ-001").
					With("status", "accepted").
					Build())
			},
			run:        func() error { return runValidations(workspace.AnalyzeOptions{}) },
			wantStatus: "error",
			wantCount:  1,
		},
		{
			name:       "all analyses pass",
			setup:      setupAnalyzeTestGraph,
			run:        func() error { return analyzeAllCmd.RunE(nil, nil) },
			wantStatus: "success",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runJSONTest(t, tt.name, tt.setup, tt.run, tt.wantStatus, tt.wantCount)
		})
	}
}

func TestResolveAnalyzeOptsErrors(t *testing.T) {
	// Save and restore original flag values
	origView := analyzeViewName
	origEntry := analyzeEntryID
	defer func() {
		analyzeViewName = origView
		analyzeEntryID = origEntry
		resetAnalyzeOptsCache()
	}()

	tests := []struct {
		name      string
		view      string
		entry     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "no view returns empty options",
			view:    "",
			entry:   "",
			wantErr: false,
		},
		{
			name:      "view without entry returns error",
			view:      "some-view",
			entry:     "",
			wantErr:   true,
			errSubstr: "--entry is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAnalyzeOptsCache()
			analyzeViewName = tt.view
			analyzeEntryID = tt.entry

			opts, err := resolveAnalyzeOpts()

			if tt.wantErr {
				if err == nil {
					t.Error("resolveAnalyzeOpts() expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("resolveAnalyzeOpts() error = %q, want substring %q", err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("resolveAnalyzeOpts() unexpected error: %v", err)
				}
				if tt.view == "" && opts.Scope != nil {
					t.Error("resolveAnalyzeOpts() expected nil scope when no view specified")
				}
			}
		})
	}
}
