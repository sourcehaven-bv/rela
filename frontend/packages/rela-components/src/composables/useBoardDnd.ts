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
 * - **Grouping by default.** With `group`, the middle of a card is a third
 *   drop zone, reported as `onto`: the caller groups the two cards. Without
 *   it, every part of a card reorders.
 *
 * Columns can be dragged too, by a handle, on a board that opts in with
 * `sectionsMovable`. A column drop reports the column it landed beside.
 */
import { onScopeDispose, readonly, ref, watch, type Ref } from 'vue'
import { draggable, dropTargetForElements } from '@atlaskit/pragmatic-drag-and-drop/element/adapter'
import { autoScrollForElements } from '@atlaskit/pragmatic-drag-and-drop-auto-scroll/element'
import { setCustomNativeDragPreview } from '@atlaskit/pragmatic-drag-and-drop/utils/set-custom-native-drag-preview'
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

/** Marks the payload of a dragged column, as opposed to a card. */
const BOARD_SECTION = Symbol('rl-board-section')

/** What a dragged column carries. */
export interface BoardSectionDragData {
  [BOARD_SECTION]: true
  sectionId: string
}

function isSectionDrag(data: Record<string | symbol, unknown>): boolean {
  return data[BOARD_SECTION] === true
}

/** Where a dragged column was dropped: before or after another column. */
export interface BoardSectionDropPosition {
  sectionId: string
  targetId: string
  placement: 'before' | 'after'
}

/** Marks a card as a drop target, as opposed to its column. */
const BOARD_CARD = Symbol('rl-board-card')

/** Where in a column a card was dropped: before or after another card. */
export interface BoardDropPosition {
  targetId: string
  /** `onto`: dropped on the middle of the card, to group the two. */
  placement: 'before' | 'after' | 'onto'
}

/**
 * Which part of a card the pointer is over. In a wrapping grid the order
 * runs left to right, so the halves are the left and right ones; in a list
 * they are the top and bottom. With `group`, the middle half is a zone of
 * its own and only the outer quarters reorder.
 */
export type CardZone = RowEdge | 'left' | 'right' | 'center'

export function zoneAt(
  input: { clientX: number; clientY: number },
  element: HTMLElement,
  group: boolean,
): CardZone {
  const rect = element.getBoundingClientRect()
  const parent = element.parentElement
  const horizontal = !!parent && getComputedStyle(parent).display === 'grid'
  const at = horizontal
    ? (input.clientX - rect.left) / rect.width
    : (input.clientY - rect.top) / rect.height
  if (group && at > 0.25 && at < 0.75) return 'center'
  if (horizontal) return at < 0.5 ? 'left' : 'right'
  return closestEdge(input.clientY, rect)
}

interface BoardCardTargetData {
  [BOARD_CARD]: true
  itemId: string
  edge: CardZone
}

/**
 * Makes one card a drop target for its siblings, on a board that reports
 * where a card lands (`reorder`) or groups cards (`group`). The drop itself
 * is reported by the column, which reads the card it landed on; the card
 * only works out which zone of it the pointer is over, for the column and
 * for its own drop mark.
 */
