/**
 * Maps rela's operator-authored `ListColumn[]` onto the component library's
 * `TableColumn[]`.
 *
 * The two models disagree on identity, which is the whole reason this is a
 * module with tests rather than an inline `.map()`. A `ListColumn` has no id:
 * it is keyed by `property` OR `relation`, and an incoming relation column is
 * distinguished from an outgoing one of the same type only by `direction`. A
 * `TableColumn.key` has to be a single stable string, because it names the
 * column AND its `cell-<key>` slot.
 *
 * A wrong key here fails SILENTLY: Vue resolves slot names at runtime, so a
 * `cell-<key>` slot that matches nothing renders empty with no error from
 * vue-tsc, eslint or the unit suite. That is not hypothetical — the library
 * hit exactly this during its own column rename, where two untyped column
 * literals compiled clean and a status picker quietly fell back to plain
 * text. Hence `columnKey` is pinned by its own tests, collisions are asserted
 * impossible, and the arrays are annotated `TableColumn[]` so the compiler
 * sees a rename rather than an object literal that happens to fit.
 */
import type { ListColumn } from '@/types/config'
import type { PropertyDef } from '@/types'
import type { TableColumn } from 'rela-components/components/table/types'
import { t } from '@/i18n'

/**
 * The stable identifier for a column, naming both the column and its
 * `cell-<key>` slot.
 *
 * Prefixed by kind so a property and a relation that share a name cannot
 * collide, and suffixed by direction so the incoming and outgoing halves of
 * one relation type stay distinct — a list may legitimately show both, which
 * is the case a bare relation name would silently merge.
 */
export function columnKey(column: ListColumn): string {
  if (column.face) return 'face'
  // A list has at most one busy column (load-checked), so the key is fixed.
  if (column.busy) return 'busy'
  if (column.property) return `prop:${column.property}`
  if (column.relation) {
    // Outgoing is the default in ListColumn, so spell it rather than leaving
    // it implicit: `rel:blocks` and `rel:blocks:incoming` would otherwise
    // differ in shape as well as in value.
    return `rel:${column.relation}:${column.direction ?? 'outgoing'}`
  }
  // Neither key present is malformed config rather than a shape to support.
  // A stable placeholder keeps the table rendering instead of collapsing the
  // whole list, and repeated placeholders are made unique by the caller.
  return 'col:unnamed'
}

/** The header text a column shows, falling back the way rela's table did. */
export function columnHeader(column: ListColumn): string {
  const own = column.label || column.property || column.relation
  if (own) return own
  if (column.face) return t('list.column.face')
  // A busy column's header is never the raw channel type name, which reads
  // like an identifier; each cell's accessible name carries the type label.
  return column.busy ? t('list.column.busy') : ''
}

/**
 * The fixed width of a busy column: the dots, a short state label, the time
 * and "+1" ("Planning · 3h +1"). A longer label is cut off with an ellipsis.
 */
export const BUSY_COLUMN_WIDTH = 160

/** The width of an image column: the 32px thumbnail plus the cell padding. */
export const IMAGE_COLUMN_WIDTH = 64

/** True for a `file` property marked `display: {image: true}`. */
export function isImageProperty(property: PropertyDef | undefined): boolean {
  return property?.type === 'file' && !property.list && property.display?.image === true
}

/**
 * The image columns listed before the name column. The library's table puts
 * the name first and cannot place a column before it, so these render inside
 * the name cell, left of the title, like an avatar. `toTableColumns` drops
 * them so they are not repeated as ordinary columns.
 */
export function leadingImageColumns(
  columns: ListColumn[],
  name: ListColumn | undefined,
  properties: Record<string, PropertyDef> = {}
): ListColumn[] {
  const at = name ? columns.indexOf(name) : -1
  if (at <= 0) return []
  const seen = new Set<string>()
  return columns.slice(0, at).filter((column) => {
    if (!column.property || !isImageProperty(properties[column.property]) || seen.has(column.property)) return false
    seen.add(column.property)
    return true
  })
}

/**
 * A property column's preferred and minimum width, by the property's type.
 *
 * The library's default (150px, shrinking to 112px) suits free text. A date,
 * an amount or a yes/no needs far less, and giving it the text width is what
 * pushed a five-column list past a laptop page beside the sidebar. Sized for
 * the formatted value plus the cell's padding; a longer value is cut with an
 * ellipsis, as any cell's is. Undefined keeps the library's default.
 */
