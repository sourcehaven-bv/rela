import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { CHANNEL_WIRE_FIELDS } from './channels'

// The other half of TestChannelWireGolden (internal/dataentry): Go writes the
// JSON field set of each channel wire type, and this compares it with the
// hand-kept key lists next to the TypeScript mirror. A field added, removed
// or renamed on one side alone fails here.
const FIXTURE = resolve(__dirname, '../../../internal/dataentry/testdata/channel_wire_fields.json')
// Real answers of the handlers, written by the same Go test.
const SAMPLES = resolve(__dirname, '../../../internal/dataentry/testdata/channel_wire.json')

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

  it('head', () => {
    const head = samples.head!
    keysOf('ChannelHead', head)
    keysOf('Channel', head.channel)
    keysOf('ChannelActions', head._actions)
    keysOf('Message', head.last)
  })

  it('messages', () => {
    const page = samples.messages!
    keysOf('MessagePage', page)
    for (const m of page.messages as Json[]) keysOf('Message', m)
  })

  it('list', () => {
    const list = samples.list!
    keysOf('ChannelList', list)
    for (const t of list.types as Json[]) {
      keysOf('ChannelTypeList', t)
      for (const c of t.channels as Json[]) keysOf('Channel', c)
    }
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
