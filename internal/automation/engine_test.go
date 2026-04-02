package automation

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestEngine_EntityCreated(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("set-created-at").
			OnCreate("ticket").
			Set("created_at", "{{today}}").
			Build(),
	})
	engine.SetTemplateVars(TemplateVars{
		Now: func() time.Time { return time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC) },
	})

	entity := testutil.Entity("ticket").Build()
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if result.PropertiesSet["created_at"] != "2025-01-15" {
		t.Errorf("expected created_at=2025-01-15, got %q", result.PropertiesSet["created_at"])
	}
}

func TestEngine_EntityCreated_WrongType(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().OnCreate("ticket").Set("created_at", "{{today}}").Build(),
	})

	// Entity of different type - should not trigger
	entity := testutil.Entity("bug").Build()
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no properties to be set for wrong entity type")
	}
}

func TestEngine_PropertyChange(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnProperty("ticket", "status", "in-progress").
			Set("started_at", "{{today}}").
			Build(),
	})
	engine.SetTemplateVars(TemplateVars{
		Now: func() time.Time { return time.Date(2025, 2, 10, 10, 0, 0, 0, time.UTC) },
	})

	oldEntity := testutil.Entity("ticket").With("status", "backlog").Build()
	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "in-progress"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.PropertiesSet["started_at"] != "2025-02-10" {
		t.Errorf("expected started_at=2025-02-10, got %q", result.PropertiesSet["started_at"])
	}
}

func TestEngine_PropertyChange_NoChange(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnProperty("ticket", "status", "in-progress").
			Set("started_at", "{{today}}").
			Build(),
	})

	// Status already "in-progress" - no change
	oldEntity := testutil.Entity("ticket").With("status", "in-progress").Build()
	newEntity := oldEntity.Clone()

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no properties to be set when value doesn't change")
	}
}

func TestEngine_PropertyChange_FromConstraint(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnPropertyFrom("ticket", "status", "backlog", "in-progress").
			Set("was_backlog", "true").
			Build(),
	})

	// From "backlog" to "in-progress" - should trigger
	oldEntity := testutil.Entity("ticket").With("status", "backlog").Build()
	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "in-progress"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.PropertiesSet["was_backlog"] != "true" {
		t.Error("expected trigger when changing from backlog")
	}

	// From "ready" to "in-progress" - should NOT trigger
	oldEntity2 := testutil.Entity("ticket").With("status", "ready").Build()
	newEntity2 := oldEntity2.Clone()
	newEntity2.Properties["status"] = "in-progress"

	result2 := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity2,
		OldEntity: oldEntity2,
	})

	if len(result2.PropertiesSet) > 0 {
		t.Error("expected no trigger when not changing from backlog")
	}
}

func TestEngine_ValidationWarning(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnProperty("bug", "status", "in-progress").
			ValidateWarning("why1!=", "Please add a why1 analysis").
			Build(),
	})

	oldEntity := testutil.Entity("bug").With("status", "backlog").Build()
	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "in-progress"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if !result.HasWarnings() {
		t.Error("expected warning for missing why1")
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "Please add a why1 analysis" {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

func TestEngine_ValidationPasses(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnProperty("bug", "status", "in-progress").
			ValidateWarning("why1!=", "Please add a why1 analysis").
			Build(),
	})

	oldEntity := testutil.Entity("bug").With("status", "backlog").Build()
	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "in-progress"
	newEntity.Properties["why1"] = "Database connection timeout"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.HasWarnings() {
		t.Errorf("expected no warnings when why1 is set, got: %v", result.Warnings)
	}
}

func TestEngine_CreateRelation(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnCreate("ticket").
			CreateRelation("belongs-to", "sprint-current").
			Build(),
	})

	entity := testutil.Entity("ticket").Build()
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.RelationsToCreate) != 1 {
		t.Fatalf("expected 1 relation to create, got %d", len(result.RelationsToCreate))
	}
	rel := result.RelationsToCreate[0]
	if rel.From != entity.ID || rel.Type != "belongs-to" || rel.To != "sprint-current" {
		t.Errorf("unexpected relation: %+v", rel)
	}
}