export function useDropTargetCard(options: {
  element: Ref<HTMLElement | undefined>
  enabled: Ref<boolean>
  itemId: Ref<string>
  /** Whether the middle of the card groups. See `group` on `RlBoard`. */
  group?: Ref<boolean>
}) {
  const edge = ref<CardZone | null>(null)

  watch(
    [options.element, options.enabled],
    ([element, enabled], _prev, onCleanup) => {
      if (!element || !enabled) return
      type Input = { clientX: number; clientY: number }
      const edgeAt = (input: Input) => zoneAt(input, element, options.group?.value ?? false)
      const track = (source: { data: Record<string | symbol, unknown> }, input: Input) => {
        const self = (source.data as unknown as BoardDragData).itemId === options.itemId.value
        edge.value = self ? null : edgeAt(input)
      }
      onCleanup(
        dropTargetForElements({
          element,
          canDrop: ({ source }) => isBoardDrag(source.data),
          getData: ({ input }) => ({ [BOARD_CARD]: true, itemId: options.itemId.value, edge: edgeAt(input) }),
          onDragEnter: ({ source, location }) => track(source, location.current.input),
          onDrag: ({ source, location }) => track(source, location.current.input),
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
  return { targetId: itemId, placement: edge === 'center' ? 'onto' : edge === 'top' || edge === 'left' ? 'before' : 'after' }
}

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** Room around the lifted card, so its shadow and tilt are not clipped. */
const PREVIEW_PAD = 16

/**
 * Draws the held card as lifted off the board: a copy, tilted and a little
 * larger, with a deeper shadow. Under reduced motion the copy keeps its
 * shape and only gains the shadow. The copy is wrapped in a shell that
 * carries the board's classes and attributes, so the board's own (scoped)
 * card styles still apply outside it.
 */
function setLiftedPreview(
  element: HTMLElement,
  input: { clientX: number; clientY: number },
  nativeSetDragImage: Parameters<typeof setCustomNativeDragPreview>[0]['nativeSetDragImage'],
) {
  const rect = element.getBoundingClientRect()
  setCustomNativeDragPreview({
    nativeSetDragImage,
    getOffset: () => ({ x: input.clientX - rect.left + PREVIEW_PAD, y: input.clientY - rect.top + PREVIEW_PAD }),
    render: ({ container }) => {
      const board = element.closest('.rl-board')
      const shell = document.createElement('div')
      // The board's class and scoped-style attributes only: a copied id,
      // role or label would put a second landmark in the page while dragging.
      for (const attr of board?.attributes ?? []) {
        if (attr.name === 'class' || attr.name.startsWith('data-v-')) shell.setAttribute(attr.name, attr.value)
      }
      Object.assign(shell.style, {
        display: 'block',
        padding: `${PREVIEW_PAD}px`,
        overflow: 'visible',
        background: 'transparent',
        width: 'auto',
      })
      const copy = element.cloneNode(true) as HTMLElement
      copy.classList.remove('rl-board-card--dragging')
      Object.assign(copy.style, {
        width: `${rect.width}px`,
        transform: prefersReducedMotion() ? 'none' : 'rotate(2.5deg) scale(1.05)',
        boxShadow: '0 14px 28px rgb(0 0 0 / 35%)',
      })
      shell.appendChild(copy)
      container.appendChild(shell)
    },
  })
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
          onGenerateDragPreview: ({ nativeSetDragImage, location }) => {
            setLiftedPreview(element, location.current.input, nativeSetDragImage)
          },
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
  /** Whether a drop on a card's middle reports `onto`. See `group` on `RlBoard`. */
  group?: Ref<boolean>
  /**
   * Whether another column may be dropped beside this one, and what to call
   * when it is. See `sectionsMovable` on `RlBoard`.
   */
  acceptsSection?: Ref<boolean>
  onSectionDrop?: (move: BoardSectionDropPosition) => void
}) {
  const over = ref(false)
  /** The side a held column would land on, while one is over this column. */
  const sectionEdge = ref<'before' | 'after' | null>(null)

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

      /** Which side of this column a held column is over: left half is before. */
      const sideOf = (source: { data: Record<string | symbol, unknown> }, input: { clientX: number }) => {
        const drag = source.data as unknown as BoardSectionDragData
        if (drag.sectionId === options.data.value.sectionId) return null
        const rect = element.getBoundingClientRect()
        return input.clientX < rect.left + rect.width / 2 ? 'before' : 'after'
      }

      /** A position counts only for what the board opted into. */
      const positionFor = (at: BoardDropPosition | undefined) => {
        if (!at) return undefined
        if (at.placement === 'onto') return options.group?.value ? at : undefined
        return options.reorder?.value ? at : undefined
      }

      onCleanup(
        dropTargetForElements({
          element,
          getData: () => ({ ...options.data.value, [BOARD_DRAG]: true }),
          canDrop: ({ source }) =>
            isSectionDrag(source.data) ? !!options.acceptsSection?.value : !isForeign(source),
          onDragEnter: ({ source, location }) => {
            if (isSectionDrag(source.data)) {
              sectionEdge.value = sideOf(source, location.current.input)
              return
            }
            const drag = source.data as unknown as BoardDragData
            over.value = !isHome(drag)
          },
          onDrag: ({ source, location }) => {
            if (isSectionDrag(source.data)) sectionEdge.value = sideOf(source, location.current.input)
          },
          onDragLeave: () => {
            over.value = false
            sectionEdge.value = null
          },
          onDrop: ({ source, location }) => {
            over.value = false
            sectionEdge.value = null
            if (isSectionDrag(source.data)) {
              const placement = sideOf(source, location.current.input)
              const drag = source.data as unknown as BoardSectionDragData
              if (placement) {
                options.onSectionDrop?.({ sectionId: drag.sectionId, targetId: options.data.value.sectionId, placement })
              }
              return
            }
            const drag = source.data as unknown as BoardDragData
            const at = positionFor(dropPosition(location.current.dropTargets, drag.itemId))
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
    sectionEdge.value = null
  })

  return { over: readonly(over), sectionEdge: readonly(sectionEdge) }
}

/**
 * Makes one column draggable by its handle. The column is what is dragged,
 * so the preview shows the whole column; only a press on the handle starts
 * the drag, so the heading's own controls keep their clicks.
 */
export function useDraggableSection(options: {
  element: Ref<HTMLElement | undefined>
  handle: Ref<HTMLElement | undefined>
  enabled: Ref<boolean>
  sectionId: Ref<string>
}) {
  const dragging = ref(false)

  watch(
    [options.element, options.handle, options.enabled],
    ([element, handle, enabled], _prev, onCleanup) => {
      if (!element || !handle || !enabled) return
      onCleanup(
        draggable({
          element,
          dragHandle: handle,
          getInitialData: () => ({ sectionId: options.sectionId.value, [BOARD_SECTION]: true }),
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
          canScroll: ({ source }) => isBoardDrag(source.data) || isSectionDrag(source.data),
        }),
      )
    },
    { immediate: true },
  )
}
