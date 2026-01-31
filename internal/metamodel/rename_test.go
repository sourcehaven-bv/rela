package metamodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameEntityType(t *testing.T) {
	t.Run("renames entity key", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    properties:
      title:
        type: string
relations: {}
`
		writeFile(t, path, input)

		if err := RenameEntityTypeFS(path, "requirement", "feature", testMetaFS); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile(t, path)
		if !strings.Contains(got, "feature:") {
			t.Errorf("expected 'feature:' key, got:\n%s", got)
		}
		if strings.Contains(got, "requirement:") {
			t.Errorf("old key 'requirement:' should be gone, got:\n%s", got)
		}
	})

	t.Run("updates relation from and to", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
entities:
  requirement:
    label: Requirement
  decision:
    label: Decision
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
  dependsOn:
    label: depends on
    from: [requirement, decision]
    to: [requirement, decision]
`
		writeFile(t, path, input)

		if err := RenameEntityTypeFS(path, "requirement", "feature", testMetaFS); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile(t, path)

		// Check relations are updated
		if strings.Contains(got, "to: [requirement]") {
			t.Errorf("relation 'to' should reference 'feature', got:\n%s", got)
		}
		if !strings.Contains(got, "feature") {
			t.Errorf("expected 'feature' in relations, got:\n%s", got)
		}
		// Decision should be untouched
		if !strings.Contains(got, "decision") {
			t.Errorf("decision should be untouched, got:\n%s", got)
		}
	})

	t.Run("updates validation entity_type", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
entities:
  requirement:
    label: Requirement
relations: {}
validations:
  - name: req-needs-priority
    entity_type: requirement
    then:
      - "priority!="
  - name: other-rule
    entity_type: decision
    then:
      - "title!="
`
		writeFile(t, path, input)

		if err := RenameEntityTypeFS(path, "requirement", "feature", testMetaFS); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile(t, path)
		if strings.Contains(got, "entity_type: requirement") {
			t.Errorf("validation entity_type should be updated, got:\n%s", got)
		}
		if !strings.Contains(got, "entity_type: feature") {
			t.Errorf("expected 'entity_type: feature', got:\n%s", got)
		}
		// Other validation should be untouched
		if !strings.Contains(got, "entity_type: decision") {
			t.Errorf("decision validation should be untouched, got:\n%s", got)
		}
	})

	t.Run("error when type not found", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
entities:
  requirement:
    label: Requirement
relations: {}
`
		writeFile(t, path, input)

		err := RenameEntityTypeFS(path, "nonexistent", "feature", testMetaFS)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("error when new type already exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
entities:
  requirement:
    label: Requirement
  decision:
    label: Decision
relations: {}
`
		writeFile(t, path, input)

		err := RenameEntityTypeFS(path, "requirement", "decision", testMetaFS)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}

		// Verify file was not modified
		got := readFile(t, path)
		if !strings.Contains(got, "requirement:") {
			t.Error("original file should not be modified on error")
		}
	})

	t.Run("error when file not found", func(t *testing.T) {
		err := RenameEntityTypeFS("/nonexistent/path/metamodel.yaml", "old", "new", testMetaFS)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("handles no relations section", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
entities:
  requirement:
    label: Requirement
`
		writeFile(t, path, input)

		if err := RenameEntityTypeFS(path, "requirement", "feature", testMetaFS); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile(t, path)
		if !strings.Contains(got, "feature:") {
			t.Errorf("expected 'feature:', got:\n%s", got)
		}
	})

	t.Run("preserves comments", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "metamodel.yaml")

		input := `version: "1.0"
# Entity definitions
entities:
  requirement: # The main requirement type
    label: Requirement
relations: {}
`
		writeFile(t, path, input)

		if err := RenameEntityTypeFS(path, "requirement", "feature", testMetaFS); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile(t, path)
		if !strings.Contains(got, "# Entity definitions") {
			t.Errorf("comments should be preserved, got:\n%s", got)
		}
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	return string(data)
}