func TestEngine_MultipleEntityTypes(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("mark-created").
			OnCreate("ticket", "bug", "feature").
			Set("created", "true").
			Build(),
	})

	for _, entityType := range []string{"ticket", "bug", "feature"} {
		entity := testutil.Entity(entityType).ID("E-001").Build()
		result := engine.Process(Event{
			Type:   EventEntityCreated,
			Entity: entity,
		})

		if result.PropertiesSet["created"] != "true" {
			t.Errorf("expected trigger for entity type %s", entityType)
		}
	}

	// Other type should not trigger
	entity := testutil.Entity("decision").ID("D-001").Build()
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no trigger for decision type")
	}
}

func TestEngine_RelationCreated(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("mark-linked").
			OnRelationCreated("implements").
			Set("has_implementation", "true").
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()
	rel := testutil.NewRelation("S-001", "implements", "T-001").Build()

	result := engine.Process(Event{
		Type:     EventRelationCreated,
		Entity:   entity,
		Relation: rel,
	})

	if result.PropertiesSet["has_implementation"] != "true" {
		t.Error("expected trigger on relation created")
	}
}

func TestEngine_CreateEntity_OnPropertyChange(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-planning-checklist").
			OnProperty("ticket", "status", "planning").
			CreateEntityWithRelation("planning-checklist", "has-planning", map[string]string{
				"title":  "Planning: {{new.title}}",
				"status": "pending",
			}).
			Build(),
	})

	oldEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "backlog").
		With("title", "Implement feature X").
		Build()

	newEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "planning").
		With("title", "Implement feature X").
		Build()

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.Type != "planning-checklist" {
		t.Errorf("expected type planning-checklist, got %s", toCreate.Type)
	}
	if toCreate.Properties["title"] != "Planning: Implement feature X" {
		t.Errorf("expected interpolated title, got %v", toCreate.Properties["title"])
	}
	if toCreate.Properties["status"] != "pending" {
		t.Errorf("expected status pending, got %v", toCreate.Properties["status"])
	}
	if toCreate.RelationFromTrigger != "has-planning" {
		t.Errorf("expected relation has-planning, got %s", toCreate.RelationFromTrigger)
	}
}

func TestEngine_CreateEntity_OnCreated(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-default-checklist").
			OnCreate("ticket").
			CreateEntity("checklist", map[string]string{
				"title": "Checklist for {{entity.id}}",
			}).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").With("title", "New ticket").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.Type != "checklist" {
		t.Errorf("expected type checklist, got %s", toCreate.Type)
	}
	if toCreate.Properties["title"] != "Checklist for T-001" {
		t.Errorf("expected interpolated title with entity ID, got %v", toCreate.Properties["title"])
	}
}

func TestEngine_CreateEntity_NoRelation(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-without-relation").
			OnCreate("ticket").
			CreateEntity("note", map[string]string{
				"content": "Auto-generated note",
			}).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.RelationFromTrigger != "" {
		t.Errorf("expected no relation, got %s", toCreate.RelationFromTrigger)
	}
}

