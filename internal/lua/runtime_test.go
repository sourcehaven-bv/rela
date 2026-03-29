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
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// testWorkspace creates a workspace with test entities for testing.
func testWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
			},
			"feature": {
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

	ws := workspace.NewForTest(g, meta)
	return ws
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
