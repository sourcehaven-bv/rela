/**
 * Column definitions follow the conventions the established table libraries
 * settled on, so an adopter's existing mental model carries over:
 *
 * - `key` identifies the column and names its slot (Vuetify).
 * - `field` is the accessor (AG Grid, MUI Data Grid, PrimeVue).
 * - `header` is the label (TanStack, PrimeVue).
 *
 * The accessor is deliberately separate from the rendering, as it is in
 * every one of them: sorting and export need to know a cell's value without
 * caring what it looks like. PrimeVue conflated the two and had to add
 * `sortField` and `filterField` to get back out.
 */
export interface TableColumn {
  /** Identifies the column, and names its `cell-<key>` slot. */
  key: string
  /** The column's label, shown in the header and on a stacked row. */
  header: string
  /** Fixed pixel width. The first column is flexible when this is omitted. */
  width?: number
  /**
   * Which property of the row this column reads, for the plain-text default.
   * Anything that is not a string or number needs the `cell` slot, which is
   * also the only way to render a widget, a tag or a control in a cell.
   */
  field?: string
  /**
   * Whether the header is a sort control. The table renders the indicator and
   * emits `sort-click`; the sort itself belongs to the caller, because a
   * collection of any size sorts on the server rather than in the browser.
   */
  sortable?: boolean
  /**
   * Whether the column may be hidden. Defaults to true; `false` pins it
   * visible, which is what the column holding a row's identity usually wants.
   *
   * Deliberately separate from whether it *is* hidden, which is state and
   * lives in `RlTable`'s `column-visibility` instead. The mature libraries
   * all split these two — a single boolean on the definition forces a caller
   * who wants a columns menu to mutate their own column array.
   */
  hideable?: boolean
  /**
   * Whether the column survives the compressed narrow layout, where the table
   * keeps the row on one line and drops every column that is not primary.
   * Defaults to false, so a table switching to `compact: 'compress'` starts
   * with the title alone and the caller adds back what a glance needs.
   *
   * Separate from `hideable`, which is about a column the user chose to put
   * away and applies at every width. This is about a column worth the space
   * when there is almost none: a status badge stays, a due date does not, and
   * both are still in the columns menu.
   *
   * Ignored by `compact: 'stack'`, which has room for every column because it
   * gives each one its own line.
   */
  primary?: boolean
  /**
   * Which side the cell's contents sit on. Numbers usually read better
   * trailing, so a column of them lines up on the decimal.
   */
  align?: 'start' | 'end'
  /**
   * Whether this column has anything to show for a given row. A stacked row
   * hides its empty cells, so a bare label is never left beside a blank.
   *
   * Needed because a cell rendered through a slot is opaque to the table:
   * `field` lets it check a plain value, but it cannot see inside a slot and
   * so assumes a slot-rendered cell is filled. A column that renders a widget
   * declares emptiness here instead.
   */
  isEmpty?: (item: unknown) => boolean
  /**
   * Anything the caller wants to carry per column that this library has no
   * opinion about: a permission, a provenance flag, the widget to use.
   *
   * Namespaced on purpose. Consumers adding their own top-level keys to a
   * column type is how MUI's `GridColDef` works, and it makes every new
   * property this library adds a potential collision. Reaching this bag
   * through `column.meta` cannot collide with anything.
   */
  meta?: Record<string, unknown>
}

/**
 * One column's contribution to the sort, as an ordered list: the first entry
 * is the primary key, the second breaks its ties, and so on. The header shows
 * each column's position in that order, so a three-key sort is legible from
 * the table rather than only from the query that produced it.
 */
export interface TableSort {
  /** Matches `TableColumn.key`. */
  key: string
  dir: 'asc' | 'desc'
}

/**
 * What the header knew at the moment a sort control was pressed.
 *
 * The caller owns the sort, so the header reports the press rather than
 * deciding what it means. `additive` is the only part the caller cannot work
 * out for itself: the modifier is gone by the time the event is handled.
 *
 * The conventional reading is that a plain press replaces the sort with this
 * one column and an additive press appends it as the next tie-breaker, and
 * that pressing a column already in the sort cycles it ascending, then
 * descending, then out of the sort entirely. That third step matters: without
 * it a key can never be dropped once added, except by discarding the whole
 * sort, and removing the primary key is how the second key gets promoted.
 * `dir` and `position` are passed so the caller can implement that cycle
 * without tracking the state twice.
 */
export interface SortClickEvent {
  /** The press asked to add a key rather than replace the sort. */
  additive: boolean
  /** The column's 1-based place in the sort, or 0 when it is not in it. */
  position: number
  /** The column's current direction, or undefined when it is not sorted. */
  dir?: 'asc' | 'desc'
}

/**
 * Which columns are showing, keyed by `TableColumn.key`. A column absent from
 * the record is visible, so `{}` shows everything and a caller only records
 * the exceptions.
 *
 * Held as state rather than as a flag on the column because the same record
 * serves a columns menu, a saved view and a narrow screen at once. Drive it
 * from a media query to drop columns on a phone; this library does not do
 * that for you, because it cannot see the table's real width — the table
 * sits beside a detail panel that may or may not be open, so a viewport
 * breakpoint would be measuring the wrong box. PrimeVue shipped exactly that
 * and removed it again in v4.
 */
export type ColumnVisibility = Record<string, boolean>

/**
 * What the table does once it is too narrow for its fixed columns.
 *
 * - `stack` turns each row into a card, giving every populated cell its own
 *   line under the title with its column's header as a label. Nothing is
 *   lost, and the row grows as tall as it needs to.
 * - `compress` keeps the row on one line and drops every column that is not
 *   `primary`. Nothing is labelled, because nothing needs to be: what is left
 *   is the title and at most a badge or two.
 *
 * Which one is right depends on whether the narrow table is the whole view or
 * a list beside something else. A phone showing only the table wants `stack`:
 * it is the only place the data is. A master list beside an open detail panel
 * wants `compress`: the row is an index entry, the detail is already on
 * screen, and a stack of labelled fields repeats it at three times the height.
 */
export type TableCompact = 'stack' | 'compress'
