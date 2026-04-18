package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
)

func entityRecord(e *entity.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt}
}
