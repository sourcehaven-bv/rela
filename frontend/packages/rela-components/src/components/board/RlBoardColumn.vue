<script setup lang="ts" generic="T extends CollectionItem">
/**
 * One column of the board, generic over the card's record.
 *
 * The column owns the heading, the scroll and the add control. What a card
 * looks like is the consumer's through the `card` slot, which is why the
 * column needs nothing from an item but `id` and `title`. `RlTaskCard` is this
 * library's own card for its demo row; pass it, or your own, in the slot.
 */
import { computed, ref } from 'vue'
import type { CollectionItem, Section } from '../../types'
import RlAddButton from '../common/RlAddButton.vue'
import RlSectionHeading from '../layout/RlSectionHeading.vue'
import RlHeading from '../common/RlHeading.vue'
import RlBoardCard from './RlBoardCard.vue'
import RlEmptyState from '../feedback/RlEmptyState.vue'
import {
  useDropTargetColumn,
  type BoardDragData,
  type BoardDropPosition,
} from '../../composables/useBoardDnd'

const props = withDefaults(
  defineProps<{
    section: Section<T>
    addLabel?: string
    selectedId?: string
    /** Whether the column offers its add control. */
    showAdd?: boolean
    /**
     * What to say when the column has no cards. Without it an empty column
     * draws its heading and then nothing, which reads as a column that failed
     * to load rather than one that is genuinely empty.
     *
     * It also gives the column back the height a drop needs. A board whose
     * cards move is asking the user to aim at the empty column above all
     * others, and a 4px strip under a heading is not a target.
     *
     * A column with cards ignores it, so it can be set once for the whole
     * board rather than per column.
     */
    emptyLabel?: string
    /** A line under `emptyLabel`, for what would put a card here. */
    emptyDescription?: string
    /**
     * Whether a given card may be dragged out of this column. Absent, the
     * column has no drag at all, which is what a board that never moves a
     * card wants and what every existing caller already gets.
     */
    canMove?: (item: T) => boolean
    /** The card the keyboard has picked up, if it is in this column. */
    grabbedId?: string
    /** Whether a keyboard move is in flight, so the column shows as a target. */
    keyboardTarget?: boolean
    /** Whether a drop reports where among the cards it landed. See `RlBoard`. */
    reorder?: boolean
    /** Whether the heading offers a control that collapses the column. */
    collapsible?: boolean
  }>(),
  {
    collapsible: false,
    reorder: false,
    showAdd: true,
    canMove: undefined,
    grabbedId: undefined,
    keyboardTarget: false,
    emptyLabel: undefined,
    emptyDescription: undefined,
  },
)

const emit = defineEmits<{
  add: [section: Section<T>]
  select: [item: T]
  drop: [payload: { drag: BoardDragData; section: Section<T>; at?: BoardDropPosition }]
  grab: [item: T]
  release: []
  collapse: [section: Section<T>]
}>()

defineSlots<{
  card?: (props: { item: T; selected: boolean }) => unknown
}>()

const element = ref<HTMLElement>()

const { over } = useDropTargetColumn({
  element,
  data: computed(() => ({ sectionId: props.section.id })),
  onDrop: (drag, at) => emit('drop', { drag, section: props.section, at }),
  reorder: computed(() => props.reorder),
})

/** Whether the column is lit as a place the held card could land. */
const target = computed(() => over.value || props.keyboardTarget)

/*
 * Named rather than inline, because an inline handler inside `v-for` is
 * compiled into a cached closure that keeps the first iteration's scope.
 */
function onGrab(item: T) {
  emit('grab', item)
}

function onRelease() {
  emit('release')
}
</script>

<template>
  <section
    ref="element"
    class="rl-board-column"
    :class="{ 'rl-board-column--target': target }"
    :data-section-id="section.id"
  >
    <RlSectionHeading
      class="rl-board-column__heading"
      :title="section.title"
      :color="section.color"
      :icon="section.icon"
      :count="section.count ?? section.items.length"
      :collapsible="collapsible"
      @collapse="emit('collapse', section)"
    />

    <div class="rl-board-column__cards">
      <template v-for="item in section.items" :key="item.id">
        <!--
          The fallback shows the title alone, because that is all the column
          can know an item has. Anything richer, including this library's own
          `RlTaskCard`, goes in the slot.
        -->
        <RlBoardCard
          :item="item"
          :section-id="section.id"
          :section-title="section.title"
          :draggable="canMove?.(item) ?? false"
          :grabbed="item.id === grabbedId"
          :reorder="reorder"
          @grab="onGrab"
          @release="onRelease"
        >
          <slot name="card" :item="item" :selected="item.id === selectedId">
            <article
              class="rl-board-column__card"
              :class="{ 'rl-board-column__card--selected': item.id === selectedId }"
            >
              <RlHeading :level="3" size="md" weight="normal" line-height="normal">
                <button
                  type="button"
                  class="rl-board-column__card-button"
                  :aria-current="item.id === selectedId ? 'true' : undefined"
                  @click="emit('select', item)"
                >
                  {{ item.title }}
                </button>
              </RlHeading>
            </article>
          </slot>
        </RlBoardCard>
      </template>

      <!--
        Inside the card list, not beside it, so the drop target the column
        registers covers the message: the empty column's whole point is that
        it is the one you are aiming at.
      -->
      <RlEmptyState
        v-if="emptyLabel && !section.items.length"
        :title="emptyLabel"
        :description="emptyDescription"
        icon="inbox"
        size="sm"
        class="rl-board-column__empty"
      />
    </div>

    <RlAddButton
      v-if="showAdd"
      class="rl-board-column__add"
      :label="addLabel ?? 'Add'"
      @click="emit('add', section)"
    />
  </section>
