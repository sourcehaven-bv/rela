package dataentryconfig

import (
	"fmt"
	"sort"
)

// checkListBusyColumn checks the shape of a list busy column (TKT-TU3XH4):
// it takes no key beside Label, a list has at most one, and it is not the
// first column. The SPA renders column 0 as the row title and link, so a
// busy column there would leave the row without either. earlierBusy says
// whether an earlier column of the same list is busy.
func checkListBusyColumn(listID string, i int, c ListColumn, earlierBusy bool) []string {
	at := fmt.Sprintf("list %q: column[%d]", listID, i)
	var errs []string
	if c.Property != "" || c.Relation != "" || c.Direction != "" || c.Face || c.Link != "" || c.Sortable {
		errs = append(errs, at+": busy cannot be combined with property, relation, direction, face, link or sortable")
	}
	if earlierBusy {
		errs = append(errs, at+": a list has at most one busy column")
	}
	if i == 0 {
		errs = append(errs, at+": the first column is the row title and cannot be busy")
	}
	return errs
}

// refuseListOnlyColumn rejects the list-only column keys on the other
// surfaces that share [ListColumn]: dashboard card columns and view section
// columns. Those surfaces do not request busy state, so a busy column there
// would render nothing. It mirrors [refuseKanbanOnlyFields].
func refuseListOnlyColumn(prefix string, c ListColumn) []string {
	if c.Busy != "" {
		return []string{prefix + ": busy is only supported on list columns"}
	}
	return nil
}

// refuseSectionBusyColumns applies [refuseListOnlyColumn] to every column
// list of a view section: columns, parent_columns, child_columns and
// columns_by_type. It needs no metamodel, so the caller runs it whether or
// not the section's source resolves, and each column is reported once.
func refuseSectionBusyColumns(viewID string, i int, s ViewSection) []string {
	prefix := fmt.Sprintf("view %q: section[%d]", viewID, i)
	var errs []string
	for j, c := range s.Columns {
		errs = append(errs, refuseListOnlyColumn(fmt.Sprintf("%s column[%d]", prefix, j), c)...)
	}
	for _, group := range []struct {
		key  string
		cols map[string][]ListColumn
	}{
		{"parent_columns", s.ParentColumns},
		{"child_columns", s.ChildColumns},
		{"columns_by_type", s.ColumnsByType},
	} {
		types := make([]string, 0, len(group.cols))
		for t := range group.cols {
			types = append(types, t)
		}
		sort.Strings(types)
		for _, t := range types {
			for j, c := range group.cols[t] {
				errs = append(errs, refuseListOnlyColumn(fmt.Sprintf("%s %s.%s[%d]", prefix, group.key, t, j), c)...)
			}
		}
	}
	return errs
}
