package repository

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

const txTestMetamodelYAML = `version: "1.0"
entities:
  task:
    label: Task
    plural: tasks
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
relations:
  depends-on:
    label: Depends On
    from: [task]
    to: [task]
`

func setupTxTestEnv(t *testing.T) (*Repository, *metamodel.Metamodel, storage.FS) {
	t.Helper()
	fs := storage.NewMemFS()

	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/tasks", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(txTestMetamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(txTestMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := New(fs, ctx)
	return repo, meta, fs
}

func TestTransaction_CommitsOnSuccess(t *testing.T) {
	repo, meta, fs := setupTxTestEnv(t)

	entity := testutil.NewEntity("TASK-001", "task").With("title", "Test Task").Build()

	err := repo.Transaction(func(tx Tx) error {
		return tx.WriteEntity(entity, meta)
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}

	// Entity should exist after commit
	if _, readErr := repo.ReadEntity("task", "TASK-001", meta); readErr != nil {
		t.Errorf("entity should exist after commit: %v", readErr)
	}

	// No temp files should remain
	entries, _ := fs.ReadDir("/project/entities/tasks")
	for _, entry := range entries {
		if name := entry.Name(); len(name) > 4 && name[len(name)-4:] == ".new" {
			t.Errorf("temp file should not exist: %s", name)
		}
	}
}

func TestTransaction_RollsBackOnError(t *testing.T) {
	repo, meta, fs := setupTxTestEnv(t)

	entity := testutil.NewEntity("TASK-001", "task").With("title", "Test Task").Build()

	err := repo.Transaction(func(tx Tx) error {
		if writeErr := tx.WriteEntity(entity, meta); writeErr != nil {
			return writeErr
		}
		return errors.New("simulated error")
	})
	if err == nil {
		t.Fatal("Transaction() should return error")
	}

	// Entity should NOT exist after rollback
	if _, readErr := repo.ReadEntity("task", "TASK-001", meta); readErr == nil {
		t.Error("entity should not exist after rollback")
	}

	// No temp files should remain
	entries, _ := fs.ReadDir("/project/entities/tasks")
	for _, entry := range entries {
		if name := entry.Name(); len(name) > 4 && name[len(name)-4:] == ".new" {
			t.Errorf("temp file should not exist after rollback: %s", name)
		}
	}
}

func TestTransaction_MultipleOperations(t *testing.T) {
	repo, meta, _ := setupTxTestEnv(t)

	task1 := testutil.NewEntity("TASK-001", "task").With("title", "Task 1").Build()
	task2 := testutil.NewEntity("TASK-002", "task").With("title", "Task 2").Build()

	err := repo.Transaction(func(tx Tx) error {
		if err := tx.WriteEntity(task1, meta); err != nil {
			return err
		}
		if err := tx.WriteEntity(task2, meta); err != nil {
			return err
		}
		rel := testutil.NewRelation("TASK-001", "depends-on", "TASK-002").Build()
		return tx.WriteRelation(rel)
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}

	// Both entities should exist
	if _, readErr := repo.ReadEntity("task", "TASK-001", meta); readErr != nil {
		t.Errorf("TASK-001 should exist: %v", readErr)
	}
	if _, readErr := repo.ReadEntity("task", "TASK-002", meta); readErr != nil {
		t.Errorf("TASK-002 should exist: %v", readErr)
	}

	// Relation should exist
	if _, readErr := repo.ReadRelation("TASK-001", "depends-on", "TASK-002"); readErr != nil {
		t.Errorf("relation should exist: %v", readErr)
	}
}

func TestTransaction_DeleteOperations(t *testing.T) {
	repo, meta, _ := setupTxTestEnv(t)

	// First create an entity outside transaction
	entity := testutil.NewEntity("TASK-001", "task").With("title", "To Delete").Build()
	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	// Now delete in a transaction
	err := repo.Transaction(func(tx Tx) error {
		return tx.DeleteEntity("task", "TASK-001", meta)
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}

	// Entity should be deleted
	if _, readErr := repo.ReadEntity("task", "TASK-001", meta); readErr == nil {
		t.Error("entity should be deleted after transaction")
	}
}

func TestTransaction_WriteAndDeleteInSameTransaction(t *testing.T) {
	repo, meta, _ := setupTxTestEnv(t)

	// Create an entity outside transaction
	oldEntity := testutil.NewEntity("TASK-001", "task").With("title", "Old").Build()
	if err := repo.WriteEntity(oldEntity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	// Simulate a rename: write new, delete old
	newEntity := testutil.NewEntity("TASK-100", "task").With("title", "Old").Build()

	err := repo.Transaction(func(tx Tx) error {
		if writeErr := tx.WriteEntity(newEntity, meta); writeErr != nil {
			return writeErr
		}
		return tx.DeleteEntity("task", "TASK-001", meta)
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}

	// Old entity should be gone
	if _, readErr := repo.ReadEntity("task", "TASK-001", meta); readErr == nil {
		t.Error("old entity should be deleted")
	}

	// New entity should exist
	if _, readErr := repo.ReadEntity("task", "TASK-100", meta); readErr != nil {
		t.Errorf("new entity should exist: %v", readErr)
	}
}

func TestFindOrphanedTempFiles_NoOrphans(t *testing.T) {
	repo, _, _ := setupTxTestEnv(t)

	orphaned, err := repo.FindOrphanedTempFiles()
	if err != nil {
		t.Fatalf("FindOrphanedTempFiles() error = %v", err)
	}

	if len(orphaned) != 0 {
		t.Errorf("expected no orphans, got %d", len(orphaned))
	}
}

func TestFindOrphanedTempFiles_FindsOrphans(t *testing.T) {
	repo, _, fs := setupTxTestEnv(t)

	// Create some orphaned .new files
	_ = fs.WriteFile("/project/entities/tasks/TASK-001.md.new", []byte("orphan1"), 0o644)
	_ = fs.WriteFile("/project/relations/TASK-001--depends-on--TASK-002.md.new", []byte("orphan2"), 0o644)

	orphaned, err := repo.FindOrphanedTempFiles()
	if err != nil {
		t.Fatalf("FindOrphanedTempFiles() error = %v", err)
	}

	if len(orphaned) != 2 {
		t.Errorf("expected 2 orphans, got %d: %v", len(orphaned), orphaned)
	}
}

func TestCleanupOrphanedTempFiles(t *testing.T) {
	repo, _, fs := setupTxTestEnv(t)

	// Create orphaned .new files
	_ = fs.WriteFile("/project/entities/tasks/TASK-001.md.new", []byte("orphan"), 0o644)

	count, err := repo.CleanupOrphanedTempFiles()
	if err != nil {
		t.Fatalf("CleanupOrphanedTempFiles() error = %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 cleaned, got %d", count)
	}

	// Verify file is gone
	if _, err := fs.ReadFile("/project/entities/tasks/TASK-001.md.new"); err == nil {
		t.Error("orphaned file should be removed")
	}
}
