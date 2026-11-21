import { describe, it, expect } from 'vitest'
import {
  addDays,
  applyDayDelta,
  compareDays,
  dayFromKey,
  dayKey,
  daysBetween,
  eventDay,
  eventMinutes,
  isSameMonth,
  monthGrid,
  shiftAnchor,
  startOfWeek,
  todayIn,
  weekGrid,
  windowBounds,
  type CalendarDay,
} from './calendarGrid'

const d = (year: number, month: number, day: number): CalendarDay => ({ year, month, day })

describe('dayKey / dayFromKey', () => {
  it('round-trips a day', () => {
    expect(dayKey(d(2026, 8, 22))).toBe('2026-08-22')
    expect(dayFromKey('2026-08-22')).toEqual(d(2026, 8, 22))
  })

  it('zero-pads single digits', () => {
    expect(dayKey(d(2026, 1, 5))).toBe('2026-01-05')
  })

  it('rejects an overflow date rather than rolling it over', () => {
    // 2026-13-45 would silently become 2027-02-14 via the Date constructor.
    expect(dayFromKey('2026-13-45')).toBeNull()
  })
})

describe('eventDay', () => {
  // An all-day date is a calendar date: the same day in every zone. This is the
  // rule that breaks if the value is ever pushed through a timezone.
  it.each([
    ['UTC'],
    ['America/New_York'],
    ['Asia/Tokyo'],
    ['Pacific/Kiritimati'],
  ])('places an all-day value on its literal date in %s', (tz) => {
    expect(eventDay('2026-03-01', 'date', tz)).toEqual(d(2026, 3, 1))
  })

  // A timed value is an instant, so its day depends on the display zone. This
  // is the invariant that keeps a chip's cell agreeing with its printed time.
  it.each([
    ['UTC', d(2026, 3, 1)],
    ['Europe/Amsterdam', d(2026, 3, 1)], // +01:00 → 01:30 same day
    ['America/New_York', d(2026, 2, 28)], // −05:00 → 19:30 previous day
    ['Pacific/Kiritimati', d(2026, 3, 1)], // +14:00 → 14:30 same day
  ])('places a timed value by display timezone: %s', (tz, want) => {
    expect(eventDay('2026-03-01T00:30:00Z', 'datetime', tz)).toEqual(want)
  })

  it('puts the same instant in different cells at the extremes of the zone range', () => {
    const instant = '2026-08-22T12:00:00Z'
    const east = eventDay(instant, 'datetime', 'Pacific/Kiritimati') // +14
    const west = eventDay(instant, 'datetime', 'Pacific/Niue') // −11
    expect(east).toEqual(d(2026, 8, 23))
    expect(west).toEqual(d(2026, 8, 22))
  })

  it('returns null for empty or unparseable values so the event is skipped', () => {
    expect(eventDay('', 'date', 'UTC')).toBeNull()
    expect(eventDay(null, 'date', 'UTC')).toBeNull()
    expect(eventDay('not-a-date', 'datetime', 'UTC')).toBeNull()
  })
})

describe('eventMinutes', () => {
  it('reports minutes past midnight in the display timezone', () => {
    expect(eventMinutes('2026-08-22T09:30:00Z', 'datetime', 'UTC')).toBe(570)
    expect(eventMinutes('2026-08-22T09:30:00Z', 'datetime', 'Europe/Amsterdam')).toBe(690) // +02:00 in Aug
  })

  it('has no time for an all-day value', () => {
    expect(eventMinutes('2026-08-22', 'date', 'UTC')).toBeNull()
  })
})

