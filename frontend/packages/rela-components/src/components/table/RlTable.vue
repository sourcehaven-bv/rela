<script setup lang="ts" generic="T extends CollectionItem">
/**
 * A sectioned list of rows, generic over the row.
 *
 * The table reads `id` and `title` off an item and nothing else. Give each
 * column its contents through the `cell-<key>` slot, or `cell` for all of them;
 * `column.field` covers only a plain string or number.
 *
 * ## There is no per-row action slot, on purpose
 *
 * Reach for selection instead: pass `selectedIds` to turn on the checkboxes
 * and put the actions in an `RlBulkActionBar`. Deleting three rows is then one
 * gesture rather than three, and the list stays readable.
 *
 * That also settles how a destructive action confirms. Checking a box, moving
 * to the bar and pressing Delete is already three deliberate acts with the
 * target in view, so a dialog on top of it confirms what the user just spent
 * three steps demonstrating; an undo toast covers the slip at no cost. A
 * per-row button needs the dialog precisely because one stray click is the
 * whole interaction.
 *
 * If a row genuinely needs its own control, give it a trailing column with
 * `align: 'end'` and `hideable: false`. Not the `name` slot: `__primary` is
 * `display: contents`, so anything after the title there sits wherever that
 * title happens to end, and a column of them comes out ragged.
 */
import type { CollectionItem, Section } from '../../types'
import { watchEffect } from 'vue'
import type {
  ColumnVisibility,
  SortClickEvent,
  TableColumn,
  TableCompact,
  TableSort,
} from './types'
import RlTableSection from './RlTableSection.vue'

