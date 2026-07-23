import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getTransforms, entityExportUrl, listExportUrl } from './transforms'
import { registerEntityPlurals } from './entities'
import { api } from './client'

vi.mock('./client', () => ({
  api: { get: vi.fn().mockResolvedValue([{ name: 'pdf', produces: 'application/pdf' }]) },
}))

describe('transforms api', () => {
  beforeEach(() => vi.clearAllMocks())

  it('getTransforms hits /_transforms', async () => {
    const got = await getTransforms()
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_transforms')
    expect(got).toEqual([{ name: 'pdf', produces: 'application/pdf' }])
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
})
