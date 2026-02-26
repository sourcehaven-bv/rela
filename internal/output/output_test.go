package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// TestNew tests creating a new writer
func TestNew(t *testing.T) {
	w := New(FormatTable)
	if w.Format != FormatTable {
		t.Errorf("expected FormatTable, got %v", w.Format)
	}
	if w.Out == nil {
		t.Error("expected Out to be set")
	}
}

// TestNewWithWriter tests creating a writer with custom io.Writer
func TestNewWithWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)
	if w.Format != FormatJSON {
		t.Errorf("expected FormatJSON, got %v", w.Format)
	}
	if w.Out != buf {
		t.Error("expected Out to be the provided buffer")
	}
	if !w.NoColor {
		t.Error("expected NoColor to be true for custom writers")
	}
}

// TestWriteEntities tests writing entities in table format
func TestWriteEntities(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	entities := []*model.Entity{
		{
			ID:   "REQ-001",
			Type: "requirement",
			Properties: map[string]interface{}{
				"title":  "Test Requirement",
				"status": "accepted",
			},
		},
		{
			ID:   "REQ-002",
			Type: "requirement",
			Properties: map[string]interface{}{
				"title":  "Another Requirement",
				"status": "draft",
			},
		},
	}

	err := w.WriteEntities(entities)
	if err != nil {
		t.Fatalf("WriteEntities failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "REQ-001") {
		t.Error("expected output to contain REQ-001")
	}
	if !strings.Contains(output, "Test Requirement") {
		t.Error("expected output to contain Test Requirement")
	}
}

// TestWriteEntitiesJSON tests writing entities in JSON format
func TestWriteEntitiesJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	entities := []*model.Entity{
		{
			ID:   "REQ-001",
			Type: "requirement",
			Properties: map[string]interface{}{
				"title": "Test",
			},
		},
	}

	err := w.WriteEntities(entities)
	if err != nil {
		t.Fatalf("WriteEntities failed: %v", err)
	}

	// Parse JSON to verify it's valid
	var result []*model.Entity
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 entity in JSON, got %d", len(result))
	}
}

// TestWriteEntitiesWithSummary tests writing entities with summary footer
func TestWriteEntitiesWithSummary(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	entities := []*model.Entity{
		{
			ID:   "REQ-001",
			Type: "requirement",
			Properties: map[string]interface{}{
				"title":  "Test",
				"status": "accepted",
			},
		},
		{
			ID:   "REQ-002",
			Type: "requirement",
			Properties: map[string]interface{}{
				"title":  "Test 2",
				"status": "draft",
			},
		},
	}

	err := w.WriteEntitiesWithSummary(entities)
	if err != nil {
		t.Fatalf("WriteEntitiesWithSummary failed: %v", err)
	}

	output := buf.String()
	// Should contain summary line
	if !strings.Contains(output, "2 entities") {
		t.Error("expected output to contain '2 entities' summary")
	}
	if !strings.Contains(output, "accepted") || !strings.Contains(output, "draft") {
		t.Error("expected output to contain status breakdown")
	}

	// Test with JSON format
	bufJSON := &bytes.Buffer{}
	wJSON := NewWithWriter(bufJSON, FormatJSON)
	err = wJSON.WriteEntitiesWithSummary(entities)
	if err != nil {
		t.Fatalf("WriteEntitiesWithSummary JSON failed: %v", err)
	}

	// Test with empty entities
	bufEmpty := &bytes.Buffer{}
	wEmpty := NewWithWriter(bufEmpty, FormatTable)
	err = wEmpty.WriteEntitiesWithSummary([]*model.Entity{})
	if err != nil {
		t.Fatalf("WriteEntitiesWithSummary empty failed: %v", err)
	}
}

