import { describe, it, expect } from 'vitest'
import { isStandIn } from './standIn'
import type { EntityWorld } from '@/types'

const world = (w: Partial<EntityWorld>): EntityWorld => ({
  name: 'editorial',
  face: 'draft',
  via: 'chain',
  ...w,
})

describe('isStandIn', () => {
  it.each([
    ['no world', undefined, false],
    ['unscoped', world({ via: 'unscoped' }), false],
    ['chain prime', world({ via: 'chain', chain_position: 0 }), false],
    ['chain without position', world({ via: 'chain' }), false],
    ['later chain entry', world({ via: 'chain', chain_position: 1 }), true],
    ['fallback default', world({ via: 'fallback-default' }), true],
  ])('%s', (_name, w, want) => {
    expect(isStandIn(w)).toBe(want)
  })
})
