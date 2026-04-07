package lua

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// testMeta returns the metamodel used for testing.
func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefix: "TKT",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
			},
			"feature": {
				IDPrefix: "FEAT",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				From: []string{"ticket"},
				To:   []string{"feature"},
			},
		},
	}
}

// mockWorkspace implements WorkspaceInterface for testing.
// This avoids importing the workspace package which would cause an import cycle.
type mockWorkspace struct {
	graph *graph.Graph
	meta  *metamodel.Metamodel
}

// newMockWorkspace creates a mock workspace with test entities.
func newMockWorkspace(t *testing.T) *mockWorkspace {
	t.Helper()

	g := graph.New()
	g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Test Ticket",
			"status": "open",
		},
		Content: "Test content",
	})
	g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Done Ticket",
			"status": "done",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "FEAT-001",
		Type: "feature",
		Properties: map[string]interface{}{
			"title": "Test Feature",
		},
	})
	g.AddEdge(&model.Relation{
		From: "TKT-001",
		Type: "implements",
		To:   "FEAT-001",
	})

	return &mockWorkspace{
		graph: g,
		meta:  testMeta(),
	}
}

// Entity queries
func (m *mockWorkspace) GetEntity(id string) (*model.Entity, bool) {
	return m.graph.GetNode(id)
}

func (m *mockWorkspace) EntitiesByType(entityType string) []*model.Entity {
	return m.graph.NodesByType(entityType)
}

// Entity mutations
func (m *mockWorkspace) CreateEntityLua(entityType, id string, props map[string]interface{}, content string) (*model.Entity, error) {
	// Generate a simple ID
	if id == "" {
		id = fmt.Sprintf("%s-%03d", strings.ToUpper(entityType[:3]), m.graph.NodeCount()+1)
	}
	entity := &model.Entity{
		ID:         id,
		Type:       entityType,
		Properties: props,
		Content:    content,
	}
	m.graph.AddNode(entity)
	return entity, nil
}

func (m *mockWorkspace) UpdateEntityLua(entity, _ *model.Entity) error {
	m.graph.AddNode(entity)
	return nil
}

func (m *mockWorkspace) DeleteEntityLua(_, id string, _ bool) error {
	if _, ok := m.graph.GetNode(id); !ok {
		return fmt.Errorf("entity not found: %s", id)
	}
	m.graph.RemoveNode(id)
	return nil
}

// Relation queries
func (m *mockWorkspace) AllRelations() []*model.Relation {
	return m.graph.AllEdges()
}

// Relation mutations
func (m *mockWorkspace) CreateRelationLua(from, relType, to string) (*model.Relation, error) {
	rel := model.NewRelation(from, relType, to)
	m.graph.AddEdge(rel)
	return rel, nil
}

func (m *mockWorkspace) DeleteRelation(from, relType, to string) error {
	m.graph.RemoveEdge(from, relType, to)
	return nil
}

// Graph operations
func (m *mockWorkspace) TraceFrom(id string, maxDepth int) *model.TraceResult {
	return m.graph.TraceFrom(id, maxDepth)
}

func (m *mockWorkspace) TraceTo(id string, maxDepth int) *model.TraceResult {
	return m.graph.TraceTo(id, maxDepth)
}

func (m *mockWorkspace) FindPath(from, to string) []model.PathStep {
	return m.graph.FindPath(from, to)
}

