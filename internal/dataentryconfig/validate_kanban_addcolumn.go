package dataentryconfig

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateKanbanAddColumn checks that a board with `add_column: true` can
// add a column from the board (TKT-1X5G3BP).
//
// A column on a column_property board is a value of that enum, so adding
// one is a schema write. The SPA makes it through the Configure API, the
// only server-side config writer. It appends the value to the property's
// own `values:`, so the property must be an inline `type: enum`. A named
// custom type is refused: its values are shared by every property and board
// that uses the type, and the Configure space's choice-list editor is the
// place to change those. A columns_from board or a plain wall has no enum
// column; its add-group button already makes a new column.
func validateKanbanAddColumn(kanbanID string, kanban Kanban, entDef *metamodel.EntityDef) []string {
	if !kanban.AddColumn {
		return nil
	}
	prefix := fmt.Sprintf("kanban %q: add_column", kanbanID)
	if kanban.ColumnsFrom != nil || kanban.ColumnProperty == "" {
		return []string{prefix + " needs column_property; a columns_from board or a wall adds columns as groups"}
	}
	// The edit is made on schema.yaml's own tree, under the type's name in
	// the root file. A namespaced type is declared in another file.
	if ns := metamodel.NamespaceOf(kanban.EntityType); ns != "" {
		return []string{fmt.Sprintf(
			"%s needs a type declared in schema.yaml itself, but %q comes from namespace %q",
			prefix, kanban.EntityType, ns)}
	}
	prop, ok := entDef.Properties[kanban.ColumnProperty]
	if !ok {
		// validateKanbans already reports the unknown column property.
		return nil
	}
	if prop.Type != "enum" {
		return []string{fmt.Sprintf(
			"%s needs column property %q to be an inline enum (type: enum with values:), "+
				"but its type is %q; change a shared type's values in the Configure space instead",
			prefix, kanban.ColumnProperty, prop.Type)}
	}
	return nil
}
