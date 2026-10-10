/**
 * Drag a gantt bar to move it, or one of its edges to change its start or
 * end. One gesture writes one PATCH.
 *
 * Whether a bar may be dragged is decided lazily, per entity: when the
 * pointer rests on a bar or the bar's label takes focus, the entity is read
 * once and its `_actions` and `_fields` decide. The gantt response carries no
 * write affordance because computing one per row would cost store queries per
 * row, and `update` alone says nothing about whether the two date fields are
 * writable. The verdict is a UI hint: the write re-reads the entity, sends
 * field preconditions, and the server re-authorizes.
 *
 * Only the entity's own planned window moves. A rolled-up span belongs to
 * its children and is never a drag target.
 */
import { onBeforeUnmount, ref } from 'vue'
import { ApiError, getEntity, getErrorMessage, updateEntity, type GanttNode } from '@/api'
import type { Entity } from '@/types/entity'
import { isFieldWritable, isPropertyRedacted } from '@/utils/affordances'
import { actionAllowed } from '@/utils/affordancesWarning'
import type { DateKind } from '@/utils/calendarGrid'
import { isoDay, parseDay } from '@/utils/ganttLayout'

export type GanttDragMode = 'move' | 'start' | 'end'

/** The two date properties a gantt source maps, each with its kind. */
export interface GanttDates {
  start: string
  end: string
  startKind: DateKind
  endKind: DateKind
}

/** A window in epoch days, inclusive. */
export interface DayWindow {
  start: number
  end: number
}

/** A bar's window while it is being dragged. */
export interface GanttPreview extends DayWindow {
  id: string
  /** The stored window the gesture began from. The view draws nodes with
   * their preview applied, so a handler cannot read it back off the node. */
  from: DayWindow
}

export interface GanttDragDeps {
  /** Nil: returned when the source maps fewer than both dates; such a bar
   * is not draggable. */
  datesOf(type: string): GanttDates | null
  pxPerDay(): number
  /** Reloads the chart after a write; resolves once it shows the result. */
  refresh(type: string): Promise<void>
  error(message: string): void
  warning(message: string): void
}

/** How far the pointer must travel before a press becomes a drag; a shorter
 * press stays a click and drills in. */
const DRAG_THRESHOLD_PX = 4
/** How long the pointer must rest on a bar before its entity is read. */
const PROBE_DELAY_MS = 150
const DAY_PREFIX = /^(\d{4}-\d{2}-\d{2})(.*)$/
const SLIDER_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'Enter', 'Escape'])

/** The address that reads and writes the face the chart shows. */
export function nodeAddress(node: GanttNode): string {
  return node.face ? `${node.id}@${node.face}` : node.id
}

/** The day a stored date or datetime falls on, as the gantt endpoint draws
 * it: the date part in the value's own offset. */
export function storedDay(value: unknown): number | null {
  if (value == null || value === '') return null
  const m = DAY_PREFIX.exec(String(value))
  return m ? parseDay(m[1]) : null
}

/**
 * shiftStored moves a stored value by whole days in the frame the gantt
 * draws it in: the date part changes and a datetime keeps its time and
 * offset verbatim. Shifting in the browser's zone instead would move a value
 * near midnight to a day the chart does not show.
 *
 * Nil: returns null for a value that is not a date, so the caller can refuse
 * the write rather than overwrite it.
 */
export function shiftStored(value: unknown, delta: number, kind: DateKind): string | null {
  const m = DAY_PREFIX.exec(String(value ?? ''))
  const day = m ? parseDay(m[1]) : null
  if (!m || day === null) return null
  const key = isoDay(day + delta)
  return kind === 'date' ? key : key + m[2]
}

/** canDrag reads a fetched entity: the reader may update it, and may read and
 * write both date fields, and both hold a date. */
export function canDrag(entity: Entity, dates: GanttDates): boolean {
  if (!actionAllowed(entity, 'update')) return false
  for (const prop of [dates.start, dates.end]) {
    // A redacted property has no `_fields` verdict, which would read as writable.
    if (isPropertyRedacted(prop, entity._redacted)) return false
    if (!isFieldWritable(entity._fields?.[prop])) return false
    if (storedDay(entity.properties?.[prop]) === null) return false
  }
  return true
}

/** shift applies a day delta to a window for one drag mode. A resized edge
 * stops at the other one, so a bar is never shorter than one day. */
export function shift(w: DayWindow, mode: GanttDragMode, delta: number): DayWindow {
  if (mode === 'move') return { start: w.start + delta, end: w.end + delta }
  if (mode === 'start') return { start: Math.min(w.start + delta, w.end), end: w.end }
  return { start: w.start, end: Math.max(w.end + delta, w.start) }
}

