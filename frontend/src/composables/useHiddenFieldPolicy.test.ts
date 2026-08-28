import { describe, it, expect } from 'vitest'
import type { ClearWhenHidden } from '@/types'
import { useHiddenFieldPolicy, clearWhenHiddenOf } from './useHiddenFieldPolicy'

// BUG-FB0LN8. The bug was that hiding a conditional field destroyed its stored
// value AND left the field unrenderable, so the loss was silent and permanent.
// These tests pin the reveal direction, which nothing covered before.

function makePolicy(policies: Record<string, ClearWhenHidden> = {}) {
  return useHiddenFieldPolicy({ policyFor: (p) => policies[p] ?? 'no' })
}

describe('useHiddenFieldPolicy — retention', () => {
  it('round-trips a hidden value so a reveal is lossless', () => {
    const policy = makePolicy()
    policy.retain('inschrijfdeadline', '2026-09-15')
    expect(policy.hasRetained('inschrijfdeadline')).toBe(true)
    expect(policy.retainedValue('inschrijfdeadline')).toBe('2026-09-15')
    policy.release('inschrijfdeadline')
    expect(policy.hasRetained('inschrijfdeadline')).toBe(false)
  })

  it('does not retain undefined (nothing to restore)', () => {
    const policy = makePolicy()
    policy.retain('missing', undefined)
    expect(policy.hasRetained('missing')).toBe(false)
  })

  it('retains falsy-but-real values', () => {
    // A boolean false and an empty string are values a user may have chosen;
    // dropping them on hide is the data loss this ticket exists to stop.
    const policy = makePolicy()
    policy.retain('flag', false)
    policy.retain('note', '')
    expect(policy.retainedValue('flag')).toBe(false)
    expect(policy.retainedValue('note')).toBe('')
  })

  it('keeps retention out of any caller-visible form state', () => {
    // The map is the composable's own; the point of the fix is that it is NOT
    // formData, so a caller must ask for it explicitly.
    const policy = makePolicy()
    policy.retain('a', 1)
    expect(policy.retained.value).toEqual({ a: 1 })
  })

  it('releaseAll drops everything (entity switch / reload)', () => {
    const policy = makePolicy()
    policy.retain('a', 1)
    policy.retain('b', 2)
    policy.releaseAll()
    expect(policy.retained.value).toEqual({})
  })
})

describe('useHiddenFieldPolicy — clearOnHide', () => {
  it('clears nothing by default — the whole point of the fix', () => {
    const policy = makePolicy({ deadline: 'no' })
    expect(policy.clearOnHide(['deadline'])).toEqual([])
  })

  it('treats an unconfigured field as the non-destructive default', () => {
    const policy = makePolicy()
    expect(policy.clearOnHide(['anything'])).toEqual([])
  })

  it('clears a yes-policy field', () => {
    const policy = makePolicy({ deadline: 'yes' })
    expect(policy.clearOnHide(['deadline'])).toEqual(['deadline'])
  })

  it('selects only the yes-policy fields from a mixed batch', () => {
    const policy = makePolicy({ keep: 'no', drop: 'yes', alsoDrop: 'yes' })
    expect(policy.clearOnHide(['keep', 'drop', 'alsoDrop'])).toEqual(['drop', 'alsoDrop'])
  })

  it('is a pure query — it does not release retention as a side effect', () => {
    // The caller decides when to release, so a hide that is later revealed can
    // still restore. Folding release() in here would make that impossible.
    const policy = makePolicy({ drop: 'yes' })
    policy.retain('drop', 'x')
    policy.clearOnHide(['drop'])
    expect(policy.hasRetained('drop')).toBe(true)
  })
})

describe('clearWhenHiddenOf', () => {
  it('defaults to the non-destructive policy', () => {
    expect(clearWhenHiddenOf(undefined)).toBe('no')
    expect(clearWhenHiddenOf({})).toBe('no')
  })

  it('reads an explicit setting', () => {
    expect(clearWhenHiddenOf({ clear_when_hidden: 'yes' })).toBe('yes')
    expect(clearWhenHiddenOf({ clear_when_hidden: 'no' })).toBe('no')
  })
})
