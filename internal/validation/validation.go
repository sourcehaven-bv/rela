// Package validation provides custom validation rule checking for entities.
// It encapsulates the logic for evaluating metamodel validation rules against
// entity sets, supporting scoped validation for view-based analysis.
package validation

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Violation represents a custom validation rule violation.
type Violation struct {
	RuleName    string
	Description string
	Severity    string // "error" or "warning"
	EntityID    string
	EntityTitle string
}

// Result aggregates the outputs of a validation pass: rule findings,
// Lua failures, and load-time failures. The three slices are
// semantically distinct:
//
//   - Violations: rule ran successfully and found a problem with an
//     entity.
//   - ScriptErrors: a Lua rule failed to run (compile, runtime,
//     timeout, or contract violation like wrong return type) — the
//     rule did NOT report on entities.
//   - LoadErrors: a `lua_file:` rule's script could not be opened
//     (config-level failure, no Lua VM ever ran).
//
// Splitting these prevents the CLI from conflating "entity violated
// rule" (a finding) with "rule did not run" (an environment problem).
type Result struct {
	Violations   []Violation
	ScriptErrors []*lua.ScriptError
	LoadErrors   []LoadError
}

// LoadError records a `lua_file:` rule whose script could not be
// loaded. Message is already sanitized by loadLuaScript (no system
// paths leaked).
type LoadError struct {
	RuleName string
	Message  string
}

// HasErrors reports whether any rule failed to execute (Lua failure
// or script-load failure). Unrelated to whether Violations were
// found — a clean run with violations still has HasErrors() == false.
func (r Result) HasErrors() bool {
	return len(r.ScriptErrors) > 0 || len(r.LoadErrors) > 0
}

// Service validates entities against custom metamodel rules.
type Service struct {
	deps  lua.ReadDeps
	cache *lua.Cache
}

// New creates a validation service for the given metamodel.
// deps provides read-only lua access for rules that use Lua scripts.
// The ProjectRoot field of deps is used to resolve lua_file paths from
// validations/. The supplied metamodel is authoritative — it overwrites
// deps.Meta so the Go-side rule evaluator and the Lua-side filter helpers
// cannot disagree about which schema is in effect.
func New(meta *metamodel.Metamodel, deps lua.ReadDeps) *Service {
	deps.Meta = meta
	return &Service{deps: deps}
}

// WithCache wires a shared Lua cache into validation runs so
// rela.cache.* inside validation scripts is functional and namespaced
// per-rule. Zero-or-nil cache leaves validation runtimes un-cached.
func (s *Service) WithCache(c *lua.Cache) *Service {
	s.cache = c
	return s
}

// Rules returns the validation rules from the metamodel.
func (s *Service) Rules() []metamodel.ValidationRule {
	return s.deps.Meta.Validations
}

// Check runs all validation rules against the given entities.
// If scope is non-nil, only entities in scope are checked.
//
// ctx is propagated into Lua execution: canceling it interrupts an
// in-flight rule. Pass cmd.Context() (cobra) or r.Context() (HTTP) for
// real cancellation; tests use context.Background.
func (s *Service) Check(ctx context.Context, entities []*entity.Entity, scope map[string]bool) Result {
	return s.CheckRules(ctx, entities, scope, nil)
}

// CheckRules runs validation rules against the given entities.
// If ruleNames is nil, all rules are run. Otherwise, only rules in the set are run.
// If scope is non-nil, only entities in scope are checked.
func (s *Service) CheckRules(
	ctx context.Context,
	entities []*entity.Entity,
	scope, ruleNames map[string]bool,
) Result {
	var result Result

	for _, rule := range s.deps.Meta.Validations {
		// Skip rules not in the filter set (if filter is specified)
		if ruleNames != nil && !ruleNames[rule.Name] {
			continue
		}

		ruleResult := s.CheckRule(ctx, rule, entities, scope)
		result.Violations = append(result.Violations, ruleResult.Violations...)
		result.ScriptErrors = append(result.ScriptErrors, ruleResult.ScriptErrors...)
		result.LoadErrors = append(result.LoadErrors, ruleResult.LoadErrors...)
	}

	return result
}

