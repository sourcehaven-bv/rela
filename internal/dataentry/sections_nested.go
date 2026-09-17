package dataentry

import (
	"context"
	"sort"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// nestedNodeBudget caps how many rows ONE nested section may emit, parents and
// children together.
//
// The view pipeline has no other bound: `_views` takes no paging parameter and
// applies no node budget, only a traversal depth cap, so a project with 1,400
// tasks would otherwise ship all of them in one response. 2000 matches the
// gantt's default (internal/dataentryconfig.defaultGanttMaxNodes) because it is
// the same kind of limit on the same kind of tree; unlike the gantt's, this one
// is not operator-configurable yet — a section that needs more should say so
// with a ticket rather than a knob nobody tunes.
const nestedNodeBudget = 2000

// nestedChildPreview caps how many children ONE parent contributes, so a
// single huge parent cannot consume the whole section budget and starve its
// siblings. A parent over the cap reports HasMoreChildren with the true
// ChildCount, which the UI renders as "Showing N of M".
const nestedChildPreview = 25

// buildNestedTree assembles the parent rows of a `display: nested` section,
// each with the children nested under it, and reports whether the budget cut
// anything visible.
//
// parents arrives ALREADY row-gated and field-redacted: the Redact/Filter
// block at the end of [viewsHandler.executeViewRef] (views.go) runs
// viewReader.Filter over every collection once, before any section builder,
// and PolicyReader.Filter redacts each surviving row. Children are resolved by
// looking their ids up in the equally-gated child collection, so that lookup
// IS the row gate: an id in result.Parents naming an entity this caller cannot
// read simply misses, and is dropped.
//
// Nothing here re-reads the store, which is what keeps the gate in one place
// (DEC-ZBI39P) and is the property that would make a future rollup safe to
// fold here (TKT-ZAD9PS).
//
// Truncated reports that some VISIBLE row was not emitted — either a parent the
// node budget could not afford, or children beyond one parent's preview cap. It
// is never set merely because the budget reached zero, so a caller whose gated
// tree fits sees false even when the ungated tree would not, and the flag
// cannot be used to probe how much is being withheld (the gantt's rule).
//
// It is deliberately COARSER than the per-node HasMoreChildren and implied by
// it: any parent over the preview cap sets both. The section-level flag exists
// for the case HasMoreChildren cannot express — a parent dropped entirely
// leaves no row to carry a signal.
func (h *viewsHandler) buildNestedTree(
	ctx context.Context, sec ViewSection, parents []*entity.Entity, result *viewResult,
) ([]SectionTreeNode, bool) {
	if len(parents) == 0 {
		return nil, false
	}
	s := h.schema()

	// Children of every parent, gated by the lookup described above. Built
	// once for the section so the row builders below get one relation batch
	// rather than one per parent.
	childByID := make(map[string]*entity.Entity, len(result.Collections[sec.Children]))
	for _, e := range result.Collections[sec.Children] {
		childByID[e.ID] = e
	}
	edges := result.Parents[sec.Children]

	// Decide WHAT will be emitted before resolving anything about it.
	//
	// Selection first, then resolution: relation columns cost a query per
	// (column, row type) plus a gated header batch over every neighbor, so
	// resolving over all visible children rather than the emitted ones would
	// do that work for rows the budget is about to discard. On a 1,400-epic
	// project with 200 tasks each that is ~280k ids to render at most
	// nestedNodeBudget rows.
	plan, truncated := planNestedRows(parents, edges, childByID)

	rowEntities := make([]*entity.Entity, 0, len(plan)+nestedNodeBudget)
	for _, row := range plan {
		rowEntities = append(rowEntities, row.parent)
		rowEntities = append(rowEntities, row.children...)
	}

	// ONE resolution for the emitted rows — parents and children together —
	// instead of one per node (TKT-1U8XYN).
	//
	// Resolved over the UNION of every level's columns, because a row's own
	// columns depend on its type and level; resolving per (level, type) would
	// reissue the query per group. resolveRelationColumns keys its result by
	// entity id and column index, so each row then reads the indices of the
	// columns it actually renders — see nestedColumnsFor.
	allColumns, columnIndex := nestedColumnUnion(sec)
	relValues := h.resolveRelationColumns(ctx, s, allColumns, rowEntities)

	nodes := make([]SectionTreeNode, 0, len(plan))
	for _, row := range plan {
		parentCols := sec.ParentColumns[row.parent.Type]
		parentRel := indexed(relValues, columnIndex, parentCols)
		node := SectionTreeNode{
			SectionEntityData: h.buildNestedEntityData(ctx, sec, row.parent, result),
			Columns:           parentCols,
			Row:               h.buildSectionRow(s, parentCols, row.parent, parentRel),
			ChildCount:        row.childCount,
			HasMoreChildren:   len(row.children) < row.childCount,
		}
		for _, child := range row.children {
			childCols := sec.ChildColumns[child.Type]
			childRel := indexed(relValues, columnIndex, childCols)
			node.Children = append(node.Children, SectionTreeNode{
				SectionEntityData: h.buildNestedEntityData(ctx, sec, child, result),
				Columns:           childCols,
				Row:               h.buildSectionRow(s, childCols, child, childRel),
			})
		}
		nodes = append(nodes, node)
	}
	return nodes, truncated
}

// nestedColumnUnion flattens every level's per-type columns into one list for
// a single relation-column resolution, plus an index from a column's identity
// back to its position in that list.
//
// Identity is (relation, direction, property) rather than the ListColumn value
// itself, because two levels declaring the same relation column must share one
// resolution rather than issue two.
func nestedColumnUnion(sec ViewSection) (all []ListColumn, index map[string]int) {
	index = map[string]int{}
	add := func(cols []ListColumn) {
		for _, c := range cols {
			key := columnKey(c)
			if _, seen := index[key]; seen {
				continue
			}
			index[key] = len(all)
			all = append(all, c)
		}
	}
	for _, entityType := range sortedColumnTypes(sec.ParentColumns) {
		add(sec.ParentColumns[entityType])
	}
	for _, entityType := range sortedColumnTypes(sec.ChildColumns) {
		add(sec.ChildColumns[entityType])
	}
	return all, index
}

// columnKey identifies a column for de-duplication across levels.
func columnKey(c ListColumn) string {
	return string(c.Direction) + "\x00" + c.Relation + "\x00" + c.Property
}

// sortedColumnTypes returns a map's keys in a deterministic order, so the union —
// therefore the relation query — does not vary between runs.
func sortedColumnTypes(m map[string][]ListColumn) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// indexed re-keys the section-wide relation values onto ONE row's column
// positions, so buildSectionRow can stay positional over the columns it was
// handed.
//
// Returns nil when no column is a relation column: the common case, and
// buildSectionRow only indexes this for relation columns.
func indexed(
	relValues map[string]map[int][]string, columnIndex map[string]int, cols []ListColumn,
) map[string]map[int][]string {
	var needed []int
	for _, c := range cols {
		if c.Relation != "" {
			needed = append(needed, columnIndex[columnKey(c)])
		}
	}
	if len(needed) == 0 {
		return nil
	}
	out := make(map[string]map[int][]string, len(relValues))
	for entityID, byUnionIdx := range relValues {
		row := map[int][]string{}
		for rowIdx, c := range cols {
			if c.Relation == "" {
				continue
			}
			if vals, ok := byUnionIdx[columnIndex[columnKey(c)]]; ok {
				row[rowIdx] = vals
			}
		}
		out[entityID] = row
	}
	return out
}

// nestedRow is one planned parent row: the parent, the children that fit the
// budget, and how many children it actually has.
type nestedRow struct {
	parent   *entity.Entity
	children []*entity.Entity
	// childCount is the number of READABLE children, which exceeds
	// len(children) when the budget or the per-parent preview cut some.
	childCount int
}

// planNestedRows picks the rows a nested section will emit, spending the node
// budget across parents and their children, and reports whether anything
// visible was cut.
//
// edges may name children this caller cannot read — it was built from raw
// store rows before the gate. childByID holds only readable children, so a
// missing id is a hidden one and skipping it IS the row gate. Every count
// returned is therefore over readable children only, and cannot disclose a
// hidden child's existence.
//
// truncated is true when a VISIBLE row was not emitted (a dropped parent or
// children past the preview cap), never merely because the budget reached zero
// — so it cannot be used to probe how large the ungated tree is. Same rule as
// the gantt's budget.
func planNestedRows(
	parents []*entity.Entity, edges map[string][]string, childByID map[string]*entity.Entity,
) (rows []nestedRow, truncated bool) {
	budget := nestedNodeBudget
	rows = make([]nestedRow, 0, len(parents))
	for _, p := range parents {
		if budget <= 0 {
			// A visible parent we cannot emit: that IS the truncation signal.
			// Note a dropped parent leaves no row to carry HasMoreChildren,
			// so this flag is the only way the caller learns of it.
			return rows, true
		}
		budget--

		var kids []*entity.Entity
		for _, childID := range edges[p.ID] {
			if child, ok := childByID[childID]; ok {
				kids = append(kids, child)
			}
		}

		limit := min(len(kids), nestedChildPreview, budget)
		budget -= limit
		if limit < len(kids) {
			truncated = true
		}
		rows = append(rows, nestedRow{parent: p, children: kids[:limit], childCount: len(kids)})
	}
	return rows, truncated
}

// sectionTreeNodeToV1 projects one nested row onto the wire, recursing into
// its children.
//
// Recursion is bounded by construction: [viewsHandler.buildNestedTree] emits
// exactly two levels, so this cannot run away on a cyclic containment graph —
// the cycle would have to exist in the built tree, and nothing builds one.
func sectionTreeNodeToV1(node SectionTreeNode) v1.ViewTreeNode {
	out := v1.ViewTreeNode{
		Entity:          sectionEntityToV1(node.SectionEntityData),
		ChildCount:      node.ChildCount,
		HasMoreChildren: node.HasMoreChildren,
	}
	for _, col := range node.Columns {
		out.Columns = append(out.Columns, v1.ViewColumn{
			Property: col.Property, Relation: col.Relation, Label: col.Label, Link: col.Link,
		})
	}
	for _, cell := range node.Row.Cells {
		out.Cells = append(out.Cells, v1.ViewCell(cell))
	}
	for _, child := range node.Children {
		out.Children = append(out.Children, sectionTreeNodeToV1(child))
	}
	return out
}

// buildNestedEntityData is [viewsHandler.buildSectionEntityData] for a nested
// row, which carries no markdown body.
//
// A nested section is a scannable tree of many rows, so shipping every body
// would send content nobody renders — the cost `include_content` exists to
// avoid on the list path (see rowcontent.go). Content stays opt-in by display
// mode: only `content`/`cards` set it.
func (h *viewsHandler) buildNestedEntityData(
	ctx context.Context, sec ViewSection, e *entity.Entity, result *viewResult,
) SectionEntityData {
	eDef, _ := h.schema().Meta.GetEntityDef(e.Type)
	return h.buildSectionEntityData(ctx, e, sec.Fields, eDef, sec.Render, result.World)
}

// buildSectionRow composes one row's configured `columns:` cells.
//
// relValues holds the section's pre-resolved relation-column values keyed by
// entity id then column index; it MUST already cover e, since a miss here
// silently renders an empty cell rather than issuing a per-row query.
//
// Extracted from the `table` arm so `nested` renders the same cell vocabulary
// from the same code — two implementations of a cell would drift on widget
// resolution and multi-value handling.
func (h *viewsHandler) buildSectionRow(
	s *Schema, columns []ListColumn, e *entity.Entity, relValues map[string]map[int][]string,
) SectionRowData {
	eDef, _ := s.Meta.GetEntityDef(e.Type)
	row := SectionRowData{
		EntityID: e.ID, EntityType: e.Type, EditFormID: h.editFormForType(e.Type),
		Self: rowSelfHref(s.Meta, e),
	}
	for ci, col := range columns {
		cell := SectionColumnData{
			Link: resolveLinkTarget(col.Link, e.Type, e.ID), EntityID: e.ID, EntityType: e.Type,
		}
		if col.Relation != "" {
			cell.Values = relValues[e.ID][ci]
		} else {
			fillPropertyCell(&cell, s, eDef, e, col.Property)
		}
		row.Cells = append(row.Cells, cell)
	}
	return row
}

// fillPropertyCell resolves a property column's type, widget and values onto
// cell.
//
// A property the entity type does not declare is not an error: the store is
// permissive and a hand-edited file may carry anything, so an unknown property
// yields an untyped, empty cell rather than a failure.
func fillPropertyCell(
	cell *SectionColumnData, s *Schema, eDef *metamodel.EntityDef, e *entity.Entity, property string,
) {
	var pd metamodel.PropertyDef
	if eDef != nil {
		if propDef, ok := eDef.Properties[property]; ok {
			pd = propDef
			cell.PropType = pd.Type
		}
	}
	cell.Widget = resolveWidget(pd, s.Meta)
	if vs := e.GetAttributeStrings(property); vs != nil {
		cell.Values = vs
	} else if val := e.GetAttributeString(property); val != "" {
		cell.Values = []string{val}
	}
}
