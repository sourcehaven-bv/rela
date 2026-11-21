import { describe, it, expect } from 'vitest'
import { entityRef, refBareId, refFace } from './entityRef'

describe('entityRef', () => {
  it('reads the address off _self, face included', () => {
    expect(entityRef({ id: 'POL-1', _self: '/api/v1/policys/POL-1@published' })).toBe('POL-1@published')
  })

  it('is the bare id for a bare face', () => {
    expect(entityRef({ id: 'POL-1', _self: '/api/v1/policys/POL-1' })).toBe('POL-1')
  })

  it('falls back to the id when the response carries no _self', () => {
    // An older server, or a synthetic entity: the pre-worlds behaviour.
    expect(entityRef({ id: 'POL-1' })).toBe('POL-1')
  })

  it('decodes a percent-encoded segment', () => {
    expect(entityRef({ id: 'POL-1', _self: '/api/v1/policys/POL-1%40nl' })).toBe('POL-1@nl')
  })

  it('splits an address into id and face', () => {
    expect(refBareId('POL-1@published')).toBe('POL-1')
    expect(refFace('POL-1@published')).toBe('published')
    expect(refBareId('POL-1')).toBe('POL-1')
    expect(refFace('POL-1')).toBe('')
  })
})
