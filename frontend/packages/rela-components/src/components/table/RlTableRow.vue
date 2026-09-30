<script setup lang="ts" generic="T extends CollectionItem">
/**
 * One row of the table: the title as the row's own button, and a cell per
 * column.
 *
 * The row reads only `id` and `title` off the item. A cell's contents are the
 * consumer's, through the per-column `cell-<key>` slot or the `cell` fallback;
 * `column.field` is a convenience for a plain string or number, not a way to
 * render anything richer.
 */
import type { CollectionItem } from '../../types'
import type { TableColumn, TableCompact } from './types'
import RlCheckbox from '../form/RlCheckbox.vue'

const props = withDefaults(
  defineProps<{
    item: T
    columns: TableColumn[]
    /** The row whose detail is open. Distinct from `checked` and `cursor`. */
    selected?: boolean
    /** Whether the row carries a select box at all. */
    selectable?: boolean
    /** Whether this row is in the selection. */
    checked?: boolean
    /**
     * Whether the keyboard cursor is on this row. Distinct from `selected`:
     * see the note on `cursorId` in `RlTable`.
     *
     * Marked with a ring rather than a fill, because the cursor can sit on a
     * row that is also open or checked and all three have to stay legible at
     * once.
     */
    cursor?: boolean
    /**
     * What the row does once the table is too narrow for its fixed columns.
     * `stack` gives every populated cell its own labelled line; `compress`
     * stays on one line and keeps only the `primary` columns. See `RlTable`.
     */
    compact?: TableCompact
    /**
     * DOM id for the row element, so the grid can point
     * `aria-activedescendant` at it. Set by the section; a caller passing
     * `cursorId` never supplies this.
     */
    rowId?: string
  }>(),
  {
    selected: false,
    selectable: false,
    checked: false,
    cursor: false,
    rowId: undefined,
    compact: 'stack',
  },
)
const emit = defineEmits<{ click: [item: T]; toggle: [item: T] }>()

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
 * - `name` receives `{ item, selected }` and replaces the row's primary
 *   control. Put a link here for a row that navigates; the row stretches
 *   whatever element it gets across the full row, so no class is needed.
 */
defineSlots<Record<string, (props: { item: T; column?: TableColumn; selected?: boolean }) => unknown>>()

/** The plain-text default for a column that names a field. */
function fieldValue(column: TableColumn): unknown {
  if (!column.field) return undefined
  return (props.item as Record<string, unknown>)[column.field]
}

/**
 * Whether a column has anything to show for this item. Stacked rows hide
 * empty cells entirely, so a bare label is never left behind.
 *
 * A column rendered through a slot counts as filled unless the column says
 * otherwise through `isEmpty`: the row cannot see what the slot put there,
 * and guessing wrong would hide real content.
 */
function hasValue(column: TableColumn) {
  // The column's own answer wins: only it can see inside its slot.
  if (column.isEmpty) return !column.isEmpty(props.item)
  if (!column.field) return true
  const value = fieldValue(column)
  return Array.isArray(value) ? value.length > 0 : value !== undefined && value !== null && value !== ''
}

/** Only a primitive renders as text; anything else needs the slot. */
function asText(value: unknown): string | undefined {
  return typeof value === 'string' || typeof value === 'number' ? String(value) : undefined
}
</script>

