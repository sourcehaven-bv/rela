// Unit tests for the proposal primitives (TKT-7S5735).
//
// These are the layer the propose/commit seam rests on, and they are pure
// functions precisely so this can be tested with no mount, no dialog and no
// fake timers — the four BUG-FB0LN8 bugs all lived in logic that previously
// could only be reached through a mounted component mid-flush.

import { describe, it, expect } from 'vitest'
import { proposedBindings, wouldHide, type Proposal } from './useProposal'

const proposal = (property: string, value: unknown, previous: unknown = undefined): Proposal => ({
  property,
  value,
  previous,
})

describe('proposedBindings', () => {
  it('returns bindings with the proposed value applied', () => {
    const current = { form: { route: 'aanbesteding', deadline: '2026-09-15' } }
    const next = proposedBindings(current, proposal('route', 'onderhands', 'aanbesteding'))
    expect((next.form as Record<string, unknown>).route).toBe('onderhands')
  })

  it('leaves other properties untouched', () => {
    const current = { form: { route: 'aanbesteding', deadline: '2026-09-15' } }
    const next = proposedBindings(current, proposal('route', 'onderhands'))
    expect((next.form as Record<string, unknown>).deadline).toBe('2026-09-15')
  })

  // The whole design rests on asking the question being free of side effects.
  // If this leaked, "what would happen" would become "what just happened" —
  // which is the bug the seam exists to prevent.
  it('does NOT mutate the live bindings', () => {
    const form = { route: 'aanbesteding' }
    const current = { form }
    proposedBindings(current, proposal('route', 'onderhands'))
    expect(form.route).toBe('aanbesteding')
    expect(current.form).toBe(form)
  })

  it('preserves sibling namespaces', () => {
    const current = { form: { a: 1 }, entity: { id: 'TKT-1' }, current_user: { name: 'jo' } }
    const next = proposedBindings(current, proposal('a', 2))
    expect(next.entity).toEqual({ id: 'TKT-1' })
    expect(next.current_user).toEqual({ name: 'jo' })
  })

  it('tolerates an absent form namespace', () => {
    const next = proposedBindings({}, proposal('a', 1))
    expect((next.form as Record<string, unknown>).a).toBe(1)
  })

  // A proposal that clears a value is still a proposal — `undefined` must be
  // applied, not skipped, or clearing a trigger field would evaluate against
  // its old value.
  it('applies an undefined value rather than ignoring it', () => {
    const current = { form: { route: 'aanbesteding' } }
    const next = proposedBindings(current, proposal('route', undefined, 'aanbesteding'))
    expect('route' in (next.form as Record<string, unknown>)).toBe(true)
    expect((next.form as Record<string, unknown>).route).toBeUndefined()
  })
})

describe('wouldHide', () => {
  const managed = new Set(['route', 'deadline', 'vragenronde'])

  it('reports a property that is visible now but not after', () => {
    const now = new Set(['route', 'deadline'])
    const after = new Set(['route'])
    expect(wouldHide(now, after, managed)).toEqual(['deadline'])
  })

  it('reports every property the change hides, not just the first', () => {
    const now = new Set(['route', 'deadline', 'vragenronde'])
    const after = new Set(['route'])
    expect(wouldHide(now, after, managed).sort()).toEqual(['deadline', 'vragenronde'])
  })

  it('reports nothing when visibility is unchanged', () => {
    const now = new Set(['route', 'deadline'])
    expect(wouldHide(now, new Set(now), managed)).toEqual([])
  })

  // A reveal is not a hide. The reveal path restores retained values and must
  // not be confused with the destructive one.
  it('ignores properties that become visible', () => {
    const now = new Set(['route'])
    const after = new Set(['route', 'deadline'])
    expect(wouldHide(now, after, managed)).toEqual([])
  })

  // An unmanaged key is one the wizard does not govern — e.g. a metamodel
  // default seeded into form state but surfaced in no step. Pruning it would
  // destroy a value the form never chose to show.
  it('ignores a property the wizard does not manage', () => {
    const now = new Set(['route', 'stray'])
    const after = new Set(['route'])
    expect(wouldHide(now, after, managed)).toEqual([])
  })

  it('returns a plain array so the caller can batch one dialog', () => {
    const now = new Set(['deadline', 'vragenronde'])
    expect(Array.isArray(wouldHide(now, new Set(), managed))).toBe(true)
  })
})
