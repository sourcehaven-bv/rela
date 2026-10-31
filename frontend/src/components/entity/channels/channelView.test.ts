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
  ChannelSessionTransition,
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
  decisionCause,
  decisionReplyTo,
  decisionViews,
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
  rowDecisions,
  sessionView,
  choiceViews,
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

  it('passes display', () => {
    expect(
      presentationView({
        messages: { claim: { display: 'hidden' }, progress: { display: 'collapsed' }, card: {} },
      })
    ).toEqual({
      messages: {
        claim: { fields: undefined, display: 'hidden' },
        progress: { fields: undefined, display: 'collapsed' },
        card: { fields: undefined },
      },
    })
  })

  it('drops an unknown display, so the messages show in full', () => {
    const p = { messages: { claim: { display: 'folded' } } } as unknown as Parameters<
      typeof presentationView
    >[0]
    expect(presentationView(p)).toEqual({ messages: { claim: { fields: undefined } } })
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
        { messageType: 'assessed', toLabels: ['Risk assessed'], guarded: true },
        { messageType: 'contained', toLabels: ['Contained'] },
      ])
    })

    // TKT-FPXTQJ AC 11: a type with several entries is one view with
    // every target once, in the server's order.
    it('groups the entries of one type into one view with each target once', () => {
      const branching = {
        ...inDetected,
        transitions: [
          { message_type: 'contained', to: 'assessed', guarded: true },
          { message_type: 'contained', to: 'contained' },
          { message_type: 'contained', to: 'assessed' },
          { message_type: 'dismiss', to: 'dismissed' },
        ],
      }
      expect(composerTransitions(breach, headWith(branching, ['contained', 'dismiss']))).toEqual([
        { messageType: 'contained', toLabels: ['Risk assessed', 'Contained'], mayMove: true },
        { messageType: 'dismiss', toLabels: ['Dismissed'] },
      ])
    })

    it('marks one target guarded only when every entry is, else may move', () => {
      const split = (guards: boolean[]) => ({
        ...inDetected,
        transitions: guards.map((g) => ({
          message_type: 'contained',
          to: 'contained',
          guarded: g,
        })),
      })
      const view = (guards: boolean[]) =>
        composerTransitions(breach, headWith(split(guards), ['contained']))
      expect(view([true, true])).toEqual([
        { messageType: 'contained', toLabels: ['Contained'], guarded: true },
      ])
      expect(view([false, true])).toEqual([
        { messageType: 'contained', toLabels: ['Contained'], mayMove: true },
      ])
      expect(view([false, false])).toEqual([{ messageType: 'contained', toLabels: ['Contained'] }])
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

describe('session decisions', () => {
  const at = '2026-10-11T09:00:00Z'
  const logType = (
    name: string,
    label: string,
    properties: ChannelMessageTypeSchema['properties'] = [],
    reply = true
  ): ChannelMessageTypeSchema => ({
    name,
    label,
    persist: 'log',
    reply,
    actions: [],
    properties,
  })
  const run: ChannelTypeSchema = {
    ...room,
    name: 'run',
    message_types: [
      logType('approve', 'Approve plan', [{ name: 'note', label: 'Note', type: 'string' }]),
      logType('revise', 'Revise plan', [
        { name: 'feedback', label: 'Feedback', type: 'markdown', required: true },
      ]),
      logType('confirm', 'Confirm', [
        { name: 'sure', label: 'Sure', type: 'boolean', required: true },
      ]),
      logType('accept', 'Accept'),
      logType('abandon', 'Abandon', [], false),
      logType('drop', 'Drop'),
      logType('split', 'Split'),
      logType('check', 'Check'),
      { ...logType('nudge', 'Nudge'), persist: 'ephemeral' },
    ],
    session: {
      label: 'Run',
      initial: 'plan-review',
      states: [
        { name: 'plan-review', label: 'Plan review', status: 'input_required' },
        { name: 'working', label: 'Working', status: 'working' },
        { name: 'accepted', label: 'Accepted', final: true, status: 'completed' },
        { name: 'abandoned', label: 'Abandoned', final: true, status: 'cancelled' },
        { name: 'failed', label: 'Failed', final: true, status: 'failed' },
      ],
    },
  }
  const inReview: NonNullable<Channel['session']> = {
    state: 'plan-review',
    status: 'input_required',
    generation: 4,
    entered_at: at,
    transitions: [
      { message_type: 'approve', to: 'working' },
      { message_type: 'revise', to: 'working' },
      { message_type: 'accept', to: 'accepted' },
      { message_type: 'abandon', to: 'abandoned' },
      { message_type: 'drop', to: 'abandoned' },
      { message_type: 'abandon', to: 'failed', guarded: true },
      { message_type: 'split', to: 'working' },
      { message_type: 'split', to: 'accepted', guarded: true },
      { message_type: 'check', to: 'working', guarded: true },
      { message_type: 'confirm', to: 'working' },
      { message_type: 'nudge', to: 'working' },
      { message_type: 'ghost', to: 'working' },
      { message_type: 'timer', to: 'working', by: 'trigger' },
    ],
  }
  const all = [
    'approve',
    'revise',
    'confirm',
    'accept',
    'abandon',
    'drop',
    'split',
    'check',
    'nudge',
    'ghost',
  ]
  function head(
    over: Partial<NonNullable<Channel['session']>> = {},
    transitions: string[] = ['approve', 'revise'],
    opts: { post?: string[]; state?: Channel['state'] } = {}
  ) {
    return {
      channel: {
        anchor: 'TKT-1',
        type: 'run',
        state: opts.state ?? ('open' as const),
        last_seq: 12,
        rev: 12,
        session: { ...inReview, ...over },
      },
      _actions: { post: opts.post ?? all, transitions },
    }
  }

  describe('decisionViews', () => {
    const approve = {
      key: 'approve',
      messageType: 'approve',
      label: 'Approve plan',
      toLabel: 'Working',
    }
    const revise = {
      ...approve,
      key: 'revise',
      messageType: 'revise',
      label: 'Revise plan',
      opensForm: true,
    }
    const cases: {
      name: string
      head: ReturnType<typeof head>
      gen?: number
      want: unknown[]
    }[] = [
      {
        name: 'the owner in plan review gets approve and revise',
        head: head(),
        want: [approve, revise],
      },
      { name: 'the runner gets none', head: head({}, []), want: [] },
      {
        name: 'a closed channel with stale transitions gets none',
        head: head({}, ['approve'], { state: 'closed' }),
        want: [],
      },
      { name: 'a final session gets none', head: head({ final: true }), want: [] },
      { name: 'a stranded session gets none', head: head({ stranded: true }), want: [] },
      { name: 'a state the descriptor lacks gets none', head: head({ state: 'gone' }), want: [] },
      { name: 'actions of another generation give none', head: head(), gen: 3, want: [] },
      { name: 'unknown actions generation gives none', head: head(), gen: -1, want: [] },
      {
        name: 'a type not postable is dropped',
        head: head({}, ['approve'], { post: ['revise'] }),
        want: [],
      },
      {
        name: 'a guarded edge opens the form',
        head: head({}, ['check']),
        want: [
          {
            key: 'check',
            messageType: 'check',
            label: 'Check',
            toLabel: 'Working',
            opensForm: true,
          },
        ],
      },
      {
        name: 'a required boolean opens the form',
        head: head({}, ['confirm']),
        want: [
          {
            key: 'confirm',
            messageType: 'confirm',
            label: 'Confirm',
            toLabel: 'Working',
            opensForm: true,
          },
        ],
      },
      { name: 'an ephemeral type is dropped', head: head({}, ['nudge']), want: [] },
      { name: 'a type the descriptor lacks is dropped', head: head({}, ['ghost']), want: [] },
      {
        name: 'two edges to different states give one form button without a target',
        head: head({}, ['split']),
        want: [{ key: 'split', messageType: 'split', label: 'Split', opensForm: true }],
      },
      {
        name: 'a move into a completed final state asks, without danger',
        head: head({}, ['accept']),
        want: [
          {
            key: 'accept',
            messageType: 'accept',
            label: 'Accept',
            toLabel: 'Accepted',
            confirm: true,
          },
        ],
      },
      {
        name: 'a form decision into final states does not ask',
        head: head({}, ['abandon']),
        want: [{ key: 'abandon', messageType: 'abandon', label: 'Abandon', opensForm: true }],
      },
      {
        name: 'a one-click move into a cancelled final state asks in the danger tone',
        head: head({}, ['drop']),
        want: [
          {
            key: 'drop',
            messageType: 'drop',
            label: 'Drop',
            toLabel: 'Abandoned',
            confirm: true,
            danger: true,
          },
        ],
      },
      {
        name: "the order is the server's and a trigger edge is ignored",
        head: head({}, ['revise', 'timer', 'approve']),
        want: [revise, approve],
      },
    ]
    for (const tc of cases) {
      it(tc.name, () => {
        const gen = tc.gen === -1 ? undefined : (tc.gen ?? 4)
        expect(decisionViews(run, tc.head, gen)).toEqual(tc.want)
      })
    }

    it('is empty for a channel type without a session', () => {
      expect(decisionViews(room, head(), 4)).toEqual([])
    })
  })

  describe('decisionViews presets', () => {
    const plan: ChannelTypeSchema = {
      ...run,
      message_types: [
        ...run.message_types,
        logType('decide', 'Decide', [
          { name: 'decision', label: 'Decision', type: 'string', required: true },
          { name: 'tags', label: 'Tags', type: 'string', list: true },
        ]),
        logType('stop', 'Stop'),
      ],
    }
    type Row = ChannelSessionTransition
    const approve: Row = {
      message_type: 'decide',
      to: 'working',
      guarded: true,
      entry: 0,
      button: { label: 'Approve', props: { decision: 'approve' } },
    }
    const revise: Row = {
      message_type: 'decide',
      to: 'plan-review',
      entry: 1,
      button: { label: 'Revise', props: { decision: 'revise' } },
    }
    const approveView = {
      key: 'preset:decide:0',
      messageType: 'decide',
      label: 'Approve',
      props: { decision: 'approve' },
      toLabel: 'Working',
    }
    const reviseView = {
      key: 'preset:decide:1',
      messageType: 'decide',
      label: 'Revise',
      props: { decision: 'revise' },
      toLabel: 'Plan review',
    }
    function ph(
      rows: Row[],
      entries: [string, number][] | undefined,
      transitions: string[] = ['decide']
    ) {
      const h = head({ transitions: rows }, transitions, { post: [...all, 'decide', 'stop'] })
      return {
        ...h,
        _actions: {
          ...h._actions,
          ...(entries && {
            entries: entries.map(([message_type, entry]) => ({ message_type, entry })),
          }),
        },
      }
    }
    const cases: { name: string; head: ReturnType<typeof ph>; gen?: number; want: unknown[] }[] = [
      {
        name: 'two presets and no type button',
        head: ph(
          [approve, revise],
          [
            ['decide', 0],
            ['decide', 1],
          ]
        ),
        want: [approveView, reviseView],
      },
      {
        name: 'one preset and a type button computed from the rest',
        head: ph(
          [
            { message_type: 'stop', to: 'abandoned', entry: 0, button: { label: 'Stop now' } },
            { message_type: 'stop', to: 'accepted', entry: 1 },
          ],
          [
            ['stop', 0],
            ['stop', 1],
          ],
          ['stop']
        ),
        want: [
          {
            key: 'preset:stop:0',
            messageType: 'stop',
            label: 'Stop now',
            props: {},
            toLabel: 'Abandoned',
            confirm: true,
            danger: true,
          },
          { key: 'stop', messageType: 'stop', label: 'Stop', toLabel: 'Accepted', confirm: true },
        ],
      },
      {
        name: 'a lead-only entry not listed for a member gives no type button target',
        head: ph(
          [{ message_type: 'decide', to: 'accepted', entry: 0 }, { ...revise }],
          [['decide', 1]]
        ),
        want: [reviseView],
      },
      {
        name: 'an earlier guarded preset entry makes the type button open the form',
        head: ph(
          [
            {
              message_type: 'stop',
              to: 'working',
              guarded: true,
              entry: 0,
              button: { label: 'Go' },
            },
            { message_type: 'stop', to: 'accepted', entry: 1 },
          ],
          [
            ['stop', 0],
            ['stop', 1],
          ],
          ['stop']
        ),
        want: [
          { key: 'preset:stop:0', messageType: 'stop', label: 'Go', props: {}, toLabel: 'Working' },
          { key: 'stop', messageType: 'stop', label: 'Stop', toLabel: 'Accepted', opensForm: true },
        ],
      },
      {
        name: 'trigger entries are ignored',
        head: ph(
          [
            { message_type: 'stop', to: 'accepted', by: 'trigger', guarded: true, entry: 0 },
            { message_type: 'stop', to: 'working', entry: 1 },
          ],
          [['stop', 1]],
          ['stop']
        ),
        want: [{ key: 'stop', messageType: 'stop', label: 'Stop', toLabel: 'Working' }],
      },
      {
        name: 'a preset not in _actions.entries is hidden',
        head: ph([approve, revise], []),
        want: [],
      },
      {
        name: 'a listed entry without a served row shows nothing',
        head: ph(
          [revise],
          [
            ['decide', 1],
            ['decide', 5],
          ]
        ),
        want: [reviseView],
      },
      {
        name: 'a preset to a final state asks',
        head: ph([{ ...revise, to: 'accepted' }], [['decide', 1]]),
        want: [{ ...reviseView, toLabel: 'Accepted', confirm: true }],
      },
      {
        name: 'a preset to a cancelled state is danger',
        head: ph([{ ...revise, to: 'abandoned' }], [['decide', 1]]),
        want: [{ ...reviseView, toLabel: 'Abandoned', confirm: true, danger: true }],
      },
      {
        name: 'the order follows the types, then the entries',
        head: ph(
          [
            revise,
            approve,
            { message_type: 'accept', to: 'accepted', entry: 0, button: { label: 'Done' } },
          ],
          [
            ['accept', 0],
            ['decide', 0],
            ['decide', 1],
          ],
          ['accept', 'decide']
        ),
        want: [
          {
            key: 'preset:accept:0',
            messageType: 'accept',
            label: 'Done',
            props: {},
            toLabel: 'Accepted',
            confirm: true,
          },
          approveView,
          reviseView,
        ],
      },
      {
        name: 'a stale actions generation shows none',
        head: ph(
          [approve, revise],
          [
            ['decide', 0],
            ['decide', 1],
          ]
        ),
        gen: 3,
        want: [],
      },
      {
        name: 'a list preset is carried as an array',
        head: ph(
          [
            {
              ...revise,
              button: { label: 'Tag', props: { decision: 'revise', tags: ['a', 'b'] } },
            },
          ],
          [['decide', 1]]
        ),
        want: [{ ...reviseView, label: 'Tag', props: { decision: 'revise', tags: ['a', 'b'] } }],
      },
      {
        name: 'a compose preset opens the form and keeps its props',
        head: ph(
          [
            {
              ...revise,
              button: { label: 'Revise', props: { decision: 'revise' }, compose: true },
            },
          ],
          [['decide', 1]]
        ),
        want: [{ ...reviseView, opensForm: true, compose: true }],
      },
      {
        name: 'a compose preset to a final state does not ask',
        head: ph(
          [
            {
              ...revise,
              to: 'abandoned',
              button: { label: 'Revise', props: { decision: 'revise' }, compose: true },
            },
          ],
          [['decide', 1]]
        ),
        want: [{ ...reviseView, toLabel: 'Abandoned', opensForm: true, compose: true }],
      },
      {
        name: 'a compose preset takes style but not confirm',
        head: ph(
          [
            {
              ...revise,
              button: {
                label: 'Revise',
                props: { decision: 'revise' },
                compose: true,
                confirm: true,
                style: 'danger',
              },
            },
          ],
          [['decide', 1]]
        ),
        want: [{ ...reviseView, opensForm: true, compose: true, danger: true }],
      },
      {
        name: 'a compose preset beside a one-click one',
        head: ph(
          [
            approve,
            {
              ...revise,
              button: { label: 'Revise', props: { decision: 'revise' }, compose: true },
            },
          ],
          [
            ['decide', 0],
            ['decide', 1],
          ]
        ),
        want: [approveView, { ...reviseView, opensForm: true, compose: true }],
      },
      {
        name: 'without _actions.entries the type button stands for every entry',
        head: ph([approve, revise], undefined),
        want: [{ key: 'decide', messageType: 'decide', label: 'Decide', opensForm: true }],
      },
    ]
    for (const tc of cases) {
      it(tc.name, () => {
        expect(decisionViews(plan, tc.head, tc.gen ?? 4)).toEqual(tc.want)
      })
    }
  })

  describe('decisionViews options', () => {
    type Options = NonNullable<ChannelMessageTypeSchema['decision_button']>
    /** run with decision_button on the named message types. */
    function withOptions(byType: Record<string, Options>): ChannelTypeSchema {
      return {
        ...run,
        message_types: run.message_types.map((mt) =>
          byType[mt.name] ? { ...mt, decision_button: byType[mt.name] } : mt
        ),
      }
    }
    /** The type button of one row of type into to, with the type's options. */
    function typeView(name: string, to: string, options?: Options) {
      const ct = options ? withOptions({ [name]: options }) : run
      return decisionViews(ct, head({ transitions: [{ message_type: name, to }] }, [name]), 4)
    }
    const accept = { key: 'accept', messageType: 'accept', label: 'Accept' }

    it('no options keep the derived rule', () => {
      expect(typeView('accept', 'accepted')).toEqual([
        { ...accept, toLabel: 'Accepted', confirm: true },
      ])
      expect(typeView('accept', 'abandoned')).toEqual([
        { ...accept, toLabel: 'Abandoned', confirm: true, danger: true },
      ])
      expect(typeView('accept', 'working')).toEqual([{ ...accept, toLabel: 'Working' }])
    })
    it('form button into a cancelled state stays plain without options', () => {
      expect(typeView('revise', 'abandoned')).toEqual([
        {
          key: 'revise',
          messageType: 'revise',
          label: 'Revise plan',
          toLabel: 'Abandoned',
          opensForm: true,
        },
      ])
    })
    it('confirm true asks into a non-final state', () => {
      expect(typeView('accept', 'working', { confirm: true })).toEqual([
        { ...accept, toLabel: 'Working', confirm: true, movesOnly: true },
      ])
    })
    it('confirm false does not ask into a final state', () => {
      expect(typeView('accept', 'accepted', { confirm: false })).toEqual([
        { ...accept, toLabel: 'Accepted' },
      ])
      expect(typeView('accept', 'abandoned', { confirm: false })).toEqual([
        { ...accept, toLabel: 'Abandoned', danger: true },
      ])
    })
    it('confirm text is carried', () => {
      expect(
        typeView('accept', 'accepted', { confirm: true, confirm_text: 'Runs the tool.' })
      ).toEqual([{ ...accept, toLabel: 'Accepted', confirm: true, confirmText: 'Runs the tool.' }])
    })
    it('style danger without a cancelled target', () => {
      expect(typeView('accept', 'working', { style: 'danger' })).toEqual([
        { ...accept, toLabel: 'Working', danger: true },
      ])
    })
    it('style default drops the derived danger', () => {
      expect(typeView('accept', 'abandoned', { style: 'default' })).toEqual([
        { ...accept, toLabel: 'Abandoned', confirm: true, outcomeDanger: true },
      ])
    })
    it('style primary', () => {
      expect(typeView('accept', 'working', { style: 'primary' })).toEqual([
        { ...accept, toLabel: 'Working', primary: true },
      ])
      expect(typeView('accept', 'abandoned', { style: 'primary' })).toEqual([
        { ...accept, toLabel: 'Abandoned', confirm: true, primary: true, outcomeDanger: true },
      ])
    })
    it("type button reads the message type's decision_button", () => {
      const ct = withOptions({ accept: { style: 'danger' }, drop: { style: 'primary' } })
      const h = head(
        {
          transitions: [
            { message_type: 'accept', to: 'working' },
            { message_type: 'drop', to: 'working' },
          ],
        },
        ['accept', 'drop']
      )
      expect(decisionViews(ct, h, 4)).toEqual([
        { ...accept, toLabel: 'Working', danger: true },
        { key: 'drop', messageType: 'drop', label: 'Drop', toLabel: 'Working', primary: true },
      ])
    })
    it('type button that opens the form keeps style and drops confirm', () => {
      expect(typeView('revise', 'abandoned', { confirm: true, style: 'danger' })).toEqual([
        {
          key: 'revise',
          messageType: 'revise',
          label: 'Revise plan',
          toLabel: 'Abandoned',
          opensForm: true,
          danger: true,
        },
      ])
    })

    /** A head whose accept rows are listed for the caller. */
    function listed(rows: ChannelSessionTransition[]) {
      const h = head({ transitions: rows }, ['accept'])
      return {
        ...h,
        _actions: {
          ...h._actions,
          entries: rows.map((r) => ({ message_type: r.message_type, entry: r.entry ?? 0 })),
        },
      }
    }
    it('preset options do not reach the type button', () => {
      const h = listed([
        {
          message_type: 'accept',
          to: 'working',
          entry: 0,
          button: { label: 'Go', style: 'primary', confirm: true },
        },
        { message_type: 'accept', to: 'working', entry: 1 },
      ])
      expect(decisionViews(run, h, 4)).toEqual([
        {
          key: 'preset:accept:0',
          messageType: 'accept',
          label: 'Go',
          props: {},
          toLabel: 'Working',
          confirm: true,
          movesOnly: true,
          primary: true,
        },
        { ...accept, toLabel: 'Working' },
      ])
    })
    it('a preset reads the merged row options', () => {
      // The type's decision_button is already merged into the row by the
      // server; the preset reads only the row.
      const ct = withOptions({ accept: { style: 'primary' } })
      const h = listed([
        {
          message_type: 'accept',
          to: 'working',
          entry: 0,
          button: { label: 'Go', confirm: true, confirm_text: 'Sure?', style: 'danger' },
        },
      ])
      expect(decisionViews(ct, h, 4)).toEqual([
        {
          key: 'preset:accept:0',
          messageType: 'accept',
          label: 'Go',
          props: {},
          toLabel: 'Working',
          confirm: true,
          confirmText: 'Sure?',
          movesOnly: true,
          danger: true,
        },
      ])
    })
  })

  const msgAt = (seq: number, over: Partial<Message> = {}): Message => ({
    seq,
    rev: seq,
    type: 'text',
    author: { user: 'ann' },
    at,
    props: { body: `m${seq}` },
    ...over,
  })
  const notice = (seq: number, generation: number, cause?: number): Message =>
    msgAt(seq, {
      type: 'session',
      author: { user: 'system', system: true },
      triggered_by: 'session:run',
      props: { from: 'working', to: 'plan-review', generation, ...(cause ? { cause } : {}) },
    })

  describe('decisionCause', () => {
    const cases: { name: string; messages: Message[]; gen: number | undefined; want?: number }[] = [
      { name: 'found', messages: [msgAt(10), msgAt(12), notice(13, 4, 12)], gen: 4, want: 12 },
      {
        name: 'only an older generation is held',
        messages: [msgAt(12), notice(13, 3, 12)],
        gen: 4,
      },
      { name: 'generation 0', messages: [msgAt(12), notice(13, 0, 12)], gen: 0 },
      { name: 'no session', messages: [msgAt(12), notice(13, 4, 12)], gen: undefined },
      { name: 'a timer notice has no cause', messages: [msgAt(12), notice(13, 4)], gen: 4 },
      { name: 'the cause is not held', messages: [msgAt(11), notice(13, 4, 12)], gen: 4 },
      {
        name: 'the cause is removed',
        messages: [
          msgAt(12, { removed: { kind: 'moderator', by: { user: 'bob' }, at } }),
          notice(13, 4, 12),
        ],
        gen: 4,
      },
      {
        name: 'the notice of the current generation wins over an older one',
        messages: [msgAt(8), notice(9, 3, 8), msgAt(12), notice(13, 4, 12)],
        gen: 4,
        want: 12,
      },
    ]
    for (const tc of cases) {
      it(tc.name, () => {
        const session = tc.gen === undefined ? undefined : { generation: tc.gen }
        expect(decisionCause(tc.messages, session)).toBe(tc.want)
      })
    }
  })

  describe('decisionReplyTo', () => {
    const msgs = [msgAt(12), msgAt(14, { reply_to: 12 })]
    const cases: { name: string; cause?: number; reply?: boolean; want?: number }[] = [
      { name: 'a top-level cause and a type that replies', cause: 12, reply: true, want: 12 },
      { name: 'the cause is itself a reply', cause: 14, reply: true },
      { name: 'the type may not reply', cause: 12, reply: false },
      { name: 'no cause', reply: true },
      { name: 'the cause is not held', cause: 13, reply: true },
    ]
    for (const tc of cases) {
      it(tc.name, () => {
        expect(decisionReplyTo(msgs, tc.cause, { reply: tc.reply ?? false })).toBe(tc.want)
      })
    }
  })
})

describe('choices', () => {
  const at = '2026-10-11T09:00:00Z'
  const ask: ChannelMessageTypeSchema = {
    name: 'ask',
    label: 'Ask',
    persist: 'log',
    reply: true,
    actions: [],
    properties: [
      { name: 'note', label: 'Note', type: 'string' },
      { name: 'options', label: 'Options', type: 'string', list: true, max: 80, max_items: 4 },
    ],
    choices: { from: 'options', answer: 'answer', into: 'body', other: true },
  }
  const answer: ChannelMessageTypeSchema = {
    name: 'answer',
    label: 'Answer',
    persist: 'log',
    reply: true,
    actions: [],
    properties: [{ name: 'body', label: 'Body', type: 'string' }],
  }
  const plain: ChannelTypeSchema = { ...room, name: 'plain', message_types: [ask, answer] }
  const withSession: ChannelTypeSchema = {
    ...plain,
    name: 'run',
    session: {
      label: 'Run',
      initial: 'working',
      states: [
        { name: 'working', label: 'Working', status: 'working' },
        { name: 'asking', label: 'Asking', status: 'input_required' },
      ],
    },
  }
  const q = (seq: number, over: Partial<Message> = {}): Message => ({
    seq,
    rev: seq,
    type: 'ask',
    author: { user: 'robot' },
    at,
    props: { note: 'Which?', options: ['A', 'B'] },
    ...over,
  })
  const reply = (seq: number, to: number, type = 'answer'): Message => ({
    seq,
    rev: seq,
    type,
    author: { user: 'ann' },
    at,
    props: { body: 'A' },
    reply_to: to,
  })
  const notice = (seq: number, generation: number, cause?: number): Message => ({
    seq,
    rev: seq,
    type: 'session',
    author: { user: 'system', system: true },
    at,
    triggered_by: 'session:run',
    props: { from: 'working', to: 'asking', generation, ...(cause ? { cause } : {}) },
  })
  const head = (
    over: { state?: Channel['state']; post?: string[]; generation?: number } = {}
  ): ChannelHead =>
    ({
      channel: {
        state: over.state ?? 'open',
        ...(over.generation !== undefined
          ? {
              session: {
                state: 'asking',
                status: 'input_required',
                generation: over.generation,
                entered_at: at,
                transitions: [],
              },
            }
          : {}),
      },
      _actions: { post: over.post ?? ['ask', 'answer'] },
    }) as unknown as ChannelHead
  const keys = (m: Map<number, { key: string }[]>) =>
    [...m].map(([seq, vs]) => [seq, vs.map((v) => v.key)])

  const cases: {
    name: string
    ct?: ChannelTypeSchema
    head?: ChannelHead
    actionsGeneration?: number
    hasNewer?: boolean
    messages: Message[]
    want: [number, string[]][]
  }[] = [
    {
      name: 'live',
      messages: [q(1)],
      want: [[1, ['choice:1:0', 'choice:1:1', 'choice:1:other']]],
    },
    {
      name: 'removed',
      messages: [q(1, { removed: { kind: 'author', at }, props: undefined })],
      want: [],
    },
    {
      name: 'a reply gets no buttons',
      messages: [q(1), q(2, { reply_to: 1 })],
      want: [[1, ['choice:1:0', 'choice:1:1', 'choice:1:other']]],
    },
    { name: 'no post right', head: head({ post: ['ask'] }), messages: [q(1)], want: [] },
    { name: 'answered', messages: [q(1), reply(2, 1)], want: [] },
    {
      name: 'a removed answer brings the buttons back',
      messages: [
        q(1),
        {
          ...reply(2, 1),
          props: undefined,
          removed: { kind: 'moderator', by: { user: 'bob' }, at },
        },
      ],
      want: [[1, ['choice:1:0', 'choice:1:1', 'choice:1:other']]],
    },
    {
      name: 'a reply of another type does not answer it',
      messages: [q(1), reply(2, 1, 'ask')],
      want: [[1, ['choice:1:0', 'choice:1:1', 'choice:1:other']]],
    },
    { name: 'hasNewer', hasNewer: true, messages: [q(1)], want: [] },
    { name: 'closed channel', head: head({ state: 'closed' }), messages: [q(1)], want: [] },
    { name: 'no options', messages: [q(1, { props: { note: 'x' } })], want: [] },
    { name: 'empty options', messages: [q(1, { props: { options: [] } })], want: [] },
    {
      name: 'options that are not a list',
      messages: [q(1, { props: { options: 'A' } })],
      want: [],
    },
    {
      name: 'other: false',
      ct: {
        ...plain,
        message_types: [{ ...ask, choices: { ...ask.choices!, other: false } }, answer],
      },
      messages: [q(1)],
      want: [[1, ['choice:1:0', 'choice:1:1']]],
    },
    {
      name: 'session: the question caused the current generation',
      ct: withSession,
      head: head({ generation: 5 }),
      actionsGeneration: 5,
      messages: [q(1), q(10), notice(11, 5, 10)],
      want: [[10, ['choice:10:0', 'choice:10:1', 'choice:10:other']]],
    },
    {
      name: 'session: a later notice moved it',
      ct: withSession,
      head: head({ generation: 6 }),
      actionsGeneration: 6,
      messages: [q(10), notice(11, 5, 10), reply(12, 1, 'text'), notice(13, 6, 12)],
      want: [],
    },
    {
      name: 'session: the head is ahead of the held notice',
      ct: withSession,
      head: head({ generation: 6 }),
      actionsGeneration: 6,
      messages: [q(10), notice(11, 5, 10)],
      want: [],
    },
    {
      name: 'session: the question did not move it',
      ct: withSession,
      head: head({ generation: 5 }),
      actionsGeneration: 5,
      messages: [q(9), q(10, { type: 'text' }), notice(11, 5, 10)],
      want: [],
    },
    {
      name: 'session: _actions are stale',
      ct: withSession,
      head: head({ generation: 5 }),
      actionsGeneration: 4,
      messages: [q(10), notice(11, 5, 10)],
      want: [],
    },
  ]
  for (const tc of cases) {
    it(tc.name, () => {
      const got = choiceViews(
        tc.ct ?? plain,
        tc.head ?? head(),
        tc.actionsGeneration,
        tc.hasNewer ?? false,
        tc.messages
      )
      expect(keys(got)).toEqual(tc.want)
    })
  }

  it('a view posts the answer with the option in into; other opens the form', () => {
    const [a, , other] = choiceViews(plain, head(), undefined, false, [q(1)]).get(1)!
    expect(a).toEqual({
      key: 'choice:1:0',
      messageType: 'answer',
      label: 'A',
      props: { body: 'A' },
    })
    expect(other).toEqual({
      key: 'choice:1:other',
      messageType: 'answer',
      label: 'Other answer…',
      opensForm: true,
    })
  })

  it('rowDecisions drops the decision of the answer type on the question row', () => {
    const choices = choiceViews(plain, head(), undefined, false, [q(1)])
    const decisions = [
      { key: 'answer', messageType: 'answer', label: 'Answer' },
      { key: 'go', messageType: 'go', label: 'Go' },
    ]
    expect(rowDecisions(decisions, choices, 1).map((d) => d.key)).toEqual(['go'])
    expect(rowDecisions(decisions, choices, 2)).toBe(decisions)
    expect(rowDecisions(decisions, choices, undefined)).toBe(decisions)
  })

  it('drops presets of the answer type', () => {
    const choices = choiceViews(plain, head(), undefined, false, [q(1)])
    const decisions = [
      { key: 'preset:answer:0', messageType: 'answer', label: 'Yes', props: { body: 'yes' } },
      { key: 'preset:go:1', messageType: 'go', label: 'Go now' },
    ]
    expect(rowDecisions(decisions, choices, 1).map((d) => d.key)).toEqual(['preset:go:1'])
    expect(rowDecisions(decisions, choices, undefined)).toBe(decisions)
  })

  it('an excerpt reads a list as its items', () => {
    expect(excerptOf(q(1, { props: { options: ['A', 'B'] } }), ask)).toBe('A, B')
  })

  it('a list prop is no ref, and a stored non-array value passes through as text', () => {
    const [v] = messageViews([q(1, { props: { options: 'A' } })], () => ask, ctx)
    expect(v!.props).toEqual({ options: 'A' })
    const [w] = messageViews([q(2)], () => ask, ctx)
    expect(w!.props?.options).toEqual(['A', 'B'])
    expect(w!.html).toBeUndefined()
  })
})
