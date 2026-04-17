package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func entityRecord(e *model.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.ModTime}
}

func toFilterSortSpecs(specs []model.SortSpec) []filter.SortSpec {
	out := make([]filter.SortSpec, len(specs))
	for i, s := range specs {
		out[i] = filter.SortSpec{Property: s.Property, Direction: s.Direction}
	}
	return out
}
