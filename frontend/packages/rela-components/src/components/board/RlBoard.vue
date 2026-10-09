<script setup lang="ts" generic="T extends CollectionItem">
/**
 * A board of sectioned columns, generic over the card's record.
 *
 * The board reads `id` and `title` off an item. Give a card its contents
 * through the `card` slot, which is forwarded to every column.
 */
import { ref } from 'vue'
import type { CollectionItem, Section } from '../../types'
import RlAddButton from '../common/RlAddButton.vue'
import RlBoardColumn from './RlBoardColumn.vue'
import RlBoardColumnCollapsed from './RlBoardColumnCollapsed.vue'
import {
  useBoardAutoScroll,
  type BoardDragData,
  type BoardDropPosition,
} from '../../composables/useBoardDnd'
import { useBoardKeyboardMove } from './useBoardKeyboardMove'
import { useSectionToggleFocus } from './useSectionToggleFocus'

const props = withDefaults(
  defineProps<{
    sections: Section<T>[]
    showAddSection?: boolean
    /** Id of the item whose detail is open, marked in the board. */
    selectedId?: string
    /** Label for each column's add control, such as `Add task`. */
    addLabel?: string
    /**
     * Whether each column offers its add control. A board whose items have no
     * create form, or a user without permission to create one, must not be
     * offered it.
     */
    showAdd?: boolean
    /**
     * Whether a given card may be moved, answered per item.
     *
     * Absent, the board has no drag at all. That is the default because a
     * board that cannot save a move should not invite one, and because every
     * caller that predates this keeps the behaviour it had.
     */
    canMove?: (item: T) => boolean
    /**
     * What a column with no cards says. Set once for the board; columns with
     * cards ignore it. See `RlBoardColumn` for why an empty column needs it.
     */
    emptyLabel?: string
    /** A line under `emptyLabel`, for what would put a card in the column. */
    emptyDescription?: string
    /**
     * Whether a drop onto a card reports where it landed, for a board whose
     * card order the reader sets by hand. With it, `move` carries `at`, and a
     * card dropped among the cards of its own column is reported too.
     *
     * The keyboard move still picks a column only.
     */
    reorder?: boolean
    /**
     * Whether each column heading offers a collapse control. The board does
     * not fold the column itself: it emits `collapseSection`, and the caller
     * sets `collapsed` on the section, as it does for `expandSection`.
     */
    collapsible?: boolean
  }>(),
  {
    reorder: false,
    collapsible: false,
    showAddSection: true,
    addLabel: 'Add',
    showAdd: true,
    canMove: undefined,
    emptyLabel: undefined,
    emptyDescription: undefined,
  },
)

const emit = defineEmits<{
  add: [section: Section<T>]
  select: [item: T]
  expandSection: [section: Section<T>]
  collapseSection: [section: Section<T>]
  addSection: []
  /**
   * A card was dropped on a column other than its own, or, with `reorder`,
   * before or after another card (`at`). The board does not move it; the
   * caller's data decides whether the move happens.
   */
  move: [payload: { item: T; to: Section<T>; at?: BoardDropPosition }]
}>()

const element = ref<HTMLElement>()
useBoardAutoScroll(element)

const toggleFocus = useSectionToggleFocus(element, () =>
  props.sections.map((section) => `${section.id}:${!!section.collapsed}`).join('|'),
)

function onExpand(section: Section<T>) {
  toggleFocus.expect(section.id)
  emit('expandSection', section)
}

function onCollapse(section: Section<T>) {
  toggleFocus.expect(section.id)
  emit('collapseSection', section)
}

/** Finds the dragged item, which the drag payload carries only by id. */
function itemById(id: string) {
  for (const section of props.sections) {
    const found = section.items.find((item) => item.id === id)
    if (found) return found
  }
  return undefined
}

function onDrop({ drag, section, at }: { drag: BoardDragData; section: Section<T>; at?: BoardDropPosition }) {
  const item = itemById(drag.itemId)
  if (item) emit('move', { item, to: section, at })
}

/*
 * Destructured, because a `setup` return only unwraps refs at its top level:
 * left inside the object, `keyboard.grabbedId` reaches the template as the
 * ref itself and every comparison against it is false.
 */
const {
  grabbedId,
  targetId,
  announcement,
  grab,
  cancel: cancelMove,
  onKeydown,
} = useBoardKeyboardMove<T>({
  /* A collapsed column is not a place a card can be put down. */
  targets: () => props.sections.filter((section) => !section.collapsed),
  onMove: (item, to) => emit('move', { item, to }),
})

defineSlots<{
  card?: (props: { item: T; selected: boolean }) => unknown
}>()
</script>

<template>
  <div ref="element" class="rl-board" @keydown="onKeydown">
    <template v-for="section in sections" :key="section.id">
      <RlBoardColumnCollapsed
        v-if="section.collapsed"
        :section="section"
        @expand="onExpand"
      />
      <RlBoardColumn
        v-else
        :section="section"
        :selected-id="selectedId"
        :add-label="addLabel"
        :show-add="showAdd"
        :empty-label="emptyLabel"
        :empty-description="emptyDescription"
        :can-move="canMove"
        :grabbed-id="grabbedId"
        :keyboard-target="targetId === section.id"
        :reorder="reorder"
        :collapsible="collapsible"
        @add="emit('add', $event)"
        @select="emit('select', $event)"
        @drop="onDrop"
        @grab="grab($event, section)"
        @release="cancelMove()"
        @collapse="onCollapse"
      >
        <template v-if="$slots.card" #card="cardProps">
          <slot name="card" v-bind="cardProps" />
        </template>
      </RlBoardColumn>
    </template>

    <RlAddButton
      v-if="showAddSection"
      class="rl-board__add-section"
      label="Add section"
      @click="emit('addSection')"
    />

    <!--
      What a keyboard move is doing, for someone who cannot see the column
      light up. Polite, because the user caused it and is not waiting on it.
    -->
    <div class="rl-visually-hidden" role="status" aria-live="polite">
      {{ announcement }}
    </div>
  </div>
</template>

<style scoped>
.rl-board {
  display: flex;
  gap: var(--rl-space-6);
  flex: 1;
  min-height: 0;
  padding: var(--rl-space-5) var(--rl-page-gutter-right) var(--rl-space-5) var(--rl-page-gutter-left);
  /*
   * Horizontal only. The vertical scroll belongs to each column, so the board
   * never scrolls as one sheet and the column headings stay on screen however
   * far down a column you are.
   */
  overflow-x: auto;
  overflow-y: hidden;
  align-items: stretch;
}

@media (max-width: 767px) {
  .rl-board {
    gap: var(--rl-space-3);
    padding: var(--rl-space-4) var(--rl-page-gutter-right) var(--rl-space-4) var(--rl-page-gutter-left);
    /* Swiping settles on one column at a time. */
    scroll-snap-type: x mandatory;
    scroll-padding-left: var(--rl-page-gutter-left);
  }
}

.rl-board__add-section {
  flex: none;
  align-self: flex-start;
}
</style>
