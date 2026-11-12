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

// rollupMisuse reports every place a kanban filters on a roll-up property.
// A board filters its cards in the browser, over the values the page
// carries, so a filter there could judge a card before its roll-up is
// computed. A list filters, sorts and groups in the server, where a roll-up
// is computed for every row first (TKT-6I2OPG), so a list may use one
// anywhere.
func rollupMisuse(where string, def *metamodel.EntityDef, filters []FilterConfig, controls []FilterControl) []string {
	var errs []string
	add := func(what, prop string) {
		errs = append(errs, fmt.Sprintf(
			"%s: %s uses roll-up property %q; a board cannot filter on a roll-up, a list can", where, what, prop))
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
	return errs
}
