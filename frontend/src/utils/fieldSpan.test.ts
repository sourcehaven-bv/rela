import { describe, it, expect } from 'vitest'
import { fieldSpanStyle, isValidSpan, SPAN_COLUMNS } from './fieldSpan'

describe('fieldSpanStyle', () => {
  it('emits no style for an unauthored span so the CSS fallback gives full width', () => {
    // The "no span" default lives in properties-list.css as
    // var(--field-span, 12). Emitting nothing here keeps it in ONE place —
    // returning {'--field-span': '12'} would duplicate the default.
    expect(fieldSpanStyle(undefined)).toBeUndefined()
    expect(fieldSpanStyle(0)).toBeUndefined()
  })

  it.each([1, 3, 4, 6, 8, SPAN_COLUMNS])('passes through valid span %i', (span) => {
    expect(fieldSpanStyle(span)).toEqual({ '--field-span': String(span) })
  })

  it.each([
    ['above the grid', SPAN_COLUMNS + 1],
    ['far above the grid', 99],
    ['negative', -1],
    ['fractional', 4.5],
    ['NaN', NaN],
    ['Infinity', Infinity],
  ])('falls back to full width for %s', (_label, span) => {
    // Defence-in-depth: the server rejects these at config load, so a value
    // arriving here means a hand-crafted response or an older server. Degrade
    // rather than emit a broken grid-column.
    expect(fieldSpanStyle(span as number)).toBeUndefined()
  })

  it('never emits a value that could escape the custom property', () => {
    // The span reaches CSS, so a non-numeric value must not pass through.
    // Guards against config -> stylesheet injection if a caller bypasses types.
    const hostile = '4; background: url(evil)' as unknown as number
    expect(fieldSpanStyle(hostile)).toBeUndefined()
  })
})

describe('isValidSpan', () => {
  it('accepts the inclusive bounds of the grid', () => {
    expect(isValidSpan(1)).toBe(true)
    expect(isValidSpan(SPAN_COLUMNS)).toBe(true)
  })

  it('rejects 0, which means "unauthored" rather than a real width', () => {
    expect(isValidSpan(0)).toBe(false)
  })

  it('rejects non-numeric input', () => {
    expect(isValidSpan(undefined)).toBe(false)
    expect(isValidSpan('4' as unknown as number)).toBe(false)
  })
})
