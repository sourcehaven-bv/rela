// Package validator provides a Validator service that runs metamodel
// validation rules over a store.
//
// Following the same pattern as tracer and search: validation is
// a separate query service that reads from a store.EntityReader. Smart
// backends (e.g. Postgres with constraints) could implement Validator
// natively. The generic GenericValidator iterates the store and runs each
// rule via a metamodel.Metamodel + validation.Service.
package validator

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/validation"
)

// Violation represents a custom validation rule violation.
type Violation struct {
	RuleName    string
	Description string
	Severity    string
	EntityID    string
	EntityType  string
	EntityTitle string
}

// RuleResult is the full per-rule outcome surfaced to consumers that
// want to render Lua-script failures (rule did not run) distinctly
// from violations (rule ran and found a problem with an entity).
type RuleResult struct {
	// Violations is the list of entity IDs that violated the rule.
	Violations []string
	// ScriptErrors describe Lua failures (compile, runtime, timeout,
	// contract). Each carries enough context (path, line, source
	// slice) to render an actionable message.
	ScriptErrors []*lua.ScriptError
	// LoadErrors describe lua_file: rules whose script could not be
	// opened.
	LoadErrors []LoadError
}

// LoadError records a `lua_file:` rule whose script could not be
// loaded. Mirrors validation.LoadError without exposing the
// validation package directly.
type LoadError struct {
	RuleName string
	Message  string
}

// Validator runs custom metamodel validation rules over a store.
type Validator interface {
	// CheckRule returns IDs of entities that violate the given rule.
	// Lua-script failures (rule did not run) are dropped; consumers
	// that need to render those should use CheckRuleFull instead.
	CheckRule(ctx context.Context, rule metamodel.ValidationRule) ([]string, error)

	// CheckRuleFull returns the full per-rule result including
	// ScriptErrors and LoadErrors so the caller can distinguish
	// "rule ran, here are the violations" from "rule did not run."
	CheckRuleFull(ctx context.Context, rule metamodel.ValidationRule) (RuleResult, error)

	// CheckAll runs all rules from the metamodel and returns all violations.
	CheckAll(ctx context.Context) ([]Violation, error)
}

// GenericValidator implements Validator by reading from a store.
type GenericValidator struct {
	r    store.EntityReader
	meta *metamodel.Metamodel
	svc  *validation.Service
}

var _ Validator = (*GenericValidator)(nil)

// New creates a Validator backed by an EntityReader and a metamodel.
// deps provides read-only Lua access for validation rules that use Lua scripts.
// deps.ProjectRoot is used to resolve lua_file paths from validations/.
func New(r store.EntityReader, meta *metamodel.Metamodel, deps lua.ReadDeps) *GenericValidator {
	return &GenericValidator{
		r:    r,
		meta: meta,
		svc:  validation.New(meta, deps),
	}
}

// CheckRule returns IDs of entities that violate the given rule.
// Lua-script failures are dropped; use CheckRuleFull to see them.
func (v *GenericValidator) CheckRule(ctx context.Context, rule metamodel.ValidationRule) ([]string, error) {
	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		return nil, err
	}
	return full.Violations, nil
}

// CheckRuleFull runs a single rule and returns the full result —
// violation entity IDs, ScriptErrors, and LoadErrors — so callers
// that render in a UI can distinguish "rule did not run" from "rule
// ran and flagged an entity."
func (v *GenericValidator) CheckRuleFull(
	ctx context.Context,
	rule metamodel.ValidationRule,
) (RuleResult, error) {
	candidates, err := v.loadCandidates(ctx, rule.EntityType)
	if err != nil {
		return RuleResult{}, err
	}

	result := v.svc.CheckRule(ctx, rule, candidates, nil)
	out := RuleResult{
		Violations:   make([]string, 0, len(result.Violations)),
		ScriptErrors: result.ScriptErrors,
	}
	for _, vi := range result.Violations {
		out.Violations = append(out.Violations, vi.EntityID)
	}
	for _, le := range result.LoadErrors {
		out.LoadErrors = append(out.LoadErrors, LoadError{
			RuleName: le.RuleName,
			Message:  le.Message,
		})
	}
	return out, nil
}

// CheckAll runs all rules from the metamodel and returns all violations.
func (v *GenericValidator) CheckAll(ctx context.Context) ([]Violation, error) {
	candidates, err := v.loadCandidates(ctx, "")
	if err != nil {
		return nil, err
	}

	raw := v.svc.Check(ctx, candidates, nil)
	out := make([]Violation, 0, len(raw.Violations))
	for _, r := range raw.Violations {
		out = append(out, Violation{
			RuleName:    r.RuleName,
			Description: r.Description,
			Severity:    r.Severity,
			EntityID:    r.EntityID,
			EntityTitle: r.EntityTitle,
		})
	}
	return out, nil
}

// loadCandidates loads entities of the given type from the store.
func (v *GenericValidator) loadCandidates(ctx context.Context, entityType string) ([]*entity.Entity, error) {
	q := store.EntityQuery{}
	if entityType != "" {
		q.Type = entityType
	}

	out := make([]*entity.Entity, 0)
	for e, err := range v.r.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