describe('addDays / daysBetween', () => {
  it.each([
    [d(2026, 8, 22), 1, d(2026, 8, 23)],
    [d(2026, 8, 31), 1, d(2026, 9, 1)],
    [d(2026, 12, 31), 1, d(2027, 1, 1)],
    [d(2026, 1, 1), -1, d(2025, 12, 31)],
    [d(2028, 2, 28), 1, d(2028, 2, 29)], // leap year
    [d(2026, 2, 28), 1, d(2026, 3, 1)], // non-leap
  ])('adds across boundaries: %o + %i', (from, n, want) => {
    expect(addDays(from, n)).toEqual(want)
  })

  // A DST day is 23 or 25 hours, so millisecond arithmetic would drift here.
  it('crosses a DST boundary by whole days', () => {
    expect(addDays(d(2026, 3, 28), 1)).toEqual(d(2026, 3, 29)) // EU spring forward
    expect(addDays(d(2026, 10, 24), 1)).toEqual(d(2026, 10, 25)) // EU fall back
  })

  it('measures whole days between dates', () => {
    expect(daysBetween(d(2026, 8, 22), d(2026, 8, 25))).toBe(3)
    expect(daysBetween(d(2026, 8, 25), d(2026, 8, 22))).toBe(-3)
    expect(daysBetween(d(2026, 3, 28), d(2026, 3, 29))).toBe(1) // spans DST
    expect(daysBetween(d(2026, 2, 28), d(2026, 3, 1))).toBe(1)
  })
})

describe('compareDays', () => {
  it('orders chronologically across all three fields', () => {
    expect(compareDays(d(2026, 1, 1), d(2027, 1, 1))).toBeLessThan(0)
    expect(compareDays(d(2026, 5, 1), d(2026, 4, 1))).toBeGreaterThan(0)
    expect(compareDays(d(2026, 5, 2), d(2026, 5, 3))).toBeLessThan(0)
    expect(compareDays(d(2026, 5, 2), d(2026, 5, 2))).toBe(0)
  })
})

describe('startOfWeek / weekGrid', () => {
  // 2026-08-22 is a Saturday.
  it('honours a Monday week start', () => {
    expect(startOfWeek(d(2026, 8, 22), 'monday')).toEqual(d(2026, 8, 17))
  })

  it('honours a Sunday week start', () => {
    expect(startOfWeek(d(2026, 8, 22), 'sunday')).toEqual(d(2026, 8, 16))
  })

  it('leaves a day that is already the week start alone', () => {
    expect(startOfWeek(d(2026, 8, 17), 'monday')).toEqual(d(2026, 8, 17))
    expect(startOfWeek(d(2026, 8, 16), 'sunday')).toEqual(d(2026, 8, 16))
  })

  it('produces seven consecutive days', () => {
    const week = weekGrid(d(2026, 8, 22), 'monday')
    expect(week).toHaveLength(7)
    expect(week[0]).toEqual(d(2026, 8, 17))
    expect(week[6]).toEqual(d(2026, 8, 23))
  })
})

describe('monthGrid', () => {
  it('covers whole weeks around the month', () => {
    // August 2026: 1st is a Saturday, 31st a Monday.
    const grid = monthGrid(d(2026, 8, 15), 'monday')
    expect(grid[0]).toEqual(d(2026, 7, 27)) // Monday before the 1st
    expect(grid[grid.length - 1]).toEqual(d(2026, 9, 6)) // Sunday after the 31st
    expect(grid.length % 7).toBe(0)
  })

  it.each([
    [2026, 8, 'monday'],
    [2026, 8, 'sunday'],
    [2026, 2, 'monday'],
    [2028, 2, 'monday'], // leap February
    [2026, 5, 'monday'],
    [2027, 1, 'sunday'],
  ] as const)('always spans whole weeks: %i-%i (%s)', (year, month, weekStart) => {
    const grid = monthGrid(d(year, month, 1), weekStart)
    expect(grid.length % 7).toBe(0)
    // Every day of the month must be present.
    const lastDay = new Date(year, month, 0).getDate()
    for (let day = 1; day <= lastDay; day++) {
      expect(grid.some((g) => g.year === year && g.month === month && g.day === day)).toBe(true)
    }
  })

  it('includes 29 February in a leap year', () => {
    const grid = monthGrid(d(2028, 2, 1), 'monday')
    expect(grid.some((g) => g.month === 2 && g.day === 29)).toBe(true)
  })

  it('does not pad to a fixed six rows when five suffice', () => {
    // February 2027 starts on a Monday and has 28 days: exactly four weeks.
    const grid = monthGrid(d(2027, 2, 1), 'monday')
    expect(grid).toHaveLength(28)
  })
})

