package automation

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
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

func TestEngine_WhenConditionMet(t *testing.T) {
	automations := []Automation{
		{
			Name: "docs-for-enhancements",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "review",
				When:     parseFilters(t, "kind=enhancement"),
			},
			Do: []Action{
				{Set: "needs_docs", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "in-progress"
	oldEntity.Properties["kind"] = "enhancement"

	newEntity := model.NewEntity("T-001", "ticket")
	newEntity.Properties["status"] = "review"
	newEntity.Properties["kind"] = "enhancement"

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
	automations := []Automation{
		{
			Name: "docs-for-enhancements",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "review",
				When:     parseFilters(t, "kind=enhancement"),
			},
			Do: []Action{
				{Set: "needs_docs", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "in-progress"
	oldEntity.Properties["kind"] = "bug" // Not an enhancement

	newEntity := model.NewEntity("T-001", "ticket")
	newEntity.Properties["status"] = "review"
	newEntity.Properties["kind"] = "bug"

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
	automations := []Automation{
		{
			Name: "high-priority-enhancements",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "review",
				When:     parseFilters(t, "kind=enhancement", "priority=high"),
			},
			Do: []Action{
				{Set: "urgent_review", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	// Test: both conditions met
	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "in-progress"
	oldEntity.Properties["kind"] = "enhancement"
	oldEntity.Properties["priority"] = "high"

	newEntity := model.NewEntity("T-001", "ticket")
	newEntity.Properties["status"] = "review"
	newEntity.Properties["kind"] = "enhancement"
	newEntity.Properties["priority"] = "high"

	result := engine.Process(Event{
		Type:      EventEntityUpdated,
		Entity:    newEntity,
		OldEntity: oldEntity,
	})

	if result.PropertiesSet["urgent_review"] != "true" {
		t.Error("expected trigger when all conditions are met")
	}

	// Test: only one condition met
	oldEntity2 := model.NewEntity("T-002", "ticket")
	oldEntity2.Properties["status"] = "in-progress"
	oldEntity2.Properties["kind"] = "enhancement"
	oldEntity2.Properties["priority"] = "low" // Not high

	newEntity2 := model.NewEntity("T-002", "ticket")
	newEntity2.Properties["status"] = "review"
	newEntity2.Properties["kind"] = "enhancement"
	newEntity2.Properties["priority"] = "low"

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
	automations := []Automation{
		{
			Name: "always-trigger",
			On: Trigger{
				Entity:   []string{"ticket"},
				Property: "status",
				Becomes:  "review",
				// No When conditions
			},
			Do: []Action{
				{Set: "reviewed", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	oldEntity := model.NewEntity("T-001", "ticket")
	oldEntity.Properties["status"] = "in-progress"

	newEntity := model.NewEntity("T-001", "ticket")
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
	automations := []Automation{
		{
			Name: "init-enhancement",
			On: Trigger{
				Entity:  []string{"ticket"},
				Created: true,
				When:    parseFilters(t, "kind=enhancement"),
			},
			Do: []Action{
				{Set: "needs_planning", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	// Enhancement ticket
	entity := model.NewEntity("T-001", "ticket")
	entity.Properties["kind"] = "enhancement"

	result := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: entity,
	})

	if result.PropertiesSet["needs_planning"] != "true" {
		t.Error("expected trigger on created when condition met")
	}

	// Bug ticket - should not trigger
	bugEntity := model.NewEntity("T-002", "ticket")
	bugEntity.Properties["kind"] = "bug"

	result2 := engine.Process(Event{
		Type:   EventEntityCreated,
		Entity: bugEntity,
	})

	if len(result2.PropertiesSet) > 0 {
		t.Error("expected no trigger on created when condition not met")
	}
}

func TestEngine_WhenConditionOnRelationCreated(t *testing.T) {
	automations := []Automation{
		{
			Name: "link-only-enhancements",
			On: Trigger{
				RelationCreated: "implements",
				When:            parseFilters(t, "kind=enhancement"),
			},
			Do: []Action{
				{Set: "has_impl", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	// Enhancement ticket - should trigger
	entity := model.NewEntity("T-001", "ticket")
	entity.Properties["kind"] = "enhancement"
	rel := model.NewRelation("S-001", "implements", "T-001")

	result := engine.Process(Event{
		Type:     EventRelationCreated,
		Entity:   entity,
		Relation: rel,
	})

	if result.PropertiesSet["has_impl"] != "true" {
		t.Error("expected trigger on relation created when condition met")
	}

	// Bug ticket - should not trigger
	bugEntity := model.NewEntity("T-002", "ticket")
	bugEntity.Properties["kind"] = "bug"
	rel2 := model.NewRelation("S-002", "implements", "T-002")

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
	automations := []Automation{
		{
			Name: "with-when",
			On: Trigger{
				Property: "status",
				Becomes:  "done",
				When:     parseFilters(t, "kind=enhancement"),
			},
			Do: []Action{
				{Set: "done", Value: "true"},
			},
		},
	}

	engine := NewEngine(automations)

	// Nil entity should not panic and should not fire
	result := engine.Process(Event{
		Type:   EventEntityUpdated,
		Entity: nil,
	})

	if len(result.PropertiesSet) > 0 {
		t.Error("expected no trigger when entity is nil")
	}
}

// parseFilters is a test helper to parse filter strings.
func parseFilters(t *testing.T, conditions ...string) []*filter.Filter {
	t.Helper()
	filters := make([]*filter.Filter, 0, len(conditions))
	for _, c := range conditions {
		f, err := filter.Parse(c)
		if err != nil {
			t.Fatalf("failed to parse filter %q: %v", c, err)
		}
		filters = append(filters, f)
	}
	return filters
}