const props = withDefaults(
  defineProps<{
    sections: Section<T>[]
    columns?: TableColumn[]
    showHeaderRow?: boolean
    /** Id of the item whose detail is open, marked in the list. */
    selectedId?: string
    /**
     * Id of the item the keyboard cursor is on, for a list driven by `j`/`k`
     * or the arrow keys.
     *
     * The third of three independent row states, and the reason it needs its
     * own prop: `selectedIds` is what a bulk action would apply to,
     * `selectedId` is the row whose detail is open, and this is the row the
     * next keystroke acts on. A reader can move the cursor down the list
     * while the panel stays where it is, then press Enter to bring the panel
     * to the cursor — two positions that are genuinely separate until that
     * press, so one prop cannot carry both.
     *
     * Marked with a ring rather than a fill, since all three states can land
     * on one row at once.
     *
     * The table paints the cursor and announces it; moving it is the
     * caller's, because what `j` means at the end of a section — stop, wrap,
     * or cross into the next one — depends on the list, not on the table.
     * Keep DOM focus on the grid and change this prop; the row is named
     * through `aria-activedescendant`, so a screen reader follows the cursor
     * without focus moving row to row.
     */
    cursorId?: string
    /** Rows each section shows before the rest go behind "show more". */
    pageSize?: number
    /** Label for each section's add control, such as `Add task`. */
    addLabel?: string
    /**
     * Header for the flexible first column, which holds each row's title.
     * Defaults to `Name`.
     */
    nameLabel?: string
    /**
     * The first column described as a column, when it needs to sort. Its
     * `header` replaces `nameLabel`, and its `key` is what `sort` and
     * `sort-click` carry for it.
     */
    nameColumn?: TableColumn
    /**
     * Whether each section shows its own header. A flat list is one unnamed
     * section, where a heading, a count, a collapse toggle and an "Add to"
     * button all describe a grouping the caller does not have.
     *
     * Hiding it also moves the column header to the top of the scroll area.
     * Left where it was, it would float over the first row and take its
     * clicks: the row stays visible and correctly marked up, and is
     * unpressable.
     */
    showSectionHeader?: boolean
    /**
     * Whether a section offers a way to add a row. Off hides both controls
     * at once: the "Add to" button in a section header and the one at the
     * foot of its list are the same affordance in two places.
     *
     * For a list that refuses creation, whether because of a permission or
     * because the list has no form to create with. A control that emits an
     * event nothing can act on is worse than no control. This is the same
     * prop, with the same name and default, that the boards take.
     */
    showAdd?: boolean
    /**
     * What a section says when it has no rows. Without it an empty section
     * draws its header and then nothing, which reads as a failure to load
     * rather than an empty list. Sections with rows ignore it.
     */
    emptyLabel?: string
    /** A line under `emptyLabel`, for what would put a row here. */
    emptyDescription?: string
    /**
     * Current sort, primary key first. Mark a column `sortable` to give it
     * the control; the sort itself is the caller's, since a collection of
     * any size sorts on the server.
     */
    sort?: TableSort[]
    /**
     * Ids of the checked rows. Passing this — even empty — turns on the
     * select column; leaving it off keeps the plain list.
     *
     * Separate from `selectedId`, which marks the one row whose detail is
     * open, and from `cursorId`, which marks the row the keyboard is on. A
     * table can have all three: three rows checked for a bulk action, a
     * fourth open in the panel, and the cursor resting on a fifth.
     */
    selectedIds?: Set<string> | string[]
    /**
     * Which columns are showing, keyed by `TableColumn.key`. Absent from the
     * record means visible, so `{}` shows everything.
     *
     * State rather than a flag on the column, so one record serves a columns
     * menu and a saved view at once.
     *
     * Fitting a narrow pane is not what this is for. The table reshapes its
     * rows on its own once it is too narrow for fixed columns — see `compact`
     * — measuring itself rather than the window, so a caller does not have to
     * hide columns to stay readable beside an open detail panel. Use this to
     * drop a column the user does not want to see, which is a different
     * question from whether it fits.
     */
    columnVisibility?: ColumnVisibility
    /**
     * What the table does once it is too narrow for its fixed columns.
     *
     * `stack` gives each populated cell its own labelled line under the
     * title, so the row keeps everything and grows tall. `compress` keeps the
     * row on one line and drops every column not marked `primary`.
     *
     * Defaults to `stack`, which is what the table has always done.
     *
     * Pick by what the narrow table is. A phone showing nothing else wants
     * `stack`, because the row is the only place the data is. A master list
     * beside an open detail panel wants `compress`: the row is an index entry
     * into a panel that is already showing those fields, so stacking them
     * repeats the panel at three times the height and pushes the next row off
     * screen.
     */
    compact?: TableCompact
    /**
     * Extra attributes per row, such as `data-entity-id`. The caller does not
     * render rows itself, so this is the only way to reach one: useful for a
     * test hook, and for addressing a row whose cells are all withheld and
     * which therefore renders no link to key off.
     */
    rowAttrs?: (item: T) => Record<string, unknown>
  }>(),
  {
    columns: () => [],
    showHeaderRow: true,
    cursorId: undefined,
    pageSize: 0,
    addLabel: 'Add',
    nameLabel: 'Name',
    nameColumn: undefined,
    showSectionHeader: true,
    showAdd: true,
    emptyLabel: undefined,
    emptyDescription: undefined,
    sort: () => [],
    selectedIds: undefined,
    columnVisibility: () => ({}),
    rowAttrs: undefined,
    compact: 'stack',
  },
)

/*
 * A duplicated key is silent and looks like it works: each cell still renders
 * its own column, because the slot is handed the column it is iterating. What
 * breaks is everything that looks a column up BY key — hiding one of the pair
 * hides both, and a sort indicator cannot say which of the two it means.
 *
 * Dev only. This is a mistake in how the caller built its column list, which
 * is a development-time fact rather than something to check on every render in
 * production.
 */
if (import.meta.env.DEV) {
  watchEffect(() => {
    const seen = new Set<string>()
    const duplicates = new Set<string>()
    for (const column of props.columns) {
      if (seen.has(column.key)) duplicates.add(column.key)
      seen.add(column.key)
    }
    if (duplicates.size) {
      console.warn(
        `[RlTable] Duplicate column key(s): ${[...duplicates].map((k) => `"${k}"`).join(', ')}. ` +
          'Keys identify a column for its cell slot, for hiding and for sorting, ' +
          'so two columns sharing one cannot be told apart: hiding either hides both. ' +
          'Give each column a distinct key.',
      )
    }
  })
}

