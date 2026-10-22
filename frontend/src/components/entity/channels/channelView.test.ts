// @vitest-environment jsdom
//
// jsdom because the markdown cases run the real DOMPurify, which needs
// browser-accurate serialization (BUG-SQSV6V); under happy-dom it lets
// everything through.
import { describe, it, expect, vi } from 'vitest'
import type {
  ChannelMessageTypeSchema,
  ChannelTypeSchema,
  EphemeralMessage,
  Message,
} from '@/types/channels'
import {
  EXCERPT_MAX,
  authorView,
  channelGroups,
  channelKey,
  editingFor,
  enumOptionsFor,
  ephemeralViews,
  typingViews,
  excerptOf,
  lifecycleDefs,
  messageViews,
  nextExpiry,
  parseChannelKey,
  presentationView,
} from './channelView'

const renderSpy = vi.hoisted(() => vi.fn())
vi.mock('@/utils/markdown', async (orig) => {
  const actual = await orig<typeof import('@/utils/markdown')>()
  return {
    ...actual,
    renderMessageMarkdown: (...a: Parameters<typeof actual.renderMessageMarkdown>) => {
      renderSpy(...a)
      return actual.renderMessageMarkdown(...a)
    },
  }
})

const card: ChannelMessageTypeSchema = {
  name: 'card',
  label: 'Card',
  persist: 'log',
  reply: true,
  actions: [],
  properties: [
    { name: 'body', label: 'Body', type: 'markdown', required: true },
    { name: 'due', label: 'Due', type: 'datetime' },
    { name: 'day', label: 'Day', type: 'date' },
    { name: 'owner', label: 'Owner', type: 'ref', to: ['person'] },
    { name: 'tone', label: 'Tone', type: 'enum', enum: 'tone', values: ['calm', 'loud'] },
  ],
}
const typing: ChannelMessageTypeSchema = {
  name: 'typing',
  label: 'Typing',
  persist: 'ephemeral',
  reply: false,
  actions: [],
  properties: [
    { name: 'tone', label: 'Tone', type: 'enum', enum: 'tone', values: ['calm', 'loud'] },
  ],
}
const room: ChannelTypeSchema = {
  name: 'room',
  label: 'Room',
  attach_to: ['feature'],
  anchor: 'family',
  cardinality: 'one',
  remove_reason_required: false,
  typing: false,
  lifecycle: [
    { name: 'close', label: 'Close' },
    { name: 'delete', label: 'Delete' },
  ],
  message_types: [card, typing],
}
const byName = (n: string) => room.message_types.find((m) => m.name === n)
const ctx = { tz: 'UTC' }

describe('authorView', () => {
  it.each([
    [{ user: 'ann' }, { name: 'ann', kind: 'person' }],
    [
      { user: 'bot', principal_type: 'agent' },
      { name: 'bot', kind: 'agent' },
    ],
    [
      { user: 'ann', tool: 'mcp' },
      { name: 'ann', kind: 'agent' },
    ],
    [
      { user: '', system: true },
      { name: 'system', kind: 'system' },
    ],
  ])('%o', (author, want) => {
    expect(authorView(author)).toEqual(want)
  })
})

describe('lifecycleDefs', () => {
  it('draws delete in the danger tone only', () => {
    expect(lifecycleDefs(room)).toEqual([
      { name: 'close', label: 'Close', danger: false },
      { name: 'delete', label: 'Delete', danger: true },
    ])
  })
})

