package config_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newFSView(t *testing.T, loader config.Loader) fs.FS {
	t.Helper()
	v, err := config.NewFSView(loader)
	if err != nil {
		t.Fatalf("NewFSView: %v", err)
	}
	return v
}

func TestFSView_ReadFile(t *testing.T) {
	t.Parallel()
	want := []byte("steps:\n  - kind: rename\n")
	v := newFSView(t, mapLoader{"migrations/001.yaml": want})

	got, err := fs.ReadFile(v, "migrations/001.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ReadFile = %q, want %q", got, want)
	}
}

func TestFSView_ReadFile_MissingIsNotExist(t *testing.T) {
	t.Parallel()
	v := newFSView(t, mapLoader{})

	// datamigration.LoadDir distinguishes a missing directory from an
	// unreadable one with errors.Is(err, fs.ErrNotExist), so the adapter must
	// preserve that rather than flattening every failure into one error.
	_, err := fs.ReadFile(v, "migrations/nope.yaml")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %v is not fs.ErrNotExist-compatible", err)
	}
}

func TestFSView_ReadDir(t *testing.T) {
	t.Parallel()
	v := newFSView(t, mapLoader{
		"migrations/002.yaml": nil,
		"migrations/001.yaml": nil,
		"scripts/other.lua":   nil,
	})

	entries, err := fs.ReadDir(v, "migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("entry %q reported as a directory; the Loader lists files only", e.Name())
		}
		names = append(names, e.Name())
	}
	want := []string{"001.yaml", "002.yaml"}
	if !slices.Equal(names, want) {
		t.Errorf("ReadDir = %v, want %v (sorted, scoped to the named directory)", names, want)
	}
}

func TestFSView_Open(t *testing.T) {
	t.Parallel()
	want := []byte("kind: lua\n")
	v := newFSView(t, mapLoader{"migrations/003.yaml": want})

	f, err := v.Open("migrations/003.yaml")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Open+ReadAll = %q, want %q", got, want)
	}

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != int64(len(want)) {
		t.Errorf("Size = %d, want %d", info.Size(), len(want))
	}
	if info.IsDir() {
		t.Error("a file reported IsDir")
	}
	// Deliberately the zero time rather than time.Now(): a fabricated mtime
	// would make a consumer comparing it treat every read as freshly modified.
	if !info.ModTime().IsZero() {
		t.Errorf("ModTime = %v, want the zero time", info.ModTime())
	}
}

func TestFSView_RejectsInvalidPaths(t *testing.T) {
	t.Parallel()
	v := newFSView(t, mapLoader{})
	for _, name := range []string{"", "../escape.yaml", "/absolute.yaml", "./dot.yaml"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := fs.ReadFile(v, name); err == nil {
				t.Errorf("ReadFile(%q) should be rejected", name)
			}
			if _, err := fs.ReadDir(v, name); err == nil {
				t.Errorf("ReadDir(%q) should be rejected", name)
			}
		})
	}
}

func TestNewFSView_RejectsNil(t *testing.T) {
	t.Parallel()
	v, err := config.NewFSView(nil)
	if err == nil {
		t.Fatal("NewFSView(nil) should be rejected")
	}
	if v != nil {
		t.Errorf("NewFSView returned %v alongside an error, want nil", v)
	}
}

// TestFSView_OverFSLoader is the end-to-end shape the migration commands use:
// a real directory, read through the Loader, seen as an fs.FS.
func TestFSView_OverFSLoader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(root+"/migrations", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/migrations/001.yaml", []byte("a: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := newFSView(t, config.NewFSLoader(storage.NewOsFS(), root))

	entries, err := fs.ReadDir(v, "migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "001.yaml" {
		t.Fatalf("ReadDir = %v, want [001.yaml]", entries)
	}
	got, err := fs.ReadFile(v, "migrations/001.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "a: 1\n" {
		t.Errorf("ReadFile = %q", got)
	}

	// A project with no migrations/ at all lists empty rather than erroring —
	// the contract datamigration.LoadDir relies on.
	empty, err := fs.ReadDir(newFSView(t, config.NewFSLoader(storage.NewOsFS(), t.TempDir())), "migrations")
	if err != nil {
		t.Fatalf("ReadDir of a project with no migrations/: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ReadDir = %v, want empty", empty)
	}
}

