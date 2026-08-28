package project

import (
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// TestDiscoverSchemaFileName pins the dual-name resolution introduced when
// metamodel.yaml was renamed to schema.yaml (TKT-FNARO6). The resolved path
// matters as much as the root: in-place writers (rename-type, migrate) rewrite
// ctx.SchemaPath, so resolving to the wrong name would make them write a second
// schema file rather than editing the existing one.
func TestDiscoverSchemaFileName(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		wantBase     string
		wantIsLegacy bool
	}{
		{
			name:     "new name only",
			files:    []string{SchemaFile},
			wantBase: SchemaFile,
		},
		{
			name:         "legacy name only",
			files:        []string{LegacySchemaFile},
			wantBase:     LegacySchemaFile,
			wantIsLegacy: true,
		},
		{
			// Mid-migration: schema.yaml is the file the user just wrote, so
			// prefer it rather than erroring on an unambiguous intent.
			name:     "both present prefers new name",
			files:    []string{SchemaFile, LegacySchemaFile},
			wantBase: SchemaFile,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := testutil.TempDirWithCleanup(t)
			for _, f := range tc.files {
				testutil.CreateFile(t, filepath.Join(tmpDir, f), "version: 1.0\n")
			}

			ctx, err := Discover(tmpDir, testProjectFS)
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, ctx.Root, tmpDir)
			testutil.AssertEqual(t, filepath.Base(ctx.SchemaPath), tc.wantBase)
			testutil.AssertEqual(t, ctx.SchemaIsLegacy, tc.wantIsLegacy)
		})
	}
}

// TestDiscoverNearestRootWins guards the per-level (rather than two-pass)
// name check. Two separate walks — all of schema.yaml, then all of
// metamodel.yaml — would climb past a legacy-named child to a new-named
// parent and silently open the wrong project.
func TestDiscoverNearestRootWins(t *testing.T) {
	parent := testutil.TempDirWithCleanup(t)
	testutil.CreateFile(t, filepath.Join(parent, SchemaFile), "version: 1.0\n")

	child := filepath.Join(parent, "child")
	testutil.CreateDir(t, child)
	testutil.CreateFile(t, filepath.Join(child, LegacySchemaFile), "version: 1.0\n")

	ctx, err := Discover(child, testProjectFS)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, ctx.Root, child)
	testutil.AssertEqual(t, filepath.Base(ctx.SchemaPath), LegacySchemaFile)
	testutil.AssertEqual(t, ctx.SchemaIsLegacy, true)
}

// TestDiscoverRelaMarkerWithoutSchemaFile documents the .rela/ fallback's
// behavior rather than changing it (RR-JANQG7). A stray .rela/ shadows a real
// parent project because the marker is checked before ascending. What this
// pins is the empty state: SchemaPath must still name the PREFERRED file, so
// the downstream "missing schema" error tells the operator what to create.
func TestDiscoverRelaMarkerWithoutSchemaFile(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)
	testutil.CreateDir(t, filepath.Join(tmpDir, CacheDir))

	ctx, err := Discover(tmpDir, testProjectFS)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, filepath.Base(ctx.SchemaPath), SchemaFile)
	testutil.AssertEqual(t, ctx.SchemaIsLegacy, false)
}

// TestSchemaFileAtIgnoresDirectories ensures a directory named schema.yaml is
// not mistaken for a project marker.
func TestSchemaFileAtIgnoresDirectories(t *testing.T) {
	tmpDir := testutil.TempDirWithCleanup(t)
	testutil.CreateDir(t, filepath.Join(tmpDir, SchemaFile))

	if _, _, found := SchemaFileAt(tmpDir, testProjectFS); found {
		t.Error("a directory named schema.yaml must not count as a schema file")
	}
}

// TestExistsAcceptsLegacyName is the guard behind the init refusal: Exists must
// report true for a legacy project, or project creation would treat it as an
// empty directory and write a default schema.yaml over the top of it.
func TestExistsAcceptsLegacyName(t *testing.T) {
	for _, name := range []string{SchemaFile, LegacySchemaFile} {
		t.Run(name, func(t *testing.T) {
			tmpDir := testutil.TempDirWithCleanup(t)
			testutil.CreateFile(t, filepath.Join(tmpDir, name), "version: 1.0\n")

			ctx := newContext(tmpDir)
			if !ctx.Exists(testProjectFS) {
				t.Errorf("Exists() = false for a project holding %s", name)
			}
		})
	}
}
