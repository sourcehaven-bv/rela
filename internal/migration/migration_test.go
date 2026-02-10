package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var testMigrationFS = storage.NewOsFS()

func TestDetect_Integration(t *testing.T) {
	// Create a temp file with entity missing id_type
	content := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ"
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	detections, err := Detect(tmpFile, FileTypeMetamodel, testMigrationFS)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(detections) != 1 {
		t.Errorf("got %d detections, want 1", len(detections))
	}

	if len(detections) > 0 && detections[0].Migration.Name() != "short-id-default" {
		t.Errorf("got migration %q, want %q", detections[0].Migration.Name(), "short-id-default")
	}
}

func TestApply_Integration(t *testing.T) {
	// Create a temp file with entity missing id_type and one with deprecated 'string'
	content := `version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ"
  component:
    label: Component
    id_type: string
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	result, err := Apply(tmpFile, FileTypeMetamodel, testMigrationFS)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !result.NeedsMigration() {
		t.Error("expected migration to be needed")
	}

	if result.HasErrors() {
		t.Errorf("unexpected error: %v", result.Error)
	}

	// Read the file and verify it was updated
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	updatedStr := string(updated)

	// Check that id_type: sequential was added for requirement
	if !strings.Contains(updatedStr, "id_type: sequential") {
		t.Error("file does not contain 'id_type: sequential'")
	}
	// Check that string was renamed to manual
	if !strings.Contains(updatedStr, "id_type: manual") {
		t.Error("file does not contain 'id_type: manual'")
	}
	// Check that deprecated 'string' is gone
	if strings.Contains(updatedStr, "id_type: string") {
		t.Error("file still contains deprecated 'id_type: string'")
	}
}

func TestCheckOnly_DoesNotModify(t *testing.T) {
	content := `version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ"
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	result, err := CheckOnly(tmpFile, FileTypeMetamodel, testMigrationFS)
	if err != nil {
		t.Fatalf("CheckOnly() error = %v", err)
	}

	if !result.NeedsMigration() {
		t.Error("expected migration to be needed")
	}

	// Verify file was not modified
	after, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(after) != content {
		t.Error("CheckOnly modified the file")
	}
}

func TestMigrationError(t *testing.T) {
	err := &Error{
		FilePath: "metamodel.yaml",
		Detections: []DetectionResult{
			{Description: "test migration"},
		},
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "metamodel.yaml") {
		t.Error("error should contain file path")
	}
	if !strings.Contains(errStr, "test migration") {
		t.Error("error should contain migration description")
	}
	if !strings.Contains(errStr, "rela migrate") {
		t.Error("error should suggest running rela migrate")
	}
}

func TestForFileType(t *testing.T) {
	migrations := ForFileType(FileTypeMetamodel)

	if len(migrations) == 0 {
		t.Error("expected at least one migration for metamodel files")
	}

	found := false
	for _, m := range migrations {
		if m.Name() == "short-id-default" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find short-id-default migration")
	}
}