describe('messageViews', () => {
  const m1: Message = {
    seq: 1,
    rev: 1,
    type: 'card',
    author: { user: 'ann' },
    at: '2026-03-01T10:00:00Z',
    props: {
      body: 'See `P-1` <img src=x onerror=alert(1)>',
      due: '2026-03-02T14:30:00Z',
      day: '2026-03-05',
      owner: { id: 'P-1', type: 'person', title: 'Pat' },
    },
  }
  const m2: Message = { seq: 2, rev: 2, type: 'card', reply_to: 1, props: { body: 'agreed' } }

  it('renders markdown through the strict renderer and links readable refs', () => {
    const [v] = messageViews([m1], byName, ctx)
    expect(v.html?.body).toContain('class="entity-ref"')
    expect(v.html?.body).toContain('entity/person/P-1')
    expect(v.html?.body).not.toContain('<img')
    expect(v.html?.body).not.toContain('onerror')
  })

  it('formats dates and times in the given zone', () => {
    const [v] = messageViews([m1], byName, ctx)
    expect(v.text?.due).toBe('Mar 2, 2026, 2:30 PM')
    expect(v.text?.day).toMatch(/2026/)
    expect(v.at).toBe('Mar 1, 2026, 10:00 AM')
    expect(v.datetime).toBe('2026-03-01T10:00:00Z')
  })

  it('quotes the original of a reply from the held list', () => {
    const [, v2] = messageViews([m1, m2], byName, ctx)
    expect(v2.replyTo).toEqual({ seq: 1, author: 'ann', excerpt: expect.stringContaining('See') })
  })

  it('quotes a reply whose original is not held by seq only', () => {
    const [v] = messageViews([m2], byName, ctx)
    expect(v.replyTo).toEqual({ seq: 1, author: undefined, excerpt: undefined })
  })

  it('renders a revision once (R19)', () => {
    const memo = new Map<string, string>()
    renderSpy.mockClear()
    messageViews([m1], byName, ctx, memo)
    messageViews([m1], byName, ctx, memo)
    expect(renderSpy).toHaveBeenCalledTimes(1)
    messageViews([{ ...m1, rev: 3 }], byName, ctx, memo)
    expect(renderSpy).toHaveBeenCalledTimes(2)
  })

  it('keeps memo entries for held revisions only', () => {
    const memo = new Map<string, string>()
    messageViews([m1, m2], byName, ctx, memo)
    expect([...memo.keys()].sort()).toEqual(['1:1:body:', '2:2:body:'])
    // m1 edited, m2 dropped by the held-list cap.
    messageViews([{ ...m1, rev: 3 }], byName, ctx, memo)
    expect([...memo.keys()]).toEqual(['1:3:body:'])
  })

  it('shows a removed message as its removal only', () => {
    const removed: Message = {
      seq: 3,
      rev: 4,
      type: 'card',
      removed: {
        at: '2026-03-01T11:00:00Z',
        kind: 'moderator',
        by: { user: 'mod' },
        reason: 'spam',
      },
    }
    const [v] = messageViews([removed], byName, ctx)
    expect(v.removed).toEqual({
      kind: 'moderator',
      by: 'mod',
      reason: 'spam',
      at: expect.any(String),
    })
    expect(v.props).toBeUndefined()
    expect(v.html).toBeUndefined()
  })
})

describe('excerptOf', () => {
  it('collapses whitespace and cuts at EXCERPT_MAX code points', () => {
    const long = 'é'.repeat(EXCERPT_MAX + 10)
    const out = excerptOf({ seq: 1, rev: 1, type: 'card', props: { body: `a\n\n${long}` } }, card)
    expect([...(out ?? '')].length).toBe(EXCERPT_MAX)
    expect(out?.startsWith('a é')).toBe(true)
    expect(out?.endsWith('…')).toBe(true)
  })
})

describe('editingFor', () => {
  it('prefills values and names restricted refs', () => {
    const m: Message = {
      seq: 5,
      rev: 5,
      type: 'card',
      props: { body: 'hi', owner: { restricted: true }, tone: 'calm' },
    }
    expect(editingFor(m, card)).toEqual({
      seq: 5,
      type: 'card',
      values: { body: 'hi', due: undefined, day: undefined, owner: undefined, tone: 'calm' },
      refs: {},
      restricted: ['owner'],
    })
  })

  it('carries a readable ref for the chip', () => {
    const m: Message = {
      seq: 6,
      rev: 6,
      type: 'card',
      props: { owner: { id: 'P-1', type: 'person', title: 'Pat' } },
    }
    const e = editingFor(m, card)
    expect(e.values.owner).toBe('P-1')
    expect(e.refs).toEqual({ owner: { id: 'P-1', type: 'person', title: 'Pat' } })
    expect(e.restricted).toEqual([])
  })
})

describe('enumOptionsFor', () => {
  it('takes labels from the custom type and colours from the styles', () => {
    const out = enumOptionsFor(
      room,
      new Map([['tone', { values: ['calm', 'loud'], labels: { loud: 'Loud!' } }]]),
      { tone: { loud: 'badge-red' } }
    )
    expect(out).toEqual({
      tone: [
        { value: 'calm', label: 'calm', color: undefined },
        { value: 'loud', label: 'Loud!', color: 'red' },
      ],
    })
  })
})

