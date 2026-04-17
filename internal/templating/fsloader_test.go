package templating

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

type testPaths struct {
	root                 string
	entityTemplatesDir   string
	relationTemplatesDir string
}

func setupTestPaths(t *testing.T) testPaths {
	t.Helper()
	tmpDir := t.TempDir()
	return testPaths{
		root:                 tmpDir,
		entityTemplatesDir:   filepath.Join(tmpDir, "templates", "entities"),
		relationTemplatesDir: filepath.Join(tmpDir, "templates", "relations"),
	}
}

func (p testPaths) entityTemplatePath(entityType string) string {
	return filepath.Join(p.entityTemplatesDir, entityType+".md")
}

func (p testPaths) entityTemplateVariantPath(entityType, variant string) string {
	if variant == "" {
		return p.entityTemplatePath(entityType)
	}
	return filepath.Join(p.entityTemplatesDir, entityType+"--"+variant+".md")
}

func (p testPaths) relationTemplatePath(relationType string) string {
	return filepath.Join(p.relationTemplatesDir, relationType+".md")
}

func testFS() storage.FS { return storage.NewOsFS() }

func TestLoadEntityTemplate_NotFound(t *testing.T) {
	paths := setupTestPaths(t)

	doc, err := loadEntityTemplateDoc(testFS(), paths.entityTemplatePath("requirement"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil document for non-existent template, got %+v", doc)
	}
}

func TestLoadEntityTemplate_Success(t *testing.T) {
	paths := setupTestPaths(t)

	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	templateContent := `---
title: Default Title
status: proposed
priority: high
---

# Description

This is a template description.
`
	templatePath := paths.entityTemplatePath("requirement")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	doc, err := loadEntityTemplateDoc(testFS(), templatePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("expected document, got nil")
	}

	if doc.GetString("title") != "Default Title" {
		t.Errorf("title = %q, want %q", doc.GetString("title"), "Default Title")
	}
	if doc.GetString("status") != "proposed" {
		t.Errorf("status = %q, want %q", doc.GetString("status"), "proposed")
	}
	if doc.GetString("priority") != "high" {
		t.Errorf("priority = %q, want %q", doc.GetString("priority"), "high")
	}
	if doc.Content == "" {
		t.Error("expected content, got empty string")
	}
}

func TestLoadRelationTemplate_NotFound(t *testing.T) {
	paths := setupTestPaths(t)

	doc, err := loadRelationTemplateDoc(testFS(), paths.relationTemplatePath("addresses"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil document for non-existent template, got %+v", doc)
	}
}

func TestGenerateEntityTemplate(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label: "Requirement",
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string", Required: true},
					"status":      {Type: "status", Default: "draft"},
					"priority":    {Type: "priority"},
					"description": {Type: "string"},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"draft", "proposed", "accepted"},
				Default: "draft",
			},
			"priority": {
				Values:  []string{"critical", "high", "medium", "low"},
				Default: "medium",
			},
		},
	}

	templatePath := paths.entityTemplatePath("requirement")
	created, err := generateEntityTemplate(testFS(), templatePath, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}

	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}
	if string(content) == "" {
		t.Error("template content is empty")
	}

	doc, err := markdown.ParseDocument(string(content))
	if err != nil {
		t.Fatalf("failed to parse generated template: %v", err)
	}

	if doc.GetString("status") != "draft" {
		t.Errorf("status = %q, want %q", doc.GetString("status"), "draft")
	}
	if doc.GetString("priority") != "medium" {
		t.Errorf("priority = %q, want %q", doc.GetString("priority"), "medium")
	}
	if doc.Content == "" {
		t.Error("expected placeholder content")
	}
}

func TestGenerateEntityTemplate_NoOverwrite(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
	}

	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	templatePath := paths.entityTemplatePath("requirement")
	existingContent := "existing content"
	if err := os.WriteFile(templatePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing template: %v", err)
	}

	created, err := generateEntityTemplate(testFS(), templatePath, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected template NOT to be created (file exists)")
	}

	content, _ := os.ReadFile(templatePath)
	if string(content) != existingContent {
		t.Errorf("content = %q, want %q (should not be overwritten)", string(content), existingContent)
	}
}

func TestGenerateEntityTemplate_ForceOverwrite(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
	}

	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	templatePath := paths.entityTemplatePath("requirement")
	if err := os.WriteFile(templatePath, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write existing template: %v", err)
	}

	created, err := generateEntityTemplate(testFS(), templatePath, meta, "requirement", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created with force flag")
	}

	content, _ := os.ReadFile(templatePath)
	if string(content) == "old" {
		t.Error("content should have been overwritten")
	}
}

func TestGenerateEntityTemplate_UnknownType(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}

	_, err := generateEntityTemplate(testFS(), paths.entityTemplatePath("unknown"), meta, "unknown", false)
	if err == nil {
		t.Error("expected error for unknown entity type")
	}
}

func TestGenerateEntityTemplate_Variant(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label: "Requirement",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "status", Default: "draft"},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"draft", "proposed", "accepted"},
				Default: "draft",
			},
		},
	}

	variantPath := paths.entityTemplateVariantPath("requirement", "epic")
	created, err := generateEntityTemplate(testFS(), variantPath, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected variant template to be created")
	}

	if _, statErr := os.Stat(variantPath); os.IsNotExist(statErr) {
		t.Error("variant template file should exist")
	}

	defaultPath := paths.entityTemplatePath("requirement")
	if _, statErr := os.Stat(defaultPath); !os.IsNotExist(statErr) {
		t.Error("default template should not be created when generating variant")
	}

	content, err := os.ReadFile(variantPath)
	if err != nil {
		t.Fatalf("failed to read variant template: %v", err)
	}
	if len(content) == 0 {
		t.Error("variant template content is empty")
	}
}

