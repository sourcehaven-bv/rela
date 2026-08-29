import { describe, expect, it } from 'vitest'

import type { GanttNode } from '@/api/gantts'
import {
  barSpan,
  findNode,
  flattenRows,
  forestSpan,
  parseDay,
  pct,
  ticksFor,
} from './ganttLayout'

const node = (id: string, extra: Partial<GanttNode> = {}): GanttNode => ({
  id,
  type: 'project',
  ...extra,
})

describe('parseDay', () => {
  it('parses ISO dates and rejects garbage', () => {
    expect(parseDay('1970-01-01')).toBe(0)
    expect(parseDay('1970-01-11')).toBe(10)
    expect(parseDay(undefined)).toBeNull()
    expect(parseDay('')).toBeNull()
    expect(parseDay('not-a-date')).toBeNull()
  })
})

describe('barSpan', () => {
  it('unions planned and rolled — an overrun must widen the bar', () => {
    const s = barSpan(
      node('a', {
        planned: { start: '2026-02-01', end: '2026-03-01' },
        rolled: { start: '2026-01-15', end: '2026-04-01' },
      }),
    )
    expect(s.start).toBe(parseDay('2026-01-15'))
    expect(s.end).toBe(parseDay('2026-04-01'))
  })

  it('handles each span being absent', () => {
    expect(barSpan(node('a'))).toEqual({ start: null, end: null })
    const rolledOnly = barSpan(node('a', { rolled: { start: '2026-01-01', end: '2026-02-01' } }))
    expect(rolledOnly.start).toBe(parseDay('2026-01-01'))
  })
})

describe('forestSpan / pct', () => {
  it('pads the envelope and positions days within it', () => {
    const axis = forestSpan([
      node('a', { planned: { start: '2026-01-01', end: '2026-12-31' } }),
    ])!
    expect(axis.start).toBeLessThan(parseDay('2026-01-01')!)
    expect(axis.end).toBeGreaterThan(parseDay('2026-12-31')!)
    expect(pct(axis.start, axis)).toBe(0)
    expect(pct(axis.end, axis)).toBe(100)
  })

  it('is null for an undated forest', () => {
    expect(forestSpan([node('a')])).toBeNull()
    expect(forestSpan([])).toBeNull()
  })
})

describe('ticksFor', () => {
  const axis = { start: parseDay('2026-01-15')!, end: parseDay('2026-06-15')! }

  it('emits month boundaries inside the axis', () => {
    const labels = ticksFor(axis, 'month').map((t) => t.label)
    expect(labels).toEqual(['Feb', 'Mar', 'Apr', 'May', 'Jun'])
  })

  it('emits quarter boundaries', () => {
    const labels = ticksFor(axis, 'quarter').map((t) => t.label)
    expect(labels).toEqual(["Q2 '26"])
  })

  it('week ticks all fall on Mondays and carry ISO week numbers', () => {
    const ticks = ticksFor({ start: parseDay('2026-03-01')!, end: parseDay('2026-03-31')! }, 'week')
    expect(ticks.length).toBeGreaterThan(3)
    for (const t of ticks) {
      expect(new Date(t.day * 86_400_000).getUTCDay()).toBe(1)
      expect(t.label).toMatch(/^W\d{1,2}$/)
    }
    // 2026-03-02 is the Monday of ISO week 10.
    expect(ticks[0].label).toBe('W10')
  })

  it('caps density over a two-year span instead of smearing labels', () => {
    const twoYears = { start: parseDay('2026-01-01')!, end: parseDay('2027-12-31')! }
    for (const zoom of ['week', 'month', 'quarter'] as const) {
      const ticks = ticksFor(twoYears, zoom)
      expect(ticks.length).toBeLessThanOrEqual(22)
      expect(ticks.length).toBeGreaterThan(3)
    }
    // A ~104-week span emits every 4th week, still Mondays, still ISO-numbered.
    const weeks = ticksFor(twoYears, 'week')
    for (const t of weeks) {
      expect(new Date(t.day * 86_400_000).getUTCDay()).toBe(1)
      expect(t.label).toMatch(/^W\d{1,2}$/)
    }
  })

  it('months carry the year at January on a multi-year axis', () => {
    const twoYears = { start: parseDay('2026-06-01')!, end: parseDay('2027-08-31')! }
    const labels = ticksFor(twoYears, 'month').map((t) => t.label)
    expect(labels).toContain("Jan '27")
  })

  it('January survives striding — the year anchor is never skipped', () => {
    const threeYears = { start: parseDay('2026-01-01')!, end: parseDay('2028-12-31')! }
    const labels = ticksFor(threeYears, 'month').map((t) => t.label)
    for (const y of ["'26", "'27", "'28"]) {
      expect(labels.some((l) => l === `Jan ${y}`)).toBe(true)
    }
  })
})

describe('flattenRows', () => {
  // A ⊃ B ⊃ C ⊃ D — the recursive-depth case the view exists for.
  const deep = node('A', {
    children: [node('B', { children: [node('C', { children: [node('D')] })] })],
  })

  it('cuts at default depth', () => {
    const rows = flattenRows([deep], 2, new Set())
    expect(rows.map((r) => r.node.id)).toEqual(['A', 'B'])
    expect(rows.map((r) => r.indent)).toEqual([0, 1])
  })

  it('expand opens one subtree past the cut without opening siblings', () => {
    const rows = flattenRows([deep], 2, new Set(['B']))
    expect(rows.map((r) => r.node.id)).toEqual(['A', 'B', 'C'])
  })

  it('chained expansions reach arbitrary depth', () => {
    const rows = flattenRows([deep], 2, new Set(['B', 'C']))
    expect(rows.map((r) => r.node.id)).toEqual(['A', 'B', 'C', 'D'])
  })

  it('depth 99 shows everything', () => {
    expect(flattenRows([deep], 99, new Set())).toHaveLength(4)
  })
})

describe('findNode', () => {
  it('locates nodes at any depth, or null', () => {
    const forest = [
      node('A', { children: [node('B', { children: [node('C')] })] }),
      node('X'),
    ]
    expect(findNode(forest, 'C')?.id).toBe('C')
    expect(findNode(forest, 'X')?.id).toBe('X')
    expect(findNode(forest, 'ghost')).toBeNull()
  })
})
