import { describe, it, expect } from 'vitest'
import { isFieldWritable, isPropertyRedacted, optionVerdictsFor } from './affordances'
import type { FieldAffordance } from '@/types'

describe('isFieldWritable', () => {
  it('returns true when verdict is undefined and fieldReadonly is undefined', () => {
    expect(isFieldWritable(undefined)).toBe(true)
  })

  it('returns true when verdict.writable is undefined (default writable)', () => {
    expect(isFieldWritable({})).toBe(true)
  })

  it('returns true when verdict.writable is explicitly true', () => {
    expect(isFieldWritable({ writable: true })).toBe(true)
  })

  it('returns false when verdict.writable is false', () => {
    expect(isFieldWritable({ writable: false })).toBe(false)
  })

  it('returns false when fieldReadonly is true regardless of verdict', () => {
    expect(isFieldWritable(undefined, true)).toBe(false)
    expect(isFieldWritable({ writable: true }, true)).toBe(false)
  })

  it('returns true when fieldReadonly is explicitly false', () => {
    expect(isFieldWritable(undefined, false)).toBe(true)
  })

  it('combines both channels — verdict false wins over fieldReadonly undefined', () => {
    expect(isFieldWritable({ writable: false }, undefined)).toBe(false)
  })

  it('combines both channels — fieldReadonly true wins over verdict writable', () => {
    expect(isFieldWritable({ writable: true }, true)).toBe(false)
  })
})

describe('isPropertyRedacted', () => {
  it('returns true only for a property the server names redacted', () => {
    expect(isPropertyRedacted('salary', ['salary'])).toBe(true)
    expect(isPropertyRedacted('salary', ['notes', 'salary'])).toBe(true)
  })

  it('returns false for a property absent from the redacted list', () => {
    expect(isPropertyRedacted('title', ['salary'])).toBe(false)
  })

  // The bug this whole mechanism exists to prevent (BUG-MLT9DE): an unset
  // property is absent from `properties` exactly like a redacted one, so
  // absence must NOT be read as redaction. `_redacted: []` is the server
  // saying "evaluated, nothing hidden" — every field must render.
  it('returns false for every property when nothing is redacted', () => {
    expect(isPropertyRedacted('newly_added_prop', [])).toBe(false)
  })

  // A shape that reports no `_redacted` (list row, pre-affordance response)
  // must render rather than hide — hiding would resurrect the silent-hiding
  // bug on those shapes, and the server's write gate is the real boundary.
  it('returns false when the server reported no redaction list', () => {
    expect(isPropertyRedacted('anything', undefined)).toBe(false)
  })
})

describe('optionVerdictsFor', () => {
  it('returns undefined when verdict is undefined', () => {
    expect(optionVerdictsFor(undefined)).toBeUndefined()
  })

  it('returns undefined when verdict.options is undefined', () => {
    expect(optionVerdictsFor({ writable: true })).toBeUndefined()
  })

  it('returns the sparse options map verbatim', () => {
    const verdict: FieldAffordance = { options: { rejected: false } }
    expect(optionVerdictsFor(verdict)).toEqual({ rejected: false })
  })
})
