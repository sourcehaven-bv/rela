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

// Validator runs custom metamodel validation rules over a store.
type Validator interface {
	// CheckRule returns IDs of entities that violate the given rule.
	CheckRule(ctx context.Context, rule metamodel.ValidationRule) ([]string, error)

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
func (v *GenericValidator) CheckRule(ctx context.Context, rule metamodel.ValidationRule) ([]string, error) {
	candidates, err := v.loadCandidates(ctx, rule.EntityType)
	if err != nil {
		return nil, err
	}

	violations := v.svc.CheckRule(rule, candidates, nil)
	ids := make([]string, 0, len(violations))
	for _, vi := range violations {
		ids = append(ids, vi.EntityID)
	}
	return ids, nil
}

// CheckAll runs all rules from the metamodel and returns all violations.
func (v *GenericValidator) CheckAll(ctx context.Context) ([]Violation, error) {
	candidates, err := v.loadCandidates(ctx, "")
	if err != nil {
		return nil, err
	}

	raw := v.svc.Check(candidates, nil)
	out := make([]Violation, 0, len(raw))
	for _, r := range raw {
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
