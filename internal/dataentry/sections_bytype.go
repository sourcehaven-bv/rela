package dataentry

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// fieldsFor returns the fields a row of entityType renders in sec: its
// `fields_by_type` entry, or the section's `fields:` (TKT-HF0D5W).
func fieldsFor(sec ViewSection, entityType string) []ViewSectionField {
	if f, ok := sec.FieldsByType[entityType]; ok {
		return f
	}
	return sec.Fields
}

// columnsFor returns the columns a row of entityType renders in sec: its
// `columns_by_type` entry, or the section's `columns:`.
func columnsFor(sec ViewSection, entityType string) []ListColumn {
	if c, ok := sec.ColumnsByType[entityType]; ok {
		return c
	}
	return sec.Columns
}

// typeGroup is the rows of one entity type in a `columns_by_type` table.
type typeGroup struct {
	entityType string
	entities   []*entity.Entity
}

// groupByType splits entities into one group per type (TKT-HF0D5W).
//
// Groups are ordered by type name, so the order does not change with the
// data. Rows keep the section's order within their group. The caller
// resolves relation columns once per group, which is still one query per
// (column, row type), as on an ungrouped table.
func groupByType(entities []*entity.Entity) []typeGroup {
	byType := map[string][]*entity.Entity{}
	for _, e := range entities {
		byType[e.Type] = append(byType[e.Type], e)
	}
	groups := make([]typeGroup, 0, len(byType))
	for t, es := range byType {
		groups = append(groups, typeGroup{entityType: t, entities: es})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].entityType < groups[j].entityType })
	return groups
}
