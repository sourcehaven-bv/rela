package automation

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Engine evaluates automations against entity events.
type Engine struct {
	automations []Automation
	vars        TemplateVars
}

// NewEngine creates an automation engine with the given automations.
func NewEngine(automations []Automation) *Engine {
	return &Engine{
		automations: automations,
		vars:        DefaultTemplateVars(),
	}
}

// NewEngineFromMetamodel creates an automation engine from metamodel definitions.
func NewEngineFromMetamodel(defs []metamodel.AutomationDef) *Engine {
	automations := make([]Automation, len(defs))
	for i, def := range defs {
		automations[i] = convertFromMetamodel(def)
	}
	return NewEngine(automations)
}

// convertFromMetamodel converts a metamodel AutomationDef to the internal Automation type.
func convertFromMetamodel(def metamodel.AutomationDef) Automation {
	// Parse when conditions
	// Note: Invalid filters are silently skipped. This could be improved by
	// validating at metamodel load time (requires breaking filter/metamodel import cycle).
	whenFilters := make([]*filter.Filter, 0, len(def.On.When))
	for _, w := range def.On.When {
		f, err := filter.Parse(w)
		if err != nil {
			// Skip invalid conditions - this automation will have fewer constraints
			// than intended, which is safer than having no constraints at all
			continue
		}
		whenFilters = append(whenFilters, f)
	}

	auto := Automation{
		Name:        def.Name,
		Description: def.Description,
		On: Trigger{
			Entity:          []string(def.On.Entity),
			Property:        def.On.Property,
			Becomes:         def.On.Becomes,
			From:            def.On.From,
			Created:         def.On.Created,
			RelationCreated: def.On.RelationCreated,
			RelationRemoved: def.On.RelationRemoved,
			When:            whenFilters,
		},
		Do:       make([]Action, len(def.Do)),
		Validate: make([]Validation, len(def.Validate)),
	}

	for i, a := range def.Do {
		action := Action{
			Set:     a.Set,
			Value:   a.Value,
			Lua:     a.Lua,
			LuaFile: a.LuaFile,
		}
		if a.CreateRelation != nil {
			action.CreateRelation = &CreateRelationAction{
				Relation: a.CreateRelation.Relation,
				To:       a.CreateRelation.To,
			}
		}
		if a.CreateEntity != nil {
			action.CreateEntity = &CreateEntityAction{
				Type:       a.CreateEntity.Type,
				Template:   a.CreateEntity.Template,
				Properties: a.CreateEntity.Properties,
				Relation:   a.CreateEntity.Relation,
				IfExists:   a.CreateEntity.IfExists,
			}
		}
		auto.Do[i] = action
	}

	for i, v := range def.Validate {
		auto.Validate[i] = Validation{
			Check:    v.Check,
			Severity: v.Severity,
			Message:  v.Message,
		}
	}

	return auto
}

// SetTemplateVars sets the template variables for interpolation.
func (e *Engine) SetTemplateVars(vars TemplateVars) {
	e.vars = vars
}

// Process evaluates all automations against an event and returns the result.
func (e *Engine) Process(event Event) *Result {
	result := &Result{
		PropertiesSet:     make(map[string]string),
		RelationsToCreate: make([]*entity.Relation, 0),
		EntitiesToCreate:  make([]EntityToCreate, 0),
		LuaToExecute:      make([]LuaToExecute, 0),
		Warnings:          make([]string, 0),
		Errors:            make([]string, 0),
	}

	for _, auto := range e.automations {
		if !e.matches(auto.On, event) {
			continue
		}

		// Execute actions
		for _, action := range auto.Do {
			e.executeAction(action, event, result)
		}

		// Evaluate validations
		for _, validation := range auto.Validate {
			e.evaluateValidation(validation, event, result)
		}
	}

	return result
}

