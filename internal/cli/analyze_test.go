package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func setupAnalyzeTestGraph() {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC-",
			},
			"component": {
				Label:    "Component",
				IDPrefix: "CMP-",
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
	req1 := model.NewEntity("REQ-001", "requirement")
	req1.Properties["title"] = "First Requirement"
	g.AddNode(req1)

	req2 := model.NewEntity("REQ-002", "requirement")
	req2.Properties["title"] = "Second Requirement"
	g.AddNode(req2)

	dec1 := model.NewEntity("DEC-001", "decision")
	dec1.Properties["title"] = "Important Decision"
	g.AddNode(dec1)

	cmp1 := model.NewEntity("CMP-001", "component")
	cmp1.Properties["title"] = "API Component"
	g.AddNode(cmp1)

	cmp2 := model.NewEntity("CMP-002", "component")
	cmp2.Properties["title"] = "Database Component"
	g.AddNode(cmp2)

	// Add relations
	g.AddEdge(model.NewRelation("DEC-001", "implements", "REQ-001"))
	g.AddEdge(model.NewRelation("DEC-001", "implements", "REQ-002"))
	g.AddEdge(model.NewRelation("CMP-001", "uses", "CMP-002"))
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
						"requirement": {Label: "Requirement", IDPrefix: "REQ-"},
					},
				}
				ws = workspace.NewForTest(g, meta)
				orphan := model.NewEntity("REQ-003", "requirement")
				orphan.Properties["title"] = "Orphan Requirement"
				g.AddNode(orphan)
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
						"requirement": {Label: "Requirement", IDPrefix: "REQ-"},
					},
				}
				ws = workspace.NewForTest(g, meta)
				e1 := model.NewEntity("REQ-001", "requirement")
				e1.Properties["title"] = "Same Title"
				g.AddNode(e1)
				e2 := model.NewEntity("REQ-002", "requirement")
				e2.Properties["title"] = "Same Title"
				g.AddNode(e2)
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
						"requirement": {Label: "Requirement", IDPrefix: "REQ-"},
					},
				}
				ws = workspace.NewForTest(g, meta)
				g.AddNode(model.NewEntity("REQ-001", "requirement"))
				g.AddNode(model.NewEntity("REQ-003", "requirement"))
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
				e := model.NewEntity("REQ-001", "requirement")
				e.Properties["status"] = "accepted"
				g.AddNode(e)
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
