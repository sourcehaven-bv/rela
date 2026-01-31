package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var testMigrationFS = storage.NewOsFS()

func TestIDTypeRenameMigration_Detect(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantFind bool
	}{
		{
			name: "detects sequential",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
`,
			wantFind: true,
		},
		{
			name: "detects string",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_type: string
`,
			wantFind: true,
		},
		{
			name: "detects both",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
  component:
    label: Component
    id_type: string
`,
			wantFind: true,
		},
		{
			name: "no detection for auto",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: auto
`,
			wantFind: false,
		},
		{
			name: "no detection for manual",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_type: manual
`,
			wantFind: false,
		},
		{
			name: "no detection when id_type is missing",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
`,
			wantFind: false,
		},
		{
			name: "no detection for empty entities",
			yaml: `
version: "1.0"
entities: {}
`,
			wantFind: false,
		},
	}

	m := &IDTypeRenameMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			got := m.Detect(&doc)
			if got != tt.wantFind {
				t.Errorf("Detect() = %v, want %v", got, tt.wantFind)
			}
		})
	}
}

func TestIDTypeRenameMigration_Apply(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantValues map[string]string // entity name -> expected id_type value
	}{
		{
			name: "converts sequential to auto",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
`,
			wantValues: map[string]string{"requirement": "auto"},
		},
		{
			name: "converts string to manual",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_type: string
`,
			wantValues: map[string]string{"component": "manual"},
		},
		{
			name: "converts multiple entities",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
  component:
    label: Component
    id_type: string
`,
			wantValues: map[string]string{
				"requirement": "auto",
				"component":   "manual",
			},
		},
		{
			name: "leaves auto unchanged",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: auto
`,
			wantValues: map[string]string{"requirement": "auto"},
		},
	}

	m := &IDTypeRenameMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Verify the values
			root := GetDocumentRoot(&doc)
			entities := GetMapValue(root, "entities")
			if entities == nil {
				t.Fatal("entities not found")
			}

			for i := 0; i < len(entities.Content)-1; i += 2 {
				entityName := entities.Content[i].Value
				entityDef := entities.Content[i+1]
				idTypeValue := GetMapValue(entityDef, "id_type")

				if expected, ok := tt.wantValues[entityName]; ok {
					if idTypeValue == nil {
						t.Errorf("entity %s: id_type not found", entityName)
					} else if idTypeValue.Value != expected {
						t.Errorf("entity %s: id_type = %q, want %q", entityName, idTypeValue.Value, expected)
					}
				}
			}
		})
	}
}

func TestIDTypeRenameMigration_PreservesComments(t *testing.T) {
	input := `# Metamodel config
version: "1.0"

entities:
  # Requirements entity
  requirement:
    label: Requirement
    id_type: sequential  # Use sequential IDs
    id_patterns: ["REQ-"]
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &IDTypeRenameMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Re-encode and check for comments
	output, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	outputStr := string(output)

	// Check that the value was changed
	if !strings.Contains(outputStr, "id_type: auto") {
		t.Error("id_type was not changed to auto")
	}

	// Check that comments are preserved (yaml.v3 preserves head comments)
	if !strings.Contains(outputStr, "# Metamodel config") {
		t.Error("top comment was not preserved")
	}
	if !strings.Contains(outputStr, "# Requirements entity") {
		t.Error("entity comment was not preserved")
	}
}

func TestDetect_Integration(t *testing.T) {
	// Create a temp file with deprecated syntax
	content := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	detections, err := DetectFS(tmpFile, FileTypeMetamodel, testMigrationFS)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(detections) != 1 {
		t.Errorf("got %d detections, want 1", len(detections))
	}

	if len(detections) > 0 && detections[0].Migration.Name() != "id-type-rename" {
		t.Errorf("got migration %q, want %q", detections[0].Migration.Name(), "id-type-rename")
	}
}

func TestApply_Integration(t *testing.T) {
	// Create a temp file with deprecated syntax
	content := `version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
  component:
    label: Component
    id_type: string
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	result, err := ApplyFS(tmpFile, FileTypeMetamodel, testMigrationFS)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !result.NeedsMigration() {
		t.Error("expected migration to be needed")
	}

	if result.HasErrors() {
		t.Errorf("unexpected error: %v", result.Error)
	}

	if result.MigrationsApplied() != 1 {
		t.Errorf("got %d migrations applied, want 1", result.MigrationsApplied())
	}

	// Read the file and verify it was updated
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	updatedStr := string(updated)

	if strings.Contains(updatedStr, "id_type: sequential") {
		t.Error("file still contains 'sequential'")
	}
	if strings.Contains(updatedStr, "id_type: string") {
		t.Error("file still contains 'string'")
	}
	if !strings.Contains(updatedStr, "id_type: auto") {
		t.Error("file does not contain 'auto'")
	}
	if !strings.Contains(updatedStr, "id_type: manual") {
		t.Error("file does not contain 'manual'")
	}
}

func TestCheckOnly_DoesNotModify(t *testing.T) {
	content := `version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	result, err := CheckOnlyFS(tmpFile, FileTypeMetamodel, testMigrationFS)
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
		if m.Name() == "id-type-rename" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find id-type-rename migration")
	}
}
