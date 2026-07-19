import { describe, it, expect } from 'vitest'
import { STAGED_ID, isStaged, adoptLockedFieldValues } from './stagedEntity'

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

describe('adoptLockedFieldValues (TKT-3G93B8 / BUG-X1C7S)', () => {
  it('adopts the server value for a read-only (machine-locked) field', () => {
    const formData: Record<string, unknown> = { status: 'doing', title: 'user typed' }
    adoptLockedFieldValues(
      { status: { writable: false } },
      { status: 'todo', title: 'user typed' },
      formData
    )
    // The locked field takes the server's initial; the editable field is untouched.
    expect(formData.status).toBe('todo')
    expect(formData.title).toBe('user typed')
  })

  it('leaves writable (editable) fields untouched even if echoed', () => {
    const formData: Record<string, unknown> = { status: 'mine' }
    adoptLockedFieldValues({ status: { writable: true } }, { status: 'server' }, formData)
    expect(formData.status).toBe('mine')
  })

  it('does not adopt a read-only field the server did not send', () => {
    const formData: Record<string, unknown> = { status: 'mine' }
    // writable=false but absent from properties (e.g. hidden + stripped): skip.
    adoptLockedFieldValues({ status: { writable: false } }, {}, formData)
    expect(formData.status).toBe('mine')
  })

  it('is a no-op when fields or properties are absent', () => {
    const formData: Record<string, unknown> = { status: 'mine' }
    adoptLockedFieldValues(undefined, { status: 'x' }, formData)
    adoptLockedFieldValues({ status: { writable: false } }, undefined, formData)
    expect(formData.status).toBe('mine')
  })
})
