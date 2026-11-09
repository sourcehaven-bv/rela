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

	assertNoTempFiles(t, dir)
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

func TestSafeFS_OnPostWrite_FanOutAndRemove(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewOsFS())

	var aCalls, bCalls int
	removeA := sfs.OnPostWrite(func(_ string, _ []byte) { aCalls++ })
	sfs.OnPostWrite(func(_ string, _ []byte) { bCalls++ })

	path := filepath.Join(dir, "fanout.txt")
	if err := sfs.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if aCalls != 1 || bCalls != 1 {
		t.Errorf("both observers must fire once: a=%d b=%d", aCalls, bCalls)
	}

	// Removing one observer leaves the other installed; a second
	// removal is a harmless no-op.
	removeA()
	removeA()
	if err := sfs.WriteFile(path, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if aCalls != 1 {
		t.Errorf("removed observer still fired: aCalls = %d", aCalls)
	}
	if bCalls != 2 {
		t.Errorf("remaining observer fired %d times, want 2", bCalls)
	}

	// nil installs nothing and its remover does nothing.
	sfs.OnPostWrite(nil)()
	if err := sfs.WriteFile(path, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if bCalls != 3 {
		t.Errorf("nil install disturbed the registry: bCalls = %d", bCalls)
	}
}

// Over anything but an OsFS, SafeFS must write through the wrapped FS and
// leave the disk alone: the atomic path uses os calls, which would put the
// file on disk at the same path (TKT-J4HMZ2B).
func TestSafeFS_WriteFileOverNonDiskFSStaysInMemory(t *testing.T) {
	tests := []struct {
		name string
		wrap func(*MemFS) FS
	}{
		{"memfs", func(m *MemFS) FS { return m }},
		{"safefs over memfs", func(m *MemFS) FS { return NewSafeFS(m) }},
		{"error fs over memfs", func(m *MemFS) FS { return NewErrorFS(m) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mem := NewMemFS()
			sfs := NewSafeFS(tc.wrap(mem))
			var observed []string
			sfs.OnPostWrite(func(p string, _ []byte) { observed = append(observed, p) })

			path := filepath.Join(dir, "sub", "state.json")
			data := []byte("in memory")
			if err := sfs.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			got, err := mem.ReadFile(path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("MemFS ReadFile() = %q, %v; want %q", got, err, data)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("disk directory has %d entries, want none", len(entries))
			}
			if len(observed) != 1 || observed[0] != path {
				t.Errorf("observers saw %v, want [%s]", observed, path)
			}
		})
	}
}

// A SafeFS over a SafeFS over the disk still writes atomically: the outer one
// delegates and the inner one does the temp-file rename.
func TestSafeFS_NestedOverDiskLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	sfs := NewSafeFS(NewSafeFS(NewOsFS()))
	path := filepath.Join(dir, "nested.txt")
	if err := sfs.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "nested.txt" {
		t.Errorf("directory holds %v, want only nested.txt", entries)
	}
}