// Search
func (m *mockWorkspace) SearchSimple(query string, limit int) ([]*model.Entity, error) {
	// Simple search: return all entities that have the query in title
	var results []*model.Entity
	query = strings.ToLower(query)
	for _, e := range m.graph.AllNodes() {
		title := strings.ToLower(e.GetString("title"))
		if strings.Contains(title, query) {
			results = append(results, e)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Sync
func (m *mockWorkspace) SyncLua() error {
	return nil
}

// Meta returns the metamodel for the mock workspace.
func (m *mockWorkspace) Meta() *metamodel.Metamodel {
	return m.meta
}

// testWorkspace is an alias for newMockWorkspace for tests that use the older naming.
func testWorkspace(t *testing.T) *mockWorkspace {
	t.Helper()
	return newMockWorkspace(t)
}

// newMockWorkspaceWith creates a mock workspace with a custom graph and metamodel.
func newMockWorkspaceWith(g *graph.Graph, meta *metamodel.Metamodel) *mockWorkspace {
	return &mockWorkspace{
		graph: g,
		meta:  meta,
	}
}

// Graph returns the graph for the mock workspace.
func (m *mockWorkspace) Graph() *graph.Graph {
	return m.graph
}

func TestRunFile_BasicOutput(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	// Create a temp script
	script := `rela.output({status = "ok"})`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", result["status"])
	}
}

func TestRunFile_Args(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.output(rela.args)`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, []string{"foo", "bar"}); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(result) != 2 || result[0] != "foo" || result[1] != "bar" {
		t.Errorf("Expected [foo, bar], got %v", result)
	}
}

func TestGetEntity(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	// Get reference entity for assertions
	entity, _ := ws.GetEntity("TKT-001")

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local e = rela.get_entity("TKT-001")
rela.output({
    id = e.id,
    type = e.type,
    title = e.properties.title,
    content = e.content
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["id"] != entity.ID {
		t.Errorf("Expected id=%s, got %v", entity.ID, result["id"])
	}
	if result["type"] != entity.Type {
		t.Errorf("Expected type=%s, got %v", entity.Type, result["type"])
	}
	if result["title"] != entity.Properties["title"] {
		t.Errorf("Expected title=%v, got %v", entity.Properties["title"], result["title"])
	}
	if result["content"] != entity.Content {
		t.Errorf("Expected content=%s, got %v", entity.Content, result["content"])
	}
}

func TestGetEntity_NotFound(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local e = rela.get_entity("NONEXISTENT")
if e == nil then
    rela.output({found = false})
else
    rela.output({found = true})
end
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["found"] != false {
		t.Errorf("Expected found=false, got %v", result["found"])
	}
}

func TestListEntities(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local tickets = rela.list_entities("ticket")
rela.output({count = #tickets})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["count"] != float64(2) {
		t.Errorf("Expected count=2, got %v", result["count"])
	}
}

func TestListEntities_WithFilter(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local tickets = rela.list_entities("ticket", "status=done")
rela.output({count = #tickets})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["count"] != float64(1) {
		t.Errorf("Expected count=1, got %v", result["count"])
	}
}

func TestGetRelations(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	// Get reference relation for assertions
	rels := ws.Graph().RelationsOfType("implements")
	if len(rels) == 0 {
		t.Fatal("Expected at least one implements relation in test workspace")
	}
	testRel := rels[0]

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local rels = rela.get_relations({type = "implements"})
rela.output({
    count = #rels,
    first = rels[1]
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["count"] != float64(1) {
		t.Errorf("Expected count=1, got %v", result["count"])
	}

	first := result["first"].(map[string]interface{})
	if first["from"] != testRel.From {
		t.Errorf("Expected from=%s, got %v", testRel.From, first["from"])
	}
	if first["type"] != testRel.Type {
		t.Errorf("Expected type=%s, got %v", testRel.Type, first["type"])
	}
	if first["to"] != testRel.To {
		t.Errorf("Expected to=%s, got %v", testRel.To, first["to"])
	}
}

func TestWriteFile(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Files are written to output/ directory
	script := `rela.write_file("result.txt", "hello world")`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	// File should be in output/ subdirectory
	outFile := filepath.Join(projectRoot, "output", "result.txt")
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "hello world" {
		t.Errorf("Expected 'hello world', got %q", string(content))
	}
}

func TestScriptError_Syntax(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `print(`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for syntax error")
	}
}

func TestScriptError_Runtime(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `error("boom")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for runtime error")
	}
}

func TestProjectRoot(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/my/project", &buf)
	defer r.Close()

	script := `rela.output({root = rela.project_root})`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["root"] != "/my/project" {
		t.Errorf("Expected root=/my/project, got %v", result["root"])
	}
}

func TestWriteFile_PathTraversal(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Try to escape output/ using path traversal
	script := `rela.write_file("../outside.txt", "malicious")`
	err := r.RunString(script)
	if err == nil {
		t.Fatal("Expected error for path traversal attempt")
	}
	// Error message should indicate the path is not local
	if !strings.Contains(err.Error(), "local path") {
		t.Errorf("Expected 'local path' error, got: %v", err)
	}

	// Verify file was NOT created outside output/
	outsideFile := filepath.Join(projectRoot, "outside.txt")
	if _, err := os.Stat(outsideFile); err == nil {
		t.Error("File should not have been created outside output/")
	}
}

func TestWriteFile_AbsolutePathOutside(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Try to write to absolute path (should be rejected)
	script := `rela.write_file("/tmp/outside.txt", "malicious")`
	err := r.RunString(script)
	if err == nil {
		t.Fatal("Expected error for absolute path")
	}
}

func TestWriteFile_InOutputDir(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Write to output directory
	script := `rela.write_file("result.txt", "allowed")`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	// File should be in output/ subdirectory
	outFile := filepath.Join(projectRoot, "output", "result.txt")
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "allowed" {
		t.Errorf("Expected 'allowed', got %q", string(content))
	}
}

func TestListEntities_EmptyType(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.list_entities("")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for empty entity type")
	}
}

func TestListEntities_InvalidFilter(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.list_entities("ticket", "invalid[filter")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for invalid filter")
	}
}

