/**
 * The phone layout of a `display: table` view section (BUG-6WQ1U8S).
 *
 * On a narrow screen the section stacks each row into a block: the first
 * column is the row's heading, and every other column is a "label  value"
 * line. The CSS lives in EntityDetail; this module decides what each cell
 * carries, so the plain and the editable row render the same classes.
 */
import type { ViewCell, ViewColumn } from '@/api/views'

/** The header text of a column, the same text the table's `<th>` shows. */
export function sectionColumnLabel(col: ViewColumn | undefined): string {
  return col?.label || col?.property || col?.relation || ''
}

/**
 * Whether a read-only cell has nothing to show. The server sends `null`
 * values for an unset property or a relation without targets, whatever the
 * type says.
 */
export function isSectionCellEmpty(cell: ViewCell | undefined): boolean {
  return !cell || (cell.values ?? []).every((value) => value.trim() === '')
}

/**
 * The classes of one cell in a stacked row.
 *
 * - `--title`: the first column, which heads the stacked row.
 * - `--empty`: a read-only cell with no value. Its label would have no
 *   answer, so the stacked row leaves it out. An editable cell is never
 *   empty: the reader needs it to fill the value in.
 * - `--secondary`: the column has `primary: false`, so the stacked row
 *   leaves it out. The first column always stays: it is the row's heading.
 */
export function sectionCellClasses(
  col: ViewColumn | undefined,
  cell: ViewCell | undefined,
  index: number,
  editable = false
): Record<string, boolean> {
  const title = index === 0
  return {
    'section-cell--title': title,
    'section-cell--empty': !title && !editable && isSectionCellEmpty(cell),
    'section-cell--secondary': !title && col?.primary === false,
  }
}
