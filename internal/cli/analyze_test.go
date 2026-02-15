package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/output"
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

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple lowercase",
			input: "Hello World",
			want:  "hello world",
		},
		{
			name:  "trim whitespace",
			input: "  Hello  ",
			want:  "hello",
		},
		{
			name:  "collapse multiple spaces",
			input: "Hello    World",
			want:  "hello world",
		},
		{
			name:  "mixed case with extra spaces",
			input: "  HELLO   WoRlD  ",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  "",
		},
		{
			name:  "tabs and newlines",
			input: "Hello\t\nWorld",
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountOutgoingByType(t *testing.T) {
	setupAnalyzeTestGraph()

	tests := []struct {
		name     string
		entityID string
		relType  string
		want     int
	}{
		{
			name:     "DEC-001 has 2 implements",
			entityID: "DEC-001",
			relType:  "implements",
			want:     2,
		},
		{
			name:     "CMP-001 has 1 uses",
			entityID: "CMP-001",
			relType:  "uses",
			want:     1,
		},
		{
			name:     "CMP-002 has no outgoing uses",
			entityID: "CMP-002",
			relType:  "uses",
			want:     0,
		},
		{
			name:     "REQ-001 has no outgoing implements",
			entityID: "REQ-001",
			relType:  "implements",
			want:     0,
		},
		{
			name:     "nonexistent relation type",
			entityID: "DEC-001",
			relType:  "nonexistent",
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countOutgoingByType(tt.entityID, tt.relType)
			if got != tt.want {
				t.Errorf("countOutgoingByType(%q, %q) = %d, want %d",
					tt.entityID, tt.relType, got, tt.want)
			}
		})
	}
}

func TestCountIncomingByType(t *testing.T) {
	setupAnalyzeTestGraph()

	tests := []struct {
		name     string
		entityID string
		relType  string
		want     int
	}{
		{
			name:     "REQ-001 has 1 incoming implements",
			entityID: "REQ-001",
			relType:  "implements",
			want:     1,
		},
		{
			name:     "REQ-002 has 1 incoming implements",
			entityID: "REQ-002",
			relType:  "implements",
			want:     1,
		},
		{
			name:     "DEC-001 has no incoming implements",
			entityID: "DEC-001",
			relType:  "implements",
			want:     0,
		},
		{
			name:     "CMP-002 has 1 incoming uses",
			entityID: "CMP-002",
			relType:  "uses",
			want:     1,
		},
		{
			name:     "CMP-001 has no incoming uses",
			entityID: "CMP-001",
			relType:  "uses",
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countIncomingByType(tt.entityID, tt.relType)
			if got != tt.want {
				t.Errorf("countIncomingByType(%q, %q) = %d, want %d",
					tt.entityID, tt.relType, got, tt.want)
			}
		})
	}
}

func TestCountMinOutgoingViolations(t *testing.T) {
	setupAnalyzeTestGraph()

	one := 1
	two := 2
	three := 3

	tests := []struct {
		name    string
		relName string
		relDef  metamodel.RelationDef
		want    int
	}{
		{
			name:    "no MinOutgoing constraint",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinOutgoing: nil,
			},
			want: 0,
		},
		{
			name:    "MinOutgoing 0 is ignored",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinOutgoing: new(int), // 0
			},
			want: 0,
		},
		{
			name:    "MinOutgoing 1, DEC-001 has 2 - no violation",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinOutgoing: &one,
			},
			want: 0,
		},
		{
			name:    "MinOutgoing 2, DEC-001 has 2 - no violation",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinOutgoing: &two,
			},
			want: 0,
		},
		{
			name:    "MinOutgoing 3, DEC-001 has 2 - 1 violation",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinOutgoing: &three,
			},
			want: 1,
		},
		{
			name:    "MinOutgoing 1 for uses, CMP-001 has 1, CMP-002 has 0 - 1 violation",
			relName: "uses",
			relDef: metamodel.RelationDef{
				From:        []string{"component"},
				To:          []string{"component"},
				MinOutgoing: &one,
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countMinOutgoingViolations(tt.relName, tt.relDef)
			if got != tt.want {
				t.Errorf("countMinOutgoingViolations(%q, ...) = %d, want %d",
					tt.relName, got, tt.want)
			}
		})
	}
}

func TestCountMaxOutgoingViolations(t *testing.T) {
	setupAnalyzeTestGraph()

	one := 1
	two := 2
	three := 3

	tests := []struct {
		name    string
		relName string
		relDef  metamodel.RelationDef
		want    int
	}{
		{
			name:    "no MaxOutgoing constraint",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxOutgoing: nil,
			},
			want: 0,
		},
		{
			name:    "MaxOutgoing 3, DEC-001 has 2 - no violation",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxOutgoing: &three,
			},
			want: 0,
		},
		{
			name:    "MaxOutgoing 2, DEC-001 has 2 - no violation",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxOutgoing: &two,
			},
			want: 0,
		},
		{
			name:    "MaxOutgoing 1, DEC-001 has 2 - 1 violation",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxOutgoing: &one,
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countMaxOutgoingViolations(tt.relName, tt.relDef)
			if got != tt.want {
				t.Errorf("countMaxOutgoingViolations(%q, ...) = %d, want %d",
					tt.relName, got, tt.want)
			}
		})
	}
}

