import { describe, it, expect, beforeEach } from 'vitest'
import { registerEntityPlurals } from './entities'
import { api } from './client'
import { liveEntityPath, liveListPath, liveViewPath } from './live'

// A subscription must name the request the page actually made, so each
// helper is checked against what the API client itself would send.
describe('live subscription paths', () => {
  beforeEach(() => registerEntityPlurals(new Map([['ticket', 'tickets']])))

  function sent(url: string, params?: object): string {
    return api.getAxios().getUri({ baseURL: '/api/v1', url, params })
  }

  it('builds an entity path', () => {
    expect(liveEntityPath('ticket', 'TKT-1')).toBe('/api/v1/tickets/TKT-1')
    expect(liveEntityPath('ticket', 'TKT-1', { include: 'implements', world: 'pub' })).toBe(
      sent('/tickets/TKT-1', { include: 'implements', world: 'pub' })
    )
  })

  it('builds a list path with filters and drops unset params', () => {
    const path = liveListPath('ticket', {
      per_page: 20,
      'filter[status]': 'open',
      world: undefined,
    })
    expect(path).toBe('/api/v1/tickets?per_page=20&filter%5Bstatus%5D=open')
  })

  it('builds a view path', () => {
    expect(liveViewPath('ticket', 'TKT-1')).toBe('/api/v1/_views/ticket/TKT-1')
    expect(liveViewPath('ticket', 'TKT-1', 'pub')).toBe('/api/v1/_views/ticket/TKT-1?world=pub')
  })
})
