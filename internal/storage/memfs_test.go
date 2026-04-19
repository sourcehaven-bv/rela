package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestMemFS_Getwd(t *testing.T) {
	m := storage.NewMemFS()

	cwd, err := m.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if cwd != "/" {
		t.Errorf("default cwd = %q, want %q", cwd, "/")
	}

	m.SetCwd("/home/test")
	cwd, err = m.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if cwd != "/home/test" {
		t.Errorf("cwd = %q, want %q", cwd, "/home/test")
	}
}

func TestMemFS_WriteFileDataIsolation(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/dir", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write original data.
	original := []byte("hello")
	if err := m.WriteFile("/dir/test.txt", original, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Mutate the slice we passed in.
	original[0] = 'X'

	// Read should return original data, not mutated.
	got, err := m.ReadFile("/dir/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q (data should be isolated from caller mutation)", got, "hello")
	}

	// Mutate the returned slice.
	got[0] = 'Y'

	// Read again should still return original data.
	got2, err := m.ReadFile("/dir/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got2) != "hello" {
		t.Errorf("got %q, want %q (returned data should be isolated)", got2, "hello")
	}
}

func TestMemFS_RemoveEmptyDir(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/a/b", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Removing empty leaf directory should work.
	if err := m.Remove("/a/b"); err != nil {
		t.Fatalf("Remove empty dir: %v", err)
	}

	_, err := m.Stat("/a/b")
	if !os.IsNotExist(err) {
		t.Errorf("expected dir to be gone, got: %v", err)
	}
}

func TestMemFS_RemoveNonEmptyDirFails(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/a/b", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("/a/b/file.txt", []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := m.Remove("/a/b")
	if err == nil {
		t.Fatal("expected error when removing non-empty directory")
	}
}

func TestMemFS_RenameDir(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/old/sub", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("/old/file.txt", []byte("root"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.WriteFile("/old/sub/deep.txt", []byte("deep"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := m.Rename("/old", "/new"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Old paths should not exist.
	if _, err := m.Stat("/old"); !os.IsNotExist(err) {
		t.Errorf("old dir should not exist, got: %v", err)
	}

	// New paths should exist.
	got, err := m.ReadFile("/new/file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "root" {
		t.Errorf("got %q, want %q", got, "root")
	}

	got, err = m.ReadFile("/new/sub/deep.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "deep" {
		t.Errorf("got %q, want %q", got, "deep")
	}

	// Subdirectory should exist.
	info, err := m.Stat("/new/sub")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestMemFS_WalkSkipDir(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/root/skip/deep", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.MkdirAll("/root/keep", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("/root/skip/deep/file.txt", []byte("skip"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.WriteFile("/root/keep/file.txt", []byte("keep"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var walked []string
	err := m.Walk("/root", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "skip" {
			return filepath.SkipDir
		}
		walked = append(walked, path)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Should not contain any paths under /root/skip.
	for _, p := range walked {
		if len(p) >= 10 && p[:10] == "/root/skip" {
			t.Errorf("should have skipped %q", p)
		}
	}

	// Should contain /root/keep/file.txt.
	found := false
	for _, p := range walked {
		if p == "/root/keep/file.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected /root/keep/file.txt in walked paths: %v", walked)
	}
}

func TestMemFS_ReadDirEmpty(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/empty", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	entries, err := m.ReadDir("/empty")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestMemFS_WriteReadEmptyFile(t *testing.T) {
	m := storage.NewMemFS()
	if err := m.MkdirAll("/dir", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := m.WriteFile("/dir/empty.txt", []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := m.ReadFile("/dir/empty.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(got))
	}
}

func TestMemFS_OnPostWriteFiresWithOnDiskBytes(t *testing.T) {
	m := storage.NewMemFS()
	var calls []struct {
		path string
		data []byte
	}
	m.OnPostWrite(func(p string, d []byte) {
		cp := make([]byte, len(d))
		copy(cp, d)
		calls = append(calls, struct {
			path string
			data []byte
		}{path: p, data: cp})
	})

	if err := m.WriteFile("/obs.txt", []byte("observed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("observer calls = %d, want 1", len(calls))
	}
	if string(calls[0].data) != "observed" {
		t.Errorf("observer got %q, want %q", calls[0].data, "observed")
	}

	// Replace with nil clears the observer.
	m.OnPostWrite(nil)
	if err := m.WriteFile("/obs2.txt", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Errorf("cleared observer fired: calls = %d", len(calls))
	}
}

func TestMemFS_WriteFileExternalSkipsObserver(t *testing.T) {
	m := storage.NewMemFS()
	var fired bool
	m.OnPostWrite(func(_ string, _ []byte) { fired = true })

	if err := m.WriteFileExternal("/ext.txt", []byte("external edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fired {
		t.Error("WriteFileExternal must NOT fire the observer")
	}

	// Observer should still be active for subsequent normal WriteFile.
	if err := m.WriteFile("/internal.txt", []byte("self edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fired {
		t.Error("observer should fire for regular WriteFile after WriteFileExternal")
	}
}

func TestMemFS_WriteFileExternalFailurePreservesObserver(t *testing.T) {
	m := storage.NewMemFS()
	obs := func(_ string, _ []byte) {}
	m.OnPostWrite(obs)

	// Write to a missing parent fails (no MkdirAll). Observer must still
	// be installed after the failure.
	if err := m.WriteFileExternal("/never/exists/path.txt", []byte("x"), 0o644); err == nil {
		t.Fatal("expected error writing to missing parent")
	}
	// Install a marker and verify it sticks (implies restoration happened).
	var fired bool
	m.OnPostWrite(func(_ string, _ []byte) { fired = true })
	if err := m.WriteFile("/ok.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fired {
		t.Error("observer set after WriteFileExternal failure did not fire")
	}
}