interface Gesture {
  node: GanttNode
  mode: GanttDragMode
  x0: number
  pointerId: number
  el: HTMLElement
  moved: boolean
  /** The window the pointer started from: a keyboard preview, if one is held. */
  base: DayWindow
  from: DayWindow
}

export function useGanttDrag(deps: GanttDragDeps) {
  /** Verdicts per address; absent means not asked yet. */
  const verdicts = ref(new Map<string, boolean>())
  const preview = ref<GanttPreview | null>(null)
  /** The address being written; its bar keeps its preview until the reload. */
  const committing = ref<string | null>(null)

  let probeTimer: ReturnType<typeof setTimeout> | undefined
  const probing = new Set<string>()
  /** Bumped by reset(): a read started before it must not write its verdict. */
  let generation = 0
  let gesture: Gesture | null = null
  let suppressClick = false
  let alive = true

  function plannedDays(node: GanttNode): DayWindow | null {
    const start = parseDay(node.planned?.start)
    const end = parseDay(node.planned?.end)
    return start === null || end === null ? null : { start, end }
  }

  function draggable(node: GanttNode): boolean {
    return verdicts.value.get(nodeAddress(node)) === true
  }

  function setVerdict(addr: string, ok: boolean) {
    verdicts.value = new Map(verdicts.value).set(addr, ok)
  }

  async function readVerdict(node: GanttNode, dates: GanttDates) {
    const addr = nodeAddress(node)
    if (verdicts.value.has(addr) || probing.has(addr)) return
    const gen = generation
    probing.add(addr)
    try {
      const entity = await getEntity(node.type, addr)
      if (gen === generation) setVerdict(addr, canDrag(entity, dates))
    } catch (err) {
      // A refusal is an answer; a network or server failure is not, so the
      // next hover asks again. Either way the bar still drills.
      const status = err instanceof ApiError ? err.status : undefined
      if (gen === generation && (status === 403 || status === 404)) setVerdict(addr, false)
    } finally {
      probing.delete(addr)
    }
  }

  /** Reads the entity's verdict once the pointer rests (or now, on focus). */
  function probe(node: GanttNode, now = false) {
    const dates = plannedDays(node) ? deps.datesOf(node.type) : null
    if (!dates || verdicts.value.has(nodeAddress(node))) return
    clearTimeout(probeTimer)
    if (now) void readVerdict(node, dates)
    else probeTimer = setTimeout(() => void readVerdict(node, dates), PROBE_DELAY_MS)
  }

  function unprobe() {
    clearTimeout(probeTimer)
  }

  function endGesture(): Gesture | null {
    const g = gesture
    gesture = null
    window.removeEventListener('keydown', onGestureKey)
    if (g?.el.hasPointerCapture?.(g.pointerId)) g.el.releasePointerCapture(g.pointerId)
    return g
  }

  /** Swallows the click that follows the press still held down, if one comes. */
  function suppressNextClick() {
    suppressClick = true
    // Cleared once the press ends, in case the browser sends no click.
    window.addEventListener('pointerup', () => setTimeout(() => (suppressClick = false)), {
      once: true,
    })
  }

  /** Abandons a pointer gesture or a keyboard preview (pointercancel,
   * lostpointercapture, Escape, navigation). A write in flight keeps its
   * preview until the reload. */
  function cancel() {
    const g = endGesture()
    if (g?.moved) suppressNextClick()
    if (!committing.value) preview.value = null
  }

  function onGestureKey(e: KeyboardEvent) {
    if (e.key === 'Escape') cancel()
  }

  /** Forgets every verdict, for a chart whose sources may map other fields. */
  function reset() {
    generation++
    verdicts.value = new Map()
    cancel()
  }

  function onPointerDown(e: PointerEvent, node: GanttNode, mode: GanttDragMode) {
    if (e.button !== 0 || gesture || committing.value || !draggable(node)) return
    // The view passes the node as drawn: with a keyboard preview held, its
    // dates are the preview, and the drag continues from there.
    const held = preview.value?.id === node.id ? preview.value : null
    const base = plannedDays(node)
    if (!base) return
    if (!held) preview.value = null
    suppressClick = false
    const el = e.currentTarget as HTMLElement
    el.setPointerCapture?.(e.pointerId)
    gesture = {
      node,
      mode,
      x0: e.clientX,
      pointerId: e.pointerId,
      el,
      moved: false,
      base,
      from: held?.from ?? base,
    }
    window.addEventListener('keydown', onGestureKey)
  }

  function onPointerMove(e: PointerEvent) {
    const g = gesture
    if (!g || e.pointerId !== g.pointerId) return
    const dx = e.clientX - g.x0
    if (!g.moved && Math.abs(dx) < DRAG_THRESHOLD_PX) return
    g.moved = true
    const next = shift(g.base, g.mode, Math.round(dx / deps.pxPerDay()))
    const cur = preview.value
    if (cur?.id === g.node.id && cur.start === next.start && cur.end === next.end) return
    preview.value = { id: g.node.id, ...next, from: g.from }
  }

  function onPointerUp(e: PointerEvent) {
    if (!gesture || e.pointerId !== gesture.pointerId) return
    const g = endGesture()
    if (!g?.moved) return
    // The click that follows this pointerup must not drill. Cleared on the
    // next tick in case the browser sends no click.
    suppressClick = true
    setTimeout(() => (suppressClick = false))
    void commit(g.node)
  }

  /** Capture-phase click handler on the chart: swallows the click a drag ends with. */
  function onClickCapture(e: MouseEvent) {
    if (!suppressClick) return
    suppressClick = false
    e.stopPropagation()
    e.preventDefault()
  }

  /** Slider keys on a focused handle: arrows preview (Shift: a week), Enter
   * writes, Escape abandons. */
  function onHandleKey(e: KeyboardEvent, node: GanttNode, mode: GanttDragMode) {
    if (!SLIDER_KEYS.has(e.key)) return
    // Claimed even while a write is in flight, or the arrows scroll the chart.
    e.preventDefault()
    const days = plannedDays(node)
    if (!days || committing.value || !draggable(node)) return
    const held = preview.value?.id === node.id ? preview.value : null
    if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      const step = e.shiftKey ? 7 : 1
      const delta = e.key === 'ArrowLeft' ? -step : step
      // `days` is the drawn window, which is the preview when one is held.
      preview.value = { id: node.id, ...shift(days, mode, delta), from: held?.from ?? days }
    } else if (e.key === 'Enter' && held) {
      void commit(node)
    } else if (e.key === 'Escape' && held) {
      preview.value = null
    }
  }

  /** A handle losing focus to anything but another handle of its bar drops
   * an unsaved keyboard preview; within the bar, start and end can both be
   * adjusted and saved in one write. */
  function onHandleBlur(node: GanttNode, toSameBar: boolean) {
    if (toSameBar || gesture || committing.value) return
    if (preview.value?.id === node.id) preview.value = null
  }

  async function commit(node: GanttNode) {
    const next = preview.value
    const dates = deps.datesOf(node.type)
    if (!next || next.id !== node.id || !dates) return
    const edges = [
      {
        prop: dates.start,
        kind: dates.startKind,
        from: next.from.start,
        delta: next.start - next.from.start,
      },
      {
        prop: dates.end,
        kind: dates.endKind,
        from: next.from.end,
        delta: next.end - next.from.end,
      },
    ]
    if (edges.every((x) => x.delta === 0)) {
      preview.value = null
      return
    }
    const addr = nodeAddress(node)
    const name = node.title || node.id
    committing.value = addr
    try {
      // Read again: the hover verdict may be stale, and the write needs the
      // raw stored values (a datetime keeps its time) and their versions.
      const entity = await getEntity(node.type, addr)
      if (!alive) return
      if (!canDrag(entity, dates)) {
        setVerdict(addr, false)
        deps.error(`You can no longer change the dates of ${name}.`)
        return
      }
      // The versions only guard the moment between this read and the
      // write. A date that moved since the chart loaded is a conflict too,
      // or the delta would land on a value the user never saw.
      if (edges.some((x) => storedDay(entity.properties[x.prop]) !== x.from)) {
        deps.error(`${name} was changed by someone else. The chart now shows its current dates.`)
        return
      }
      const properties: Record<string, string> = {}
      const versions: Record<string, string> = {}
      for (const x of edges) {
        // canDrag checked that both values hold a date, so `moved` is set.
        const moved = shiftStored(entity.properties[x.prop], x.delta, x.kind)
        if (x.delta === 0 || moved === null) continue
        properties[x.prop] = moved
        const v = entity._versions?.properties?.[x.prop]
        if (v) versions[x.prop] = v
      }
      const res = await updateEntity(node.type, addr, {
        properties,
        preconditions: { properties: versions },
      })
      if (alive && res.warnings?.length) {
        deps.warning(
          `${name} was rescheduled with warnings: ${res.warnings.map((w) => w.detail).join('; ')}`
        )
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) setVerdict(addr, false)
      if (alive) deps.error(`Could not reschedule ${name}: ${getErrorMessage(err)}`)
    } finally {
      // Keep the preview until the chart shows the server's dates, written
      // or not, so the bar does not jump back and forth. Verdicts stay, so
      // a focused handle keeps its focus through the reload.
      try {
        if (alive) await deps.refresh(node.type)
      } finally {
        committing.value = null
        preview.value = null
      }
    }
  }

  onBeforeUnmount(() => {
    alive = false
    clearTimeout(probeTimer)
    endGesture()
  })

  return {
    preview,
    committing,
    draggable,
    probe,
    unprobe,
    reset,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    cancel,
    onClickCapture,
    onHandleKey,
    onHandleBlur,
  }
}
