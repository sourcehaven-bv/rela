package automation

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// Engine evaluates automations against entity events.
type Engine struct {
	automations []Automation
	vars        TemplateVars
	// meta is the active metamodel, used to evaluate `when:` and
	// `validate:` comparisons against the property's declared type
	// (so `count>9` compares numerically, not lexicographically).
	// Optional: nil falls back to string-only matching, preserving the
	// pre-metamodel behavior for engines constructed via [NewEngine].
	meta *metamodel.Metamodel

	// ev is the predicate evaluator for typed `when:`/`validate:`
	// comparisons — the condition engine (TKT-J4IR1G). Built lazily on
	// first typed match against the current meta; rebuilt if meta changes.
	// Guarded by evMu.
	evMu sync.Mutex
	ev   *predicatefns.Evaluator
}

// evaluator returns the predicate Evaluator bound to the current
// metamodel, building it once. Returns nil when there is no metamodel
// (the string-only fallback path handles that).
func (e *Engine) evaluator() *predicatefns.Evaluator {
	if e.meta == nil {
		return nil
	}
	e.evMu.Lock()
	defer e.evMu.Unlock()
	if e.ev == nil {
		e.ev = predicatefns.NewEvaluator(e.meta)
	}
	return e.ev
}

// NewEngine creates an automation engine with the given automations.
// The engine has no metamodel, so `when:`/`validate:` comparisons fall
// back to string matching; use [Engine.SetMetamodel] (or
// [NewEngineFromMetamodel]) to get type-aware comparison.
func NewEngine(automations []Automation) *Engine {
	return &Engine{
		automations: automations,
		vars:        DefaultTemplateVars(),
	}
}

// NewEngineFromMetamodel creates an automation engine from metamodel
// definitions, wiring the metamodel itself so `when:`/`validate:`
// comparisons evaluate against each property's declared type.
//
// It returns an error when a definition cannot be honored as written —
// an unparseable `when:` clause, or a `condition:` that does not
// compile. Both used to be swallowed, which silently WIDENED the
// automation: a dropped constraint fires on more entities than the
// operator asked for. Failing the load is the safe direction, and the
// operator sees the mistake at startup rather than in a diff a week
// later.
func NewEngineFromMetamodel(
	meta *metamodel.Metamodel, defs []metamodel.AutomationDef,
) (*Engine, error) {
	automations := make([]Automation, len(defs))
	for i, def := range defs {
		auto, err := convertFromMetamodel(def)
		if err != nil {
			return nil, err
		}
		automations[i] = auto
	}
	e := NewEngine(automations)
	e.meta = meta
	if err := e.compileConditions(); err != nil {
		return nil, err
	}
	return e, nil
}

// compileConditions compiles every `condition:` expression up front, so a
// syntax error or an unknown property surfaces at load rather than on the
// first matching write. Compiled programs are cached inside the
// Evaluator, keyed by (entityType, source), so this also warms it.
//
// A condition needs a metamodel to build its typed env. An engine
// constructed without one (NewEngine, used by tests and the memory
// backend) therefore cannot honor a condition at all — that is an
// error, not a silent no-op, for the same widening reason.
func (e *Engine) compileConditions() error {
	for _, auto := range e.automations {
		src := auto.On.Condition
		if src == "" {
			continue
		}
		if e.meta == nil {
			return fmt.Errorf(
				"automation %q: condition %q requires a metamodel", auto.Name, src)
		}
		types := auto.On.Entity
		if len(types) == 0 {
			// A condition references entity properties, so it must be
			// compiled against a concrete type's env. Without `entity:`
			// there is nothing to compile against.
			return fmt.Errorf(
				"automation %q: condition requires `entity:` naming the type(s) it applies to",
				auto.Name)
		}
		ev := e.evaluator()
		for _, t := range types {
			if _, err := ev.Compile(t, src); err != nil {
				return fmt.Errorf("automation %q: condition %q on %q: %w",
					auto.Name, src, t, err)
			}
		}
	}
	return nil
}