// CountBySeverity returns counts of errors and warnings from violations.
func CountBySeverity(violations []Violation) (errors, warnings int) {
	for _, v := range violations {
		if v.Severity == "error" {
			errors++
		} else {
			warnings++
		}
	}
	return
}

// CheckRule checks a single rule against entities.
func (s *Service) CheckRule(
	ctx context.Context,
	rule metamodel.ValidationRule,
	entities []*entity.Entity,
	scope map[string]bool,
) Result {
	// Parse filters
	whenFilters, err := filter.ParseAll(rule.When)
	if err != nil {
		return Result{}
	}
	thenFilters, err := filter.ParseAll(rule.Then)
	if err != nil {
		return Result{}
	}

	// Filter candidates
	candidates := s.filterCandidates(entities, scope, rule.EntityType)

	// Check each candidate
	var result Result
	for _, e := range candidates {
		entityViolations := s.checkEntityAgainstRule(ctx, e, rule, whenFilters, thenFilters)
		result.Violations = append(result.Violations, entityViolations...)
	}
	return result
}

// filterCandidates filters entities by scope and entity type.
func (s *Service) filterCandidates(
	entities []*entity.Entity,
	scope map[string]bool,
	entityType string,
) []*entity.Entity {
	var candidates []*entity.Entity
	for _, e := range entities {
		if scope != nil {
			if _, ok := scope[e.ID]; !ok {
				continue
			}
		}
		if entityType == "" || e.Type == entityType {
			candidates = append(candidates, e)
		}
	}
	return candidates
}

// checkEntityAgainstRule checks if an entity violates the given rule.
// Returns violations found, or empty slice if entity passes.
func (s *Service) checkEntityAgainstRule(
	ctx context.Context,
	e *entity.Entity,
	rule metamodel.ValidationRule,
	whenFilters, thenFilters []*filter.Filter,
) []Violation {
	entityDef, ok := s.deps.Meta.GetEntityDef(e.Type)
	if !ok {
		return nil
	}

	rec := filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties}

	// Check 'when' conditions - if they don't match, rule doesn't apply
	if len(whenFilters) > 0 {
		matches, err := filter.MatchAll(rec, whenFilters, entityDef, s.deps.Meta)
		if err != nil || !matches {
			return nil
		}
	}

	// Check 'then' conditions - if they don't satisfy, it's a violation
	if len(thenFilters) > 0 {
		satisfies, err := filter.MatchAll(rec, thenFilters, entityDef, s.deps.Meta)
		if err != nil || !satisfies {
			return []Violation{{
				RuleName:    rule.Name,
				Description: rule.Description,
				Severity:    rule.GetSeverity(),
				EntityID:    e.ID,
				EntityTitle: e.Title(),
			}}
		}
	}

	// Check Lua validation rules
	if rule.Lua != "" || rule.LuaFile != "" {
		luaViolations := s.runLuaValidation(ctx, e, rule)
		if len(luaViolations) > 0 {
			return luaViolations
		}
	}

	// Check content rules
	if rule.Content != nil && !CheckContentRule(e.Content, rule.Content) {
		return []Violation{{
			RuleName:    rule.Name,
			Description: rule.Description,
			Severity:    rule.GetSeverity(),
			EntityID:    e.ID,
			EntityTitle: e.Title(),
		}}
	}

	return nil
}

// runLuaValidation runs Lua validation and returns violations.
// Returns empty slice if validation passes or Lua is not configured.
func (s *Service) runLuaValidation(
	ctx context.Context, e *entity.Entity, rule metamodel.ValidationRule,
) []Violation {
	luaViolations := s.validateLua(ctx, e, rule)
	if len(luaViolations) == 0 {
		return nil
	}

	// Convert LuaViolations to Violations
	violations := make([]Violation, len(luaViolations))
	for i, lv := range luaViolations {
		violations[i] = Violation{
			RuleName:    rule.Name,
			Description: lv.Message, // Use Lua's custom message
			Severity:    lv.Severity,
			EntityID:    e.ID,
			EntityTitle: e.Title(),
		}
	}
	return violations
}
