import { describe, it, expect } from 'vitest'
import { makeRefResolver } from './entityRefResolver'

describe('makeRefResolver', () => {
  it('returns undefined when the response carried no mentions', () => {
    expect(makeRefResolver(undefined)).toBeUndefined()
    expect(makeRefResolver(null)).toBeUndefined()
  })

  it('resolves an id present in the map', () => {
    const r = makeRefResolver({ 'TKT-1': { type: 'ticket', title: 'A ticket' } })!
    expect(r('TKT-1')).toEqual({
      type: 'ticket',
      title: 'A ticket',
      inaccessible: undefined,
      inaccessibleReason: undefined,
    })
  })

  it('carries the current id of a renamed entity', () => {
    const r = makeRefResolver({ 'FEAT-1': { id: 'TKT-9', type: 'ticket', title: 'Converted' } })!
    expect(r('FEAT-1')).toMatchObject({ entityId: 'TKT-9', type: 'ticket' })
  })

  it('returns null for an id the server declined to resolve', () => {
    // An entity the principal may not read has no entry at all. The caller
    // must render the bare id, not invent a title.
    const r = makeRefResolver({ 'TKT-1': { type: 'ticket', title: 'A' } })!
    expect(r('TKT-HIDDEN')).toBeNull()
  })

  it('carries the inaccessible flag and reason through', () => {
    const r = makeRefResolver({
      'TKT-1': {
        type: 'ticket',
        title: 'TKT-1',
        inaccessible: true,
        inaccessible_reason: 'git-crypt',
      },
    })!
    expect(r('TKT-1')).toMatchObject({
      inaccessible: true,
      inaccessibleReason: 'git-crypt',
    })
  })

  it('distinguishes an empty map from an absent one', () => {
    const r = makeRefResolver({})
    expect(r).toBeDefined()
    expect(r!('anything')).toBeNull()
  })
})
