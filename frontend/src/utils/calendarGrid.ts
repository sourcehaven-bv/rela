import { TZDate } from '@date-fns/tz'
import { parseDate } from './format'

/**
 * Pure date arithmetic for calendar views.
 *
 * Everything here is a pure function over strings and a timezone, deliberately:
 * the whole class of calendar defects — off-by-one days, DST drift, events in
 * the wrong cell — is timezone arithmetic, and arithmetic tested through a
 * mounted component is arithmetic that is barely tested at all. Keeping it here
 * makes those cases a table rather than a fixture.
 *
 * # The two kinds of date, and why they never mix
 *
 * A `date` property is a CALENDAR DATE: `2026-03-01` is 1 March everywhere on
 * earth. It must never be converted through a timezone.
 *
 * A `datetime` property is an INSTANT: `2026-03-01T00:30:00Z` is 1 March in UTC
 * but 28 February in New York. Which grid cell it belongs to is a function of
 * the DISPLAY timezone, not of the browser's zone and not of UTC.
 *
 * Mixing the two conversions is the classic calendar bug. A source is validated
 * at config load to be wholly one kind or the other.
 */

/** A calendar day, timezone-free. The identity of a grid cell. */
export interface CalendarDay {
  year: number
  /** 1-12, not the 0-based month a JS Date uses. */
  month: number
  day: number
}

export type DateKind = 'date' | 'datetime'
export type WeekStart = 'monday' | 'sunday'
export type CalendarView = 'month' | 'week'

/** `YYYY-MM-DD` for a day. This is also the storage form of a `date` property. */
export function dayKey(d: CalendarDay): string {
  return `${d.year}-${pad2(d.month)}-${pad2(d.day)}`
}

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** Parse a `YYYY-MM-DD` key back into a day. Returns null when malformed. */
export function dayFromKey(key: string): CalendarDay | null {
  const d = parseDate(key)
  if (isNaN(d.getTime())) return null
  return { year: d.getFullYear(), month: d.getMonth() + 1, day: d.getDate() }
}

/**
 * The day a stored value occupies on the grid.
 *
 * This is the single rule the rest of the calendar depends on: an all-day value
 * is its literal date, and a timed value is its calendar date IN THE DISPLAY
 * TIMEZONE. Reaching for `new Date(iso).getDate()` instead would silently use
 * the browser's zone, which disagrees with the app's configured display zone —
 * putting the event in a cell whose printed time contradicts its position, and
 * then feeding that wrong day into the drag delta.
 *
 * Nil: returns null for an empty or unparseable value; callers skip the event
 * rather than treating it as an error (an entity with no date is simply not on
 * the calendar).
 */
export function eventDay(value: string | null | undefined, kind: DateKind, tz: string): CalendarDay | null {
  if (!value) return null

  if (kind === 'date') {
    return dayFromKey(value)
  }

  const d = new TZDate(value, tz)
  if (isNaN(d.getTime())) return null
  return { year: d.getFullYear(), month: d.getMonth() + 1, day: d.getDate() }
}

/** Minutes past midnight of a timed value in the display timezone, for the
 * week-view hour axis. Returns null for all-day values, which have no time. */
export function eventMinutes(value: string | null | undefined, kind: DateKind, tz: string): number | null {
  if (!value || kind !== 'datetime') return null
  const d = new TZDate(value, tz)
  if (isNaN(d.getTime())) return null
  return d.getHours() * 60 + d.getMinutes()
}

export function sameDay(a: CalendarDay, b: CalendarDay): boolean {
  return a.year === b.year && a.month === b.month && a.day === b.day
}

/** Chronological ordering of two days. */
export function compareDays(a: CalendarDay, b: CalendarDay): number {
  if (a.year !== b.year) return a.year - b.year
  if (a.month !== b.month) return a.month - b.month
  return a.day - b.day
}

/**
 * Add a whole number of days to a calendar day.
 *
 * Uses Date's own month/year rollover rather than millisecond arithmetic:
 * `+86400000` is not "tomorrow" on a DST transition day, which is 23 or 25
 * hours long.
 */
export function addDays(d: CalendarDay, n: number): CalendarDay {
  const js = new Date(d.year, d.month - 1, d.day + n)
  return { year: js.getFullYear(), month: js.getMonth() + 1, day: js.getDate() }
}

/** Whole days from `a` to `b` (negative when b precedes a). */
export function daysBetween(a: CalendarDay, b: CalendarDay): number {
  // Both sides are UTC midnights of a timezone-free date, so this subtraction
  // never straddles a DST offset and the division is exact.
  const ms = Date.UTC(b.year, b.month - 1, b.day) - Date.UTC(a.year, a.month - 1, a.day)
  return Math.round(ms / 86_400_000)
}

/** The day `today` is, in the display timezone. */
export function todayIn(tz: string, now: Date = new Date()): CalendarDay {
  const d = new TZDate(now, tz)
  return { year: d.getFullYear(), month: d.getMonth() + 1, day: d.getDate() }
}

/** 0 = Sunday … 6 = Saturday, for a timezone-free day. */
function weekdayOf(d: CalendarDay): number {
  return new Date(d.year, d.month - 1, d.day).getDay()
}

