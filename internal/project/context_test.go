package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestDiscover(t *testing.T) {
	t.Run("finds project by metamodel.yaml", func(t *testing.T) {
		// Create a temporary directory structure
		tmpDir := testutil.TempDirWithCleanup(t)
		subDir := filepath.Join(tmpDir, "subdir", "nested")
		testutil.CreateDir(t, subDir)

		// Create metamodel.yaml in root
		metamodelPath := filepath.Join(tmpDir, MetamodelFile)
		testutil.CreateFile(t, metamodelPath, "version: 1.0\n")

		// Discover from nested directory
		ctx, err := Discover(subDir)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, ctx.Root, tmpDir)
	})

	t.Run("finds project by .rela directory", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)
		subDir := filepath.Join(tmpDir, "subdir")
		testutil.CreateDir(t, subDir)

		// Create .rela directory in root
		relaDir := filepath.Join(tmpDir, CacheDir)
		testutil.CreateDir(t, relaDir)

		ctx, err := Discover(subDir)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, ctx.Root, tmpDir)
	})

	t.Run("uses current directory when startDir is empty", func(t *testing.T) {
		// Create temp directory with metamodel
		tmpDir := testutil.TempDirWithCleanup(t)
		metamodelPath := filepath.Join(tmpDir, MetamodelFile)
		testutil.CreateFile(t, metamodelPath, "version: 1.0\n")

		// Change to temp directory
		cleanup := testutil.ChangeDir(t, tmpDir)
		defer cleanup()

		// Resolve symlinks (important on macOS where /tmp -> /private/tmp)
		tmpDir, evalErr := filepath.EvalSymlinks(tmpDir)
		testutil.AssertNoError(t, evalErr)

		ctx, err := Discover("")
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, ctx.Root, tmpDir)
	})

	t.Run("returns error when no project found", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)

		_, err := Discover(tmpDir)
		if !errors.Is(err, relaerrors.ErrNoProject) {
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
		tmpDir := testutil.TempDirWithCleanup(t)
		ctx := newContext(tmpDir)

		err := ctx.Initialize()
		testutil.AssertNoError(t, err)

		// Check that directories were created
		dirs := []string{ctx.CacheDir, ctx.EntitiesDir, ctx.RelationsDir}
		for _, dir := range dirs {
			testutil.AssertIsDir(t, dir)
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
		tmpDir := testutil.TempDirWithCleanup(t)
		ctx := newContext(tmpDir)

		// Create .rela successfully first
		testutil.CreateDir(t, ctx.CacheDir)

		// Create entities as a file (not directory) to cause error
		testutil.CreateFile(t, ctx.EntitiesDir, "test")

		err := ctx.Initialize()
		testutil.AssertError(t, err)
	})

	t.Run("handles error when creating relations directory", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)
		ctx := newContext(tmpDir)

		// Create .rela and entities successfully
		testutil.CreateDir(t, ctx.CacheDir)
		testutil.CreateDir(t, ctx.EntitiesDir)

		// Create relations as a file (not directory) to cause error
		testutil.CreateFile(t, ctx.RelationsDir, "test")

		err := ctx.Initialize()
		testutil.AssertError(t, err)
	})
}

func TestContextEntityTypeDir(t *testing.T) {
	ctx := newContext("/test")

	t.Run("simple pluralization", func(t *testing.T) {
		got := ctx.EntityTypeDir("requirement")
		want := "/test/" + EntitiesDir + "/requirements"
		if got != want {
			t.Errorf("expected %s, got %s", want, got)
		}
	})
}

func TestContextEntityTypeDirWithPlural(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityTypeDirWithPlural("decisions")
	want := "/test/" + EntitiesDir + "/decisions"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextEntityFilePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityFilePath("requirement", "REQ-001")
	want := "/test/" + EntitiesDir + "/requirements/REQ-001.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextEntityFilePathWithPlural(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityFilePathWithPlural("requirements", "REQ-001")
	want := "/test/" + EntitiesDir + "/requirements/REQ-001.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextRelationFilePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.RelationFilePath("REQ-001", "satisfies", "DEC-001")
	want := "/test/" + RelationsDir + "/REQ-001--satisfies--DEC-001.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextExists(t *testing.T) {
	t.Run("returns true when metamodel exists", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)
		ctx := newContext(tmpDir)

		// Create metamodel.yaml
		testutil.CreateFile(t, ctx.MetamodelPath, "version: 1.0\n")

		if !ctx.Exists() {
			t.Error("expected Exists() to return true")
		}
	})

	t.Run("returns false when metamodel does not exist", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)
		ctx := newContext(tmpDir)

		if ctx.Exists() {
			t.Error("expected Exists() to return false")
		}
	})
}

func TestContextEntityTemplatePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.EntityTemplatePath("requirement")
	want := "/test/" + TemplatesDir + "/" + EntityTemplatesDir + "/requirement.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextRelationTemplatePath(t *testing.T) {
	ctx := newContext("/test")

	got := ctx.RelationTemplatePath("satisfies")
	want := "/test/" + TemplatesDir + "/" + RelationTemplatesDir + "/satisfies.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
