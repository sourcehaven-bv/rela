<script setup lang="ts" generic="T extends CollectionItem">
/**
 * One horizontal band of a swimlane board: a lane header and that lane's
 * columns, left to right in the order the lane gives them.
 *
 * The lane draws no column headings. Those belong to the board, which shows
 * them once above every lane; a heading repeated per lane would say the same
 * thing a dozen times and cost a row of height each time.
 */
import { computed } from 'vue'
import type { CollectionItem, Section, Swimlane } from '../../types'
import RlDisclosure from '../common/RlDisclosure.vue'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlCount from '../common/RlCount.vue'
import RlHeading from '../common/RlHeading.vue'
import RlSwimlaneColumn from './RlSwimlaneColumn.vue'
import type { BoardDragData } from '../../composables/useBoardDnd'

const props = withDefaults(
  defineProps<{
    lane: Swimlane<T>
    /**
     * The board's columns, in order, so every lane renders the same cells in
     * the same places.
     *
     * A lane with no section for a column still gets a cell, empty; without
     * it the lane would close the gap and every column to its right would sit
     * under the wrong heading. Absent, the lane's own sections are the
     * columns, which is what a lane rendered on its own has.
     */
    columns?: Pick<Section<T>, 'id' | 'title' | 'color' | 'icon' | 'collapsed'>[]
    addLabel?: string
    selectedId?: string
    /** Whether each column offers an add control for this lane. */
    showAdd?: boolean
    /** Whether a given card may be moved. See `RlSwimlaneBoard`. */
    canMove?: (item: T) => boolean
    /** The card the keyboard has picked up, if it is in this lane. */
    grabbedId?: string
    /** The cell a keyboard move is aimed at, when it is in this lane. */
    keyboardTargetId?: string
  }>(),
  {
    columns: undefined,
    addLabel: 'Add',
    showAdd: true,
    canMove: undefined,
    grabbedId: undefined,
    keyboardTargetId: undefined,
  },
)

const emit = defineEmits<{
  add: [payload: { lane: Swimlane<T>; section: Section<T> }]
  select: [item: T]
  toggle: [lane: Swimlane<T>]
  drop: [payload: { drag: BoardDragData; section: Section<T>; lane: Swimlane<T> }]
  grab: [payload: { item: T; section: Section<T> }]
  release: []
}>()

defineSlots<{
  card?: (props: { item: T; selected: boolean; lane: Swimlane<T> }) => unknown
}>()

const total = computed(
  () => props.lane.count ?? props.lane.sections.reduce((n, s) => n + s.items.length, 0),
)

/** One cell per board column, in the board's order, empty where the lane has none. */
const cells = computed<Section<T>[]>(() => {
  if (!props.columns) return props.lane.sections
  return props.columns.map((column) => ({
    ...column,
    ...(props.lane.sections.find((section) => section.id === column.id) ?? { items: [] }),
    /* The board owns the collapse, so a lane's own flag cannot contradict it. */
    collapsed: column.collapsed,
  }))
})
</script>

<template>
  <section class="rl-swimlane" :class="{ 'rl-swimlane--collapsed': lane.collapsed }">
    <!--
      The header is a rail beside the columns rather than a row above them,
      so it can stay put while the board scrolls sideways without ending up
      over a card. See the stylesheet.
    -->
    <header class="rl-swimlane__header">
      <RlDisclosure
        :open="!lane.collapsed"
        :label="lane.title"
        @toggle="emit('toggle', lane)"
      />
      <component :is="lane.icon" v-if="lane.icon" class="rl-swimlane__icon" :size="16" />
      <RlStatusDot v-else-if="lane.color" :color="lane.color" />
      <RlHeading :level="3" size="md" line-height="normal">{{ lane.title }}</RlHeading>
      <RlCount :value="total" />
    </header>

    <div v-if="!lane.collapsed" class="rl-swimlane__columns">
      <RlSwimlaneColumn
        v-for="section in cells"
        :key="section.id"
        :section="section"
        :lane-title="lane.title"
        :add-label="addLabel"
        :selected-id="selectedId"
        :show-add="showAdd"
        :collapsed="section.collapsed"
        :lane-id="lane.id"
        :can-move="canMove"
        :grabbed-id="grabbedId"
        :keyboard-target="keyboardTargetId === section.id"
        @add="emit('add', { lane, section: $event })"
        @select="emit('select', $event)"
        @drop="emit('drop', { ...$event, lane })"
        @grab="emit('grab', { item: $event, section })"
        @release="emit('release')"
      >
        <template v-if="$slots.card" #card="cardProps">
          <slot name="card" v-bind="cardProps" :lane="lane" />
        </template>
      </RlSwimlaneColumn>
    </div>
  </section>
</template>

<style scoped>
.rl-swimlane {
  display: flex;
  align-items: flex-start;
  /* So the lane's rule runs the full width of the board's scroll, not just
     the part of it on screen. */
  min-width: max-content;
  padding-bottom: var(--rl-space-5);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-swimlane--collapsed { padding-bottom: var(--rl-space-3); }

/*
 * The header stays put while the board scrolls sideways, so the lane a card
 * is in is still named once its cards have moved off screen. That is the one
 * thing a swimlane board has to keep on screen, and a title that scrolls
 * away leaves rows of cards belonging to nobody.
 *
 * It is a rail of its own beside the columns rather than a row above them,
 * so the stuck title slides over its own gutter rather than over the first
 * column's cards, where it would read as part of a card it does not belong
 * to. The board's column header row is indented by the same width, which is
 * what keeps the two in line.
 */
.rl-swimlane__header {
  position: sticky;
  left: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  box-sizing: border-box;
  width: var(--rl-swimlane-header-width);
  flex: none;
  padding-right: var(--rl-space-3);
  /* Aligned with the first card, not with the top of the lane's padding. */
  padding-top: 2px;
  /* Opaque, so a card that scrolls under the gutter does not show through. */
  background: var(--rl-color-bg);
}

/* Replaces the status dot rather than joining it. See `RlSectionHeading`. */
.rl-swimlane__icon {
  flex: none;
  color: var(--rl-color-text-subtle);
}

.rl-swimlane__columns {
  display: flex;
  align-items: flex-start;
  /* Sized to its cells. See the board's header row for why. */
  width: max-content;
  flex: none;
  /* The cells hold their own focus-ring padding inside their width, so the
     row needs no compensation; it starts where the headings do. */
  gap: var(--rl-space-6);
}

/* The lane's hover reveals every add control in it. See `RlSwimlaneColumn`. */
.rl-swimlane:hover :deep(.rl-swimlane-column__add) { visibility: visible; }

@media (max-width: 767px) {
  .rl-swimlane__columns { gap: var(--rl-space-3); }
}
</style>
