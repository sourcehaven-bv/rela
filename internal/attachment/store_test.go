package attachment

import (
	"bytes"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestStoreAdd(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	// Create a source file
	fs.MkdirAll("/tmp", 0755)
	fs.WriteFile("/tmp/test.png", []byte("fake png data"), 0644)

	// Add the file
	att, err := store.Add("/tmp/test.png", "testuser")
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	// Verify attachment
	if att.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if att.Ext != ".png" {
		t.Errorf("got ext %q, want .png", att.Ext)
	}
	if att.Path == "" {
		t.Error("expected non-empty path")
	}

	// Verify file was copied
	if !store.Exists(att.Path) {
		t.Error("attachment file should exist")
	}

	// Verify metadata
	meta, err := store.GetMetadata(att.Path)
	if err != nil {
		t.Fatalf("GetMetadata error: %v", err)
	}
	if meta.OriginalName != "test.png" {
		t.Errorf("got original name %q, want test.png", meta.OriginalName)
	}
	if meta.Size != 13 { // "fake png data" is 13 bytes
		t.Errorf("got size %d, want 13", meta.Size)
	}
	if meta.AddedBy != "testuser" {
		t.Errorf("got added by %q, want testuser", meta.AddedBy)
	}
}

func TestStoreDeduplication(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	// Create source files with same content
	fs.MkdirAll("/tmp", 0755)
	fs.WriteFile("/tmp/file1.png", []byte("same content"), 0644)
	fs.WriteFile("/tmp/file2.png", []byte("same content"), 0644)

	// Add both files
	att1, err := store.Add("/tmp/file1.png", "user1")
	if err != nil {
		t.Fatalf("Add file1 error: %v", err)
	}

	att2, err := store.Add("/tmp/file2.png", "user2")
	if err != nil {
		t.Fatalf("Add file2 error: %v", err)
	}

	// Should have same hash and path (deduplication)
	if att1.Hash != att2.Hash {
		t.Errorf("hashes should match: %q != %q", att1.Hash, att2.Hash)
	}
	if att1.Path != att2.Path {
		t.Errorf("paths should match: %q != %q", att1.Path, att2.Path)
	}
}

func TestStoreAddBytes(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	data := []byte("test data")
	att, err := store.AddBytes(data, "document.pdf", "testuser")
	if err != nil {
		t.Fatalf("AddBytes error: %v", err)
	}

	if att.Ext != ".pdf" {
		t.Errorf("got ext %q, want .pdf", att.Ext)
	}

	// Verify data can be retrieved
	retrieved, err := store.Get(att.Path)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !bytes.Equal(retrieved, data) {
		t.Errorf("data mismatch: got %q, want %q", retrieved, data)
	}
}

func TestStoreRemove(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	// Add a file
	fs.MkdirAll("/tmp", 0755)
	fs.WriteFile("/tmp/test.txt", []byte("test"), 0644)

	att, err := store.Add("/tmp/test.txt", "")
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	// Verify it exists
	if !store.Exists(att.Path) {
		t.Error("file should exist before removal")
	}

	// Remove it
	if err := store.Remove(att.Path); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	// Verify it's gone
	if store.Exists(att.Path) {
		t.Error("file should not exist after removal")
	}
}

func TestStoreGC(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	// Add files
	fs.MkdirAll("/tmp", 0755)
	fs.WriteFile("/tmp/keep.txt", []byte("keep this"), 0644)
	fs.WriteFile("/tmp/remove.txt", []byte("remove this"), 0644)

	attKeep, err := store.Add("/tmp/keep.txt", "")
	if err != nil {
		t.Fatalf("Add keep error: %v", err)
	}

	attRemove, err := store.Add("/tmp/remove.txt", "")
	if err != nil {
		t.Fatalf("Add remove error: %v", err)
	}

	// Run GC with only attKeep referenced
	result, err := store.GC([]string{attKeep.Path})
	if err != nil {
		t.Fatalf("GC error: %v", err)
	}

	// Should find one unreferenced file
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(result.Removed))
	}
	if len(result.Removed) > 0 && result.Removed[0] != attRemove.Path {
		t.Errorf("wrong file marked for removal: %q", result.Removed[0])
	}

	// Actually remove them
	if err := store.RemoveUnreferenced(result); err != nil {
		t.Fatalf("RemoveUnreferenced error: %v", err)
	}

	// Verify attKeep still exists, attRemove is gone
	if !store.Exists(attKeep.Path) {
		t.Error("kept file should still exist")
	}
	if store.Exists(attRemove.Path) {
		t.Error("removed file should not exist")
	}
}

func TestStoreList(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	// Add files
	fs.MkdirAll("/tmp", 0755)
	fs.WriteFile("/tmp/a.txt", []byte("a"), 0644)
	fs.WriteFile("/tmp/b.txt", []byte("b"), 0644)

	store.Add("/tmp/a.txt", "")
	store.Add("/tmp/b.txt", "")

	paths, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"attachments/ab/ab3f8c2e9d1a5b6c.png", false},
		{"attachments/cd/cd7e2f1b8a4c.pdf", false},
		{"entities/foo/bar.md", true},             // wrong prefix
		{"attachments/ab/short.png", true},        // hash too short
		{"attachments/ab/ab3f8c2e9d1a5b6c", true}, // no extension
	}

	for _, tt := range tests {
		err := ValidatePath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePath(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0B"},
		{100, "100B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{1572864, "1.5MB"},
		{1073741824, "1.0GB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.size)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestStoreNoExtension(t *testing.T) {
	fs := storage.NewMemFS()
	store := NewStore(fs, "/project")

	// File without extension
	fs.MkdirAll("/tmp", 0755)
	fs.WriteFile("/tmp/noext", []byte("data"), 0644)

	att, err := store.Add("/tmp/noext", "")
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	// Should get .bin as default extension
	if att.Ext != ".bin" {
		t.Errorf("got ext %q, want .bin", att.Ext)
	}
}
