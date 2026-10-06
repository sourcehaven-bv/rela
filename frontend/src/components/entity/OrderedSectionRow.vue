<script setup lang="ts">
/**
 * A row of a detail-view section table whose rows the reader may reorder: a
 * handle cell to drag the row, or to move it a place with the arrow keys,
 * then the caller's cells.
 *
 * The section table is a plain `<table>` rather than `RlTable`, so it takes
 * the library's row-reorder composable directly. Like the table, the row
 * moves nothing itself; it reports the move.
 */
import { computed, nextTick, ref } from 'vue'
import type { RowMove } from 'rela-components/components/table/types'
import { useReorderableRow } from 'rela-components/composables/useRowReorder'

const props = defineProps<{
  rowId: string
  /** Names the row for the handle's label. */
  title: string
  /** Which table the row belongs to, so drops stay inside it. */
  group: string
}>()

const emit = defineEmits<{ reorder: [move: RowMove] }>()

const rowEl = ref<HTMLElement>()
const handleEl = ref<HTMLElement>()
const { dragging, edge } = useReorderableRow({
  element: rowEl,
  handle: handleEl,
  enabled: computed(() => true),
  itemId: computed(() => props.rowId),
  group: computed(() => props.group),
  onDrop: (drop) => emit('reorder', drop),
})

// See RlTableRow: focus returns to the handle once the row has moved.
function onHandleKey(event: KeyboardEvent) {
  const step = event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0
  if (!step) return
  event.preventDefault()
  event.stopPropagation()
  emit('reorder', { itemId: props.rowId, step })
  void nextTick(() => handleEl.value?.focus())
}
</script>

<template>
  <tr
    ref="rowEl"
    :class="{ 'ordered-row--dragging': dragging, [`ordered-row--drop-${edge}`]: edge !== null }"
  >
    <td class="ordered-row__handle-cell">
      <button
        ref="handleEl"
        type="button"
        class="ordered-row__handle"
        :aria-label="`Move ${title}`"
        aria-description="Use the up and down arrow keys to move this row"
        @keydown="onHandleKey"
      >
        <svg viewBox="0 0 10 16" width="10" height="16" fill="currentColor" aria-hidden="true">
          <circle cx="2.5" cy="3" r="1.3" />
          <circle cx="7.5" cy="3" r="1.3" />
          <circle cx="2.5" cy="8" r="1.3" />
          <circle cx="7.5" cy="8" r="1.3" />
          <circle cx="2.5" cy="13" r="1.3" />
          <circle cx="7.5" cy="13" r="1.3" />
        </svg>
      </button>
    </td>
    <slot />
  </tr>
</template>

<style scoped>
/*
 * The table's cell rule lives in EntityDetail's scoped styles, which reach
 * the caller's cells but not this one, so the handle cell repeats its
 * border.
 */
.ordered-row__handle-cell {
  width: 1px;
  padding-inline: var(--space-xs);
  border-bottom: 1px solid var(--rl-color-border);
}

.ordered-row__handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--rl-color-text-muted);
  cursor: grab;
}

.ordered-row__handle:hover {
  color: var(--rl-color-text);
}

.ordered-row__handle:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 0;
}

.ordered-row--dragging {
  opacity: 0.5;
}

/* The drop line, as an inset edge so it cannot shift the layout. */
.ordered-row--drop-top > :deep(td) {
  box-shadow: inset 0 2px 0 var(--rl-color-accent);
}

.ordered-row--drop-bottom > :deep(td) {
  box-shadow: inset 0 -2px 0 var(--rl-color-accent);
}
</style>
