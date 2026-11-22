import { describe, it, expect } from 'vitest'
import { isSectionCellEmpty, sectionCellClasses, sectionColumnLabel } from './sectionTable'

// The phone layout of a table section (BUG-6WQ1U8S): which cells a stacked
// row shows, and the label beside each value.

describe('sectionColumnLabel', () => {
  it('reads the label, then the property, then the relation', () => {
    expect(sectionColumnLabel({ property: 'due', label: 'Due' })).toBe('Due')
    expect(sectionColumnLabel({ property: 'due' })).toBe('due')
    expect(sectionColumnLabel({ relation: 'invoice-for' })).toBe('invoice-for')
    expect(sectionColumnLabel(undefined)).toBe('')
  })
})

describe('isSectionCellEmpty', () => {
  it('treats no values and blank values as empty', () => {
    expect(isSectionCellEmpty(undefined)).toBe(true)
    expect(isSectionCellEmpty({ values: [] })).toBe(true)
    expect(isSectionCellEmpty({ values: ['', '  '] })).toBe(true)
    expect(isSectionCellEmpty({ values: ['21 Nov 2026'] })).toBe(false)
  })

  // An unset property arrives as `"values": null` (a nil Go slice).
  it('treats null values as empty', () => {
    expect(isSectionCellEmpty({ values: null as unknown as string[] })).toBe(true)
  })
})

describe('sectionCellClasses', () => {
  it('makes the first column the heading, never empty or dropped', () => {
    expect(sectionCellClasses({ property: 'number', primary: false }, { values: [] }, 0)).toEqual({
      'section-cell--title': true,
      'section-cell--empty': false,
      'section-cell--secondary': false,
    })
  })

  it('marks an empty read-only cell', () => {
    expect(sectionCellClasses({ property: 'due' }, { values: [] }, 1)['section-cell--empty']).toBe(
      true
    )
  })

  it('never marks an editable cell empty', () => {
    expect(
      sectionCellClasses({ property: 'due' }, { values: [] }, 1, true)['section-cell--empty']
    ).toBe(false)
  })

  it('drops a column set to primary: false only', () => {
    const cell = { values: ['x'] }
    expect(
      sectionCellClasses({ property: 'a', primary: false }, cell, 2)['section-cell--secondary']
    ).toBe(true)
    expect(
      sectionCellClasses({ property: 'a', primary: true }, cell, 2)['section-cell--secondary']
    ).toBe(false)
    expect(sectionCellClasses({ property: 'a' }, cell, 2)['section-cell--secondary']).toBe(false)
  })
})
