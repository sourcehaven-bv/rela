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
import type { TableColumn } from 'rela-components/components/table/types'

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
  return column.label || column.property || column.relation || (column.face ? 'Face' : '')
}

/**
 * Converts rela's list columns to the library's, dropping the FIRST column.
 *
 * The library's table owns the leading name column itself — it holds the
 * row's title and its primary control — and rela's convention is that
 * `columns[0]` is that title (the mobile card layout already reads it that
 * way). Passing it again would render the title twice.
 *
 * `nameColumn` returns the one taken out, since the caller still needs it to
 * render the title cell's contents through the `name` slot.
 */
export function toTableColumns(columns: ListColumn[]): TableColumn[] {
  const seen = new Set<string>()
  return columns.slice(1).map((column) => {
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
 * The column holding the row title, which the library's table renders itself.
 *
 * Nil: undefined when the list declares no columns at all.
 */
export function nameColumn(columns: ListColumn[]): ListColumn | undefined {
  return columns[0]
}

/** Reads a converted column's originating rela config back out of `meta`. */
export function listColumnOf(column: TableColumn): ListColumn | undefined {
  return (column.meta as { listColumn?: ListColumn } | undefined)?.listColumn
}