// matches checks if a trigger matches an event.
func (e *Engine) matches(trigger Trigger, event Event) bool {
	// Check entity type constraint
	if len(trigger.Entity) > 0 && event.Entity != nil {
		matched := false
		for _, entityType := range trigger.Entity {
			if event.Entity.Type == entityType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check when conditions (property filters on the entity)
	if !e.matchesWhenConditions(trigger, event.Entity) {
		return false
	}

	switch event.Type {
	case EventEntityCreated:
		return trigger.Created

	case EventEntityUpdated:
		if trigger.Property == "" {
			return false
		}
		return e.matchesPropertyChange(trigger, event)

	case EventRelationCreated:
		if trigger.RelationCreated == "" {
			return false
		}
		return event.Relation != nil && event.Relation.Type == trigger.RelationCreated

	case EventRelationRemoved:
		if trigger.RelationRemoved == "" {
			return false
		}
		return event.Relation != nil && event.Relation.Type == trigger.RelationRemoved
	}

	return false
}

// matchesPropertyChange checks if a property change event matches the trigger.
func (e *Engine) matchesPropertyChange(trigger Trigger, event Event) bool {
	if event.Entity == nil {
		return false
	}

	newValue := event.Entity.GetString(trigger.Property)
	oldValue := ""
	if event.OldEntity != nil {
		oldValue = event.OldEntity.GetString(trigger.Property)
	}

	// No change occurred
	if newValue == oldValue {
		return false
	}

	// Check "from" constraint
	if trigger.From != "" && oldValue != trigger.From {
		return false
	}

	// Check "becomes" constraint
	if trigger.Becomes != "" && newValue != trigger.Becomes {
		return false
	}

	return true
}

// matchesWhenConditions checks if all when conditions are satisfied.
// Returns true if no conditions are specified (backward compatible).
func (e *Engine) matchesWhenConditions(trigger Trigger, entity *entity.Entity) bool {
	if len(trigger.When) == 0 {
		return true
	}
	if entity == nil {
		return false
	}

	for _, f := range trigger.When {
		val := entity.Properties[f.Property]
		if !matchSimple(val, f) {
			return false
		}
	}
	return true
}

// executeAction performs an action and updates the result.
func (e *Engine) executeAction(action Action, event Event, result *Result) {
	if action.Set != "" {
		value := e.interpolate(action.Value, event)
		result.PropertiesSet[action.Set] = value
	}

	if action.CreateRelation != nil {
		targetID := e.interpolate(action.CreateRelation.To, event)
		if targetID != "" && event.Entity != nil {
			rel := entity.NewRelation(event.Entity.ID, action.CreateRelation.Relation, targetID)
			result.RelationsToCreate = append(result.RelationsToCreate, rel)
		}
	}

	if action.CreateEntity != nil {
		entityType := action.CreateEntity.Type
		if entityType == "" {
			result.Errors = append(result.Errors, "create_entity action requires 'type' field")
			return
		}

		// Interpolate template name (allows {{new.kind}} etc.)
		template := e.interpolate(action.CreateEntity.Template, event)

		// Validate template name using allowlist (alphanumeric, hyphen, underscore only).
		if !isValidTemplateName(template) {
			result.Errors = append(result.Errors,
				"create_entity template name contains invalid characters (only alphanumeric, hyphen, underscore allowed)")
			return
		}

		props := make(map[string]interface{})
		for k, v := range action.CreateEntity.Properties {
			props[k] = e.interpolate(v, event)
		}

		// Default to skip if not specified.
		ifExists := action.CreateEntity.IfExists
		if ifExists == "" {
			ifExists = IfExistsSkip
		}

		result.EntitiesToCreate = append(result.EntitiesToCreate, EntityToCreate{
			Type:                entityType,
			Template:            template,
			Properties:          props,
			RelationFromTrigger: action.CreateEntity.Relation,
			IfExists:            ifExists,
		})
	}

	if action.Lua != "" {
		// Interpolate only safe values ({{today}}, {{user.name}}, etc.)
		// Entity properties are accessed via Lua globals, NOT interpolated into code
		code := e.interpolateSafeOnly(action.Lua, event)
		result.LuaToExecute = append(result.LuaToExecute, LuaToExecute{
			Code: code,
		})
	}

	if action.LuaFile != "" {
		// Path validation is handled by the script package at execution time.
		// This keeps validation centralized and consistent across all script execution paths.
		result.LuaToExecute = append(result.LuaToExecute, LuaToExecute{
			FilePath: action.LuaFile,
		})
	}
}

// evaluateValidation checks a validation and adds warnings/errors to the result.
func (e *Engine) evaluateValidation(validation Validation, event Event, result *Result) {
	if event.Entity == nil {
		return
	}

	// Parse the check expression and evaluate against the entity
	f, err := filter.Parse(validation.Check)
	if err != nil {
		result.Warnings = append(result.Warnings, "Invalid validation check: "+validation.Check)
		return
	}

	// Use simple value matching (works without full metamodel context)
	val := event.Entity.Properties[f.Property]
	if !matchSimple(val, f) {
		msg := e.interpolate(validation.Message, event)
		if validation.GetSeverity() == "error" {
			result.Errors = append(result.Errors, msg)
		} else {
			result.Warnings = append(result.Warnings, msg)
		}
	}
}

// matchSimple does simple value matching without metamodel context.
// Handles the most common automation validation cases.
func matchSimple(val interface{}, f *filter.Filter) bool {
	// Handle nil/missing/empty values
	if val == nil || val == "" {
		// Only match if explicitly comparing to empty with = operator
		if f.Operator == filter.OpEqual && f.Value == "" {
			return true
		}
		// For "!=" with empty value, missing/nil means "is empty", so it should NOT match "is not empty"
		return false
	}

	// Use the filter package's MatchValue for the actual comparison
	return filter.MatchValue(val, f)
}

// interpolate replaces template variables in a string.
func (e *Engine) interpolate(template string, event Event) string {
	return Interpolate(template, e.vars, event.Entity, event.OldEntity)
}

// interpolateSafeOnly replaces only safe template variables (not entity properties).
// Used for Lua code where entity properties should be accessed via globals.
func (e *Engine) interpolateSafeOnly(template string, _ Event) string {
	return InterpolateSafeOnly(template, e.vars)
}

// isValidTemplateName validates that a template name contains only safe identifier characters.
// Uses an allowlist approach: only alphanumeric, hyphen, and underscore are allowed.
// Empty template names are valid (means use default template).
func isValidTemplateName(name string) bool {
	if name == "" {
		return true
	}
	// Allowlist approach: only allow identifier-like characters.
	// This is safer than blocklisting dangerous patterns (path separators, .., null bytes, etc.)
	for _, ch := range name {
		isLower := ch >= 'a' && ch <= 'z'
		isUpper := ch >= 'A' && ch <= 'Z'
		isDigit := ch >= '0' && ch <= '9'
		isAllowed := isLower || isUpper || isDigit || ch == '-' || ch == '_'
		if !isAllowed {
			return false
		}
	}
	return true
}
