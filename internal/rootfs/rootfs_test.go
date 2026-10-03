package rootfs_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/rootfs"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "schema.yaml", "schema")
	write(t, root, "scripts/a.lua", "a")
	write(t, root, "scripts/sub/b.lua", "b")
	write(t, root, "custom/theme.css", "css")
	return root
}

func TestDir_Load(t *testing.T) {
	d := rootfs.New(project(t))
	ctx := context.Background()
	for name, want := range map[string]string{
		"schema.yaml":       "schema",
		"scripts/a.lua":     "a",
		"scripts/sub/b.lua": "b",
	} {
		got, err := d.Load(ctx, name)
		if err != nil || string(got) != want {
			t.Errorf("Load(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"absent.yaml", "scripts/absent.lua", "nodir/x.lua"} {
		if _, err := d.Load(ctx, name); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Load(%q) err = %v, want ErrNotExist", name, err)
		}
	}
	for _, name := range []string{"", ".", "../x", "/abs", "scripts/../schema.yaml"} {
		if _, err := d.Load(ctx, name); err == nil {
			t.Errorf("Load(%q) accepted an invalid name", name)
		}
	}
	if _, err := d.Load(ctx, "scripts"); err == nil {
		t.Error("Load of a directory succeeded")
	}
}

// A symlink inside a subdirectory that points elsewhere in the project is
// refused by the nested root, and the refusal is not ErrNotExist, so a
// layered loader cannot fall through past it.
func TestDir_NestedRootRefusesSymlinks(t *testing.T) {
	root := project(t)
	if err := os.Symlink(filepath.Join("..", "schema.yaml"), filepath.Join(root, "custom", "leak.css")); err != nil {
		t.Fatal(err)
	}
	_, err := rootfs.New(root).Load(context.Background(), "custom/leak.css")
	if err == nil {
		t.Fatal("served a file through a symlink escaping custom/")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("escape reported as ErrNotExist: %v", err)
	}
}

func TestDir_List(t *testing.T) {
	root := project(t)
	if err := os.Symlink("a.lua", filepath.Join(root, "scripts", "link.lua")); err != nil {
		t.Fatal(err)
	}
	d := rootfs.New(root)
	ctx := context.Background()
	got, err := d.List(ctx, "scripts")
	if err != nil || !slices.Equal(got, []string{"a.lua"}) {
		t.Fatalf("List(scripts) = %v, %v; want [a.lua]", got, err)
	}
	if got, err := d.List(ctx, "absent"); err != nil || len(got) != 0 {
		t.Fatalf("List(absent) = %v, %v; want empty", got, err)
	}
	if _, err := d.List(ctx, "../x"); err == nil {
		t.Fatal("List accepted a traversal name")
	}
}

func TestDir_Stat(t *testing.T) {
	d := rootfs.New(project(t))
	info, err := d.Stat(context.Background(), "custom/theme.css")
	if err != nil || info.Size() != 3 {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	if _, err := d.Stat(context.Background(), "custom/absent.css"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(absent) err = %v", err)
	}
}

func TestDir_Subscribe(t *testing.T) {
	root := project(t)
	changed := make(chan struct{}, 1)
	stop, err := rootfs.New(root).Subscribe(context.Background(), "schema.yaml", func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	write(t, root, "schema.yaml", "edited")
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("no change notification")
	}
	if _, err := rootfs.New(root).Subscribe(context.Background(), "../x", func() {}); err == nil {
		t.Fatal("Subscribe accepted a traversal name")
	}
}
