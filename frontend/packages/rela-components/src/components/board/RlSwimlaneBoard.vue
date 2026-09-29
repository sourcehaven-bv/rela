<script setup lang="ts" generic="T extends CollectionItem">
/**
 * A board whose columns are crossed by named horizontal lanes: one row per
 * lane, one column per section, a card in the cell where they meet.
 *
 * `RlBoard` answers "what state is this in". A swimlane board answers that
 * and "whose is it" at once, which is what a board grouped by assignee,
 * priority or epic needs and what sorting a plain board cannot show.
 *
 * The columns are named once, in a header row above every lane, and the
 * board scrolls as one sheet in both directions so those columns stay
 * aligned down the page. That is the whole reason the lane does not reuse
 * `RlBoardColumn`, which owns its own heading and its own scroll.
 */
import { computed, ref } from 'vue'
import type { CollectionItem, Section, Swimlane } from '../../types'
import RlAddButton from '../common/RlAddButton.vue'
import RlSectionHeading from '../layout/RlSectionHeading.vue'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlCount from '../common/RlCount.vue'
import RlSwimlane from './RlSwimlane.vue'
import { useBoardAutoScroll, type BoardDragData } from '../../composables/useBoardDnd'
import { useSwimlaneKeyboardMove } from './useBoardKeyboardMove'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    lanes: Swimlane<T>[]
    /**
     * The columns, named once for the whole board.
     *
     * Given as sections so the heading reads the same as `RlBoard`'s, but
     * only `id`, `title`, `color` and `icon` are used: the cards come from each
     * lane's matching section, and a column count that ignored the lanes
     * would contradict them. Absent, the columns are taken from the first
     * lane, which is the common case where every lane has the same columns.
     */
    columns?: Pick<Section<T>, 'id' | 'title' | 'color' | 'icon' | 'collapsed'>[]
    showAddSection?: boolean
    /** Id of the item whose detail is open, marked in the board. */
    selectedId?: string
    /** Label for each cell's add control, such as `Add task`. */
    addLabel?: string
    /** Whether the cells offer an add control. */
    showAdd?: boolean
    /**
     * Whether a given card may be moved, answered per item. Absent, the board
     * has no drag at all. See `RlBoard`.
     */
    canMove?: (item: T) => boolean
  }>(),
  { showAddSection: true, addLabel: 'Add', showAdd: true, canMove: undefined },
)

const emit = defineEmits<{
  add: [payload: { lane: Swimlane<T>; section: Section<T> }]
  select: [item: T]
  toggleLane: [lane: Swimlane<T>]
  /** A collapsed column's rail was clicked. Carries the column's id. */
  expandSection: [id: string]
  addSection: []
  /**
   * A card was dropped on a cell other than its own, naming both the column
   * and the lane it landed in. The board does not move it.
   */
  move: [payload: { item: T; to: Section<T>; lane: Swimlane<T> }]
}>()

defineSlots<{
  card?: (props: { item: T; selected: boolean; lane: Swimlane<T> }) => unknown
}>()

const columns = computed(() => props.columns ?? props.lanes[0]?.sections ?? [])

/** A column's count is every card in it, down all the lanes. */
function columnCount(id: string) {
  return props.lanes.reduce(
    (n, lane) => n + (lane.sections.find((s) => s.id === id)?.items.length ?? 0),
    0,
  )
}

const element = ref<HTMLElement>()
useBoardAutoScroll(element)

/** Finds the dragged item, which the drag payload carries only by id. */
function itemById(id: string) {
  for (const lane of props.lanes) {
    for (const section of lane.sections) {
      const found = section.items.find((item) => item.id === id)
      if (found) return found
    }
  }
  return undefined
}

function onDrop({
  drag,
  section,
  lane,
}: {
  drag: BoardDragData
  section: Section<T>
  lane: Swimlane<T>
}) {
  const item = itemById(drag.itemId)
  if (item) emit('move', { item, to: section, lane })
}

/* Destructured for the template's sake. See `RlBoard`. */
const {
  grabbedId,
  targetId,
  targetLaneId,
  announcement,
  grab,
  cancel: cancelMove,
  onKeydown,
} = useSwimlaneKeyboardMove<T>({
  /* A collapsed lane has nowhere visible to put a card down. */
  lanes: () => props.lanes.filter((lane) => !lane.collapsed),
  columns: () => columns.value.filter((column) => !column.collapsed),
  onMove: (item, to, lane) => emit('move', { item, to, lane }),
})
</script>