// TestBuildEntitySummary tests summary building logic
func TestBuildEntitySummary(t *testing.T) {
	w := New(FormatTable)

	tests := []struct {
		name         string
		total        int
		statusCounts map[string]int
		expected     string
	}{
		{
			name:         "single entity",
			total:        1,
			statusCounts: map[string]int{"accepted": 1},
			expected:     "1 entity (1 accepted)",
		},
		{
			name:         "multiple entities",
			total:        3,
			statusCounts: map[string]int{"accepted": 2, "draft": 1},
			expected:     "3 entities (2 accepted, 1 draft)",
		},
		{
			name:         "no status",
			total:        1,
			statusCounts: map[string]int{},
			expected:     "1 entity",
		},
		{
			name:         "custom status",
			total:        2,
			statusCounts: map[string]int{"custom": 2},
			expected:     "2 entities (2 custom)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := w.buildEntitySummary(tt.total, tt.statusCounts)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestWriteEntity tests writing a single entity with details
func TestWriteEntity(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	entity := &model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":       "Test Requirement",
			"status":      "accepted",
			"priority":    "high",
			"description": "Test description",
			"custom":      "custom value",
		},
		Content: "Some content here",
	}

	incoming := []*model.Relation{
		{From: "DEC-001", Type: "implements", To: "REQ-001"},
	}

	outgoing := []*model.Relation{
		{From: "REQ-001", Type: "depends_on", To: "REQ-002"},
	}

	err := w.WriteEntity(entity, incoming, outgoing)
	if err != nil {
		t.Fatalf("WriteEntity failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "REQ-001") {
		t.Error("expected output to contain entity ID")
	}
	if !strings.Contains(output, "Test Requirement") {
		t.Error("expected output to contain title")
	}
	if !strings.Contains(output, "Some content here") {
		t.Error("expected output to contain content")
	}
	if !strings.Contains(output, "DEC-001") {
		t.Error("expected output to contain incoming relation")
	}
	if !strings.Contains(output, "REQ-002") {
		t.Error("expected output to contain outgoing relation")
	}
	if !strings.Contains(output, "custom") {
		t.Error("expected output to contain custom property")
	}

	// Test entity with minimal fields
	buf2 := &bytes.Buffer{}
	w2 := NewWithWriter(buf2, FormatTable)
	entity2 := &model.Entity{
		ID:         "REQ-002",
		Type:       "requirement",
		Properties: map[string]interface{}{},
	}
	err = w2.WriteEntity(entity2, nil, nil)
	if err != nil {
		t.Fatalf("WriteEntity minimal failed: %v", err)
	}
	output2 := buf2.String()
	if !strings.Contains(output2, "REQ-002") {
		t.Error("expected minimal output to contain entity ID")
	}
}

// TestWriteEntityJSON tests writing entity in JSON format
func TestWriteEntityJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	entity := &model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
	}

	err := w.WriteEntity(entity, nil, nil)
	if err != nil {
		t.Fatalf("WriteEntity failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result["entity"] == nil {
		t.Error("expected JSON to contain 'entity' key")
	}
}

// TestWriteRelations tests writing relations
func TestWriteRelations(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	relations := []*model.Relation{
		{From: "REQ-001", Type: "implements", To: "DEC-001"},
		{From: "REQ-002", Type: "depends_on", To: "REQ-001"},
	}

	err := w.WriteRelations(relations)
	if err != nil {
		t.Fatalf("WriteRelations failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "REQ-001") {
		t.Error("expected output to contain REQ-001")
	}
	if !strings.Contains(output, "implements") {
		t.Error("expected output to contain relation type")
	}
}

// TestWriteRelationsJSON tests writing relations in JSON format
func TestWriteRelationsJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	relations := []*model.Relation{
		{From: "REQ-001", Type: "implements", To: "DEC-001"},
	}

	err := w.WriteRelations(relations)
	if err != nil {
		t.Fatalf("WriteRelations failed: %v", err)
	}

	var result []*model.Relation
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 relation in JSON, got %d", len(result))
	}
}