func TestEngine_CreateEntity_MissingType(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-invalid").
			OnCreate("ticket").
			CreateEntityAction(&CreateEntityAction{
				// Type is missing
				Properties: map[string]string{
					"title": "Should fail",
				},
			}).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 0 {
		t.Error("expected no entities to create when type is missing")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestEngine_CreateEntity_IfExistsDefaultsToSkip(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-checklist").
			OnCreate("ticket").
			CreateEntityWithRelation("checklist", "has-checklist", nil).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.IfExists != IfExistsSkip {
		t.Errorf("expected IfExists to default to 'skip', got %q", toCreate.IfExists)
	}
}

func TestEngine_CreateEntity_IfExistsExplicit(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-with-error").
			OnCreate("ticket").
			CreateEntityAction(&CreateEntityAction{
				Type:     "checklist",
				Relation: "has-checklist",
				IfExists: IfExistsError,
			}).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.IfExists != IfExistsError {
		t.Errorf("expected IfExists to be 'error', got %q", toCreate.IfExists)
	}
}

func TestEngine_CreateEntity_WithTemplate(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-checklist-with-template").
			OnProperty("ticket", "status", "planning").
			CreateEntityAction(&CreateEntityAction{
				Type:     "planning-checklist",
				Template: "{{new.kind}}", // Interpolate from entity property
				Properties: map[string]string{
					"title": "Planning: {{new.title}}",
				},
				Relation: "has-planning",
			}).
			Build(),
	})

	oldEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "backlog").
		With("kind", "enhancement").
		With("title", "Add new feature").
		Build()

	newEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "planning").
		With("kind", "enhancement").
		With("title", "Add new feature").
		Build()

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.Template != "enhancement" {
		t.Errorf("expected template 'enhancement', got %q", toCreate.Template)
	}
	if toCreate.Properties["title"] != "Planning: Add new feature" {
		t.Errorf("expected interpolated title, got %v", toCreate.Properties["title"])
	}
}

func TestEngine_CreateEntity_TemplateEmpty(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("create-checklist-no-template").
			OnCreate("ticket").
			CreateEntity("checklist", nil).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	if toCreate.Template != "" {
		t.Errorf("expected empty template, got %q", toCreate.Template)
	}
}

func TestEngine_CreateEntity_TemplateMissingProperty(t *testing.T) {
	// When the property used in template interpolation doesn't exist,
	// the template becomes empty string (uses default template).
	engine := NewEngine([]Automation{
		newAutomation("create-with-missing-property").
			OnCreate("ticket").
			CreateEntityAction(&CreateEntityAction{
				Type:     "checklist",
				Template: "{{new.kind}}", // kind property not set on entity
			}).
			Build(),
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	toCreate := result.EntitiesToCreate[0]
	// Missing property interpolates to empty string -> uses default template
	if toCreate.Template != "" {
		t.Errorf("expected empty template for missing property, got %q", toCreate.Template)
	}
}

func TestEngine_CreateEntity_TemplatePathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		wantErr  bool
		template string
	}{
		// Valid templates (allowlist: a-z, A-Z, 0-9, -, _)
		{"valid template", "enhancement", false, "enhancement"},
		{"valid with hyphen", "my-template", false, "my-template"},
		{"valid with underscore", "template_v2", false, "template_v2"},
		{"valid uppercase", "MyTemplate", false, "MyTemplate"},
		{"valid mixed", "Template-V2_final", false, "Template-V2_final"},
		{"empty kind becomes empty template", "", false, ""},

		// Invalid: path traversal attempts
		{"path traversal with ..", "../../../etc/passwd", true, ""},
		{"path with forward slash", "foo/bar", true, ""},
		{"path with backslash", "foo\\bar", true, ""},
		{"double dots in middle", "foo..bar", true, ""},

		// Invalid: null byte injection
		{"null byte injection", "valid\x00../etc/passwd", true, ""},
		{"null byte only", "\x00", true, ""},

		// Invalid: special characters blocked by allowlist
		{"dots only", "...", true, ""},
		{"single dot", ".", true, ""},
		{"unicode characters", "template-名前", true, ""},
		{"space in name", "my template", true, ""},
		{"colon in name", "foo:bar", true, ""},
		{"semicolon in name", "foo;bar", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewEngine([]Automation{
				newAutomation("create-with-template").
					OnCreate("ticket").
					CreateEntityAction(&CreateEntityAction{
						Type:     "checklist",
						Template: "{{new.kind}}", // Interpolate from entity property
					}).
					Build(),
			})

			entity := testutil.Entity("ticket").ID("T-001").With("kind", tc.kind).Build()

			result := engine.Process(Event{
				Type:   EventEntityCreated,
				Entity: entity,
			})

			hasError := len(result.Errors) > 0
			entityCount := len(result.EntitiesToCreate)

			if tc.wantErr && !hasError {
				t.Error("expected error for path traversal attempt")
			}
			if tc.wantErr && entityCount != 0 {
				t.Error("expected no entities to create on error")
			}
			if !tc.wantErr && hasError {
				t.Errorf("unexpected errors: %v", result.Errors)
			}
			if !tc.wantErr && entityCount != 1 {
				t.Fatalf("expected 1 entity, got %d", entityCount)
			}
			if !tc.wantErr && entityCount == 1 && result.EntitiesToCreate[0].Template != tc.template {
				t.Errorf("expected template %q, got %q", tc.template, result.EntitiesToCreate[0].Template)
			}
		})
	}
}