const emit = defineEmits<{
  add: [section: Section<T>]
  /** A section was opened or closed. See `RlTableSection`. */
  collapse: [section: Section<T>, collapsed: boolean]
  select: [item: T]
  /**
   * A sortable column's header was pressed. The event carries the modifier
   * state and the column's current place in the sort, so the caller can
   * implement add-a-key and the asc/desc/remove cycle without tracking it
   * separately.
   */
  sortClick: [column: TableColumn, event: SortClickEvent]
  /** A row's select box was ticked or cleared. */
  toggle: [item: T]
  /** A section's select-all box was ticked or cleared. */
  toggleAll: [section: Section<T>, checked: boolean]
}>()

/*
 * Declared loosely on purpose: a per-column `cell-<key>` slot has a name only
 * the consumer knows, and a templated-key record here makes `$slots`
 * un-indexable for the forwarding below. The cell slot props are documented
 * above instead.
 */
/*
 * Slots, declared loosely on purpose. A per-column slot is named `cell-<key>`
 * for an id only the consumer knows, and a templated-key record here makes
 * `$slots` un-indexable for the forwarding this component does.
 *
 * - `cell-<key>` and `cell` receive `{ item, column }`.
 * - `meta` receives `{ item }`, for trailing content in the name cell.
 * - `bulk` receives `{ section, count }` and replaces the column headers
 *   while that section has rows checked.
 * - `name` receives `{ item, selected }` and replaces each row's primary
 *   control, for a row that navigates rather than opens a panel.
 */
defineSlots<
  {
    /** Replaces a row's primary control; put a link here for a row that navigates. */
    name?: (props: { item: T; selected: boolean }) => unknown
    /** Trailing content in the name cell. */
    meta?: (props: { item: T }) => unknown
    /** Replaces the column headers while a section has rows checked. */
    bulk?: (props: { section: Section<T>; count: number }) => unknown
    /** Fallback for every column without its own `cell-<key>`. */
    cell?: (props: { item: T; column: TableColumn }) => unknown
  } & Record<string, ((props: any) => unknown) | undefined>
>()
</script>

<template>
  <div class="rl-table">
    <RlTableSection
      v-for="section in sections"
      :key="section.id"
      :section="section"
      :columns="columns"
      :show-header-row="showHeaderRow"
      :selected-id="selectedId"
      :cursor-id="cursorId"
      :page-size="pageSize"
      :add-label="addLabel"
      :name-label="nameLabel"
      :name-column="nameColumn"
      :show-section-header="showSectionHeader"
      :show-add="showAdd"
      :empty-label="emptyLabel"
      :empty-description="emptyDescription"
      :sort="sort"
      :selected-ids="selectedIds"
      :column-visibility="columnVisibility"
      :row-attrs="rowAttrs"
      :compact="compact"
      @add="emit('add', $event)"
      @collapse="(target, collapsed) => emit('collapse', target, collapsed)"
      @select="emit('select', $event)"
      @sort-click="(column, event) => emit('sortClick', column, event)"
      @toggle="emit('toggle', $event)"
      @toggle-all="(target, checked) => emit('toggleAll', target, checked)"
    >
      <template v-for="(_, name) in $slots" #[name]="slotProps">
        <slot :name="name" v-bind="slotProps ?? {}" />
      </template>
    </RlTableSection>
  </div>
</template>

<style scoped>
.rl-table {
  flex: 1;
  min-height: 0;
  padding: var(--rl-space-4) 0;
  overflow-y: auto;
  /*
   * The query container for the stacked layout in `RlTableRow` and
   * `RlTableSection`. Those rules ask how wide the table actually is, which
   * is the question a media query cannot answer: the table sits beside a
   * detail panel that may or may not be open, so a wide viewport says
   * nothing about the space a row has. A 520px pane in a 1500px window is
   * the case that matters, and to a media query it reads as a desktop.
   *
   * Named, so a row asks this container rather than whichever ancestor a
   * consuming app happened to make a container for its own layout.
   */
  container: rl-table / inline-size;
}
</style>
