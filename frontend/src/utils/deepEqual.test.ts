import { describe, it, expect } from 'vitest'
import { deepEqual } from './deepEqual'

describe('deepEqual', () => {
  it('ignores key order', () => {
    expect(deepEqual({ a: 1, b: [1, { c: 2 }] }, { b: [1, { c: 2 }], a: 1 })).toBe(true)
  })
  it('tells different values apart', () => {
    expect(deepEqual({ a: 1 }, { a: 2 })).toBe(false)
    expect(deepEqual([1, 2], [2, 1])).toBe(false)
    expect(deepEqual({ a: 1 }, { b: 1 })).toBe(false)
    expect(deepEqual([], {})).toBe(false)
    expect(deepEqual(null, undefined)).toBe(false)
  })
})