func TestGetRelations_NoFilters(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local rels = rela.get_relations()
rela.output({count = #rels})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Should return all relations (we have 1 in test workspace)
	if result["count"] != float64(1) {
		t.Errorf("Expected count=1, got %v", result["count"])
	}
}

func TestTraceFrom_NonExistent(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local trace = rela.trace_from("NONEXISTENT", 0)
if trace == nil then
    rela.output({found = false})
else
    rela.output({found = true})
end
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// For non-existent entity, trace_from returns nil
	if result["found"] != false {
		t.Errorf("Expected found=false, got %v", result["found"])
	}
}

func TestSearch(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local results = rela.search("Test")
rela.output({count = #results})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Should find entities with "Test" in their title
	count := result["count"].(float64)
	if count < 1 {
		t.Errorf("Expected at least 1 result, got %v", count)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.search("")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for empty search query")
	}
}

func TestCreateEntity(t *testing.T) {
	ws, root := newMockWorkspace(t), t.TempDir()
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), root, &buf)
	defer r.Close()

	script := `
local entity = rela.create_entity("ticket", {title = "New Ticket", status = "open"}, "Content here")
rela.output({
    id = entity.id,
    type = entity.type,
    title = entity.properties.title,
    status = entity.properties.status
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["type"] != "ticket" {
		t.Errorf("Expected type=ticket, got %v", result["type"])
	}
	if result["title"] != "New Ticket" {
		t.Errorf("Expected title=New Ticket, got %v", result["title"])
	}
	if result["status"] != "open" {
		t.Errorf("Expected status=open, got %v", result["status"])
	}
}

func TestCreateEntity_EmptyType(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.create_entity("", {title = "Test"})`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for empty entity type")
	}
}

func TestUpdateEntity(t *testing.T) {
	ws, root := newMockWorkspace(t), t.TempDir()
	var buf bytes.Buffer

	// Get reference entity for assertions
	entity, _ := ws.GetEntity("TKT-001")
	entityID := entity.ID

	r := New(ws, ws.Meta(), root, &buf)
	defer r.Close()

	script := `
local entity = rela.update_entity("TKT-001", {status = "closed"})
rela.output({
    id = entity.id,
    status = entity.properties.status
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["id"] != entityID {
		t.Errorf("Expected id=%s, got %v", entityID, result["id"])
	}
	if result["status"] != "closed" {
		t.Errorf("Expected status=closed, got %v", result["status"])
	}
}

func TestUpdateEntity_NotFound(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.update_entity("NONEXISTENT", {status = "closed"})`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for non-existent entity")
	}
	if !strings.Contains(err.Error(), "entity not found") {
		t.Errorf("Expected 'entity not found' error, got: %v", err)
	}
}

func TestDeleteEntity(t *testing.T) {
	ws, root := newMockWorkspace(t), t.TempDir()
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), root, &buf)
	defer r.Close()

	script := `
local success = rela.delete_entity("TKT-002", false)
rela.output({success = success})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}

	// Verify entity is deleted
	_, found := ws.GetEntity("TKT-002")
	if found {
		t.Error("Entity TKT-002 should have been deleted")
	}
}

func TestDeleteEntity_NotFound(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.delete_entity("NONEXISTENT")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for non-existent entity")
	}
}

func TestCreateRelation(t *testing.T) {
	ws, root := newMockWorkspace(t), t.TempDir()
	var buf bytes.Buffer

	// Get reference entities for assertions
	fromEntity, _ := ws.GetEntity("TKT-002")
	toEntity, _ := ws.GetEntity("FEAT-001")
	relType := "implements"

	r := New(ws, ws.Meta(), root, &buf)
	defer r.Close()

	// TKT-002 doesn't have a relation to FEAT-001 yet
	script := `
local rel = rela.create_relation("TKT-002", "implements", "FEAT-001")
rela.output({
    from = rel.from,
    type = rel.type,
    to = rel.to
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["from"] != fromEntity.ID {
		t.Errorf("Expected from=%s, got %v", fromEntity.ID, result["from"])
	}
	if result["type"] != relType {
		t.Errorf("Expected type=%s, got %v", relType, result["type"])
	}
	if result["to"] != toEntity.ID {
		t.Errorf("Expected to=%s, got %v", toEntity.ID, result["to"])
	}
}

func TestCreateRelation_MissingArgs(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.create_relation("", "implements", "FEAT-001")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for missing from argument")
	}
}

