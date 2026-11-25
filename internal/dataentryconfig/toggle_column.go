package dataentryconfig

import (
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Toggle columns (TKT-M0AT73B).
//
// A list property column with `widget: toggle` or `widget: switch` renders a
// control the user flips in place. A flip is an ordinary PATCH, so the write
// keeps every field permission and transition check; the row's `_fields`
// only decides whether the SPA offers the control.

// Column widgets a list column accepts.
const (
	ColumnWidgetToggle = "toggle"
	ColumnWidgetSwitch = "switch"
)

// Values of a toggle column's `other:`, the state of a row whose enum value
// is neither `on` nor `off`.
const (
	ToggleOtherDisabled = "disabled"
	ToggleOtherHidden   = "hidden"
	ToggleOtherOn       = "on"
)

var (
	validColumnWidgets = map[string]bool{ColumnWidgetToggle: true, ColumnWidgetSwitch: true}
	validToggleOthers  = map[string]bool{
		ToggleOtherDisabled: true, ToggleOtherHidden: true, ToggleOtherOn: true,
	}
)

// hasToggleKeys reports whether c sets any toggle column key.
func hasToggleKeys(c ListColumn) bool {
	return c.Widget != "" || c.On != "" || c.Off != "" || c.Other != ""
}

// refuseToggleColumn rejects the toggle keys outside a list. The other
// surfaces sharing [ListColumn] have no write path for a cell.
func refuseToggleColumn(prefix string, c ListColumn) []string {
	if hasToggleKeys(c) {
		return []string{prefix + ": widget, on, off and other are only supported on list columns"}
	}
	return nil
}

// checkListToggleColumn checks one list column's toggle keys. The column
// must be a plain property column that is not the title column. A boolean
// property takes no on or off. An enum needs two distinct declared values,
// and `other:` exactly when it declares more values than those two, so a
// row holding a third value never renders as if it were off.
func checkListToggleColumn(
	listID string, i int, c ListColumn, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	if !hasToggleKeys(c) {
		return nil
	}
	at := fmt.Sprintf("list %q: column[%d]", listID, i)
	if c.Widget == "" {
		return []string{at + ": on, off and other need widget: toggle or widget: switch"}
	}
	if !validColumnWidgets[c.Widget] {
		return []string{fmt.Sprintf("%s has invalid widget %q (valid: %s)",
			at, c.Widget, joinMapKeys(validColumnWidgets))}
	}
	if i == 0 {
		return []string{at + ": the first column is the row title and cannot be a " + c.Widget}
	}
	if c.Property == "" || c.Relation != "" || c.Face || c.Busy != "" || c.Link != "" {
		return []string{fmt.Sprintf("%s: widget %s needs a property column without relation, face, busy or link",
			at, c.Widget)}
	}
	if c.Other != "" && !validToggleOthers[c.Other] {
		return []string{fmt.Sprintf("%s has invalid other %q (valid: %s)",
			at, c.Other, joinMapKeys(validToggleOthers))}
	}
	pd, ok := entDef.Properties[c.Property]
	if !ok {
		return nil
	}
	if pd.List || pd.Computed != "" {
		return []string{fmt.Sprintf("%s: widget %s cannot edit property %q, which is a list or computed",
			at, c.Widget, c.Property)}
	}
	if pd.Type == "boolean" {
		if c.On != "" || c.Off != "" || c.Other != "" {
			return []string{fmt.Sprintf("%s: property %q is a boolean; widget %s takes no on, off or other",
				at, c.Property, c.Widget)}
		}
		return nil
	}
	values := GetValidEnumValues(pd, meta)
	if len(values) == 0 {
		return []string{fmt.Sprintf("%s: widget %s needs a boolean or enum property; %q is %s",
			at, c.Widget, c.Property, pd.Type)}
	}
	return checkToggleEnum(at, c, values)
}

// checkToggleEnum checks on, off and other against an enum's values.
func checkToggleEnum(at string, c ListColumn, values []string) []string {
	if c.On == "" || c.Off == "" {
		return []string{fmt.Sprintf("%s: property %q is an enum; widget %s needs on and off", at, c.Property, c.Widget)}
	}
	var errs []string
	for _, v := range []string{c.On, c.Off} {
		if !slices.Contains(values, v) {
			errs = append(errs, fmt.Sprintf("%s: %q is not a value of property %q", at, v, c.Property))
		}
	}
	if c.On == c.Off {
		errs = append(errs, at+": on and off must differ")
	}
	if len(errs) > 0 {
		return errs
	}
	hasOthers := len(values) > 2
	switch {
	case hasOthers && c.Other == "":
		return []string{fmt.Sprintf("%s: property %q has values other than %q and %q; set other: disabled, hidden or on",
			at, c.Property, c.On, c.Off)}
	case !hasOthers && c.Other != "":
		return []string{fmt.Sprintf("%s: property %q has no values besides on and off; other does nothing",
			at, c.Property)}
	}
	return nil
}
