package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

var testProjectFS = storage.NewOsFS()

func TestDiscover(t *testing.T) {
	t.Run("finds project by legacy metamodel.yaml", func(t *testing.T) {
		// Create a temporary directory structure
		tmpDir := testutil.TempDirWithCleanup(t)
		subDir := filepath.Join(tmpDir, "subdir", "nested")
		testutil.CreateDir(t, subDir)

		// Create metamodel.yaml in root
		metamodelPath := filepath.Join(tmpDir, LegacySchemaFile)
		testutil.CreateFile(t, metamodelPath, "version: 1.0\n")

		// Discover from nested directory
		ctx, err := Discover(subDir, testProjectFS)
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

		ctx, err := Discover(subDir, testProjectFS)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, ctx.Root, tmpDir)
	})

	t.Run("uses current directory when startDir is empty", func(t *testing.T) {
		// Create temp directory with metamodel
		tmpDir := testutil.TempDirWithCleanup(t)
		metamodelPath := filepath.Join(tmpDir, LegacySchemaFile)
		testutil.CreateFile(t, metamodelPath, "version: 1.0\n")

		// Change to temp directory
		cleanup := testutil.ChangeDir(t, tmpDir)
		defer cleanup()

		// Resolve symlinks (important on macOS where /tmp -> /private/tmp)
		tmpDir, evalErr := filepath.EvalSymlinks(tmpDir)
		testutil.AssertNoError(t, evalErr)

		ctx, err := Discover("", testProjectFS)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, ctx.Root, tmpDir)
	})

	t.Run("returns error when no project found", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)

		_, err := Discover(tmpDir, testProjectFS)
		if !errors.Is(err, relaerrors.ErrNoProject) {
			t.Errorf("expected ErrNoProject, got %v", err)
		}
	})

	t.Run("handles invalid path", func(t *testing.T) {
		// Note: The null byte test only works reliably on Linux.
		// On macOS, filepath.Abs doesn't fail with null bytes in the path.
		// We skip this test on non-Linux platforms for reliability.
		if runtime.GOOS != "linux" {
			t.Skip("null byte path handling differs by platform")
		}
		// Test with path that contains null byte - this should fail in Abs()
		_, err := Discover("/tmp/\x00invalid", testProjectFS)
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
		{"SchemaPath", ctx.SchemaPath, filepath.Join(root, SchemaFile)},
		{"CacheDir", ctx.CacheDir, filepath.Join(root, CacheDir)},
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

		err := ctx.Initialize(testProjectFS)
		testutil.AssertNoError(t, err)

		// Check that directories were created
		dirs := []string{ctx.CacheDir, ctx.EntitiesDir, ctx.RelationsDir}
		for _, dir := range dirs {
			testutil.AssertIsDir(t, dir)
		}
	})

	t.Run("handles error when creating cache directory", func(t *testing.T) {
		// Create context with invalid root (file instead of directory)
		tmpFile, err := os.CreateTemp(t.TempDir(), "testfile")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		ctx := newContext(tmpFile.Name())
		err = ctx.Initialize(testProjectFS)
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

		err := ctx.Initialize(testProjectFS)
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

		err := ctx.Initialize(testProjectFS)
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
		testutil.CreateFile(t, ctx.SchemaPath, "version: 1.0\n")

		if !ctx.Exists(testProjectFS) {
			t.Error("expected Exists() to return true")
		}
	})

	t.Run("returns false when metamodel does not exist", func(t *testing.T) {
		tmpDir := testutil.TempDirWithCleanup(t)
		ctx := newContext(tmpDir)

		if ctx.Exists(testProjectFS) {
			t.Error("expected Exists() to return false")
		}
	})
}

