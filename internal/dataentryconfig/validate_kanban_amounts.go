package dataentryconfig

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateKanbanAmounts checks the two money-shaped kanban keys
// (TKT-SVLQQ6): `column_total:` on the board and `currency:` on a card field.
func validateKanbanAmounts(kanbanID string, kanban Kanban, entDef *metamodel.EntityDef) []string {
	var errs []string
	if t := kanban.ColumnTotal; t != nil {
		at := fmt.Sprintf("kanban %q: column_total", kanbanID)
		if kanban.SwimlaneProperty != "" {
			// A swimlane board has a heading per column per lane, so there is
			// no one place to show a column's total. Refuse rather than
			// accept a key that would render nothing.
			errs = append(errs, at+" is not supported on a board with swimlanes")
		}
		errs = append(errs, checkIntegerProperty(at, t.Property, kanban.EntityType, entDef, true)...)
		errs = append(errs, checkCurrencyCode(at, t.Currency)...)
	}
	for i, f := range kanban.Card.Fields {
		if f.Currency == "" {
			continue
		}
		at := fmt.Sprintf("kanban %q: card.fields[%d]", kanbanID, i)
		errs = append(errs, checkCurrencyCode(at, f.Currency)...)
		if f.Relation != "" || f.Comments || f.Display != "" {
			errs = append(errs, at+": currency needs a property field, not a relation, comments or display")
			continue
		}
		// validateKanbans already reports a property the type does not have.
		errs = append(errs, checkIntegerProperty(at, f.Property, kanban.EntityType, entDef, false)...)
	}
	return errs
}

// checkIntegerProperty reports unless name is a single-valued integer
// property of the entity type. rela has no decimal type, so integer is the
// only numeric one. A list property arrives as an array, which the board
// cannot add up, so it would show 0 rather than fail.
//
// reportUnknown is false where the caller already reports a property the
// type does not have, so the author sees one error, not two.
func checkIntegerProperty(at, name, entityType string, entDef *metamodel.EntityDef, reportUnknown bool) []string {
	if name == "" {
		return []string{at + ": property is required"}
	}
	def, ok := entDef.Properties[name]
	switch {
	case !ok && reportUnknown:
		return []string{fmt.Sprintf("%s: property %q not in entity %q", at, name, entityType)}
	case !ok:
		return nil
	case def.Type != metamodel.PropertyTypeInteger:
		return []string{fmt.Sprintf("%s: property %q must be an integer property, is %q", at, name, def.Type)}
	case def.List:
		return []string{fmt.Sprintf("%s: property %q is a list; use a single-valued integer property", at, name)}
	}
	return nil
}

// checkCurrencyCode accepts an empty code or three upper-case ASCII letters,
// the shape of an ISO 4217 code. It checks the shape, not a list of codes:
// the browser formats every well-formed code (an unknown one shows as the
// code itself), so a list would only reject codes it does not know yet.
func checkCurrencyCode(at, code string) []string {
	if code == "" {
		return nil
	}
	if len(code) != 3 || !isUpperASCII(code) {
		return []string{fmt.Sprintf("%s: currency %q is not an ISO 4217 code (three upper-case letters, such as EUR)", at, code)}
	}
	return nil
}

func isUpperASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}
