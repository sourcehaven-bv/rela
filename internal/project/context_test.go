package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/errors"
)

func TestDiscover(t *testing.T) {
	t.Run("finds project by metamodel.yaml", func(t *testing.T) {
		// Create a temporary directory structure
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir", "nested")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create metamodel.yaml in root
		metamodelPath := filepath.Join(tmpDir, MetamodelFile)
		if err := os.WriteFile(metamodelPath, []byte("version: 1.0\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Discover from nested directory
		ctx, err := Discover(subDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if ctx.Root != tmpDir {
			t.Errorf("expected root %s, got %s", tmpDir, ctx.Root)
		}
	})

	t.Run("finds project by .rela directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create .rela directory in root
		relaDir := filepath.Join(tmpDir, CacheDir)
		if err := os.MkdirAll(relaDir, 0755); err != nil {
			t.Fatal(err)
		}

		ctx, err := Discover(subDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if ctx.Root != tmpDir {
			t.Errorf("expected root %s, got %s", tmpDir, ctx.Root)
		}
	})

	t.Run("uses current directory when startDir is empty", func(t *testing.T) {
		// Save current directory
		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(originalWd)

		// Create temp directory with metamodel
		tmpDir := t.TempDir()
		// Resolve symlinks (important on macOS where /tmp -> /private/tmp)
		tmpDir, err = filepath.EvalSymlinks(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		metamodelPath := filepath.Join(tmpDir, MetamodelFile)
		if err := os.WriteFile(metamodelPath, []byte("version: 1.0\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Change to temp directory
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		ctx, err := Discover("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if ctx.Root != tmpDir {
			t.Errorf("expected root %s, got %s", tmpDir, ctx.Root)
		}
	})

	t.Run("returns error when no project found", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := Discover(tmpDir)
		if err != errors.ErrNoProject {
			t.Errorf("expected ErrNoProject, got %v", err)
		}
	})

	t.Run("handles invalid path", func(t *testing.T) {
		// Test with path that contains null byte - this should fail in Abs()
		_, err := Discover("/tmp/\x00invalid")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

func TestNewContext(t *testing.T) {
	root := "/test/project"
	ctx := newContext(root)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Root", ctx.Root, root},
		{"MetamodelPath", ctx.MetamodelPath, filepath.Join(root, MetamodelFile)},
		{"CacheDir", ctx.CacheDir, filepath.Join(root, CacheDir)},
		{"CachePath", ctx.CachePath, filepath.Join(root, CacheDir, CacheFile)},
		{"EntitiesDir", ctx.EntitiesDir, filepath.Join(root, EntitiesDir)},
		{"RelationsDir", ctx.RelationsDir, filepath.Join(root, RelationsDir)},
		{"TemplatesDir", ctx.TemplatesDir, filepath.Join(root, TemplatesDir)},
		{"EntityTemplatesDir", ctx.EntityTemplatesDir, filepath.Join(root, TemplatesDir, EntityTemplatesDir)},
		{"RelationTemplatesDir", ctx.RelationTemplatesDir, filepath.Join(root, TemplatesDir, RelationTemplatesDir)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("expected %s, got %s", tt.want, tt.got)
			}
		})
	}
}

func TestContextInitialize(t *testing.T) {
	t.Run("creates directories successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := newContext(tmpDir)

		if err := ctx.Initialize(); err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Check that directories were created
		dirs := []string{ctx.CacheDir, ctx.EntitiesDir, ctx.RelationsDir}
		for _, dir := range dirs {
			info, err := os.Stat(dir)
			if err != nil {
				t.Errorf("directory %s not created: %v", dir, err)
			} else if !info.IsDir() {
				t.Errorf("%s is not a directory", dir)
			}
		}
	})

	t.Run("handles error when creating cache directory", func(t *testing.T) {
		// Create context with invalid root (file instead of directory)
		tmpFile, err := os.CreateTemp("", "testfile")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		ctx := newContext(tmpFile.Name())
		err = ctx.Initialize()
		if err == nil {
			t.Error("expected error when creating directories under a file")
		}
	})

	t.Run("handles error when creating entities directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := newContext(tmpDir)

		// Create .rela successfully first
		if err := os.MkdirAll(ctx.CacheDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create entities as a file (not directory) to cause error
		if err := os.WriteFile(ctx.EntitiesDir, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		err := ctx.Initialize()
		if err == nil {
			t.Error("expected error when entities path is a file")
		}
	})

	t.Run("handles error when creating relations directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := newContext(tmpDir)

		// Create .rela and entities successfully
		if err := os.MkdirAll(ctx.CacheDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(ctx.EntitiesDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create relations as a file (not directory) to cause error
		if err := os.WriteFile(ctx.RelationsDir, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		err := ctx.Initialize()
		if err == nil {
			t.Error("expected error when relations path is a file")
		}
	})
}

func TestContextEntityTypeDir(t *testing.T) {
	ctx := newContext("/test")

	t.Run("simple pluralization", func(t *testing.T) {
		got := ctx.EntityTypeDir("requirement")
		want := filepath.Join("/test", EntitiesDir, "requirements")
		if got != want {
			t.Errorf("expected %s, got %s", want, got)
		}
	})
}

func TestContextEntityTypeDirWithPlural(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityTypeDirWithPlural("decisions")
	want := filepath.Join("/test", EntitiesDir, "decisions")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextEntityFilePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityFilePath("requirement", "REQ-001")
	want := filepath.Join("/test", EntitiesDir, "requirements", "REQ-001.md")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextEntityFilePathWithPlural(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityFilePathWithPlural("requirements", "REQ-001")
	want := filepath.Join("/test", EntitiesDir, "requirements", "REQ-001.md")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextRelationFilePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.RelationFilePath("REQ-001", "satisfies", "DEC-001")
	want := filepath.Join("/test", RelationsDir, "REQ-001--satisfies--DEC-001.md")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextExists(t *testing.T) {
	t.Run("returns true when metamodel exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := newContext(tmpDir)

		// Create metamodel.yaml
		if err := os.WriteFile(ctx.MetamodelPath, []byte("version: 1.0\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if !ctx.Exists() {
			t.Error("expected Exists() to return true")
		}
	})

	t.Run("returns false when metamodel does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := newContext(tmpDir)

		if ctx.Exists() {
			t.Error("expected Exists() to return false")
		}
	})
}

func TestContextEntityTemplatePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityTemplatePath("requirement")
	want := filepath.Join("/test", TemplatesDir, EntityTemplatesDir, "requirement.md")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextRelationTemplatePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.RelationTemplatePath("satisfies")
	want := filepath.Join("/test", TemplatesDir, RelationTemplatesDir, "satisfies.md")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
