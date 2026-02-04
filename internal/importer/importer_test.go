package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// testMetamodel creates a test metamodel
func testMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"status": {
						Type:   "status",
						Values: []string{"draft", "approved", "rejected"},
					},
					"priority": {
						Type:   "enum",
						Values: []string{"low", "medium", "high"},
					},
				},
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"status": {
						Type:   "status",
						Values: []string{"draft", "accepted", "rejected"},
					},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"addresses": {
				Label: "Addresses",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}
}

// setupTestProject creates a temporary test project and returns a repository and import source
func setupTestProject(t *testing.T) (
	repo *repository.Repository, src *ImportSource, ctx *project.Context, cleanup func(),
) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "rela-import-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ctx = &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
		CachePath:    filepath.Join(tmpDir, ".rela", "cache.json"),
	}

	// Create directories
	_ = os.MkdirAll(ctx.EntitiesDir, 0755)
	_ = os.MkdirAll(ctx.RelationsDir, 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".rela"), 0755)

	fs := storage.NewOsFS()
	repo = repository.New(fs, ctx)
	src = NewImportSource(fs)

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return repo, src, ctx, cleanup
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEnts int
		wantRels int
		wantErr  bool
	}{
		{
			name: "full format",
			input: `{
				"entities": [
					{"id": "REQ-001", "type": "requirement", "properties": {"title": "Test"}}
				],
				"relations": [
					{"from": "DEC-001", "relation": "addresses", "to": "REQ-001"}
				]
			}`,
			wantEnts: 1,
			wantRels: 1,
		},
		{
			name: "array format",
			input: `[
				{"id": "REQ-001", "type": "requirement", "properties": {"title": "Test 1"}},
				{"id": "REQ-002", "type": "requirement", "properties": {"title": "Test 2"}}
			]`,
			wantEnts: 2,
			wantRels: 0,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   `"just a string"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parseJSON(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(data.Entities) != tt.wantEnts {
				t.Errorf("parseJSON() entities = %d, want %d", len(data.Entities), tt.wantEnts)
			}
			if len(data.Relations) != tt.wantRels {
				t.Errorf("parseJSON() relations = %d, want %d", len(data.Relations), tt.wantRels)
			}
		})
	}
}

func TestParseYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEnts int
		wantRels int
		wantErr  bool
	}{
		{
			name: "full format",
			input: `
entities:
  - id: REQ-001
    type: requirement
    properties:
      title: Test
relations:
  - from: DEC-001
    relation: addresses
    to: REQ-001
`,
			wantEnts: 1,
			wantRels: 1,
		},
		{
			name: "array format",
			input: `
- id: REQ-001
  type: requirement
  properties:
    title: Test 1
- id: REQ-002
  type: requirement
  properties:
    title: Test 2
`,
			wantEnts: 2,
			wantRels: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parseYAML(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(data.Entities) != tt.wantEnts {
				t.Errorf("parseYAML() entities = %d, want %d", len(data.Entities), tt.wantEnts)
			}
			if len(data.Relations) != tt.wantRels {
				t.Errorf("parseYAML() relations = %d, want %d", len(data.Relations), tt.wantRels)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEnts int
		wantErr  bool
	}{
		{
			name: "basic csv",
			input: `id,type,title,status
REQ-001,requirement,Test requirement,draft
REQ-002,requirement,Another requirement,approved
`,
			wantEnts: 2,
		},
		{
			name:    "missing id column",
			input:   "type,title\nrequirement,Test\n",
			wantErr: true,
		},
		{
			name:    "missing type column",
			input:   "id,title\nREQ-001,Test\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parseCSV(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCSV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(data.Entities) != tt.wantEnts {
				t.Errorf("parseCSV() entities = %d, want %d", len(data.Entities), tt.wantEnts)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path string
		want Format
	}{
		{"data.json", FormatJSON},
		{"data.JSON", FormatJSON},
		{"data.yaml", FormatYAML},
		{"data.yml", FormatYAML},
		{"data.csv", FormatCSV},
		{"data.txt", ""},
		{"data", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectFormat(tt.path)
			if got != tt.want {
				t.Errorf("detectFormat(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestImportDryRun(t *testing.T) {
	repo, src, ctx, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	imp := New(repo, meta, g, Options{DryRun: true}, src)

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Test"}},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 1 {
		t.Errorf("EntitiesCreated = %d, want 1", result.EntitiesCreated)
	}

	// Check that no files were created
	files, _ := filepath.Glob(filepath.Join(ctx.EntitiesDir, "*", "*.md"))
	if len(files) > 0 {
		t.Errorf("Expected no files in dry run, found %d", len(files))
	}

	// Check that graph is empty
	if g.NodeCount() != 0 {
		t.Errorf("Expected empty graph in dry run, found %d nodes", g.NodeCount())
	}
}

func TestImportEntities(t *testing.T) {
	repo, src, _, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	imp := New(repo, meta, g, Options{}, src)

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "First requirement", "status": "draft"}},
			{ID: "REQ-002", Type: "requirement", Properties: map[string]interface{}{"title": "Second requirement", "status": "accepted"}},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 2 {
		t.Errorf("EntitiesCreated = %d, want 2", result.EntitiesCreated)
	}

	// Check graph
	if g.NodeCount() != 2 {
		t.Errorf("Graph nodes = %d, want 2", g.NodeCount())
	}

	// Check files were created
	node1, ok := g.GetNode("REQ-001")
	if !ok {
		t.Error("REQ-001 not found in graph")
	} else if node1.Title() != "First requirement" {
		t.Errorf("REQ-001 title = %q, want %q", node1.Title(), "First requirement")
	}
}

func TestImportWithRelations(t *testing.T) {
	repo, src, _, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	imp := New(repo, meta, g, Options{}, src)

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Requirement"}},
			{ID: "DEC-001", Type: "decision", Properties: map[string]interface{}{"title": "Decision"}},
		},
		Relations: []RelationData{
			{From: "DEC-001", Relation: "addresses", To: "REQ-001"},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 2 {
		t.Errorf("EntitiesCreated = %d, want 2", result.EntitiesCreated)
	}
	if result.RelationsCreated != 1 {
		t.Errorf("RelationsCreated = %d, want 1", result.RelationsCreated)
	}

	// Check edge exists
	if _, ok := g.GetEdge("DEC-001", "addresses", "REQ-001"); !ok {
		t.Error("Relation DEC-001 --addresses--> REQ-001 not found in graph")
	}
}

func TestImportValidationErrors(t *testing.T) {
	repo, src, _, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	tests := []struct {
		name    string
		data    *ImportData
		wantErr string
	}{
		{
			name: "missing id",
			data: &ImportData{
				Entities: []EntityData{{Type: "requirement", Properties: map[string]interface{}{"title": "Test"}}},
			},
			wantErr: "missing required field: id",
		},
		{
			name: "missing type",
			data: &ImportData{
				Entities: []EntityData{{ID: "REQ-001", Properties: map[string]interface{}{"title": "Test"}}},
			},
			wantErr: "missing required field: type",
		},
		{
			name: "unknown type",
			data: &ImportData{
				Entities: []EntityData{{ID: "FOO-001", Type: "unknown", Properties: map[string]interface{}{"title": "Test"}}},
			},
			wantErr: "unknown entity type",
		},
		{
			name: "missing required property",
			data: &ImportData{
				Entities: []EntityData{{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{}}},
			},
			wantErr: "This field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := New(repo, meta, g, Options{}, src)
			_, err := imp.Import(tt.data)
			if err == nil {
				t.Error("Expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestImportUpdate(t *testing.T) {
	repo, src, _, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	// First import
	imp := New(repo, meta, g, Options{}, src)
	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Original"}},
		},
	}
	_, err := imp.Import(data)
	if err != nil {
		t.Fatalf("First import error: %v", err)
	}

	// Second import without update - should fail
	imp2 := New(repo, meta, g, Options{}, src)
	data2 := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Updated"}},
		},
	}
	_, err = imp2.Import(data2)
	if err == nil {
		t.Error("Expected error for duplicate without --update")
	}

	// Third import with update - should succeed
	imp3 := New(repo, meta, g, Options{Update: true}, src)
	result, err := imp3.Import(data2)
	if err != nil {
		t.Fatalf("Update import error: %v", err)
	}
	if result.EntitiesUpdated != 1 {
		t.Errorf("EntitiesUpdated = %d, want 1", result.EntitiesUpdated)
	}

	// Check title was updated
	node, _ := g.GetNode("REQ-001")
	if node.Title() != "Updated" {
		t.Errorf("Title = %q, want %q", node.Title(), "Updated")
	}
}

func TestImportSkipErrors(t *testing.T) {
	repo, src, _, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	imp := New(repo, meta, g, Options{SkipErrors: true}, src)

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Valid"}},
			{ID: "BAD-001", Type: "unknown", Properties: map[string]interface{}{"title": "Invalid type"}},
			{ID: "REQ-002", Type: "requirement", Properties: map[string]interface{}{"title": "Also valid"}},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 2 {
		t.Errorf("EntitiesCreated = %d, want 2", result.EntitiesCreated)
	}
	if result.EntitiesSkipped != 1 {
		t.Errorf("EntitiesSkipped = %d, want 1", result.EntitiesSkipped)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors = %d, want 1", len(result.Errors))
	}
}

func TestImportFile(t *testing.T) {
	repo, src, ctx, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	// Create a test JSON file
	jsonFile := filepath.Join(ctx.Root, "test.json")
	content := `{
		"entities": [
			{"id": "REQ-001", "type": "requirement", "properties": {"title": "From file"}}
		]
	}`
	if err := os.WriteFile(jsonFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	imp := New(repo, meta, g, Options{}, src)
	result, err := imp.ImportFile(jsonFile)
	if err != nil {
		t.Fatalf("ImportFile() error = %v", err)
	}

	if result.EntitiesCreated != 1 {
		t.Errorf("EntitiesCreated = %d, want 1", result.EntitiesCreated)
	}
}

func TestImportDefaultStatus(t *testing.T) {
	repo, src, _, cleanup := setupTestProject(t)
	defer cleanup()

	meta := testMetamodel()
	g := graph.New()

	imp := New(repo, meta, g, Options{}, src)

	// Import entity without status - should get default
	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "No status"}},
		},
	}

	_, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	node, _ := g.GetNode("REQ-001")
	status := node.GetString("status")
	if status != "draft" {
		t.Errorf("Status = %q, want %q", status, "draft")
	}
}
