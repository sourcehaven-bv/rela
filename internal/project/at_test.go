package project

import (
	"errors"
	"path/filepath"
	"testing"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// At resolves a directory AS the project root. These tests pin the difference
// from Discover, which is the whole reason it exists (TKT-SK2QQW).

// The motivating case: a throwaway project inside a real one. Discover walks
// upward and resolves the PARENT — so a capture server seeding fixtures into
// its temp project would write into the user's live database and take its
// single-writer lock. At refuses to ascend, so the temp dir is isolated
// regardless of where TMPDIR points.
func TestAt_DoesNotEscapeIntoAParentProject(t *testing.T) {
	parent := testutil.TempDirWithCleanup(t)
	testutil.CreateFile(t, filepath.Join(parent, SchemaFile), "version: 1.0\n")

	child := filepath.Join(parent, "scratch")
	testutil.CreateDir(t, child)
	testutil.CreateFile(t, filepath.Join(child, SchemaFile), "version: 1.0\n")

	ctx, err := At(child, testProjectFS)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, ctx.Root, child)

	// The contrast that makes the point: Discover on a child with NO schema
	// finds the parent, which is exactly the escape At prevents.
	bare := filepath.Join(parent, "bare")
	testutil.CreateDir(t, bare)

	discovered, err := Discover(bare, testProjectFS)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, discovered.Root, parent)

	if _, err := At(bare, testProjectFS); !errors.Is(err, relaerrors.ErrNoProject) {
		t.Fatalf("At must refuse a directory that is not itself a project, got %v", err)
	}
}

func TestAt_AcceptsBothSchemaNames(t *testing.T) {
	for _, tc := range []struct {
		name         string
		file         string
		wantIsLegacy bool
	}{
		{"new name", SchemaFile, false},
		{"legacy name", LegacySchemaFile, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := testutil.TempDirWithCleanup(t)
			testutil.CreateFile(t, filepath.Join(dir, tc.file), "version: 1.0\n")

			ctx, err := At(dir, testProjectFS)
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, ctx.Root, dir)
			testutil.AssertEqual(t, filepath.Base(ctx.SchemaPath), tc.file)
			testutil.AssertEqual(t, ctx.SchemaIsLegacy, tc.wantIsLegacy)
		})
	}
}

// The `.rela/` marker counts, for the same reason Discover honors it: a
// project created by `rela init` but not yet given a schema is still a project.
func TestAt_AcceptsTheRelaMarker(t *testing.T) {
	dir := testutil.TempDirWithCleanup(t)
	testutil.CreateDir(t, filepath.Join(dir, CacheDir))

	ctx, err := At(dir, testProjectFS)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, ctx.Root, dir)
	testutil.AssertEqual(t, filepath.Base(ctx.SchemaPath), SchemaFile)
}

func TestAt_RejectsANonProjectDirectory(t *testing.T) {
	dir := testutil.TempDirWithCleanup(t)

	if _, err := At(dir, testProjectFS); !errors.Is(err, relaerrors.ErrNoProject) {
		t.Fatalf("want ErrNoProject so callers need no new error handling, got %v", err)
	}
}

// Empty means "no directory given", which cannot be a root. Discover treats it
// as "use the cwd" — the opposite reading, and the wrong one for a caller that
// is supposed to name its root explicitly.
func TestAt_RejectsAnEmptyDirectory(t *testing.T) {
	if _, err := At("", testProjectFS); !errors.Is(err, relaerrors.ErrNoProject) {
		t.Fatalf("want ErrNoProject for an empty dir, got %v", err)
	}
}
