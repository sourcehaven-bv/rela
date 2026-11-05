package dataentryconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func loadVariantMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"schema.yaml": `
version: "1.0"
includes: [isms.yaml, projects.yaml]
entities:
  task:
    abstract: true
    label: Task
    properties:
      title: { type: string, required: true }
`,
		"isms.yaml": `
namespace: isms
entities:
  task: { extends: task, label: ISMS task, id_prefix: "ISMS-" }
`,
		"projects.yaml": `
namespace: projects
entities:
  task: { extends: task, label: Project task, id_prefix: "PRJ-" }
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	meta, _, err := metamodel.Load(filepath.Join(dir, "schema.yaml"), storage.NewOsFS())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return meta
}

func TestValidateConfig_ExpandsAbstractTypes(t *testing.T) {
	meta := loadVariantMeta(t)
	data := []byte(`
version: "1.0"
spaces:
  - id: work
    label: Work
    create: [task]
entity_views:
  task: { mentioned_by_panel: false }
  isms.task: { mentioned_by_panel: true }
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(data, &cfg, meta); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if want := []string{"isms.task", "projects.task"}; !slices.Equal(cfg.Spaces[0].Create, want) {
		t.Errorf("create = %v, want %v", cfg.Spaces[0].Create, want)
	}
	if _, ok := cfg.EntityViews["task"]; ok {
		t.Error("abstract key kept in entity_views")
	}
	if p := cfg.EntityViews["isms.task"].MentionedByPanel; p == nil || !*p {
		t.Error("explicit variant entity_view did not win")
	}
	if p := cfg.EntityViews["projects.task"].MentionedByPanel; p == nil || *p {
		t.Error("variant did not inherit the abstract entity_view")
	}
}

func TestValidateConfig_SingleTypeAbstractNamesVariants(t *testing.T) {
	meta := loadVariantMeta(t)
	data := []byte(`
version: "1.0"
lists:
  tasks:
    title: Tasks
    entity_type: task
    columns: [{property: title}]
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	err := ValidateConfig(data, &cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "use one of its variants: isms.task, projects.task") {
		t.Fatalf("err = %v, want the variants named", err)
	}
}