export function propertyColumnSize(
  property: PropertyDef | undefined
): Pick<TableColumn, 'width' | 'minWidth'> | undefined {
  if (!property || property.list) return undefined
  // An image shows as a 32px thumbnail (ImageCellWidget), so the column is the
  // thumbnail plus the cell's padding, not the text width.
  if (isImageProperty(property)) return { width: IMAGE_COLUMN_WIDTH, minWidth: IMAGE_COLUMN_WIDTH }
  if (property.type === 'enum' || (property.values?.length ?? 0) > 0) return { width: 128, minWidth: 96 }
  switch (property.type) {
    case 'date':
      return { width: 128, minWidth: 116 }
    case 'datetime':
      return { width: 168, minWidth: 136 }
    case 'integer':
      return { width: 104, minWidth: 80 }
    case 'decimal':
      return { width: 112, minWidth: 88 }
    case 'boolean':
      return { width: 96, minWidth: 72 }
    default:
      return undefined
  }
}

/**
 * Converts rela's list columns to the library's, dropping the name column.
 *
 * The library's table owns the leading name column itself — it holds the
 * row's title and its primary control. Passing that column again would
 * render the title twice.
 *
 * `name` is the column taken out, as `nameColumn` picks it; the caller still
 * needs it to render the title cell's contents through the `name` slot. Any
 * other column is an ordinary column and renders as configured, except an
 * image column listed before the name (`leadingImageColumns`), which renders
 * inside the name cell.
 */
export function toTableColumns(
  columns: ListColumn[],
  name: ListColumn | undefined = nameColumn(columns),
  properties: Record<string, PropertyDef> = {}
): TableColumn[] {
  const seen = new Set<string>()
  const leading = new Set(leadingImageColumns(columns, name, properties))
  return columns
    .filter((column) => column !== name && !leading.has(column))
    .map((column) => {
      let key = columnKey(column)
      // Malformed config can repeat the placeholder. Duplicate keys would make
      // one `cell-<key>` slot serve two columns, so disambiguate rather than
      // let the second one quietly take the first one's rendering.
      if (seen.has(key)) {
        let n = 2
        while (seen.has(`${key}:${n}`)) n += 1
        key = `${key}:${n}`
      }
      seen.add(key)

      return {
        key,
        header: columnHeader(column),
        // No `field`: every cell goes through the `cell-<key>` slot, because
        // rela renders property widgets, relation chips, a lock for an
        // inaccessible cell and a provenance badge. `field` covers only a
        // plain string or number, which is the one case rela does not have.
        sortable: column.sortable !== false && !!column.property,
        ...(column.busy ? { width: BUSY_COLUMN_WIDTH } : {}),
        ...(column.property ? propertyColumnSize(properties[column.property]) : {}),
        // `meta` carries the originating config so a cell slot can render from
        // the same object the rest of EntityList already reasons about,
        // instead of looking it back up by key. Namespaced by the library
        // precisely so this cannot collide with a future library property.
        meta: { listColumn: column },
      }
    })
}

/**
 * The title column described as a `TableColumn`, so the table's name header
 * carries a sort control like any other.
 *
 * Shares `columnKey`, so the key the header emits on `sort-click` resolves
 * back to the same `ListColumn` every other column does.
 *
 * Nil: undefined when the list declares no columns at all.
 */
export function toTableNameColumn(column: ListColumn | undefined): TableColumn | undefined {
  if (!column) return undefined
  return {
    key: columnKey(column),
    header: columnHeader(column),
    sortable: column.sortable !== false && !!column.property,
    meta: { listColumn: column },
  }
}

/**
 * The column holding the row title, which the library's table renders itself:
 * the column that carries `link`, else the first column.
 *
 * Nil: undefined when the list declares no columns at all.
 */
export function nameColumn(columns: ListColumn[]): ListColumn | undefined {
  return columns.find((column) => column.link) ?? columns[0]
}

/** Reads a converted column's originating rela config back out of `meta`. */
export function listColumnOf(column: TableColumn): ListColumn | undefined {
  return (column.meta as { listColumn?: ListColumn } | undefined)?.listColumn
}
