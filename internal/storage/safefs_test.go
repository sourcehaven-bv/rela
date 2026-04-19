package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeFS_WriteFile(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	path := filepath.Join(dir, "test.txt")
	data := []byte("hello world")

	if err := sfs.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("ReadFile() = %q, want %q", got, data)
	}
}

func TestSafeFS_WriteFileCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	path := filepath.Join(dir, "sub", "dir", "test.txt")
	data := []byte("nested write")

	if err := sfs.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("ReadFile() = %q, want %q", got, data)
	}
}

func TestSafeFS_WriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	path := filepath.Join(dir, "test.txt")

	// Write initial content
	if err := sfs.WriteFile(path, []byte("initial"), 0644); err != nil {
		t.Fatalf("first WriteFile() error = %v", err)
	}

	// Overwrite
	if err := sfs.WriteFile(path, []byte("updated"), 0644); err != nil {
		t.Fatalf("second WriteFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, []byte("updated")) {
		t.Errorf("ReadFile() = %q, want %q", got, "updated")
	}
}

func TestSafeFS_WriteFileNoTempLeftBehind(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	path := filepath.Join(dir, "test.txt")
	if err := sfs.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Check that .tmp file is cleaned up
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temp file should not exist after successful write")
	}
}

func TestSafeFS_WriteFilePermissions(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	path := filepath.Join(dir, "test.txt")
	if err := sfs.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	// On most POSIX systems, the permission should match (modulo umask)
	perm := info.Mode().Perm()
	if perm&0600 != 0600 {
		t.Errorf("permissions = %o, want at least 0600", perm)
	}
}

func TestSafeFS_DelegatesOtherOperations(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	// Test that Read, Stat, etc. work through the embedded FS
	path := filepath.Join(dir, "delegate.txt")
	if err := sfs.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	data, err := sfs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(data, []byte("test")) {
		t.Errorf("ReadFile() = %q, want %q", data, "test")
	}

	info, err := sfs.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != 4 {
		t.Errorf("Stat().Size() = %d, want 4", info.Size())
	}
}

func TestSafeFS_ImplementsFS(_ *testing.T) {
	// Compile-time check that SafeFS implements FS
	var _ FS = NewSafeFS(NewOsFS())
}

// --- PostWrite observer ---

func TestSafeFS_OnPostWrite_FiresWithBytesOnDisk(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	type call struct {
		path string
		data []byte
	}
	var calls []call
	sfs.OnPostWrite(func(p string, d []byte) {
		// Copy d: the hook contract is "with the bytes that hit disk,"
		// and the caller may recycle the slice.
		cp := make([]byte, len(d))
		copy(cp, d)
		calls = append(calls, call{path: p, data: cp})
	})

	path := filepath.Join(dir, "observed.txt")
	data := []byte("observed bytes")
	if err := sfs.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("observer calls = %d, want exactly 1", len(calls))
	}
	if calls[0].path != path {
		t.Errorf("observer path = %q, want %q", calls[0].path, path)
	}
	if !bytes.Equal(calls[0].data, data) {
		t.Errorf("observer data = %q, want %q", calls[0].data, data)
	}

	// Verify the observed bytes match what actually sits on disk.
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, calls[0].data) {
		t.Errorf("observed bytes differ from on-disk bytes")
	}
}

func TestSafeFS_OnPostWrite_DoesNotFireOnFailedWrite(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	var fired bool
	sfs.OnPostWrite(func(_ string, _ []byte) {
		fired = true
	})

	// Writing to a path inside a non-directory produces a failure
	// during the temp-file open (ENOTDIR). Reliable cross-platform.
	notADir := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(notADir, "child.txt")

	if err := sfs.WriteFile(badPath, []byte("data"), 0o644); err == nil {
		t.Fatal("expected WriteFile to fail inside a non-directory")
	}
	if fired {
		t.Error("observer fired on a failed write; must only fire on successful durable writes")
	}
}

func TestSafeFS_OnPostWrite_NoObserverIsFineByDefault(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	// With no observer installed, WriteFile must still work normally.
	path := filepath.Join(dir, "no-obs.txt")
	if err := sfs.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile without observer: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Errorf("ReadFile = %q, want %q", got, "hello")
	}
}

func TestSafeFS_OnPostWrite_ReplacesPreviousObserver(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	var aCalls, bCalls int
	sfs.OnPostWrite(func(_ string, _ []byte) { aCalls++ })
	sfs.OnPostWrite(func(_ string, _ []byte) { bCalls++ })

	path := filepath.Join(dir, "replaced.txt")
	if err := sfs.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if aCalls != 0 {
		t.Errorf("replaced observer still fired: aCalls = %d", aCalls)
	}
	if bCalls != 1 {
		t.Errorf("current observer fired %d times, want 1", bCalls)
	}

	// nil clears the observer.
	sfs.OnPostWrite(nil)
	if err := sfs.WriteFile(path, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if bCalls != 1 {
		t.Errorf("cleared observer still fired: bCalls = %d", bCalls)
	}
}
