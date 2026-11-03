import { describe, it, expect } from 'vitest'
import { addressSegment } from './entityAddress'

// An address goes into a request path as one segment. One that could move
// the request to another API path is refused (TKT-91BK6F security review).
describe('addressSegment', () => {
  it.each(['TKT-1', 'TKT-1@draft', 'a.b_c-d', 'X@y.z'])('accepts %s', (a) => {
    expect(addressSegment(a)).toBe(a)
  })

  it.each([
    '',
    '../_piles/x?y=',
    'TKT-1/x',
    'TKT-1?x',
    'TKT-1#x',
    'TKT%2F1',
    '.x',
    'a@',
    '@b',
    'a@b@c',
  ])('refuses %j', (a) => {
    expect(() => addressSegment(a)).toThrow(/not an entity address/)
  })
})