func TestEngine_WhenConditionMet(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("docs-for-enhancements").
			OnProperty("ticket", "status", "review").
			When("kind=enhancement").
			Set("needs_docs", "true").
			Build(),
	})

	oldEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "in-progress").
		With("kind", "enhancement").
		Build()

	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "review"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.PropertiesSet["needs_docs"] != "true" {
		t.Error("expected trigger to fire when condition is met")
	}
}

func TestEngine_WhenConditionNotMet(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("docs-for-enhancements").
			OnProperty("ticket", "status", "review").
			When("kind=enhancement").
			Set("needs_docs", "true").
			Build(),
	})

	oldEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "in-progress").
		With("kind", "bug").
		Build()

	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "review"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected trigger NOT to fire when condition is not met")
	}
}

func TestEngine_MultipleWhenConditions(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("high-priority-enhancements").
			OnProperty("ticket", "status", "review").
			When("kind=enhancement").
			When("priority=high").
			Set("urgent_review", "true").
			Build(),
	})

	// Test: both conditions met
	oldEntity := testutil.Entity("ticket").ID("T-001").
		With("status", "in-progress").
		With("kind", "enhancement").
		With("priority", "high").
		Build()

	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "review"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.PropertiesSet["urgent_review"] != "true" {
		t.Error("expected trigger when all conditions are met")
	}

	// Test: only one condition met
	oldEntity2 := testutil.Entity("ticket").ID("T-002").
		With("status", "in-progress").
		With("kind", "enhancement").
		With("priority", "low").
		Build()

	newEntity2 := oldEntity2.Clone()
	newEntity2.Properties["status"] = "review"

	result2 := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity2,
		OldEntity: oldEntity2,
	})

	if len(result2.PropertiesSet) > 0 {
		t.Error("expected trigger NOT to fire when only one condition met")
	}
}

func TestEngine_NoWhenConditions(t *testing.T) {
	// Backward compatibility: no when conditions = always match
	engine := NewEngine([]Automation{
		newAutomation("always-trigger").
			OnProperty("ticket", "status", "review").
			Set("reviewed", "true").
			Build(),
	})

	oldEntity := testutil.Entity("ticket").ID("T-001").With("status", "in-progress").Build()

	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "review"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.PropertiesSet["reviewed"] != "true" {
		t.Error("expected trigger without when conditions to fire")
	}
}

func TestEngine_WhenConditionOnCreated(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("init-enhancement").
			OnCreate("ticket").
			When("kind=enhancement").
			Set("needs_planning", "true").
			Build(),
	})

	// Enhancement ticket
	entity := testutil.Entity("ticket").ID("T-001").With("kind", "enhancement").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if result.PropertiesSet["needs_planning"] != "true" {
		t.Error("expected trigger on created when condition met")
	}

	// Bug ticket - should not trigger
	bugEntity := testutil.Entity("ticket").ID("T-002").With("kind", "bug").Build()

	result2 := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: bugEntity,
	})

	if len(result2.PropertiesSet) > 0 {
		t.Error("expected no trigger on created when condition not met")
	}
}

