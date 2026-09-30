/**
 * Moving a card with the keyboard: pick it up, choose a column, put it down.
 *
 * Dragging is a pointer gesture, and the HTML drag events the boards are
 * built on never fire without one. That leaves anyone who does not use a
 * mouse unable to do a thing the board otherwise presents as its main
 * action, so the keyboard gets its own path rather than a simulated drag.
 *
 * It is a grab-and-place, deliberately:
 *
 *   Enter/Space   pick the focused card up, or put it down where it is aimed
 *   Left/Right    choose which column it would land in
 *   Escape        put it back where it came from
 *
 * A continuous drag driven by arrow keys would have to invent a cursor
 * position and fire move events against it, which means the card follows
 * something the user cannot see. Choosing a named column instead is the same
 * decision the drop makes, and it is announceable: the board says which
 * column is aimed at, and a screen reader reads it.
 */
import { computed, ref, type Ref } from 'vue'
import type { CollectionItem, Section, Swimlane } from '../../types'
import { useMessages } from '../../composables/useMessages'

export interface BoardKeyboardMove<T extends CollectionItem> {
  /** The card currently held, if any. */
  grabbedId: Ref<string | undefined>
  /** The column the held card is aimed at. */
  targetId: Ref<string | undefined>
  /** A sentence naming what is held and where it is aimed, for announcing. */
  announcement: Ref<string>
  grab: (item: T, from: Section<T>) => void
  cancel: () => void
  onKeydown: (event: KeyboardEvent) => void
}

export function useBoardKeyboardMove<T extends CollectionItem>(options: {
  /** The columns a card may be put down in, in the order they appear. */
  targets: () => Section<T>[]
  onMove: (item: T, to: Section<T>) => void
}): BoardKeyboardMove<T> {
  const messages = useMessages()
  const grabbed = ref<T>()
  const targetId = ref<string>()
  const grabbedId = computed(() => grabbed.value?.id)

  const announcement = computed(() => {
    if (!grabbed.value) return ''
    const to = options.targets().find((section) => section.id === targetId.value)
    if (!to) return ''
    return messages.cardOverTarget({ title: grabbed.value.title, target: to.title })
  })

  function cancel() {
    grabbed.value = undefined
    targetId.value = undefined
  }

  function grab(item: T, from: Section<T>) {
    grabbed.value = item
    targetId.value = from.id
  }

  /** Steps the aim one column along, stopping at each end rather than wrapping. */
  function step(by: number) {
    const targets = options.targets()
    const at = targets.findIndex((section) => section.id === targetId.value)
    if (at === -1) return
    const next = targets[Math.min(Math.max(at + by, 0), targets.length - 1)]
    targetId.value = next.id
  }

  function drop() {
    const item = grabbed.value
    const to = options.targets().find((section) => section.id === targetId.value)
    /*
     * A drop back where the card started moves nothing, and reporting it
     * would have the caller write a value it already holds.
     */
    const movedFrom = to?.items.some((candidate) => candidate.id === item?.id)
    if (item && to && !movedFrom) options.onMove(item, to)
    cancel()
  }

  function onKeydown(event: KeyboardEvent) {
    if (!grabbed.value) return

    switch (event.key) {
      case 'ArrowRight':
        event.preventDefault()
        step(1)
        break
      case 'ArrowLeft':
        event.preventDefault()
        step(-1)
        break
      case 'Enter':
      case ' ':
        event.preventDefault()
        drop()
        break
      case 'Escape':
        event.preventDefault()
        cancel()
        break
    }
  }

  return { grabbedId, targetId, announcement, grab, cancel, onKeydown }
}

/**
 * The same grab-and-place, on a board with two axes.
 *
 * Left and right choose a column, up and down choose a lane, because that is
 * how the board is laid out: the keys move the card the way the eye would
 * move across the grid. A drop names both, which is what `RlSwimlaneBoard`'s
 * `move` carries.
 */
export interface SwimlaneKeyboardMove<T extends CollectionItem> {
  grabbedId: Ref<string | undefined>
  /** The column aimed at. */
  targetId: Ref<string | undefined>
  /** The lane aimed at. */
  targetLaneId: Ref<string | undefined>
  announcement: Ref<string>
  grab: (item: T, from: Section<T>, lane: Swimlane<T>) => void
  cancel: () => void
  onKeydown: (event: KeyboardEvent) => void
}

export function useSwimlaneKeyboardMove<T extends CollectionItem>(options: {
  lanes: () => Swimlane<T>[]
  columns: () => Pick<Section<T>, 'id' | 'title'>[]
  onMove: (item: T, to: Section<T>, lane: Swimlane<T>) => void
}): SwimlaneKeyboardMove<T> {
  const messages = useMessages()
  const grabbed = ref<T>()
  const targetId = ref<string>()
  const targetLaneId = ref<string>()
  const fromSectionId = ref<string>()
  const fromLaneId = ref<string>()
  const grabbedId = computed(() => grabbed.value?.id)

  const announcement = computed(() => {
    if (!grabbed.value) return ''
    const column = options.columns().find((c) => c.id === targetId.value)
    const lane = options.lanes().find((l) => l.id === targetLaneId.value)
    if (!column || !lane) return ''
    return messages.cardOverSwimlaneTarget({
      title: grabbed.value.title,
      target: column.title,
      lane: lane.title,
    })
  })

  function cancel() {
    grabbed.value = undefined
    targetId.value = undefined
    targetLaneId.value = undefined
    fromSectionId.value = undefined
    fromLaneId.value = undefined
  }

  function grab(item: T, from: Section<T>, lane: Swimlane<T>) {
    grabbed.value = item
    targetId.value = from.id
    targetLaneId.value = lane.id
    fromSectionId.value = from.id
    fromLaneId.value = lane.id
  }

  /** Steps along one axis, stopping at each end rather than wrapping. */
  function step(axis: 'column' | 'lane', by: number) {
    const list: { id: string }[] = axis === 'column' ? options.columns() : options.lanes()
    const aim = axis === 'column' ? targetId : targetLaneId
    const at = list.findIndex((entry) => entry.id === aim.value)
    if (at === -1) return
    aim.value = list[Math.min(Math.max(at + by, 0), list.length - 1)].id
  }

  function drop() {
    const item = grabbed.value
    const lane = options.lanes().find((l) => l.id === targetLaneId.value)
    const to = lane?.sections.find((section) => section.id === targetId.value)
    /* A drop back in the cell it came from moves nothing. */
    const home = targetId.value === fromSectionId.value && targetLaneId.value === fromLaneId.value

    if (item && lane && to && !home) options.onMove(item, to, lane)
    cancel()
  }

  function onKeydown(event: KeyboardEvent) {
    if (!grabbed.value) return

    switch (event.key) {
      case 'ArrowRight':
        event.preventDefault()
        step('column', 1)
        break
      case 'ArrowLeft':
        event.preventDefault()
        step('column', -1)
        break
      case 'ArrowDown':
        event.preventDefault()
        step('lane', 1)
        break
      case 'ArrowUp':
        event.preventDefault()
        step('lane', -1)
        break
      case 'Enter':
      case ' ':
        event.preventDefault()
        drop()
        break
      case 'Escape':
        event.preventDefault()
        cancel()
        break
    }
  }

  return { grabbedId, targetId, targetLaneId, announcement, grab, cancel, onKeydown }
}
