package dataentryconfig

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateDashboardCardSource checks a card's row source and the properties
// and scope it folds over (TKT-2SGPJZ). prefix and i name the card.
func validateDashboardCardSource(
	cfg *Config, meta *metamodel.Metamodel, prefix string, i int, card DashboardCard,
) []string {
	at := fmt.Sprintf("%s: card[%d] %q", prefix, i, card.Title)
	switch {
	case card.List != "" && card.Query != "":
		return []string{at + " sets both list: and query:; set exactly one"}
	case card.List == "" && card.Query == "":
		return []string{at + " sets neither list: nor query:; set exactly one"}
	case card.List == "":
		if card.Display == "matrix" || card.Display == "ratio" {
			return []string{fmt.Sprintf("%s uses %s display, which needs list:", at, card.Display)}
		}
		return nil
	}
	listCfg, ok := cfg.Lists[card.List]
	if !ok {
		return []string{fmt.Sprintf("%s: list %q is not configured under lists:", at, card.List)}
	}
	if meta == nil {
		return nil
	}
	def, ok := meta.GetEntityDef(listCfg.EntityType)
	if !ok {
		// validateLists reports the unknown type.
		return nil
	}
	var errs []string
	prop := func(key, name string) (metamodel.PropertyDef, bool) {
		pd, ok := def.Properties[name]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: %s %q is not a property of %q (the type of list %q)",
				at, key, name, listCfg.EntityType, card.List))
		}
		return pd, ok
	}
	if card.GroupBy != "" {
		// A list value has no single group to count the row in.
		if pd, ok := prop("group_by", card.GroupBy); ok && pd.List {
			errs = append(errs, fmt.Sprintf("%s: group_by %q must be a single-valued property", at, card.GroupBy))
		}
	}
	switch card.Display {
	case "matrix":
		errs = append(errs, validateMatrixCard(cfg, meta, at, card, prop)...)
	case "ratio":
		errs = append(errs, validateRatioCard(meta, at, listCfg.EntityType, card)...)
	case "table":
		errs = append(errs, validateTableCard(at, card, prop)...)
	}
	if card.Display != "matrix" && card.setsMatrixOption() {
		errs = append(errs, at+": rows, columns (a property), tone_by, colors, rows_label, columns_label, "+
			"rows_order and columns_order apply to matrix display only")
	}
	if card.Display != "ratio" && card.Match != "" {
		errs = append(errs, at+": match applies to ratio display only")
	}
	return errs
}

// validateRatioCard checks a ratio card's match scope. `all` admits every row,
// so a ratio over it always reads 100%; it is refused as a likely mistake.
func validateRatioCard(meta *metamodel.Metamodel, at, typeName string, card DashboardCard) []string {
	switch card.Match {
	case "":
		return []string{at + " uses ratio display but has no match:"}
	case metamodel.AllQueryScopeName:
		return []string{fmt.Sprintf("%s: match %q admits every row, so the ratio is always 100%%", at, card.Match)}
	}
	// Name the card the way every other card error does.
	errs := checkQueryScopeRef(meta, "match", card.Match, typeName, card.Match)
	for i, e := range errs {
		errs[i] = at + ": " + e
	}
	return errs
}

// validateTableCard checks a list-sourced table card's columns, sort and
// limit against the list's type.
func validateTableCard(
	at string, card DashboardCard, prop func(key, name string) (metamodel.PropertyDef, bool),
) []string {
	var errs []string
	for _, c := range card.Columns {
		if c.Property != "" {
			prop("column", c.Property)
		}
	}
	for _, spec := range card.Sort {
		if spec.Property != "" {
			prop("sort", spec.Property)
		}
	}
	if card.Limit > maxListCardTableLimit {
		errs = append(errs, fmt.Sprintf("%s: limit %d is above the maximum of %d", at, card.Limit, maxListCardTableLimit))
	}
	return errs
}

// maxListCardTableLimit is the most rows a list-sourced table card shows. The
// server computes the card, so the limit bounds its work per request.
const maxListCardTableLimit = 100

// The values of a matrix card's `rows_order:` and `columns_order:`.
const (
	MatrixOrderAsc  = "asc"
	MatrixOrderDesc = "desc"
)

// MaxMatrixAxisValues caps an integer matrix axis. A declared range wider
// than this is a config error; an observed spread wider than this is cut
// off when the card is computed, and the card says so.
const MaxMatrixAxisValues = 50