func TestEngine_WhenConditionOnRelationCreated(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("link-only-enhancements").
			OnRelationCreated("implements").
			When("kind=enhancement").
			Set("has_impl", "true").
			Build(),
	})

	// Enhancement ticket - should trigger
	entity := testutil.Entity("ticket").ID("T-001").With("kind", "enhancement").Build()
	rel := testutil.NewRelation("S-001", "implements", "T-001").Build()

	result := engine.Process(Event{
		Type:     EventRelationCreated,
		Entity:   entity,
		Relation: rel,
	})

	if result.PropertiesSet["has_impl"] != "true" {
		t.Error("expected trigger on relation created when condition met")
	}

	// Bug ticket - should not trigger
	bugEntity := testutil.Entity("ticket").ID("T-002").With("kind", "bug").Build()
	rel2 := testutil.NewRelation("S-002", "implements", "T-002").Build()

	result2 := engine.Process(Event{
		Type:     EventRelationCreated,
		Entity:   bugEntity,
		Relation: rel2,
	})

	if len(result2.PropertiesSet) > 0 {
		t.Error("expected no trigger on relation created when condition not met")
	}
}

func TestEngine_WhenConditionNilEntity(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("with-when").
			OnProperty("", "status", "done").
			When("kind=enhancement").
			Set("done", "true").
			Build(),
	})

	// Nil entity should not panic and should not fire
	result := engine.Process(Event{
		Type:   EventEntityUpdated,
		Entity: nil,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no trigger when entity is nil")
	}
}

func TestEngine_LuaInline(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnProperty("ticket", "status", "done").
			Lua(`-- this is a lua script`).
			Build(),
	})

	oldEntity := testutil.Entity("ticket").With("status", "in-progress").Build()
	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "done"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}

	if result.LuaToExecute[0].Code != `-- this is a lua script` {
		t.Errorf("unexpected Lua code: %q", result.LuaToExecute[0].Code)
	}
	if result.LuaToExecute[0].FilePath != "" {
		t.Errorf("expected empty file path, got: %q", result.LuaToExecute[0].FilePath)
	}
}

func TestEngine_LuaFile(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnProperty("ticket", "status", "archived").
			LuaFile("archive_notify.lua").
			Build(),
	})

	oldEntity := testutil.Entity("ticket").With("status", "done").Build()
	newEntity := oldEntity.Clone()
	newEntity.Properties["status"] = "archived"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}

	if result.LuaToExecute[0].FilePath != "archive_notify.lua" {
		t.Errorf("unexpected file path: %q", result.LuaToExecute[0].FilePath)
	}
	if result.LuaToExecute[0].Code != "" {
		t.Errorf("expected empty code, got: %q", result.LuaToExecute[0].Code)
	}
}

func TestEngine_LuaInlineWithSafeInterpolation(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation("run-lua-with-vars").
			OnCreate("ticket").
			Lua(`local date = "{{today}}"
local user = "{{user.name}}"`).
			Build(),
	})
	engine.SetTemplateVars(TemplateVars{
		Now:  func() time.Time { return time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC) },
		User: UserVars{Name: "Alice", Email: "alice@example.com"},
	})

	entity := testutil.Entity("ticket").ID("T-001").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}

	expectedCode := `local date = "2025-03-15"
local user = "Alice"`

	if result.LuaToExecute[0].Code != expectedCode {
		t.Errorf("expected safe interpolation, got: %q", result.LuaToExecute[0].Code)
	}
}