func TestCountMinIncomingViolations(t *testing.T) {
	setupAnalyzeTestGraph()

	one := 1
	two := 2

	tests := []struct {
		name    string
		relName string
		relDef  metamodel.RelationDef
		want    int
	}{
		{
			name:    "no MinIncoming constraint",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinIncoming: nil,
			},
			want: 0,
		},
		{
			name:    "MinIncoming 0 is ignored",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinIncoming: new(int), // 0
			},
			want: 0,
		},
		{
			name:    "MinIncoming 1, REQ-001 and REQ-002 each have 1 - no violations",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinIncoming: &one,
			},
			want: 0,
		},
		{
			name:    "MinIncoming 2, REQ-001 and REQ-002 each have 1 - 2 violations",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MinIncoming: &two,
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countMinIncomingViolations(tt.relName, tt.relDef)
			if got != tt.want {
				t.Errorf("countMinIncomingViolations(%q, ...) = %d, want %d",
					tt.relName, got, tt.want)
			}
		})
	}
}

func TestCountMaxIncomingViolations(t *testing.T) {
	setupAnalyzeTestGraph()

	zero := 0
	one := 1
	two := 2

	tests := []struct {
		name    string
		relName string
		relDef  metamodel.RelationDef
		want    int
	}{
		{
			name:    "no MaxIncoming constraint",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxIncoming: nil,
			},
			want: 0,
		},
		{
			name:    "MaxIncoming 2, REQ-001 and REQ-002 each have 1 - no violations",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxIncoming: &two,
			},
			want: 0,
		},
		{
			name:    "MaxIncoming 1, REQ-001 and REQ-002 each have 1 - no violations",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxIncoming: &one,
			},
			want: 0,
		},
		{
			name:    "MaxIncoming 0, REQ-001 and REQ-002 each have 1 - 2 violations",
			relName: "implements",
			relDef: metamodel.RelationDef{
				From:        []string{"decision"},
				To:          []string{"requirement"},
				MaxIncoming: &zero,
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countMaxIncomingViolations(tt.relName, tt.relDef)
			if got != tt.want {
				t.Errorf("countMaxIncomingViolations(%q, ...) = %d, want %d",
					tt.relName, got, tt.want)
			}
		})
	}
}

