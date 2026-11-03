import { describe, it, expect } from 'vitest'
import { formatSummary, NO_VALUE, type SummarizedEntity } from './summary'
import { formatAmount, formatDate, formatDatetime } from './format'
import type { PropertyDef } from '@/types'

const e = (properties: Record<string, unknown>, redacted?: string[]): SummarizedEntity => ({
  properties,
  ...(redacted ? { _redacted: redacted } : {}),
})
const int: PropertyDef = { type: 'integer' }
const eur: PropertyDef = { type: 'integer', display: { currency: 'EUR' } }

describe('formatSummary', () => {
  const set = [e({ v: 4 }), e({ v: 1 }), e({ v: 2 }), e({}), e({ v: 'x' })]

  it('counts the entities, whatever their values', () => {
    expect(formatSummary({ function: 'count' }, set, undefined)).toBe('Count: 5')
  })

  it('aggregates the numeric values and skips the rest', () => {
    expect(formatSummary({ function: 'sum', property: 'v' }, set, int)).toBe('Total: 7')
    expect(formatSummary({ function: 'avg', property: 'v' }, set, int)).toBe(`Average: ${formatAmount(7 / 3)}`)
    expect(formatSummary({ function: 'min', property: 'v' }, set, int)).toBe('Minimum: 1')
    expect(formatSummary({ function: 'max', property: 'v' }, set, int)).toBe('Maximum: 4')
  })

  it('uses the label when given', () => {
    expect(formatSummary({ function: 'sum', property: 'v', label: 'Totaal' }, set, eur)).toBe(
      `Totaal: ${formatAmount(7, 'EUR')}`,
    )
  })

  it('reports an empty set as 0 or a dash', () => {
    expect(formatSummary({ function: 'count' }, [], undefined)).toBe('Count: 0')
    expect(formatSummary({ function: 'sum', property: 'v' }, [], int)).toBe('Total: 0')
    expect(formatSummary({ function: 'avg', property: 'v' }, [], int)).toBe(`Average: ${NO_VALUE}`)
    expect(formatSummary({ function: 'min', property: 'v' }, [e({})], int)).toBe(`Minimum: ${NO_VALUE}`)
  })

  it('picks the earliest and latest date and formats it like the property', () => {
    const dates = [e({ d: '2026-03-01' }), e({ d: '2026-01-15' }), e({ d: '' }), e({ d: '2026-12-31' })]
    const date: PropertyDef = { type: 'date' }
    expect(formatSummary({ function: 'min', property: 'd' }, dates, date)).toBe(`Minimum: ${formatDate('2026-01-15')}`)
    expect(formatSummary({ function: 'max', property: 'd' }, dates, date)).toBe(`Maximum: ${formatDate('2026-12-31')}`)
  })

  it('compares datetimes as instants, not as text', () => {
    const times = [e({ t: '2026-05-01T10:00:00+02:00' }), e({ t: '2026-05-01T09:30:00Z' })]
    const dt: PropertyDef = { type: 'datetime' }
    expect(formatSummary({ function: 'min', property: 't' }, times, dt, 'UTC')).toBe(
      `Minimum: ${formatDatetime('2026-05-01T10:00:00+02:00', 'UTC')}`,
    )
  })

  it('marks a summary partial when an entity hides the property', () => {
    const hidden = [e({ v: 3 }), e({}, ['v'])]
    expect(formatSummary({ function: 'sum', property: 'v' }, hidden, int)).toBe('Total: 3 (partial)')
    expect(formatSummary({ function: 'count' }, hidden, undefined)).toBe('Count: 2')
  })
})
