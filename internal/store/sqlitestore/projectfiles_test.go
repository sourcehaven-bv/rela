package sqlitestore_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TestProjectFilesSatisfiesConfigLoader is the whole point of the type: it
// must be usable as a config.Loader WITHOUT sqlitestore importing config
// (arch-lint forbids a store depending on an application package). The match
// is therefore structural, and nothing but this assertion proves it holds —
// renaming a method or changing a signature would otherwise fail far away, at
// the wiring site, with a confusing message.
func TestProjectFilesSatisfiesConfigLoader(t *testing.T) {
	var _ config.Loader = (*sqlitestore.ProjectFiles)(nil)
}

func newProjectFiles(t *testing.T) (*sqlitestore.ProjectFiles, context.Context) {
	t.Helper()
	ctx := context.Background()
	conn, err := sqlitestore.Connect(ctx, sqlitestore.Options{
		Path: filepath.Join(t.TempDir(), "cfg.db"),
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn.ProjectFiles(), ctx
}

func TestProjectFiles_RoundTrip(t *testing.T) {
	pf, ctx := newProjectFiles(t)
	want := []byte("entity_types:\n  - ticket\n")

	if err := pf.Put(ctx, "schema.yaml", want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := pf.Load(ctx, "schema.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Load = %q, want %q", got, want)
	}

	// Put replaces rather than duplicating: config is loaded as a set.
	if err := pf.Put(ctx, "schema.yaml", []byte("replaced")); err != nil {
		t.Fatalf("Put again: %v", err)
	}
	got, err = pf.Load(ctx, "schema.yaml")
	if err != nil {
		t.Fatalf("Load after replace: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("Load after replace = %q, want %q", got, "replaced")
	}
}

func TestProjectFiles_MissingIsNotExist(t *testing.T) {
	pf, ctx := newProjectFiles(t)

	// A layered loader falls through to the next source on exactly this error
	// and nothing else, so a different one here would make a baked-in file
	// shadow the one on disk.
	if _, err := pf.Load(ctx, "absent.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %v is not fs.ErrNotExist-compatible", err)
	}
}

func TestProjectFiles_List(t *testing.T) {
	pf, ctx := newProjectFiles(t)
	for _, p := range []string{
		"scripts/zeta.lua",
		"scripts/alpha.lua",
		"scripts/nested/deep.lua",
		"templates/other.md",
		"scriptsnotadir.yaml",
	} {
		if err := pf.Put(ctx, p, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", p, err)
		}
	}

	got, err := pf.List(ctx, "scripts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Sorted, scoped to the directory, non-recursive — and "scriptsnotadir"
	// must not match, which a bare prefix comparison without the separator
	// would get wrong.
	want := []string{"alpha.lua", "zeta.lua"}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestProjectFiles_ListAbsentDirIsEmpty(t *testing.T) {
	pf, ctx := newProjectFiles(t)

	// A project with no scripts/ is ordinary, not an error — the same
	// asymmetry the filesystem loader has, and the one datamigration.LoadDir
	// relies on to tell "no migrations" from "unreadable migrations".
	got, err := pf.List(ctx, "scripts")
	if err != nil {
		t.Fatalf("List of an absent directory: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want empty", got)
	}
}

func TestProjectFiles_ListTreatsDirAsLiteral(t *testing.T) {
	pf, ctx := newProjectFiles(t)
	for _, p := range []string{"a_b/one.lua", "axb/two.lua"} {
		if err := pf.Put(ctx, p, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", p, err)
		}
	}

	// A LIKE- or GLOB-based implementation would treat the caller's dir as a
	// pattern, so "a_b" would also match "axb" ('_' being LIKE's
	// single-character wildcard). The literal prefix comparison keeps a
	// directory name a directory name.
	got, err := pf.List(ctx, "a_b")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.Equal(got, []string{"one.lua"}) {
		t.Errorf("List = %v, want [one.lua]; the directory must be matched literally", got)
	}
}

func TestProjectFiles_Paths(t *testing.T) {
	pf, ctx := newProjectFiles(t)
	for _, p := range []string{"schema.yaml", "scripts/a.lua", "acl.yaml"} {
		if err := pf.Put(ctx, p, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", p, err)
		}
	}

	got, err := pf.Paths(ctx)
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	want := []string{"acl.yaml", "schema.yaml", "scripts/a.lua"}
	if !slices.Equal(got, want) {
		t.Errorf("Paths = %v, want %v", got, want)
	}
}

func TestProjectFiles_RejectsUnsafeNames(t *testing.T) {
	pf, ctx := newProjectFiles(t)

	// The two backends must accept and reject exactly the same set. A name
	// that works on disk but fails once baked in would break a project at
	// load time that was fine the day before.
	for _, tc := range []struct{ name, input string }{
		{"empty", ""},
		{"NUL", "a\x00b.yaml"},
		{"control character", "a\x1fb.yaml"},
		{"backslash", `sub\file.yaml`},
		{"absolute", "/secret.yaml"},
		{"parent traversal", "../secret.yaml"},
		{"embedded traversal", "sub/../../secret.yaml"},
		{"dot segment", "./file.yaml"},
		{"empty segment", "sub//file.yaml"},
		{"drive letter", "C:secret.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pf.Load(ctx, tc.input); err == nil {
				t.Errorf("Load(%q) should be rejected", tc.input)
			}
			if _, err := pf.List(ctx, tc.input); err == nil {
				t.Errorf("List(%q) should be rejected", tc.input)
			}
			if err := pf.Put(ctx, tc.input, nil); err == nil {
				t.Errorf("Put(%q) should be rejected", tc.input)
			}
		})
	}
}