describe('isSameMonth', () => {
  it('distinguishes spill days from the anchor month', () => {
    expect(isSameMonth(d(2026, 8, 15), d(2026, 8, 1))).toBe(true)
    expect(isSameMonth(d(2026, 7, 31), d(2026, 8, 1))).toBe(false)
    expect(isSameMonth(d(2025, 8, 15), d(2026, 8, 1))).toBe(false)
  })
})

describe('shiftAnchor', () => {
  it('moves a week view by seven days', () => {
    expect(shiftAnchor('week', d(2026, 8, 22), 1)).toEqual(d(2026, 8, 29))
    expect(shiftAnchor('week', d(2026, 8, 22), -1)).toEqual(d(2026, 8, 15))
  })

  it('moves a month view by one month', () => {
    expect(shiftAnchor('month', d(2026, 8, 15), 1)).toEqual(d(2026, 9, 15))
    expect(shiftAnchor('month', d(2026, 8, 15), -1)).toEqual(d(2026, 7, 15))
  })

  it('rolls across a year boundary', () => {
    expect(shiftAnchor('month', d(2026, 12, 15), 1)).toEqual(d(2027, 1, 15))
    expect(shiftAnchor('month', d(2026, 1, 15), -1)).toEqual(d(2025, 12, 15))
  })

  // Without clamping, 31 January + 1 month rolls into 3 March.
  it('clamps to the last day of a shorter target month', () => {
    expect(shiftAnchor('month', d(2026, 1, 31), 1)).toEqual(d(2026, 2, 28))
    expect(shiftAnchor('month', d(2028, 1, 31), 1)).toEqual(d(2028, 2, 29))
    expect(shiftAnchor('month', d(2026, 3, 31), -1)).toEqual(d(2026, 2, 28))
  })
})

describe('windowBounds', () => {
  it('is half-open: lt is midnight after the last visible day', () => {
    const { gte, lt } = windowBounds('week', d(2026, 8, 22), 'monday', 'UTC')
    expect(gte).toBe('2026-08-17T00:00:00.000Z')
    // Monday 17th + 7 days = Monday 24th, exclusive.
    expect(lt).toBe('2026-08-24T00:00:00.000Z')
  })

  // A `lte` bound of the last day would drop everything after its midnight;
  // this is the case that guards against that.
  it('includes a late-evening event on the final visible day', () => {
    const { lt } = windowBounds('week', d(2026, 8, 22), 'monday', 'UTC')
    expect('2026-08-23T23:30:00.000Z' < lt).toBe(true)
  })

  it('expresses bounds as instants in the display timezone', () => {
    // Midnight in Amsterdam (+02:00 in August) is 22:00Z the previous day.
    const { gte } = windowBounds('week', d(2026, 8, 22), 'monday', 'Europe/Amsterdam')
    expect(gte).toBe('2026-08-16T22:00:00.000Z')
  })

  it('widens the lower bound by the past pad, leaving the upper bound alone', () => {
    const plain = windowBounds('week', d(2026, 8, 22), 'monday', 'UTC')
    const padded = windowBounds('week', d(2026, 8, 22), 'monday', 'UTC', 31)
    expect(padded.gte).toBe('2026-07-17T00:00:00.000Z')
    expect(padded.lt).toBe(plain.lt)
  })
})

describe('todayIn', () => {
  it('reports the day in the display timezone, not the runner timezone', () => {
    const instant = new Date('2026-08-22T12:00:00Z')
    expect(todayIn('Pacific/Kiritimati', instant)).toEqual(d(2026, 8, 23))
    expect(todayIn('Pacific/Niue', instant)).toEqual(d(2026, 8, 22))
  })
})

