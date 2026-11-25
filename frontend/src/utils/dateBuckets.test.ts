import { describe, it, expect } from 'vitest'
import { BUCKET_ORDER, WEEK_BUCKET_ORDER, bucketDate, bucketMeta, bucketOf } from './dateBuckets'

const AMS = 'Europe/Amsterdam'
const NYC = 'America/New_York'

describe('bucketOf', () => {
  // Noon on Wednesday 2026-06-10 in Amsterdam (10:00 UTC).
  const now = new Date('2026-06-10T10:00:00Z')

  it.each([
    ['2026-06-09', 'overdue'],
    ['2026-06-10', 'today'],
    ['2026-06-11', 'tomorrow'],
    ['2026-06-12', 'next_7_days'],
    ['2026-06-17', 'next_7_days'],
    ['2026-06-18', 'later'],
    ['', 'no_date'],
    [undefined, 'no_date'],
    ['not a date', 'no_date'],
    ['2026-02-31', 'no_date'],
  ])('puts date %s in %s', (value, bucket) => {
    expect(bucketOf(value, now, AMS)).toBe(bucket)
  })

  it('reads a date sent as midnight UTC as its calendar day', () => {
    // As an instant this is still the 9th in New York.
    expect(bucketOf('2026-06-10T00:00:00Z', now, NYC)).toBe('overdue')
    expect(bucketOf('2026-06-10T00:00:00Z', now, NYC, true)).toBe('today')
  })

  it('takes a date as written in every zone', () => {
    // A bare date is a calendar day, not an instant: 2026-06-10 is today in
    // Amsterdam and in New York alike, even though it is still the 9th in
    // some zones at the UTC instant it would parse to.
    expect(bucketOf('2026-06-10', now, AMS)).toBe('today')
    expect(bucketOf('2026-06-10', now, NYC)).toBe('today')
  })

  it("decides today by the reader's zone near midnight", () => {
    // 22:30 UTC on the 1st is 00:30 on the 2nd in Amsterdam, 18:30 on the
    // 1st in New York.
    const lateEvening = new Date('2026-06-01T22:30:00Z')
    expect(bucketOf('2026-06-01', lateEvening, AMS)).toBe('overdue')
    expect(bucketOf('2026-06-01', lateEvening, NYC)).toBe('today')
    expect(bucketOf('2026-06-02', lateEvening, AMS)).toBe('today')
    expect(bucketOf('2026-06-02', lateEvening, NYC)).toBe('tomorrow')
  })

  it('places a datetime on its day in the display zone', () => {
    // 23:30 UTC on the 10th is 01:30 on the 11th in Amsterdam.
    expect(bucketOf('2026-06-10T23:30:00Z', now, AMS)).toBe('tomorrow')
    expect(bucketOf('2026-06-10T23:30:00Z', now, NYC)).toBe('today')
  })

  it('reads a naive datetime as wall clock time in the display zone', () => {
    expect(bucketOf('2026-06-10T23:30', now, AMS)).toBe('today')
    expect(bucketOf('2026-06-11T00:30', now, AMS)).toBe('tomorrow')
  })

  it('does not shift a boundary across a DST change', () => {
    // Amsterdam springs forward at 02:00 on 2026-03-29, so that day is 23
    // hours long. Counted in hours, the 30th would be less than a day after
    // 00:30 on the 29th's eve; counted in calendar days it is two.
    const beforeSpring = new Date('2026-03-28T23:30:00Z') // 00:30 on the 29th
    expect(bucketOf('2026-03-29', beforeSpring, AMS)).toBe('today')
    expect(bucketOf('2026-03-30', beforeSpring, AMS)).toBe('tomorrow')
    // Autumn: 2026-10-25 is 25 hours long.
    const afterAutumn = new Date('2026-10-25T22:30:00Z') // 23:30 on the 25th
    expect(bucketOf('2026-10-25', afterAutumn, AMS)).toBe('today')
    expect(bucketOf('2026-10-26', afterAutumn, AMS)).toBe('tomorrow')
  })

  it('lists the buckets in date order', () => {
    expect(BUCKET_ORDER).toEqual([
      'overdue',
      'today',
      'tomorrow',
      'next_7_days',
      'later',
      'no_date',
    ])
  })
})

describe('bucketOf with weeks', () => {
  const wed = new Date('2026-11-25T12:00:00Z')
  const sun = new Date('2026-11-29T12:00:00Z')

  it('splits the days after tomorrow into this week, next week and later', () => {
    const of = (d: string, now = wed) => bucketOf(d, now, AMS, true, true)
    expect(of('2026-11-26')).toBe('tomorrow')
    expect(of('2026-11-27')).toBe('this_week')
    expect(of('2026-11-29')).toBe('this_week')
    expect(of('2026-11-30')).toBe('next_week')
    expect(of('2026-12-06')).toBe('next_week')
    expect(of('2026-12-07')).toBe('later')
    expect(of('2026-11-24')).toBe('overdue')
    expect(of('')).toBe('no_date')
  })

  it('has no this-week days left on a Sunday', () => {
    expect(bucketOf('2026-11-30', sun, AMS, true, true)).toBe('tomorrow')
    expect(bucketOf('2026-12-01', sun, AMS, true, true)).toBe('next_week')
  })

  it('lists the week buckets in date order', () => {
    expect(WEEK_BUCKET_ORDER).toEqual([
      'overdue',
      'today',
      'tomorrow',
      'this_week',
      'next_week',
      'later',
      'no_date',
    ])
  })
})

describe('bucketDate', () => {
  it("names today and tomorrow in the reader's zone and nothing else", () => {
    const lateEvening = new Date('2026-06-30T22:30:00Z') // 00:30 on 1 July in Amsterdam
    expect(bucketDate('today', lateEvening, AMS)).toBe('2026-07-01')
    expect(bucketDate('tomorrow', lateEvening, AMS)).toBe('2026-07-02')
    expect(bucketDate('today', lateEvening, NYC)).toBe('2026-06-30')
    expect(bucketDate('overdue', lateEvening, AMS)).toBeUndefined()
    expect(bucketDate('next_7_days', lateEvening, AMS)).toBeUndefined()
  })
})

describe('bucketMeta', () => {
  it('shows the time for a datetime today, in the display zone', () => {
    expect(bucketMeta('2026-06-10T07:05:00Z', 'today', AMS, 'en-GB')).toBe('09:05')
  })

  it('shows nothing for a bare date today, since the heading says it all', () => {
    expect(bucketMeta('2026-06-10', 'today', AMS, 'en-GB')).toBeUndefined()
  })

  it('shows no time for a date sent as midnight UTC', () => {
    expect(bucketMeta('2026-06-10T00:00:00Z', 'today', AMS, 'en-GB', true)).toBeUndefined()
  })

  it('shows the weekday within the coming week', () => {
    expect(bucketMeta('2026-06-12', 'next_7_days', AMS, 'en-GB')).toBe('Fri')
    expect(bucketMeta('2026-06-11', 'tomorrow', AMS, 'en-GB')).toBe('Thu')
  })

  it('shows a short date for overdue and later rows', () => {
    expect(bucketMeta('2026-06-01', 'overdue', AMS, 'en-GB')).toBe('1 Jun')
    expect(bucketMeta('2026-08-20', 'later', NYC, 'en-GB')).toBe('20 Aug')
  })

  it('shows nothing without a date', () => {
    expect(bucketMeta(undefined, 'no_date', AMS)).toBeUndefined()
  })
})