func TestDeleteRelation(t *testing.T) {
	ws, root := newMockWorkspace(t), t.TempDir()
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), root, &buf)
	defer r.Close()

	script := `
local success = rela.delete_relation("TKT-001", "implements", "FEAT-001")
rela.output({success = success})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
}

func TestFindPath(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local path = rela.find_path("TKT-001", "FEAT-001")
if path then
    rela.output({found = true, length = #path})
else
    rela.output({found = false})
end
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["found"] != true {
		t.Errorf("Expected found=true, got %v", result["found"])
	}
	// Path should have 2 steps: TKT-001 -> FEAT-001
	if result["length"] != float64(2) {
		t.Errorf("Expected length=2, got %v", result["length"])
	}
}

func TestFindPath_NoPath(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local path = rela.find_path("TKT-001", "NONEXISTENT")
if path then
    rela.output({found = true})
else
    rela.output({found = false})
end
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["found"] != false {
		t.Errorf("Expected found=false, got %v", result["found"])
	}
}

func TestFindPath_MissingArgs(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.find_path("", "FEAT-001")`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for empty from argument")
	}
}

func TestRefresh(t *testing.T) {
	ws, root := newMockWorkspace(t), t.TempDir()
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), root, &buf)
	defer r.Close()

	script := `
local success = rela.refresh()
rela.output({success = success})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
}

func TestTraceFrom(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	// Get reference entity for assertions
	entity, _ := ws.GetEntity("TKT-001")

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local trace = rela.trace_from("TKT-001", 0)
rela.output({
    id = trace.id,
    has_children = #trace.children > 0
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["id"] != entity.ID {
		t.Errorf("Expected id=%s, got %v", entity.ID, result["id"])
	}
	if result["has_children"] != true {
		t.Errorf("Expected has_children=true, got %v", result["has_children"])
	}
}

func TestTraceTo(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	// Get reference entity for assertions
	entity, _ := ws.GetEntity("FEAT-001")

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local trace = rela.trace_to("FEAT-001", 0)
rela.output({
    id = trace.id,
    has_children = #trace.children > 0
})
`
	tmpFile := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["id"] != entity.ID {
		t.Errorf("Expected id=%s, got %v", entity.ID, result["id"])
	}
	if result["has_children"] != true {
		t.Errorf("Expected has_children=true (TKT-001 -> FEAT-001), got %v", result["has_children"])
	}
}

// TestSandbox_DangerousLibrariesUnavailable verifies that dangerous Lua libraries
// (io, os, debug) are not available in the sandboxed runtime.
func TestSandbox_DangerousLibrariesUnavailable(t *testing.T) {
	ws := newMockWorkspace(t)

	tests := []struct {
		name   string
		script string
	}{
		{"io library", `if io then error("io should not be available") end`},
		{"os library", `if os then error("os should not be available") end`},
		{"debug library", `if debug then error("debug should not be available") end`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(ws, ws.Meta(), "/tmp", &buf)
			defer r.Close()

			// Should succeed because the libraries are not available
			err := r.RunString(tt.script)
			if err != nil {
				t.Errorf("Script failed unexpectedly: %v", err)
			}
		})
	}
}

// TestSandbox_DangerousFunctionsRemoved verifies that dangerous base functions
// are removed from the sandbox.
func TestSandbox_DangerousFunctionsRemoved(t *testing.T) {
	ws := newMockWorkspace(t)

	dangerousFuncs := []string{
		"loadfile",
		"dofile",
		"load",
		"loadstring",
		"rawget",
		"rawset",
		"rawequal",
		"rawlen",
		"getmetatable",
		"setmetatable",
	}

	for _, fn := range dangerousFuncs {
		t.Run(fn, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(ws, ws.Meta(), "/tmp", &buf)
			defer r.Close()

			script := `if ` + fn + ` ~= nil then error("` + fn + ` should be nil") end`
			err := r.RunString(script)
			if err != nil {
				t.Errorf("Function %s should be nil but script failed: %v", fn, err)
			}
		})
	}
}

// TestSandbox_SafeLibrariesAvailable verifies that safe Lua libraries are available.
func TestSandbox_SafeLibrariesAvailable(t *testing.T) {
	ws := newMockWorkspace(t)

	tests := []struct {
		name   string
		script string
	}{
		{"string library", `if not string.len then error("string.len not available") end`},
		{"table library", `if not table.insert then error("table.insert not available") end`},
		{"math library", `if not math.floor then error("math.floor not available") end`},
		{"coroutine library", `if not coroutine.create then error("coroutine.create not available") end`},
		{"base print", `if not print then error("print not available") end`},
		{"base pairs", `if not pairs then error("pairs not available") end`},
		{"base ipairs", `if not ipairs then error("ipairs not available") end`},
		{"base type", `if not type then error("type not available") end`},
		{"base tostring", `if not tostring then error("tostring not available") end`},
		{"base tonumber", `if not tonumber then error("tonumber not available") end`},
		{"base error", `if not error then error("error not available") end`},
		{"base pcall", `if not pcall then error("pcall not available") end`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(ws, ws.Meta(), "/tmp", &buf)
			defer r.Close()

			err := r.RunString(tt.script)
			if err != nil {
				t.Errorf("Script failed: %v", err)
			}
		})
	}
}

