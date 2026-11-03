package dataentryconfig

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// isRollup reports whether def declares prop as a roll-up (TKT-6I2OPG).
func isRollup(def *metamodel.EntityDef, prop string) bool {
	pd, ok := def.Properties[prop]
	return ok && pd.Rollup != nil
}

// rollupMisuse reports every place a list or a kanban filters, sorts or
// groups on a roll-up property. A roll-up is computed per reader when a row
// is served, so no store query and no list filter can see it. A roll-up may
// be a column; the column is served, not sortable.
func rollupMisuse(
	where string, def *metamodel.EntityDef, columns []ListColumn, sort []SortSpec,
	filters []FilterConfig, controls []FilterControl, groupBy *ListGroupBy,
) []string {
	var errs []string
	add := func(what, prop string) {
		errs = append(errs, fmt.Sprintf(
			"%s: %s uses roll-up property %q; a roll-up is computed when a row is read, "+
				"so it cannot be filtered, sorted or grouped on", where, what, prop))
	}
	for i, c := range columns {
		if c.Sortable && isRollup(def, c.Property) {
			add(fmt.Sprintf("column[%d] (sortable)", i), c.Property)
		}
	}
	for i, s := range sort {
		if isRollup(def, s.Property) {
			add(fmt.Sprintf("sort[%d]", i), s.Property)
		}
	}
	for i, f := range filters {
		if isRollup(def, f.Property) {
			add(fmt.Sprintf("filters[%d]", i), f.Property)
		}
	}
	for i, fc := range controls {
		if isRollup(def, fc.Property) {
			add(fmt.Sprintf("filter_controls[%d]", i), fc.Property)
		}
	}
	if groupBy != nil && isRollup(def, groupBy.Property) {
		add("group_by", groupBy.Property)
	}
	return errs
}
