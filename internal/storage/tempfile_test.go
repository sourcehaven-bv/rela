package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTempPath_SameDirUniqueAndRecognized(t *testing.T) {
	target := filepath.Join("some", "dir", "REQ-1.md")
	a, b := TempPath(target), TempPath(target)
	if a == b {
		t.Errorf("two temp paths are equal: %q", a)
	}
	for _, p := range []string{a, b} {
		if filepath.Dir(p) != filepath.Dir(target) {
			t.Errorf("temp path %q is not in the target's directory", p)
		}
		if !strings.HasPrefix(filepath.Base(p), "REQ-1.md.") || !strings.HasSuffix(p, ".tmp") {
			t.Errorf("temp path %q is not of the form <base>.<random>.tmp", p)
		}
		if !IsTempName(p) {
			t.Errorf("IsTempName(%q) = false", p)
		}
	}
}

// A temp name adds a random part and ".tmp" to the base. For a base near
// the 255-byte name limit the base is shortened, on a character boundary.
func TestTempPath_LongNameFitsLimit(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "ascii at limit", base: strings.Repeat("a", 252) + ".md"},
		{name: "multibyte at limit", base: "a" + strings.Repeat("é", 125) + ".md"},
		{name: "short", base: "A--rel--B.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := TempPath(filepath.Join("dir", tc.base))
			name := filepath.Base(p)
			if len(name) > maxNameBytes {
				t.Errorf("temp name is %d bytes, limit %d", len(name), maxNameBytes)
			}
			if !utf8.ValidString(name) {
				t.Errorf("temp name %q is not valid UTF-8", name)
			}
			if !IsTempName(name) {
				t.Errorf("IsTempName(%q) = false", name)
			}
		})
	}
}

func TestSafeFS_WriteFileLongName(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())
	path := filepath.Join(dir, strings.Repeat("a", 252)+".md")
	if err := sfs.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile with a 255-byte name: %v", err)
	}
	assertNoTempFiles(t, dir)
}

func TestIsTempName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"REQ-1.md.0123456789abcdef.tmp", true},
		{"REQ-1.md.tmp", true},
		{"REQ-1.md.new", true},
		{"REQ-1.md", false},
		{"theme/logo.png", false},
		{"tmp", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTempName(tc.name); got != tc.want {
				t.Errorf("IsTempName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestIsStaleTemp(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		modTime time.Time
		want    bool
	}{
		{"just written", now, false},
		{"inside grace period", now.Add(-TempStaleAfter / 2), false},
		{"past grace period", now.Add(-TempStaleAfter - time.Second), true},
		{"future mtime from clock skew", now.Add(time.Hour), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStaleTemp(tc.modTime, now); got != tc.want {
				t.Errorf("IsStaleTemp = %v, want %v", got, tc.want)
			}
		})
	}
}

// AC1: concurrent writers of one path never share a temp file, so the
// final file is always one writer's complete bytes and no write fails.
// Before unique names both writers truncated and wrote the same
// "<path>.tmp", and one renamed the other's partial bytes into place or
// failed its rename because the other had already moved the file.
func TestSafeFS_WriteFileConcurrentSamePath(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())
	path := filepath.Join(dir, "REQ-1.md")

	// Large enough that a write takes several syscalls' worth of time.
	inputs := [][]byte{
		bytes.Repeat([]byte("a"), 64<<10),
		bytes.Repeat([]byte("b"), 32<<10),
	}

	for i := range 40 {
		var wg sync.WaitGroup
		errs := make([]error, len(inputs))
		for w, data := range inputs {
			wg.Go(func() { errs[w] = sfs.WriteFile(path, data, 0o644) })
		}
		wg.Wait()
		for w, err := range errs {
			if err != nil {
				t.Fatalf("iteration %d: writer %d failed: %v", i, w, err)
			}
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, inputs[0]) && !bytes.Equal(got, inputs[1]) {
			t.Fatalf("iteration %d: file is %d bytes and matches neither input", i, len(got))
		}
	}
	assertNoTempFiles(t, dir)
}

// AC2: the file gets the requested mode with the umask applied, as any
// newly created file does. The umask is read off a probe file, so the
// test does not change process state.
func TestSafeFS_WriteFileModeHonoursUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	dir := t.TempDir()

	probe := filepath.Join(dir, "probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY, 0o777)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	info, err := os.Stat(probe)
	if err != nil {
		t.Fatal(err)
	}
	umask := 0o777 &^ info.Mode().Perm()

	path := filepath.Join(dir, "f.md")
	if err = NewSafeFS(NewOsFS()).WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := os.FileMode(0o644) &^ umask; info.Mode().Perm() != want {
		t.Errorf("mode = %o, want %o (0644 with umask %o)", info.Mode().Perm(), want, umask)
	}
}

// AC4: a write whose rename fails removes its temp file.
func TestSafeFS_WriteFileRenameFailureLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file in the target directory keeps the rename from replacing it.
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := NewSafeFS(NewOsFS()).WriteFile(target, []byte("data"), 0o644); err == nil {
		t.Fatal("expected the rename over a directory to fail")
	}
	assertNoTempFiles(t, dir)
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if IsTempName(e.Name()) {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