// TestWriteFile_NestedDirectories verifies that write_file creates nested directories in output/.
func TestWriteFile_NestedDirectories(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Write to a deeply nested path within output/
	script := `rela.write_file("reports/2024/summary.txt", "nested content")`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	// Verify file was created in output/ subdirectory
	outFile := filepath.Join(projectRoot, "output", "reports", "2024", "summary.txt")
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "nested content" {
		t.Errorf("Expected 'nested content', got %q", string(content))
	}
}

// TestSetArgs verifies that SetArgs correctly sets script arguments.
func TestSetArgs(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	r.SetArgs([]string{"arg1", "arg2", "arg3"})

	script := `rela.output(rela.args)`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(result) != 3 || result[0] != "arg1" || result[1] != "arg2" || result[2] != "arg3" {
		t.Errorf("Expected [arg1, arg2, arg3], got %v", result)
	}
}

func TestSortEntities_ByStringProperty(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	// Tickets have statuses "open" and "done" - sort by status
	script := `
local tickets = rela.list_entities("ticket")
local sorted = rela.sort_entities(tickets, "status")
local result = {}
for i, e in ipairs(sorted) do
    result[i] = e.properties.status
end
rela.output(result)
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// "done" < "open" alphabetically
	if len(result) != 2 || result[0] != "done" || result[1] != "open" {
		t.Errorf("Expected [done, open], got %v", result)
	}
}

func TestSortEntities_ByNumericProperty(t *testing.T) {
	// Create workspace with entities that have numeric-like order property
	g := graph.New()
	g.AddNode(&model.Entity{
		ID:   "DOC-001",
		Type: "doc",
		Properties: map[string]interface{}{
			"title": "Third",
			"order": "3",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "DOC-002",
		Type: "doc",
		Properties: map[string]interface{}{
			"title": "First",
			"order": "1",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "DOC-003",
		Type: "doc",
		Properties: map[string]interface{}{
			"title": "Second",
			"order": "2",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "DOC-010",
		Type: "doc",
		Properties: map[string]interface{}{
			"title": "Tenth",
			"order": "10",
		},
	})

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {
				IDPrefix: "DOC",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
					"order": {Type: "string"},
				},
			},
		},
	}
	ws := newMockWorkspaceWith(g, meta)

	var buf bytes.Buffer
	r := New(ws, meta, "/tmp", &buf)
	defer r.Close()

	script := `
local docs = rela.list_entities("doc")
local sorted = rela.sort_entities(docs, "order")
local result = {}
for i, e in ipairs(sorted) do
    result[i] = e.properties.order
end
rela.output(result)
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Should sort numerically: 1, 2, 3, 10 (not lexicographically: 1, 10, 2, 3)
	expected := []string{"1", "2", "3", "10"}
	if len(result) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result))
	}
	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("At index %d: expected %s, got %v", i, exp, result[i])
		}
	}
}

func TestSortEntities_Descending(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local tickets = rela.list_entities("ticket")
local sorted = rela.sort_entities(tickets, "status", "desc")
local result = {}
for i, e in ipairs(sorted) do
    result[i] = e.properties.status
end
rela.output(result)
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Descending: "open" before "done"
	if len(result) != 2 || result[0] != "open" || result[1] != "done" {
		t.Errorf("Expected [open, done], got %v", result)
	}
}

func TestSortEntities_EmptyProperty(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.sort_entities({}, "")`
	err := r.RunString(script)
	if err == nil {
		t.Fatal("Expected error for empty property")
	}
	if !strings.Contains(err.Error(), "property cannot be empty") {
		t.Errorf("Expected 'property cannot be empty' error, got: %v", err)
	}
}

func TestEntityProp(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local e = rela.get_entity("TKT-001")
rela.output({
    title = e:prop("title", "default"),
    missing = e:prop("nonexistent", "fallback"),
    status = e:prop("status")
})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["title"] != "Test Ticket" {
		t.Errorf("Expected title='Test Ticket', got %v", result["title"])
	}
	if result["missing"] != "fallback" {
		t.Errorf("Expected missing='fallback', got %v", result["missing"])
	}
	if result["status"] != "open" {
		t.Errorf("Expected status='open', got %v", result["status"])
	}
}

func TestEntityProp_EmptyStringUsesDefault(t *testing.T) {
	// Create entity with empty string property
	g := graph.New()
	g.AddNode(&model.Entity{
		ID:   "TEST-001",
		Type: "test",
		Properties: map[string]interface{}{
			"title":   "Has Title",
			"summary": "", // empty string
		},
	})

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"test": {
				IDPrefix: "TEST",
				Properties: map[string]metamodel.PropertyDef{
					"title":   {Type: "string"},
					"summary": {Type: "string"},
				},
			},
		},
	}
	ws := newMockWorkspaceWith(g, meta)

	var buf bytes.Buffer
	r := New(ws, meta, "/tmp", &buf)
	defer r.Close()

	script := `
local e = rela.get_entity("TEST-001")
rela.output({
    summary = e:prop("summary", "no summary")
})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Empty string should use default
	if result["summary"] != "no summary" {
		t.Errorf("Expected summary='no summary', got %v", result["summary"])
	}
}

