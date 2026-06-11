import { describe, it, expect } from 'vitest'
import { isClearedForType } from './formValue'
import type { PropertyDef } from '@/types'

describe('isClearedForType', () => {
  it('returns false for booleans regardless of value (false is legitimate)', () => {
    const def: PropertyDef = { type: 'boolean' } as PropertyDef
    expect(isClearedForType(false, def)).toBe(false)
    expect(isClearedForType(true, def)).toBe(false)
  })

  it('returns true for empty arrays (multi-select cleared)', () => {
    expect(isClearedForType([], undefined)).toBe(true)
  })

  it('returns false for non-empty arrays', () => {
    expect(isClearedForType(['a'], undefined)).toBe(false)
  })

  it('returns true for empty string / null / undefined', () => {
    expect(isClearedForType('', undefined)).toBe(true)
    expect(isClearedForType(null, undefined)).toBe(true)
    expect(isClearedForType(undefined, undefined)).toBe(true)
  })

  it('returns false for a non-empty string', () => {
    expect(isClearedForType('hello', undefined)).toBe(false)
  })

  it('returns false for zero (numbers cleared only via empty string from inputs)', () => {
    expect(isClearedForType(0, undefined)).toBe(false)
  })
})
