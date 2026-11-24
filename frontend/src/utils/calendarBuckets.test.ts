import { describe, expect, it } from 'vitest'
import {
  calendarBucketDate,
  calendarBucketOf,
  calendarBucketTitle,
  compareCalendarBuckets,
} from '@/utils/calendarBuckets'
import type { CalendarBuckets } from '@/types'

const AMS = 'Europe/Amsterdam'
// Tuesday 2026-11-24, 10:00 in Amsterdam.
const now = new Date('2026-11-24T09:00:00Z')

const of = (v: unknown, b: CalendarBuckets, at = now, tz = AMS) =>
  calendarBucketOf(v, b, at, tz, true)

describe('calendarBucketOf', () => {
  it('keys a week by its Monday across a year boundary', () => {
    const week: CalendarBuckets = { by: 'week' }
    const ids = ['2026-12-28', '2026-12-31', '2027-01-01', '2027-01-03'].map((d) => of(d, week).id)
    expect(new Set(ids).size).toBe(1)
    expect(of('2027-01-04', week).id).not.toBe(ids[0])
    expect(calendarBucketTitle(of('2027-01-02', week), now, AMS, 'en-GB')).toBe('Week of 28 Dec')
  })

  it('splits by month and year', () => {
    expect(of('2026-11-01', { by: 'month' }).id).toBe(of('2026-11-30', { by: 'month' }).id)
    expect(of('2026-11-30', { by: 'month' }).id).not.toBe(of('2026-12-01', { by: 'month' }).id)
    expect(calendarBucketTitle(of('2026-03-05', { by: 'month' }), now, AMS, 'en-GB')).toBe(
      'March 2026'
    )
    expect(calendarBucketTitle(of('2025-03-05', { by: 'year' }), now, AMS)).toBe('2025')
  })

  it('takes the near window by day, then months, with overdue first', () => {
    const b: CalendarBuckets = { by: 'day', for: '7d', then: 'month' }
    expect(of('2026-11-23', b).fixed).toBe('overdue')
    expect(of('2026-11-24', b).fixed).toBe('today')
    expect(of('2026-11-25', b).fixed).toBe('tomorrow')
    expect(of('2026-11-30', b).unit).toBe('day')
    // Day 7 from today starts the partial first month section.
    expect(of('2026-12-01', b).unit).toBe('month')
    expect(of('2026-12-01', b).id).toBe(of('2026-12-31', b).id)
    expect(of('2026-12-01', { by: 'day', for: '7d' }).fixed).toBe('later')
    expect(of('', b).fixed).toBe('no_date')
    expect(of('2026-02-31', b).fixed).toBe('no_date')
  })

  it('reads a datetime on its day in the display zone across DST', () => {
    const b: CalendarBuckets = { by: 'day' }
    // 23:30 UTC on 28 March is 00:30 on 29 March in Amsterdam (after the switch).
    const at = calendarBucketOf('2026-03-28T23:30:00Z', b, now, AMS)
    expect(calendarBucketDate(at)).toBe('2026-03-29')
  })

  it('orders newest first and keeps no date last', () => {
    const b: CalendarBuckets = { by: 'month', order: 'desc' }
    const sorted = [of('2026-01-05', b), of('', b), of('2026-06-05', b)].sort((x, y) =>
      compareCalendarBuckets(x, y, true)
    )
    expect(sorted.map((s) => s.fixed ?? s.unit)).toEqual(['month', 'month', 'no_date'])
    expect(sorted[0].start).toBeGreaterThan(sorted[1].start)
  })

  it('prefills only single-day sections', () => {
    expect(calendarBucketDate(of('2026-11-27', { by: 'day' }))).toBe('2026-11-27')
    expect(calendarBucketDate(of('2026-11-27', { by: 'week' }))).toBeUndefined()
  })
})

describe('calendarBucketTitle', () => {
  it('names each unit and shows another year', () => {
    const title = (v: string, b: CalendarBuckets) =>
      calendarBucketTitle(of(v, b), now, AMS, 'en-GB')
    expect(title('2026-11-26', { by: 'day' })).toBe('Thu 26 Nov')
    expect(title('2027-01-07', { by: 'day' })).toBe('Thu, 7 Jan 2027')
    expect(title('2025-12-29', { by: 'week' })).toBe('Week of 29 Dec 2025')
    expect(title('2024-02-29', { by: 'month' })).toBe('February 2024')
  })

  it('keeps a date value on its day west of UTC', () => {
    const b = calendarBucketOf(
      '2026-11-30T00:00:00Z',
      { by: 'day' },
      now,
      'America/Los_Angeles',
      true
    )
    expect(calendarBucketDate(b)).toBe('2026-11-30')
  })
})
