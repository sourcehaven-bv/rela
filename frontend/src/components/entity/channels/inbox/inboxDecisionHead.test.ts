import { describe, it, expect } from 'vitest'
import type { InboxEntry, Message } from '@/types/channels'
import { inboxHead, inboxMessages, newerCount, offersChoices } from './inboxDecisionHead'

const ask: Message = {
  seq: 41,
  rev: 41,
  type: 'ask',
  author: { user: 'runner' },
  at: '2026-11-01T09:00:00Z',
  props: { question: 'Which?', options: ['A', 'B'] },
}

function entry(over: Partial<InboxEntry> = {}): InboxEntry {
  return {
    kind: 'waiting',
    channel: {
      anchor: 'TKT-2',
      entity_type: 'ticket',
      anchor_title: 'Two',
      type: 'run',
      state: 'open',
    },
    session: {
      state: 'asking',
      status: 'input_required',
      generation: 7,
      cause: 41,
      transitions: [{ message_type: 'answer', to: 'working', entry: 0 }],
    },
    roles: ['owner'],
    since: '2026-11-01T09:00:00Z',
    transitions: ['answer'],
    actions: {
      post: ['answer', 'note'],
      transitions: ['answer'],
      entries: [{ message_type: 'answer', entry: 0 }],
    },
    last_seq: 42,
    message: ask,
    links: { entity: '', channel: '' },
    ...over,
  }
}

describe('inboxHead', () => {
  it('carries the decision inputs of the entry', () => {
    const h = inboxHead(entry())
    expect(h.channel.state).toBe('open')
    expect(h.channel.last_seq).toBe(42)
    expect(h.channel.session).toMatchObject({
      state: 'asking',
      generation: 7,
      cause: 41,
      transitions: [{ message_type: 'answer', to: 'working', entry: 0 }],
    })
    expect(h._actions).toMatchObject({
      post: ['answer', 'note'],
      transitions: ['answer'],
      entries: [{ message_type: 'answer', entry: 0 }],
      lifecycle: [],
      typing: false,
    })
  })

  it('reads an entry without decision inputs as offering nothing', () => {
    const e = entry()
    const bare = { ...e, actions: undefined } as unknown as InboxEntry
    expect(inboxHead(bare)._actions.post).toEqual([])
  })
})

describe('inboxMessages', () => {
  it('holds the waiting message or nothing', () => {
    expect(inboxMessages(entry())).toEqual([ask])
    expect(inboxMessages(entry({ message: undefined }))).toEqual([])
  })
})

describe('offersChoices', () => {
  it.each([
    { name: 'a fresh question', m: ask, want: true },
    { name: 'no message', m: undefined, want: false },
    { name: 'a reply', m: { ...ask, reply_to: 3 }, want: false },
    { name: 'already replied to', m: { ...ask, reply_count: 1 }, want: false },
    {
      name: 'removed',
      m: {
        ...ask,
        props: undefined,
        removed: { kind: 'author' as const, at: '2026-11-01T09:00:00Z' },
      },
      want: false,
    },
  ])('$name', ({ m, want }) => {
    expect(offersChoices(entry({ message: m }))).toBe(want)
  })
})

describe('newerCount', () => {
  it.each([
    { name: 'only the notice follows', last: 42, cause: 41, want: 0 },
    { name: 'two messages follow', last: 44, cause: 41, want: 2 },
    { name: 'no cause', last: 44, cause: undefined, want: 0 },
  ])('$name', ({ last, cause, want }) => {
    const e = entry({ last_seq: last })
    e.session.cause = cause
    expect(newerCount(e)).toBe(want)
  })
})
