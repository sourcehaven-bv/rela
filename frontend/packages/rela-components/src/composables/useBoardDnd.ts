/**
 * Drag and drop for the boards: a card is a drag source, a column is a drop
 * target, and a completed drag reports where the card was dropped.
 *
 * The library never moves the card itself. A drop emits an event naming the
 * item and the column it landed on, and the caller's data is what decides
 * whether the card actually moves; a board that moved its own cards would
 * disagree with the caller's data the moment a move failed to save.
 *
 * Built on `@atlaskit/pragmatic-drag-and-drop`, which drives the browser's
 * own drag events rather than replacing them with pointer tracking. That is
 * what lets a card stay whatever the `card` slot rendered: the drag lives on
 * a wrapper around the consumer's markup, so the card keeps its own clicks
 * and its own link.
 *
 * Native draggables inside that markup have to be turned off for this to
 * work, because a link or an image would otherwise drag itself and the
 * card's drag would never start. `RlBoardCard` does that.
 *
 * What it deliberately does not do:
 *
 * - **Touch.** The HTML drag events this is built on do not fire on touch.
 *   The keyboard path below is the accessible route and works for touch users
 *   through a screen reader, but a finger cannot drag a card. A board that
 *   needs it should offer a "move to" menu on the card instead.
 * - **Reordering by default.** A drop reports the column, not a position in
 *   it, unless the board opts in with `reorder`: most boards derive their
 *   order from a field rather than from where a card was let go. With it, a
 *   drop onto a card also names that card and which side of it the dropped
 *   card lands on, and a drop inside the card's own column is reported too.
 *   Storing the order is still the caller's.
 */
import { onScopeDispose, readonly, ref, watch, type Ref } from 'vue'
import { draggable, dropTargetForElements } from '@atlaskit/pragmatic-drag-and-drop/element/adapter'
import { autoScrollForElements } from '@atlaskit/pragmatic-drag-and-drop-auto-scroll/element'
import { closestEdge, type RowEdge } from './useRowReorder'

/** Marks the payload as one of this library's, so a board ignores foreign drags. */
const BOARD_DRAG = Symbol('rl-board-drag')

/** What a dragged card carries: which item, and where it came from. */
export interface BoardDragData {
  [BOARD_DRAG]: true
  itemId: string
  /** The column the card started in, so a drop back into it is a no-op. */
  fromSectionId: string
  /** The lane the card started in, absent on a board without lanes. */
  fromLaneId?: string
}

/** What a column advertises as a drop target. */
export interface BoardDropData {
  [BOARD_DRAG]: true
  sectionId: string
  laneId?: string
}

function isBoardDrag(data: Record<string | symbol, unknown>): boolean {
  return data[BOARD_DRAG] === true
}

/** Marks a card as a drop target, as opposed to its column. */
const BOARD_CARD = Symbol('rl-board-card')

/** Where in a column a card was dropped: before or after another card. */
export interface BoardDropPosition {
  targetId: string
  placement: 'before' | 'after'
}

interface BoardCardTargetData {
  [BOARD_CARD]: true
  itemId: string
  edge: RowEdge
}

/**
 * Makes one card a drop target for its siblings, on a board that reports
 * where a card lands (`reorder`). The drop itself is reported by the column,
 * which reads the card it landed on; the card only works out which half of
 * it the pointer is over, for the column and for its own drop line.
 */
export function useDropTargetCard(options: {
  element: Ref<HTMLElement | undefined>
  enabled: Ref<boolean>
  itemId: Ref<string>
}) {
  const edge = ref<RowEdge | null>(null)

  watch(
    [options.element, options.enabled],
    ([element, enabled], _prev, onCleanup) => {
      if (!element || !enabled) return
      const edgeAt = (clientY: number) => closestEdge(clientY, element.getBoundingClientRect())
      const track = (source: { data: Record<string | symbol, unknown> }, clientY: number) => {
        const self = (source.data as unknown as BoardDragData).itemId === options.itemId.value
        edge.value = self ? null : edgeAt(clientY)
      }
      onCleanup(
        dropTargetForElements({
          element,
          canDrop: ({ source }) => isBoardDrag(source.data),
          getData: ({ input }) => ({ [BOARD_CARD]: true, itemId: options.itemId.value, edge: edgeAt(input.clientY) }),
          onDragEnter: ({ source, location }) => track(source, location.current.input.clientY),
          onDrag: ({ source, location }) => track(source, location.current.input.clientY),
          onDragLeave: () => {
            edge.value = null
          },
          onDrop: () => {
            edge.value = null
          },
        }),
      )
    },
    { immediate: true },
  )

  onScopeDispose(() => {
    edge.value = null
  })

  return { edge: readonly(edge) }
}