// MatrixAxisTooWide reports whether the integers lo..hi are more than
// MaxMatrixAxisValues. It never overflows, whatever the bounds.
func MatrixAxisTooWide(lo, hi int64) bool {
	const span = MaxMatrixAxisValues - 1
	return lo <= math.MaxInt64-span && hi > lo+span
}

// setsMatrixOption reports whether the card sets a key that only a matrix
// card reads.
func (c DashboardCard) setsMatrixOption() bool {
	return c.Rows != "" || c.MatrixColumns != "" || c.ToneBy != "" || len(c.Colors) > 0 ||
		c.RowsLabel != "" || c.ColumnsLabel != "" || c.RowsOrder != "" || c.ColumnsOrder != ""
}

func validateMatrixCard(
	cfg *Config, meta *metamodel.Metamodel, at string, card DashboardCard,
	prop func(key, name string) (metamodel.PropertyDef, bool),
) []string {
	var errs []string
	axis := func(key, name, order string) {
		if order != "" && order != MatrixOrderAsc && order != MatrixOrderDesc {
			errs = append(errs, fmt.Sprintf("%s: %s_order %q must be %q or %q",
				at, key, order, MatrixOrderAsc, MatrixOrderDesc))
		}
		if name == "" {
			errs = append(errs, fmt.Sprintf("%s uses matrix display but has no %s:", at, key))
			return
		}
		pd, ok := prop(key, name)
		if !ok {
			return
		}
		isEnum := len(GetValidEnumValues(pd, meta)) > 0
		if pd.List || (!isEnum && pd.Type != metamodel.PropertyTypeInteger) {
			errs = append(errs, fmt.Sprintf("%s: %s %q must be a single-valued enum or integer property", at, key, name))
			return
		}
		// A declared range is the whole axis, so it must fit the cap. An
		// undeclared range is capped when the card is computed.
		if !isEnum && pd.Min != nil && pd.Max != nil && MatrixAxisTooWide(int64(*pd.Min), int64(*pd.Max)) {
			errs = append(errs, fmt.Sprintf("%s: %s %q declares %d..%d, more than the %d values a matrix axis shows",
				at, key, name, *pd.Min, *pd.Max, MaxMatrixAxisValues))
		}
	}
	axis("rows", card.Rows, card.RowsOrder)
	axis("columns", card.MatrixColumns, card.ColumnsOrder)
	if card.ToneBy == "" {
		if len(card.Colors) > 0 {
			errs = append(errs, at+": colors needs tone_by")
		}
		return errs
	}
	pd, ok := prop("tone_by", card.ToneBy)
	if !ok {
		return errs
	}
	values := GetValidEnumValues(pd, meta)
	if pd.List || len(values) == 0 {
		return append(errs, fmt.Sprintf("%s: tone_by %q must be a single-valued enum property", at, card.ToneBy))
	}
	for _, value := range slices.Sorted(maps.Keys(card.Colors)) {
		if !slices.Contains(values, value) {
			errs = append(errs, fmt.Sprintf("%s: colors key %q is not a value of %q (values: %s)",
				at, value, card.ToneBy, strings.Join(values, ", ")))
		}
		color := card.Colors[value]
		if SemanticTones[color] {
			continue
		}
		if _, err := ParseColor(color, cfg.OperatorColors()); err != nil {
			errs = append(errs, fmt.Sprintf("%s: colors[%s] %q is neither a tone (%s) nor a color: %v",
				at, value, color, joinMapKeys(SemanticTones), err))
		}
	}
	return errs
}

// validDashboardCardWidths are the values of a card's `width:` (TKT-H21TYS).
// An empty width means normal. A slice, unlike validDashboardDisplayModes, so
// the error lists the sizes from narrow to wide.
var validDashboardCardWidths = []string{"normal", "wide", "full"}

// validateDashboardCardWidth checks a card's `width:`. A number gets its own
// hint, because a column count is the shape an operator is most likely to
// guess.
func validateDashboardCardWidth(prefix string, i int, card DashboardCard) []string {
	if card.Width == "" || slices.Contains(validDashboardCardWidths, card.Width) {
		return nil
	}
	msg := fmt.Sprintf("%s: card[%d] %q has invalid width %q (valid: %s)",
		prefix, i, card.Title, card.Width, strings.Join(validDashboardCardWidths, ", "))
	if _, err := strconv.Atoi(card.Width); err == nil {
		msg += "; width takes a named size, not a column count"
	}
	return []string{msg}
}
