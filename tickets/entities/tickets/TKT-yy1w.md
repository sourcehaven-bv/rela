---
id: TKT-yy1w
type: ticket
title: Implement relation properties with PropertySchema interface
kind: enhancement
priority: medium
effort: l
status: done
description: Add properties and content support to relations, using a shared PropertySchema interface to maximize code reuse with entity property handling
---

# Implementation Plan

## Phase 1: PropertySchema Interface (metamodel)

### 1.1 Add interface to `internal/metamodel/types.go`

```go
// PropertySchema abstracts property definitions for entities and relations
type PropertySchema interface {
    GetProperties() map[string]PropertyDef
    HasContent() bool
}
```

### 1.2 Implement for EntityDef

```go
func (e *EntityDef) GetProperties() map[string]PropertyDef { return e.Properties }
func (e *EntityDef) HasContent() bool                      { return true }
```

### 1.3 Extend RelationDef

```go
type RelationDef struct {
    // ... existing fields ...
    Properties map[string]PropertyDef `yaml:"properties,omitempty"`
    Content    bool                   `yaml:"content,omitempty"`
}

func (r *RelationDef) GetProperties() map[string]PropertyDef { return r.Properties }
func (r *RelationDef) HasContent() bool                      { return r.Content }

// Helper for UI detection
func (r *RelationDef) HasAdvancedFeatures() bool {
    return len(r.Properties) > 0 || r.Content
}
```

## Phase 2: Shared Validation (metamodel)

### 2.1 Extract shared validation in `internal/metamodel/validation.go`

```go
// ValidateProperties validates a properties map against a schema
func (m *Metamodel) ValidateProperties(
    props map[string]interface{},
    schema PropertySchema,
) []*ValidationError {
    var errs []*ValidationError

    for propName, propDef := range schema.GetProperties() {
        // Required check
        if propDef.Required {
            val, exists := props[propName]
            if !exists || val == nil || val == "" {
                errs = append(errs, &ValidationError{
                    Type:     ValidationErrorRequired,
                    Property: propName,
                    Message:  "This field is required",
                })
                continue
            }
        }

        // Type validation (reuse existing)
        val, exists := props[propName]
        if !exists || val == nil || val == "" {
            continue
        }
        if err := m.validatePropertyValue(propName, &propDef, val); err != nil {
            errs = append(errs, err)
        }
    }

    return errs
}
```

### 2.2 Refactor ValidateEntity to use shared function

```go
func (m *Metamodel) ValidateEntity(entity *model.Entity) []*ValidationError {
    def, ok := m.GetEntityDef(entity.Type)
    if !ok {
        return []*ValidationError{{Type: ValidationErrorUnknownType, ...}}
    }

    errs := m.ValidateProperties(entity.Properties, def)
    errs = append(errs, m.validateEntityID(entity, def)...)

    return errs
}
```

### 2.3 Add ValidateRelationProperties

```go
func (m *Metamodel) ValidateRelationProperties(rel *model.Relation) []*ValidationError {
    def, ok := m.Relations[rel.Type]
    if !ok {
        return nil // Unknown type handled elsewhere
    }
    return m.ValidateProperties(rel.Properties, &def)
}
```

## Phase 3: Loader Updates (metamodel)

### 3.1 Update `internal/metamodel/loader.go`

- Parse `properties` field on relation definitions
- Validate property types using existing `isKnownPropertyType()`
- Check for reserved property names (from, relation, to)

## Phase 4: Analysis Updates

### 4.1 Update `analyze_properties` command

- Include relation property validation in `rela analyze properties`
- Update MCP `analyze_properties` tool

## Phase 5: Data Entry UI

### 5.1 Update `internal/dataentry/form.go`

```go
type ResolvedRelation struct {
    // ... existing fields ...
    AdvancedMode bool // true if relation has properties or content
    PropertyDefs map[string]PropertyDef
    ContentEnabled bool
}

func (s *Server) resolveRelation(...) ResolvedRelation {
    relDef := s.meta.Relations[formRel.Relation]
    resolved.AdvancedMode = relDef.HasAdvancedFeatures()
    resolved.PropertyDefs = relDef.Properties
    resolved.ContentEnabled = relDef.Content
    // ...
}
```

### 5.2 Add templates

- `relation-cards.html` - card display with properties
- `relation-modal.html` - edit modal with property fields + content editor

### 5.3 Update form rendering

Template branching based on AdvancedMode:
```html
{{if .AdvancedMode}}
  {{template "relation-cards" .}}
{{else}}
  {{template "relation-select" .}}
{{end}}
```

## Phase 6: MCP Updates

### 6.1 Update `create_relation` tool

Add optional `properties` and `content` parameters to MCP create_relation tool.

## Testing

- Unit tests for PropertySchema interface
- Unit tests for ValidateProperties with both entity and relation schemas
- Loader tests for relation property parsing
- Data-entry E2E tests for cards/modal UI

## Files Changed

| File | Change |
|------|--------|
| `internal/metamodel/types.go` | Add PropertySchema interface, extend RelationDef |
| `internal/metamodel/validation.go` | Extract ValidateProperties, add ValidateRelationProperties |
| `internal/metamodel/loader.go` | Parse relation properties |
| `internal/cli/analyze.go` | Include relation properties in analyze |
| `internal/mcp/tools_analysis.go` | Update analyze_properties tool |
| `internal/mcp/tools_relation.go` | Add properties/content to create_relation |
| `internal/dataentry/form.go` | Add AdvancedMode detection |
| `internal/dataentry/templates/` | Add relation-cards, relation-modal templates |
