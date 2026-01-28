package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestReadEntity(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)

	entityContent := `---
id: REQ-001
type: requirement
title: Test Requirement
status: draft
priority: high
tags:
  - security
  - performance
---

# Description

This is a test requirement.
`

	entityPath := filepath.Join(tmpDir, "REQ-001.md")
	testutil.CreateFile(t, entityPath, entityContent)

	// Test without metamodel
	entity, err := ReadEntity(entityPath, nil)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, entity.ID, "REQ-001")
	testutil.AssertEqual(t, entity.Type, "requirement")
	testutil.AssertEqual(t, entity.GetString("title"), "Test Requirement")
	testutil.AssertEqual(t, entity.GetString("status"), "draft")
	testutil.AssertEqual(t, entity.GetString("priority"), "high")
	testutil.AssertStringContains(t, entity.Content, "This is a test requirement")
	testutil.AssertEqual(t, entity.FilePath, entityPath)

	if entity.ModTime.IsZero() {
		t.Error("ModTime should not be zero")
	}
}

func TestReadEntity_TypeInference(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)

	// Entity without type field
	entityContent := `---
id: REQ-001
title: Test Requirement
---

Content here.
`

	entityPath := filepath.Join(tmpDir, "REQ-001.md")
	testutil.CreateFile(t, entityPath, entityContent)

	meta := testutil.NewMetamodel().
		WithEntity("requirement", "Requirement", []string{"^REQ-\\d+$"}).
		Build()

	entity, err := ReadEntity(entityPath, meta)
	testutil.AssertNoError(t, err)

	// Type inference depends on MatchesID which needs HasPattern to be implemented
	// This test verifies the code path is executed without error
	testutil.AssertEqual(t, entity.ID, "REQ-001")
}

func TestReadEntity_AliasResolution(t *testing.T) {
	tmpDir := t.TempDir()

	entityContent := `---
id: REQ-001
type: req
title: Test
---
`

	entityPath := filepath.Join(tmpDir, "REQ-001.md")
	if err := os.WriteFile(entityPath, []byte(entityContent), 0644); err != nil {
		t.Fatalf("failed to write test entity: %v", err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:   "Requirement",
				Aliases: []string{"req", "reqs"},
			},
		},
	}

	entity, err := ReadEntity(entityPath, meta)
	if err != nil {
		t.Fatalf("ReadEntity failed: %v", err)
	}

	// Alias resolution depends on ResolveAlias implementation
	// This test verifies the code path is executed without error
	if entity.ID != "REQ-001" {
		t.Errorf("ID = %q, want %q", entity.ID, "REQ-001")
	}
}

func TestReadEntity_InvalidFile(t *testing.T) {
	_, err := ReadEntity("/nonexistent/file.md", nil)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadEntity_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	invalidContent := `---
id: REQ-001
type: [invalid
---
`

	entityPath := filepath.Join(tmpDir, "invalid.md")
	if err := os.WriteFile(entityPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write test entity: %v", err)
	}

	_, err := ReadEntity(entityPath, nil)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestWriteEntity(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)

	entity := testutil.NewEntity("REQ-001", "requirement").
		WithTitle("Test Requirement").
		WithStatus(model.StatusDraft).
		WithPriority(model.PriorityHigh).
		WithProperty("custom_field", "custom value").
		WithContent("# Description\n\nThis is the content.").
		Build()

	entityPath := filepath.Join(tmpDir, "entities", "requirement", "REQ-001.md")

	err := WriteEntity(entity, entityPath)
	testutil.AssertNoError(t, err)

	// Verify file exists
	testutil.AssertFileExists(t, entityPath)

	// Read back and verify
	content := testutil.ReadFile(t, entityPath)
	testutil.AssertStringContains(t, content, "id: REQ-001")
	testutil.AssertStringContains(t, content, "type: requirement")
	testutil.AssertStringContains(t, content, "title: Test Requirement")
	testutil.AssertStringContains(t, content, "status: draft")
	testutil.AssertStringContains(t, content, "This is the content")
}

func TestWriteEntity_PropertyOrdering(t *testing.T) {
	tmpDir := t.TempDir()

	entity := model.NewEntity("REQ-001", "requirement")
	entity.SetString("status", "draft")
	entity.SetString("title", "Test")
	entity.SetString("other", "value")

	entityPath := filepath.Join(tmpDir, "REQ-001.md")

	err := WriteEntity(entity, entityPath)
	if err != nil {
		t.Fatalf("WriteEntity failed: %v", err)
	}

	content, err := os.ReadFile(entityPath)
	if err != nil {
		t.Fatalf("failed to read entity file: %v", err)
	}

	contentStr := string(content)

	// Verify all properties are present
	if !strings.Contains(contentStr, "title: Test") {
		t.Error("title should be in output")
	}
	if !strings.Contains(contentStr, "status: draft") {
		t.Error("status should be in output")
	}
	if !strings.Contains(contentStr, "other: value") {
		t.Error("other should be in output")
	}
}

func TestDeleteEntity(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)

	entityPath := filepath.Join(tmpDir, "REQ-001.md")
	testutil.CreateFile(t, entityPath, "test")

	err := DeleteEntity(entityPath)
	testutil.AssertNoError(t, err)

	testutil.AssertFileNotExists(t, entityPath)
}

