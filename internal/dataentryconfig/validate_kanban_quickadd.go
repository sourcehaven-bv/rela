package dataentryconfig

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// kanbanQuickAddTitle is the property a quick-add card's title is written
// to: the card's title property, else the type's primary property. The SPA
// applies the same rule. "" means the board has none.
func kanbanQuickAddTitle(kanban Kanban, entDef *metamodel.EntityDef) string {
	if kanban.Card.Title != "" {
		return kanban.Card.Title
	}
	return entDef.GetPrimaryProperty()
}

// validateKanbanQuickAdd checks that a board with `quick_add: true` can
// create a card from a title alone (TKT-IP4273S).
//
// A quick-add create carries the title, the column value and the swimlane
// value, and nothing else. A required property with no other source would
// therefore be unset on every card the board creates. At write time that is
// only a warning (DEC-HWZHA), but here it is a config error, for the reason
// [validateCalDAVConstructible] gives: the operator sees it once at startup,
// whereas a warning on every card is seen by nobody. Falling back to the
// full form instead was rejected, because the board cannot know at Enter
// time what the form would ask for.
//
// Template defaults are not consulted, as for CalDAV: templates live on
// disk and this validator receives only (cfg, meta). The template still
// applies at create time, so a property it fills is filled; it just does not
// count here. The error says to add a schema default instead.
func validateKanbanQuickAdd(
	kanbanID string, kanban Kanban, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	if !kanban.QuickAdd {
		return nil
	}
	prefix := fmt.Sprintf("kanban %q: quick_add", kanbanID)
	var errs []string
	if entDef.IsManualID() {
		errs = append(errs, fmt.Sprintf(
			"%s: entity %q uses manual ids, and a quick-add card has no id field", prefix, kanban.EntityType))
	}
	title := kanbanQuickAddTitle(kanban, entDef)
	titleDef, ok := entDef.Properties[title]
	switch {
	case title == "":
		errs = append(errs, fmt.Sprintf(
			"%s: needs a title property; set card.title or a display_property on %q", prefix, kanban.EntityType))
	case !ok:
		errs = append(errs, fmt.Sprintf("%s: title property %q is not a property of %q", prefix, title, kanban.EntityType))
	case titleDef.Computed != "" || titleDef.Rollup != nil:
		errs = append(errs, fmt.Sprintf("%s: title property %q is computed, so a card cannot set it", prefix, title))
	}

	filled := map[string]bool{title: true, kanban.ColumnProperty: true, kanban.SwimlaneProperty: true}
	var missing []string
	for _, prop := range sortedPropertyNames(entDef) {
		def := entDef.Properties[prop]
		if !def.Required || filled[prop] || def.Default != "" || meta.Types[def.Type].Default != "" {
			continue
		}
		missing = append(missing, prop)
	}
	if len(missing) > 0 {
		errs = append(errs, fmt.Sprintf(
			"%s: required propert%s %s of %q %s no value on a quick-add card, which sets only the title, "+
				"column and swimlane. Give %s a default in the schema, or remove quick_add",
			prefix, plural(len(missing), "y", "ies"), strings.Join(quoteAll(missing), ", "), kanban.EntityType,
			plural(len(missing), "has", "have"), plural(len(missing), "it", "them")))
	}
	return errs
}