/** The card a drop landed on, read off the innermost drop target. */
function dropPosition(
  targets: { data: Record<string | symbol, unknown> }[],
  dragged: string,
): BoardDropPosition | undefined {
  const card = targets[0]?.data
  if (!card || card[BOARD_CARD] !== true) return undefined
  const { itemId, edge } = card as unknown as BoardCardTargetData
  if (itemId === dragged) return undefined
  return { targetId: itemId, placement: edge === 'top' ? 'before' : 'after' }
}

/**
 * Makes one card draggable.
 *
 * `enabled` is a ref rather than a plain boolean because whether a card may
 * move is the caller's answer per item, and it can change without the card
 * being re-created: a permission arriving, or an edit finishing, must be able
 * to turn dragging on for a card already on screen.
 */
export function useDraggableCard(options: {
  element: Ref<HTMLElement | undefined>
  enabled: Ref<boolean>
  data: Ref<Omit<BoardDragData, typeof BOARD_DRAG>>
}) {
  const dragging = ref(false)

  watch(
    [options.element, options.enabled],
    ([element, enabled], _prev, onCleanup) => {
      if (!element || !enabled) return

      onCleanup(
        draggable({
          element,
          getInitialData: () => ({ ...options.data.value, [BOARD_DRAG]: true }),
          onDragStart: () => {
            dragging.value = true
          },
          onDrop: () => {
            dragging.value = false
          },
        }),
      )
    },
    { immediate: true },
  )

  onScopeDispose(() => {
    dragging.value = false
  })

  return { dragging: readonly(dragging) }
}

/**
 * Makes one column a drop target, and reports a card dropped on it.
 *
 * `over` is true only while a card that could actually land here is above it.
 * A card dragged back over the column it came from reports false, because
 * highlighting the column a card already lives in promises a move that will
 * not happen.
 */
export function useDropTargetColumn(options: {
  element: Ref<HTMLElement | undefined>
  data: Ref<Omit<BoardDropData, typeof BOARD_DRAG>>
  /**
   * The drop, and the card it landed on when the board reports positions.
   * Called for a drop inside the card's own column only when it has a
   * position, since that is the one drop there that can change anything.
   */
  onDrop: (drag: BoardDragData, at?: BoardDropPosition) => void
  /** Whether drops report a position. See `reorder` on `RlBoard`. */
  reorder?: Ref<boolean>
}) {
  const over = ref(false)

  watch(
    options.element,
    (element, _prev, onCleanup) => {
      if (!element) return

      const isForeign = (source: { data: Record<string | symbol, unknown> }) =>
        !isBoardDrag(source.data)

      /** A drop back where the card started moves nothing. */
      const isHome = (drag: BoardDragData) =>
        drag.fromSectionId === options.data.value.sectionId &&
        drag.fromLaneId === options.data.value.laneId

      onCleanup(
        dropTargetForElements({
          element,
          getData: () => ({ ...options.data.value, [BOARD_DRAG]: true }),
          canDrop: ({ source }) => !isForeign(source),
          onDragEnter: ({ source }) => {
            const drag = source.data as unknown as BoardDragData
            over.value = !isHome(drag)
          },
          onDragLeave: () => {
            over.value = false
          },
          onDrop: ({ source, location }) => {
            over.value = false
            const drag = source.data as unknown as BoardDragData
            const at = options.reorder?.value
              ? dropPosition(location.current.dropTargets, drag.itemId)
              : undefined
            if (isHome(drag) && !at) return
            options.onDrop(drag, at)
          },
        }),
      )
    },
    { immediate: true },
  )

  onScopeDispose(() => {
    over.value = false
  })

  return { over: readonly(over) }
}

/**
 * Scrolls the board while a card is held near one of its edges.
 *
 * A board wider or taller than the screen has columns and lanes that cannot
 * be reached while a card is held, because the hand holding the card is the
 * one that would otherwise scroll. This belongs to the board rather than to
 * a column: the board is the element that scrolls.
 */
export function useBoardAutoScroll(element: Ref<HTMLElement | undefined>) {
  watch(
    element,
    (el, _prev, onCleanup) => {
      if (!el) return
      onCleanup(
        autoScrollForElements({
          element: el,
          canScroll: ({ source }) => isBoardDrag(source.data),
        }),
      )
    },
    { immediate: true },
  )
}
