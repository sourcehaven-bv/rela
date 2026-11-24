package dataentryconfig

import (
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Toggle columns (TKT-FJG779Q).
//
// A list column with `widget: toggle` shows a checkbox for a boolean or enum
// property. On and Off name the two values it writes; Other says how a row
// with a third value shows.

// refuseToggleColumn rejects the toggle keys outside a list.
func refuseToggleColumn(prefix string, c ListColumn) []string {
	if c.Widget != "" || c.On != "" || c.Off != "" || c.Other != "" {
		return []string{prefix + ": widget, on, off and other are only supported on list columns"}
	}
	return nil
}

// validateToggleColumn checks the toggle keys of one list column.
func validateToggleColumn(
	prefix string, entDef *metamodel.EntityDef, c ListColumn, meta *metamodel.Metamodel,
) []string {
	if !c.IsToggle() {
		if c.Widget != "" {
			return []string{fmt.Sprintf("%s: unknown widget %q (valid: %s)", prefix, c.Widget, WidgetToggle)}
		}
		if c.On != "" || c.Off != "" || c.Other != "" {
			return []string{prefix + ": on, off and other need widget: toggle"}
		}
		return nil
	}
	if c.Property == "" || c.Relation != "" || c.Face || c.Busy != "" {
		return []string{prefix + ": a toggle column needs a property and takes no relation, face or busy"}
	}
	prop, ok := entDef.Properties[c.Property]
	if !ok {
		return nil // the column loop reports the unknown property
	}
	if c.On == "" || c.Off == "" {
		return []string{prefix + ": a toggle column needs both on and off"}
	}
	if c.On == c.Off {
		return []string{fmt.Sprintf("%s: on and off are both %q", prefix, c.On)}
	}
	if prop.Type == metamodel.PropertyTypeBoolean {
		return checkBoolToggle(prefix, c)
	}
	values := metamodel.EnumValues(prop, meta)
	if len(values) == 0 {
		return []string{fmt.Sprintf("%s: property %q is not a boolean or enum, so it cannot be a toggle", prefix, c.Property)}
	}
	return checkEnumToggle(prefix, c, values)
}

func checkBoolToggle(prefix string, c ListColumn) []string {
	var errs []string
	for _, v := range []string{c.On, c.Off} {
		if v != "true" && v != "false" {
			errs = append(errs, fmt.Sprintf("%s: a boolean toggle takes on and off of true or false, not %q", prefix, v))
		}
	}
	if c.Other != "" {
		errs = append(errs, prefix+": other is not needed on a boolean toggle")
	}
	return errs
}

func checkEnumToggle(prefix string, c ListColumn, values []string) []string {
	var errs []string
	for _, v := range []string{c.On, c.Off} {
		if !slices.Contains(values, v) {
			errs = append(errs, fmt.Sprintf("%s: %q is not a value of property %q", prefix, v, c.Property))
		}
	}
	switch {
	case len(values) > 2 && c.Other == "":
		errs = append(errs, fmt.Sprintf("%s: property %q has more than two values, so other is required (%s, %s or %s)",
			prefix, c.Property, ToggleOtherDisabled, ToggleOtherHidden, ToggleOtherOn))
	case len(values) <= 2 && c.Other != "":
		errs = append(errs, fmt.Sprintf("%s: other is not needed, property %q has only two values", prefix, c.Property))
	case c.Other != "" && c.Other != ToggleOtherDisabled && c.Other != ToggleOtherHidden && c.Other != ToggleOtherOn:
		errs = append(errs, fmt.Sprintf("%s: unknown other %q (valid: %s, %s, %s)",
			prefix, c.Other, ToggleOtherDisabled, ToggleOtherHidden, ToggleOtherOn))
	}
	return errs
}
