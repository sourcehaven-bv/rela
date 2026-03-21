// Package validation provides custom validation rule checking for entities.
// It encapsulates the logic for evaluating metamodel validation rules against
// entity sets, supporting scoped validation for view-based analysis.
package validation

import (
	"github.com/Sourcehaven-BV/rela/internal/filter"
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
	meta *metamodel.Metamodel
}

// New creates a validation service for the given metamodel.
func New(meta *metamodel.Metamodel) *Service {
	return &Service{meta: meta}
}

// Rules returns the validation rules from the metamodel.
func (s *Service) Rules() []metamodel.ValidationRule {
	return s.meta.Validations
}

// Check runs all validation rules against the given entities.
// If scope is non-nil, only entities in scope are checked.
func (s *Service) Check(entities []*model.Entity, scope map[string]bool) []Violation {
	var violations []Violation

	for _, rule := range s.meta.Validations {
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

	// Check content rules
	if rule.Content != nil && !markdown.CheckContentRule(entity, rule.Content) {
		return true
	}

	return false
}