<template>
  <div
    :id="rowId"
    class="rl-table-row"
    :class="[
      `rl-table-row--${compact}`,
      {
        'rl-table-row--selected': selected,
        'rl-table-row--checked': checked,
        'rl-table-row--cursor': cursor,
      },
    ]"
    role="row"
    :aria-selected="selectable ? checked : undefined"
  >
    <span class="rl-table-row__name" role="gridcell">
      <!--
        Named for the row it checks, so a column of boxes is not a column of
        identical "Select" controls to a screen reader.
      -->
      <RlCheckbox
        v-if="selectable"
        class="rl-table-row__check"
        :model-value="checked"
        :label="`Select ${item.title}`"
        label-hidden
        @update:model-value="emit('toggle', item)"
      />
      <!--
        The row's primary control. A row that navigates should be a real
        link, so the slot takes whatever element the caller needs; the row
        stretches it across the row either way, so the caller never has to
        know about the hit area.
      -->
      <span class="rl-table-row__primary">
        <slot name="name" :item="item" :selected="selected">
          <!-- Interaction lives on a real button; the row-wide hit area is CSS. -->
          <button
            type="button"
            :aria-current="selected ? 'true' : undefined"
            @click="emit('click', item)"
          >
            {{ item.title }}
          </button>
        </slot>
      </span>
      <span v-if="$slots.meta" class="rl-table-row__meta">
        <slot name="meta" :item="item" />
      </span>
    </span>

    <span
      v-for="column in columns"
      :key="column.key"
      class="rl-table-row__cell"
      :class="{
        'rl-table-row__cell--empty': !hasValue(column),
        'rl-table-row__cell--end': column.align === 'end',
        'rl-table-row__cell--secondary': !column.primary,
      }"
      role="gridcell"
      :style="{ '--rl-cell-width': `${column.width ?? 150}px` }"
    >
      <!-- Visible only once the row stacks, where the header is out of view. -->
      <span class="rl-table-row__cell-label" aria-hidden="true">{{ column.header }}</span>
      <slot :name="`cell-${column.key}`" :item="item" :column="column">
        <slot name="cell" :item="item" :column="column">
          <span v-if="asText(fieldValue(column)) !== undefined" class="rl-table-row__cell-text">
            {{ asText(fieldValue(column)) }}
          </span>
        </slot>
      </slot>
    </span>
  </div>
</template>

<style scoped>
.rl-table-row {
  position: relative;
  /*
   * Below the sticky headers. The row is positioned for its selection accent,
   * and a positioned element with `z-index: auto` paints above an earlier
   * sibling that has one, so rows would otherwise scroll over the header that
   * is meant to cover them.
   */
  z-index: 0;
  display: flex;
  align-items: center;
  min-height: var(--rl-row-height);
  border-bottom: 1px solid var(--rl-color-border);

  /*
   * Skip the layout and paint work for rows scrolled out of view. A workspace
   * list runs to thousands of rows, and every one of them otherwise costs a
   * full layout on the first paint even though about thirty are visible.
   *
   * Not a virtual list, deliberately. Windowing means the offscreen rows are
   * not in the DOM at all, which breaks the things that make a list usable: the
   * browser's own find-in-page, Ctrl+A, and a screen reader's ability to report
   * how many rows there are. This keeps every row real and only defers the
   * rendering, so all of that keeps working and the cost is paid on scroll.
   *
   * `contain-intrinsic-size` is what makes it safe: without it a skipped row
   * has zero height, so the scrollbar is wrong and jumps as rows render. The
   * height is the row's own minimum, which is exact for a row on one line and
   * an underestimate for a stacked one — the `auto` keyword then remembers the
   * real height once a row has been rendered, so the estimate only matters
   * before a row has been seen.
   */
  content-visibility: auto;
  contain-intrinsic-size: auto var(--rl-row-height);
}
.rl-table-row:hover { background: var(--rl-color-bg-sunken); }

/*
 * The open task is marked in the list so the detail panel has a visible
 * origin. The accent sits inside the row rather than on its border, so the
 * row height does not shift when selection moves.
 */
.rl-table-row--selected,
.rl-table-row--selected:hover { background: var(--rl-color-bg-selected); }

/*
 * A checked row is tinted but carries no accent edge: the edge means "this is
 * the row you are looking at", and a bulk selection has no such row.
 */
.rl-table-row--checked,
.rl-table-row--checked:hover { background: var(--rl-color-bg-selected); }

.rl-table-row--selected::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
  background: var(--rl-color-accent);
}

