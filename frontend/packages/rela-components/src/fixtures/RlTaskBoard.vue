<script setup lang="ts">
/**
 * `RlBoard` wired up for this library's own `Task` row.
 *
 * The board is generic and its fallback card shows the title alone, so the
 * task card comes from the `card` slot. This is a fixture for the demo
 * stories and pages, not part of the library's surface.
 */
import type { Section, Task } from '../types'
import RlBoard from '../components/board/RlBoard.vue'
import RlTaskCard from '../components/board/RlTaskCard.vue'
import type { BoardDropPosition } from '../composables/useBoardDnd'

withDefaults(
  defineProps<{
    sections: Section<Task>[]
    showAddSection?: boolean
    selectedId?: string
    showAdd?: boolean
    canMove?: (item: Task) => boolean
    emptyLabel?: string
    emptyDescription?: string
    reorder?: boolean
    collapsible?: boolean
  }>(),
  {
    reorder: false,
    collapsible: false,
    showAddSection: true,
    showAdd: true,
    canMove: undefined,
    emptyLabel: undefined,
    emptyDescription: undefined,
  },
)

const emit = defineEmits<{
  add: [section: Section<Task>]
  select: [item: Task]
  expandSection: [section: Section<Task>]
  collapseSection: [section: Section<Task>]
  addSection: []
  move: [payload: { item: Task; to: Section<Task>; at?: BoardDropPosition }]
}>()
</script>

<template>
  <RlBoard
    :sections="sections"
    :show-add-section="showAddSection"
    :selected-id="selectedId"
    :show-add="showAdd"
    :can-move="canMove"
    :empty-label="emptyLabel"
    :empty-description="emptyDescription"
    :reorder="reorder"
    :collapsible="collapsible"
    add-label="Add task"
    @add="emit('add', $event)"
    @select="emit('select', $event)"
    @expand-section="emit('expandSection', $event)"
    @collapse-section="emit('collapseSection', $event)"
    @add-section="emit('addSection')"
    @move="emit('move', $event)"
  >
    <template #card="{ item, selected }">
      <RlTaskCard :task="item" :selected="selected" @click="emit('select', item)" />
    </template>
  </RlBoard>
</template>
