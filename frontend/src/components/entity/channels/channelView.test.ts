// @vitest-environment jsdom
//
// jsdom because the markdown cases run the real DOMPurify, which needs
// browser-accurate serialization (BUG-SQSV6V); under happy-dom it lets
// everything through.
import { describe, it, expect, vi } from 'vitest'
import type {
  Channel,
  ChannelHead,
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
  composerTransitions,
  editingFor,
  enumOptionsFor,
  ephemeralViews,
  typingViews,
  excerptOf,
  lifecycleDefs,
  messageViews,
  nextExpiry,
  noticeView,
  parseChannelKey,
  presentationView,
  sessionView,
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

  it('passes the message mentions to the renderer (TKT-6RXAC0)', () => {
    const m: Message = {
      seq: 5,
      rev: 5,
      type: 'card',
      props: { body: 'ask `usr-7` and `@jeroen`' },
      mentions: [
        { text: 'usr-7', kind: 'user', ref: { id: 'USR-7', type: 'user', title: 'Ann' } },
        { text: '@jeroen', kind: 'login' },
      ],
    }
    renderSpy.mockClear()
    const [v] = messageViews([m], byName, ctx)
    expect(renderSpy).toHaveBeenCalledWith('ask `usr-7` and `@jeroen`', {
      refs: [],
      mentions: [
        { text: 'usr-7', kind: 'user', ref: { id: 'USR-7', type: 'user', title: 'Ann' } },
        { text: '@jeroen', kind: 'login', ref: undefined },
      ],
      world: undefined,
    })
    expect(v.html?.body).toContain('entity/user/USR-7')
    expect(v.html?.body).toContain('<span class="mention">@jeroen</span>')
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

describe('channelGroups unread', () => {
  it('carries the unread count of a channel with a marker, and none at 0', () => {
    const c = { anchor: 'F-1', type: 'thread', state: 'open' as const, last_seq: 42, rev: 57 }
    const groups = channelGroups(
      [
        {
          type: 'thread',
          cardinality: 'many',
          create: false,
          channels: [
            { ...c, name: 'a', read: { seq: 39, unread: 2 } },
            { ...c, name: 'b', read: { seq: 42, unread: 0 } },
            { ...c, name: 'c' },
          ],
        },
      ],
      () => undefined
    )
    expect(groups[0].channels).toEqual([
      { key: 'thread/a', title: 'a', state: 'open', unread: 2 },
      { key: 'thread/b', title: 'b', state: 'open' },
      { key: 'thread/c', title: 'c', state: 'open' },
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

describe('sessions', () => {
  const breach: ChannelTypeSchema = {
    ...room,
    name: 'breach',
    message_types: [
      ...['contained', 'dismiss', 'assessed', 'text'].map((name) => ({
        name,
        label: name,
        persist: 'log' as const,
        reply: false,
        actions: [],
        properties: [],
      })),
    ],
    session: {
      label: 'GDPR breach response',
      initial: 'detected',
      states: [
        { name: 'detected', label: 'Detected', status: 'working' },
        {
          name: 'contained',
          label: 'Contained',
          status: 'input_required',
          waiting_for: ['lead'],
        },
        { name: 'assessed', label: 'Risk assessed', status: 'working' },
        { name: 'notified', label: 'AP notified', final: true, status: 'completed' },
        { name: 'dismissed', label: 'Dismissed', final: true, status: 'cancelled' },
      ],
    },
  }
  const at = '2026-10-11T09:00:00Z'
  const inDetected: NonNullable<Channel['session']> = {
    state: 'detected',
    status: 'working',
    generation: 0,
    entered_at: at,
    transitions: [
      { message_type: 'contained', to: 'contained' },
      { message_type: 'dismiss', to: 'dismissed' },
      { message_type: 'escalated', to: 'contained', by: 'trigger' },
    ],
  }
  function channelWith(session: Channel['session'], state: Channel['state'] = 'open'): Channel {
    return { anchor: 'TKT-1', type: 'breach', state, last_seq: 1, rev: 1, session }
  }
  function headWith(
    session: Channel['session'],
    transitions: string[],
    post = ['contained', 'dismiss', 'assessed', 'text']
  ): Pick<ChannelHead, 'channel' | '_actions'> {
    return {
      channel: channelWith(session),
      _actions: {
        post,
        edit_own: true,
        delete_own: true,
        remove: false,
        lifecycle: [],
        typing: false,
        transitions,
      },
    }
  }

  describe('sessionView', () => {
    it('marks the current state and the distinct next states', () => {
      expect(sessionView(breach, channelWith(inDetected))).toEqual({
        label: 'GDPR breach response',
        states: breach.session!.states.map(({ name, label, final }) => ({ name, label, final })),
        current: 'detected',
        currentLabel: 'Detected',
        next: ['contained', 'dismissed'],
        status: 'working',
      })
    })

    it.each([
      [
        'input required, waiting for a labelled role and an unlabelled one',
        {
          state: 'contained',
          status: 'input_required' as const,
          generation: 1,
          entered_at: at,
          waiting_for: [
            { role: 'lead', label: 'the incident lead', holders: ['USR-1'] },
            { role: 'starter' },
          ],
        },
        {
          status: 'input_required',
          waitingFor: [{ label: 'the incident lead' }, { label: 'starter' }],
        },
      ],
      [
        'waiting on me',
        {
          state: 'contained',
          status: 'input_required' as const,
          generation: 1,
          waiting_for: [{ role: 'lead' }],
          waiting_on_me: true,
        },
        { status: 'input_required', waitingOnMe: true },
      ],
      [
        'cancelled',
        {
          state: 'dismissed',
          status: 'cancelled' as const,
          generation: 1,
          entered_at: at,
          final: true,
        },
        { status: 'cancelled', final: true },
      ],
      [
        'failed when stranded',
        {
          state: 'triage',
          status: 'failed' as const,
          generation: 2,
          entered_at: at,
          stranded: true,
        },
        { status: 'failed', stranded: true },
      ],
    ])('maps the status: %s', (_name, session: NonNullable<Channel['session']>, want) => {
      const got = sessionView(breach, channelWith(session))!
      expect(got).toMatchObject(want)
      if (!session.waiting_on_me) expect(got.waitingOnMe).toBeUndefined()
      expect(JSON.stringify(got)).not.toContain('USR-1')
    })

    it.each([
      [
        'stranded wins over paused',
        {
          state: 'triage',
          status: 'failed' as const,
          generation: 2,
          entered_at: at,
          stranded: true,
        },
        'closed' as const,
        { stranded: true, currentLabel: 'triage', paused: 'closed' },
      ],
      [
        'final on a closed channel',
        {
          state: 'notified',
          status: 'completed' as const,
          generation: 3,
          entered_at: at,
          final: true,
        },
        'closed' as const,
        { final: true, current: 'notified', currentLabel: 'AP notified', paused: 'closed' },
      ],
      [
        'paused on an archived channel',
        { state: 'contained', status: 'working' as const, generation: 1, entered_at: at },
        'archived' as const,
        { current: 'contained', paused: 'archived', next: [] },
      ],
    ])('%s', (_name, session, state, want) => {
      const got = sessionView(breach, channelWith(session, state))!
      expect(got).toMatchObject(want)
      if (got.stranded) expect(got.current).toBeUndefined()
      if (got.stranded) expect(got.final).toBeUndefined()
    })

    describe('a state the held descriptor lacks', () => {
      const added = {
        state: 'escalated',
        status: 'completed' as const,
        generation: 4,
        entered_at: at,
        final: true,
        transitions: [{ message_type: 'assessed', to: 'assessed' }],
      }

      it('is loading, not stranded, until a fresh descriptor answers', () => {
        const got = sessionView(breach, channelWith(added))!
        expect(got).toMatchObject({ loading: true, currentLabel: 'escalated', next: [] })
        expect(got.stranded).toBeUndefined()
        expect(got.current).toBeUndefined()
        expect(got.final).toBeUndefined()
      })

      it('is stranded when a fresh descriptor still lacks it', () => {
        const got = sessionView(breach, channelWith(added), (s) => s === 'escalated')!
        expect(got).toMatchObject({ stranded: true, currentLabel: 'escalated', next: [] })
        expect(got.loading).toBeUndefined()
      })

      it('is a known state again once the descriptor has it', () => {
        const got = sessionView(breach, channelWith(inDetected), () => true)!
        expect(got).toMatchObject({ current: 'detected' })
        expect(got.loading).toBeUndefined()
        expect(got.stranded).toBeUndefined()
      })

      it('marks no composer transition', () => {
        expect(composerTransitions(breach, headWith(added, ['assessed'], ['assessed']))).toEqual([])
      })
    })

    it('is undefined without a session on the type or the channel', () => {
      expect(sessionView(room, channelWith(inDetected))).toBeUndefined()
      expect(sessionView(breach, channelWith(undefined))).toBeUndefined()
    })
  })

  describe('noticeView', () => {
    const notice: Message = {
      seq: 3,
      rev: 3,
      type: 'session',
      author: { user: 'system', system: true },
      at,
      triggered_by: 'session:breach',
      props: { from: 'detected', to: 'contained', generation: 1, cause: 2 },
    }

    it('maps a session notice to its labels and names no waiting roles', () => {
      expect(noticeView(notice, breach.session)).toEqual({
        fromLabel: 'Detected',
        toLabel: 'Contained',
        reentry: false,
      })
      expect(
        noticeView({ ...notice, props: { from: 'contained', to: 'assessed' } }, breach.session)
      ).toEqual({ fromLabel: 'Contained', toLabel: 'Risk assessed', reentry: false })
    })

    it('marks a re-entry and falls back to the name of a removed state', () => {
      expect(
        noticeView({ ...notice, props: { from: 'triage', to: 'triage' } }, breach.session)
      ).toEqual({
        fromLabel: 'triage',
        toLabel: 'triage',
        reentry: true,
      })
    })

    it('does not take a message named session without the marker for a notice', () => {
      expect(noticeView({ ...notice, triggered_by: undefined }, breach.session)).toBeUndefined()
      expect(noticeView({ ...notice, triggered_by: 'trigger:x' }, breach.session)).toBeUndefined()
    })

    it('sets the notice on the message view and no card fields', () => {
      const [view] = messageViews([notice], () => undefined, { tz: 'UTC', session: breach.session })
      expect(view!.notice).toEqual({ fromLabel: 'Detected', toLabel: 'Contained', reentry: false })
      expect(view!.html).toBeUndefined()
    })
  })

  describe('composerTransitions', () => {
    it('keeps the fireable, postable transitions out of the current state', () => {
      const guarded = {
        ...inDetected,
        transitions: [
          ...inDetected.transitions!,
          { message_type: 'assessed', to: 'assessed', guarded: true },
        ],
      }
      expect(
        composerTransitions(
          breach,
          headWith(guarded, ['assessed', 'contained', 'dismiss'], ['contained', 'assessed'])
        )
      ).toEqual([
        { messageType: 'assessed', toLabel: 'Risk assessed', guarded: true },
        { messageType: 'contained', toLabel: 'Contained' },
      ])
    })

    it('drops types the session already left before the head refetch', () => {
      const moved = {
        state: 'contained',
        status: 'working' as const,
        generation: 1,
        entered_at: at,
        transitions: [{ message_type: 'assessed', to: 'assessed' }],
      }
      expect(composerTransitions(breach, headWith(moved, ['contained', 'dismiss']))).toEqual([])
    })

    it('is empty for a stranded session or a type without one', () => {
      expect(
        composerTransitions(breach, headWith({ ...inDetected, stranded: true }, ['contained']))
      ).toEqual([])
      expect(composerTransitions(room, headWith(inDetected, ['contained']))).toEqual([])
    })
  })
})
