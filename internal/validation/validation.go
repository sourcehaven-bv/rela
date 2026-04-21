// Package validation provides custom validation rule checking for entities.
// It encapsulates the logic for evaluating metamodel validation rules against
// entity sets, supporting scoped validation for view-based analysis.
package validation

import (
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
func (s *Service) Check(entities []*entity.Entity, scope map[string]bool) []Violation {
	return s.CheckRules(entities, scope, nil)
}

// CheckRules runs validation rules against the given entities.
// If ruleNames is nil, all rules are run. Otherwise, only rules in the set are run.
// If scope is non-nil, only entities in scope are checked.
func (s *Service) CheckRules(entities []*entity.Entity, scope, ruleNames map[string]bool) []Violation {
	var violations []Violation

	for _, rule := range s.deps.Meta.Validations {
		// Skip rules not in the filter set (if filter is specified)
		if ruleNames != nil && !ruleNames[rule.Name] {
			continue
		}

		ruleViolations := s.CheckRule(rule, entities, scope)
		violations = append(violations, ruleViolations...)
	}

	return violations
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
	rule metamodel.ValidationRule,
	entities []*entity.Entity,
	scope map[string]bool,
) []Violation {
	// Parse filters
	whenFilters, err := filter.ParseAll(rule.When)
	if err != nil {
		return nil
	}
	thenFilters, err := filter.ParseAll(rule.Then)
	if err != nil {
		return nil
	}

	// Filter candidates
	candidates := s.filterCandidates(entities, scope, rule.EntityType)

	// Check each candidate
	var violations []Violation
	for _, e := range candidates {
		entityViolations := s.checkEntityAgainstRule(e, rule, whenFilters, thenFilters)
		violations = append(violations, entityViolations...)
	}
	return violations
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
		luaViolations := s.runLuaValidation(e, rule)
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
func (s *Service) runLuaValidation(e *entity.Entity, rule metamodel.ValidationRule) []Violation {
	luaViolations := s.validateLua(e, rule)
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