func TestWriteFile_EnsureNewline(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Write without trailing newline, but with ensure_newline option
	script := `rela.write_file("test.txt", "no newline", {ensure_newline = true})`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	outFile := filepath.Join(projectRoot, "output", "test.txt")
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "no newline\n" {
		t.Errorf("Expected 'no newline\\n', got %q", string(content))
	}
}

func TestWriteFile_EnsureNewline_AlreadyHasNewline(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Write with trailing newline and ensure_newline option - should not double
	script := `rela.write_file("test.txt", "has newline\n", {ensure_newline = true})`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	outFile := filepath.Join(projectRoot, "output", "test.txt")
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "has newline\n" {
		t.Errorf("Expected 'has newline\\n', got %q", string(content))
	}
}

func TestWriteFile_EnsureNewline_EmptyContent(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Empty content should stay empty even with ensure_newline
	script := `rela.write_file("empty.txt", "", {ensure_newline = true})`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	outFile := filepath.Join(projectRoot, "output", "empty.txt")
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if len(content) != 0 {
		t.Errorf("Expected empty string, got %q", string(content))
	}
}

func TestEntityStripPrefix(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local e = rela.get_entity("TKT-001")
rela.output({slug = e:strip_prefix()})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["slug"] != "001" {
		t.Errorf("Expected slug='001', got %v", result["slug"])
	}
}

func TestEntityStripPrefix_NoHyphen(t *testing.T) {
	// Create entity with ID that has no hyphen
	g := graph.New()
	g.AddNode(&model.Entity{
		ID:         "NOHYPHEN",
		Type:       "test",
		Properties: map[string]interface{}{},
	})

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"test": {IDPrefix: "TEST"},
		},
	}
	ws := newMockWorkspaceWith(g, meta)

	var buf bytes.Buffer
	r := New(ws, meta, "/tmp", &buf)
	defer r.Close()

	script := `
local e = rela.get_entity("NOHYPHEN")
rela.output({slug = e:strip_prefix()})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// No hyphen, should return ID as-is
	if result["slug"] != "NOHYPHEN" {
		t.Errorf("Expected slug='NOHYPHEN', got %v", result["slug"])
	}
}

func TestMdLink(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.output({link = rela.md.link("Guide", "docs/guide.md")})`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	expected := "[Guide](docs/guide.md)"
	if result["link"] != expected {
		t.Errorf("Expected link=%q, got %v", expected, result["link"])
	}
}

func TestMdRef(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `rela.output({ref = rela.md.ref("GUIDE-001", "the guide")})`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	expected := "[the guide][GUIDE-001]"
	if result["ref"] != expected {
		t.Errorf("Expected ref=%q, got %v", expected, result["ref"])
	}
}

func TestMdTable(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local tbl = rela.md.table(
    {"Name", "Status"},
    {
        {"Alice", "active"},
        {"Bob", "pending"}
    }
)
rela.output({table = tbl})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	expected := "| Name | Status |\n| -------- | -------- |\n| Alice | active |\n| Bob | pending |\n"
	if result["table"] != expected {
		t.Errorf("Expected table=%q, got %q", expected, result["table"])
	}
}

func TestMdEntityTable_PropertyColumns(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local tickets = rela.sort_entities(rela.list_entities("ticket"), "status")
local tbl = rela.md.entity_table(tickets, {
    {"Title", "title"},
    {"Status", "status", "unknown"}
})
rela.output({table = tbl})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	tableStr := result["table"].(string)
	// Should have header row
	if !strings.Contains(tableStr, "| Title | Status |") {
		t.Errorf("Expected header row, got %q", tableStr)
	}
	// Should have data rows with sorted values (done before open)
	if !strings.Contains(tableStr, "| Done Ticket | done |") {
		t.Errorf("Expected 'Done Ticket' row, got %q", tableStr)
	}
	if !strings.Contains(tableStr, "| Test Ticket | open |") {
		t.Errorf("Expected 'Test Ticket' row, got %q", tableStr)
	}
}

