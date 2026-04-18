package cli

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
)

// storeEntityRecord converts an entity.Entity to a filter.Record.
func storeEntityRecord(e *entity.Entity) filter.Record {
	return filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt}
}
