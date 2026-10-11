<script setup lang="ts" generic="T extends CollectionItem">
/**
 * A vertical list whose order is the user's to set: form fields, table
 * columns, the values of a choice list, the items in a navigation group.
 *
 * Each row has a handle. Drag it with a pointer, or focus it and use the
 * keyboard:
 *
 *   Enter/Space   pick the row up, or put it down where it now is
 *   Up/Down       move the held row one place
 *   Home/End      move the held row to the start or the end
 *   Escape        put it back where it started
 *
 * ## The list never reorders itself
 *
 * A finished move emits `move` with the item and the index it should end up
 * at, and the caller's data decides the order. That is the board's rule
 * (`useBoardDnd`) for the same reason: a list that moved its own rows would
 * disagree with the caller the moment a move failed to save.
 *
 * While a row is held with the keyboard the list does show it at its new
 * place, because a move the user cannot see is no move at all. That order is
 * a preview held here, and nothing reaches the caller until the row is put
 * down. Escape discards the preview, so a cancelled move costs the caller
 * nothing to undo.
 *
 * ## One level, on purpose
 *
 * Rows do not nest. A tree that accepts drops between levels is a different
 * and much larger component, and the screens that looked like they needed one
 * (a sidebar's groups and items) read better as one sortable list per group.
 *
 * The handle is the only drag source. The rest of the row stays the caller's,
 * so a row can hold a link, a switch or a menu that keeps its own clicks.
 */
import { computed, nextTick, ref, watch } from 'vue'
import { combine } from '@atlaskit/pragmatic-drag-and-drop/combine'
import { draggable, dropTargetForElements } from '@atlaskit/pragmatic-drag-and-drop/element/adapter'
import RlIcon from '../common/RlIcon.vue'
import { useMessages } from '../../composables/useMessages'
import type { CollectionItem } from '../../types'

type Edge = 'top' | 'bottom'

const props = withDefaults(
  defineProps<{
    items: T[]
    /** Names the list for a screen reader: `Form fields`. */
    label: string
    /** Turns the handles off, for a reader who may see the order but not change it. */
    disabled?: boolean
  }>(),
  { disabled: false },
)

const emit = defineEmits<{
  /** A row was put down somewhere new. `toIndex` is its index in the reordered list. */
  move: [item: T, toIndex: number]
}>()

defineSlots<{
  /** The row's content, after the handle. Defaults to the item's title. */
  item?: (props: { item: T; index: number; held: boolean }) => unknown
}>()

const messages = useMessages()

/**
 * Marks drags as belonging to this list, so a row from another list on the
 * same page is not accepted. One symbol per instance.
 */
const listKey = Symbol('rl-sortable-list')

const rowEls = new Map<string, HTMLElement>()
const handleEls = new Map<string, HTMLElement>()

function setRow(id: string, el: unknown) {
  if (el instanceof HTMLElement) rowEls.set(id, el)
  else rowEls.delete(id)
}
function setHandle(id: string, el: unknown) {
  if (el instanceof HTMLElement) handleEls.set(id, el)
  else handleEls.delete(id)
}

// --- Keyboard: grab, move, place ---

const heldId = ref<string>()
const heldFrom = ref(0)
const heldAt = ref(0)
const announcement = ref('')

/** The items as shown: the caller's order, with a held row at its previewed place. */
const shown = computed<T[]>(() => {
  if (heldId.value === undefined) return props.items
  const rest = props.items.filter((item) => item.id !== heldId.value)
  const held = props.items.find((item) => item.id === heldId.value)
  if (!held) return props.items
  rest.splice(heldAt.value, 0, held)
  return rest
})

// A row removed from under the keyboard ends the hold rather than leaving a
// preview of something that no longer exists.
watch(
  () => props.items,
  (items) => {
    if (heldId.value !== undefined && !items.some((item) => item.id === heldId.value)) {
      heldId.value = undefined
    }
  },
)

function describe(item: T, index: number) {
  return { title: item.title, position: index + 1, count: props.items.length }
}

/**
 * Moving a focused element through the DOM drops its focus, and Vue's keyed
 * diff may move the held row rather than its neighbour. Put focus back.
 */
