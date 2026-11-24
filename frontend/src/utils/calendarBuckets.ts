/**
 * Calendar date sections for a list grouped with the mapping form of
 * `buckets:` (`{ by: month }`, `{ by: day, for: 7d, then: month }`).
 *
 * The time zone rule is the one utils/dateBuckets.ts documents for the
 * relative scheme: a value is reduced to a calendar day in the reader's
 * display zone, and every unit is derived from that day's number. No unit is
 * found by adding hours to a Date, so a 23- or 25-hour day cannot move a row
 * across a boundary.
 *
 * A week starts on Monday and is keyed by its Monday's day number, not by
 * ISO week and year: around 29 December those disagree with the calendar
 * year, and the Monday is the one key that never splits a week.
 */
import { intlLocale } from '@/i18n/locale'
import { t } from '@/i18n'
import {
  asDateValue,
  calendarDayOf,
  dayFromNumber,
  dayNumber,
  dayOfInstant,
  defaultBucketLabel,
} from '@/utils/dateBuckets'
import type { CalendarBuckets, CalendarFixedBucket, CalendarUnit, DateBucket } from '@/types'

/** One calendar section, before rows are attached. */
export interface CalendarBucket {
  /** Stable section id, such as `cal:month:20393`. */
  id: string
  /** Day number of the section's first day; orders the sections. */
  start: number
  /** Set for the fixed sections `labels:` can rename. */
  fixed?: CalendarFixedBucket
  unit?: CalendarUnit
  /**
   * The relative bucket whose row note fits this section (see bucketMeta):
   * the time for a single day, the weekday within a week, a short date for
   * anything longer.
   */
  meta: DateBucket
}

const DAY_MS = 86_400_000

/** Day number of the Monday of the week holding `day`. 1970-01-01 was a Thursday. */
function mondayOf(day: number): number {
  return day - ((((day + 3) % 7) + 7) % 7)
}

/** Day number of the first day of the `unit` holding day number `n`. */
function unitStart(n: number, unit: CalendarUnit): number {
  if (unit === 'day') return n
  if (unit === 'week') return mondayOf(n)
  const d = dayFromNumber(n)
  return dayNumber({ year: d.year, month: unit === 'month' ? d.month : 1, day: 1 })
}

/** The number of days in `for`, or 0 when it is unset or not `<n>d`. */
export function forDays(buckets: CalendarBuckets): number {
  const m = /^(\d+)d$/.exec(buckets.for ?? '')
  return m ? Number(m[1]) : 0
}

const FAR = Number.MAX_SAFE_INTEGER

function unitBucket(n: number, unit: CalendarUnit, today: number): CalendarBucket {
  const start = unitStart(n, unit)
  if (unit === 'day' && start === today)
    return { id: 'cal:today', start, fixed: 'today', unit, meta: 'today' }
  if (unit === 'day' && start === today + 1) {
    return { id: 'cal:tomorrow', start, fixed: 'tomorrow', unit, meta: 'today' }
  }
  const meta: DateBucket = unit === 'day' ? 'today' : unit === 'week' ? 'next_7_days' : 'later'
  return { id: `cal:${unit}:${start}`, start, unit, meta }
}

/**
 * The calendar section a stored date or datetime falls in, seen from `now`
 * in `tz`. `dateOnly` is true for a `date` property (see asDateValue).
 */
export function calendarBucketOf(
  value: unknown,
  buckets: CalendarBuckets,
  now: Date,
  tz: string,
  dateOnly = false
): CalendarBucket {
  const day = calendarDayOf(asDateValue(value, dateOnly), tz)
  if (!day) return { id: 'cal:no_date', start: FAR, fixed: 'no_date', meta: 'no_date' }
  const n = dayNumber(day)
  const today = dayNumber(dayOfInstant(now.getTime(), tz))
  const window = forDays(buckets)
  if (window === 0) return unitBucket(n, buckets.by, today)
  if (n < today) return { id: 'cal:overdue', start: -FAR, fixed: 'overdue', meta: 'overdue' }
  if (n < today + window) return unitBucket(n, 'day', today)
  if (!buckets.then) return { id: 'cal:later', start: FAR - 1, fixed: 'later', meta: 'later' }
  // The first `then` section may be partial: it holds only the days after
  // the window. It is still titled by its unit.
  return unitBucket(n, buckets.then, today)
}

/**
 * Orders sections: by start day, newest first under `order: desc`. "No
 * date" is last either way, since it is not a place on the calendar.
 */
// Overdue (-FAR) and Later (FAR - 1) exist only with `for`, which the server
// refuses with `desc`, so the sentinels never meet a descending sort.
export function compareCalendarBuckets(
  a: CalendarBucket,
  b: CalendarBucket,
  desc: boolean
): number {
  if (a.fixed === 'no_date' || b.fixed === 'no_date') {
    return (a.fixed === 'no_date' ? 1 : 0) - (b.fixed === 'no_date' ? 1 : 0)
  }
  return desc ? b.start - a.start : a.start - b.start
}

/** A section's title, unless `labels:` names its fixed key. */
export function calendarBucketTitle(
  bucket: CalendarBucket,
  now: Date,
  tz: string,
  locale?: string
): string {
  locale ??= intlLocale()
  if (bucket.fixed) return defaultBucketLabel(bucket.fixed)
  // Formatted from UTC noon in UTC, so the zone cannot move it a day.
  const date = new Date(bucket.start * DAY_MS + DAY_MS / 2)
  const thisYear = dayOfInstant(now.getTime(), tz).year
  const year = date.getUTCFullYear() !== thisYear ? ('numeric' as const) : undefined
  switch (bucket.unit) {
    case 'year':
      return String(date.getUTCFullYear())
    case 'month':
      return date.toLocaleDateString(locale, { month: 'long', year: 'numeric', timeZone: 'UTC' })
    case 'week':
      return t('list.bucket.weekOf', {
        date: date.toLocaleDateString(locale, {
          day: 'numeric',
          month: 'short',
          year,
          timeZone: 'UTC',
        }),
      })
    default:
      return date.toLocaleDateString(locale, {
        weekday: 'short',
        day: 'numeric',
        month: 'short',
        year,
        timeZone: 'UTC',
      })
  }
}

/**
 * The date a row added to a section should carry, as `YYYY-MM-DD`: only a
 * single-day section names one.
 */
export function calendarBucketDate(bucket: CalendarBucket): string | undefined {
  if (bucket.unit !== 'day') return undefined
  return new Date(bucket.start * DAY_MS).toISOString().slice(0, 10)
}