func TestEngine_LuaInlineDoesNotInterpolateEntityProperties(t *testing.T) {
	// Security test: entity properties should NOT be interpolated into Lua code
	engine := NewEngine([]Automation{
		newAutomation("check-no-entity-interpolation").
			OnCreate("ticket").
			Lua(`local title = "{{new.title}}"`).
			Build(),
	})

	// Even if title contains dangerous Lua code, it should NOT be interpolated
	entity := testutil.Entity("ticket").ID("T-001").
		With("title", `"; os.execute("rm -rf /"); --`).
		Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}

	// Entity properties should NOT be interpolated - template stays as-is
	if result.LuaToExecute[0].Code != `local title = "{{new.title}}"` {
		t.Errorf("entity properties should not be interpolated, got: %q", result.LuaToExecute[0].Code)
	}
}

func TestEngine_LuaOnCreated(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnCreate("ticket").
			Lua(`rela.update_entity(entity.id, {initialized = "true"})`).
			Build(),
	})

	entity := testutil.Entity("ticket").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}
}

func TestEngine_LuaEmptyAction(t *testing.T) {
	// Both Lua and LuaFile empty - should not add to LuaToExecute
	engine := NewEngine([]Automation{
		newAutomation().
			OnCreate("ticket").
			Do(Action{Lua: "", LuaFile: ""}). // Empty action
			Build(),
	})

	entity := testutil.Entity("ticket").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.LuaToExecute) != 0 {
		t.Errorf("expected no Lua actions for empty Lua/LuaFile, got %d", len(result.LuaToExecute))
	}
}

func TestEngine_LuaMultipleActions(t *testing.T) {
	engine := NewEngine([]Automation{
		newAutomation().
			OnCreate("ticket").
			Lua(`-- action 1`).
			LuaFile("action2.lua").
			Lua(`-- action 3`).
			Build(),
	})

	entity := testutil.Entity("ticket").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.LuaToExecute) != 3 {
		t.Fatalf("expected 3 Lua actions, got %d", len(result.LuaToExecute))
	}

	if result.LuaToExecute[0].Code != `-- action 1` {
		t.Errorf("action 1: unexpected code")
	}
	if result.LuaToExecute[1].FilePath != "action2.lua" {
		t.Errorf("action 2: unexpected file path")
	}
	if result.LuaToExecute[2].Code != `-- action 3` {
		t.Errorf("action 3: unexpected code")
	}
}

func TestEngine_LuaFilePathPassthrough(t *testing.T) {
	// Test that lua_file paths are passed through to LuaToExecute.
	// Path validation is centralized in the script package at execution time.
	engine := NewEngine([]Automation{
		newAutomation().
			OnCreate("ticket").
			LuaFile("../../../etc/passwd").
			Build(),
	})

	entity := testutil.Entity("ticket").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	// Engine should pass through the path without errors.
	// Validation happens in the script package at execution time.
	if len(result.Errors) != 0 {
		t.Errorf("expected no engine errors (validation is done at execution time), got: %v", result.Errors)
	}

	// Path should be queued for execution.
	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}
	if result.LuaToExecute[0].FilePath != "../../../etc/passwd" {
		t.Errorf("expected path to be passed through, got: %s", result.LuaToExecute[0].FilePath)
	}
}

func TestEngine_LuaFileExtensionPassthrough(t *testing.T) {
	// Test that lua_file paths are passed through regardless of extension.
	// Extension validation is centralized in the script package at execution time.
	engine := NewEngine([]Automation{
		newAutomation().
			OnCreate("ticket").
			LuaFile("script.txt").
			Build(),
	})

	entity := testutil.Entity("ticket").Build()

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	// Engine should pass through the path without errors.
	// Extension validation happens in the script package at execution time.
	if len(result.Errors) != 0 {
		t.Errorf("expected no engine errors (validation is done at execution time), got: %v", result.Errors)
	}

	// Path should be queued for execution.
	if len(result.LuaToExecute) != 1 {
		t.Fatalf("expected 1 Lua action, got %d", len(result.LuaToExecute))
	}
	if result.LuaToExecute[0].FilePath != "script.txt" {
		t.Errorf("expected path to be passed through, got: %s", result.LuaToExecute[0].FilePath)
	}
}
