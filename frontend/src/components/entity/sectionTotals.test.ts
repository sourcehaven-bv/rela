import { describe, expect, it } from 'vitest'
import type { EntityType } from '@/types'
import { sectionTotalRows } from './sectionTotals'

const quote = {
  properties: {
    subtotal: { type: 'decimal', label: 'Subtotal', display: { currency: 'EUR' }, scale: 2 },
    lines: { type: 'integer' },
  },
} as unknown as EntityType

describe('sectionTotalRows', () => {
  it('reads values from the entry and keeps the configured order and emphasis', () => {
    const rows = sectionTotalRows(
      [
        { property: 'lines', label: 'Lines' },
        { property: 'subtotal', emphasis: true },
      ],
      { lines: 3, subtotal: '120.5' },
      quote
    )
    expect(rows.map((r) => r.label)).toEqual(['Lines', 'Subtotal'])
    expect(rows[0]).toMatchObject({ value: '3' })
    expect(rows[0].emphasis).toBeUndefined()
    expect(rows[1].emphasis).toBe(true)
    expect(rows[1].value).toContain('120')
  })

  it('leaves out a total the entry does not carry rather than showing zero', () => {
    expect(sectionTotalRows([{ property: 'subtotal' }], { lines: 2 }, quote)).toEqual([])
  })

  it('returns nothing without totals or an entry', () => {
    expect(sectionTotalRows(undefined, { lines: 1 }, quote)).toEqual([])
    expect(sectionTotalRows([{ property: 'lines' }], undefined, quote)).toEqual([])
  })
})
