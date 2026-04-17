package lua

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
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

// mockWorkspace is a test helper that bundles a memstore-backed lua.Services
// with a mirrored graph for test assertions. The graph and store are kept in
// sync so existing graph-based test helpers keep working.
type mockWorkspace struct {
	graph *graph.Graph
	meta  *metamodel.Metamodel
	store *memstore.MemStore
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

	return newMockWorkspaceWith(g, testMeta())
}

// newMockWorkspaceWith creates a mock workspace with a custom graph and metamodel.
// The graph contents are copied into a memstore so the lua runtime can query them
// through the Services interface.
func newMockWorkspaceWith(g *graph.Graph, meta *metamodel.Metamodel) *mockWorkspace {
	st := memstore.New()
	ctx := context.Background()
	for _, e := range g.AllNodes() {
		_ = st.CreateEntity(ctx, model.EntityToDomain(e))
	}
	for _, r := range g.AllEdges() {
		_, _ = st.CreateRelation(ctx, r.From, r.Type, r.To, nil)
	}
	return &mockWorkspace{graph: g, meta: meta, store: st}
}

// services returns a lua.Services bound to the mock's store, with projectRoot set.
func (m *mockWorkspace) services(projectRoot string) Services {
	return Services{
		Store:       m.store,
		Manager:     &mockManager{ws: m},
		Tracer:      tracer.New(m.store),
		Searcher:    &mockSearcher{ws: m},
		Meta:        m.meta,
		ProjectRoot: projectRoot,
		Sync:        func() error { return nil },
	}
}

// GetEntity returns an entity from the underlying store (test helper).
func (m *mockWorkspace) GetEntity(id string) (*model.Entity, bool) {
	return m.graph.GetNode(id)
}

// Meta returns the metamodel for the mock workspace.
func (m *mockWorkspace) Meta() *metamodel.Metamodel {
	return m.meta
}

// Graph returns the graph for the mock workspace.
func (m *mockWorkspace) Graph() *graph.Graph {
	return m.graph
}

// testWorkspace is an alias for newMockWorkspace for tests that use the older naming.
func testWorkspace(t *testing.T) *mockWorkspace {
	t.Helper()
	return newMockWorkspace(t)
}

// mockManager is a minimal entitymanager.EntityManager for tests. It delegates
// to the underlying memstore and mirrors writes into the graph so test
// assertions using graph queries continue to work.
type mockManager struct {
	ws *mockWorkspace
}

var _ entitymanager.EntityManager = (*mockManager)(nil)

func (m *mockManager) CreateEntity(
	ctx context.Context, e *entity.Entity, opts entitymanager.CreateOptions,
) (*entitymanager.CreateResult, error) {
	if e == nil {
		return nil, nil
	}
	id := opts.ID
	if id == "" {
		id = fmt.Sprintf("%s-%03d", strings.ToUpper(e.Type[:3]), m.ws.graph.NodeCount()+1)
	}
	newE := &entity.Entity{
		ID:         id,
		Type:       e.Type,
		Properties: e.Properties,
		Content:    e.Content,
	}
	if err := m.ws.store.CreateEntity(ctx, newE); err != nil {
		return nil, err
	}
	mdl := model.EntityFromDomain(newE)
	m.ws.graph.AddNode(mdl)
	return &entitymanager.CreateResult{Entity: newE}, nil
}

func (m *mockManager) UpdateEntity(
	ctx context.Context, e *entity.Entity,
) (*entitymanager.UpdateResult, error) {
	if err := m.ws.store.UpdateEntity(ctx, e); err != nil {
		return nil, err
	}
	m.ws.graph.AddNode(model.EntityFromDomain(e))
	return &entitymanager.UpdateResult{Entity: e}, nil
}

func (m *mockManager) DeleteEntity(
	ctx context.Context, id string, cascade bool,
) (*entitymanager.DeleteResult, error) {
	current, ok := m.ws.graph.GetNode(id)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	if _, err := m.ws.store.DeleteEntity(ctx, id, cascade); err != nil {
		return nil, err
	}
	m.ws.graph.RemoveNode(id)
	return &entitymanager.DeleteResult{
		DeletedEntities: []*entity.Entity{model.EntityToDomain(current)},
	}, nil
}