// SetMetamodel wires the metamodel used for type-aware `when:`/
// `validate:` comparison. Optional; without it the engine compares as
// strings.
func (e *Engine) SetMetamodel(meta *metamodel.Metamodel) {
	e.meta = meta
	e.evMu.Lock()
	e.ev = nil // rebuild against the new meta on next use
	e.evMu.Unlock()
}

// convertFromMetamodel converts a metamodel AutomationDef to the internal
// Automation type. An unparseable `when:` clause is an ERROR, not a skip:
// dropping a constraint makes the automation fire on MORE entities than the
// operator wrote, so failing the load is the safe direction.
//
// Upgrade impact is narrow by construction. filter.Parse rejects exactly
// three things — an empty string, a clause with no operator, and one with
// an empty property name. Everything else parses, including odd-looking
// input like "a=b=c" or "spaces in prop=x". So the only projects this can
// newly fail are ones whose clause was ALREADY broken and silently
// matching nothing; a clause that did real work keeps working. Verified
// against all 70 distinct when:/then: clauses in this repo's own
// schema.yaml, every one of which still parses.
func convertFromMetamodel(def metamodel.AutomationDef) (Automation, error) {
	whenFilters := make([]*filter.Filter, 0, len(def.On.When))
	for _, w := range def.On.When {
		f, err := filter.Parse(w)
		if err != nil {
			return Automation{}, fmt.Errorf(
				"automation %q: when clause %q: %w", def.Name, w, err)
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
			Faces:           []string(def.On.Faces),
			When:            whenFilters,
			Condition:       def.On.Condition,
		},
		Do:       make([]Action, len(def.Do)),
		Validate: make([]Validation, len(def.Validate)),
	}

	for i, a := range def.Do {
		action := Action{
			Set:            a.Set,
			Value:          a.Value,
			Lua:            a.Lua,
			LuaFile:        a.LuaFile,
			AllowACLBypass: a.AllowACLBypass,
			// Carried like AllowACLBypass: omitting it here silently drops every
			// automation's `capabilities:` block (TKT-YH52OM), which is the
			// hand-copy failure mode this hop is most prone to.
			Capabilities: a.Capabilities,
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

	return auto, nil
}

// SetTemplateVars sets the template variables for interpolation.
func (e *Engine) SetTemplateVars(vars TemplateVars) {
	e.vars = vars
}

// Process evaluates all automations against an event and returns the result.
func (e *Engine) Process(ctx context.Context, event Event) *Result {
	result := &Result{
		PropertiesSet:     make(map[string]string),
		RelationsToCreate: make([]RelationToCreate, 0),
		EntitiesToCreate:  make([]EntityToCreate, 0),
		LuaToExecute:      make([]LuaToExecute, 0),
		Warnings:          make([]string, 0),
		Errors:            make([]string, 0),
	}

	for _, auto := range e.automations {
		if !e.matches(ctx, auto.On, event, result) {
			continue
		}

		// Execute actions
		for _, action := range auto.Do {
			e.executeAction(action, event, result, auto.Name)
		}

		// Evaluate validations
		for _, validation := range auto.Validate {
			e.evaluateValidation(ctx, validation, event, result)
		}
	}

	return result
}

// matches checks if a trigger matches an event.
func (e *Engine) matches(ctx context.Context, trigger Trigger, event Event, res *Result) bool {
	// Check entity type constraint
	if len(trigger.Entity) > 0 && event.Entity != nil {
		matched := slices.Contains(trigger.Entity, event.Entity.Type)
		if !matched {
			return false
		}
	}

	// Check the content-state constraint before any condition runs: a trigger
	// scoped away from this row does not apply, which is a different thing from
	// a condition that failed to match, and evaluating conditions for it would
	// be wasted work.
	if !e.matchesFace(trigger, event.Entity) {
		return false
	}

	// Check when conditions (property filters on the entity)
	if !e.matchesWhenConditions(ctx, trigger, event.Entity, res) {
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
func (e *Engine) matchesWhenConditions(
	ctx context.Context, trigger Trigger, entity *entity.Entity, res *Result,
) bool {
	if len(trigger.When) == 0 && trigger.Condition == "" {
		return true
	}
	if entity == nil {
		return false
	}

	for _, f := range trigger.When {
		if !e.matchProperty(ctx, entity, f) {
			return false
		}
	}
	matched, err := e.matchesCondition(ctx, trigger, entity)
	if err != nil && res != nil {
		res.Warnings = append(res.Warnings,
			"automation condition could not be evaluated: "+err.Error())
	}
	return matched
}

// matchesCondition evaluates the trigger's `condition:` expression, ANDed
// with the When clauses the caller has already checked.
//
// The program was compiled at load, so a failure here is an EVAL error (a
// missing property binding, the step budget) rather than a syntax
// mistake. The automation does NOT fire — it must not act on a condition
// that could not be shown to hold — but the error is returned rather than
// swallowed, so [Engine.Process] can surface it as a warning. A condition
// that silently never matches is the exact failure this whole key exists
// to remove; reintroducing it here would defeat the point.
func (e *Engine) matchesCondition(
	ctx context.Context, trigger Trigger, ent *entity.Entity,
) (bool, error) {
	if trigger.Condition == "" {
		return true, nil
	}
	ev := e.evaluator()
	if ev == nil {
		// compileConditions rejects this at load; reaching it means an
		// engine was assembled by another path. Fail closed, and say so.
		return false, fmt.Errorf("condition %q: no metamodel", trigger.Condition)
	}
	prog, err := ev.Compile(ent.Type, trigger.Condition)
	if err != nil {
		return false, fmt.Errorf("condition %q: %w", trigger.Condition, err)
	}
	matched, err := ev.Matches(ctx, prog, ent.Type, ent.ID, ent.Properties)
	if err != nil {
		return false, fmt.Errorf("condition %q: %w", trigger.Condition, err)
	}
	return matched, nil
}

// executeAction performs an action and updates the result.
func (e *Engine) executeAction(action Action, event Event, result *Result, automationName string) {
	if action.Set != "" {
		value := e.interpolate(action.Value, event)
		result.PropertiesSet[action.Set] = value
	}

	if action.CreateRelation != nil {
		targetID := e.interpolate(action.CreateRelation.To, event)
		if targetID != "" && event.Entity != nil {
			rel := entity.NewRelation(event.Entity.ID, action.CreateRelation.Relation, targetID)
			result.RelationsToCreate = append(result.RelationsToCreate,
				RelationToCreate{Relation: rel, AutomationName: automationName})
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

		props := make(map[string]any)
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
			AutomationName:      automationName,
		})
	}

	if action.Lua != "" {
		// Interpolate only safe values ({{today}}, {{user.name}}, etc.)
		// Entity properties are accessed via Lua globals, NOT interpolated into code
		code := e.interpolateSafeOnly(action.Lua, event)
		result.LuaToExecute = append(result.LuaToExecute, LuaToExecute{
			Code:           code,
			AutomationName: automationName,
			AllowACLBypass: action.AllowACLBypass,
			Capabilities:   action.Capabilities,
		})
	}

	if action.LuaFile != "" {
		// Path validation is handled by the script package at execution time.
		// This keeps validation centralized and consistent across all script execution paths.
		result.LuaToExecute = append(result.LuaToExecute, LuaToExecute{
			FilePath:       action.LuaFile,
			AutomationName: automationName,
			AllowACLBypass: action.AllowACLBypass,
			Capabilities:   action.Capabilities,
		})
	}
}

// evaluateValidation checks a validation and adds warnings/errors to the result.
func (e *Engine) evaluateValidation(ctx context.Context, validation Validation, event Event, result *Result) {
	if event.Entity == nil {
		return
	}

	// Parse the check expression and evaluate against the entity
	f, err := filter.Parse(validation.Check)
	if err != nil {
		result.Warnings = append(result.Warnings, "Invalid validation check: "+validation.Check)
		return
	}

	if !e.matchProperty(ctx, event.Entity, f) {
		msg := e.interpolate(validation.Message, event)
		if validation.GetSeverity() == "error" {
			result.Errors = append(result.Errors, msg)
		} else {
			result.Warnings = append(result.Warnings, msg)
		}
	}
}

// matchProperty evaluates a filter against an entity property. When the
// engine has a metamodel and the property is declared, it uses the
// type-aware [filter.Match] so ordered comparisons respect the
// property's type (`count>9` is numeric, `due<2025-02-01` is a date).
// Without a metamodel, or for an undeclared property, it falls back to
// the string-only [matchSimple] — preserving the pre-metamodel
// behavior and tolerating ad-hoc/unknown properties rather than
// rejecting them.
func (e *Engine) matchProperty(ctx context.Context, ent *entity.Entity, f *filter.Filter) bool {
	if matched, handled := e.matchTyped(ctx, ent, f); handled {
		return matched
	}
	var val any
	if ent != nil {
		val = ent.Properties[f.Property]
	}
	return matchSimple(val, f)
}

// matchTyped attempts a type-aware comparison through the predicate
// condition engine (TKT-J4IR1G): the filter clause is transpiled to a
// predicate via predicatefns.FromFilter, compiled once (cached in the
// Evaluator), and evaluated against the entity. handled is false when
// there is no metamodel, no entity, or the property isn't declared — in
// which case the caller falls back to string matching, preserving the
// pre-metamodel behavior for ad-hoc properties.
//
// A transpile/compile error (e.g. an unsupported filter form) also
// returns handled=false so the string fallback still runs — this keeps
// the migration strictly no-worse than the prior filter.Match path.
func (e *Engine) matchTyped(ctx context.Context, ent *entity.Entity, f *filter.Filter) (matched, handled bool) {
	ev := e.evaluator()
	if ev == nil || ent == nil {
		return false, false
	}
	def, ok := e.meta.GetEntityDef(ent.Type)
	if !ok {
		return false, false
	}
	if _, ok := def.Properties[f.Property]; !ok {
		return false, false
	}
	propDef := def.Properties[f.Property]
	prog, err := ev.CompileFilter(ent.Type, []*filter.Filter{f})
	if err != nil {
		// The clause is untranspilable (e.g. fuzzy-with-wildcard, or an
		// operator/type combination FromFilter rejects). Reproduce the
		// EXACT legacy verdict via filter.Match rather than dropping to
		// the string matcher — the string path would accept ops
		// filter.Match rejected (e.g. regex on an int property), flipping
		// the verdict (RR-G9KT8H). handled=true so the caller does NOT
		// fall back to matchSimple.
		rec := filter.Record{ID: ent.ID, Type: ent.Type, Properties: ent.Properties}
		m, mErr := filter.Match(rec, f, &propDef, e.meta)
		if mErr != nil {
			return false, true
		}
		return m, true
	}
	m, err := ev.Matches(ctx, prog, ent.Type, ent.ID, ent.Properties)
	if err != nil {
		// A type/parse error means the comparison can't hold — no match,
		// same as the prior type-aware path.
		return false, true
	}
	return m, true
}

// matchSimple does simple value matching without metamodel context.
// Handles the most common automation validation cases.
func matchSimple(val any, f *filter.Filter) bool {
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

// matchesFace reports whether a trigger's `faces:` scope covers the row the
// event is about.
//
// An empty scope means every state — see Trigger.Faces for why that is the
// default. The comparison is against the DECLARED face name, because the bare
// face is stored as the empty coordinate and an operator writes `faces: [en]`,
// not `faces: [""]`.
//
// Nil meta: falls back to the stored coordinate. An engine built via NewEngine
// has no metamodel (the pre-metamodel construction path), and a scoped trigger
// there can only be compared literally. Unscoped triggers — every existing one
// — are unaffected either way.
func (e *Engine) matchesFace(trigger Trigger, ent *entity.Entity) bool {
	if len(trigger.Faces) == 0 {
		return true
	}
	if ent == nil {
		// A relation event with no entity in hand cannot be face-scoped; a
		// scoped trigger simply does not apply to it.
		return false
	}
	declared := ent.Face.String()
	if e.meta != nil {
		declared = metamodel.DeclaredFace(e.meta, ent.Type, ent.Face.String())
	}
	return slices.Contains(trigger.Faces, declared)
}
