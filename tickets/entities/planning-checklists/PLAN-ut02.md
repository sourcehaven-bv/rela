---
id: PLAN-ut02
status: done
title: 'Planning: Add automatic entity creation to automation engine'
type: planning-checklist
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN SCOPE:
- New `create_entity` action type for automations
- Template support for created entities (using existing template system)
- Property interpolation from triggering entity (e.g., `{{entity.id}}`)
- Automatic relation creation between triggering entity and created entity
- Integration with workspace's entity creation flow

OUT OF SCOPE:
- Nested/cascading automation triggers (created entity triggering further automations)
- Conditional entity creation based on complex expressions
- Bulk entity creation (multiple entities per action)

**Acceptance Criteria:**

1. When an automation with `create_entity` action fires, a new entity of the specified type is created
2. The created entity uses templates if available for the entity type
3. Properties can be set on the created entity, with interpolation from the triggering entity
4. A relation can be automatically created between the triggering entity and the created entity
5. The automation result includes the created entity for caller visibility
6. Validation errors from entity creation are surfaced as automation errors

## Approach

- [x] Codebase researched (existing patterns, related code)
- [x] Technical approach chosen and documented
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### 1. Add `CreateEntityAction` type (internal/automation/types.go)

```go
type CreateEntityAction struct {
    Type       string            // Entity type to create (e.g., "planning-checklist")
    Properties map[string]string // Properties to set (values support interpolation)
    Relation   string            // Optional: relation type to create FROM triggering entity
}
```

### 2. Extend `Action` struct (internal/automation/types.go)

```go
type Action struct {
    Set            string
    Value          string
    CreateRelation *CreateRelationAction
    CreateEntity   *CreateEntityAction  // NEW
}
```

### 3. Add `EntitiesToCreate` to `Result` (internal/automation/types.go)

```go
type EntityToCreate struct {
    Type       string
    Properties map[string]interface{}
    RelationFromTrigger string // Relation type from triggering entity to this entity
}

type Result struct {
    PropertiesSet     map[string]string
    RelationsToCreate []*model.Relation
    EntitiesToCreate  []EntityToCreate  // NEW
    Warnings          []string
    Errors            []string
}
```

### 4. Add action execution in engine (internal/automation/engine.go)

In `executeAction()`, handle `CreateEntity`:
```go
if action.CreateEntity != nil {
    props := make(map[string]interface{})
    for k, v := range action.CreateEntity.Properties {
        props[k] = e.interpolate(v, event)
    }
    result.EntitiesToCreate = append(result.EntitiesToCreate, EntityToCreate{
        Type:                action.CreateEntity.Type,
        Properties:          props,
        RelationFromTrigger: action.CreateEntity.Relation,
    })
}
```

### 5. Handle entity creation in workspace (internal/workspace/workspace.go)

In `CreateEntity()` and `UpdateEntity()`, after processing automation results:
```go
for _, toCreate := range autoResult.EntitiesToCreate {
    created, _, createErr := w.CreateEntity(toCreate.Type, CreateOptions{
        Properties: toCreate.Properties,
    })
    if createErr != nil {
        log.Printf("Failed to create automation entity: %v", createErr)
        continue
    }
    result.EntitiesCreated = append(result.EntitiesCreated, created)
    
    if toCreate.RelationFromTrigger != "" {
        rel := model.NewRelation(entity.ID, toCreate.RelationFromTrigger, created.ID)
        // ... write relation
    }
}
```

### 6. Add metamodel types (internal/metamodel/types.go)

```go
type CreateEntityActionDef struct {
    Type       string            `yaml:"type"`
    Properties map[string]string `yaml:"properties,omitempty"`
    Relation   string            `yaml:"relation,omitempty"`
}

type AutomationAction struct {
    Set            string                 `yaml:"set,omitempty"`
    Value          string                 `yaml:"value,omitempty"`
    CreateRelation *CreateRelationAction  `yaml:"create_relation,omitempty"`
    CreateEntity   *CreateEntityActionDef `yaml:"create_entity,omitempty"` // NEW
}
```

**Example YAML usage:**

```yaml
automations:
  - name: create-planning-checklist
    on:
      entity: ticket
      property: status
      becomes: planning
    do:
      - create_entity:
          type: planning-checklist
          properties:
            title: "Planning: {{entity.title}}"
            status: in-progress
          relation: has-planning
```

**Files to modify:**

1. `internal/automation/types.go` - Add CreateEntityAction, EntityToCreate, extend Action and Result
2. `internal/automation/engine.go` - Handle create_entity action in executeAction()
3. `internal/metamodel/types.go` - Add CreateEntityActionDef, extend AutomationAction
4. `internal/workspace/workspace.go` - Handle EntitiesToCreate in CreateEntity/UpdateEntity results
5. `internal/automation/engine_test.go` - Add tests for create_entity action

**Alternatives Considered:**

1. **Deferred creation (return specs, let caller create)** - CHOSEN
   - Pros: Engine stays pure, testable without workspace
   - Cons: Caller must handle creation

2. **Direct creation in engine (inject workspace)**
   - Pros: Self-contained
   - Cons: Creates circular dependency, harder to test

3. **Event-based (emit events, external handler creates)**
   - Pros: Very decoupled
   - Cons: Over-engineered for this use case

**Dependencies:**

- `internal/model` - Entity, Relation types
- `internal/workspace` - CreateEntity for actual creation
- `internal/metamodel` - YAML parsing for new action type

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
|-----------|------|
| 1. Entity created on trigger | `TestEngine_CreateEntity_OnPropertyChange` |
| 2. Template support | Integration test in workspace_test.go |
| 3. Property interpolation | `TestEngine_CreateEntity_PropertyInterpolation` |
| 4. Relation creation | `TestEngine_CreateEntity_WithRelation` |
| 5. Result includes entity | Assert `EntitiesToCreate` populated in unit tests |
| 6. Validation errors surfaced | Integration test with invalid entity type |

**Edge Cases:**

- Empty entity type → Error
- Unknown entity type → Error (surfaced by workspace)
- Property interpolation with missing entity field → Empty string
- Relation type doesn't exist → Error (surfaced by workspace)
- Entity with same ID already exists → Error (workspace handles)
- Automation creates entity, which would trigger another automation → NOT supported (out of scope)

**Integration Tests:**

Add tests in `internal/workspace/workspace_test.go`:
- `TestWorkspace_UpdateEntity_CreatesAutomationEntity`
- `TestWorkspace_UpdateEntity_CreatesAutomationEntityWithRelation`

## Risk Assessment

- [x] Risks assessed with mitigations
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Impact | Mitigation |
|------|--------|------------|
| Infinite recursion (created entity triggers automation) | High | Explicitly don't process automations for automation-created entities |
| ID generation race conditions | Medium | Workspace locks handle this already |
| Breaking change to automation YAML | Low | New optional field, fully backward compatible |

**Effort: M** (medium)
- Types and engine changes: 2 hours
- Workspace integration: 2 hours  
- Tests: 2 hours
- Documentation: 1 hour
