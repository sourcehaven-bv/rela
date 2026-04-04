// Package validation provides custom validation rule checking for entities.
// It encapsulates the logic for evaluating metamodel validation rules against
// entity sets, supporting scoped validation for view-based analysis.
package validation

import (
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
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
	meta        *metamodel.Metamodel
	ws          lua.WorkspaceInterface // Optional: for Lua validation rules
	projectRoot string                 // Optional: for loading lua_file scripts
	luaExec     *luaExecutor           // Lazy-initialized Lua executor
}

// Option configures a validation Service.
type Option func(*Service)

// WithWorkspace sets the workspace for Lua validation rules.
// This enables Lua scripts to access entities and relations via rela.get_entity(), etc.
// The workspace is wrapped in a read-only layer to prevent mutations.
func WithWorkspace(ws lua.WorkspaceInterface) Option {
	return func(s *Service) {
		s.ws = ws
	}
}

// WithProjectRoot sets the project root for loading lua_file scripts.
// Scripts are loaded from the scripts/ directory within the project root.
func WithProjectRoot(root string) Option {
	return func(s *Service) {
		s.projectRoot = root
	}
}

// New creates a validation service for the given metamodel.
// Use options to enable Lua validation support:
//
//	svc := validation.New(meta, validation.WithWorkspace(ws), validation.WithProjectRoot(root))
func New(meta *metamodel.Metamodel, opts ...Option) *Service {
	s := &Service{meta: meta}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Rules returns the validation rules from the metamodel.
func (s *Service) Rules() []metamodel.ValidationRule {
	return s.meta.Validations
}

// Check runs all validation rules against the given entities.
// If scope is non-nil, only entities in scope are checked.
func (s *Service) Check(entities []*model.Entity, scope map[string]bool) []Violation {
	return s.CheckRules(entities, scope, nil)
}

// CheckRules runs validation rules against the given entities.
// If ruleNames is nil, all rules are run. Otherwise, only rules in the set are run.
// If scope is non-nil, only entities in scope are checked.
func (s *Service) CheckRules(entities []*model.Entity, scope, ruleNames map[string]bool) []Violation {
	var violations []Violation

	for _, rule := range s.meta.Validations {
		// Skip rules not in the filter set (if filter is specified)
		if ruleNames != nil && !ruleNames[rule.Name] {
			continue
		}

		ruleViolations := s.checkRule(rule, entities, scope)
		severity := rule.GetSeverity()
		for _, entity := range ruleViolations {
			violations = append(violations, Violation{
				RuleName:    rule.Name,
				Description: rule.Description,
				Severity:    severity,
				EntityID:    entity.ID,
				EntityTitle: entity.Title(),
			})
		}
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

// checkRule checks a single rule against entities.
func (s *Service) checkRule(
	rule metamodel.ValidationRule,
	entities []*model.Entity,
	scope map[string]bool,
) []*model.Entity {
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
	var violations []*model.Entity
	for _, entity := range candidates {
		if s.entityViolatesRule(entity, rule, whenFilters, thenFilters) {
			violations = append(violations, entity)
		}
	}
	return violations
}

// filterCandidates filters entities by scope and entity type.
func (s *Service) filterCandidates(
	entities []*model.Entity,
	scope map[string]bool,
	entityType string,
) []*model.Entity {
	var candidates []*model.Entity
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

// entityViolatesRule checks if an entity violates the given rule.
func (s *Service) entityViolatesRule(
	entity *model.Entity,
	rule metamodel.ValidationRule,
	whenFilters, thenFilters []*filter.Filter,
) bool {
	entityDef, ok := s.meta.GetEntityDef(entity.Type)
	if !ok {
		return false
	}

	// Check 'when' conditions - if they don't match, rule doesn't apply
	if len(whenFilters) > 0 {
		matches, err := filter.MatchAll(entity, whenFilters, entityDef, s.meta)
		if err != nil || !matches {
			return false
		}
	}

	// Check 'then' conditions - if they don't satisfy, it's a violation
	if len(thenFilters) > 0 {
		satisfies, err := filter.MatchAll(entity, thenFilters, entityDef, s.meta)
		if err != nil || !satisfies {
			return true
		}
	}

	// Check Lua validation rules
	if (rule.Lua != "" || rule.LuaFile != "") && !s.luaValidates(entity, rule) {
		return true
	}

	// Check content rules
	if rule.Content != nil && !markdown.CheckContentRule(entity, rule.Content) {
		return true
	}

	return false
}

// luaValidates runs Lua validation and returns true if the entity passes.
// Returns true (no violation) if:
//   - No workspace configured (Lua rules are skipped)
//   - Lua validation passes
//
// Returns false (violation) if Lua returns false/nil.
func (s *Service) luaValidates(entity *model.Entity, rule metamodel.ValidationRule) bool {
	// Skip Lua validation if no workspace configured
	if s.ws == nil {
		return true
	}

	// Lazy-initialize the Lua executor
	if s.luaExec == nil {
		s.luaExec = newLuaExecutor(s.ws, s.meta, s.projectRoot)
	}

	return s.luaExec.validate(entity, rule)
}
