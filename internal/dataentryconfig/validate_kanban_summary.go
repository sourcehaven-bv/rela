package dataentryconfig

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateKanbanColumnSummary checks a board's `column_summary:` list
// (TKT-SVLQQ6).
func validateKanbanColumnSummary(kanbanID string, kanban Kanban, entDef *metamodel.EntityDef) []string {
	if len(kanban.ColumnSummary) == 0 {
		return nil
	}
	at := fmt.Sprintf("kanban %q: column_summary", kanbanID)
	var errs []string
	if kanban.SwimlaneProperty != "" {
		// A swimlane board has a heading per column per lane, so there is no
		// one place to show a column's summary. Refuse rather than accept a
		// key that would render nothing.
		errs = append(errs, at+" is not supported on a board with swimlanes")
	}
	return append(errs, validateSummaries(at, kanban.ColumnSummary, kanban.EntityType, entDef)...)
}
