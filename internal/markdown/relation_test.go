package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestReadRelation(t *testing.T) {
	tmpDir := t.TempDir()

	relationContent := `---
from: DEC-001
relation: addresses
to: REQ-001
rationale: Because it makes sense
---
`

	relationPath := filepath.Join(tmpDir, "DEC-001--addresses--REQ-001.md")
	if err := os.WriteFile(relationPath, []byte(relationContent), 0644); err != nil {
		t.Fatalf("failed to write test relation: %v", err)
	}

	relation, err := testIO.ReadRelation(relationPath)
	if err != nil {
		t.Fatalf("ReadRelation failed: %v", err)
	}

	if relation.From != "DEC-001" {
		t.Errorf("From = %q, want %q", relation.From, "DEC-001")
	}
	if relation.Type != "addresses" {
		t.Errorf("Type = %q, want %q", relation.Type, "addresses")
	}
	if relation.To != "REQ-001" {
		t.Errorf("To = %q, want %q", relation.To, "REQ-001")
	}
	if relation.FilePath != relationPath {
		t.Errorf("FilePath = %q, want %q", relation.FilePath, relationPath)
	}
	if relation.ModTime.IsZero() {
		t.Error("ModTime should not be zero")
	}
	if relation.Properties["rationale"] != "Because it makes sense" {
		t.Errorf("rationale = %v, want %q", relation.Properties["rationale"], "Because it makes sense")
	}
}

