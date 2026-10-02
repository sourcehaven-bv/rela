<script setup lang="ts" generic="T extends CollectionItem">
import { computed, ref, useId } from 'vue'
import type { CollectionItem, Section } from '../../types'
import type {
  ColumnVisibility,
  SortClickEvent,
  TableColumn,
  TableCompact,
  TableSort,
} from './types'
import RlIcon from '../common/RlIcon.vue'
import RlCheckbox from '../form/RlCheckbox.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlAddButton from '../common/RlAddButton.vue'
import RlEmptyState from '../feedback/RlEmptyState.vue'
import RlDisclosure from '../common/RlDisclosure.vue'
import RlSectionHeading from '../layout/RlSectionHeading.vue'
import RlTableRow from './RlTableRow.vue'

const props = withDefaults(
  defineProps<{
    section: Section<T>
    columns?: TableColumn[]
    showHeaderRow?: boolean
    addLabel?: string
    /** Id of the item whose detail is open, marked in the list. */
    selectedId?: string
    /** Id of the item the keyboard cursor is on. See `RlTable`. */
    cursorId?: string
    /**
     * Rows to show before the rest are held behind a "show more" control.
     * Zero shows every row.
     *
     * A backlog runs to hundreds of rows, and a section that long buries the
     * sections under it: reaching the next heading means scrolling past every
     * row of this one. Capping the section keeps all the headings within
     * reach, and the count on the control says what is being held back.
     */
    pageSize?: number
    /** Header for the flexible first column. */
    nameLabel?: string
    /**
     * The first column as a column, when it needs to sort. Its `header`
     * overrides `nameLabel`; `key` is what `sort` and `sort-click` carry.
     */
    nameColumn?: TableColumn
    /**
     * Whether the section's own header shows. A flat list is one unnamed
     * section, where a heading, a count, a collapse toggle and an "Add to"
     * button all describe a grouping the caller does not have.
     */
    showSectionHeader?: boolean
    /**
     * Whether the section offers a way to add a row. Off hides both controls
     * at once: the "Add to" button in the header and the one at the foot of
     * the list are the same affordance in two places, so a list that cannot
     * take a new row must not show either.
     *
     * For a list that refuses creation, whether because of a permission or
     * because the list has no form to create with. A control that emits an
     * event nothing can act on is worse than no control.
     */
    showAdd?: boolean
    /**
     * What to say when the section has no rows. Without it an empty section
     * draws its header and then nothing, which reads as a list that failed
     * to load rather than one that is genuinely empty.
     *
     * A section with rows ignores it, so it can be set once for every
     * section rather than conditionally.
     */
    emptyLabel?: string
    /** A line under `emptyLabel`, for what would put a row here. */
    emptyDescription?: string
    /** Current sort, primary key first. */
    sort?: TableSort[]
    /** Ids of the checked rows. Absent means the table has no selection. */
    selectedIds?: Set<string> | string[]
    /** Which columns are showing. Absent from the record means visible. */
    columnVisibility?: ColumnVisibility
    /** What the row does once the table is too narrow. See `RlTable`. */
    compact?: TableCompact
    /**
     * Extra attributes per row, such as a `data-` hook for tests. The caller
     * never renders the row itself, so this is the only place to reach it.
     */
    rowAttrs?: (item: T) => Record<string, unknown>
  }>(),
  {
    columns: () => [],
    showHeaderRow: true,
    addLabel: 'Add',
    cursorId: undefined,
    pageSize: 0,
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

const emit = defineEmits<{
  add: [section: Section<T>]
  /**
   * The header's disclosure opened or closed the section. The section keeps
   * its own open state, so this is a report rather than a request: a caller
   * that wants the choice to outlive the page stores it and passes it back
   * as `section.collapsed` next time.
   */
  collapse: [section: Section<T>, collapsed: boolean]
  select: [item: T]
  sortClick: [column: TableColumn, event: SortClickEvent]
  toggle: [item: T]
  toggleAll: [section: Section<T>, checked: boolean]
}>()

/*
 * A column is showing unless the record says otherwise, so `{}` shows
 * everything and a caller records only the exceptions. `hideable: false`
 * outranks the record, so a column carrying the row's identity cannot be
 * hidden by a stale saved view.
 */
const visibleColumns = computed(() =>
  props.columns.filter(
    (column) => column.hideable === false || props.columnVisibility[column.key] !== false,
  ),
)

/*
 * The narrowest the name column may get before the table scrolls sideways.
 * The name cell is the flexible one, so without a floor enough fixed columns
 * squeeze it to nothing and its header overlaps the next one.
 */
const NAME_MIN_WIDTH = 240

/* The row's full width: the name's floor plus every fixed column. */
const minWidth = computed(
  () => NAME_MIN_WIDTH + visibleColumns.value.reduce((sum, column) => sum + (column.width ?? 150), 0),
)

/*
 * Collapsing is the header's control, so a section without one is always
 * open: otherwise a `collapsed` section would render nothing and offer no way
 * back.
 */
const open = ref(!props.section.collapsed)

function toggleOpen() {
  open.value = !open.value
  emit('collapse', props.section, !open.value)
}
const expanded = computed(() => open.value || !props.showSectionHeader)
const count = computed(() => props.section.count ?? props.section.items.length)

/** How many rows the section is currently willing to show. */
const shown = ref(props.pageSize || Number.POSITIVE_INFINITY)

const visibleItems = computed(() => props.section.items.slice(0, shown.value))
const remaining = computed(() => props.section.items.length - visibleItems.value.length)

/*
 * One more page per press rather than all of it, so the control cannot drop
 * a thousand rows into the page in a single step.
 */
function showMore() {
  shown.value += props.pageSize || props.section.items.length
}

/*
 * Selection is on only when the caller passes the set. A table without it is
 * the plain list it has always been, with no checkbox column.
 */
const selectable = computed(() => props.selectedIds !== undefined)

const uid = useId()

/*
 * A DOM id per row, so the grid can name the cursor row through
 * `aria-activedescendant`.
 *
 * Built from the section's own uid rather than from the item id alone: item
 * ids come from the caller's data and two tables on one screen — a list
 * beside a picker, say — would otherwise mint the same DOM id twice.
 */
const rowDomId = (itemId: string) => `${uid}-row-${itemId}`

/*
 * The cursor is only announced while it is on a row THIS section is
 * showing. A section that has the cursor id in its data but is holding that
 * row behind "show more" must not claim it, or the grid would point
 * `aria-activedescendant` at an element that is not in the DOM.
 */
const cursorHere = computed(() =>
  props.cursorId !== undefined && visibleItems.value.some((item) => item.id === props.cursorId)
    ? props.cursorId
    : undefined,
)

const selected = computed(() =>
  Array.isArray(props.selectedIds) ? new Set(props.selectedIds) : props.selectedIds ?? new Set<string>(),
)

/** Checked rows in this section. Each section counts its own. */
const selectedHere = computed(
  () => props.section.items.filter((item) => selected.value.has(item.id)).length,
)

const allSelected = computed(
  () => selectedHere.value > 0 && selectedHere.value === props.section.items.length,
)

/*
 * Part of a section reads as indeterminate rather than unchecked, so the box
 * says "some of these" instead of inviting a click that would clear them.
 */
const someSelected = computed(
  () => selectedHere.value > 0 && !allSelected.value,
)

/*
 * The modifier travels with the press because only the header knows it, and
 * the caller cannot recover it afterwards. Which gesture means "add a key"
 * rather than "replace the sort" is the caller's decision — the header does
 * not interpret it, it only reports it.
 */
function onSortClick(column: TableColumn, event: MouseEvent | KeyboardEvent) {
  emit('sortClick', column, {
    additive: event.shiftKey,
    position: sortPosition(column),
    dir: sortDir(column),
  })
}

/*
 * The first column, described the same way as the rest so it can carry a sort
 * control. Falls back to a plain label, which is the column as it was before
 * anything needed to sort by it.
 */
const nameCol = computed<TableColumn>(
  () => props.nameColumn ?? { key: '__name', header: props.nameLabel },
)

/** Where a column sits in the sort, 1-based, or zero when it is not in it. */
function sortPosition(column: TableColumn) {
  return props.sort.findIndex((entry) => entry.key === column.key) + 1
}

function sortDir(column: TableColumn) {
  return props.sort.find((entry) => entry.key === column.key)?.dir
}

/*
 * Spelled out for the screen reader, which otherwise gets an arrow glyph and
 * a bare number. `aria-sort` carries the direction on its own, so this says
 * what a press would do and where the column sits in a multi-key sort.
 */
function sortHint(column: TableColumn) {
  const dir = sortDir(column)
  const position = sortPosition(column)
  if (!dir) return `${column.header}, sort by this column`
  const rank = props.sort.length > 1 ? `, sort key ${position} of ${props.sort.length}` : ''
  return `${column.header}, sorted ${dir === 'asc' ? 'ascending' : 'descending'}${rank}`
}
</script>

<template>
  <section
    class="rl-table-section"
    :class="{ 'rl-table-section--headerless': !showSectionHeader }"
    :style="{ '--rl-table-min-width': `${minWidth}px` }"
  >
    <header v-if="showSectionHeader" class="rl-table-section__header">
      <RlDisclosure :open="open" :label="section.title" @toggle="toggleOpen" />

      <RlSectionHeading
        :title="section.title"
        :color="section.color"
        :count="count"
        size="sm"
      />

      <RlIconButton
        v-if="showAdd"
        icon="plus"
        :label="`${addLabel} to ${section.title}`"
        :size="14"
        @click="emit('add', section)"
      />
    </header>

    <template v-if="expanded">
      <!--
        role="grid" gives the row/gridcell descendants the required parent.
        A native <table> would be wrong here: cells are widget-bearing and
        the first column is a flexible button, not tabular data.
      -->
      <div
        class="rl-table-section__grid"
        role="grid"
        :aria-label="section.title"
        :aria-activedescendant="cursorHere ? rowDomId(cursorHere) : undefined"
      >
        <div v-if="showHeaderRow" class="rl-table-section__columns-group" role="rowgroup">
          <!--
            While rows are checked the bulk bar takes the header's place
            rather than sitting above it: the actions apply to the selection,
            and the column labels describe rows the user has stopped reading
            as columns. The row keeps its `role="row"` either way, so the
            grid is never left with a rowgroup holding something else.
          -->
          <div
            v-if="selectable && selectedHere > 0 && $slots.bulk"
            class="rl-table-section__columns rl-table-section__columns--bulk"
            role="row"
          >
            <span class="rl-table-section__columns-name" role="columnheader">
              <RlCheckbox
                :model-value="allSelected"
                :indeterminate="someSelected"
                :label="`Select all in ${section.title}`"
                label-hidden
                @update:model-value="emit('toggleAll', section, $event)"
              />
              <slot name="bulk" :section="section" :count="selectedHere" />
            </span>
          </div>

          <div v-else class="rl-table-section__columns" role="row">
            <span
              class="rl-table-section__columns-name"
              role="columnheader"
              :aria-sort="
                sortDir(nameCol)
                  ? sortDir(nameCol) === 'asc'
                    ? 'ascending'
                    : 'descending'
                  : nameCol.sortable
                    ? 'none'
                    : undefined
              "
            >
              <RlCheckbox
                v-if="selectable"
                :model-value="allSelected"
                :indeterminate="someSelected"
                :label="`Select all in ${section.title}`"
                label-hidden
                @update:model-value="emit('toggleAll', section, $event)"
              />
              <!--
                Sorting a list by its title is ordinary, so the first column
                takes the same control as any other once it is given a key to
                sort by. Without `nameColumn` it stays the plain label it was.
              -->
              <button
                v-if="nameCol.sortable"
                type="button"
                class="rl-table-section__sort rl-table-section__sort--name"
                :aria-label="sortHint(nameCol)"
                @click="onSortClick(nameCol, $event)"
              >
                <span class="rl-table-section__sort-label">{{ nameCol.header }}</span>
                <RlIcon
                  v-if="sortDir(nameCol)"
                  :name="sortDir(nameCol) === 'asc' ? 'chevron-up' : 'chevron-down'"
                  :size="12"
                />
                <span
                  v-if="sortDir(nameCol) && sort.length > 1"
                  class="rl-table-section__sort-rank"
                  aria-hidden="true"
                >
                  {{ sortPosition(nameCol) }}
                </span>
              </button>
              <template v-else>{{ nameCol.header }}</template>
            </span>
            <!--
              `aria-sort` belongs on the header cell and the control inside it
              is what takes the press, so a sortable column is a cell wrapping
              a button rather than a button acting as the cell.
            -->
            <span
              v-for="column in visibleColumns"
              :key="column.key"
              class="rl-table-section__columns-cell"
              :class="{ 'rl-table-section__columns-cell--end': column.align === 'end' }"
              role="columnheader"
              :aria-sort="
                sortDir(column)
                  ? sortDir(column) === 'asc'
                    ? 'ascending'
                    : 'descending'
                  : column.sortable
                    ? 'none'
                    : undefined
              "
              :style="{ width: `${column.width ?? 150}px` }"
            >
              <button
                v-if="column.sortable"
                type="button"
                class="rl-table-section__sort"
                :aria-label="sortHint(column)"
                @click="onSortClick(column, $event)"
              >
                <span class="rl-table-section__sort-label">{{ column.header }}</span>
                <RlIcon
                  v-if="sortDir(column)"
                  :name="sortDir(column) === 'asc' ? 'chevron-up' : 'chevron-down'"
                  :size="12"
                />
                <!--
                  Only once more than one column is in play: a lone "1" beside
                  the only sorted column says nothing.
                -->
                <span
                  v-if="sortDir(column) && sort.length > 1"
                  class="rl-table-section__sort-rank"
                  aria-hidden="true"
                >
                  {{ sortPosition(column) }}
                </span>
              </button>
              <template v-else>{{ column.header }}</template>
            </span>
          </div>
        </div>

        <div role="rowgroup">
          <RlTableRow
            v-for="item in visibleItems"
            :key="item.id"
            v-bind="rowAttrs?.(item)"
            :item="item"
            :columns="visibleColumns"
            :row-id="rowDomId(item.id)"
            :selected="item.id === selectedId"
            :selectable="selectable"
            :checked="selected.has(item.id)"
            :cursor="cursorId !== undefined && item.id === cursorId"
            :compact="compact"
            @click="emit('select', $event)"
            @toggle="emit('toggle', $event)"
          >
            <template v-for="(_, name) in $slots" #[name]="slotProps">
              <slot :name="name" v-bind="slotProps ?? {}" />
            </template>
          </RlTableRow>
        </div>
      </div>

      <!--
        Inside the expanded block, so collapsing an empty section hides the
        message with everything else: a collapsed section is already saying
        there is nothing to see.
      -->
      <RlEmptyState
        v-if="emptyLabel && !section.items.length"
        :title="emptyLabel"
        :description="emptyDescription"
        icon="inbox"
        size="sm"
        class="rl-table-section__empty"
      />

      <button
        v-if="remaining > 0"
        type="button"
        class="rl-table-section__more"
        @click="showMore"
      >
        Show {{ Math.min(remaining, pageSize || remaining) }} more
        <span class="rl-table-section__more-rest">of {{ remaining }} remaining</span>
      </button>

      <RlAddButton
        v-if="showAdd"
        class="rl-table-section__add"
        :label="addLabel"
        block
        @click="emit('add', section)"
      />
    </template>
  </section>
</template>

<style scoped>
.rl-table-section + .rl-table-section { margin-top: var(--rl-space-6); }

/*
 * Both headers stick, because a long section scrolls for many screens and a
 * row read without them is a row you cannot interpret: the section header
 * says which bucket you are in, the column header says what each cell means.
 *
 * The section header sits at the top of the band and the column header
 * directly beneath it, so the two stack rather than overlap. The section
 * header is opaque and outranks the column header, so the outgoing section's
 * columns are covered as the next section arrives.
 */
.rl-table-section__header {
  position: sticky;
  top: 0;
  z-index: var(--rl-z-sticky-raised);
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  /* Box-sized to the offset its column header parks at, so the two agree. */
  box-sizing: border-box;
  min-height: var(--rl-table-header-offset);
  padding: var(--rl-space-2) var(--rl-space-4);
  background: var(--rl-color-bg);
  /*
   * The table is padded, so without this the outgoing section's last row
   * shows through the gap above the header as it parks.
   */
  box-shadow: 0 calc(-1 * var(--rl-space-4)) 0 var(--rl-color-bg);
}

/*
 * The rowgroup is required by the grid role but must not become the header's
 * containing block: it is only as tall as the header, which would leave the
 * header nowhere to travel and so no way to stick. `display: contents` keeps
 * the element in the accessibility tree while taking it out of the layout, so
 * the header sticks against the full-height grid instead.
 */
/*
 * Wider than the pane when the columns need it, so the table scrolls sideways
 * rather than crushing the name. The narrow layouts drop the fixed widths and
 * reset this.
 */
.rl-table-section { min-width: var(--rl-table-min-width); }

.rl-table-section__columns-group { display: contents; }

.rl-table-section__columns {
  position: sticky;
  top: var(--rl-table-header-offset);
  z-index: var(--rl-z-sticky);
  display: flex;
  align-items: center;
  background: var(--rl-color-bg-sunken);
  border-block: 1px solid var(--rl-color-border);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}

/*
 * With no section header there is nothing above the column header to clear.
 * Leaving the offset would float it down over the first row: visible and
 * correctly marked up, but swallowing the press meant for the row beneath.
 *
 * Rebinding the token rather than overriding `top`, so the token keeps
 * reporting the offset actually in force. An override leaves the two
 * disagreeing, and the token is the more obvious thing to read: anything
 * measuring it would see 38px on a section whose header sits at 0.
 */
.rl-table-section--headerless { --rl-table-header-offset: 0px; }

.rl-table-section__columns-name {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  flex: 1;
  min-width: 0;
  padding: var(--rl-space-1) var(--rl-space-4);
}

/*
 * The bulk bar replaces the column labels, so it carries the row's full
 * height and an accent tint: the header changing colour is what says the
 * table is in a selection mode rather than its usual browsing one.
 */
.rl-table-section__columns--bulk {
  background: var(--rl-color-bg-selected);
  color: var(--rl-color-text);
}

.rl-table-section__columns--bulk .rl-table-section__columns-name {
  gap: var(--rl-space-3);
  padding-block: var(--rl-space-2);
}

/*
 * The whole header cell is the target, so the press lands anywhere on the
 * label rather than only on the glyph.
 */
.rl-table-section__sort {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  /*
   * Fills the header cell rather than sitting inside its padding, so the
   * press lands anywhere on the cell and the label keeps the full column
   * width before it truncates. `width: 100%` would measure the content box
   * and so fall short by the padding the negative margins cancel.
   */
  position: absolute;
  inset: 0;
  padding: var(--rl-space-1) var(--rl-space-4);
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.rl-table-section__sort:hover { color: var(--rl-color-text); }

/*
 * The name cell is flexible and shares its row with a select box, so its
 * control sits in the flow rather than filling the cell the way a fixed-width
 * column's does.
 */
.rl-table-section__sort--name {
  position: static;
  width: auto;
  margin: 0;
  padding: 0;
}

/* The header sits on the same side as the cells it labels. */
.rl-table-section__columns-cell--end { justify-content: flex-end; }
.rl-table-section__columns-cell--end .rl-table-section__sort { justify-content: flex-end; }

.rl-table-section__sort-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/*
 * The column's place in a multi-key sort. Sits tight against the arrow it
 * qualifies, and is small enough to read as an annotation on it rather than
 * as part of the label.
 */
.rl-table-section__sort-rank {
  flex: none;
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-accent);
}

.rl-table-section__columns-cell {
  position: relative;
  flex: none;
  align-self: stretch;
  display: flex;
  align-items: center;
  padding: var(--rl-space-1) var(--rl-space-4);
  border-left: 1px solid var(--rl-color-border);
}

/*
 * Indented to the rows it stands in for, so the message sits where the first
 * row would rather than centred in a band of its own.
 */
.rl-table-section__empty {
  padding: var(--rl-space-6) var(--rl-page-gutter-right) var(--rl-space-6) var(--rl-page-gutter-left);
}

.rl-table-section__add {
  border-radius: 0;
  padding-inline: var(--rl-space-4);
}

.rl-table-section__more {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 100%;
  min-height: var(--rl-row-height);
  padding: var(--rl-space-2) var(--rl-space-4);
  border: none;
  border-bottom: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
  font: inherit;
  font-size: var(--rl-font-size-sm);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
  text-align: left;
  cursor: pointer;
}

.rl-table-section__more:hover { background: var(--rl-color-bg-hover); }

.rl-table-section__more-rest {
  font-weight: var(--rl-font-weight-normal);
  color: var(--rl-color-text-subtle);
}

/*
 * Matches the narrow rows in `RlTableRow`, and asks the same container: the
 * headers and the rows they label have to change shape together, or the
 * labels describe a layout that is no longer there.
 *
 * Both narrow modes want the same thing here, so this is not split by mode.
 * The column header goes either way: `stack` has no columns left to label,
 * and `compress` still has cells but no longer lays them out at the widths
 * the header was aligned to. The gutter is about the table being narrow
 * rather than about what the rows chose to do with it.
 */
@container rl-table (max-width: 767px) {
  /*
   * The compact gutter, matching the rows. Set on the section so the header
   * and the add button below it both inherit one value: the three line up
   * down the left edge, and they cannot drift apart by being given the gutter
   * separately. `RlTableRow` explains why the derived tokens are the ones set
   * here.
   */
  .rl-table-section {
    min-width: 0;
    --rl-page-gutter-left: calc(var(--rl-space-4) + var(--rl-safe-inset-left));
    --rl-page-gutter-right: calc(var(--rl-space-4) + var(--rl-safe-inset-right));
  }

  .rl-table-section__header { padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right); }

  /*
   * The labels go; the bar that replaces them stays.
   *
   * The two share a class because the bulk bar takes the header's place in
   * the grid, but only one of them is a label for columns that are no longer
   * laid out. The bar is the only place the actions for a selection live, so
   * hiding it here would let a reader check a row on a narrow table and find
   * nothing to do with it.
   */
  .rl-table-section__columns:not(.rl-table-section__columns--bulk) { display: none; }

  /*
   * With no column header beneath the section header, the bar parks directly
   * under it. `--rl-table-header-offset` is already what the section header
   * occupies, so the bar keeps using it and needs no separate value.
   */
  .rl-table-section__columns--bulk {
    padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right);
  }

  /*
   * The bar holds a count and a row of buttons, which do not fit one narrow
   * line. Wrapping keeps every action reachable instead of pushing the last
   * one past the edge, where the header's own `overflow` would clip it.
   */
  .rl-table-section__columns--bulk .rl-table-section__columns-name {
    flex-wrap: wrap;
    gap: var(--rl-space-2);
    padding-inline: 0;
  }

  .rl-table-section__add { padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right); }
}
</style>
