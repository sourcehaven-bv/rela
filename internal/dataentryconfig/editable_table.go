package dataentryconfig

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Editable table columns and section totals (TKT-4Q2EIVZ).
//
// A column's `render:` and a section's `totals:` only mean something on a
// view section with `display: table`. Every other surface that shares
// [ListColumn] or [ViewSection] refuses them, so a key that would do nothing
// is an error rather than a silent no-op.

// refuseColumnRender rejects `render:` on a column of a surface that has no
// editable cells: list columns, dashboard card columns, the nested levels of
// a view section, and side-panel sections.
func refuseColumnRender(prefix string, c ListColumn) []string {
	if c.Render != "" {
		return []string{prefix + ": render is only supported on the columns of a display: table view section"}
	}
	return nil
}

// validateSectionColumnRender checks `render:` on a view section's
// `columns:` and `columns_by_type:`, and refuses it on its nested levels.
func validateSectionColumnRender(viewID string, i int, s ViewSection) []string {
	prefix := fmt.Sprintf("view %q: section[%d]", viewID, i)
	var errs []string
	for j, c := range s.Columns {
		errs = append(errs, checkTableColumnRender(fmt.Sprintf("%s column[%d]", prefix, j), s.Display, c)...)
	}
	for _, t := range sortedColumnTypes(s.ColumnsByType) {
		for j, c := range s.ColumnsByType[t] {
			errs = append(errs, checkTableColumnRender(
				fmt.Sprintf("%s columns_by_type.%s[%d]", prefix, t, j), s.Display, c)...)
		}
	}
	for _, level := range []struct {
		key  string
		cols map[string][]ListColumn
	}{{"parent_columns", s.ParentColumns}, {"child_columns", s.ChildColumns}} {
		for _, t := range sortedColumnTypes(level.cols) {
			for j, c := range level.cols[t] {
				errs = append(errs, refuseColumnRender(fmt.Sprintf("%s %s.%s[%d]", prefix, level.key, t, j), c)...)
			}
		}
	}
	return errs
}

// checkTableColumnRender checks one view section column's render mode. An
// input column must be a plain property column: a relation column has no
// single value to edit, a face or busy column has no property at all, and a
// linked cell would navigate away on the click meant to edit it.
func checkTableColumnRender(prefix, display string, c ListColumn) []string {
	if c.Render == "" {
		return nil
	}
	if !validSectionRenderModes[c.Render] {
		return []string{fmt.Sprintf("%s has invalid render mode %q (valid: %s)",
			prefix, c.Render, joinMapKeys(validSectionRenderModes))}
	}
	if display != "table" {
		return []string{fmt.Sprintf("%s sets render on display mode %q; column render applies to display: table only",
			prefix, display)}
	}
	if c.Render != RenderInput {
		return nil
	}
	if c.Property == "" || c.Relation != "" || c.Face || c.Busy != "" {
		return []string{prefix + ": render: input needs a property column (not relation, face or busy)"}
	}
	if c.Link != "" {
		return []string{prefix + ": render: input cannot be combined with link; a click on the cell would navigate away"}
	}
	return nil
}

// validateSectionTotals checks `totals:` on a view section: a table only,
// and each line naming a property the view's entry type declares. Whether
// the reader may see the property is a per-principal question, answered
// when the view is read: a hidden total is left out then.
func validateSectionTotals(viewID string, i int, s ViewSection, entryType string, meta *metamodel.Metamodel) []string {
	if len(s.Totals) == 0 {
		return nil
	}
	prefix := fmt.Sprintf("view %q: section[%d]", viewID, i)
	if s.Display != "table" {
		return []string{fmt.Sprintf("%s sets totals on display mode %q; totals apply to display: table only",
			prefix, s.Display)}
	}
	var errs []string
	def, hasDef := meta.GetEntityDef(entryType)
	for j, tot := range s.Totals {
		at := fmt.Sprintf("%s totals[%d]", prefix, j)
		if tot.Property == "" {
			errs = append(errs, at+": property is required")
			continue
		}
		if !hasDef {
			continue // the unknown entry type is reported by the view check
		}
		if _, ok := def.Properties[tot.Property]; !ok {
			errs = append(errs, fmt.Sprintf("%s: entity %q has no property %q", at, entryType, tot.Property))
		}
	}
	return errs
}

// refuseSidePanelTableKeys rejects column render and totals on a side-panel
// section, whose renderer has neither editable cells nor a totals block.
func refuseSidePanelTableKeys(formID string, i int, sec ViewSection) []string {
	prefix := fmt.Sprintf("form %q: side_panel section[%d]", formID, i)
	var errs []string
	if len(sec.Totals) > 0 {
		errs = append(errs, prefix+" sets totals, which a side panel does not support")
	}
	for j, c := range sec.Columns {
		if c.Render != "" {
			errs = append(errs, fmt.Sprintf("%s column[%d] sets render, which a side panel does not support", prefix, j))
		}
	}
	for _, t := range sortedColumnTypes(sec.ColumnsByType) {
		for j, c := range sec.ColumnsByType[t] {
			if c.Render != "" {
				errs = append(errs, fmt.Sprintf("%s columns_by_type.%s[%d] sets render, which a side panel does not support",
					prefix, t, j))
			}
		}
	}
	return errs
}

// validateSectionInputColumnTypes refuses `render: input` on a property no
// cell can edit: a file (a cell has no attachment data, and its widget is the
// dense FileCellWidget), and a computed, roll-up or external-ref property,
// which the write path always refuses. The row types are the column's
// `columns_by_type` key, or every type the section's source collects. A type
// that does not declare the property is skipped: it renders an empty cell.
func validateSectionInputColumnTypes(
	viewID string, i int, s ViewSection, view ViewConfig, meta *metamodel.Metamodel,
) []string {
	if s.Display != "table" {
		return nil
	}
	prefix := fmt.Sprintf("view %q: section[%d]", viewID, i)
	var errs []string
	check := func(at string, c ListColumn, types []string) {
		if c.Render != RenderInput || c.Property == "" {
			return
		}
		for _, t := range types {
			def, ok := meta.GetEntityDef(t)
			if !ok {
				continue
			}
			pd, ok := def.Properties[c.Property]
			if !ok {
				continue
			}
			if why := uneditableInCell(pd); why != "" {
				errs = append(errs, fmt.Sprintf("%s: render: input on %s.%s, which %s", at, t, c.Property, why))
			}
		}
	}
	sourceTypes := collectionTypes(view, s.Source, meta)
	for j, c := range s.Columns {
		check(fmt.Sprintf("%s column[%d]", prefix, j), c, sourceTypes)
	}
	for _, t := range sortedColumnTypes(s.ColumnsByType) {
		for j, c := range s.ColumnsByType[t] {
			check(fmt.Sprintf("%s columns_by_type.%s[%d]", prefix, t, j), c, []string{t})
		}
	}
	return errs
}

// uneditableInCell says why a table cell cannot edit the property, or "".
func uneditableInCell(pd metamodel.PropertyDef) string {
	switch {
	case pd.Type == metamodel.PropertyTypeFile:
		return "is a file property; edit files on the form"
	case pd.Computed != "":
		return "is computed"
	case pd.Rollup != nil:
		return "is a roll-up"
	case pd.Type == metamodel.PropertyTypeExternalRef:
		return "is an external reference, written only by scripts and imports"
	}
	return ""
}

func sortedColumnTypes(m map[string][]ListColumn) []string {
	types := make([]string, 0, len(m))
	for t := range m {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