// TestWriteTrace tests writing trace results
func TestWriteTrace(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	trace := &model.TraceResult{
		ID:    "REQ-001",
		Title: "Root",
		Children: []*model.TraceResult{
			{
				ID:       "REQ-002",
				Title:    "Child 1",
				Relation: "depends_on",
				Children: []*model.TraceResult{
					{
						ID:       "REQ-004",
						Title:    "Grandchild",
						Relation: "implements",
					},
				},
			},
			{
				ID:       "REQ-003",
				Title:    "Child 2",
				Relation: "depends_on",
			},
		},
	}

	err := w.WriteTrace(trace)
	if err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "REQ-001") {
		t.Error("expected output to contain root node")
	}
	if !strings.Contains(output, "REQ-002") {
		t.Error("expected output to contain child node")
	}
	if !strings.Contains(output, "REQ-004") {
		t.Error("expected output to contain grandchild node")
	}
	// The connectors are only shown when there's nesting
	if !strings.Contains(output, "[depends_on]") {
		t.Error("expected output to contain relation info")
	}

	// Test trace without relation info
	buf2 := &bytes.Buffer{}
	w2 := NewWithWriter(buf2, FormatTable)
	trace2 := &model.TraceResult{
		ID:    "REQ-005",
		Title: "No relations",
	}
	err = w2.WriteTrace(trace2)
	if err != nil {
		t.Fatalf("WriteTrace no relations failed: %v", err)
	}
	output2 := buf2.String()
	if !strings.Contains(output2, "REQ-005") {
		t.Error("expected output to contain node without relations")
	}
}

// TestWriteTraceJSON tests writing trace in JSON format
func TestWriteTraceJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	trace := &model.TraceResult{
		ID:    "REQ-001",
		Title: "Root",
	}

	err := w.WriteTrace(trace)
	if err != nil {
		t.Fatalf("WriteTrace failed: %v", err)
	}

	var result model.TraceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result.ID != "REQ-001" {
		t.Errorf("expected ID REQ-001, got %s", result.ID)
	}
}

// TestWritePath tests writing path results
func TestWritePath(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	path := []model.PathStep{
		{ID: "REQ-001", Type: "requirement"},
		{ID: "REQ-002", Type: "requirement", Relation: "depends_on"},
		{ID: "DEC-001", Type: "decision", Relation: "implements"},
	}

	err := w.WritePath(path)
	if err != nil {
		t.Fatalf("WritePath failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "REQ-001") {
		t.Error("expected output to contain start node")
	}
	if !strings.Contains(output, "DEC-001") {
		t.Error("expected output to contain end node")
	}
	if !strings.Contains(output, "2 hops") {
		t.Error("expected output to contain hop count")
	}

	// Test with color
	buf2 := &bytes.Buffer{}
	w2 := New(FormatTable)
	w2.Out = buf2
	w2.NoColor = false
	err = w2.WritePath(path)
	if err != nil {
		t.Fatalf("WritePath with color failed: %v", err)
	}
	output2 := buf2.String()
	if !strings.Contains(output2, "REQ-001") {
		t.Error("expected colored output to contain start node")
	}

	// Test with single hop
	path1 := []model.PathStep{
		{ID: "REQ-001", Type: "requirement"},
		{ID: "REQ-002", Type: "requirement", Relation: "depends_on"},
	}
	buf3 := &bytes.Buffer{}
	w3 := NewWithWriter(buf3, FormatTable)
	err = w3.WritePath(path1)
	if err != nil {
		t.Fatalf("WritePath single hop failed: %v", err)
	}
	output3 := buf3.String()
	if !strings.Contains(output3, "1 hop") {
		t.Error("expected output to contain '1 hop' (singular)")
	}
}

// TestWritePathEmpty tests writing empty path
func TestWritePathEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	err := w.WritePath([]model.PathStep{})
	if err != nil {
		t.Fatalf("WritePath failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No path found") {
		t.Error("expected output to indicate no path found")
	}
}

// TestWritePathJSON tests writing path in JSON format
func TestWritePathJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	path := []model.PathStep{
		{ID: "REQ-001", Type: "requirement"},
	}

	err := w.WritePath(path)
	if err != nil {
		t.Fatalf("WritePath failed: %v", err)
	}

	var result []model.PathStep
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 step in JSON, got %d", len(result))
	}
}

// TestWriteMessage tests message output
func TestWriteMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	w.WriteMessage("Test message: %s", "hello")

	output := buf.String()
	if !strings.Contains(output, "Test message: hello") {
		t.Errorf("expected message in output, got: %s", output)
	}
}

