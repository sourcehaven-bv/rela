package schema

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PropertyError aggregates metamodel validation errors for a single
// entity in the store.
type PropertyError struct {
	EntityID   string
	EntityType string
	Errors     []*metamodel.ValidationError
}

// ValidateEntityProperties iterates every entity in st and returns a
// PropertyError for each entity whose properties fail the metamodel's
// entity-property validation. Callers that need scope filtering should
// filter the returned slice themselves.
func ValidateEntityProperties(ctx context.Context, st store.Store, meta *metamodel.Metamodel) []PropertyError {
	if st == nil || meta == nil {
		return nil
	}
	var out []PropertyError
	// AllStates: each content state holds its own property values, so a
	// required property missing from one face is a real violation. The default
	// query loads only default-state rows and would report a clean run over
	// data it never looked at (TKT-4Y6CMV).
	for e, err := range st.ListEntities(ctx, store.EntityQuery{AllStates: true}) {
		if err != nil {
			continue
		}
		errs := meta.ValidateEntity(e.ID, e.Type, e.Properties)
		if len(errs) > 0 {
			out = append(out, PropertyError{
				EntityID:   e.ID,
				EntityType: e.Type,
				Errors:     errs,
			})
		}
	}
	return out
}

// RelationPropertyError aggregates metamodel validation errors for a
// single relation in the store.
type RelationPropertyError struct {
	RelationKey  string // "from--type--to"
	RelationType string
	Errors       []*metamodel.ValidationError
}

// RelationLister is the single capability [ValidateRelationProperties]
// needs. Declared at the call site rather than taking `store.Store`, so a
// caller may pass an ACL-scoped reader whose ListRelations yields only the
// principal's visible edges — the check then reports on that slice instead
// of the whole graph. `store.Store` satisfies it structurally, so existing
// callers are unaffected.
type RelationLister interface {
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// ValidateRelationProperties iterates every relation st yields and returns
// a RelationPropertyError for each relation whose properties fail the
// metamodel's relation-property validation.
func ValidateRelationProperties(
	ctx context.Context, st RelationLister, meta *metamodel.Metamodel,
) []RelationPropertyError {
	if st == nil || meta == nil {
		return nil
	}
	var out []RelationPropertyError
	for rel, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			continue
		}
		errs := meta.ValidateRelationProperties(rel.Type, rel.Properties)
		if len(errs) > 0 {
			out = append(out, RelationPropertyError{
				RelationKey:  rel.From + "--" + rel.Type + "--" + rel.To,
				RelationType: rel.Type,
				Errors:       errs,
			})
		}
	}
	return out
}
