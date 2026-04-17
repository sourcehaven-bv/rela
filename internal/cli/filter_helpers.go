package cli

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// storeEntityRecord converts an entity.Entity to a filter.Record.
func storeEntityRecord(e *entity.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt}
}

// modelEntityRecord converts a model.Entity to a filter.Record.
// Used by commands that still receive model.Entity from workspace methods.
func modelEntityRecord(e *model.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.ModTime}
}

// modelToEntitySlice converts []*model.Entity to []*entity.Entity for output.
func modelToEntitySlice(models []*model.Entity) []*entity.Entity {
	out := make([]*entity.Entity, len(models))
	for i, m := range models {
		out[i] = model.EntityToDomain(m)
	}
	return out
}

// modelToRelationSlice converts []*model.Relation to []*entity.Relation.
func modelToRelationSlice(models []*model.Relation) []*entity.Relation {
	out := make([]*entity.Relation, len(models))
	for i, m := range models {
		out[i] = model.RelationToDomain(m)
	}
	return out
}
