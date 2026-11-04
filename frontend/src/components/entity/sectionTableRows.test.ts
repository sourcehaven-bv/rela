import { describe, it, expect } from 'vitest'
import type { ViewRow, ViewSection } from '@/api/views'
import {
  applyPropertyToTableRow,
  indexTableRows,
  tableRowCount,
  tableRowEditable,
  tableRowKey,
} from './sectionTableRows'

function row(id: string, props?: Record<string, unknown>): ViewRow {
  return { entityId: id, entityType: 'line', cells: [], _props: props }
}

function section(over: Partial<ViewSection>): ViewSection {
  return {
    heading: 'Lines',
    sectionId: 'lines',
    display: 'table',
    isEmpty: false,
    isGrouped: false,
    hasContent: false,
    ...over,
  }
}

const INPUT = [{ property: 'qty', render: 'input' as const }]

describe('tableRowCount', () => {
  it('counts flat rows and every group row', () => {
    const s = section({
      rows: [row('A')],
      groups: [{ groupName: 'x', rows: [row('B'), row('C')] }, { groupName: 'y' }],
    })
    expect(tableRowCount(s)).toBe(3)
  })
})

describe('tableRowEditable', () => {
  it.each([
    { name: 'input column with _props', r: row('A', {}), cols: INPUT, count: 1, want: true },
    { name: 'no _props', r: row('A'), cols: INPUT, count: 1, want: false },
    {
      name: 'no input column',
      r: row('A', {}),
      cols: [{ property: 'qty' }],
      count: 1,
      want: false,
    },
    { name: 'at the cap', r: row('A', {}), cols: INPUT, count: 3, want: true },
    { name: 'over the cap', r: row('A', {}), cols: INPUT, count: 4, want: false },
  ])('$name', ({ r, cols, count, want }) => {
    expect(tableRowEditable(r, cols, count, 3)).toBe(want)
  })
})

describe('applyPropertyToTableRow', () => {
  const flat = row('A', { qty: 1 })
  const grouped = row('B', { qty: 2, note: 'x' })
  const other = row('C', { qty: 3 })
  const sections = [
    section({ display: 'cards' }),
    section({ rows: [flat] }),
    section({ isGrouped: true, groups: [{ groupName: 'g', rows: [other, grouped] }] }),
  ]
  const index = indexTableRows(sections)

  it('indexes flat and group rows by address', () => {
    expect(index.get('line/A')).toEqual({ section: 1, group: -1, row: 0 })
    expect(index.get('line/B')).toEqual({ section: 2, group: 0, row: 1 })
  })

  it('keys a faced row on its address', () => {
    expect(tableRowKey({ entityType: 'line', entityId: 'A', _self: 'A@draft' })).toBe(
      'line/A@draft'
    )
  })

  it('replaces a flat row value and keeps the other sections', () => {
    const next = applyPropertyToTableRow(sections, index.get('line/A')!, 'line/A', 'qty', 5)!
    expect(next[1].rows?.[0]._props).toEqual({ qty: 5 })
    expect(next[0]).toBe(sections[0])
    expect(next[2]).toBe(sections[2])
    expect(flat._props).toEqual({ qty: 1 })
  })

  it('replaces a group row value and keeps its neighbours', () => {
    const next = applyPropertyToTableRow(sections, index.get('line/B')!, 'line/B', 'qty', 7)!
    const rows = next[2].groups?.[0].rows
    expect(rows?.[1]._props).toEqual({ qty: 7, note: 'x' })
    expect(rows?.[0]).toBe(other)
  })

  it('returns the same sections when the row already holds the value', () => {
    expect(applyPropertyToTableRow(sections, index.get('line/A')!, 'line/A', 'qty', 1)).toBe(
      sections
    )
  })

  it('removes a property set to undefined', () => {
    const next = applyPropertyToTableRow(
      sections,
      index.get('line/B')!,
      'line/B',
      'note',
      undefined
    )!
    expect(next[2].groups?.[0].rows?.[1]._props).toEqual({ qty: 2 })
  })

  it('refuses when the row at the location is another entity', () => {
    expect(applyPropertyToTableRow(sections, index.get('line/A')!, 'line/B', 'qty', 1)).toBeNull()
    expect(
      applyPropertyToTableRow(sections, { section: 9, group: -1, row: 0 }, 'line/A', 'qty', 1)
    ).toBeNull()
  })
})