.rl-table-row:has(.rl-table-row__primary > *:focus-visible) {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

/*
 * The keyboard cursor: a ring, not a fill.
 *
 * All three row states can land on one row at once — the cursor can rest on
 * the open row while that row is also checked — so the cursor cannot use the
 * background, which the other two already spend. An inset outline reads on
 * top of either fill and leaves the accent edge visible beside it.
 *
 * `outline-offset` is negative so the ring sits inside the row: an outset
 * ring would be clipped by the scroll container on the first and last row.
 */
.rl-table-row--cursor {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-table-row__name {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  flex: 1;
  min-width: 0;
  padding: var(--rl-space-2) var(--rl-space-4);
}

/*
 * The row's primary control, whatever element it turned out to be: the
 * default button, or a link the caller put in the `name` slot.
 *
 * Styled by descendant rather than by a class the caller must apply, so a
 * consumer writes a plain `<RouterLink>` and gets the row-wide hit area, the
 * truncation and the focus ring without knowing this contract exists. `:deep`
 * because the slot's content belongs to the caller's scope, not this one.
 */
.rl-table-row__primary { display: contents; }

.rl-table-row__primary :deep(> *) {
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/*
 * Stretched across the row, so a click anywhere on it opens the row. The
 * row is the positioning context; cells after this one sit above the overlay
 * and so keep their own clicks.
 */
.rl-table-row__primary :deep(> *)::after {
  content: '';
  position: absolute;
  inset: 0;
}

.rl-table-row__primary :deep(> *:focus-visible) { outline: none; }

/*
 * Above the row-wide hit area, so ticking the box selects the row instead of
 * opening it. The box precedes the button in the DOM, so unlike the cells
 * after it, `position` alone would still leave it under the stretched
 * `::after`: it needs to be lifted explicitly.
 */
.rl-table-row__check {
  position: relative;
  z-index: 1;
  flex: none;
}

/* Numbers read better trailing, so a column of them lines up on the right. */
.rl-table-row__cell--end { justify-content: flex-end; }

.rl-table-row__meta {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  /* Sit above the row-wide hit area so counts stay readable. */
  position: relative;
}

/*
 * Above the row-wide hit area, so a cell that holds its own control — an
 * inline-edit trigger, say — receives the click rather than the row's open
 * button that is stretched across everything.
 */
.rl-table-row__cell {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  flex: none;
  align-self: stretch;
  box-sizing: border-box;
  width: var(--rl-cell-width);
  /*
   * A fixed width alone does not contain the contents: flex children take
   * their own intrinsic size and push past the edge, so a cell with several
   * tags spills into the column beside it. The min-width lets the cell
   * actually shrink its children, and the clip keeps anything still too wide
   * inside its own column instead of painting over the next one.
   */
  min-width: 0;
  overflow: hidden;
  padding: var(--rl-space-2) var(--rl-space-4);
  border-left: 1px solid var(--rl-color-border);
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text-muted);
}

/* Plain text cells ellipsize rather than cut a glyph in half. */
.rl-table-row__cell-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-table-row__cell-label { display: none; }

/*
 * Both narrow layouts are keyed to the table's own width rather than the
 * viewport's, because those two come apart exactly where this matters. The
 * row's space is the content pane, and a detail panel opening halves it
 * without the window changing size at all: a 520px pane in a 1500px window
 * reads as a desktop to a media query, so the fixed columns stay and the name
 * column, the only child that can give, absorbs the whole shortfall and
 * collapses to nothing. The row keeps its data and loses its title, which is
 * the one thing it cannot lose.
 *
 * What the row does about it is the caller's choice, because the two answers
 * suit different places. `stack` is below; `compress` follows it.
 */

/*
 * Stack: the row becomes a card. Fixed column widths cannot fit a narrow row,
 * so each populated cell takes its own line and shows its own label. Nothing
 * is dropped, and the row grows as tall as its data needs.
 */
@container rl-table (max-width: 767px) {
  .rl-table-row--stack {
    flex-direction: column;
    align-items: stretch;
    gap: var(--rl-space-1);
    padding: var(--rl-space-3) 0;
    /*
     * The compact gutter, on the same threshold as the stacking rather than
     * on the viewport's. `tokens.css` switches `--rl-page-gutter` at 767px of
     * window, which stopped meaning the same thing as this rule the moment
     * the stacking started measuring the table: with a sidebar the table is
     * always the narrower of the two, so a stacked card would otherwise keep
     * the 32px desktop gutter across every width where the window is wide and
     * the pane is not.
     *
     * The derived tokens are set rather than `--rl-page-gutter` itself,
     * because `tokens.css` resolves those two on `:root`; redefining the base
     * here would leave the pair already computed from the old value. The safe
     * insets are added the same way it adds them, so a rotated phone still
     * keeps the notch out of the text.
     */
    --rl-page-gutter-left: calc(var(--rl-space-4) + var(--rl-safe-inset-left));
    --rl-page-gutter-right: calc(var(--rl-space-4) + var(--rl-safe-inset-right));
  }

  .rl-table-row--stack .rl-table-row__name {
    padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right);
  }

  .rl-table-row--stack .rl-table-row__cell {
    width: auto;
    padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right);
    border-left: none;
    font-size: var(--rl-font-size-sm);
  }

  /* Cells with no value would otherwise leave stray labels behind. */
  .rl-table-row--stack .rl-table-row__cell--empty { display: none; }

  /*
   * A stacked cell is a label-and-value pair reading left to right, so the
   * column's own alignment no longer applies: trailing the value would put
   * it against the far edge, away from the label naming it.
   */
  .rl-table-row--stack .rl-table-row__cell--end { justify-content: flex-start; }

  .rl-table-row--stack .rl-table-row__cell-label {
    display: block;
    min-width: 72px;
    color: var(--rl-color-text-subtle);
  }
}

