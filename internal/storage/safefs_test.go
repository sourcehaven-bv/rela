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
