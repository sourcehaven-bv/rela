/**
 * Drag and drop for reordering table rows: a row's handle starts the drag,
 * every row of the same table is a drop target, and a drop reports which row
 * the dragged one should land before or after.
 *
 * Like the boards (`useBoardDnd`), the table never moves a row itself. A drop
 * emits an event, and the caller's data decides whether the row moves; a table
 * that moved its own rows would disagree with the caller the moment a move
 * failed to save.
 *
 * The drag starts only from the handle. The rest of the row keeps its own
 * clicks and its link, and a drag that starts on the title stays the
 * browser's own link drag, which no row accepts.
 *
 * Touch is not supported, for the reason `useBoardDnd` gives. The handle's
 * arrow keys are the accessible route.
 */
import { onScopeDispose, readonly, ref, watch, type Ref } from 'vue'
import { draggable, dropTargetForElements } from '@atlaskit/pragmatic-drag-and-drop/element/adapter'

/** Marks the payload as a row drag, so a row ignores cards and foreign drags. */
const ROW_DRAG = Symbol('rl-row-drag')

/** Which half of the target row the pointer is over. */
export type RowEdge = 'top' | 'bottom'

/** A completed drop: the dragged row lands before or after the target. */
export interface RowDrop {
  itemId: string
  targetId: string
  placement: 'before' | 'after'
}

interface RowDragData {
  [ROW_DRAG]: true
  itemId: string
  /** Which table the row belongs to. A row never lands in another table. */
  group: string
}

/**
 * The half of a row a pointer at `clientY` is over. The upper half means
 * "before this row", the lower half "after it".
 *
 * Computed here rather than with the library's hitbox package, which this
 * project does not carry for one comparison.
 */
export function closestEdge(clientY: number, rect: { top: number; height: number }): RowEdge {
  return clientY < rect.top + rect.height / 2 ? 'top' : 'bottom'
}

/**
 * Makes one row draggable by its handle and a drop target for its siblings.
 *
 * `edge` is the half of this row a sibling is held over, for the drop line,
 * and null otherwise. A row held over itself shows no line, because dropping
 * it there moves nothing.
 */
export function useReorderableRow(options: {
  element: Ref<HTMLElement | undefined>
  handle: Ref<HTMLElement | undefined>
  enabled: Ref<boolean>
  itemId: Ref<string>
  group: Ref<string>
  onDrop: (drop: RowDrop) => void
}) {
  const dragging = ref(false)
  const edge = ref<RowEdge | null>(null)

  const isOurs = (data: Record<string | symbol, unknown>) =>
    data[ROW_DRAG] === true && (data as unknown as RowDragData).group === options.group.value

  watch(
    [options.element, options.handle, options.enabled],
    ([element, handle, enabled], _prev, onCleanup) => {
      if (!element || !handle || !enabled) return

      const stopDrag = draggable({
        element,
        dragHandle: handle,
        getInitialData: () => ({ [ROW_DRAG]: true, itemId: options.itemId.value, group: options.group.value }),
        onDragStart: () => {
          dragging.value = true
        },
        onDrop: () => {
          dragging.value = false
        },
      })

      const track = (source: { data: Record<string | symbol, unknown> }, clientY: number) => {
        const self = (source.data as unknown as RowDragData).itemId === options.itemId.value
        edge.value = self ? null : closestEdge(clientY, element.getBoundingClientRect())
      }

      const stopTarget = dropTargetForElements({
        element,
        canDrop: ({ source }) => isOurs(source.data),
        onDragEnter: ({ source, location }) => track(source, location.current.input.clientY),
        onDrag: ({ source, location }) => track(source, location.current.input.clientY),
        onDragLeave: () => {
          edge.value = null
        },
        onDrop: ({ source, location }) => {
          edge.value = null
          const drag = source.data as unknown as RowDragData
          if (drag.itemId === options.itemId.value) return
          const at = closestEdge(location.current.input.clientY, element.getBoundingClientRect())
          options.onDrop({
            itemId: drag.itemId,
            targetId: options.itemId.value,
            placement: at === 'top' ? 'before' : 'after',
          })
        },
      })

      onCleanup(() => {
        stopDrag()
        stopTarget()
      })
    },
    { immediate: true },
  )

  onScopeDispose(() => {
    dragging.value = false
    edge.value = null
  })

  return { dragging: readonly(dragging), edge: readonly(edge) }
}
