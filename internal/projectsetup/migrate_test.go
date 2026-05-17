package projectsetup_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestDetectMigrationsWithFS_NoProject(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := projectsetup.DetectMigrationsWithFS("/missing", fs)
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
}

func TestMigrateWithFS_NoProject(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := projectsetup.MigrateWithFS("/missing", fs)
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
}

func TestDetectMigrationsWithFS_RunsSuccessfully(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := projectsetup.DetectMigrationsWithFS(root, fs); err != nil {
		t.Fatalf("DetectMigrationsWithFS: %v", err)
	}
}

func TestMigrateWithFS_AppliesPending(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := projectsetup.MigrateWithFS(root, fs)
	if err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}

	after, err := projectsetup.DetectMigrationsWithFS(root, fs)
	if err != nil {
		t.Fatalf("DetectMigrationsWithFS after migrate: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected no pending migrations after Migrate, got %d", len(after))
	}
}
