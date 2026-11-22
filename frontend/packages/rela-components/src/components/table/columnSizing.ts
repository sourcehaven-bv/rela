/**
 * How wide the table's columns are, in one place for the header and the rows.
 *
 * The header and every row are separate flex lines, so they line up only
 * because each one sizes its cells from the same numbers. Keeping those
 * numbers here, rather than in two templates, is what stops them drifting.
 *
 * A column's `width` is its preferred width, not a fixed one. When the table
 * is narrower than the name's basis plus every column, the name and the
 * columns all give up the same share of their basis, down to their minimums.
 * The flex algorithm distributes that by the basis alone, never by content,
 * so every row lands on the same widths as the header. Only below the sum of
 * the minimums does the table scroll sideways. A laptop page beside a sidebar
 * keeps its full table this way, where fixed widths used to overflow it.
 */
import type { TableColumn } from './types'

/** A column's preferred width when it names none. */
export const DEFAULT_COLUMN_WIDTH = 150

/** The name column's preferred width; it grows past this into free space. */
export const NAME_BASIS = 200

/** The narrowest the name column gets before a wide table scrolls sideways. */
export const NAME_MIN_WIDTH = 160

/**
 * The narrowest the title gets in a compressed row before the kept cells move
 * to a line of their own under it. Lower than the wide floor, so a title and
 * two badges still share one line on a phone.
 */
export const NAME_COMPACT_MIN_WIDTH = 120

/** How far a column may shrink below its width when it names no minimum. */
const DEFAULT_MIN_SHARE = 0.75

export function columnWidth(column: TableColumn): number {
  return column.width ?? DEFAULT_COLUMN_WIDTH
}

/** The column's floor, never above its width. */
export function columnMinWidth(column: TableColumn): number {
  const width = columnWidth(column)
  return Math.min(column.minWidth ?? Math.round(width * DEFAULT_MIN_SHARE), width)
}

/** The custom properties a header cell and a row cell size themselves by. */
export function cellSizeStyle(column: TableColumn): Record<string, string> {
  return {
    '--rl-cell-width': `${columnWidth(column)}px`,
    '--rl-cell-min-width': `${columnMinWidth(column)}px`,
  }
}

/** The narrowest the whole row gets before the table scrolls sideways. */
export function tableMinWidth(columns: TableColumn[]): number {
  return NAME_MIN_WIDTH + columns.reduce((sum, column) => sum + columnMinWidth(column), 0)
}

/** The last column a compressed row keeps, which takes the right gutter. */
export function lastKeptKey(columns: TableColumn[]): string | undefined {
  const kept = columns.filter((column) => column.primary)
  return kept[kept.length - 1]?.key
}

/**
 * The width a compressed row's kept cells take together, as a CSS length.
 *
 * Mirrors the compressed cell rule in `RlTableRow`: each kept cell is its
 * width capped to 30% of the table, and the last one also carries the right
 * gutter. Kept as an unevaluated `calc()` because the cap is a container
 * unit and the gutter a token, neither of which is known here.
 */
export function keptWidth(columns: TableColumn[]): string {
  const kept = columns.filter((column) => column.primary)
  if (kept.length === 0) return '0px'
  const cells = kept.map((column) => `min(${columnWidth(column)}px, 30cqi)`)
  return `calc(${cells.join(' + ')} + var(--rl-page-gutter-right) - var(--rl-space-2))`
}
