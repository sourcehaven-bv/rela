import { describe, it, expect } from 'vitest'
import { inboxRoute, inboxRowKey } from './inboxRoutes'
import type { InboxChannel } from '@/types/channels'

const c = (over: Partial<InboxChannel> = {}): InboxChannel => ({
  anchor: 'TKT-1',
  entity_type: 'ticket',
  anchor_title: 'Fix login',
  type: 'chat',
  state: 'open',
  ...over,
})

describe('inboxRoutes', () => {
  it('keys a channel by type, anchor and channel key', () => {
    expect(inboxRowKey(c())).toBe('ticket/TKT-1/chat/')
    expect(inboxRowKey(c({ name: 'ops' }))).toBe('ticket/TKT-1/chat/ops')
  })

  it('routes to the channel on the anchor page, with the seq when given', () => {
    expect(inboxRoute(c())).toBe('/entity/ticket/TKT-1?channel=chat%2F')
    expect(inboxRoute(c({ name: 'ops' }), 7)).toBe('/entity/ticket/TKT-1?channel=chat%2Fops&seq=7')
  })

  it('encodes the anchor and channel name', () => {
    expect(inboxRoute(c({ anchor: 'a/b?x', name: 'a&b' }))).toBe(
      '/entity/ticket/a%2Fb%3Fx?channel=chat%2Fa%26b'
    )
  })
})