func TestMdEntityTable_FunctionColumn(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	script := `
local tickets = rela.list_entities("ticket")
local tbl = rela.md.entity_table(tickets, {
    {"Link", function(e)
        return rela.md.link(e:prop("title", e.id), e:strip_prefix() .. ".md")
    end}
})
rela.output({table = tbl})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	tableStr := result["table"].(string)
	// Should have links
	if !strings.Contains(tableStr, "[Test Ticket](001.md)") && !strings.Contains(tableStr, "[Done Ticket](002.md)") {
		t.Errorf("Expected markdown links in table, got %q", tableStr)
	}
}

func TestMdEntityTable_DefaultValue(t *testing.T) {
	// Create entity with missing property
	g := graph.New()
	g.AddNode(&model.Entity{
		ID:   "TEST-001",
		Type: "test",
		Properties: map[string]interface{}{
			"title": "Has Title",
			// no "status" property
		},
	})

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"test": {
				IDPrefix: "TEST",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
			},
		},
	}
	ws := newMockWorkspaceWith(g, meta)

	var buf bytes.Buffer
	r := New(ws, meta, "/tmp", &buf)
	defer r.Close()

	script := `
local entities = rela.list_entities("test")
local tbl = rela.md.entity_table(entities, {
    {"Title", "title"},
    {"Status", "status", "draft"}
})
rela.output({table = tbl})
`
	err := r.RunString(script)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	tableStr := result["table"].(string)
	// Should use default value "draft" for missing status
	if !strings.Contains(tableStr, "| Has Title | draft |") {
		t.Errorf("Expected default value 'draft', got %q", tableStr)
	}
}

// Tests for shebang handling

func TestStripShebang(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with shebang", "#!/usr/bin/env rela script\nprint('hello')", "\nprint('hello')"},
		{"without shebang", "print('hello')", "print('hello')"},
		{"shebang only", "#!/usr/bin/env rela script", ""},
		{"hash but not shebang", "#not a shebang\nprint('hello')", "#not a shebang\nprint('hello')"},
		{"empty string", "", ""},
		{"shebang in middle", "print('hello')\n#!/usr/bin/env rela\nprint('world')", "print('hello')\n#!/usr/bin/env rela\nprint('world')"},
		{"windows CRLF", "#!/usr/bin/env rela script\r\nprint('hello')", "\nprint('hello')"},
		{"UTF-8 BOM with shebang", "\xEF\xBB\xBF#!/usr/bin/env rela\nprint('hello')", "\nprint('hello')"},
		{"UTF-8 BOM without shebang", "\xEF\xBB\xBFprint('hello')", "print('hello')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripShebang(tt.input)
			if got != tt.expected {
				t.Errorf("stripShebang(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestShebangExecution(t *testing.T) {
	script := "#!/usr/bin/env rela script\nrela.output({status = 'ok'})"

	t.Run("RunString", func(t *testing.T) {
		ws := testWorkspace(t)
		var buf bytes.Buffer
		r := New(ws, ws.Meta(), "/tmp", &buf)
		defer r.Close()

		if err := r.RunString(script); err != nil {
			t.Fatalf("RunString with shebang failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("Failed to parse output: %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("Expected status=ok, got %v", result["status"])
		}
	})

	t.Run("RunFile", func(t *testing.T) {
		ws := testWorkspace(t)
		var buf bytes.Buffer
		r := New(ws, ws.Meta(), "/tmp", &buf)
		defer r.Close()

		tmpFile := filepath.Join(t.TempDir(), "test.lua")
		if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
			t.Fatal(err)
		}

		if err := r.RunFile(tmpFile, nil); err != nil {
			t.Fatalf("RunFile with shebang failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("Failed to parse output: %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("Expected status=ok, got %v", result["status"])
		}
	})
}

func TestRunFile_Errors(t *testing.T) {
	t.Run("line numbers preserved with shebang", func(t *testing.T) {
		ws := testWorkspace(t)
		var buf bytes.Buffer
		r := New(ws, ws.Meta(), "/tmp", &buf)
		defer r.Close()

		tmpFile := filepath.Join(t.TempDir(), "test.lua")
		if err := os.WriteFile(tmpFile, []byte("#!/usr/bin/env rela script\nsyntax error here"), 0644); err != nil {
			t.Fatal(err)
		}

		err := r.RunFile(tmpFile, nil)
		if err == nil {
			t.Fatal("Expected error for syntax error")
		}

		// "line:2" (colon, no space) is gopher-lua's error format.
		if !strings.Contains(err.Error(), "line:2") {
			t.Errorf("Expected error on line 2, got: %v", err)
		}
	})

	t.Run("includes filename", func(t *testing.T) {
		ws := testWorkspace(t)
		var buf bytes.Buffer
		r := New(ws, ws.Meta(), "/tmp", &buf)
		defer r.Close()

		tmpFile := filepath.Join(t.TempDir(), "mytest.lua")
		if err := os.WriteFile(tmpFile, []byte("syntax error"), 0644); err != nil {
			t.Fatal(err)
		}

		err := r.RunFile(tmpFile, nil)
		if err == nil {
			t.Fatal("Expected error for syntax error")
		}

		if !strings.Contains(err.Error(), "mytest.lua") {
			t.Errorf("Expected error to include filename, got: %v", err)
		}
	})
}

func TestWithParams(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	params := map[string]string{
		"entity_type":  "ticket",
		"key_property": "date",
	}
	r := New(ws, ws.Meta(), "/tmp", &buf, WithParams(params))
	defer r.Close()

	script := `rela.output({et = rela.params.entity_type, kp = rela.params.key_property})`
	if err := r.RunString(script); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["et"] != "ticket" || result["kp"] != "date" {
		t.Errorf("Expected ticket/date, got %v", result)
	}
}

func TestWithParams_Empty(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf)
	defer r.Close()

	// rela.params should exist as an empty table
	script := `
		local count = 0
		for _ in pairs(rela.params) do count = count + 1 end
		rela.output({count = count})
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]float64
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["count"] != 0 {
		t.Errorf("Expected empty params, got count=%v", result["count"])
	}
}

