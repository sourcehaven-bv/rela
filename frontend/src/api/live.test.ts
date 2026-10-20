import { describe, it, expect, beforeEach } from 'vitest'
import { registerEntityPlurals } from './entities'
import { api } from './client'
import {
  liveChannelPath,
  liveChannelsPath,
  liveEntityPath,
  liveListPath,
  livePresencePath,
  liveViewPath,
} from './live'

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

  it('does not subscribe a list with a free-text search', () => {
    expect(liveListPath('ticket', { q: 'text', per_page: 20 })).toBeNull()
    expect(liveListPath('ticket', { q: '', per_page: 20 })).toBe('/api/v1/tickets?per_page=20')
  })

  it('builds a view path', () => {
    expect(liveViewPath('ticket', 'TKT-1')).toBe('/api/v1/_views/ticket/TKT-1')
    expect(liveViewPath('ticket', 'TKT-1', 'pub')).toBe('/api/v1/_views/ticket/TKT-1?world=pub')
  })

  it('builds a presence path', () => {
    expect(livePresencePath('ticket', 'TKT-1')).toBe('/api/v1/tickets/TKT-1/_presence')
    expect(livePresencePath('ticket', 'TKT-1@draft', 'pub')).toBe(
      '/api/v1/tickets/TKT-1@draft/_presence?world=pub'
    )
  })

  it('builds channel list and channel paths', () => {
    const anchor = { type: 'ticket', address: 'TKT-1@draft', world: 'main' }
    expect(liveChannelsPath({ type: 'ticket', address: 'TKT-1' })).toBe(
      '/api/v1/tickets/TKT-1/_channels'
    )
    expect(liveChannelsPath(anchor, 'open')).toBe(
      '/api/v1/tickets/TKT-1@draft/_channels?state=open&world=main'
    )
    expect(liveChannelPath({ ...anchor, channelType: 'room' })).toBe(
      '/api/v1/tickets/TKT-1@draft/_channels/room?world=main'
    )
    expect(
      liveChannelPath({ type: 'ticket', address: 'TKT-1', channelType: 'threads', name: 'x1' })
    ).toBe('/api/v1/tickets/TKT-1/_channels/threads/x1')
  })
})
