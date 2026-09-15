import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getTransforms, entityExportUrl, listExportUrl, documentExportUrl } from './transforms'
import { registerEntityPlurals } from './entities'
import { api } from './client'

vi.mock('./client', () => ({
  api: { get: vi.fn().mockResolvedValue([{ name: 'pdf', produces: 'application/pdf' }]) },
}))

describe('transforms api', () => {
  beforeEach(() => vi.clearAllMocks())

  it('getTransforms hits /_transforms once and caches for the session', async () => {
    const got = await getTransforms()
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_transforms')
    expect(got).toEqual([{ name: 'pdf', produces: 'application/pdf' }])
    // The registry is static per server config; a second mount must reuse the
    // cached fetch rather than re-request it on every navigation.
    await getTransforms()
    expect(vi.mocked(api.get)).toHaveBeenCalledTimes(1)
  })

  it('entityExportUrl resolves plural and encodes the transform', () => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    const url = entityExportUrl('ticket', 'TKT-001', 'pdf')
    expect(url).toBe('/api/v1/tickets/TKT-001/_export?transform=pdf')
  })

  it('entityExportUrl encodes special characters', () => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    const url = entityExportUrl('ticket', 'TKT 1&x', 'pdf')
    expect(url).toContain('TKT%201%26x')
  })

  it('listExportUrl includes transform, list id, and forwarded params', () => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    const extra = new URLSearchParams({ 'filter[status]': 'open', q: 'foo' })
    const url = listExportUrl('ticket', 'tickets', 'pdf', extra)
    expect(url.startsWith('/api/v1/tickets/_export?')).toBe(true)
    expect(url).toContain('transform=pdf')
    expect(url).toContain('list=tickets')
    expect(url).toContain('filter%5Bstatus%5D=open')
    expect(url).toContain('q=foo')
  })

  it('documentExportUrl targets the anchored route when an entity id is given', () => {
    const url = documentExportUrl('release_notes', 'TKT-001', 'pdf')
    expect(url).toBe('/api/v1/_documents/release_notes/TKT-001/_export?transform=pdf')
  })

  it('documentExportUrl omits the entity segment for a standalone document', () => {
    const url = documentExportUrl('sales_review', undefined, 'pdf')
    expect(url).toBe('/api/v1/_documents/sales_review/_export?transform=pdf')
  })

  it('documentExportUrl encodes both segments', () => {
    const url = documentExportUrl('a b&c', 'TKT 1&x', 'pdf')
    expect(url).toContain('a%20b%26c')
    expect(url).toContain('TKT%201%26x')
    // The reserved segment stays literal — it is the route, not a value.
    expect(url.endsWith('/_export?transform=pdf')).toBe(true)
  })
})
