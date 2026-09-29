/**
 * Relative date buckets for a list grouped with `group_by: {buckets: relative}`.
 *
 * A bucket is decided by CALENDAR DAY in the reader's display time zone, never
 * by elapsed hours: a task due at 09:00 tomorrow is "tomorrow" at 23:00 today,
 * though it is only ten hours away. So the value and "now" are both reduced to
 * a day in that zone first, and the buckets compare day numbers. Day numbers
 * are whole days since the epoch computed with `Date.UTC`, which has no DST, so
 * a 23- or 25-hour day cannot shift a boundary.
 *
 * The server does not compute these. A list `condition:` using
 * `days_between(entity.due, today())` evaluates `today()` in the server's zone,
 * so near midnight a reader outside that zone can see a row in "Today" that
 * the condition counts as tomorrow. The buckets follow the reader, because the
 * reader is who "today" is for.
 */
import { TZDate } from '@date-fns/tz'
import { NAIVE_DATETIME_RE } from '@/utils/format'
import type { DateBucket } from '@/types'

/**
 * The buckets in the order their sections appear. Ascending by date, which is
 * the order the server sorts the rows in, so each bucket's rows arrive
 * together. `internal/dataentryconfig` holds the same keys and a test pins the
 * two to each other.
 */
export const BUCKET_ORDER: readonly DateBucket[] = [
  'overdue',
  'today',
  'tomorrow',
  'next_7_days',
  'later',
  'no_date',
]

/** Section titles when the list's `labels:` does not name one. */
export const DEFAULT_BUCKET_LABELS: Readonly<Record<DateBucket, string>> = {
  overdue: 'Overdue',
  today: 'Today',
  tomorrow: 'Tomorrow',
  next_7_days: 'Next 7 days',
  later: 'Later',
  no_date: 'No date',
}

/** How far ahead `next_7_days` reaches, counting today as day 0. */
const NEXT_DAYS = 7

const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})$/

interface CalendarDay {
  year: number
  /** 1-based. */
  month: number
  day: number
}

function dayNumber(d: CalendarDay): number {
  return Date.UTC(d.year, d.month - 1, d.day) / 86_400_000
}

function dayOfInstant(epochMs: number, tz: string): CalendarDay {
  const zoned = new TZDate(epochMs, tz)
  return { year: zoned.getFullYear(), month: zoned.getMonth() + 1, day: zoned.getDate() }
}

/** Whether a stored value is a bare date rather than a moment in time. */
function isDateOnly(value: unknown): value is string {
  return typeof value === 'string' && DATE_ONLY_RE.test(value)
}

/**
 * The calendar day a stored value falls on in `tz`, or null when it holds no
 * usable date.
 *
 * A `date` value IS a calendar day and is taken as written in every zone, the
 * same rule `parseDate` applies for display. A `datetime` is an instant, so
 * its day depends on the zone; a naive datetime (no offset) is read as wall
 * clock time in `tz`, matching how `formatDatetime` shows it.
 */
export function calendarDayOf(value: unknown, tz: string): CalendarDay | null {
  if (typeof value !== 'string' || value === '') return null
  // A naive datetime is wall-clock time in the display zone, so its day is
  // its date part. Parsing it as an instant would read it in the runtime's
  // zone instead, which is not the display zone in general.
  const dateOnly = DATE_ONLY_RE.exec(NAIVE_DATETIME_RE.test(value) ? value.slice(0, 10) : value)
  if (dateOnly) {
    const day = { year: Number(dateOnly[1]), month: Number(dateOnly[2]), day: Number(dateOnly[3]) }
    // Reject an overflow such as 2026-02-31, which Date.UTC would roll over.
    const check = new Date(Date.UTC(day.year, day.month - 1, day.day))
    if (check.getUTCMonth() !== day.month - 1 || check.getUTCDate() !== day.day) return null
    return day
  }
  const instant = new Date(value).getTime()
  if (Number.isNaN(instant)) return null
  return dayOfInstant(instant, tz)
}

/**
 * Reduces a `date` property's value to its calendar day. The server may send
 * one as midnight UTC (`2026-09-29T00:00:00Z`) rather than `2026-09-29`, and
 * read as an instant that lands on the previous day west of UTC. Only the
 * property's type can tell the two apart, so the caller says which it is.
 */
function asDateValue(value: unknown, dateOnly: boolean): unknown {
  if (!dateOnly || typeof value !== 'string') return value
  return /^\d{4}-\d{2}-\d{2}/.test(value) ? value.slice(0, 10) : value
}

/**
 * Which bucket a stored date or datetime falls in, seen from `now` in `tz`.
 * An empty or unparseable value is `no_date`: it has no place on the calendar
 * either way, and hiding it would lose the row. `dateOnly` is true for a
 * `date` property; see asDateValue.
 */
export function bucketOf(value: unknown, now: Date, tz: string, dateOnly = false): DateBucket {
  const day = calendarDayOf(asDateValue(value, dateOnly), tz)
  if (!day) return 'no_date'
  const diff = dayNumber(day) - dayNumber(dayOfInstant(now.getTime(), tz))
  if (diff < 0) return 'overdue'
  if (diff === 0) return 'today'
  if (diff === 1) return 'tomorrow'
  if (diff <= NEXT_DAYS) return 'next_7_days'
  return 'later'
}

/**
 * The date a new row added to a bucket should carry, as `YYYY-MM-DD`, or
 * undefined for a bucket that spans more than one day. Only today and
 * tomorrow name a single day.
 */
export function bucketDate(bucket: DateBucket, now: Date, tz: string): string | undefined {
  const offset = bucket === 'today' ? 0 : bucket === 'tomorrow' ? 1 : undefined
  if (offset === undefined) return undefined
  const today = dayOfInstant(now.getTime(), tz)
  const target = new Date(Date.UTC(today.year, today.month - 1, today.day + offset))
  return target.toISOString().slice(0, 10)
}

/**
 * A short note on when, for a row whose bucket already says roughly when: the
 * time for today, the weekday within the coming week, a short date otherwise.
 *
 * A bare date has no time, so a date-valued row in "Today" gets no note: the
 * heading has said everything there is.
 */
export function bucketMeta(
  raw: unknown,
  bucket: DateBucket,
  tz: string,
  locale?: string,
  dateOnly = false
): string | undefined {
  const value = asDateValue(raw, dateOnly)
  const day = calendarDayOf(value, tz)
  if (!day || bucket === 'no_date') return undefined
  if (bucket === 'today') {
    if (isDateOnly(value)) return undefined
    // A naive datetime already is wall-clock time in the display zone, so it
    // is formatted as written: read as UTC and shown in UTC.
    if (NAIVE_DATETIME_RE.test(value as string)) {
      const wall = new Date(`${(value as string).slice(0, 19)}Z`)
      return wall.toLocaleTimeString(locale, { timeStyle: 'short', timeZone: 'UTC' })
    }
    return new Date(value as string).toLocaleTimeString(locale, { timeStyle: 'short', timeZone: tz })
  }
  // Formatted from the calendar day at UTC noon in UTC, so the zone cannot
  // move it onto a neighbouring day.
  const noon = new Date(Date.UTC(day.year, day.month - 1, day.day, 12))
  if (bucket === 'tomorrow' || bucket === 'next_7_days') {
    return noon.toLocaleDateString(locale, { weekday: 'short', timeZone: 'UTC' })
  }
  return noon.toLocaleDateString(locale, { month: 'short', day: 'numeric', timeZone: 'UTC' })
}
