package automation

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestEngine_EntityCreated(t *testing.T) {
	automations := []Automation{
		{
			Name: "set-created-at",
			On: Trigger{
				Entity:  []string{"ticket"},
				Created: true,
			},
			Do: []Action{
				{Set: "created_at", Value: "{{today}}"},
			},
		},
	}

	engine := NewEngine(automations)
	engine.SetTemplateVars(TemplateVars{
		Now: func() time.Time { return time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC) },
	})

	entity := model.NewEntity("T-001", "ticket")
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if result.PropertiesSet["created_at"] != "2025-01-15" {
		t.Errorf("expected created_at=2025-01-15, got %q", result.PropertiesSet["created_at"])
	}
}

func TestEngine_EntityCreated_WrongType(t *testing.T) {
	automations := []Automation{
		{
			Name: "set-created-at",
			On: Trigger{
				Entity:  []string{"ticket"},
				Created: true,
			},
			Do: []Action{
				{Set: "created_at", Value: "{{today}}"},
			},
		},
	}

	engine := NewEngine(automations)

	// Entity of different type - should not trigger
	entity := model.NewEntity("B-001", "bug")
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no properties to be set for wrong entity type")
	}
}

func TestEngine_PropertyChange(t *testing.T) {
	automations := []Automation{
		{
			Name: "set-started-at",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "in-progress",
			},
			Do: []Action{
				{Set: "started_at", Value: "{{today}}"},
			},
		},
	}

	engine := NewEngine(automations)
	engine.SetTemplateVars(TemplateVars{
		Now: func() time.Time { return time.Date(2025, 2, 10, 10, 0, 0, 0, time.UTC) },
	})

	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "backlog"

	newEntity := model.NewEntity("T-001", "ticket")
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
	automations := []Automation{
		{
			Name: "set-started-at",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "in-progress",
			},
			Do: []Action{
				{Set: "started_at", Value: "{{today}}"},
			},
		},
	}

	engine := NewEngine(automations)

	// Status already "in-progress" - no change
	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "in-progress"

	newEntity := model.NewEntity("T-001", "ticket")
	newEntity.Properties["status"] = "in-progress"

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
	automations := []Automation{
		{
			Name: "only-from-backlog",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				From:     "backlog",
				Becomes:  "in-progress",
			},
			Do: []Action{
				{Set: "was_backlog", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	// From "backlog" to "in-progress" - should trigger
	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "backlog"

	newEntity := model.NewEntity("T-001", "ticket")
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
	oldEntity2 := model.NewEntity("T-002", "ticket")
	oldEntity2.Properties["status"] = "ready"

	newEntity2 := model.NewEntity("T-002", "ticket")
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
	automations := []Automation{
		{
			Name: "require-why1-for-in-progress-bugs",
			On: Trigger{
				Entity:   []string{"bug"},
				Property: "status",
				Becomes:  "in-progress",
			},
			Validate: []Validation{
				{
					Check:    "why1!=",
					Severity: "warning",
					Message:  "Please add a why1 analysis",
				},
			},
		},
	}

	engine := NewEngine(automations)

	oldEntity := model.NewEntity("B-001", "bug")
	oldEntity.Properties["status"] = "backlog"

	newEntity := model.NewEntity("B-001", "bug")
	newEntity.Properties["status"] = "in-progress"
	// why1 is empty

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
	automations := []Automation{
		{
			Name: "require-why1-for-in-progress-bugs",
			On: Trigger{
				Entity:   []string{"bug"},
				Property: "status",
				Becomes:  "in-progress",
			},
			Validate: []Validation{
				{
					Check:    "why1!=",
					Severity: "warning",
					Message:  "Please add a why1 analysis",
				},
			},
		},
	}

	engine := NewEngine(automations)

	oldEntity := model.NewEntity("B-001", "bug")
	oldEntity.Properties["status"] = "backlog"

	newEntity := model.NewEntity("B-001", "bug")
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
	automations := []Automation{
		{
			Name: "link-to-current-sprint",
			On: Trigger{
				Entity:  []string{"ticket"},
				Created: true,
			},
			Do: []Action{
				{
					CreateRelation: &CreateRelationAction{
						Relation: "belongs-to",
						To:       "sprint-current",
					},
				},
			},
		},
	}

	engine := NewEngine(automations)

	entity := model.NewEntity("T-001", "ticket")
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.RelationsToCreate) != 1 {
		t.Fatalf("expected 1 relation to create, got %d", len(result.RelationsToCreate))
	}
	rel := result.RelationsToCreate[0]
	if rel.From != "T-001" || rel.Type != "belongs-to" || rel.To != "sprint-current" {
		t.Errorf("unexpected relation: %+v", rel)
	}
}

func TestEngine_MultipleEntityTypes(t *testing.T) {
	automations := []Automation{
		{
			Name: "mark-created",
			On: Trigger{
				Entity:  []string{"ticket", "bug", "feature"},
				Created: true,
			},
			Do: []Action{
				{Set: "created", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	for _, entityType := range []string{"ticket", "bug", "feature"} {
		entity := model.NewEntity("E-001", entityType)
		result := engine.Process(Event{
			Type:   EventEntityCreated,
			Entity: entity,
		})

		if result.PropertiesSet["created"] != "true" {
			t.Errorf("expected trigger for entity type %s", entityType)
		}
	}

	// Other type should not trigger
	entity := model.NewEntity("D-001", "decision")
	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no trigger for decision type")
	}
}

func TestEngine_RelationCreated(t *testing.T) {
	automations := []Automation{
		{
			Name: "mark-linked",
			On: Trigger{
				RelationCreated: "implements",
			},
			Do: []Action{
				{Set: "has_implementation", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	entity := model.NewEntity("T-001", "ticket")
	rel := model.NewRelation("S-001", "implements", "T-001")

	result := engine.Process(Event{
		Type:     EventRelationCreated,
		Entity:   entity,
		Relation: rel,
	})

	if result.PropertiesSet["has_implementation"] != "true" {
		t.Error("expected trigger on relation created")
	}
}

func TestEngine_CreateEntity(t *testing.T) {
	automations := []Automation{
		{
			Name: "create-checklist-on-planning",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "planning",
			},
			Do: []Action{
				{
					CreateEntity: &CreateEntityAction{
						Type: "planning-checklist",
						ID:   "PLAN-{{entity.id}}",
						Properties: map[string]string{
							"title":  "Planning: {{entity.title}}",
							"status": "pending",
						},
						Content: "## Tasks\n\n- [ ] Review requirements",
					},
				},
			},
		},
	}

	engine := NewEngine(automations)

	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "ready"
	oldEntity.Properties["title"] = "New Feature"

	newEntity := model.NewEntity("T-001", "ticket")
	newEntity.Properties["status"] = "planning"
	newEntity.Properties["title"] = "New Feature"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	spec := result.EntitiesToCreate[0]
	if spec.Type != "planning-checklist" {
		t.Errorf("expected type=planning-checklist, got %q", spec.Type)
	}
	if spec.ID != "PLAN-T-001" {
		t.Errorf("expected ID=PLAN-T-001, got %q", spec.ID)
	}
	if spec.Properties["title"] != "Planning: New Feature" {
		t.Errorf("expected title='Planning: New Feature', got %q", spec.Properties["title"])
	}
	if spec.Properties["status"] != "pending" {
		t.Errorf("expected status=pending, got %q", spec.Properties["status"])
	}
	if spec.Content != "## Tasks\n\n- [ ] Review requirements" {
		t.Errorf("unexpected content: %q", spec.Content)
	}
}

func TestEngine_CreateEntity_NoIDProvided(t *testing.T) {
	automations := []Automation{
		{
			Name: "create-checklist-on-planning",
			On: Trigger{
				Entity:  []string{"ticket"},
				Created: true,
			},
			Do: []Action{
				{
					CreateEntity: &CreateEntityAction{
						Type: "checklist",
						// ID not provided - should be empty in spec
						Properties: map[string]string{
							"title": "Default Checklist",
						},
					},
				},
			},
		},
	}

	engine := NewEngine(automations)

	entity := model.NewEntity("T-001", "ticket")

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if len(result.EntitiesToCreate) != 1 {
		t.Fatalf("expected 1 entity to create, got %d", len(result.EntitiesToCreate))
	}

	spec := result.EntitiesToCreate[0]
	if spec.ID != "" {
		t.Errorf("expected empty ID (to be auto-generated), got %q", spec.ID)
	}
	if spec.Type != "checklist" {
		t.Errorf("expected type=checklist, got %q", spec.Type)
	}
}