func TestReadRelation_InvalidFile(t *testing.T) {
	_, err := testIO.ReadRelation("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadRelation_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	invalidContent := `---
from: DEC-001
relation: [invalid
---
`

	relationPath := filepath.Join(tmpDir, "invalid.md")
	if err := os.WriteFile(relationPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write test relation: %v", err)
	}

	_, err := testIO.ReadRelation(relationPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestWriteRelation(t *testing.T) {
	tmpDir := t.TempDir()

	relation := &model.Relation{
		From: "DEC-001",
		Type: "addresses",
		To:   "REQ-001",
		Properties: map[string]interface{}{
			"rationale": "Because it makes sense",
			"impact":    "high",
		},
	}

	relationPath := filepath.Join(tmpDir, "relations", "DEC-001--addresses--REQ-001.md")

	err := testIO.WriteRelation(relation, relationPath)
	if err != nil {
		t.Fatalf("WriteRelation failed: %v", err)
	}

	// Verify file exists
	if _, statErr := os.Stat(relationPath); os.IsNotExist(statErr) {
		t.Error("relation file should exist")
	}

	// Read back and verify
	content, err := os.ReadFile(relationPath)
	if err != nil {
		t.Fatalf("failed to read relation file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "from: DEC-001") {
		t.Error("content should contain from")
	}
	if !strings.Contains(contentStr, "relation: addresses") {
		t.Error("content should contain relation")
	}
	if !strings.Contains(contentStr, "to: REQ-001") {
		t.Error("content should contain to")
	}
	if !strings.Contains(contentStr, "rationale:") {
		t.Error("content should contain rationale property")
	}
}

func TestDeleteRelation(t *testing.T) {
	tmpDir := t.TempDir()

	relationPath := filepath.Join(tmpDir, "DEC-001--addresses--REQ-001.md")
	if err := os.WriteFile(relationPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err := testIO.DeleteRelation(relationPath)
	if err != nil {
		t.Fatalf("DeleteRelation failed: %v", err)
	}

	if _, err := os.Stat(relationPath); !os.IsNotExist(err) {
		t.Error("relation file should be deleted")
	}
}

func TestDeleteRelation_NonExistent(t *testing.T) {
	err := testIO.DeleteRelation("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestListRelationFiles(t *testing.T) {
	tmpDir := t.TempDir()
	relationsDir := filepath.Join(tmpDir, "relations")
	if err := os.MkdirAll(relationsDir, 0755); err != nil {
		t.Fatalf("failed to create relations dir: %v", err)
	}

	// Create some relation files
	relations := []string{
		"DEC-001--addresses--REQ-001.md",
		"DEC-002--addresses--REQ-002.md",
		"DEC-003--implements--SOL-001.md",
	}
	for _, name := range relations {
		path := filepath.Join(relationsDir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create relation: %v", err)
		}
	}

	// Create a non-markdown file (should be ignored)
	if err := os.WriteFile(filepath.Join(relationsDir, "README.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create non-md file: %v", err)
	}

	files, err := testIO.ListRelationFiles(relationsDir)
	if err != nil {
		t.Fatalf("ListRelationFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("got %d files, want 3", len(files))
	}

	// Verify all expected files are present
	fileMap := make(map[string]bool)
	for _, file := range files {
		fileMap[filepath.Base(file)] = true
	}
	for _, expected := range relations {
		if !fileMap[expected] {
			t.Errorf("missing file: %s", expected)
		}
	}
}

func TestListRelationFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	relationsDir := filepath.Join(tmpDir, "relations")
	if err := os.MkdirAll(relationsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	files, err := testIO.ListRelationFiles(relationsDir)
	if err != nil {
		t.Fatalf("ListRelationFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestListRelationFiles_NonExistent(t *testing.T) {
	files, err := testIO.ListRelationFiles("/nonexistent/dir")
	if err != nil {
		t.Fatalf("ListRelationFiles should not fail for nonexistent dir: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("got %d files, want 0 for nonexistent dir", len(files))
	}
}

func TestLoadAllRelations(t *testing.T) {
	tmpDir := t.TempDir()
	relationsDir := filepath.Join(tmpDir, "relations")
	if err := os.MkdirAll(relationsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create multiple relation files
	for i := 1; i <= 3; i++ {
		content := `---
from: DEC-00` + string(rune('0'+i)) + `
relation: addresses
to: REQ-00` + string(rune('0'+i)) + `
---
`
		name := "DEC-00" + string(rune('0'+i)) + "--addresses--REQ-00" + string(rune('0'+i)) + ".md"
		path := filepath.Join(relationsDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create relation: %v", err)
		}
	}

	relations, err := testIO.LoadAllRelations(relationsDir)
	if err != nil {
		t.Fatalf("LoadAllRelations failed: %v", err)
	}

	if len(relations) != 3 {
		t.Errorf("got %d relations, want 3", len(relations))
	}

	// Verify relations were loaded correctly
	fromIDs := make(map[string]bool)
	for _, relation := range relations {
		fromIDs[relation.From] = true
		if relation.Type != "addresses" {
			t.Errorf("relation %s->%s has type %q, want %q", relation.From, relation.To, relation.Type, "addresses")
		}
	}

	for i := 1; i <= 3; i++ {
		expectedFrom := "DEC-00" + string(rune('0'+i))
		if !fromIDs[expectedFrom] {
			t.Errorf("missing relation from: %s", expectedFrom)
		}
	}
}

func TestLoadAllRelations_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	relationsDir := filepath.Join(tmpDir, "relations")
	if err := os.MkdirAll(relationsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	relations, err := testIO.LoadAllRelations(relationsDir)
	if err != nil {
		t.Fatalf("LoadAllRelations failed: %v", err)
	}

	if len(relations) != 0 {
		t.Errorf("got %d relations, want 0", len(relations))
	}
}

func TestLoadAllRelations_SkipsInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	relationsDir := filepath.Join(tmpDir, "relations")
	if err := os.MkdirAll(relationsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create one valid relation
	validContent := `---
from: DEC-001
relation: addresses
to: REQ-001
---
`
	if err := os.WriteFile(filepath.Join(relationsDir, "DEC-001--addresses--REQ-001.md"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to create valid relation: %v", err)
	}

	// Create one invalid relation
	invalidContent := `---
invalid yaml: [
---
`
	if err := os.WriteFile(filepath.Join(relationsDir, "invalid.md"), []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to create invalid relation: %v", err)
	}

	relations, err := testIO.LoadAllRelations(relationsDir)
	if err != nil {
		t.Fatalf("LoadAllRelations failed: %v", err)
	}

	// Should only load the valid relation
	if len(relations) != 1 {
		t.Errorf("got %d relations, want 1 (invalid should be skipped)", len(relations))
	}
}

func TestRelationFilename(t *testing.T) {
	tests := []struct {
		from     string
		relType  string
		to       string
		expected string
	}{
		{"DEC-001", "addresses", "REQ-001", "DEC-001--addresses--REQ-001.md"},
		{"FOO-123", "implements", "BAR-456", "FOO-123--implements--BAR-456.md"},
		{"A", "b", "C", "A--b--C.md"},
	}

	for _, tt := range tests {
		got := RelationFilename(tt.from, tt.relType, tt.to)
		if got != tt.expected {
			t.Errorf("RelationFilename(%q, %q, %q) = %q, want %q", tt.from, tt.relType, tt.to, got, tt.expected)
		}
	}
}

func TestParseRelationFilename(t *testing.T) {
	tests := []struct {
		filename string
		wantFrom string
		wantType string
		wantTo   string
		wantOk   bool
	}{
		{"DEC-001--addresses--REQ-001.md", "DEC-001", "addresses", "REQ-001", true},
		{"FOO-123--implements--BAR-456.md", "FOO-123", "implements", "BAR-456", true},
		{"A--b--C.md", "A", "b", "C", true},
		{"invalid.md", "", "", "", false},
		{"one--two.md", "", "", "", false},
		{"--two--three.md", "", "", "", false},
		{"one----three.md", "", "", "", false},
		{"one--two--.md", "", "", "", false},
	}

	for _, tt := range tests {
		gotFrom, gotType, gotTo, gotOk := ParseRelationFilename(tt.filename)
		if gotOk != tt.wantOk {
			t.Errorf("ParseRelationFilename(%q) ok = %v, want %v", tt.filename, gotOk, tt.wantOk)
			continue
		}
		if gotOk {
			if gotFrom != tt.wantFrom {
				t.Errorf("ParseRelationFilename(%q) from = %q, want %q", tt.filename, gotFrom, tt.wantFrom)
			}
			if gotType != tt.wantType {
				t.Errorf("ParseRelationFilename(%q) type = %q, want %q", tt.filename, gotType, tt.wantType)
			}
			if gotTo != tt.wantTo {
				t.Errorf("ParseRelationFilename(%q) to = %q, want %q", tt.filename, gotTo, tt.wantTo)
			}
		}
	}
}
