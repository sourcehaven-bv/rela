package automation

import "github.com/Sourcehaven-BV/rela/internal/filter"

// AutomationBuilder provides a fluent interface for building test automations.
type AutomationBuilder struct {
	auto Automation
}

// newAutomation creates a new automation builder with optional name.
func newAutomation(name ...string) *AutomationBuilder {
	b := &AutomationBuilder{
		auto: Automation{},
	}
	if len(name) > 0 {
		b.auto.Name = name[0]
	}
	return b
}

// Name sets the automation name.
func (b *AutomationBuilder) Name(name string) *AutomationBuilder {
	b.auto.Name = name
	return b
}

// OnCreate triggers when entities of the given types are created.
func (b *AutomationBuilder) OnCreate(entityTypes ...string) *AutomationBuilder {
	b.auto.On.Entity = entityTypes
	b.auto.On.Created = true
	return b
}

// OnProperty triggers when a property changes to a value.
func (b *AutomationBuilder) OnProperty(entityType, property, becomes string) *AutomationBuilder {
	b.auto.On.Entity = []string{entityType}
	b.auto.On.Property = property
	b.auto.On.Becomes = becomes
	return b
}

// OnPropertyFrom triggers when a property changes from one value to another.
func (b *AutomationBuilder) OnPropertyFrom(entityType, property, from, becomes string) *AutomationBuilder {
	b.auto.On.Entity = []string{entityType}
	b.auto.On.Property = property
	b.auto.On.From = from
	b.auto.On.Becomes = becomes
	return b
}

// OnRelationCreated triggers when a relation of the given type is created.
func (b *AutomationBuilder) OnRelationCreated(relationType string) *AutomationBuilder {
	b.auto.On.RelationCreated = relationType
	return b
}

// When adds a filter condition to the trigger.
func (b *AutomationBuilder) When(condition string) *AutomationBuilder {
	f, err := filter.Parse(condition)
	if err != nil {
		panic("AutomationBuilder.When: invalid filter: " + condition)
	}
	b.auto.On.When = append(b.auto.On.When, f)
	return b
}

// Do adds an action to the automation.
func (b *AutomationBuilder) Do(action Action) *AutomationBuilder {
	b.auto.Do = append(b.auto.Do, action)
	return b
}

// Set adds a set-property action.
func (b *AutomationBuilder) Set(property, value string) *AutomationBuilder {
	return b.Do(Action{Set: property, Value: value})
}

// Lua adds an inline Lua action.
func (b *AutomationBuilder) Lua(code string) *AutomationBuilder {
	return b.Do(Action{Lua: code})
}

// LuaFile adds a Lua file action.
func (b *AutomationBuilder) LuaFile(path string) *AutomationBuilder {
	return b.Do(Action{LuaFile: path})
}

// CreateRelation adds a create-relation action.
func (b *AutomationBuilder) CreateRelation(relationType, to string) *AutomationBuilder {
	return b.Do(Action{
		CreateRelation: &CreateRelationAction{
			Relation: relationType,
			To:       to,
		},
	})
}

// CreateEntity adds a create-entity action.
func (b *AutomationBuilder) CreateEntity(entityType string, props map[string]string) *AutomationBuilder {
	return b.Do(Action{
		CreateEntity: &CreateEntityAction{
			Type:       entityType,
			Properties: props,
		},
	})
}

// CreateEntityWithRelation adds a create-entity action that links to the trigger.
func (b *AutomationBuilder) CreateEntityWithRelation(entityType, relation string, props map[string]string) *AutomationBuilder {
	return b.Do(Action{
		CreateEntity: &CreateEntityAction{
			Type:       entityType,
			Properties: props,
			Relation:   relation,
		},
	})
}

// Validate adds a validation rule.
func (b *AutomationBuilder) Validate(check, severity, message string) *AutomationBuilder {
	b.auto.Validate = append(b.auto.Validate, Validation{
		Check:    check,
		Severity: severity,
		Message:  message,
	})
	return b
}

// ValidateWarning adds a warning validation.
func (b *AutomationBuilder) ValidateWarning(check, message string) *AutomationBuilder {
	return b.Validate(check, "warning", message)
}

// Build returns the built automation.
func (b *AutomationBuilder) Build() Automation {
	return b.auto
}
