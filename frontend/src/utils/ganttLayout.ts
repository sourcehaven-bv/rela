/**
 * Pure layout math for the gantt view: axis ticks, bar positioning, and the
 * flatten/drill logic over the server's folded tree. No DOM, no store — the
 * depth/expansion cases are table-tested here rather than asserted through a
 * mounted chart (the calendarGrid.ts precedent).
 *
 * All arithmetic is DATE-granular. The server ships "YYYY-MM-DD" strings and
 * the view compares days, never instants, so DST and zone offsets cannot
 * shift a bar.
 */

import type { GanttNode } from '@/api/gantts'

export type GanttZoom = 'quarter' | 'month' | 'week'

/** One row of the flattened outline: a node plus its indent level. */
export interface GanttRow {
  node: GanttNode
  indent: number
}

/** A [start, end] pair in epoch days; either bound may be null. */
export interface DaySpan {
  start: number | null
  end: number | null
}

const MS_PER_DAY = 86_400_000

/** parseDay converts "YYYY-MM-DD" to epoch days (UTC), or null. */
export function parseDay(s: string | undefined): number | null {
  if (!s) return null
  const t = Date.parse(s + 'T00:00:00Z')
  return Number.isNaN(t) ? null : Math.floor(t / MS_PER_DAY)
}

/** dayToDate converts epoch days back to a Date at UTC midnight. */
export function dayToDate(day: number): Date {
  return new Date(day * MS_PER_DAY)
}

/**
 * barSpan is the node's full extent: the union of its planned window and its
 * rolled envelope. The two stay separate on the wire so the breach is
 * renderable; the BAR must span both or an overrun would be clipped.
 */
export function barSpan(node: GanttNode): DaySpan {
  const s = [parseDay(node.planned?.start), parseDay(node.rolled?.start)].filter(
    (v): v is number => v !== null,
  )
  const e = [parseDay(node.planned?.end), parseDay(node.rolled?.end)].filter(
    (v): v is number => v !== null,
  )
  return {
    start: s.length ? Math.min(...s) : null,
    end: e.length ? Math.max(...e) : null,
  }
}

/** forestSpan is the envelope over a set of roots, padded slightly so bars
 * never touch the chart edge. Empty forest yields null. */
export function forestSpan(roots: GanttNode[]): { start: number; end: number } | null {
  let start: number | null = null
  let end: number | null = null
  for (const r of roots) {
    const s = barSpan(r)
    if (s.start !== null && (start === null || s.start < start)) start = s.start
    if (s.end !== null && (end === null || s.end > end)) end = s.end
  }
  if (start === null || end === null) return null
  const pad = Math.max(Math.round((end - start) * 0.04), 2)
  return { start: start - pad, end: end + pad }
}

/** pct positions a day on the axis as a 0-100 percentage. */
export function pct(day: number, axis: { start: number; end: number }): number {
  const span = Math.max(axis.end - axis.start, 1)
  return ((day - axis.start) / span) * 100
}

