package markdown

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestLoadSyncData(t *testing.T) {
	entity1 := testutil.NewEntity("REQ-001", "requirement").
		WithTitle("Requirement 1").
		Build()
	entity2 := testutil.NewEntity("REQ-002", "requirement").
		WithTitle("Requirement 2").
		Build()
	entity3 := testutil.NewEntity("DEC-001", "decision").
		WithTitle("Decision 1").
		Build()

	rel1 := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
	rel2 := testutil.NewRelation("DEC-001", "addresses", "REQ-002").Build()

	meta := testutil.NewMetamodel().
		WithEntity("requirement", "Requirement", []string{"REQ-"}).
		WithEntity("decision", "Decision", []string{"DEC-"}).
		WithRelation("addresses", "Addresses", []string{"decision"}, []string{"requirement"}).
		Build()

	testCtx, _, _ := testutil.NewProject(t).
		WithMetamodel(meta).
		WithEntity(entity1).
		WithEntity(entity2).
		WithEntity(entity3).
		WithRelation(rel1).
		WithRelation(rel2).
		Build()

	ctx := &project.Context{
		Root:         testCtx.Root,
		EntitiesDir:  testCtx.EntitiesDir,
		RelationsDir: testCtx.RelationsDir,
		CacheDir:     testCtx.CacheDir,
	}

	data, err := testIO.LoadSyncData(ctx, meta)
	testutil.AssertNoError(t, err)

	if len(data.Entities) != 3 {
		t.Errorf("got %d entities, want 3", len(data.Entities))
	}
	if len(data.Relations) != 2 {
		t.Errorf("got %d relations, want 2", len(data.Relations))
	}
	if len(data.Conflicted) != 0 {
		t.Errorf("got %d conflicted, want 0", len(data.Conflicted))
	}
}

func TestLoadSyncData_EmptyDirectories(t *testing.T) {
	testCtx, meta, _ := testutil.NewProject(t).Build()

	ctx := &project.Context{
		Root:         testCtx.Root,
		EntitiesDir:  testCtx.EntitiesDir,
		RelationsDir: testCtx.RelationsDir,
		CacheDir:     testCtx.CacheDir,
	}

	data, err := testIO.LoadSyncData(ctx, meta)
	testutil.AssertNoError(t, err)

	if len(data.Entities) != 0 {
		t.Errorf("got %d entities, want 0", len(data.Entities))
	}
	if len(data.Relations) != 0 {
		t.Errorf("got %d relations, want 0", len(data.Relations))
	}
	if len(data.Conflicted) != 0 {
		t.Errorf("got %d conflicted, want 0", len(data.Conflicted))
	}
}

func TestLoadSyncData_LoadEntitiesError(t *testing.T) {
	memFS := storage.NewMemFS()
	errFS := storage.NewErrorFS(memFS)
	errFS.WalkError = errors.New("permission denied")

	errorIO := NewFileIO(errFS)

	ctx := &project.Context{
		Root:         "/test",
		EntitiesDir:  "/test/entities",
		RelationsDir: "/test/relations",
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement"},
		},
	}

	_, err := errorIO.LoadSyncData(ctx, meta)
	if err == nil {
		t.Error("expected error from LoadSyncData when LoadAllEntities fails")
	}
	if err.Error() != "permission denied" {
		t.Errorf("error = %q, want 'permission denied'", err.Error())
	}
}

func TestLoadSyncData_ConflictedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	reqDir := filepath.Join(ctx.EntitiesDir, "requirement")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	// Valid entity
	validContent := `---
id: REQ-001
type: requirement
title: Valid Entity
---
`
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-001.md"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to create valid entity: %v", err)
	}

	// Conflicted entity
	conflictedContent := `---
id: REQ-002
type: requirement
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature-branch
---
`
	conflictedEntityPath := filepath.Join(reqDir, "REQ-002.md")
	if err := os.WriteFile(conflictedEntityPath, []byte(conflictedContent), 0644); err != nil {
		t.Fatalf("failed to create conflicted entity: %v", err)
	}

	// Valid relation
	relationContent := `---
from: REQ-001
relation: depends-on
to: REQ-001
---
`
	if err := os.WriteFile(filepath.Join(ctx.RelationsDir, "REQ-001--depends-on--REQ-001.md"), []byte(relationContent), 0644); err != nil {
		t.Fatalf("failed to create relation: %v", err)
	}

	// Conflicted relation
	conflictedRelationContent := `---
from: REQ-001
relation: addresses
<<<<<<< HEAD
to: REQ-002
=======
to: REQ-003
>>>>>>> feature-branch
---
`
	conflictedRelationPath := filepath.Join(ctx.RelationsDir, "REQ-001--addresses--conflict.md")
	if err := os.WriteFile(conflictedRelationPath, []byte(conflictedRelationContent), 0644); err != nil {
		t.Fatalf("failed to create conflicted relation: %v", err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {Label: "Requirement"},
		},
	}

	data, err := testIO.LoadSyncData(ctx, meta)
	if err != nil {
		t.Fatalf("LoadSyncData failed: %v", err)
	}

	// Should load 1 valid entity
	if len(data.Entities) != 1 {
		t.Errorf("got %d entities, want 1", len(data.Entities))
	}

	// Should load 1 valid relation
	if len(data.Relations) != 1 {
		t.Errorf("got %d relations, want 1", len(data.Relations))
	}

	// Should track 2 conflicted files (1 entity + 1 relation)
	if len(data.Conflicted) != 2 {
		t.Errorf("got %d conflicted files, want 2", len(data.Conflicted))
	}

	conflictedMap := make(map[string]bool)
	for _, path := range data.Conflicted {
		conflictedMap[path] = true
	}
	if !conflictedMap[conflictedEntityPath] {
		t.Errorf("missing conflicted entity: %s", conflictedEntityPath)
	}
	if !conflictedMap[conflictedRelationPath] {
		t.Errorf("missing conflicted relation: %s", conflictedRelationPath)
	}
}

func TestSyncError_Error(t *testing.T) {
	err := &model.SyncError{
		File:    "/path/to/file.md",
		Message: "something went wrong",
	}

	expected := "/path/to/file.md: something went wrong"
	if got := err.Error(); got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}
