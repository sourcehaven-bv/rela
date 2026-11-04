// Inline editing of `display: table` section rows (TKT-4Q2EIVZ): which rows
// edit, and how a saved value reaches the row on screen. Kept out of
// EntityDetail so it is testable without mounting the page.

import type { ViewColumn, ViewRow, ViewSection } from '@/api/views'
import { entityRef } from '@/utils/entityRef'

/** Every row a table section shows: its flat rows and each group's rows. */
export function tableRowCount(section: ViewSection): number {
  let n = section.rows?.length ?? 0
  for (const group of section.groups ?? []) n += group.rows?.length ?? 0
  return n
}

/**
 * Whether a row renders editable cells. The server sends `_props` only on rows
 * of a section with an editable column, and the per-field verdict still
 * decides each cell. Over the cap the whole section renders display only: each
 * editable row holds its own autosave.
 */
export function tableRowEditable(
  row: ViewRow,
  columns: ViewColumn[] | undefined,
  rowCount: number,
  cap: number
): boolean {
  if (rowCount > cap || !row._props) return false
  return columns?.some((c) => c.render === 'input') ?? false
}

/** The key of a row in the apply index: the owner a row's autosave reports. */
export function tableRowKey(row: Pick<ViewRow, 'entityType' | 'entityId' | '_self'>): string {
  return `${row.entityType}/${entityRef({ id: row.entityId, _self: row._self })}`
}

/** Where a row sits: a flat row has `group` -1. */
export interface TableRowLoc {
  section: number
  group: number
  row: number
}

/** Indexes every table row of a view by {@link tableRowKey}. */
export function indexTableRows(sections: ViewSection[] | undefined): Map<string, TableRowLoc> {
  const index = new Map<string, TableRowLoc>()
  sections?.forEach((s, si) => {
    s.rows?.forEach((r, ri) => index.set(tableRowKey(r), { section: si, group: -1, row: ri }))
    s.groups?.forEach((g, gi) =>
      g.rows?.forEach((r, ri) => index.set(tableRowKey(r), { section: si, group: gi, row: ri }))
    )
  })
  return index
}

/**
 * Returns the sections with one property of one row replaced, or null when the
 * row at `loc` is no longer the owner (the layout changed under the save).
 * Only the changed row, its group and its section get new references. A value
 * the row already holds returns `sections` itself: a save response reports
 * every property of the row, not only the one saved.
 */
export function applyPropertyToTableRow(
  sections: ViewSection[],
  loc: TableRowLoc,
  ownerKey: string,
  prop: string,
  value: unknown
): ViewSection[] | null {
  const section = sections[loc.section]
  const rows = loc.group < 0 ? section?.rows : section?.groups?.[loc.group]?.rows
  const row = rows?.[loc.row]
  if (!section || !rows || !row || tableRowKey(row) !== ownerKey) return null
  if (sameValue(row._props?.[prop], value)) return sections
  const props = { ...(row._props ?? {}) }
  if (value === undefined) delete props[prop]
  else props[prop] = value
  const nextRows = rows.map((r, i) => (i === loc.row ? { ...row, _props: props } : r))
  const nextSection: ViewSection =
    loc.group < 0
      ? { ...section, rows: nextRows }
      : {
          ...section,
          groups: section.groups?.map((g, i) => (i === loc.group ? { ...g, rows: nextRows } : g)),
        }
  return sections.map((s, i) => (i === loc.section ? nextSection : s))
}

// Property values are JSON from the wire, so comparing their encoding is exact.
function sameValue(a: unknown, b: unknown): boolean {
  return a === b || JSON.stringify(a) === JSON.stringify(b)
}
