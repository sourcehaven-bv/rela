package markdown

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
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

func TestLoadEntityTemplate_NotFound(t *testing.T) {
	paths := setupTestPaths(t)

	doc, err := testIO.LoadEntityTemplate(paths.entityTemplatePath("requirement"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil document for non-existent template, got %+v", doc)
	}
}

func TestLoadEntityTemplate_Success(t *testing.T) {
	paths := setupTestPaths(t)

	// Create template directory and file
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

	doc, err := testIO.LoadEntityTemplate(templatePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("expected document, got nil")
	}

	// Check frontmatter
	if doc.GetString("title") != "Default Title" {
		t.Errorf("title = %q, want %q", doc.GetString("title"), "Default Title")
	}
	if doc.GetString("status") != "proposed" {
		t.Errorf("status = %q, want %q", doc.GetString("status"), "proposed")
	}
	if doc.GetString("priority") != "high" {
		t.Errorf("priority = %q, want %q", doc.GetString("priority"), "high")
	}

	// Check content
	if doc.Content == "" {
		t.Error("expected content, got empty string")
	}
}

func TestLoadRelationTemplate_NotFound(t *testing.T) {
	paths := setupTestPaths(t)

	doc, err := testIO.LoadRelationTemplate(paths.relationTemplatePath("addresses"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil document for non-existent template, got %+v", doc)
	}
}

func TestApplyEntityTemplate(t *testing.T) {
	entity := model.NewEntity("REQ-001", "requirement")
	entity.SetString("title", "My Title") // Already set

	template := &Document{
		Frontmatter: map[string]interface{}{
			"title":    "Template Title", // Should NOT override
			"status":   "proposed",       // Should be applied
			"priority": "high",           // Should be applied
			"id":       "IGNORED",        // Should be skipped
			"type":     "also-ignored",   // Should be skipped
		},
		Content: "Template content here",
	}

	ApplyEntityTemplate(entity, template)

	// Check that CLI value was preserved
	if entity.GetString("title") != "My Title" {
		t.Errorf("title = %q, want %q (should not be overridden)", entity.GetString("title"), "My Title")
	}

	// Check that template defaults were applied
	if entity.GetString("status") != "proposed" {
		t.Errorf("status = %q, want %q", entity.GetString("status"), "proposed")
	}
	if entity.GetString("priority") != "high" {
		t.Errorf("priority = %q, want %q", entity.GetString("priority"), "high")
	}

	// Check that id and type were NOT applied
	if entity.GetString("id") != "" {
		t.Errorf("id should not be set from template, got %q", entity.GetString("id"))
	}

	// Check content was applied
	if entity.Content != "Template content here" {
		t.Errorf("content = %q, want %q", entity.Content, "Template content here")
	}
}

func TestApplyEntityTemplate_Nil(t *testing.T) {
	entity := model.NewEntity("REQ-001", "requirement")
	entity.SetString("title", "My Title")

	// Should not panic
	ApplyEntityTemplate(entity, nil)

	// Entity should be unchanged
	if entity.GetString("title") != "My Title" {
		t.Errorf("title = %q, want %q", entity.GetString("title"), "My Title")
	}
}

func TestApplyEntityTemplate_ExistingContent(t *testing.T) {
	entity := model.NewEntity("REQ-001", "requirement")
	entity.Content = "Existing content"

	template := &Document{
		Frontmatter: map[string]interface{}{},
		Content:     "Template content",
	}

	ApplyEntityTemplate(entity, template)

	// Existing content should be preserved
	if entity.Content != "Existing content" {
		t.Errorf("content = %q, want %q (should not be overridden)", entity.Content, "Existing content")
	}
}

func TestApplyRelationTemplate(t *testing.T) {
	relation := model.NewRelation("DEC-001", "addresses", "REQ-001")

	template := &Document{
		Frontmatter: map[string]interface{}{
			"from":      "IGNORED",
			"relation":  "IGNORED",
			"to":        "IGNORED",
			"rationale": "Because it makes sense",
		},
	}

	ApplyRelationTemplate(relation, template)

	// Check that core fields were NOT modified
	if relation.From != "DEC-001" {
		t.Errorf("from = %q, want %q", relation.From, "DEC-001")
	}
	if relation.Type != "addresses" {
		t.Errorf("type = %q, want %q", relation.Type, "addresses")
	}
	if relation.To != "REQ-001" {
		t.Errorf("to = %q, want %q", relation.To, "REQ-001")
	}

	// Check that template properties were applied
	if relation.Properties["rationale"] != "Because it makes sense" {
		t.Errorf("rationale = %v, want %q", relation.Properties["rationale"], "Because it makes sense")
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

	// Generate template
	templatePath := paths.entityTemplatePath("requirement")
	created, err := testIO.GenerateEntityTemplate(templatePath, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	contentStr := string(content)
	if contentStr == "" {
		t.Error("template content is empty")
	}

	// Parse and verify
	doc, err := ParseDocument(contentStr)
	if err != nil {
		t.Fatalf("failed to parse generated template: %v", err)
	}

	// Check that properties have default values
	if doc.GetString("status") != "draft" {
		t.Errorf("status = %q, want %q", doc.GetString("status"), "draft")
	}
	if doc.GetString("priority") != "medium" {
		t.Errorf("priority = %q, want %q", doc.GetString("priority"), "medium")
	}

	// Check content
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

	// Create existing template
	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	templatePath := paths.entityTemplatePath("requirement")
	existingContent := "existing content"
	if err := os.WriteFile(templatePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing template: %v", err)
	}

	// Try to generate without force
	created, err := testIO.GenerateEntityTemplate(templatePath, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected template NOT to be created (file exists)")
	}

	// Verify content unchanged
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

	// Create existing template
	if err := os.MkdirAll(paths.entityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	templatePath := paths.entityTemplatePath("requirement")
	if err := os.WriteFile(templatePath, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write existing template: %v", err)
	}

	// Generate with force
	created, err := testIO.GenerateEntityTemplate(templatePath, meta, "requirement", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created with force flag")
	}

	// Verify content changed
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

	_, err := testIO.GenerateEntityTemplate(paths.entityTemplatePath("unknown"), meta, "unknown", false)
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

	// Generate variant template
	variantPath := paths.entityTemplateVariantPath("requirement", "epic")
	created, err := testIO.GenerateEntityTemplate(variantPath, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected variant template to be created")
	}

	// Verify file exists at correct path (requirement--epic.md)
	if _, statErr := os.Stat(variantPath); os.IsNotExist(statErr) {
		t.Error("variant template file should exist")
	}

	// Default template should NOT exist
	defaultPath := paths.entityTemplatePath("requirement")
	if _, statErr := os.Stat(defaultPath); !os.IsNotExist(statErr) {
		t.Error("default template should not be created when generating variant")
	}

	// Verify content
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
	created, err := testIO.GenerateRelationTemplate(templatePath, meta, "addresses", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}

	// Verify file exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Error("template file should exist")
	}
}

func TestGenerateRelationTemplate_UnknownType(t *testing.T) {
	paths := setupTestPaths(t)

	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{},
	}

	_, err := testIO.GenerateRelationTemplate(paths.relationTemplatePath("unknown"), meta, "unknown", false)
	if err == nil {
		t.Error("expected error for unknown relation type")
	}
}

func TestDiscoverEntityTemplates_NoTemplatesDir(t *testing.T) {
	paths := setupTestPaths(t)

	templates, err := testIO.DiscoverEntityTemplates(paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestDiscoverEntityTemplates_DefaultOnly(t *testing.T) {
	paths := setupTestPaths(t)

	// Create templates directory and default template
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

	templates, err := testIO.DiscoverEntityTemplates(paths.entityTemplatesDir, "requirement")
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

	// Default template
	defaultContent := `---
status: draft
---
# Default
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement.md"), []byte(defaultContent), 0644); err != nil {
		t.Fatalf("failed to write default template: %v", err)
	}

	// Epic variant
	epicContent := `---
status: proposed
priority: high
---
# Epic template
`
	if err := os.WriteFile(filepath.Join(paths.entityTemplatesDir, "requirement--epic.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic template: %v", err)
	}

	// Checklist variant
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

	templates, err := testIO.DiscoverEntityTemplates(paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(templates))
	}

	// Check order: default first, then alphabetically
	if templates[0].Name != "" {
		t.Errorf("first template should be default (empty name), got %q", templates[0].Name)
	}
	if templates[1].Name != "checklist" {
		t.Errorf("second template should be 'checklist', got %q", templates[1].Name)
	}
	if templates[2].Name != "epic" {
		t.Errorf("third template should be 'epic', got %q", templates[2].Name)
	}

	// Verify epic template properties
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

	templates, err := testIO.DiscoverEntityTemplates(paths.entityTemplatesDir, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]

	// _template_relations should be extracted and removed from Properties
	if _, exists := tmpl.Properties["_template_relations"]; exists {
		t.Error("_template_relations should not be in Properties")
	}

	// Check relations were parsed
	if len(tmpl.Relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(tmpl.Relations))
	}

	rel1 := tmpl.Relations[0]
	if rel1.Relation != "addresses" || rel1.Target != "COMP-001" {
		t.Errorf("relation 1 = {%q, %q}, want {addresses, COMP-001}", rel1.Relation, rel1.Target)
	}

	rel2 := tmpl.Relations[1]
	if rel2.Relation != "assigned-to" || rel2.Target != "USER-042" {
		t.Errorf("relation 2 = {%q, %q}, want {assigned-to, USER-042}", rel2.Relation, rel2.Target)
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
	// _template_relations is not a list
	frontmatter := map[string]interface{}{
		"_template_relations": "invalid",
	}

	relations := extractTemplateRelations(frontmatter)
	if len(relations) != 0 {
		t.Errorf("expected 0 relations for invalid format, got %d", len(relations))
	}
}

func TestExtractTemplateRelations_PartialData(t *testing.T) {
	// Some entries missing required fields
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
	if relations[0].Relation != "implements" || relations[0].Target != "REQ-001" {
		t.Errorf("relation = {%q, %q}, want {implements, REQ-001}", relations[0].Relation, relations[0].Target)
	}
}
