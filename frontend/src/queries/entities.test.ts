import { describe, it, expect } from 'vitest'
import { entityKeys, canonicalListParams } from './entities'
import type { ListParams } from '@/types'

describe('canonicalListParams', () => {
  it('is order-insensitive (the param-order cache-key bug)', () => {
    const a: ListParams = { page: 1, per_page: 25, sort: 'title' }
    const b: ListParams = { sort: 'title', per_page: 25, page: 1 }
    expect(canonicalListParams(a)).toBe(canonicalListParams(b))
  })

  it('drops undefined and empty values so they do not split the cache', () => {
    const bare = canonicalListParams({ page: 1 } as ListParams)
    expect(canonicalListParams({ page: 1, q: '' } as ListParams)).toBe(bare)
    expect(canonicalListParams({ page: 1, sort: undefined } as ListParams)).toBe(bare)
  })

  it('returns empty string for no params', () => {
    expect(canonicalListParams()).toBe('')
    expect(canonicalListParams({} as ListParams)).toBe('')
  })

  it('does not collide when a value contains & or = (the delimiter bug)', () => {
    // Two distinct filter sets that a naive k=v&... join would conflate.
    const a = canonicalListParams({ 'filter[a]': 'x', 'filter[b]': 'y' } as ListParams)
    const b = canonicalListParams({ 'filter[a]': 'x&filter[b]=y' } as ListParams)
    expect(a).not.toBe(b)
  })

  it('distinguishes different param sets', () => {
    expect(canonicalListParams({ page: 1 } as ListParams)).not.toBe(
      canonicalListParams({ page: 2 } as ListParams)
    )
  })
})

describe('entityKeys', () => {
  it('list is a prefix of every listParams variant (so it invalidates all)', () => {
    const base = entityKeys.list('ticket')
    const withParams = entityKeys.listParams('ticket', { page: 2 } as ListParams)
    expect(withParams.slice(0, base.length)).toEqual([...base])
  })

  it('type is a prefix of list and detail (SSE invalidation by type)', () => {
    const type = entityKeys.type('ticket')
    expect(entityKeys.list('ticket').slice(0, type.length)).toEqual([...type])
    expect(entityKeys.detail('ticket', 'T-1').slice(0, type.length)).toEqual([...type])
  })
})
