import { describe, expect, it } from 'vitest'
import type { TableColumn } from './types'
import {
  cellSizeStyle,
  columnMinWidth,
  columnWidth,
  DEFAULT_COLUMN_WIDTH,
  keptWidth,
  lastKeptKey,
  NAME_MIN_WIDTH,
  tableMinWidth,
} from './columnSizing'

const date: TableColumn = { key: 'issued', header: 'Issued', width: 120, minWidth: 104 }
const text: TableColumn = { key: 'note', header: 'Note' }
const badge: TableColumn = { key: 'status', header: 'Status', width: 136, primary: true }
const link: TableColumn = { key: 'pdf', header: 'PDF', primary: true }

describe('columnSizing', () => {
  it('defaults the width and shrinks to three quarters of it', () => {
    expect(columnWidth(text)).toBe(DEFAULT_COLUMN_WIDTH)
    expect(columnMinWidth(text)).toBe(Math.round(DEFAULT_COLUMN_WIDTH * 0.75))
    expect(columnMinWidth(date)).toBe(date.minWidth)
  })

  it('never puts the floor above the width', () => {
    expect(columnMinWidth({ key: 'x', header: 'X', width: 80, minWidth: 200 })).toBe(80)
  })

  it('gives the header and the rows the same two numbers', () => {
    expect(cellSizeStyle(date)).toEqual({
      '--rl-cell-width': `${date.width}px`,
      '--rl-cell-min-width': `${date.minWidth}px`,
    })
  })

  it('scrolls only below the sum of the floors', () => {
    expect(tableMinWidth([date, text])).toBe(NAME_MIN_WIDTH + columnMinWidth(date) + columnMinWidth(text))
  })

  it('fits a five-column invoice list beside a laptop sidebar', () => {
    // 1024px window less the 260px sidebar (BUG-UX4P9T2). Fixed 150px columns
    // needed 990px and dropped to the compressed layout.
    const invoice: TableColumn[] = [
      date,
      { ...date, key: 'due' },
      { key: 'total', header: 'Total', width: 120, minWidth: 88 },
      { ...badge, minWidth: 104 },
      link,
    ]
    expect(tableMinWidth(invoice)).toBeLessThanOrEqual(1024 - 260)
  })

  it('names the last kept column, which carries the gutter', () => {
    expect(lastKeptKey([badge, link, text])).toBe(link.key)
    expect(lastKeptKey([text])).toBeUndefined()
  })

  it('adds up the kept cells as the compressed rows size them', () => {
    expect(keptWidth([text])).toBe('0px')
    expect(keptWidth([badge, text, link])).toBe(
      `calc(min(${badge.width}px, 30cqi) + min(${DEFAULT_COLUMN_WIDTH}px, 30cqi) + var(--rl-page-gutter-right) - var(--rl-space-2))`,
    )
  })
})
