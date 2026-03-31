package lua

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
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

// testWorkspace creates a workspace with test entities for testing.
func testWorkspace(t *testing.T) *workspace.Workspace {
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

	ws := workspace.NewForTest(g, testMeta())
	return ws
}

// testWorkspaceWithRepo creates a workspace with a real repository for mutation tests.
// Returns the workspace and the project root path.
func testWorkspaceWithRepo(t *testing.T) (ws *workspace.Workspace, root string) {
	t.Helper()

	fs := storage.NewMemFS()
	root = "/project"
	ctx := &project.Context{
		Root:          root,
		EntitiesDir:   filepath.Join(root, "entities"),
		RelationsDir:  filepath.Join(root, "relations"),
		MetamodelPath: filepath.Join(root, "metamodel.yaml"),
		CacheDir:      filepath.Join(root, ".rela"),
	}

	// Create directories
	_ = fs.MkdirAll(ctx.CacheDir, 0755)
	_ = fs.MkdirAll(filepath.Join(ctx.EntitiesDir, "tickets"), 0755)
	_ = fs.MkdirAll(filepath.Join(ctx.EntitiesDir, "features"), 0755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0755)

	// Write metamodel
	metamodelContent := `entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: TKT
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: string}
  feature:
    label: Feature
    plural: features
    id_prefix: FEAT
    id_type: sequential
    properties:
      title: {type: string}

relations:
  implements:
    from: [ticket]
    to: [feature]
`
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(metamodelContent), 0644)

	// Write test entities
	tkt001 := `---
title: Test Ticket
status: open
---
Test content
`
	_ = fs.WriteFile(filepath.Join(ctx.EntitiesDir, "tickets", "TKT-001.md"), []byte(tkt001), 0644)

	tkt002 := `---
title: Done Ticket
status: done
---
`
	_ = fs.WriteFile(filepath.Join(ctx.EntitiesDir, "tickets", "TKT-002.md"), []byte(tkt002), 0644)

	feat001 := `---
title: Test Feature
---
`
	_ = fs.WriteFile(filepath.Join(ctx.EntitiesDir, "features", "FEAT-001.md"), []byte(feat001), 0644)

	// Write test relation
	rel := `---
---
`
	_ = fs.WriteFile(filepath.Join(ctx.RelationsDir, "TKT-001--implements--FEAT-001.md"), []byte(rel), 0644)

	// Create graph with test entities (same as testWorkspace)
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

	// Create repository and workspace with graph
	repo := repository.New(fs, ctx)
	meta, err := repo.LoadMetamodel()
	if err != nil {
		t.Fatal(err)
	}
	ws = workspace.NewWithGraph(repo, meta, g)

	return ws, root
}

func TestRunFile_BasicOutput(t *testing.T) {
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws, root := testWorkspaceWithRepo(t)
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
	ws := testWorkspace(t)
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
	ws, root := testWorkspaceWithRepo(t)
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
	ws := testWorkspace(t)
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
	ws, root := testWorkspaceWithRepo(t)
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
	ws := testWorkspace(t)
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
	ws, root := testWorkspaceWithRepo(t)
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
	ws := testWorkspace(t)
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
	ws, root := testWorkspaceWithRepo(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws, root := testWorkspaceWithRepo(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)

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
	ws := testWorkspace(t)

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
	ws := testWorkspace(t)

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
	ws := testWorkspace(t)
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
	ws := testWorkspace(t)
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
	ws := workspace.NewForTest(g, meta)

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
	ws := workspace.NewForTest(g, meta)

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