// TestWriteSuccess tests success message output
func TestWriteSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	w.WriteSuccess("Operation completed")

	output := buf.String()
	if !strings.Contains(output, "Operation completed") {
		t.Error("expected success message in output")
	}
}

// TestWriteError tests error message output
func TestWriteError(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	w.WriteError("Operation failed")

	output := buf.String()
	if !strings.Contains(output, "Operation failed") {
		t.Error("expected error message in output")
	}
}

// TestWriteWarning tests warning message output
func TestWriteWarning(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	w.WriteWarning("Warning message")

	output := buf.String()
	if !strings.Contains(output, "Warning message") {
		t.Error("expected warning message in output")
	}
}

// TestWriteInfo tests info message output
func TestWriteInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)

	w.WriteInfo("Info message")

	output := buf.String()
	if !strings.Contains(output, "Info message") {
		t.Error("expected info message in output")
	}
}

// TestColorizeStatus tests status colorization
func TestColorizeStatus(t *testing.T) {
	tests := []struct {
		status   string
		noColor  bool
		expected string
	}{
		{"accepted", true, "accepted"},
		{"draft", true, "draft"},
		{"proposed", true, "proposed"},
		{"deprecated", true, "deprecated"},
		{"rejected", true, "rejected"},
		{"retired", true, "retired"},
		{"unknown", true, "unknown"},
		// Test with colors enabled
		{"accepted", false, "accepted"}, // Will have color codes but string is still there
		{"draft", false, "draft"},
		{"proposed", false, "proposed"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := colorizeStatus(tt.status, tt.noColor)
			if tt.noColor {
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			} else {
				// When colors are enabled, just check the status text is in there
				if !strings.Contains(result, tt.status) {
					t.Errorf("expected result to contain %s, got %s", tt.status, result)
				}
			}
		})
	}
}

// TestFormatSize tests byte size formatting
func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0B"},
		{100, "100B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{1572864, "1.5MB"},
		{1073741824, "1.0GB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.size)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

// TestTruncate tests string truncation
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a very long string", 10, "this is..."},
		{"truncate me", 8, "trunc..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestWriteSectionHeader tests section header output
func TestWriteSectionHeader(t *testing.T) {
	// Test with no color
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)
	w.WriteSectionHeader("Test Section")
	output := buf.String()
	if !strings.Contains(output, "Test Section") {
		t.Error("expected section header to contain title")
	}
	if !strings.Contains(output, "─") {
		t.Error("expected section header to contain separator")
	}

	// Test with color
	buf2 := &bytes.Buffer{}
	w2 := New(FormatTable)
	w2.Out = buf2
	w2.NoColor = false
	w2.WriteSectionHeader("Test Section")
	output2 := buf2.String()
	if !strings.Contains(output2, "Test Section") {
		t.Error("expected section header to contain title with colors")
	}
}

// TestWriteSummaryBox tests summary box output
func TestWriteSummaryBox(t *testing.T) {
	// Test with no color
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)
	w.WriteSummaryBox([]string{"Item 1", "Item 2"})
	output := buf.String()
	if !strings.Contains(output, "Item 1") || !strings.Contains(output, "Item 2") {
		t.Error("expected summary box to contain items")
	}
	if !strings.Contains(output, "│") {
		t.Error("expected summary box to contain box drawing characters")
	}

	// Test with color
	buf2 := &bytes.Buffer{}
	w2 := New(FormatTable)
	w2.Out = buf2
	w2.NoColor = false
	w2.WriteSummaryBox([]string{"Item 1"})
	output2 := buf2.String()
	if !strings.Contains(output2, "Item 1") {
		t.Error("expected summary box to contain items with colors")
	}
}

// TestWriteBar tests bar chart generation
func TestWriteBar(t *testing.T) {
	w := New(FormatTable)
	w.NoColor = true

	tests := []struct {
		value    int
		maxValue int
		expected string
	}{
		{0, 10, ""},
		{5, 10, "████"},
		{10, 10, "████████"},
		{1, 100, "█"},
		{0, 0, ""},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := w.WriteBar(tt.value, tt.maxValue)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Test with color
	w2 := New(FormatTable)
	w2.NoColor = false
	result := w2.WriteBar(5, 10)
	if !strings.Contains(result, "████") {
		t.Error("expected bar to contain blocks with colors")
	}
}

// TestWriteSeparator tests separator output
func TestWriteSeparator(t *testing.T) {
	// Test with no color
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)
	w.WriteSeparator()
	output := buf.String()
	if !strings.Contains(output, "─") {
		t.Error("expected separator to contain line character")
	}

	// Test with color
	buf2 := &bytes.Buffer{}
	w2 := New(FormatTable)
	w2.Out = buf2
	w2.NoColor = false
	w2.WriteSeparator()
	output2 := buf2.String()
	if !strings.Contains(output2, "─") {
		t.Error("expected separator to contain line character with colors")
	}
}

// TestWriteFooterSummary tests footer summary output
func TestWriteFooterSummary(t *testing.T) {
	// Test with no color
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatTable)
	w.WriteFooterSummary("Summary text")
	output := buf.String()
	if !strings.Contains(output, "Summary text") {
		t.Error("expected footer summary to contain text")
	}

	// Test with color
	buf2 := &bytes.Buffer{}
	w2 := New(FormatTable)
	w2.Out = buf2
	w2.NoColor = false
	w2.WriteFooterSummary("Summary text")
	output2 := buf2.String()
	if !strings.Contains(output2, "Summary text") {
		t.Error("expected footer summary to contain text with colors")
	}
}

