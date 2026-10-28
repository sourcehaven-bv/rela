import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { CHANNEL_WIRE_FIELDS, MESSAGE_OPS } from './channels'

// The other half of TestChannelWireGolden (internal/dataentry): Go writes the
// JSON field set of each channel wire type, and this compares it with the
// hand-kept key lists next to the TypeScript mirror. A field added, removed
// or renamed on one side alone fails here.
const FIXTURE = resolve(__dirname, '../../../internal/dataentry/testdata/channel_wire_fields.json')
// Real answers of the handlers, written by the same Go test.
const SAMPLES = resolve(__dirname, '../../../internal/dataentry/testdata/channel_wire.json')
// realtimecfg.MessageOpNames, written by the same Go test.
const MESSAGE_OPS_FIXTURE = resolve(
  __dirname,
  '../../../internal/dataentry/testdata/channel_message_ops.json'
)

describe('message operation names agree with the server', () => {
  it('MESSAGE_OPS is realtimecfg.MessageOpNames', () => {
    const goOps = JSON.parse(readFileSync(MESSAGE_OPS_FIXTURE, 'utf-8')) as string[]
    expect([...MESSAGE_OPS].sort()).toEqual([...goOps].sort())
  })
})

describe('channel wire types agree with the Go wire', () => {
  const goFields = JSON.parse(readFileSync(FIXTURE, 'utf-8')) as Record<string, string[]>

  it('mirrors every Go wire type and no other', () => {
    expect(Object.keys(CHANNEL_WIRE_FIELDS).sort()).toEqual(Object.keys(goFields).sort())
  })

  it.each(Object.keys(goFields).sort())('%s has the same JSON keys', (name) => {
    expect([...(CHANNEL_WIRE_FIELDS[name] ?? [])].sort()).toEqual([...goFields[name]!].sort())
  })
})

describe('real channel answers use only mirrored keys', () => {
  type Json = Record<string, unknown>
  const samples = JSON.parse(readFileSync(SAMPLES, 'utf-8')) as Record<string, Json>

  function keysOf(type: string, value: unknown) {
    expect(
      Object.keys(value as Json).filter((k) => !CHANNEL_WIRE_FIELDS[type]!.includes(k))
    ).toEqual([])
  }

  /** Checks m and every nested wire value it holds. */
  function messageKeys(m: Json) {
    keysOf('Message', m)
    if (m.author) keysOf('ChannelAuthor', m.author)
    if (m.removed) {
      const removed = m.removed as Json
      keysOf('MessageRemoval', removed)
      if (removed.by) keysOf('ChannelAuthor', removed.by)
    }
    if (m.action) {
      const action = m.action as Json
      keysOf('MessageAction', action)
      if (action.promoted) keysOf('RefProp', action.promoted)
    }
    if (m.outputs) {
      const outputs = m.outputs as Json
      keysOf('ChannelMessageOutputs', outputs)
      keysOf('ChannelAuthor', outputs.writer)
      for (const r of (outputs.results ?? []) as Json[]) {
        keysOf('ChannelMessageOutputResult', r)
        if (r.entity) keysOf('RefProp', r.entity)
      }
    }
    for (const mention of (m.mentions ?? []) as Json[]) {
      keysOf('MessageMention', mention)
      if (mention.ref) keysOf('RefProp', mention.ref)
    }
    for (const v of Object.values((m.props ?? {}) as Json)) {
      if (v !== null && typeof v === 'object') keysOf('RefProp', v)
    }
  }

  it('head', () => {
    const head = samples.head!
    keysOf('ChannelHead', head)
    keysOf('Channel', head.channel)
    keysOf('ChannelActions', head._actions)
    messageKeys(head.last as Json)
    const signals = head.ephemeral as Json[]
    expect(signals.length).toBeGreaterThan(0)
    for (const s of signals) {
      keysOf('EphemeralMessage', s)
      keysOf('ChannelAuthor', s.author)
    }
  })

  it('messages', () => {
    const page = samples.messages!
    keysOf('MessagePage', page)
    const messages = page.messages as Json[]
    for (const m of messages) messageKeys(m)
    // The samples hold the shapes most likely to carry an unmirrored field.
    expect(messages.some((m) => m.removed)).toBe(true)
    expect(messages.some((m) => (m.action as Json | undefined)?.status === 'pending')).toBe(true)
    expect(messages.some((m) => (m.action as Json | undefined)?.status === 'done')).toBe(true)
    expect(messages.some((m) => m.edit_until && m.release_after === undefined)).toBe(true)
    expect(messages.some((m) => m.release_after)).toBe(true)
    expect(
      messages.some((m) =>
        Object.values((m.props ?? {}) as Json).some((v) => (v as Json | null)?.restricted === true)
      )
    ).toBe(true)
  })

  it('list', () => {
    const list = samples.list!
    keysOf('ChannelList', list)
    for (const t of list.types as Json[]) {
      keysOf('ChannelTypeList', t)
      for (const c of t.channels as Json[]) keysOf('Channel', c)
    }
  })

  function channelKeys(c: Json) {
    keysOf('Channel', c)
    if (!c.session) return
    const session = c.session as Json
    keysOf('ChannelSession', session)
    for (const t of (session.transitions ?? []) as Json[]) keysOf('ChannelSessionTransition', t)
    for (const w of (session.waiting_for ?? []) as Json[]) keysOf('ChannelSessionWaiting', w)
  }

  it('session', () => {
    const head = samples.session_head!
    keysOf('ChannelHead', head)
    channelKeys(head.channel as Json)
    expect((head.channel as Json).session).toBeDefined()
    const page = samples.session_messages!
    keysOf('MessagePage', page)
    channelKeys(page.channel as Json)
    const messages = page.messages as Json[]
    for (const m of messages) messageKeys(m)
    // A session notice: type session, posted by the server.
    expect(messages.some((m) => m.type === 'session' && (m.author as Json).system === true)).toBe(
      true
    )
    for (const t of samples.list!.types as Json[]) {
      for (const c of t.channels as Json[]) channelKeys(c)
    }
  })

  it('session outputs', () => {
    const head = samples.outputs_head!
    keysOf('ChannelHead', head)
    channelKeys(head.channel as Json)
    const transitions = ((head.channel as Json).session as Json).transitions as Json[]
    expect(transitions.some((t) => typeof t.outputs === 'number')).toBe(true)
    const statuses: unknown[] = []
    for (const name of ['outputs_messages', 'failed_outputs_messages']) {
      const page = samples[name]!
      keysOf('MessagePage', page)
      for (const m of page.messages as Json[]) {
        messageKeys(m)
        if (m.outputs) statuses.push((m.outputs as Json).status)
      }
    }
    expect(statuses.sort()).toEqual(['done', 'failed'])
  })

  it('schema', () => {
    const schema = samples.schema!
    keysOf('ChannelSchema', schema)
    for (const ct of schema.channel_types as Json[]) {
      keysOf('ChannelTypeSchema', ct)
      for (const mt of ct.message_types as Json[]) {
        keysOf('ChannelMessageTypeSchema', mt)
        for (const p of mt.properties as Json[]) keysOf('ChannelPropertySchema', p)
        for (const a of mt.actions as Json[]) keysOf('ChannelActionSchema', a)
      }
    }
  })
})