/** How many days back the start of `d`'s week is, under a week-start rule. */
function offsetToWeekStart(d: CalendarDay, weekStart: WeekStart): number {
  const wd = weekdayOf(d)
  return weekStart === 'monday' ? (wd + 6) % 7 : wd
}

/** The first day of the week containing `d`. */
export function startOfWeek(d: CalendarDay, weekStart: WeekStart): CalendarDay {
  return addDays(d, -offsetToWeekStart(d, weekStart))
}

/** Seven consecutive days beginning at `d`'s week start. */
export function weekGrid(anchor: CalendarDay, weekStart: WeekStart): CalendarDay[] {
  const first = startOfWeek(anchor, weekStart)
  return Array.from({ length: 7 }, (_, i) => addDays(first, i))
}

/**
 * The month grid containing `anchor`: whole weeks, so it starts on or before
 * the 1st and ends on or after the last day of the month.
 *
 * Length varies (28, 35 or 42 days) rather than being padded to a fixed 6 rows:
 * a fixed height would show a trailing week belonging entirely to the next
 * month, which reads as a rendering bug.
 */
export function monthGrid(anchor: CalendarDay, weekStart: WeekStart): CalendarDay[] {
  const first: CalendarDay = { year: anchor.year, month: anchor.month, day: 1 }
  const start = startOfWeek(first, weekStart)

  const lastDay = new Date(anchor.year, anchor.month, 0).getDate()
  const last: CalendarDay = { year: anchor.year, month: anchor.month, day: lastDay }
  const end = addDays(startOfWeek(last, weekStart), 6)

  const total = daysBetween(start, end) + 1
  return Array.from({ length: total }, (_, i) => addDays(start, i))
}

/** Whether a grid cell belongs to the anchor's own month (vs. a spill day). */
export function isSameMonth(d: CalendarDay, anchor: CalendarDay): boolean {
  return d.year === anchor.year && d.month === anchor.month
}

/** The days a view covers, given its anchor. */
export function visibleDays(view: CalendarView, anchor: CalendarDay, weekStart: WeekStart): CalendarDay[] {
  return view === 'month' ? monthGrid(anchor, weekStart) : weekGrid(anchor, weekStart)
}

/** Move the anchor one period forward (`+1`) or back (`-1`). */
export function shiftAnchor(view: CalendarView, anchor: CalendarDay, delta: number): CalendarDay {
  if (view === 'week') return addDays(anchor, 7 * delta)
  // Clamp to the last day of the target month so 31 Jan +1 lands on 28/29 Feb
  // rather than rolling into March.
  const targetMonth = anchor.month - 1 + delta
  const y = anchor.year + Math.floor(targetMonth / 12)
  const m = ((targetMonth % 12) + 12) % 12
  const lastDay = new Date(y, m + 1, 0).getDate()
  return { year: y, month: m + 1, day: Math.min(anchor.day, lastDay) }
}

/**
 * The half-open instant range `[gte, lt)` to request for a view.
 *
 * Half-open, and expressed as instants rather than bare dates, because a `lte`
 * bound of `2026-08-31` means "≤ midnight" and silently drops every timed event
 * later that day. `pastPadDays` widens the lower bound for sources with an end
 * date, so an event that starts before the window but runs into it is still
 * fetched.
 */
export function windowBounds(
  view: CalendarView,
  anchor: CalendarDay,
  weekStart: WeekStart,
  tz: string,
  pastPadDays = 0
): { gte: string; lt: string } {
  const days = visibleDays(view, anchor, weekStart)
  const first = addDays(days[0], -pastPadDays)
  const afterLast = addDays(days[days.length - 1], 1)
  return { gte: startOfDayISO(first, tz), lt: startOfDayISO(afterLast, tz) }
}

/** Midnight of a day in the display timezone, as a UTC instant. */
export function startOfDayISO(d: CalendarDay, tz: string): string {
  const local = new TZDate(d.year, d.month - 1, d.day, 0, 0, 0, 0, tz)
  return new Date(local.getTime()).toISOString()
}

/**
 * Move a stored date/datetime value by a whole-day delta, preserving its
 * time-of-day.
 *
 * The wall clock is preserved, not the elapsed duration: dragging a 09:00
 * meeting across a spring-forward boundary must leave it at 09:00, not 08:00.
 * That is why the value is decomposed into local fields and recomposed, rather
 * than having a day's worth of milliseconds added to it.
 *
 * Nil: returns null when the value is empty or unparseable, so a caller can
 * abandon the write rather than sending a corrupted one.
 */
export function applyDayDelta(
  value: string | null | undefined,
  kind: DateKind,
  deltaDays: number,
  tz: string
): string | null {
  if (!value) return null

  if (kind === 'date') {
    const d = dayFromKey(value)
    return d ? dayKey(addDays(d, deltaDays)) : null
  }

  const src = new TZDate(value, tz)
  if (isNaN(src.getTime())) return null
  const moved = addDays(
    { year: src.getFullYear(), month: src.getMonth() + 1, day: src.getDate() },
    deltaDays
  )
  const rebuilt = new TZDate(
    moved.year,
    moved.month - 1,
    moved.day,
    src.getHours(),
    src.getMinutes(),
    src.getSeconds(),
    src.getMilliseconds(),
    tz
  )
  return new Date(rebuilt.getTime()).toISOString()
}