// TestWriteAnalysisResult tests analysis result output
func TestWriteAnalysisResult(t *testing.T) {
	tests := []struct {
		name   string
		result AnalysisResult
	}{
		{
			name: "success",
			result: AnalysisResult{
				Status:  "success",
				Message: "All checks passed",
			},
		},
		{
			name: "warning",
			result: AnalysisResult{
				Status:  "warning",
				Message: "Some warnings found",
			},
		},
		{
			name: "error",
			result: AnalysisResult{
				Status:  "error",
				Message: "Errors detected",
			},
		},
		{
			name: "other",
			result: AnalysisResult{
				Status:  "info",
				Message: "Information message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := NewWithWriter(buf, FormatTable)

			err := w.WriteAnalysisResult(tt.result)
			if err != nil {
				t.Fatalf("WriteAnalysisResult failed: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.result.Message) {
				t.Error("expected output to contain message")
			}
		})
	}
}

// TestWriteAnalysisResultJSON tests analysis result in JSON format
func TestWriteAnalysisResultJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	result := AnalysisResult{
		Status:  "success",
		Message: "Test",
		Count:   5,
	}

	err := w.WriteAnalysisResult(result)
	if err != nil {
		t.Fatalf("WriteAnalysisResult failed: %v", err)
	}

	var parsed AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if parsed.Count != 5 {
		t.Errorf("expected count 5, got %d", parsed.Count)
	}
}

// Mock types for schema testing
type mockMetamodel struct{}

func (m *mockMetamodel) GetVersion() string        { return "1.0" }
func (m *mockMetamodel) GetNamespace() string      { return "test" }
func (m *mockMetamodel) GetEntities() interface{}  { return map[string]interface{}{} }
func (m *mockMetamodel) GetRelations() interface{} { return map[string]interface{}{} }
func (m *mockMetamodel) GetTypes() interface{}     { return map[string]interface{}{} }

type mockEntityDef struct{}

func (e *mockEntityDef) GetLabel() string           { return "Test Entity" }
func (e *mockEntityDef) GetAliases() []string       { return []string{"test"} }
func (e *mockEntityDef) GetIDPatterns() []string    { return []string{"TEST-*"} }
func (e *mockEntityDef) GetProperties() interface{} { return map[string]interface{}{} }
func (e *mockEntityDef) GetRDFType() string         { return "test:Entity" }
func (e *mockEntityDef) GetColor() string           { return "#FF0000" }
func (e *mockEntityDef) GetBorderColor() string     { return "#000000" }

type mockRelationDef struct {
	symmetric bool
	desc      string
	inverse   interface{}
	srcMin    *int
	srcMax    *int
	tgtMin    *int
	tgtMax    *int
}

