package markdown

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

func TestSyncFromFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	// Create entity directories
	reqDir := filepath.Join(ctx.EntitiesDir, "requirement")
	decDir := filepath.Join(ctx.EntitiesDir, "decision")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("failed to create requirement dir: %v", err)
	}
	if err := os.MkdirAll(decDir, 0755); err != nil {
		t.Fatalf("failed to create decision dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	// Create test entities
	entities := map[string]string{
		"REQ-001": `---
id: REQ-001
type: requirement
title: Requirement 1
---
`,
		"REQ-002": `---
id: REQ-002
type: requirement
title: Requirement 2
---
`,
		"DEC-001": `---
id: DEC-001
type: decision
title: Decision 1
---
`,
	}

	for id, content := range entities {
		var dir string
		if id[0] == 'R' {
			dir = reqDir
		} else {
			dir = decDir
		}
		path := filepath.Join(dir, id+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create entity %s: %v", id, err)
		}
	}

	// Create test relations
	relations := []struct {
		from, relType, to string
	}{
		{"DEC-001", "addresses", "REQ-001"},
		{"DEC-001", "addresses", "REQ-002"},
	}

	for _, rel := range relations {
		content := `---
from: ` + rel.from + `
relation: ` + rel.relType + `
to: ` + rel.to + `
---
`
		filename := RelationFilename(rel.from, rel.relType, rel.to)
		path := filepath.Join(ctx.RelationsDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create relation: %v", err)
		}
	}

	// Sync
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement"},
			"decision":    {Label: "Decision"},
		},
	}
	g := graph.New()

	result, err := SyncFromFiles(ctx, meta, g)
	if err != nil {
		t.Fatalf("SyncFromFiles failed: %v", err)
	}

	// Verify results
	if result.EntitiesLoaded != 3 {
		t.Errorf("EntitiesLoaded = %d, want 3", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 2 {
		t.Errorf("RelationsLoaded = %d, want 2", result.RelationsLoaded)
	}
	if len(result.Errors) != 0 {
		t.Errorf("got %d errors, want 0: %v", len(result.Errors), result.Errors)
	}

	// Verify graph state
	if g.NodeCount() != 3 {
		t.Errorf("graph has %d nodes, want 3", g.NodeCount())
	}
	if g.EdgeCount() != 2 {
		t.Errorf("graph has %d edges, want 2", g.EdgeCount())
	}

	// Verify specific entities exist
	if _, ok := g.GetNode("REQ-001"); !ok {
		t.Error("REQ-001 should exist in graph")
	}
	if _, ok := g.GetNode("REQ-002"); !ok {
		t.Error("REQ-002 should exist in graph")
	}
	if _, ok := g.GetNode("DEC-001"); !ok {
		t.Error("DEC-001 should exist in graph")
	}
}

func TestSyncFromFiles_MissingSourceEntity(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	// Create only target entity
	reqDir := filepath.Join(ctx.EntitiesDir, "requirement")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	entityContent := `---
id: REQ-001
type: requirement
---
`
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-001.md"), []byte(entityContent), 0644); err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	// Create relation with missing source
	relationContent := `---
from: DEC-999
relation: addresses
to: REQ-001
---
`
	relationPath := filepath.Join(ctx.RelationsDir, "DEC-999--addresses--REQ-001.md")
	if err := os.WriteFile(relationPath, []byte(relationContent), 0644); err != nil {
		t.Fatalf("failed to create relation: %v", err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement"},
		},
	}
	g := graph.New()

	result, err := SyncFromFiles(ctx, meta, g)
	if err != nil {
		t.Fatalf("SyncFromFiles failed: %v", err)
	}

	// Should have 1 entity, 0 relations, 1 error
	if result.EntitiesLoaded != 1 {
		t.Errorf("EntitiesLoaded = %d, want 1", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 0 {
		t.Errorf("RelationsLoaded = %d, want 0 (missing source)", result.RelationsLoaded)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(result.Errors))
	}

	// Verify error message
	if result.Errors[0].Error() != relationPath+": source entity not found: DEC-999" {
		t.Errorf("error = %q, want source not found error", result.Errors[0].Error())
	}
}

func TestSyncFromFiles_MissingTargetEntity(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	// Create only source entity
	decDir := filepath.Join(ctx.EntitiesDir, "decision")
	if err := os.MkdirAll(decDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	entityContent := `---
id: DEC-001
type: decision
---
`
	if err := os.WriteFile(filepath.Join(decDir, "DEC-001.md"), []byte(entityContent), 0644); err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	// Create relation with missing target
	relationContent := `---
from: DEC-001
relation: addresses
to: REQ-999
---
`
	relationPath := filepath.Join(ctx.RelationsDir, "DEC-001--addresses--REQ-999.md")
	if err := os.WriteFile(relationPath, []byte(relationContent), 0644); err != nil {
		t.Fatalf("failed to create relation: %v", err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"decision": {Label: "Decision"},
		},
	}
	g := graph.New()

	result, err := SyncFromFiles(ctx, meta, g)
	if err != nil {
		t.Fatalf("SyncFromFiles failed: %v", err)
	}

	// Should have 1 entity, 0 relations, 1 error
	if result.EntitiesLoaded != 1 {
		t.Errorf("EntitiesLoaded = %d, want 1", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 0 {
		t.Errorf("RelationsLoaded = %d, want 0 (missing target)", result.RelationsLoaded)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(result.Errors))
	}

	// Verify error message
	if result.Errors[0].Error() != relationPath+": target entity not found: REQ-999" {
		t.Errorf("error = %q, want target not found error", result.Errors[0].Error())
	}
}

func TestSyncFromFiles_EmptyDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	// Create empty directories
	if err := os.MkdirAll(ctx.EntitiesDir, 0755); err != nil {
		t.Fatalf("failed to create entities dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	meta := &metamodel.Metamodel{}
	g := graph.New()

	result, err := SyncFromFiles(ctx, meta, g)
	if err != nil {
		t.Fatalf("SyncFromFiles failed: %v", err)
	}

	if result.EntitiesLoaded != 0 {
		t.Errorf("EntitiesLoaded = %d, want 0", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 0 {
		t.Errorf("RelationsLoaded = %d, want 0", result.RelationsLoaded)
	}
	if len(result.Errors) != 0 {
		t.Errorf("got %d errors, want 0", len(result.Errors))
	}
}

func TestSyncFromFiles_ClearsExistingGraph(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	// Create entity directory
	reqDir := filepath.Join(ctx.EntitiesDir, "requirement")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	// Create one entity
	entityContent := `---
id: REQ-001
type: requirement
---
`
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-001.md"), []byte(entityContent), 0644); err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement"},
		},
	}
	g := graph.New()

	// Pre-populate graph with different data
	oldEntity := &model.Entity{
		ID:         "OLD-001",
		Type:       "old",
		Properties: make(map[string]interface{}),
	}
	g.AddNode(oldEntity)

	if g.NodeCount() != 1 {
		t.Fatalf("pre-sync: graph has %d nodes, want 1", g.NodeCount())
	}

	// Sync should clear old data
	result, err := SyncFromFiles(ctx, meta, g)
	if err != nil {
		t.Fatalf("SyncFromFiles failed: %v", err)
	}

	if result.EntitiesLoaded != 1 {
		t.Errorf("EntitiesLoaded = %d, want 1", result.EntitiesLoaded)
	}

	// Old entity should be gone
	if _, ok := g.GetNode("OLD-001"); ok {
		t.Error("OLD-001 should not exist after sync (graph should be cleared)")
	}

	// New entity should exist
	if _, ok := g.GetNode("REQ-001"); !ok {
		t.Error("REQ-001 should exist after sync")
	}

	if g.NodeCount() != 1 {
		t.Errorf("graph has %d nodes, want 1", g.NodeCount())
	}
}

func TestSyncError_Error(t *testing.T) {
	err := &SyncError{
		File:    "/path/to/file.md",
		Message: "something went wrong",
	}

	expected := "/path/to/file.md: something went wrong"
	if got := err.Error(); got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}
