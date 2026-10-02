/**
 * Pure day arithmetic for the calendar grid.
 *
 * A `CalendarDay` is timezone-free: `{ year: 2026, month: 3, day: 1 }` is
 * 1 March everywhere on earth. Everything here is a pure function over those,
 * so the whole class of off-by-one-day defects is a table of cases rather than
 * something only reachable through a mounted component.
 *
 * # What is deliberately absent
 *
 * There is no conversion from a stored value to a day. Deciding which cell
 * `2026-03-01T00:30:00Z` belongs to needs a display timezone, and it is the
 * app that knows both the timezone and the storage format. Putting that here
 * would make the library either guess the browser's zone — which disagrees with
 * a configured display zone, placing an event in a cell whose printed time
 * contradicts its position — or take a dependency on a timezone library for a
 * decision it is not entitled to make.
 *
 * So the library owns the grid, which needs no timezone, and the app owns the
 * mapping onto it. `RlCalendarGrid` is given days and events, never instants.
 */

/** A calendar day, timezone-free. The identity of a grid cell. */
export interface CalendarDay {
  year: number
  /** 1-12, not the 0-based month a JS `Date` uses. */
  month: number
  day: number
}

export type WeekStart = 'monday' | 'sunday'
export type CalendarView = 'month' | 'week'

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** `YYYY-MM-DD` for a day, for use as a list key. */
export function dayKey(d: CalendarDay): string {
  return `${d.year}-${pad2(d.month)}-${pad2(d.day)}`
}

/** Parse a `YYYY-MM-DD` key back into a day. Returns null when malformed. */
export function dayFromKey(key: string): CalendarDay | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(key)
  if (!m) return null
  const [year, month, day] = [Number(m[1]), Number(m[2]), Number(m[3])]
  // Round-trip through Date to reject 2026-02-30, which the regex accepts.
  const js = new Date(year, month - 1, day)
  if (js.getFullYear() !== year || js.getMonth() + 1 !== month || js.getDate() !== day) {
    return null
  }
  return { year, month, day }
}

export function sameDay(a: CalendarDay, b: CalendarDay): boolean {
  return a.year === b.year && a.month === b.month && a.day === b.day
}

/** Chronological ordering of two days, for `sort`. */
export function compareDays(a: CalendarDay, b: CalendarDay): number {
  if (a.year !== b.year) return a.year - b.year
  if (a.month !== b.month) return a.month - b.month
  return a.day - b.day
}

/**
 * Add a whole number of days to a calendar day.
 *
 * Uses `Date`'s own month and year rollover rather than millisecond
 * arithmetic: `+86400000` is not "tomorrow" on a DST transition day, which is
 * 23 or 25 hours long.
 */
export function addDays(d: CalendarDay, n: number): CalendarDay {
  const js = new Date(d.year, d.month - 1, d.day + n)
  return { year: js.getFullYear(), month: js.getMonth() + 1, day: js.getDate() }
}

/** Whole days from `a` to `b`, negative when `b` precedes `a`. */
export function daysBetween(a: CalendarDay, b: CalendarDay): number {
  // Both sides are UTC midnights of a timezone-free date, so this subtraction
  // never straddles a DST offset and the division is exact.
  const ms = Date.UTC(b.year, b.month - 1, b.day) - Date.UTC(a.year, a.month - 1, a.day)
  return Math.round(ms / 86_400_000)
}

/** 0 = Sunday … 6 = Saturday, for a timezone-free day. */
function weekdayOf(d: CalendarDay): number {
  return new Date(d.year, d.month - 1, d.day).getDay()
}

/** The first day of the week containing `d`, under a week-start rule. */
export function startOfWeek(d: CalendarDay, weekStart: WeekStart = 'monday'): CalendarDay {
  const wd = weekdayOf(d)
  return addDays(d, -(weekStart === 'monday' ? (wd + 6) % 7 : wd))
}

/** Seven consecutive days beginning at `anchor`'s week start. */
export function weekGrid(anchor: CalendarDay, weekStart: WeekStart = 'monday'): CalendarDay[] {
  const first = startOfWeek(anchor, weekStart)
  return Array.from({ length: 7 }, (_, i) => addDays(first, i))
}

/**
 * The month grid containing `anchor`: whole weeks, so it starts on or before
 * the 1st and ends on or after the last day of the month.
 *
 * Its length varies (28, 35 or 42 days) rather than being padded to a fixed
 * six rows. A fixed height would sometimes show a trailing week belonging
 * entirely to the next month, which reads as a rendering fault.
 */
export function monthGrid(anchor: CalendarDay, weekStart: WeekStart = 'monday'): CalendarDay[] {
  const first: CalendarDay = { year: anchor.year, month: anchor.month, day: 1 }
  const start = startOfWeek(first, weekStart)

  const lastDayOfMonth = new Date(anchor.year, anchor.month, 0).getDate()
  const last: CalendarDay = { year: anchor.year, month: anchor.month, day: lastDayOfMonth }
  const end = addDays(startOfWeek(last, weekStart), 6)

  const total = daysBetween(start, end) + 1
  return Array.from({ length: total }, (_, i) => addDays(start, i))
}

/** Whether a cell belongs to the anchor's own month, rather than a spill day. */
export function isSameMonth(d: CalendarDay, anchor: CalendarDay): boolean {
  return d.year === anchor.year && d.month === anchor.month
}

/** The days a view covers, given its anchor. */
export function visibleDays(
  view: CalendarView,
  anchor: CalendarDay,
  weekStart: WeekStart = 'monday',
): CalendarDay[] {
  return view === 'month' ? monthGrid(anchor, weekStart) : weekGrid(anchor, weekStart)
}

/**
 * Move the anchor one period forward (`+1`) or back (`-1`).
 *
 * A month step clamps to the last day of the target month, so 31 January
 * forward lands on 28 or 29 February rather than rolling into March, which
 * would skip the month the user asked for.
 */
export function shiftAnchor(
  view: CalendarView,
  anchor: CalendarDay,
  delta: number,
): CalendarDay {
  if (view === 'week') return addDays(anchor, 7 * delta)

  const targetMonth = anchor.month - 1 + delta
  const year = anchor.year + Math.floor(targetMonth / 12)
  const month = ((targetMonth % 12) + 12) % 12
  const lastDay = new Date(year, month + 1, 0).getDate()
  return { year, month: month + 1, day: Math.min(anchor.day, lastDay) }
}

/**
 * Weekday header labels in grid order, from the platform's own locale data.
 *
 * Derived rather than hard-coded so a Dutch app reads "ma di wo" without
 * passing a list in, and ordered by `weekStart` so the labels cannot fall out
 * of step with the columns they head.
 */
export function weekdayLabels(
  weekStart: WeekStart = 'monday',
  locale?: string,
  format: 'short' | 'long' | 'narrow' = 'short',
): string[] {
  const fmt = new Intl.DateTimeFormat(locale, { weekday: format })
  // 4 January 1970 was a Sunday; offsetting from it gives each weekday once.
  return Array.from({ length: 7 }, (_, i) =>
    fmt.format(new Date(1970, 0, 4 + i + (weekStart === 'monday' ? 1 : 0))),
  )
}