<template>
  <div ref="element" class="rl-swimlane-board" @keydown="onKeydown">
    <!--
      One header row, sticky to the top of the board's scroll, so the columns
      are still named twenty lanes down. It sits outside the lanes so its
      cells line up with theirs only because both use the same column width
      and the same gap; nothing here measures anything.
    -->
    <div class="rl-swimlane-board__columns">
      <template v-for="column in columns" :key="column.id">
        <!--
          A collapsed column narrows to a rail across every lane. Its cells
          narrow with it in `RlSwimlane`, so the columns to its right stay
          under their own headings.
        -->
        <button
          v-if="column.collapsed"
          type="button"
          class="rl-swimlane-board__column-collapsed"
          :aria-label="messages.expandSection({ title: column.title })"
          @click="emit('expandSection', column.id)"
        >
          <component
            :is="column.icon"
            v-if="column.icon"
            class="rl-swimlane-board__column-collapsed-icon"
            :size="16"
          />
          <RlStatusDot v-else :color="column.color" />
          <span class="rl-swimlane-board__column-collapsed-title">{{ column.title }}</span>
          <RlCount :value="columnCount(column.id)" />
        </button>

        <RlSectionHeading
          v-else
          class="rl-swimlane-board__column-heading"
          :title="column.title"
          :color="column.color"
          :icon="column.icon"
          :count="columnCount(column.id)"
        />
      </template>

      <RlAddButton
        v-if="showAddSection"
        class="rl-swimlane-board__add-section"
        label="Add section"
        @click="emit('addSection')"
      />
    </div>

    <RlSwimlane
      v-for="lane in lanes"
      :key="lane.id"
      :lane="lane"
      :columns="columns"
      :add-label="addLabel"
      :selected-id="selectedId"
      :show-add="showAdd"
      :can-move="canMove"
      :grabbed-id="grabbedId"
      :keyboard-target-id="targetLaneId === lane.id ? targetId : undefined"
      @add="emit('add', $event)"
      @select="emit('select', $event)"
      @toggle="emit('toggleLane', $event)"
      @drop="onDrop"
      @grab="grab($event.item, $event.section, lane)"
      @release="cancelMove()"
    >
      <template v-if="$slots.card" #card="cardProps">
        <slot name="card" v-bind="cardProps" />
      </template>
    </RlSwimlane>

    <!-- What a keyboard move is doing. See `RlBoard`. -->
    <div class="rl-visually-hidden" role="status" aria-live="polite">
      {{ announcement }}
    </div>
  </div>
</template>

<style scoped>
/*
 * Unlike `RlBoard`, the scroll is the board's in both directions. A lane is
 * a row across every column, so a column cannot own its vertical scroll
 * without sliding out of line with the lane headers beside it.
 */
.rl-swimlane-board {
  --rl-swimlane-column-width: 296px;
  /* Shared by the header's rail and the cell under it, so the two stay in
     step without either measuring the other. */
  --rl-swimlane-collapsed-width: 132px;
  /* The gutter each lane leaves for its sticky title, and the matching indent
     on the column header row so the two line up. */
  --rl-swimlane-header-width: 168px;

  display: flex;
  flex-direction: column;
  gap: var(--rl-space-5);
  flex: 1;
  min-height: 0;
  padding: var(--rl-space-5) var(--rl-page-gutter-right) var(--rl-space-5) var(--rl-page-gutter-left);
  overflow: auto;
}

@media (max-width: 767px) {
  .rl-swimlane-board {
    --rl-swimlane-column-width: 85vw;

    gap: var(--rl-space-4);
    padding: var(--rl-space-4) var(--rl-page-gutter-right) var(--rl-space-4) var(--rl-page-gutter-left);
  }
}

.rl-swimlane-board__columns {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  /*
   * Sized to its columns, not to the board. As a flex child of the board it
   * would otherwise shrink to the board's width, and the board would report
   * nothing to scroll sideways however many columns it had.
   */
  width: max-content;
  gap: var(--rl-space-6);
  /*
   * Indented past the lanes' header rail, so a column heading sits over the
   * cells it names. Padding rather than a margin, so the sticky background
   * covers the rail's width as well and a card scrolling up behind the lane
   * titles does not show through.
   */
  padding-left: var(--rl-swimlane-header-width);
  /* Covers the cards passing underneath, which have no background of their own
     at the board's edges. */
  background: var(--rl-color-bg);
  padding-bottom: var(--rl-space-2);
}

@media (max-width: 767px) {
  .rl-swimlane-board__columns { gap: var(--rl-space-3); }
}

.rl-swimlane-board__column-heading {
  width: var(--rl-swimlane-column-width);
  flex: none;
  padding: 0 var(--rl-space-1);
}

.rl-swimlane-board__add-section { flex: none; }

/*
 * The collapsed column's rail. Horizontal here, because the header row is one
 * line tall and a vertical title would set the height of every column heading
 * beside it; the rail below it, in the lane, is what runs down the board.
 */
.rl-swimlane-board__column-collapsed {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: var(--rl-swimlane-collapsed-width);
  flex: none;
  padding: var(--rl-space-1) var(--rl-space-2);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-sunken);
  font-family: inherit;
  cursor: pointer;
}

.rl-swimlane-board__column-collapsed:hover { background: var(--rl-color-bg-hover); }

.rl-swimlane-board__column-collapsed:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

/* Replaces the dot on the rail too, so collapsing a column keeps its glyph. */
.rl-swimlane-board__column-collapsed-icon {
  flex: none;
  color: var(--rl-color-text-subtle);
}

.rl-swimlane-board__column-collapsed-title {
  font-size: var(--rl-font-size-md);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
}
</style>
