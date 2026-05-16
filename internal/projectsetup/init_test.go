package projectsetup_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestInitializeWithFS_CreatesProject(t *testing.T) {
	fs := storage.NewMemFS()
	target := "/proj"

	result, err := projectsetup.InitializeWithFS(target, fs)
	if err != nil {
		t.Fatalf("InitializeWithFS: %v", err)
	}

	if result.Root != target {
		t.Errorf("Root = %q, want %q", result.Root, target)
	}
	want := filepath.Join(target, project.MetamodelFile)
	if result.MetamodelPath != want {
		t.Errorf("MetamodelPath = %q, want %q", result.MetamodelPath, want)
	}

	data, err := fs.ReadFile(want)
	if err != nil {
		t.Fatalf("metamodel not written: %v", err)
	}
	if len(data) == 0 {
		t.Error("metamodel.yaml is empty")
	}
}

func TestInitializeWithFS_AlreadyInitialized(t *testing.T) {
	fs := storage.NewMemFS()
	target := "/proj"

	if _, err := projectsetup.InitializeWithFS(target, fs); err != nil {
		t.Fatalf("first init: %v", err)
	}
	_, err := projectsetup.InitializeWithFS(target, fs)
	if err == nil {
		t.Fatal("expected error on re-init, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitializeWithFS_UpdatesGitignore(t *testing.T) {
	fs := storage.NewMemFS()
	target := "/proj"

	if err := fs.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(target, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("seed gitignore: %v", err)
	}

	result, err := projectsetup.InitializeWithFS(target, fs)
	if err != nil {
		t.Fatalf("InitializeWithFS: %v", err)
	}
	if !result.GitignoreUpdate {
		t.Error("expected GitignoreUpdate=true")
	}

	data, _ := fs.ReadFile(filepath.Join(target, ".gitignore"))
	if !strings.Contains(string(data), ".rela") {
		t.Errorf("gitignore not updated: %q", data)
	}
}
