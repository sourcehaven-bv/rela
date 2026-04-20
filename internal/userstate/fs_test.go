package userstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestNewForTest_RoundTrip(t *testing.T) {
	svc := NewForTest(t.TempDir())
	ctx := context.Background()

	if _, err := svc.Get(ctx, "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Get on missing key: want os.ErrNotExist, got %v", err)
	}

	if err := svc.Put(ctx, "ui-state.json", []byte("hello")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := svc.Get(ctx, "ui-state.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("Get = %q, want %q", got, "hello")
	}
}

func TestNewForTest_NestedKey(t *testing.T) {
	svc := NewForTest(t.TempDir())
	ctx := context.Background()
	if err := svc.Put(ctx, "documents/abc.html", []byte("<html/>")); err != nil {
		t.Fatalf("Put nested: %v", err)
	}
	got, err := svc.Get(ctx, "documents/abc.html")
	if err != nil {
		t.Fatalf("Get nested: %v", err)
	}
	if string(got) != "<html/>" {
		t.Errorf("got %q", got)
	}
}

func TestNewForTest_RejectsTraversal(t *testing.T) {
	svc := NewForTest(t.TempDir())
	ctx := context.Background()
	if err := svc.Put(ctx, "../outside", []byte("x")); err == nil {
		t.Fatal("want traversal rejection")
	}
	if _, err := svc.Get(ctx, "../outside"); err == nil {
		t.Fatal("want traversal rejection on Get")
	}
}

func TestNewForTest_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	svc := NewForTest(dir)
	if err := svc.Put(context.Background(), "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("orphan temp file after successful Put: %s", e.Name())
		}
	}
}

func TestNewForTest_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits are not enforced on Windows")
	}
	dir := t.TempDir()
	svc := NewForTest(dir)
	if err := svc.Put(context.Background(), "key", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "key"))
	if err != nil {
		t.Fatal(err)
	}
	const mask os.FileMode = 0o777
	if got := info.Mode() & mask; got&0o077 != 0 {
		t.Errorf("Put wrote world- or group-readable file: mode=%o", got)
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	svc := NewForTest(dir)
	if got := svc.Path("foo"); got != filepath.Join(dir, "foo") {
		t.Errorf("Path = %q, want %q", got, filepath.Join(dir, "foo"))
	}
	if got := svc.Root(); got != dir {
		t.Errorf("Root = %q, want %q", got, dir)
	}
}

func TestLock_SerializesCompoundOps(t *testing.T) {
	// Two goroutines each acquire the lock, read, increment, write,
	// release. Without lockedfile the interleaving would lose one
	// increment; with the lock the sequence is strict.
	dir := t.TempDir()
	svc := NewForTest(dir)
	ctx := context.Background()
	if err := svc.Put(ctx, "counter", []byte("0")); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	const iters = 10
	for range iters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := svc.Lock("counter")
			if err != nil {
				t.Errorf("Lock: %v", err)
				return
			}
			defer func() { _ = unlock() }()

			raw, err := svc.Get(ctx, "counter")
			if err != nil {
				t.Errorf("Get: %v", err)
				return
			}
			var n int
			for _, b := range raw {
				n = n*10 + int(b-'0')
			}
			n++
			var buf [16]byte
			w := len(buf)
			for n > 0 {
				w--
				buf[w] = byte('0' + n%10)
				n /= 10
			}
			if w == len(buf) {
				w--
				buf[w] = '0'
			}
			if err := svc.Put(ctx, "counter", buf[w:]); err != nil {
				t.Errorf("Put: %v", err)
			}
		}()
	}
	wg.Wait()

	raw, err := svc.Get(ctx, "counter")
	if err != nil {
		t.Fatal(err)
	}
	var got int
	for _, b := range raw {
		got = got*10 + int(b-'0')
	}
	if got != iters {
		t.Errorf("counter = %d, want %d (lock did not serialize)", got, iters)
	}
}

func TestNewFSWithRepoID_RejectsBadID(t *testing.T) {
	_, err := NewFSWithRepoID("", "not-a-repo-id")
	if !errors.Is(err, ErrInvalidRepoID) {
		t.Errorf("want ErrInvalidRepoID, got %v", err)
	}
}

func TestNewFSWithRepoID_RejectsOverrideInsideProject(t *testing.T) {
	project := t.TempDir()
	inside := filepath.Join(project, "rela-state")
	if err := os.MkdirAll(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvOverride, inside)

	id := "0123456789abcdef0123456789abcdef"
	_, err := NewFSWithRepoID(project, id)
	if !errors.Is(err, ErrOverrideInsideProject) {
		t.Errorf("want ErrOverrideInsideProject, got %v", err)
	}
}

func TestNewFSWithRepoID_UsesOverride(t *testing.T) {
	base := t.TempDir()
	t.Setenv(EnvOverride, base)

	id := "0123456789abcdef0123456789abcdef"
	svc, err := NewFSWithRepoID("", id)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, productDir, "repos", id)
	if got := svc.Root(); got != want {
		t.Errorf("Root = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("per-repo dir not created: %v", err)
	}
}
