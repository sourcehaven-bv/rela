import { describe, it, expect } from 'vitest'
import { STAGED_ID, isStaged } from './stagedEntity'

describe('stagedEntity sentinel (TKT-3I5U)', () => {
  it('STAGED_ID is the documented form-only sentinel', () => {
    expect(STAGED_ID).toBe('++new++')
  })

  it('isStaged matches only the sentinel', () => {
    expect(isStaged(STAGED_ID)).toBe(true)
    expect(isStaged('TKT-001')).toBe(false)
    expect(isStaged('')).toBe(false)
    expect(isStaged(undefined)).toBe(false)
  })

  it('the sentinel cannot collide with a prefix-based real ID', () => {
    // Real IDs are <PREFIX>-<n>; the sentinel has no hyphen-prefix shape.
    expect(STAGED_ID).not.toMatch(/^[A-Z]+-/)
  })
})