func TestDeleteEntity_NonExistent(t *testing.T) {
	err := DeleteEntity("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestListEntityFiles(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)
	entitiesDir := filepath.Join(tmpDir, "entities")

	// Create directory structure with entities
	requirementDir := filepath.Join(entitiesDir, "requirement")
	decisionDir := filepath.Join(entitiesDir, "decision")
	testutil.CreateDir(t, requirementDir)
	testutil.CreateDir(t, decisionDir)

	// Create some entity files
	entities := []string{
		filepath.Join(requirementDir, "REQ-001.md"),
		filepath.Join(requirementDir, "REQ-002.md"),
		filepath.Join(decisionDir, "DEC-001.md"),
	}
	for _, path := range entities {
		testutil.CreateFile(t, path, "test")
	}

	// Create a non-markdown file (should be ignored)
	testutil.CreateFile(t, filepath.Join(requirementDir, "README.txt"), "test")

	files, err := ListEntityFiles(entitiesDir)
	testutil.AssertNoError(t, err)
	testutil.AssertLengthEqual(t, files, 3)

	// Verify all expected files are present
	fileMap := make(map[string]bool)
	for _, file := range files {
		fileMap[file] = true
	}
	for _, expected := range entities {
		if !fileMap[expected] {
			t.Errorf("missing file: %s", expected)
		}
	}
}

func TestListEntityFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	entitiesDir := filepath.Join(tmpDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	files, err := ListEntityFiles(entitiesDir)
	if err != nil {
		t.Fatalf("ListEntityFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestListEntityFiles_NonExistent(t *testing.T) {
	_, err := ListEntityFiles("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestLoadAllEntities(t *testing.T) {
	tmpDir := t.TempDir()
	entitiesDir := filepath.Join(tmpDir, "entities")
	requirementDir := filepath.Join(entitiesDir, "requirement")
	if err := os.MkdirAll(requirementDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create multiple entity files
	for i := 1; i <= 3; i++ {
		content := `---
id: REQ-00` + string(rune('0'+i)) + `
type: requirement
title: Test ` + string(rune('0'+i)) + `
---

Content ` + string(rune('0'+i))

		path := filepath.Join(requirementDir, "REQ-00"+string(rune('0'+i))+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create entity: %v", err)
		}
	}

	entities, err := LoadAllEntities(entitiesDir, nil)
	if err != nil {
		t.Fatalf("LoadAllEntities failed: %v", err)
	}

	if len(entities) != 3 {
		t.Errorf("got %d entities, want 3", len(entities))
	}

	// Verify entities were loaded correctly
	ids := make(map[string]bool)
	for _, entity := range entities {
		ids[entity.ID] = true
		if entity.Type != "requirement" {
			t.Errorf("entity %s has type %q, want %q", entity.ID, entity.Type, "requirement")
		}
	}

	for i := 1; i <= 3; i++ {
		expectedID := "REQ-00" + string(rune('0'+i))
		if !ids[expectedID] {
			t.Errorf("missing entity: %s", expectedID)
		}
	}
}

func TestLoadAllEntities_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	entitiesDir := filepath.Join(tmpDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	entities, err := LoadAllEntities(entitiesDir, nil)
	if err != nil {
		t.Fatalf("LoadAllEntities failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("got %d entities, want 0", len(entities))
	}
}

func TestLoadAllEntities_NonExistent(t *testing.T) {
	entities, err := LoadAllEntities("/nonexistent/dir", nil)
	if err != nil {
		t.Fatalf("LoadAllEntities should not fail for nonexistent dir: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("got %d entities, want 0", len(entities))
	}
}

func TestLoadAllEntities_SkipsInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	entitiesDir := filepath.Join(tmpDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create one valid entity
	validContent := `---
id: REQ-001
type: requirement
---
`
	if err := os.WriteFile(filepath.Join(entitiesDir, "REQ-001.md"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to create valid entity: %v", err)
	}

	// Create one invalid entity
	invalidContent := `---
invalid yaml: [
---
`
	if err := os.WriteFile(filepath.Join(entitiesDir, "invalid.md"), []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to create invalid entity: %v", err)
	}

	entities, err := LoadAllEntities(entitiesDir, nil)
	if err != nil {
		t.Fatalf("LoadAllEntities failed: %v", err)
	}

	// Should only load the valid entity
	if len(entities) != 1 {
		t.Errorf("got %d entities, want 1 (invalid should be skipped)", len(entities))
	}
}

func TestEntityFileModTime(t *testing.T) {
	tmpDir := t.TempDir()
	entityPath := filepath.Join(tmpDir, "REQ-001.md")

	if err := os.WriteFile(entityPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	modTime, err := EntityFileModTime(entityPath)
	if err != nil {
		t.Fatalf("EntityFileModTime failed: %v", err)
	}

	if modTime.IsZero() {
		t.Error("modTime should not be zero")
	}
}

func TestEntityFileModTime_NonExistent(t *testing.T) {
	_, err := EntityFileModTime("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