func TestRunActionString_ReturnTable(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf, WithActionMode())
	defer r.Close()

	script := `return {redirect = "/foo", message = "hi"}`
	ret, err := r.RunActionString(script, "test.lua")
	if err != nil {
		t.Fatalf("RunActionString failed: %v", err)
	}

	m, ok := ret.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", ret)
	}
	if m["redirect"] != "/foo" {
		t.Errorf("expected redirect=/foo, got %v", m["redirect"])
	}
	if m["message"] != "hi" {
		t.Errorf("expected message=hi, got %v", m["message"])
	}
}

func TestRunActionString_NoReturn(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf, WithActionMode())
	defer r.Close()

	script := `local x = 1` // no return statement
	_, err := r.RunActionString(script, "test.lua")
	if !errors.Is(err, ErrNoReturnValue) {
		t.Errorf("expected ErrNoReturnValue, got %v", err)
	}
}

func TestRunActionString_Error(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf, WithActionMode())
	defer r.Close()

	script := `error("boom")`
	_, err := r.RunActionString(script, "test.lua")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error to mention 'boom', got %v", err)
	}
}

func TestActionMode_OutputIsWarning(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws, ws.Meta(), "/tmp", &buf, WithActionMode())
	defer r.Close()

	// rela.output in action mode should not write JSON, just a warning
	script := `rela.output({foo = "bar"})`
	if err := r.RunString(script); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "warning") {
		t.Errorf("expected warning in output, got %q", out)
	}
	if strings.Contains(out, `"foo"`) {
		t.Errorf("expected output to be dropped in action mode, got %q", out)
	}
}

// TestWithContext_CancellationInterruptsBusyLoop verifies that canceling the
// parent context passed via WithContext interrupts an in-flight Lua busy loop,
// rather than waiting for the internal timeout to fire.
func TestWithContext_CancellationInterruptsBusyLoop(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	// Parent context with a tight deadline. The internal Lua timeout is left
	// at its default (30s) so that any cancellation we observe must come from
	// the parent context, not the timeout fallback.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := New(ws, ws.Meta(), "/tmp", &buf, WithContext(ctx))
	defer r.Close()

	start := time.Now()
	err := r.RunString(`while true do end`)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected busy loop to be interrupted, got nil error")
	}
	// Must finish well before the default 30s timeout would fire.
	if elapsed > 5*time.Second {
		t.Fatalf("busy loop ran for %v; expected parent context to interrupt quickly", elapsed)
	}
	// gopher-lua surfaces context cancellation as "context deadline exceeded"
	// or "context canceled" embedded in the error message.
	msg := err.Error()
	if !strings.Contains(msg, "context") {
		t.Errorf("expected context-related error, got: %v", err)
	}
}

// TestWithContext_NoTimeoutStillCancels verifies that even with the internal
// timeout disabled, a cancelled parent context interrupts execution.
func TestWithContext_NoTimeoutStillCancels(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())

	r := New(ws, ws.Meta(), "/tmp", &buf, WithContext(ctx), WithTimeout(0))
	defer r.Close()

	// Cancel shortly after starting.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := r.RunString(`while true do end`)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected busy loop to be interrupted, got nil error")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("busy loop ran for %v; expected parent cancel to interrupt", elapsed)
	}
}
