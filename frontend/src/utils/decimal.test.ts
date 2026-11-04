import { describe, it, expect } from 'vitest'
import {
  compareDecimals,
  decimalText,
  divideDecimal,
  parseDecimal,
  sumDecimals,
  type Decimal,
} from './decimal'

const d = (s: string): Decimal => {
  const v = parseDecimal(s)
  if (!v) throw new Error(`not a decimal: ${s}`)
  return v
}

describe('parseDecimal', () => {
  it.each([
    ['12.50', 1250n, 2],
    ['-0.125', -125n, 3],
    ['+3', 3n, 0],
    ['007.10', 710n, 2],
  ])('reads %s', (text, units, scale) => {
    expect(parseDecimal(text)).toEqual({ units, scale })
  })

  it('reads a JSON number through its shortest text', () => {
    expect(parseDecimal(0.1)).toEqual({ units: 1n, scale: 1 })
    expect(parseDecimal(1e21)).toEqual({ units: 10n ** 21n, scale: 0 })
  })

  it.each(['', '1e3', '.5', '5.', '1,5', 'abc', ' 1', '9'.repeat(65)])('refuses %j', (text) => {
    expect(parseDecimal(text)).toBeUndefined()
  })

  it('refuses a value of another kind', () => {
    expect(parseDecimal(null)).toBeUndefined()
    expect(parseDecimal(Number.NaN)).toBeUndefined()
    expect(parseDecimal(true)).toBeUndefined()
  })
})

describe('decimal arithmetic', () => {
  it('writes the text back with its scale', () => {
    expect(decimalText(d('12.50'))).toBe('12.50')
    expect(decimalText(d('-0.05'))).toBe('-0.05')
    expect(decimalText(d('0'))).toBe('0')
  })

  it('sums exactly', () => {
    expect(decimalText(sumDecimals([d('0.1'), d('0.2')]))).toBe('0.3')
    expect(decimalText(sumDecimals([d('1'), d('2.50'), d('-0.125')]))).toBe('3.375')
    expect(decimalText(sumDecimals([]))).toBe('0')
  })

  it('compares numerically', () => {
    expect(compareDecimals(d('12.5'), d('12.50'))).toBe(0)
    expect(compareDecimals(d('9'), d('10'))).toBe(-1)
    expect(compareDecimals(d('-1'), d('-1.5'))).toBe(1)
  })

  it('divides with half-away-from-zero rounding', () => {
    expect(decimalText(divideDecimal(d('1'), 3, 2))).toBe('0.33')
    expect(decimalText(divideDecimal(d('2'), 3, 2))).toBe('0.67')
    expect(decimalText(divideDecimal(d('0.125'), 1, 2))).toBe('0.13')
    expect(decimalText(divideDecimal(d('-0.125'), 1, 2))).toBe('-0.13')
  })
})
