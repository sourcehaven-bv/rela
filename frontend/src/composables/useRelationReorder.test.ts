import { describe, expect, it } from 'vitest'
import {
  computeNewOrder,
  extractFiniteNumber,
  ORDER_COLLAPSE_THRESHOLD,
} from './useRelationReorder'

describe('computeNewOrder', () => {
  it('returns 1.0 for an empty list (no prev, no next)', () => {
    expect(computeNewOrder({ prevOrder: undefined, nextOrder: undefined })).toBe(1.0)
  })

  it('prepend: returns next - 1 when only next is set', () => {
    expect(computeNewOrder({ prevOrder: undefined, nextOrder: 5.0 })).toBe(4.0)
  })

  it('append: returns prev + 1 when only prev is set', () => {
    expect(computeNewOrder({ prevOrder: 5.0, nextOrder: undefined })).toBe(6.0)
  })

  it('midpoint: returns (prev + next) / 2 with safe gap', () => {
    expect(computeNewOrder({ prevOrder: 1, nextOrder: 2 })).toBe(1.5)
    expect(computeNewOrder({ prevOrder: 0, nextOrder: 1000 })).toBe(500)
  })

  it('midpoint: returns undefined on collapse', () => {
    const lo = 1.0
    const hi = 1.0 + ORDER_COLLAPSE_THRESHOLD / 2
    expect(computeNewOrder({ prevOrder: lo, nextOrder: hi })).toBeUndefined()
  })

  it('treats non-finite neighbour values as absent', () => {
    expect(computeNewOrder({ prevOrder: NaN, nextOrder: 2 })).toBe(1.0)
    expect(computeNewOrder({ prevOrder: 1, nextOrder: Infinity })).toBe(2.0)
  })
})

describe('extractFiniteNumber', () => {
  it('returns finite numbers verbatim', () => {
    expect(extractFiniteNumber(1)).toBe(1)
    expect(extractFiniteNumber(-3.5)).toBe(-3.5)
    expect(extractFiniteNumber(0)).toBe(0)
  })

  it('returns undefined for missing / non-numeric / non-finite', () => {
    expect(extractFiniteNumber(undefined)).toBeUndefined()
    expect(extractFiniteNumber(null)).toBeUndefined()
    expect(extractFiniteNumber('1')).toBeUndefined()
    expect(extractFiniteNumber(NaN)).toBeUndefined()
    expect(extractFiniteNumber(Infinity)).toBeUndefined()
    expect(extractFiniteNumber(-Infinity)).toBeUndefined()
    expect(extractFiniteNumber({})).toBeUndefined()
  })
})