/*
 * Compress: the row stays one line and drops every column not marked
 * `primary`. For a master list beside an open detail panel, where the row is
 * an index entry rather than the record itself: the fields a stacked card
 * would spell out are already on screen in the panel, so spelling them out
 * again triples the row height and pushes the next row out of view.
 *
 * `display: none` rather than `visibility: hidden`, so a dropped cell leaves
 * no gap and is not read out. The cell is gone from this width only and comes
 * back whole when the table widens.
 */
@container rl-table (max-width: 767px) {
  .rl-table-row--compress {
    /*
     * The same compact gutter the stacked row takes, and set here for the
     * same reason: `tokens.css` switches `--rl-page-gutter` on the window,
     * which says nothing about the pane this row is actually in. See the
     * stacked rule above for why the derived pair is set rather than the base.
     */
    --rl-page-gutter-left: calc(var(--rl-space-4) + var(--rl-safe-inset-left));
    --rl-page-gutter-right: calc(var(--rl-space-4) + var(--rl-safe-inset-right));
  }

  .rl-table-row--compress .rl-table-row__name { padding-inline: var(--rl-page-gutter-left) 0; }

  .rl-table-row--compress .rl-table-row__cell--secondary { display: none; }

  /*
   * A kept cell sizes to its contents rather than to `column.width`. The
   * width was chosen to line up a full table, and holding it here would
   * spend most of a narrow row on whitespace around a badge; there is no
   * column header left to line up with anyway, since the section hides it at
   * this width.
   *
   * `max-width` still caps it, so one long value cannot crowd out the title:
   * the name cell is the flexible one and has to keep the room it needs.
   */
  .rl-table-row--compress .rl-table-row__cell {
    width: auto;
    max-width: var(--rl-cell-width);
    padding-inline: var(--rl-space-2);
    border-left: none;
    font-size: var(--rl-font-size-sm);
  }

  /* The last kept cell sits against the gutter rather than the row's edge. */
  .rl-table-row--compress .rl-table-row__cell:last-child {
    padding-right: var(--rl-page-gutter-right);
  }

  /*
   * An empty primary cell keeps its place rather than collapsing. Unlike the
   * stacked card there is no label to strand, and letting cells disappear per
   * row would leave a column of badges that jitters left and right as it
   * scrolls.
   *
   * The `:not` is load-bearing. A cell can be both secondary and empty, and
   * `--empty` alone outranks the rule above that drops it, so without this a
   * row with no values would bring back every column compress had just
   * removed.
   */
  .rl-table-row--compress .rl-table-row__cell--empty:not(.rl-table-row__cell--secondary) {
    display: flex;
  }
}
</style>
