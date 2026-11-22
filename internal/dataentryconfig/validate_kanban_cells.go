package dataentryconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateKanbanCellColors checks a board's `cell_colors:` (TKT-83S9G0Z).
//
// A key must name a column or lane the board shows: a declared `columns:` or
// `swimlanes:` entry, or else a value of the property. A key for a value the
// board hides would paint nothing, which is dead config an operator cannot see.
func validateKanbanCellColors(
	kanbanID string, kanban Kanban, entDef *metamodel.EntityDef, meta *metamodel.Metamodel, cfg *Config,
) []string {
	if len(kanban.CellColors) == 0 {
		return nil
	}
	at := fmt.Sprintf("kanban %q: cell_colors", kanbanID)
	if kanban.SwimlaneProperty == "" {
		return []string{at + " needs swimlane_property"}
	}
	if kanban.ColumnsFrom != nil {
		return []string{at + " is not supported with columns_from"}
	}
	// Nil when the column or lane property is itself invalid, which is
	// reported already. The keys on that axis are then not checked, but every
	// cell's color still is, so a bad color does not wait for the next load.
	columns := shownKanbanValues(entDef, meta, kanban.ColumnProperty, kanbanColumnValues(kanban.Columns))
	lanes := shownKanbanValues(entDef, meta, kanban.SwimlaneProperty, kanbanLaneValues(kanban.Swimlanes))
	var errs []string
	for _, column := range slices.Sorted(maps.Keys(kanban.CellColors)) {
		if columns != nil && !slices.Contains(columns, column) {
			errs = append(errs, fmt.Sprintf("%s: %q is not a column of the board (columns: %s)",
				at, column, strings.Join(columns, ", ")))
		}
		cells := kanban.CellColors[column]
		for _, lane := range slices.Sorted(maps.Keys(cells)) {
			cellAt := fmt.Sprintf("%s.%s.%s", at, column, lane)
			if lanes != nil && !slices.Contains(lanes, lane) {
				errs = append(errs, fmt.Sprintf("%s: %q is not a swimlane of the board (swimlanes: %s)",
					cellAt, lane, strings.Join(lanes, ", ")))
			}
			cell := cells[lane]
			_, tone, err := ParseToneOrColor(cell.Color, cfg.OperatorColors())
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", cellAt, err))
				continue
			}
			if !tone && strings.TrimSpace(cell.Label) == "" {
				errs = append(errs, fmt.Sprintf(
					"%s: color %q needs a label, so the cell does not rely on color alone", cellAt, cell.Color))
			}
		}
	}
	return errs
}

// shownKanbanValues is the values a board shows for an axis: the declared
// ones, else every enum value of the property. Nil when the property is not
// an enum of the entity.
func shownKanbanValues(
	entDef *metamodel.EntityDef, meta *metamodel.Metamodel, property string, declared []string,
) []string {
	propDef, ok := entDef.Properties[property]
	if !ok {
		return nil
	}
	values := GetValidEnumValues(propDef, meta)
	if len(values) == 0 {
		return nil
	}
	if len(declared) > 0 {
		return declared
	}
	return values
}

func kanbanColumnValues(columns []KanbanColumn) []string {
	out := make([]string, len(columns))
	for i, c := range columns {
		out[i] = c.Value
	}
	return out
}

func kanbanLaneValues(lanes []KanbanSwimlane) []string {
	out := make([]string, len(lanes))
	for i, l := range lanes {
		out[i] = l.Value
	}
	return out
}

// ResolveKanbanCellClasses returns a copy of cells with each Class set to the
// style class of its color, for serving to the SPA. Config load has validated
// every color, so one that does not parse keeps an empty class.
func ResolveKanbanCellClasses(
	cells map[string]map[string]KanbanCellColor, operator map[string]OperatorColor,
) map[string]map[string]KanbanCellColor {
	if len(cells) == 0 {
		return cells
	}
	out := make(map[string]map[string]KanbanCellColor, len(cells))
	for column, lanes := range cells {
		row := make(map[string]KanbanCellColor, len(lanes))
		for lane, cell := range lanes {
			if c, _, err := ParseToneOrColor(cell.Color, operator); err == nil {
				cell.Class = c.StyleClass()
			}
			row[lane] = cell
		}
		out[column] = row
	}
	return out
}
