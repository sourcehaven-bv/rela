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

// The nested roots reach every directory level: a symlink in apps/a/ cannot
// read apps/b/, which a root at apps/ alone would allow.
func TestDir_SymlinkStaysInItsDirectory(t *testing.T) {
	root := project(t)
	write(t, root, "apps/b/secret.html", "b")
	write(t, root, "apps/a/index.html", "a")
	if err := os.Symlink(filepath.Join("..", "b", "secret.html"), filepath.Join(root, "apps", "a", "leak.html")); err != nil {
		t.Fatal(err)
	}
	d := rootfs.New(root)
	if _, err := d.Load(context.Background(), "apps/a/leak.html"); err == nil {
		t.Fatal("served apps/b through a symlink in apps/a")
	}
	if got, err := d.Load(context.Background(), "apps/a/index.html"); err != nil || string(got) != "a" {
		t.Fatalf("Load(apps/a/index.html) = %q, %v", got, err)
	}
}

func TestDir_Dirs(t *testing.T) {
	root := project(t)
	write(t, root, "apps/b/index.html", "b")
	write(t, root, "apps/a/index.html", "a")
	write(t, root, "apps/file.txt", "x")
	if err := os.Symlink("a", filepath.Join(root, "apps", "link")); err != nil {
		t.Fatal(err)
	}
	d := rootfs.New(root)
	got, err := d.Dirs(context.Background(), "apps")
	if err != nil || !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("Dirs(apps) = %v, %v; want [a b]", got, err)
	}
	if got, err := d.Dirs(context.Background(), "absent"); err != nil || len(got) != 0 {
		t.Fatalf("Dirs(absent) = %v, %v", got, err)
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

// A subscription reports a file that appears or disappears after it began,
// which a watch on the file itself cannot do. A project whose config lives
// in its database has no file at subscribe time. A change to a sibling is
// not reported.
func TestDir_SubscribeFollowsTheName(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, root string)
		change func(t *testing.T, root string)
		want   bool
	}{
		{
			name:   "file created after subscribe",
			change: func(t *testing.T, root string) { write(t, root, "data-entry.yaml", "new") },
			want:   true,
		},
		{
			name:  "file removed after subscribe",
			setup: func(t *testing.T, root string) { write(t, root, "data-entry.yaml", "old") },
			change: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "data-entry.yaml")); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name:   "sibling file changed",
			change: func(t *testing.T, root string) { write(t, root, "acl.yaml", "other") },
			want:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			if tc.setup != nil {
				tc.setup(t, root)
			}
			changed := make(chan struct{}, 1)
			stop, err := rootfs.New(root).Subscribe(context.Background(), "data-entry.yaml", func() {
				select {
				case changed <- struct{}{}:
				default:
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			tc.change(t, root)
			wait := 5 * time.Second
			if !tc.want {
				wait = time.Second
			}
			select {
			case <-changed:
				if !tc.want {
					t.Fatal("notified for a file the subscription does not name")
				}
			case <-time.After(wait):
				if tc.want {
					t.Fatal("no change notification")
				}
			}
		})
	}
}

// Containment is per area, as the readers had it before the loader seam: a
// symlink between subdirectories of scripts/ resolves, a top-level file may
// be a symlink anywhere, and a symlinked area directory is refused.
func TestDir_AreaContainment(t *testing.T) {
	root := project(t)
	write(t, root, "scripts/shared/u.lua", "u")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "shared", "u.lua"), filepath.Join(root, "scripts", "lib", "u.lua")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	write(t, outside, "data-entry.yaml", "shared")
	write(t, outside, "area/x.lua", "x")
	if err := os.Symlink(filepath.Join(outside, "data-entry.yaml"), filepath.Join(root, "data-entry.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "area"), filepath.Join(root, "actions")); err != nil {
		t.Fatal(err)
	}

	d := rootfs.New(root)
	ctx := context.Background()
	if got, err := d.Load(ctx, "scripts/lib/u.lua"); err != nil || string(got) != "u" {
		t.Errorf("Load(scripts/lib/u.lua) = %q, %v; want the file its symlink names", got, err)
	}
	if got, err := d.Load(ctx, "data-entry.yaml"); err != nil || string(got) != "shared" {
		t.Errorf("Load(data-entry.yaml) = %q, %v; want the shared file", got, err)
	}
	if _, err := d.Load(ctx, "actions/x.lua"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Load through a symlinked area = %v; want a containment error", err)
	}
}
