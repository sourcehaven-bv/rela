import { describe, it, expect } from 'vitest'
import { buildSeries, formatBucket, niceTicks, pointValue, rangeStart } from './recordsChart'
import type { RecordPoint } from '@/api/records'

const cfg = { type: 'measurement', field: 'value', owners: ['MET-A', 'MET-B', 'MET-C'] }

describe('recordsChart', () => {
  it('reads the range in record retention units', () => {
    const now = new Date('2026-11-16T00:00:00Z')
    expect(rangeStart('2d', now)).toBe('2026-11-14T00:00:00.000Z')
    expect(rangeStart('48h', now)).toBe('2026-11-14T00:00:00.000Z')
    expect(rangeStart('1w', now)).toBe('2026-11-09T00:00:00.000Z')
    expect(rangeStart('1y', now)).toBe('2025-11-16T00:00:00.000Z')
    expect(rangeStart(undefined, now)).toBeUndefined()
    expect(rangeStart('forever', now)).toBeUndefined()
    expect(rangeStart('0d', now)).toBeUndefined()
  })

  it('takes the statistic, latest by default and count from the row count', () => {
    const p: RecordPoint = { count: 3, latest: '0.82', avg: '0.5', sum: '' }
    expect(pointValue(p, undefined)).toBe(0.82)
    expect(pointValue(p, 'avg')).toBe(0.5)
    expect(pointValue(p, 'count')).toBe(3)
    expect(pointValue(p, 'sum')).toBeUndefined()
    expect(pointValue(p, 'max')).toBeUndefined()
  })

  it('keeps a slot per configured owner, so a missing owner repaints nothing', () => {
    const points: RecordPoint[] = [
      { owner: 'MET-C', start: '2026-02-01T00:00:00Z', count: 1, latest: '3' },
      { owner: 'MET-A', start: '2026-02-01T00:00:00Z', count: 1, latest: '2' },
      { owner: 'MET-A', start: '2026-01-01T00:00:00Z', count: 1, latest: '1' },
      // Not on the card, and without a start: both dropped.
      { owner: 'MET-X', start: '2026-01-01T00:00:00Z', count: 1, latest: '9' },
      { owner: 'MET-A', count: 1, latest: '9' },
    ]
    const series = buildSeries(cfg, points)
    expect(series.map((s) => [s.owner, s.slot])).toEqual([
      ['MET-A', 0],
      ['MET-C', 2],
    ])
    expect(series[0].points.map((p) => p.v)).toEqual([1, 2])
  })

  it('makes round ticks that include zero', () => {
    expect(niceTicks(0.4, 0.9)).toEqual([0, 0.25, 0.5, 0.75, 1])
    expect(niceTicks(5, 5)).toEqual([0, 2, 4, 6])
    expect(niceTicks(-3, 7)).toEqual([-5, -2.5, 0, 2.5, 5, 7.5])
  })

  it('formats a bucket in UTC', () => {
    const t = Date.parse('2026-03-01T00:00:00Z')
    expect(formatBucket(t, 'month', 'en-GB')).toBe('Mar 2026')
    expect(formatBucket(t, 'day', 'en-GB')).toBe('1 Mar 2026')
  })
})