func TestCountCardinalityViolations(t *testing.T) {
	one := 1
	three := 3

	tests := []struct {
		name      string
		setup     func()
		relations map[string]metamodel.RelationDef
		want      int
	}{
		{
			name: "no constraints - no violations",
			setup: func() {
				setupAnalyzeTestGraph()
			},
			relations: map[string]metamodel.RelationDef{
				"implements": {
					From: []string{"decision"},
					To:   []string{"requirement"},
				},
			},
			want: 0,
		},
		{
			name: "MinOutgoing satisfied - no violations",
			setup: func() {
				setupAnalyzeTestGraph()
			},
			relations: map[string]metamodel.RelationDef{
				"implements": {
					From:        []string{"decision"},
					To:          []string{"requirement"},
					MinOutgoing: &one,
				},
			},
			want: 0,
		},
		{
			name: "MinOutgoing not satisfied - violations",
			setup: func() {
				setupAnalyzeTestGraph()
			},
			relations: map[string]metamodel.RelationDef{
				"implements": {
					From:        []string{"decision"},
					To:          []string{"requirement"},
					MinOutgoing: &three,
				},
			},
			want: 1,
		},
		{
			name: "MaxOutgoing exceeded - violations",
			setup: func() {
				setupAnalyzeTestGraph()
			},
			relations: map[string]metamodel.RelationDef{
				"implements": {
					From:        []string{"decision"},
					To:          []string{"requirement"},
					MaxOutgoing: &one,
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			meta.Relations = tt.relations

			got := countCardinalityViolations()
			if got != tt.want {
				t.Errorf("countCardinalityViolations() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountPropertyErrors(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		wantZero bool
	}{
		{
			name: "valid entities - no errors",
			setup: func() {
				g = graph.New()
				meta = &metamodel.Metamodel{
					Entities: map[string]metamodel.EntityDef{
						"requirement": {
							Label:    "Requirement",
							IDPrefix: "REQ-",
							Properties: map[string]metamodel.PropertyDef{
								"title": {Type: "string"},
							},
						},
					},
				}
				out = output.New(output.FormatTable)

				entity := model.NewEntity("REQ-001", "requirement")
				entity.Properties["title"] = "Valid Title"
				g.AddNode(entity)
			},
			wantZero: true,
		},
		{
			name: "empty graph - no errors",
			setup: func() {
				g = graph.New()
				meta = &metamodel.Metamodel{
					Entities: map[string]metamodel.EntityDef{},
				}
				out = output.New(output.FormatTable)
			},
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got := countPropertyErrors()
			if tt.wantZero && got != 0 {
				t.Errorf("countPropertyErrors() = %d, want 0", got)
			}
		})
	}
}

// setupJSONTestOutput sets up JSON output writer and returns the buffer
func setupJSONTestOutput() *bytes.Buffer {
	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)
	return &buf
}

func TestAnalyzeOrphansJSONOutput(t *testing.T) {
	setupAnalyzeTestGraph()
	buf := setupJSONTestOutput()

	// Remove all edges to create orphans
	g = graph.New()
	orphan := model.NewEntity("REQ-003", "requirement")
	orphan.Properties["title"] = "Orphan Requirement"
	g.AddNode(orphan)

	err := analyzeOrphansCmd.RunE(nil, nil)
	if err != nil {
		t.Fatalf("analyzeOrphansCmd.RunE() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "warning" {
		t.Errorf("Expected status 'warning', got %q", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
}

func TestAnalyzeOrphansJSONOutputEmpty(t *testing.T) {
	setupAnalyzeTestGraph()
	buf := setupJSONTestOutput()

	err := analyzeOrphansCmd.RunE(nil, nil)
	if err != nil {
		t.Fatalf("analyzeOrphansCmd.RunE() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("Expected status 'success', got %q", result.Status)
	}
	if result.Count != 0 {
		t.Errorf("Expected count 0, got %d", result.Count)
	}
}

func TestAnalyzeDuplicatesJSONOutput(t *testing.T) {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement", IDPrefix: "REQ-"},
		},
	}
	buf := setupJSONTestOutput()

	// Create entities with duplicate titles
	e1 := model.NewEntity("REQ-001", "requirement")
	e1.Properties["title"] = "Same Title"
	g.AddNode(e1)

	e2 := model.NewEntity("REQ-002", "requirement")
	e2.Properties["title"] = "Same Title"
	g.AddNode(e2)

	err := analyzeDuplicatesCmd.RunE(nil, nil)
	if err != nil {
		t.Fatalf("analyzeDuplicatesCmd.RunE() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "warning" {
		t.Errorf("Expected status 'warning', got %q", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
}

func TestAnalyzeGapsJSONOutput(t *testing.T) {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement", IDPrefix: "REQ-"},
		},
	}
	buf := setupJSONTestOutput()

	// Create entities with a gap in IDs
	e1 := model.NewEntity("REQ-001", "requirement")
	g.AddNode(e1)
	e3 := model.NewEntity("REQ-003", "requirement")
	g.AddNode(e3)

	err := analyzeGapsCmd.RunE(nil, nil)
	if err != nil {
		t.Fatalf("analyzeGapsCmd.RunE() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "warning" {
		t.Errorf("Expected status 'warning', got %q", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
}

func TestAnalyzeCardinalityJSONOutput(t *testing.T) {
	setupAnalyzeTestGraph()
	buf := setupJSONTestOutput()

	// Add cardinality constraint that will be violated
	minTwo := 2
	meta.Relations["implements"] = metamodel.RelationDef{
		Label:       "Implements",
		From:        []string{"decision"},
		To:          []string{"requirement"},
		MinOutgoing: &minTwo,
	}

	// DEC-001 has 2 implements relations, so no violation with MinOutgoing=2
	// But let's test with 3 to cause a violation
	minThree := 3
	meta.Relations["implements"] = metamodel.RelationDef{
		Label:       "Implements",
		From:        []string{"decision"},
		To:          []string{"requirement"},
		MinOutgoing: &minThree,
	}

	err := analyzeCardinalityCmd.RunE(nil, nil)
	if err != nil {
		t.Fatalf("analyzeCardinalityCmd.RunE() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "warning" {
		t.Errorf("Expected status 'warning', got %q", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
}

func TestAnalyzeValidationsJSONOutput(t *testing.T) {
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
	buf := setupJSONTestOutput()

	// Create entity that violates the rule
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["status"] = "accepted"
	// No priority set - will violate the rule
	g.AddNode(e)

	err := runValidations()
	if err != nil {
		t.Fatalf("runValidations() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result.Status != "error" {
		t.Errorf("Expected status 'error', got %q", result.Status)
	}
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
}

func TestAnalyzeAllJSONOutput(t *testing.T) {
	setupAnalyzeTestGraph()
	buf := setupJSONTestOutput()

	err := analyzeAllCmd.RunE(nil, nil)
	if err != nil {
		t.Fatalf("analyzeAllCmd.RunE() error = %v", err)
	}

	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// With the test graph having no violations, should be success
	if result.Status != "success" {
		t.Errorf("Expected status 'success', got %q", result.Status)
	}
}