describe('applyDayDelta', () => {
  it('moves an all-day value by whole days', () => {
    expect(applyDayDelta('2026-08-22', 'date', 1, 'UTC')).toBe('2026-08-23')
    expect(applyDayDelta('2026-08-31', 'date', 1, 'UTC')).toBe('2026-09-01')
    expect(applyDayDelta('2026-08-22', 'date', -3, 'UTC')).toBe('2026-08-19')
  })

  it('is a no-op for a zero delta', () => {
    expect(applyDayDelta('2026-08-22', 'date', 0, 'UTC')).toBe('2026-08-22')
  })

  it('moves a timed value while preserving its wall-clock time', () => {
    const moved = applyDayDelta('2026-08-22T09:00:00Z', 'datetime', 1, 'UTC')
    expect(moved).toBe('2026-08-23T09:00:00.000Z')
  })

  // The defect this guards: adding 86_400_000ms across a spring-forward
  // boundary turns a 09:00 meeting into an 08:00 one.
  it('preserves wall-clock time across EU spring forward', () => {
    // 2026-03-29 is the EU DST transition; Amsterdam goes +01:00 → +02:00.
    const before = '2026-03-28T08:00:00Z' // 09:00 Amsterdam
    const moved = applyDayDelta(before, 'datetime', 1, 'Europe/Amsterdam')
    // Still 09:00 Amsterdam, now at +02:00 → 07:00Z.
    expect(moved).toBe('2026-03-29T07:00:00.000Z')
  })

  it('preserves wall-clock time across EU fall back', () => {
    // 2026-10-25 is the EU return to +01:00.
    const before = '2026-10-24T07:00:00Z' // 09:00 Amsterdam (+02:00)
    const moved = applyDayDelta(before, 'datetime', 1, 'Europe/Amsterdam')
    expect(moved).toBe('2026-10-25T08:00:00.000Z') // 09:00 at +01:00
  })

  it('preserves wall-clock time across US spring forward', () => {
    // 2026-03-08 is the US transition; New York goes −05:00 → −04:00.
    const before = '2026-03-07T14:00:00Z' // 09:00 New York
    const moved = applyDayDelta(before, 'datetime', 1, 'America/New_York')
    expect(moved).toBe('2026-03-08T13:00:00.000Z') // 09:00 at −04:00
  })

  it('preserves wall-clock time across southern-hemisphere DST', () => {
    // Sydney leaves DST at 03:00 local on 2026-04-05, +11:00 → +10:00, so
    // 09:00 on the 5th is already on the far side of the transition.
    const before = '2026-04-04T22:00:00Z' // 09:00 Sydney on the 4th (+11:00)
    const moved = applyDayDelta(before, 'datetime', 1, 'Australia/Sydney')
    expect(moved).toBe('2026-04-05T22:00:00.000Z') // 09:00 on the 5th (+10:00)
  })

  it('returns null rather than a corrupted value for bad input', () => {
    expect(applyDayDelta('', 'date', 1, 'UTC')).toBeNull()
    expect(applyDayDelta(null, 'datetime', 1, 'UTC')).toBeNull()
    expect(applyDayDelta('nonsense', 'datetime', 1, 'UTC')).toBeNull()
  })

  it('keeps a start and end the same distance apart', () => {
    // The duration-preservation half of AC#6, across a DST boundary where
    // elapsed time and wall-clock time diverge.
    const start = '2026-03-28T08:00:00Z' // 09:00 Amsterdam
    const end = '2026-03-28T09:00:00Z' // 10:00 Amsterdam
    const movedStart = applyDayDelta(start, 'datetime', 1, 'Europe/Amsterdam')
    const movedEnd = applyDayDelta(end, 'datetime', 1, 'Europe/Amsterdam')
    expect(movedStart).toBe('2026-03-29T07:00:00.000Z') // 09:00
    expect(movedEnd).toBe('2026-03-29T08:00:00.000Z') // 10:00 — still one hour
  })
})