func (m *mockManager) RenameEntity(
	_ context.Context, _, _ string, _ entitymanager.RenameOptions,
) (*entitymanager.RenameResult, error) {
	return nil, fmt.Errorf("rename not supported by mockManager")
}

func (m *mockManager) CreateRelation(
	ctx context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	var data *store.RelationData
	if len(opts.Properties) > 0 || opts.Content != "" {
		data = &store.RelationData{Properties: opts.Properties, Content: opts.Content}
	}
	r, err := m.ws.store.CreateRelation(ctx, from, relType, to, data)
	if err != nil {
		return nil, err
	}
	m.ws.graph.AddEdge(model.RelationFromDomain(r))
	return r, nil
}

func (m *mockManager) UpdateRelation(
	ctx context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	r, err := m.ws.store.UpdateRelation(ctx, from, relType, to, store.RelationData{
		Properties: opts.Properties, Content: opts.Content,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (m *mockManager) DeleteRelation(ctx context.Context, from, relType, to string) error {
	if err := m.ws.store.DeleteRelation(ctx, from, relType, to); err != nil {
		return err
	}
	m.ws.graph.RemoveEdge(from, relType, to)
	return nil
}

// mockSearcher is a naive title-substring searcher used by lua tests.
type mockSearcher struct {
	ws *mockWorkspace
}

var _ search.Searcher = (*mockSearcher)(nil)

func (s *mockSearcher) Search(_ context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		query := strings.ToLower(q.Text)
		count := 0
		for _, e := range s.ws.graph.AllNodes() {
			title := strings.ToLower(e.GetString("title"))
			if strings.Contains(title, query) {
				if !yield(search.Hit{ID: e.ID, Type: e.Type, Title: e.GetString("title")}, nil) {
					return
				}
				count++
				if q.Limit > 0 && count >= q.Limit {
					return
				}
			}
		}
	}
}

func TestRunFile_BasicOutput(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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
	rels := ws.graph.RelationsOfType("implements")
	if len(rels) == 0 {
		t.Fatal("Expected at least one implements relation in test workspace")
	}
	testRel := rels[0]

	r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

func TestWriteFile_PathTraversal(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws.services(projectRoot), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services(root), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services(root), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services(root), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services(root), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services(root), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services(root), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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
			r := New(ws.services("/tmp"), &buf)
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
			r := New(ws.services("/tmp"), &buf)
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
			r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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
	r := New(ws.services(projectRoot), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services("/tmp"), &buf)
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
		r := New(ws.services("/tmp"), &buf)
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
		r := New(ws.services("/tmp"), &buf)
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
		r := New(ws.services("/tmp"), &buf)
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
		r := New(ws.services("/tmp"), &buf)
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
	r := New(ws.services("/tmp"), &buf, WithParams(params))
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

	r := New(ws.services("/tmp"), &buf)
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

	r := New(ws.services("/tmp"), &buf, WithActionMode())
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

	r := New(ws.services("/tmp"), &buf, WithActionMode())
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

	r := New(ws.services("/tmp"), &buf, WithActionMode())
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

	r := New(ws.services("/tmp"), &buf, WithActionMode())
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

	r := New(ws.services("/tmp"), &buf, WithContext(ctx))
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

	r := New(ws.services("/tmp"), &buf, WithContext(ctx), WithTimeout(0))
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

func TestWithSecrets(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	sec := map[string]string{
		"api_key":  "sk-secret",
		"base_url": "https://example.com",
	}
	r := New(ws.services("/tmp"), &buf, WithSecrets(sec))
	defer r.Close()

	script := `rela.output({k = rela.secrets.api_key, u = rela.secrets.base_url})`
	if err := r.RunString(script); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if result["k"] != "sk-secret" || result["u"] != "https://example.com" {
		t.Errorf("expected sk-secret/https://example.com, got %v", result)
	}
}

func TestWithSecrets_Empty(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := New(ws.services("/tmp"), &buf)
	defer r.Close()

	// rela.secrets should exist as an empty table
	script := `
		local count = 0
		for _ in pairs(rela.secrets) do count = count + 1 end
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
		t.Errorf("Expected empty secrets, got count=%v", result["count"])
	}
}