</template>

<style scoped>
/*
 * A column is its own list, so it scrolls as one. With the scroll on the
 * board instead, reaching the bottom of the longest column dragged every
 * other column up with it and left the short ones trailing whitespace; the
 * column height was set by whichever column had the most cards.
 *
 * The column is bounded by the board's height and the card list takes the
 * overflow, so each column scrolls to its own end and the headings and add
 * buttons stay where they were put.
 */
.rl-board-column {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 296px;
  flex: none;
  min-height: 0;
  max-height: 100%;
}

@media (max-width: 767px) {
  .rl-board-column {
    width: 85vw;
    max-width: 320px;
    scroll-snap-align: start;
  }
}

.rl-board-column__heading { padding: 0 var(--rl-space-1) var(--rl-space-3); }

/*
 * The column a held card would land in. Drawn as a tinted, outlined well
 * behind the whole column rather than as a line between two cards, because
 * a drop here sets which column the card is in and nothing about where in
 * it: an insertion line would promise a position the board does not keep.
 */
.rl-board-column::before {
  content: '';
  position: absolute;
  inset: calc(var(--rl-space-2) * -1);
  z-index: 0;
  border: 1px dashed transparent;
  border-radius: var(--rl-radius-lg);
  opacity: 0;
  transition: opacity var(--rl-duration-fast) var(--rl-ease), background-color var(--rl-duration-fast) var(--rl-ease);
  pointer-events: none;
}

.rl-board-column--target::before {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-bg-selected);
  opacity: 1;
}

/* The cards stay above the well. */
.rl-board-column__heading,
.rl-board-column__cards,
.rl-board-column__add {
  position: relative;
  z-index: 1;
}

@media (prefers-reduced-motion: reduce) {
  .rl-board-column::before { transition: none; }
}

/* The title-only fallback card, used when no `card` slot is given. */
.rl-board-column__card {
  position: relative;
  padding: var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
}
.rl-board-column__card:hover { background: var(--rl-color-bg-hover); }
.rl-board-column__card--selected { border-color: var(--rl-color-accent); }

.rl-board-column__card-button {
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}
/* The stretched hit area makes the whole card clickable. */
.rl-board-column__card-button::after {
  content: '';
  position: absolute;
  inset: 0;
}

/*
 * Deliberately NOT `content-visibility: auto`, which `RlTableRow` does use.
 *
 * Two reasons, and the second is the blocking one. A card has no fixed height —
 * it grows with its tags, its meta line and the length of its title — so there
 * is no honest `contain-intrinsic-size` to give, and a wrong estimate makes
 * this column's scrollbar resize as cards come into view. A table row has
 * `--rl-row-height` and does not have that problem.
 *
 * More importantly, the drag-and-drop adapter measures card elements to decide
 * drop targets and to drive auto-scroll. A skipped card has no layout box, so
 * dragging toward the unseen end of a long column would find nothing to drop
 * onto. Rows are not draggable, which is why the same trick is safe there.
 *
 * A board that needs this wants real windowing, with the drag adapter told
 * about it — a bigger change than a CSS property.
 */
.rl-board-column__cards {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  min-height: 0;
  overflow-y: auto;
  /* Room for the card's focus ring, which a flush edge would clip. */
  padding: 2px;
  margin: -2px;
}

/*
 * A dashed well, where the table's equivalent is bare text: the board reads
 * as a row of columns, so an empty one has to keep occupying its share of
 * that row instead of collapsing to the height of its heading.
 *
 * `min-height` rather than a fixed height, so a longer description grows it,
 * and small enough that a board of mostly-empty columns is not all whitespace.
 */
.rl-board-column__empty {
  min-height: 120px;
  justify-content: center;
  border: 1px dashed var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
}

/*
 * Outside the scrolling list, so adding a task does not mean scrolling past
 * every card already in the column to reach the control.
 */
.rl-board-column__add {
  align-self: flex-start;
  flex: none;
  margin-top: var(--rl-space-2);
}
</style>
