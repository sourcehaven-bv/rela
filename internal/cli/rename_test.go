package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func setupRenameTestEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	g = graph.New()
	out = output.New(output.FormatTable)
	projectCtx = &project.Context{
		Root:                 dir,
		EntitiesDir:          filepath.Join(dir, "entities"),
		RelationsDir:         filepath.Join(dir, "relations"),
		CacheDir:             filepath.Join(dir, ".rela"),
		MetamodelPath:        filepath.Join(dir, "metamodel.yaml"),
		TemplatesDir:         filepath.Join(dir, "templates"),
		EntityTemplatesDir:   filepath.Join(dir, "templates", "entities"),
		RelationTemplatesDir: filepath.Join(dir, "templates", "relations"),
	}

	// Create directories
	os.MkdirAll(filepath.Join(dir, ".rela"), 0755)
	os.MkdirAll(filepath.Join(dir, "entities", "requirements"), 0755)
	os.MkdirAll(filepath.Join(dir, "relations"), 0755)

	// Write metamodel using shared helper
	metamodelYAML := testutil.SimpleMetamodelYAML()
	os.WriteFile(projectCtx.MetamodelPath, []byte(metamodelYAML), 0644)

	// Load metamodel
	var err error
	meta, err = metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}

	// Set up workspace for FS access
	fs := storage.NewSafeFS(storage.NewOsFS())
	repo := repository.New(fs, projectCtx)
	ws = workspace.NewWithGraph(repo, meta, g)

	return dir
}

func writeEntityFile(t *testing.T, path, id, entityType, title string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	content := "---\nid: " + id + "\ntype: " + entityType + "\ntitle: " + title + "\n---\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write entity file: %v", err)
	}
}

func TestValidateTypeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "ticket", false},
		{"valid with hyphen", "risk-item", false},
		{"valid with underscore", "risk_item", false},
		{"valid with digits", "issue2", false},
		{"empty", "", true},
		{"uppercase", "Ticket", true},
		{"starts with digit", "1ticket", true},
		{"path traversal dots", "a..b", true},
		{"path traversal slash", "a/b", true},
		{"path traversal backslash", "a\\b", true},
		{"spaces", "risk item", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTypeName(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("validateTypeName(%q) expected error, got nil", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateTypeName(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

func TestRenameEntityCommand(t *testing.T) {
	t.Run("renames entity type end to end", func(t *testing.T) {
		dir := setupRenameTestEnv(t)

		// Create entity files
		writeEntityFile(t,
			filepath.Join(dir, "entities", "requirements", "REQ-001.md"),
			"REQ-001", "requirement", "First Requirement")
		writeEntityFile(t,
			filepath.Join(dir, "entities", "requirements", "REQ-002.md"),
			"REQ-002", "requirement", "Second Requirement")

		// Add to graph
		g.AddNode(&model.Entity{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "First Requirement"}})
		g.AddNode(&model.Entity{ID: "REQ-002", Type: "requirement", Properties: map[string]interface{}{"title": "Second Requirement"}})

		// Run rename
		renameForce = true
		renamePlural = ""
		err := runRenameEntity("requirement", "feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check metamodel was updated
		mmData, err := os.ReadFile(projectCtx.MetamodelPath)
		if err != nil {
			t.Fatalf("failed to read metamodel: %v", err)
		}
		mmContent := string(mmData)

		if strings.Contains(mmContent, "requirement:") {
			t.Error("metamodel should not contain 'requirement:' key")
		}
		if !strings.Contains(mmContent, "feature:") {
			t.Error("metamodel should contain 'feature:' key")
		}
		if !strings.Contains(mmContent, "to: [feature]") && !strings.Contains(mmContent, "- feature") {
			t.Error("metamodel relation 'to' should reference 'feature'")
		}

		// Check directory was renamed
		if _, statErr := os.Stat(filepath.Join(dir, "entities", "requirements")); !os.IsNotExist(statErr) {
			t.Error("old directory should not exist")
		}
		if _, statErr := os.Stat(filepath.Join(dir, "entities", "features")); statErr != nil {
			t.Error("new directory should exist")
		}

		// Check entity files were updated
		data, err := os.ReadFile(filepath.Join(dir, "entities", "features", "REQ-001.md"))
		if err != nil {
			t.Fatalf("failed to read renamed entity: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "type: feature") {
			t.Errorf("entity file should have type 'feature', got:\n%s", content)
		}
		if !strings.Contains(content, "id: REQ-001") {
			t.Errorf("entity file should keep original ID, got:\n%s", content)
		}

	})

	t.Run("error when old type not found", func(t *testing.T) {
		setupRenameTestEnv(t)

		renameForce = true
		err := runRenameEntity("nonexistent", "feature")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("error when new type already exists", func(t *testing.T) {
		setupRenameTestEnv(t)

		renameForce = true
		err := runRenameEntity("requirement", "decision")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("error when new type name is invalid", func(t *testing.T) {
		setupRenameTestEnv(t)

		renameForce = true
		err := runRenameEntity("requirement", "Bad-Name")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid type name") {
			t.Errorf("expected 'invalid type name' error, got: %v", err)
		}
	})

	t.Run("custom plural form", func(t *testing.T) {
		dir := setupRenameTestEnv(t)

		writeEntityFile(t,
			filepath.Join(dir, "entities", "requirements", "REQ-001.md"),
			"REQ-001", "requirement", "Test")

		g.AddNode(&model.Entity{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Test"}})

		renameForce = true
		renamePlural = "policies"
		err := runRenameEntity("requirement", "policy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check custom plural was used for directory
		if _, err := os.Stat(filepath.Join(dir, "entities", "policies")); err != nil {
			t.Error("directory should use custom plural 'policies'")
		}
	})

	t.Run("renames template if exists", func(t *testing.T) {
		dir := setupRenameTestEnv(t)

		// Create template
		templateDir := filepath.Join(dir, "templates", "entities")
		os.MkdirAll(templateDir, 0755)
		templatePath := filepath.Join(templateDir, "requirement.md")
		os.WriteFile(templatePath, []byte("---\nstatus: draft\n---\n"), 0644)

		renameForce = true
		renamePlural = ""
		err := runRenameEntity("requirement", "feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check template was renamed
		if _, err := os.Stat(filepath.Join(templateDir, "requirement.md")); !os.IsNotExist(err) {
			t.Error("old template should not exist")
		}
		if _, err := os.Stat(filepath.Join(templateDir, "feature.md")); err != nil {
			t.Error("new template should exist")
		}
	})

	t.Run("handles no entity directory", func(t *testing.T) {
		setupRenameTestEnv(t)

		// Remove the entities directory entirely
		os.RemoveAll(projectCtx.EntitiesDir)
		os.MkdirAll(projectCtx.EntitiesDir, 0755)

		renameForce = true
		renamePlural = ""
		err := runRenameEntity("requirement", "feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Metamodel should still be updated
		mmData, _ := os.ReadFile(projectCtx.MetamodelPath)
		if !strings.Contains(string(mmData), "feature:") {
			t.Error("metamodel should contain 'feature:' key even with no entity directory")
		}
	})
}