async function refocus(id: string) {
  await nextTick()
  handleEls.get(id)?.focus()
}

function grab(item: T, index: number) {
  heldId.value = item.id
  heldFrom.value = index
  heldAt.value = index
  announcement.value = messages.sortGrabbed(describe(item, index))
}

function place(item: T) {
  const from = heldFrom.value
  const to = heldAt.value
  heldId.value = undefined
  announcement.value = messages.sortDropped(describe(item, to))
  if (to !== from) emit('move', item, to)
  void refocus(item.id)
}

function cancel(item: T) {
  const from = heldFrom.value
  heldId.value = undefined
  announcement.value = messages.sortCancelled(describe(item, from))
  void refocus(item.id)
}

function shift(item: T, to: number) {
  const clamped = Math.max(0, Math.min(props.items.length - 1, to))
  if (clamped === heldAt.value) return
  heldAt.value = clamped
  announcement.value = messages.sortGrabbed(describe(item, clamped))
  void refocus(item.id)
}

function onHandleKeydown(event: KeyboardEvent, item: T, index: number) {
  if (props.disabled) return
  const held = heldId.value === item.id

  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    if (held) place(item)
    else grab(item, index)
    return
  }
  if (!held) return

  const moves: Record<string, number> = {
    ArrowUp: heldAt.value - 1,
    ArrowDown: heldAt.value + 1,
    Home: 0,
    End: props.items.length - 1,
  }
  if (event.key in moves) {
    event.preventDefault()
    shift(item, moves[event.key])
  } else if (event.key === 'Escape') {
    event.preventDefault()
    cancel(item)
  }
}

/**
 * Leaving the list with a row still held cancels the move. A blur with no
 * related target is the DOM move itself, which `refocus` repairs, so it does
 * not count as leaving.
 */
function onFocusOut(event: FocusEvent) {
  if (heldId.value === undefined) return
  const next = event.relatedTarget
  if (!(next instanceof Node)) return
  if ((event.currentTarget as HTMLElement).contains(next)) return
  const item = props.items.find((entry) => entry.id === heldId.value)
  if (item) cancel(item)
}

// --- Pointer: drag a handle, drop above or below a row ---

const draggingId = ref<string>()
const indicator = ref<{ id: string; edge: Edge }>()

interface RowDragData {
  [key: symbol]: true
  itemId: string
}

function isOurs(data: Record<string | symbol, unknown>) {
  return data[listKey] === true
}

function edgeOf(element: Element, clientY: number): Edge {
  const rect = element.getBoundingClientRect()
  return clientY < rect.top + rect.height / 2 ? 'top' : 'bottom'
}

/**
 * Where a row dropped against `target`'s edge ends up. Removing the row from
 * above the drop point shifts every later index down by one.
 */
function dropIndex(fromIndex: number, targetIndex: number, edge: Edge) {
  const insertAt = edge === 'top' ? targetIndex : targetIndex + 1
  return fromIndex < insertAt ? insertAt - 1 : insertAt
}

watch(
  [() => props.items.map((item) => item.id), () => props.disabled],
  (_, _prev, onCleanup) => {
    void nextTick(() => {
      if (props.disabled) return
      const cleanups = props.items.flatMap((item) => {
        const row = rowEls.get(item.id)
        const handle = handleEls.get(item.id)
        if (!row || !handle) return []
        return [
          combine(
            draggable({
              element: row,
              dragHandle: handle,
              getInitialData: () => ({ [listKey]: true, itemId: item.id }),
              onDragStart: () => {
                draggingId.value = item.id
              },
              onDrop: () => {
                draggingId.value = undefined
                indicator.value = undefined
              },
            }),
            dropTargetForElements({
              element: row,
              canDrop: ({ source }) => isOurs(source.data),
              getData: ({ input, element }) => ({ itemId: item.id, edge: edgeOf(element, input.clientY) }),
              onDrag: ({ self, source }) => {
                const edge = self.data.edge as Edge
                const from = props.items.findIndex((entry) => entry.id === (source.data as unknown as RowDragData).itemId)
                const target = props.items.findIndex((entry) => entry.id === item.id)
                // An indicator that would put the row back where it is promises nothing.
                indicator.value = dropIndex(from, target, edge) === from ? undefined : { id: item.id, edge }
              },
              onDragLeave: () => {
                indicator.value = undefined
              },
              onDrop: ({ self, source }) => {
                indicator.value = undefined
                const id = (source.data as unknown as RowDragData).itemId
                const from = props.items.findIndex((entry) => entry.id === id)
                const target = props.items.findIndex((entry) => entry.id === item.id)
                const dragged = props.items[from]
                if (!dragged || target < 0) return
                const to = dropIndex(from, target, self.data.edge as Edge)
                if (to !== from) emit('move', dragged, to)
              },
            }),
          ),
        ]
      })
      onCleanup(() => cleanups.forEach((cleanup) => cleanup()))
    })
  },
  { immediate: true, flush: 'post' },
)
</script>

