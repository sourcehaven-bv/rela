package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ValidateConfig re-exports the validation function from dataentryconfig.
var ValidateConfig = dataentryconfig.ValidateConfig

// getValidEnumValues wraps the exported function from dataentryconfig
// for use within this package (e.g. handlers_kanban.go).
func getValidEnumValues(propDef metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	return dataentryconfig.GetValidEnumValues(propDef, meta)
}
