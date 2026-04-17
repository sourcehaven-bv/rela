package templating

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newFSTemplaterEnv(t *testing.T) (*FSTemplater, *project.Context) {
	t.Helper()
	tmp := t.TempDir()
	ctx := &project.Context{
		Root:                 tmp,
		EntitiesDir:          filepath.Join(tmp, "entities"),
		RelationsDir:         filepath.Join(tmp, "relations"),
		TemplatesDir:         filepath.Join(tmp, "templates"),
		EntityTemplatesDir:   filepath.Join(tmp, "templates", "entities"),
		RelationTemplatesDir: filepath.Join(tmp, "templates", "relations"),
	}
	return NewFSTemplater(storage.NewOsFS(), ctx), ctx
}

func TestFSTemplater_EntityTemplate_NotFound(t *testing.T) {
	tmpl, _ := newFSTemplaterEnv(t)
	out, err := tmpl.EntityTemplate(context.Background(), "requirement", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil, got %+v", out)
	}
}

func TestFSTemplater_EntityTemplate_Found(t *testing.T) {
	tmpl, pctx := newFSTemplaterEnv(t)
	if err := os.MkdirAll(pctx.EntityTemplatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
title: Default
status: draft
_template_relations:
  - relation: tagged
    target: feature
---

# Body

Template body.
`
	path := filepath.Join(pctx.EntityTemplatesDir, "ticket.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := tmpl.EntityTemplate(context.Background(), "ticket", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out == nil {
		t.Fatal("expected template, got nil")
	}
	if out.EntityType != "ticket" {
		t.Errorf("EntityType = %q, want ticket", out.EntityType)
	}
	if out.Properties["title"] != "Default" {
		t.Errorf("title = %v, want Default", out.Properties["title"])
	}
	if _, ok := out.Properties["_template_relations"]; ok {
		t.Error("_template_relations should be stripped from Properties")
	}
	if len(out.Relations) != 1 || out.Relations[0].Type != "tagged" || out.Relations[0].Target != "feature" {
		t.Errorf("Relations = %+v", out.Relations)
	}
}

func TestFSTemplater_RelationTemplate_Found(t *testing.T) {
	tmpl, pctx := newFSTemplaterEnv(t)
	if err := os.MkdirAll(pctx.RelationTemplatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
from: placeholder
relation: addresses
to: placeholder
severity: high
---

Why this relation exists.
`
	path := filepath.Join(pctx.RelationTemplatesDir, "addresses.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := tmpl.RelationTemplate(context.Background(), "addresses")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out == nil {
		t.Fatal("expected template, got nil")
	}
	if _, hasFrom := out.Properties["from"]; hasFrom {
		t.Error("from should be stripped")
	}
	if out.Properties["severity"] != "high" {
		t.Errorf("severity = %v, want high", out.Properties["severity"])
	}
}

func TestFSTemplater_EntityTemplates(t *testing.T) {
	tmpl, pctx := newFSTemplaterEnv(t)
	if err := os.MkdirAll(pctx.EntityTemplatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	defaultContent := `---
title: Default
---
Default body.
`
	bugContent := `---
title: Bug
priority: high
---
Bug body.
`
	_ = os.WriteFile(filepath.Join(pctx.EntityTemplatesDir, "ticket.md"), []byte(defaultContent), 0o644)
	_ = os.WriteFile(filepath.Join(pctx.EntityTemplatesDir, "ticket--bug.md"), []byte(bugContent), 0o644)

	templates, err := tmpl.EntityTemplates(context.Background(), "ticket")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}
	// Default first, then alphabetical
	if templates[0].Name != "" {
		t.Errorf("templates[0].Name = %q, want default (empty)", templates[0].Name)
	}
	if templates[1].Name != "bug" {
		t.Errorf("templates[1].Name = %q, want bug", templates[1].Name)
	}
}

func TestFSTemplater_GenerateEntity(t *testing.T) {
	tmpl, pctx := newFSTemplaterEnv(t)
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string", Default: "open"},
				},
			},
		},
	}

	created, err := tmpl.GenerateEntity(context.Background(), meta, "ticket", "", false)
	if err != nil {
		t.Fatalf("GenerateEntity error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}
	if _, err := os.Stat(filepath.Join(pctx.EntityTemplatesDir, "ticket.md")); err != nil {
		t.Errorf("template file not created: %v", err)
	}

	// Second call without force returns created=false.
	created, err = tmpl.GenerateEntity(context.Background(), meta, "ticket", "", false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if created {
		t.Error("expected created=false on re-run without force")
	}
}

func TestApplyEntity(t *testing.T) {
	t.Run("nil template returns inputs unchanged", func(t *testing.T) {
		props := map[string]interface{}{"k": "v"}
		gotProps, gotContent := ApplyEntity(props, "body", nil)
		if gotContent != "body" {
			t.Errorf("content = %q, want body", gotContent)
		}
		if gotProps["k"] != "v" {
			t.Errorf("props = %+v", gotProps)
		}
	})

	t.Run("fills in missing properties and empty content", func(t *testing.T) {
		tmpl := &Template{
			Properties: map[string]interface{}{
				"status":   "draft",
				"priority": "medium",
			},
			Content: "# Template body",
		}
		props := map[string]interface{}{"status": "open"}
		gotProps, gotContent := ApplyEntity(props, "", tmpl)
		// Existing key is preserved.
		if gotProps["status"] != "open" {
			t.Errorf("status = %v, want open (existing value wins)", gotProps["status"])
		}
		// Missing key is filled.
		if gotProps["priority"] != "medium" {
			t.Errorf("priority = %v, want medium", gotProps["priority"])
		}
		// Empty content is replaced.
		if gotContent != "# Template body" {
			t.Errorf("content = %q", gotContent)
		}
	})

	t.Run("non-empty content is preserved", func(t *testing.T) {
		tmpl := &Template{Content: "template body"}
		_, gotContent := ApplyEntity(nil, "user content", tmpl)
		if gotContent != "user content" {
			t.Errorf("content = %q, want user content", gotContent)
		}
	})
}

func TestApplyRelation(t *testing.T) {
	t.Run("nil template returns inputs unchanged", func(t *testing.T) {
		props := map[string]interface{}{"k": "v"}
		got := ApplyRelation(props, nil)
		if got["k"] != "v" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("merges defaults", func(t *testing.T) {
		tmpl := &Template{
			Properties: map[string]interface{}{
				"severity": "high",
				"reason":   "template default",
			},
		}
		props := map[string]interface{}{"severity": "low"}
		got := ApplyRelation(props, tmpl)
		if got["severity"] != "low" {
			t.Errorf("severity = %v, want existing low", got["severity"])
		}
		if got["reason"] != "template default" {
			t.Errorf("reason = %v", got["reason"])
		}
	})
}

func TestFSTemplater_GenerateRelation(t *testing.T) {
	tmpl, pctx := newFSTemplaterEnv(t)
	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{
			"addresses": {Label: "Addresses"},
		},
	}

	created, err := tmpl.GenerateRelation(context.Background(), meta, "addresses", false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}
	if _, err := os.Stat(filepath.Join(pctx.RelationTemplatesDir, "addresses.md")); err != nil {
		t.Errorf("template file not created: %v", err)
	}
}