func TestFSView_RootOpensAndListsEmpty(t *testing.T) {
	t.Parallel()
	v := newFSView(t, mapLoader{"migrations/001.yaml": nil})

	// The root must OPEN: fs.WalkDir and fs.Glob open "." before anything
	// else, and an fs.FS whose root cannot be opened is malformed.
	f, err := v.Open(".")
	if err != nil {
		t.Fatalf(`Open("."): %v`, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("the root did not report IsDir")
	}

	// It lists EMPTY: Loader has no enumerate-everything capability, so a root
	// listing could only be invented. The documented consequence is that a
	// walk from the root finds nothing — asserted so it stays a decision
	// rather than becoming a surprise.
	var walked []string
	if err := fs.WalkDir(v, ".", func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != "." {
			walked = append(walked, p)
		}
		return nil
	}); err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if len(walked) != 0 {
		t.Errorf("WalkDir found %v; an FSView is not walkable from the root", walked)
	}
}

func TestFSView_MissingFileIsNotExist(t *testing.T) {
	t.Parallel()
	v := newFSView(t, config.NewFSLoader(storage.NewOsFS(), t.TempDir()))

	// Open of a nonexistent path must report ErrNotExist, not succeed.
	//
	// It used to fall back to a directory listing when Load said "not found",
	// which returned a successful EMPTY DIRECTORY for a missing file — because
	// List reports an absent directory as an empty list and so cannot tell
	// "empty" from "does not exist". An fs.FS that opens a missing path
	// surfaces as a bizarre failure in whatever wraps it, rather than as the
	// plain "file not found" the caller asked for.
	f, err := v.Open("migrations/typo.yaml")
	if err == nil {
		_ = f.Close()
		t.Fatal("Open of a missing file succeeded; want an error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %v is not fs.ErrNotExist-compatible", err)
	}

	// fs.ReadFile falls back to Open for an FS that is not a ReadFileFS, so
	// the same must hold there.
	if _, err := fs.ReadFile(v, "migrations/typo.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile error %v is not fs.ErrNotExist-compatible", err)
	}
}

func TestFSView_ReadDir_SurfacesLoaderErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("permission denied")
	v := newFSView(t, errLoader{err: boom})

	// datamigration.LoadDir treats ONLY ErrNotExist as "no migrations" and
	// surfaces everything else, deliberately: an unreadable migrations/
	// reported as "nothing to migrate" would let a chain silently go unrun.
	// The adapter must not flatten a real error into that.
	if _, err := fs.ReadDir(v, "migrations"); !errors.Is(err, boom) {
		t.Errorf("ReadDir error = %v, want the loader's error surfaced", err)
	}
	if _, err := fs.ReadFile(v, "migrations/001.yaml"); !errors.Is(err, boom) {
		t.Errorf("ReadFile error = %v, want the loader's error surfaced", err)
	}
}

func TestFSView_ReadDir_NotExistFromListListsEmpty(t *testing.T) {
	t.Parallel()
	// FSLoader reports an absent directory as an empty list, but a Loader that
	// reports it as ErrNotExist — the obvious alternative, and what Load is
	// documented to do — must work too. Otherwise a database-backed Loader
	// would fail the whole chain on a project that simply has no migrations.
	v := newFSView(t, errLoader{err: fs.ErrNotExist})

	entries, err := fs.ReadDir(v, "migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ReadDir = %v, want empty", entries)
	}
}

func TestFSView_DirEntryInfoReportsNoSize(t *testing.T) {
	t.Parallel()
	v := newFSView(t, mapLoader{"migrations/001.yaml": []byte("a long migration file")})

	entries, err := fs.ReadDir(v, "migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	// A listing carries names, not sizes. Reporting 0 would be a WRONG number
	// rather than an absent one — the trap the zero ModTime avoids — so Info
	// reports the metadata as unavailable instead.
	if _, err := entries[0].Info(); err == nil {
		t.Error("Info() returned metadata; a listing cannot know a size")
	}
}

func TestFSView_WithContext(t *testing.T) {
	t.Parallel()
	base, err := config.NewFSView(mapLoader{"a.yaml": []byte("x")})
	if err != nil {
		t.Fatalf("NewFSView: %v", err)
	}

	// Nil is rejected rather than silently replaced, so a caller who meant to
	// pass a context finds out here.
	//nolint:staticcheck // passing nil is the case under test
	if _, nilErr := base.WithContext(nil); nilErr == nil {
		t.Error("WithContext(nil) should be rejected")
	}

	bound, err := base.WithContext(context.Background())
	if err != nil {
		t.Fatalf("WithContext: %v", err)
	}
	if _, err := fs.ReadFile(bound, "a.yaml"); err != nil {
		t.Errorf("ReadFile on a ctx-bound view: %v", err)
	}
}