describe('ephemeral signals', () => {
  const signals: EphemeralMessage[] = [
    {
      id: 'a',
      type: 'typing',
      author: { user: 'ann' },
      at: '',
      expires_at: '2026-03-01T10:00:05Z',
      props: { tone: 'loud' },
    },
    {
      id: 'b',
      type: 'typing',
      author: { user: 'bob' },
      at: '',
      expires_at: '2026-03-01T10:00:10Z',
    },
  ]
  const at = (s: string) => Date.parse(s)
  const opts = { tone: [{ value: 'loud', label: 'Loud!' }] }

  it('shows only signals that have not expired, with app-built text', () => {
    const live = ephemeralViews(signals, at('2026-03-01T10:00:06Z'), byName, undefined, opts, 'UTC')
    expect(live.map((s) => s.id)).toEqual(['b'])
    const both = ephemeralViews(signals, at('2026-03-01T10:00:00Z'), byName, undefined, opts, 'UTC')
    expect(both[0]).toMatchObject({
      id: 'a',
      text: 'Loud!',
      author: { name: 'ann', kind: 'person' },
    })
  })

  it('finds the soonest expiry still ahead', () => {
    expect(nextExpiry(signals, at('2026-03-01T10:00:00Z'))).toBe(at('2026-03-01T10:00:05Z'))
    expect(nextExpiry(signals, at('2026-03-01T10:00:06Z'))).toBe(at('2026-03-01T10:00:10Z'))
    expect(nextExpiry(signals, at('2026-03-01T10:00:10Z'))).toBeUndefined()
  })
})

describe('typingViews', () => {
  it('maps typing_now as it is, in its order, with no expiry', () => {
    const got = typingViews([
      { author: { user: 'bo', tool: 'data-entry' }, since: '2020-01-01T00:00:00Z' },
      { author: { user: 'ann', tool: 'mcp' }, since: '2020-01-01T00:00:01Z' },
    ])
    expect(got.map((v) => v.author)).toEqual([
      { name: 'bo', kind: 'person' },
      { name: 'ann', kind: 'agent' },
    ])
    expect(new Set(got.map((v) => v.key)).size).toBe(2)
  })

  it('gives one user in two organizations distinct keys', () => {
    const entry = { author: { user: 'ann' }, since: '2020-01-01T00:00:00Z' }
    const got = typingViews([entry, { ...entry }, { ...entry, since: '2020-01-01T00:00:01Z' }])
    expect(new Set(got.map((v) => v.key)).size).toBe(3)
  })

  it('is empty without typing_now', () => {
    expect(typingViews(undefined)).toEqual([])
    expect(typingViews([])).toEqual([])
  })
})

describe('channelGroups', () => {
  it('adds a row for a `one` type whose channel was never written', () => {
    const groups = channelGroups(
      [{ type: 'room', cardinality: 'one', create: false, channels: [] }],
      (n) => (n === 'room' ? room : undefined)
    )
    expect(groups).toEqual([
      {
        type: 'room',
        label: 'Room',
        cardinality: 'one',
        canCreate: false,
        createLabel: undefined,
        channels: [{ key: channelKey('room'), title: 'Room', state: 'open' }],
      },
    ])
  })

  it('titles a named channel by its title, then its name', () => {
    const groups = channelGroups(
      [
        {
          type: 'thread',
          label: 'Threads',
          cardinality: 'many',
          create: true,
          channels: [
            {
              anchor: 'F-1',
              type: 'thread',
              name: 't1',
              title: 'Launch',
              state: 'open',
              last_seq: 0,
              rev: 0,
            },
            { anchor: 'F-1', type: 'thread', name: 't2', state: 'closed', last_seq: 0, rev: 0 },
          ],
        },
      ],
      () => undefined
    )
    expect(groups[0].channels).toEqual([
      { key: 'thread/t1', title: 'Launch', state: 'open' },
      { key: 'thread/t2', title: 't2', state: 'closed' },
    ])
  })
})

describe('parseChannelKey', () => {
  it('round-trips channelKey', () => {
    expect(parseChannelKey(channelKey('room'))).toEqual({ channelType: 'room' })
    expect(parseChannelKey(channelKey('thread', 'a/b'))).toEqual({
      channelType: 'thread',
      name: 'a/b',
    })
  })
})

describe('presentationView', () => {
  it('drops a field that names no property', () => {
    expect(
      presentationView({
        messages: { card: { fields: [{ property: 'body', label: 'Text' }, { label: 'x' }] } },
      })
    ).toEqual({ messages: { card: { fields: [{ property: 'body', label: 'Text' }] } } })
  })
})
