<script setup lang="ts" generic="T extends CollectionItem">
/**
 * One column inside one swimlane: the cards for that lane and that section,
 * and the lane's own add control.
 *
 * This is not `RlBoardColumn` with the heading hidden. That column owns a
 * heading, its own vertical scroll and a height bounded by the board; in a
 * swimlane board the heading belongs to the board, the scroll belongs to the
 * board, and the column grows to whatever its cards need so the lane below
 * starts under the longest column rather than inside it.
 */
import { computed, ref } from 'vue'
import type { CollectionItem, Section } from '../../types'
import RlAddButton from '../common/RlAddButton.vue'
import RlHeading from '../common/RlHeading.vue'
import RlBoardCard from './RlBoardCard.vue'
import { useDropTargetColumn, type BoardDragData } from '../../composables/useBoardDnd'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = defineProps<{
  section: Section<T>
  /** Name of the lane this column sits in, for the add control's label. */
  laneTitle: string
  addLabel?: string
  selectedId?: string
  showAdd?: boolean
  /**
   * Whether the board has this column collapsed, in which case the cell is a
   * rail holding the column's place rather than a list of cards.
   */
  collapsed?: boolean
  /** The lane's id, carried in the drag payload and the drop event. */
  laneId: string
  /** Whether a given card may be dragged out of this cell. */
  canMove?: (item: T) => boolean
  /** The card the keyboard has picked up, if it is in this cell. */
  grabbedId?: string
  /** Whether a keyboard move is aimed here. */
  keyboardTarget?: boolean
}>()

const emit = defineEmits<{
  add: [section: Section<T>]
  select: [item: T]
  drop: [payload: { drag: BoardDragData; section: Section<T> }]
  grab: [item: T]
  release: []
}>()

defineSlots<{
  card?: (props: { item: T; selected: boolean }) => unknown
}>()

const element = ref<HTMLElement>()

const { over } = useDropTargetColumn({
  element,
  data: computed(() => ({ sectionId: props.section.id, laneId: props.laneId })),
  onDrop: (drag) => emit('drop', { drag, section: props.section }),
})

const target = computed(() => over.value || props.keyboardTarget)
</script>

<template>
  <div
    ref="element"
    class="rl-swimlane-column"
    :class="{
      'rl-swimlane-column--collapsed': collapsed,
      'rl-swimlane-column--target': target,
    }"
  >
    <!--
      A collapsed cell draws nothing. It is here to hold the column's place
      in the lane, so the columns to its right stay under their headings.
    -->
    <template v-if="!collapsed" v-for="item in section.items" :key="item.id">
      <!-- The title-only fallback, as in `RlBoardColumn`. -->
      <RlBoardCard
        :item="item"
        :section-id="section.id"
        :lane-id="laneId"
        :section-title="`${section.title}, ${laneTitle}`"
        :draggable="canMove?.(item) ?? false"
        :grabbed="item.id === grabbedId"
        @grab="emit('grab', $event)"
        @release="emit('release')"
      >
        <slot name="card" :item="item" :selected="item.id === selectedId">
          <article
            class="rl-swimlane-column__card"
            :class="{ 'rl-swimlane-column__card--selected': item.id === selectedId }"
          >
            <RlHeading :level="4" size="md" weight="normal" line-height="normal">
              <button
                type="button"
                class="rl-swimlane-column__card-button"
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
      The label names the lane as well as the section, because a swimlane
      board repeats one add control per lane per column: "Add task" alone
      would give a screen reader a dozen identical buttons that do different
      things.
    -->
    <RlAddButton
      v-if="showAdd && !collapsed"
      class="rl-swimlane-column__add"
      :label="addLabel ?? 'Add'"
      :aria-label="messages.addToSection({ addLabel: addLabel ?? 'Add', section: section.title, lane: laneTitle })"
      @click="emit('add', section)"
    />
  </div>
</template>

<style scoped>
.rl-swimlane-column {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  box-sizing: border-box;
  width: var(--rl-swimlane-column-width);
  flex: none;
  /*
   * Room for a card's focus ring, which a flush edge would clip. Inside the
   * cell's own width rather than pulled out of it with a negative margin: a
   * margin here would shrink every gap by 4px and walk the lane's cells out
   * of line with the column headings above them, one column at a time.
   */
  padding: 2px;
}

/*
 * Shown on hover or keyboard focus, so a lane of cards is not a lane of
 * buttons. It keeps its space either way, so nothing shifts when it appears.
 * The hover that reveals it is the whole lane's, so that rule lives in
 * `RlSwimlane`; this one covers the keyboard.
 */
.rl-swimlane-column__add {
  align-self: flex-start;
  flex: none;
  visibility: hidden;
}

.rl-swimlane-column__add:focus-visible { visibility: visible; }

@media (pointer: coarse) {
  /* No hover to reveal it with. */
  .rl-swimlane-column__add { visibility: visible; }
}

/*
 * As wide as the board's collapsed rail, which is its heading plus its
 * padding rather than a fixed width; `fit-content` on an empty box is zero,
 * so the width comes from the board instead.
 */
.rl-swimlane-column--collapsed { width: var(--rl-swimlane-collapsed-width); }

/* The cell a held card would land in. See `RlBoardColumn` for the reasoning. */
.rl-swimlane-column::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  border: 1px dashed transparent;
  border-radius: var(--rl-radius-lg);
  opacity: 0;
  transition: opacity var(--rl-duration-fast) var(--rl-ease), background-color var(--rl-duration-fast) var(--rl-ease);
  pointer-events: none;
}

.rl-swimlane-column--target::before {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-bg-selected);
  opacity: 1;
}

/*
 * An empty cell has no height of its own, so a lane with nothing in one
 * column would light up a sliver. This gives the well something to be.
 */
.rl-swimlane-column--target { min-height: var(--rl-space-10, 48px); }

/*
 * Hidden while the cell is a drop target. The control is revealed by the
 * lane's hover, and a card dragged over the cell counts as hovering it, so
 * without this an "Add task" appears under every held card.
 */
.rl-swimlane-column--target .rl-swimlane-column__add { visibility: hidden; }

@media (prefers-reduced-motion: reduce) {
  .rl-swimlane-column::before { transition: none; }
}

.rl-swimlane-column__card {
  position: relative;
  z-index: 1;
  padding: var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
}
.rl-swimlane-column__card:hover { background: var(--rl-color-bg-hover); }
.rl-swimlane-column__card--selected { border-color: var(--rl-color-accent); }

.rl-swimlane-column__card-button {
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}
.rl-swimlane-column__card-button::after {
  content: '';
  position: absolute;
  inset: 0;
}
</style>