func TestGenerateRelationTemplate(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{
			"addresses": {
				Label: "Addresses",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}

	templatePath := paths.relationTemplatePath("addresses")
	created, err := generateRelationTemplate(testFS(), templatePath, meta, "addresses", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}

	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Error("template file should exist")
	}
}

func TestGenerateRelationTemplate_UnknownType(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{},
	}

	_, err := generateRelationTemplate(testFS(), paths.relationTemplatePath("unknown"), meta, "unknown", false)
	if err == nil {
		t.Error("expected error for unknown relation type")
	}
}

func TestDiscoverEntityTemplates_NoTemplatesDir(t *testing.T) {
	paths := setupTestPaths(t)

	templates, err := discoverEntityTemplates(testFS(), paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestDiscoverEntityTemplates_DefaultOnly(t *testing.T) {
	paths := setupTestPaths(t)

	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	templateContent := `---
status: draft
priority: high
---
# Default content
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement.md"), []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	templates, err := discoverEntityTemplates(testFS(), paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]
	if tmpl.Name != "" {
		t.Errorf("default template Name = %q, want empty string", tmpl.Name)
	}
	if tmpl.EntityType != "requirement" {
		t.Errorf("EntityType = %q, want %q", tmpl.EntityType, "requirement")
	}
	if tmpl.Properties["status"] != "draft" {
		t.Errorf("status = %v, want %q", tmpl.Properties["status"], "draft")
	}
}

func TestDiscoverEntityTemplates_WithVariants(t *testing.T) {
	paths := setupTestPaths(t)

	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	defaultContent := `---
status: draft
---
# Default
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement.md"), []byte(defaultContent), 0644); err != nil {
		t.Fatalf("failed to write default template: %v", err)
	}

	epicContent := `---
status: proposed
priority: high
---
# Epic template
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement--epic.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic template: %v", err)
	}

	checklistContent := `---
status: draft
---
# Checklist
- [ ] Item 1
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement--checklist.md"), []byte(checklistContent), 0644); err != nil {
		t.Fatalf("failed to write checklist template: %v", err)
	}

	// Unrelated template (different entity type)
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "decision.md"), []byte("---\n---\n"), 0644); err != nil {
		t.Fatalf("failed to write unrelated template: %v", err)
	}

	templates, err := discoverEntityTemplates(testFS(), paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(templates))
	}

	if templates[0].Name != "" {
		t.Errorf("first template should be default (empty name), got %q", templates[0].Name)
	}
	if templates[1].Name != "checklist" {
		t.Errorf("second template should be 'checklist', got %q", templates[1].Name)
	}
	if templates[2].Name != "epic" {
		t.Errorf("third template should be 'epic', got %q", templates[2].Name)
	}

	epic := templates[2]
	if epic.Properties["priority"] != "high" {
		t.Errorf("epic priority = %v, want %q", epic.Properties["priority"], "high")
	}
}

func TestDiscoverEntityTemplates_WithRelations(t *testing.T) {
	paths := setupTestPaths(t)

	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	templateContent := `---
status: draft
_template_relations:
  - relation: addresses
    target: COMP-001
  - relation: assigned-to
    target: USER-042
---
# Template with relations
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement.md"), []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	templates, err := discoverEntityTemplates(testFS(), paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]

	if _, exists := tmpl.Properties["_template_relations"]; exists {
		t.Error("_template_relations should not be in Properties")
	}

	if len(tmpl.Relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(tmpl.Relations))
	}

	rel1 := tmpl.Relations[0]
	if rel1.Type != "addresses" || rel1.Target != "COMP-001" {
		t.Errorf("relation 1 = {%q, %q}, want {addresses, COMP-001}", rel1.Type, rel1.Target)
	}

	rel2 := tmpl.Relations[1]
	if rel2.Type != "assigned-to" || rel2.Target != "USER-042" {
		t.Errorf("relation 2 = {%q, %q}, want {assigned-to, USER-042}", rel2.Type, rel2.Target)
	}
}

func TestExtractTemplateRelations_Empty(t *testing.T) {
	frontmatter := map[string]interface{}{
		"status": "draft",
	}

	relations := extractTemplateRelations(frontmatter)
	if len(relations) != 0 {
		t.Errorf("expected 0 relations, got %d", len(relations))
	}
}

func TestExtractTemplateRelations_InvalidFormat(t *testing.T) {
	frontmatter := map[string]interface{}{
		"_template_relations": "invalid",
	}

	relations := extractTemplateRelations(frontmatter)
	if len(relations) != 0 {
		t.Errorf("expected 0 relations for invalid format, got %d", len(relations))
	}
}

func TestExtractTemplateRelations_PartialData(t *testing.T) {
	frontmatter := map[string]interface{}{
		"_template_relations": []interface{}{
			map[string]interface{}{"relation": "addresses"},                       // missing target
			map[string]interface{}{"target": "COMP-001"},                          // missing relation
			map[string]interface{}{"relation": "implements", "target": "REQ-001"}, // valid
		},
	}

	relations := extractTemplateRelations(frontmatter)
	if len(relations) != 1 {
		t.Fatalf("expected 1 valid relation, got %d", len(relations))
	}
	if relations[0].Type != "implements" || relations[0].Target != "REQ-001" {
		t.Errorf("relation = {%q, %q}, want {implements, REQ-001}", relations[0].Type, relations[0].Target)
	}
}
