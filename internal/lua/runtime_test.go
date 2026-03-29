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

	if result["id"] != "TKT-001" {
		t.Errorf("Expected id=TKT-001, got %v", result["id"])
	}
	if result["type"] != "ticket" {
		t.Errorf("Expected type=ticket, got %v", result["type"])
	}
	if result["title"] != "Test Ticket" {
		t.Errorf("Expected title=Test Ticket, got %v", result["title"])
	}
	if result["content"] != "Test content" {
		t.Errorf("Expected content=Test content, got %v", result["content"])
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
	if first["from"] != "TKT-001" {
		t.Errorf("Expected from=TKT-001, got %v", first["from"])
	}
	if first["type"] != "implements" {
		t.Errorf("Expected type=implements, got %v", first["type"])
	}
	if first["to"] != "FEAT-001" {
		t.Errorf("Expected to=FEAT-001, got %v", first["to"])
	}
}

func TestWriteFile(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	// Use same temp dir for project root and output file
	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	outFile := filepath.Join(projectRoot, "output.txt")

	script := `rela.write_file("` + outFile + `", "hello world")`
	tmpFile := filepath.Join(projectRoot, "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

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

	// Try to write outside project root using path traversal
	script := `rela.write_file("../outside.txt", "malicious")`
	tmpFile := filepath.Join(projectRoot, "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for path traversal attempt")
	}
	if !strings.Contains(err.Error(), "must be within project root") {
		t.Errorf("Expected 'must be within project root' error, got: %v", err)
	}
}

func TestWriteFile_AbsolutePathOutside(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	// Try to write to absolute path outside project
	script := `rela.write_file("/tmp/outside.txt", "malicious")`
	tmpFile := filepath.Join(projectRoot, "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	err := r.RunFile(tmpFile, nil)
	if err == nil {
		t.Fatal("Expected error for absolute path outside project")
	}
}

func TestWriteFile_WithinProject(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

	projectRoot := t.TempDir()
	r := New(ws, ws.Meta(), projectRoot, &buf)
	defer r.Close()

	outFile := filepath.Join(projectRoot, "output.txt")
	script := `rela.write_file("` + outFile + `", "allowed")`
	tmpFile := filepath.Join(projectRoot, "test.lua")
	if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.RunFile(tmpFile, nil); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

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

	if result["id"] != "TKT-001" {
		t.Errorf("Expected id=TKT-001, got %v", result["id"])
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

	if result["from"] != "TKT-002" {
		t.Errorf("Expected from=TKT-002, got %v", result["from"])
	}
	if result["type"] != "implements" {
		t.Errorf("Expected type=implements, got %v", result["type"])
	}
	if result["to"] != "FEAT-001" {
		t.Errorf("Expected to=FEAT-001, got %v", result["to"])
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

	if result["id"] != "TKT-001" {
		t.Errorf("Expected id=TKT-001, got %v", result["id"])
	}
	if result["has_children"] != true {
		t.Errorf("Expected has_children=true, got %v", result["has_children"])
	}
}

func TestTraceTo(t *testing.T) {
	ws := testWorkspace(t)
	var buf bytes.Buffer

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

	if result["id"] != "FEAT-001" {
		t.Errorf("Expected id=FEAT-001, got %v", result["id"])
	}
	if result["has_children"] != true {
		t.Errorf("Expected has_children=true (TKT-001 -> FEAT-001), got %v", result["has_children"])
	}
}