func TestContextEntityTemplatePath(t *testing.T) {
	ctx := newContext("/test")

	got, err := ctx.EntityTemplatePath("requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/test/" + TemplatesDir + "/" + EntityTemplatesDir + "/requirement.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestContextEntityTemplateVariantPath(t *testing.T) {
	ctx := newContext("/test")
	dir := "/test/" + TemplatesDir + "/" + EntityTemplatesDir

	t.Run("empty variant falls back to the type template", func(t *testing.T) {
		got, err := ctx.EntityTemplateVariantPath("requirement", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := dir + "/requirement.md"; got != want {
			t.Errorf("expected %s, got %s", want, got)
		}
	})

	t.Run("variant", func(t *testing.T) {
		got, err := ctx.EntityTemplateVariantPath("requirement", "detailed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := dir + "/requirement--detailed.md"; got != want {
			t.Errorf("expected %s, got %s", want, got)
		}
	})
}

// TestContextTemplatePaths_TypeNameBlocklist pins the local guard on TYPE
// names and, just as importantly, what it does NOT reject. Type names are
// metamodel names, and metamodel.ValidateSchemaName documents that shipped
// schemas legitimately use dashes, dots, internal spaces and non-ASCII — an
// allowlist here would break templates (including the DEFAULT template) for
// such a type. Only what could leave the directory is refused.
func TestContextTemplatePaths_TypeNameBlocklist(t *testing.T) {
	ctx := newContext("/test")
	accepted := []string{"requirement", "review-response", "some property", "v1.2", "café", "user_story"}
	rejected := []string{"", ".", "..", "../etc", "a/b", "a\\b", "a\x00b", " lead", "trail ", "tab\tbed"}

	for _, name := range accepted {
		t.Run("accept/"+name, func(t *testing.T) {
			if _, err := ctx.EntityTemplatePath(name); err != nil {
				t.Errorf("EntityTemplatePath(%q): %v (a valid metamodel name must be accepted)", name, err)
			}
			if _, err := ctx.EntityTemplateVariantPath(name, "detailed"); err != nil {
				t.Errorf("EntityTemplateVariantPath(%q, detailed): %v", name, err)
			}
			if _, err := ctx.RelationTemplatePath(name); err != nil {
				t.Errorf("RelationTemplatePath(%q): %v", name, err)
			}
		})
	}
	for _, name := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			if _, err := ctx.EntityTemplatePath(name); err == nil {
				t.Errorf("EntityTemplatePath(%q) = nil error, want rejection", name)
			}
			if _, err := ctx.EntityTemplateVariantPath(name, "detailed"); err == nil {
				t.Errorf("EntityTemplateVariantPath(%q, detailed) = nil error, want rejection", name)
			}
			if _, err := ctx.EntityTemplateVariantPath(name, ""); err == nil {
				t.Errorf("EntityTemplateVariantPath(%q, \"\") = nil error, want rejection", name)
			}
			if _, err := ctx.RelationTemplatePath(name); err == nil {
				t.Errorf("RelationTemplatePath(%q) = nil error, want rejection", name)
			}
		})
	}
}

// TestContextEntityTemplateVariantPath_VariantAllowlist pins the VARIANT half,
// which is an identifier allowlist kept identical to
// automation.isValidTemplateName. The variant reaches here from automation's
// {{new.kind}} interpolation — an API-settable entity property — and the
// joined path goes to a raw storage.FS that validates nothing.
func TestContextEntityTemplateVariantPath_VariantAllowlist(t *testing.T) {
	ctx := newContext("/test")
	for _, v := range []string{"detailed", "bug-report", "v2_draft", "A1"} {
		if _, err := ctx.EntityTemplateVariantPath("requirement", v); err != nil {
			t.Errorf("variant %q: %v, want accepted", v, err)
		}
	}
	for _, v := range []string{"../../../../etc/passwd", "..", "a/b", "a\\b", "a\x00b", "a b", "a.b", "café"} {
		if _, err := ctx.EntityTemplateVariantPath("requirement", v); err == nil {
			t.Errorf("variant %q = nil error, want rejection", v)
		}
	}
	// Empty variant is the documented "no variant" case and falls back to the
	// type template.
	got, err := ctx.EntityTemplateVariantPath("requirement", "")
	if err != nil {
		t.Fatalf("empty variant: %v", err)
	}
	if want := "/test/" + TemplatesDir + "/" + EntityTemplatesDir + "/requirement.md"; got != want {
		t.Errorf("empty variant = %s, want %s", got, want)
	}
}

func TestContextRelationTemplatePath(t *testing.T) {
	ctx := newContext("/test")

	got, err := ctx.RelationTemplatePath("satisfies")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/test/" + TemplatesDir + "/" + RelationTemplatesDir + "/satisfies.md"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
