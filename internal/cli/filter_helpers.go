package cli

import (
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// entityRecord converts a model.Entity to a filter.Record for matching.
func entityRecord(e *model.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.ModTime}
}

// entityAccess is a filter.Accessor for sorting slices of *model.Entity.
func entityAccess(e *model.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.ModTime}
}