<template>
  <div class="rl-sortable" @focusout="onFocusOut">
    <ol class="rl-sortable__list" :aria-label="label">
      <li
        v-for="(item, index) in shown"
        :key="item.id"
        :ref="(el) => setRow(item.id, el)"
        class="rl-sortable__row"
        :class="{
          'rl-sortable__row--held': heldId === item.id,
          'rl-sortable__row--dragging': draggingId === item.id,
          'rl-sortable__row--drop-top': indicator?.id === item.id && indicator.edge === 'top',
          'rl-sortable__row--drop-bottom': indicator?.id === item.id && indicator.edge === 'bottom',
        }"
      >
        <button
          :ref="(el) => setHandle(item.id, el)"
          type="button"
          class="rl-sortable__handle"
          :aria-label="messages.sortHandle(describe(item, index))"
          :aria-pressed="heldId === item.id"
          :disabled="disabled"
          @keydown="onHandleKeydown($event, item, index)"
        >
          <RlIcon name="grip-vertical" :size="16" aria-hidden="true" />
        </button>
        <div class="rl-sortable__content">
          <slot name="item" :item="item" :index="index" :held="heldId === item.id">
            {{ item.title }}
          </slot>
        </div>
      </li>
    </ol>
    <div class="rl-visually-hidden" role="status" aria-live="polite">{{ announcement }}</div>
  </div>
</template>

<style scoped>
.rl-sortable__list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg);
}

.rl-sortable__row {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-height: var(--rl-row-height);
  padding: var(--rl-space-1) var(--rl-space-3) var(--rl-space-1) var(--rl-space-1);
  background: var(--rl-color-bg);
  transition: background-color var(--rl-duration-fast) var(--rl-ease);
}

.rl-sortable__row + .rl-sortable__row {
  border-top: 1px solid var(--rl-color-border);
}

.rl-sortable__row:first-child { border-radius: var(--rl-radius-md) var(--rl-radius-md) 0 0; }
.rl-sortable__row:last-child { border-radius: 0 0 var(--rl-radius-md) var(--rl-radius-md); }
.rl-sortable__row:only-child { border-radius: var(--rl-radius-md); }

/* Held with the keyboard: lifted, so the row being moved is unmistakable. */
.rl-sortable__row--held {
  z-index: 1;
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-md);
}

/* The pointer drag shows its own ghost; the row left behind marks the origin. */
.rl-sortable__row--dragging { opacity: 0.4; }

.rl-sortable__row--drop-top::before,
.rl-sortable__row--drop-bottom::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  z-index: 2;
  height: 2px;
  background: var(--rl-color-focus);
  pointer-events: none;
}
.rl-sortable__row--drop-top::before { top: -1px; }
.rl-sortable__row--drop-bottom::after { bottom: -1px; }

.rl-sortable__handle {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text-subtle);
  cursor: grab;
}

.rl-sortable__handle:hover:not(:disabled) {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
}

.rl-sortable__handle[aria-pressed='true'] {
  color: var(--rl-color-text);
  cursor: grabbing;
}

.rl-sortable__handle:disabled {
  cursor: default;
  opacity: 0.4;
}

.rl-sortable__handle:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: 1px;
}

.rl-sortable__content {
  flex: 1;
  min-width: 0;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
}

@media (pointer: coarse) {
  .rl-sortable__handle {
    min-width: var(--rl-tap-target);
    min-height: var(--rl-tap-target);
  }
}
</style>
