package configsql_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config/configsql"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// The config.Loader conformance is DECLARED in configsql.go (var _ ...), not
// asserted here: this package may import config, so the compiler checks it
// directly and no test is needed to stand in for a type system.

// newLoader opens a database and returns a Loader over it.
//
// It goes through sqlitedb.Open rather than sql.Open because that is what
// creates the project_files table and stamps the schema version — owning the
// FILE is sqlitedb's job, while reading config out of it is this package's.
// Only the *sql.DB crosses the boundary.
func newLoader(t *testing.T) (*configsql.Loader, context.Context) {
	t.Helper()
	ctx := context.Background()
	conn, err := sqlitedb.Open(ctx, sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "cfg.db"),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	l, err := configsql.New(conn.DB())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l, ctx
}

func TestLoader_RoundTrip(t *testing.T) {
	l, ctx := newLoader(t)
	want := []byte("entity_types:\n  - ticket\n")

	if err := l.Put(ctx, "schema.yaml", want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := l.Load(ctx, "schema.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Load = %q, want %q", got, want)
	}

	// Put replaces rather than duplicating: config is loaded as a set.
	if putErr := l.Put(ctx, "schema.yaml", []byte("replaced")); putErr != nil {
		t.Fatalf("Put again: %v", putErr)
	}
	got, err = l.Load(ctx, "schema.yaml")
	if err != nil {
		t.Fatalf("Load after replace: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("Load after replace = %q, want %q", got, "replaced")
	}
}

func TestLoader_MissingIsNotExist(t *testing.T) {
	l, ctx := newLoader(t)

	// A layered loader falls through to the next source on exactly this error
	// and nothing else, so a different one here would make a baked-in file
	// shadow the one on disk.
	if _, err := l.Load(ctx, "absent.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %v is not fs.ErrNotExist-compatible", err)
	}
}

func TestLoader_List(t *testing.T) {
	l, ctx := newLoader(t)
	for _, p := range []string{
		"scripts/zeta.lua",
		"scripts/alpha.lua",
		"scripts/nested/deep.lua",
		"templates/other.md",
		"scriptsnotadir.yaml",
	} {
		if err := l.Put(ctx, p, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", p, err)
		}
	}

	got, err := l.List(ctx, "scripts")
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

func TestLoader_ListAbsentDirIsEmpty(t *testing.T) {
	l, ctx := newLoader(t)

	// A project with no scripts/ is ordinary, not an error — the same
	// asymmetry the filesystem loader has, and the one datamigration.LoadDir
	// relies on to tell "no migrations" from "unreadable migrations".
	got, err := l.List(ctx, "scripts")
	if err != nil {
		t.Fatalf("List of an absent directory: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want empty", got)
	}
}

func TestLoader_ListTreatsDirAsLiteral(t *testing.T) {
	l, ctx := newLoader(t)
	for _, p := range []string{"a_b/one.lua", "axb/two.lua"} {
		if err := l.Put(ctx, p, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", p, err)
		}
	}

	// A LIKE- or GLOB-based implementation would treat the caller's dir as a
	// pattern, so "a_b" would also match "axb" ('_' being LIKE's
	// single-character wildcard). The literal prefix comparison keeps a
	// directory name a directory name.
	got, err := l.List(ctx, "a_b")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.Equal(got, []string{"one.lua"}) {
		t.Errorf("List = %v, want [one.lua]; the directory must be matched literally", got)
	}
}

func TestLoader_Paths(t *testing.T) {
	l, ctx := newLoader(t)
	for _, p := range []string{"schema.yaml", "scripts/a.lua", "acl.yaml"} {
		if err := l.Put(ctx, p, []byte("x")); err != nil {
			t.Fatalf("Put %s: %v", p, err)
		}
	}

	got, err := l.Paths(ctx)
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	want := []string{"acl.yaml", "schema.yaml", "scripts/a.lua"}
	if !slices.Equal(got, want) {
		t.Errorf("Paths = %v, want %v", got, want)
	}
}

func TestLoader_RejectsUnsafeNames(t *testing.T) {
	l, ctx := newLoader(t)

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
			if _, err := l.Load(ctx, tc.input); err == nil {
				t.Errorf("Load(%q) should be rejected", tc.input)
			}
			if _, err := l.List(ctx, tc.input); err == nil {
				t.Errorf("List(%q) should be rejected", tc.input)
			}
			if err := l.Put(ctx, tc.input, nil); err == nil {
				t.Errorf("Put(%q) should be rejected", tc.input)
			}
		})
	}
}