/** An axis tick: position day plus a short label. */
export interface GanttTick {
  day: number
  label: string
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

/** MAX_TICKS caps label density: past it, adjacent labels overlap into an
 * unreadable smear, so periods are emitted at a stride instead. */
const MAX_TICKS = 20

/** isoWeek returns the ISO 8601 week number for an epoch day (UTC). */
export function isoWeek(day: number): number {
  const d = dayToDate(day)
  // Shift to the Thursday of this week; its year is the ISO year.
  const target = new Date(d)
  target.setUTCDate(d.getUTCDate() + 3 - ((d.getUTCDay() + 6) % 7))
  const yearStart = Date.UTC(target.getUTCFullYear(), 0, 1)
  return Math.ceil(((target.getTime() - yearStart) / MS_PER_DAY + 1) / 7)
}

/**
 * ticksFor lays out period boundaries across the axis for a zoom level.
 *
 * Density is bounded: when the span holds more periods than MAX_TICKS, every
 * Nth period is emitted instead — a two-year axis at week zoom would
 * otherwise draw ~104 labels into each other. Weeks are labelled with ISO
 * week numbers (the planning convention, and far shorter than dates); months
 * carry the year at January so a multi-year axis stays unambiguous.
 */
export function ticksFor(axis: { start: number; end: number }, zoom: GanttZoom): GanttTick[] {
  const out: GanttTick[] = []
  const d = dayToDate(axis.start)
  const cur = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1))
  if (zoom === 'quarter') {
    cur.setUTCMonth(Math.floor(cur.getUTCMonth() / 3) * 3)
  } else if (zoom === 'week') {
    const wd = dayToDate(axis.start)
    // back up to Monday; wd is UTC midnight and whole-day arithmetic keeps it there
    cur.setTime(wd.getTime() - ((wd.getUTCDay() + 6) % 7) * MS_PER_DAY)
  }

  const spanDays = axis.end - axis.start + 1
  const periodDays = zoom === 'quarter' ? 91 : zoom === 'month' ? 30 : 7
  const stride = Math.max(1, Math.ceil(spanDays / periodDays / MAX_TICKS))

  const endMs = (axis.end + 1) * MS_PER_DAY
  let guard = 0
  let i = 0
  while (cur.getTime() < endMs && guard++ < 800) {
    const day = Math.floor(cur.getTime() / MS_PER_DAY)
    // January is always emitted at month zoom: it carries the year, and a
    // stride that happened to skip it would leave a multi-year axis with no
    // year anchor at all.
    const emit = i % stride === 0 || (zoom === 'month' && cur.getUTCMonth() === 0)
    i++
    if (zoom === 'quarter') {
      if (emit) {
        out.push({
          day,
          label: `Q${Math.floor(cur.getUTCMonth() / 3) + 1} '${String(cur.getUTCFullYear()).slice(2)}`,
        })
      }
      cur.setUTCMonth(cur.getUTCMonth() + 3)
    } else if (zoom === 'month') {
      if (emit) {
        const m = cur.getUTCMonth()
        const label = m === 0 ? `${MONTHS[m]} '${String(cur.getUTCFullYear()).slice(2)}` : MONTHS[m]
        out.push({ day, label })
      }
      cur.setUTCMonth(cur.getUTCMonth() + 1)
    } else {
      if (emit) {
        out.push({ day, label: `W${isoWeek(day)}` })
      }
      cur.setTime(cur.getTime() + 7 * MS_PER_DAY)
    }
  }
  return out.filter((t) => t.day >= axis.start && t.day <= axis.end)
}

/**
 * flattenRows walks the tree into outline rows. A node's children render when
 * its depth is inside defaultDepth OR the user expanded it in place; both
 * navigation modes (twisty and drill) share this one walk.
 */
export function flattenRows(
  roots: GanttNode[],
  defaultDepth: number,
  expanded: ReadonlySet<string>,
): GanttRow[] {
  const rows: GanttRow[] = []
  const walk = (nodes: GanttNode[], depth: number, indent: number) => {
    for (const node of nodes) {
      rows.push({ node, indent })
      const children = node.children ?? []
      if (children.length && (depth + 1 < defaultDepth || expanded.has(node.id))) {
        walk(children, depth + 1, indent + 1)
      }
    }
  }
  walk(roots, 0, 0)
  return rows
}

/** findNode locates a node by id anywhere in the forest, for drill re-rooting. */
export function findNode(roots: GanttNode[], id: string): GanttNode | null {
  for (const r of roots) {
    if (r.id === id) return r
    const hit = findNode(r.children ?? [], id)
    if (hit) return hit
  }
  return null
}

/** isRowExpanded mirrors flattenRows' expansion rule for the twisty glyph. */
export function isRowExpanded(
  row: GanttRow,
  defaultDepth: number,
  expanded: ReadonlySet<string>,
): boolean {
  return row.indent + 1 < defaultDepth || expanded.has(row.node.id)
}
