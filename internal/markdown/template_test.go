package markdown

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

func setupTestContext(t *testing.T) *project.Context {
	t.Helper()
	tmpDir := t.TempDir()
	return &project.Context{
		Root:                 tmpDir,
		TemplatesDir:         filepath.Join(tmpDir, "templates"),
		EntityTemplatesDir:   filepath.Join(tmpDir, "templates", "entities"),
		RelationTemplatesDir: filepath.Join(tmpDir, "templates", "relations"),
	}
}

func TestLoadEntityTemplate_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	doc, err := LoadEntityTemplate(ctx, "requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil document for non-existent template, got %+v", doc)
	}
}

func TestLoadEntityTemplate_Success(t *testing.T) {
	ctx := setupTestContext(t)

	// Create template directory and file
	if err := os.MkdirAll(ctx.EntityTemplatesDir, 0755); err != nil {
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
	templatePath := ctx.EntityTemplatePath("requirement")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	doc, err := LoadEntityTemplate(ctx, "requirement")
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
	ctx := setupTestContext(t)

	doc, err := LoadRelationTemplate(ctx, "addresses")
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
	ctx := setupTestContext(t)

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
	created, err := GenerateEntityTemplate(ctx, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}

	// Verify file exists and has correct content
	templatePath := ctx.EntityTemplatePath("requirement")
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
	ctx := setupTestContext(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
	}

	// Create existing template
	if err := os.MkdirAll(ctx.EntityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	existingContent := "existing content"
	if err := os.WriteFile(ctx.EntityTemplatePath("requirement"), []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing template: %v", err)
	}

	// Try to generate without force
	created, err := GenerateEntityTemplate(ctx, meta, "requirement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected template NOT to be created (file exists)")
	}

	// Verify content unchanged
	content, _ := os.ReadFile(ctx.EntityTemplatePath("requirement"))
	if string(content) != existingContent {
		t.Errorf("content = %q, want %q (should not be overwritten)", string(content), existingContent)
	}
}

func TestGenerateEntityTemplate_ForceOverwrite(t *testing.T) {
	ctx := setupTestContext(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:      "Requirement",
				Properties: map[string]metamodel.PropertyDef{},
			},
		},
	}

	// Create existing template
	if err := os.MkdirAll(ctx.EntityTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(ctx.EntityTemplatePath("requirement"), []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write existing template: %v", err)
	}

	// Generate with force
	created, err := GenerateEntityTemplate(ctx, meta, "requirement", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created with force flag")
	}

	// Verify content changed
	content, _ := os.ReadFile(ctx.EntityTemplatePath("requirement"))
	if string(content) == "old" {
		t.Error("content should have been overwritten")
	}
}

func TestGenerateEntityTemplate_UnknownType(t *testing.T) {
	ctx := setupTestContext(t)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}

	_, err := GenerateEntityTemplate(ctx, meta, "unknown", false)
	if err == nil {
		t.Error("expected error for unknown entity type")
	}
}

func TestGenerateRelationTemplate(t *testing.T) {
	ctx := setupTestContext(t)

	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{
			"addresses": {
				Label: "Addresses",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}

	created, err := GenerateRelationTemplate(ctx, meta, "addresses", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}

	// Verify file exists
	templatePath := ctx.RelationTemplatePath("addresses")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Error("template file should exist")
	}
}

func TestGenerateRelationTemplate_UnknownType(t *testing.T) {
	ctx := setupTestContext(t)

	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{},
	}

	_, err := GenerateRelationTemplate(ctx, meta, "unknown", false)
	if err == nil {
		t.Error("expected error for unknown relation type")
	}
}