func (r *mockRelationDef) GetLabel() string  { return "Test Relation" }
func (r *mockRelationDef) GetFrom() []string { return []string{"Entity1"} }
func (r *mockRelationDef) GetTo() []string   { return []string{"Entity2"} }
func (r *mockRelationDef) GetDescription() string {
	if r.desc != "" {
		return r.desc
	}
	return ""
}
func (r *mockRelationDef) GetInverse() interface{} { return r.inverse }
func (r *mockRelationDef) IsSymmetric() bool       { return r.symmetric }
func (r *mockRelationDef) GetMinOutgoing() *int    { return r.srcMin }
func (r *mockRelationDef) GetMaxOutgoing() *int    { return r.srcMax }
func (r *mockRelationDef) GetMinIncoming() *int    { return r.tgtMin }
func (r *mockRelationDef) GetMaxIncoming() *int    { return r.tgtMax }

// TestWriteSchemaOverview tests schema overview output
func TestWriteSchemaOverview(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	mock := &mockMetamodel{}
	err := w.WriteSchemaOverview(mock)
	if err != nil {
		t.Fatalf("WriteSchemaOverview failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result["version"] != "1.0" {
		t.Error("expected version in output")
	}
}

// TestWriteSchemaEntities tests schema entities output
func TestWriteSchemaEntities(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	mock := &mockMetamodel{}
	err := w.WriteSchemaEntities(mock)
	if err != nil {
		t.Fatalf("WriteSchemaEntities failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// TestWriteSchemaRelations tests schema relations output
func TestWriteSchemaRelations(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	mock := &mockMetamodel{}
	err := w.WriteSchemaRelations(mock)
	if err != nil {
		t.Fatalf("WriteSchemaRelations failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// TestWriteSchemaTypes tests schema types output
func TestWriteSchemaTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	mock := &mockMetamodel{}
	err := w.WriteSchemaTypes(mock)
	if err != nil {
		t.Fatalf("WriteSchemaTypes failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// TestWriteSchemaEntityDetail tests entity detail output
func TestWriteSchemaEntityDetail(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	mockDef := &mockEntityDef{}
	mockMeta := &mockMetamodel{}
	err := w.WriteSchemaEntityDetail("test", mockDef, mockMeta)
	if err != nil {
		t.Fatalf("WriteSchemaEntityDetail failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result["name"] != "test" {
		t.Error("expected name in output")
	}
}

// TestWriteSchemaRelationDetail tests relation detail output
func TestWriteSchemaRelationDetail(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWithWriter(buf, FormatJSON)

	mockDef := &mockRelationDef{}
	err := w.WriteSchemaRelationDetail("test_relation", mockDef)
	if err != nil {
		t.Fatalf("WriteSchemaRelationDetail failed: %v", err)
	}

	var result map[string]interface{}
	if unmarshalErr := json.Unmarshal(buf.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("failed to parse JSON output: %v", unmarshalErr)
	}
	if result["name"] != "test_relation" {
		t.Error("expected name in output")
	}

	// Test with all optional fields
	buf2 := &bytes.Buffer{}
	w2 := NewWithWriter(buf2, FormatJSON)

	srcMin := 1
	srcMax := 5
	tgtMin := 2
	tgtMax := 10
	mockDef2 := &mockRelationDef{
		desc:      "Test description",
		inverse:   "test_inverse",
		symmetric: true,
		srcMin:    &srcMin,
		srcMax:    &srcMax,
		tgtMin:    &tgtMin,
		tgtMax:    &tgtMax,
	}
	err = w2.WriteSchemaRelationDetail("test_relation2", mockDef2)
	if err != nil {
		t.Fatalf("WriteSchemaRelationDetail with optional fields failed: %v", err)
	}

	var result2 map[string]interface{}
	if err := json.Unmarshal(buf2.Bytes(), &result2); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result2["description"] != "Test description" {
		t.Error("expected description in output")
	}
	if result2["symmetric"] != true {
		t.Error("expected symmetric in output")
	}
	if result2["min_outgoing"] != float64(1) {
		t.Error("expected min_outgoing in output")
	}
}
