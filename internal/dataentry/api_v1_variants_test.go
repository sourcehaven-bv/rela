package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestV1Schema_AbstractAndVariants pins that `/_schema` tells an
// integration which concrete types are variants of an abstract type.
func TestV1Schema_AbstractAndVariants(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"schema.yaml": `
version: "1.0"
includes: [isms.yaml]
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
  task:
    extends: task
    label: ISMS task
    id_prefix: "ISMS-"
    properties:
      evidence: { type: string }
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
	app := newAppFromParts(&Config{App: AppConfig{Name: "Variants", Description: "x"}}, meta, newFixture())

	rec := httptest.NewRecorder()
	app.handleV1Schema(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var schema v1.Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatal(err)
	}

	if _, ok := schema.Entities["task"]; ok {
		t.Error("abstract task served as a concrete entity type")
	}
	variant, ok := schema.Entities["isms.task"]
	if !ok {
		t.Fatalf("isms.task missing: %v", schema.Entities)
	}
	if variant.Namespace != "isms" || variant.Extends != "task" {
		t.Errorf("isms.task namespace=%q extends=%q, want isms, task", variant.Namespace, variant.Extends)
	}
	if _, has := variant.Properties["title"]; !has {
		t.Error("variant did not inherit title")
	}
	abstract, ok := schema.Abstract["task"]
	if !ok {
		t.Fatalf("abstract task missing: %v", schema.Abstract)
	}
	if !slices.Equal(abstract.Variants, []string{"isms.task"}) {
		t.Errorf("variants = %v", abstract.Variants)
	}
	if _, has := abstract.Properties["title"]; !has {
		t.Error("abstract shape lacks title")
	}
}
