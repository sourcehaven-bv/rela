import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import type { ChannelSchema, ChannelTypeSchema, ChannelHead, Message } from '@/types/channels'
import { useChannelSchema, useChannelSchemaStore } from './channelSchema'

// The descriptor loads lazily, apart from the schema store's boot load, and
// reloads on `refresh` (a realtime.yaml reload sends one). A failed load
// gives an empty list, which hides the channel surfaces (TKT-91BK6F R5).

const getSchema = vi.fn<() => Promise<ChannelSchema>>()
vi.mock('@/api/channels', () => ({ getChannelSchema: () => getSchema() }))

const handlers = new Map<string, () => void>()
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({ on: (type: string, fn: () => void) => handlers.set(type, fn) }),
}))

const room: ChannelTypeSchema = {
  name: 'room',
  label: 'Room',
  attach_to: ['ticket'],
  anchor: 'entity',
  cardinality: 'one',
  remove_reason_required: false,
  lifecycle: [],
  message_types: [
    {
      name: 'text',
      label: 'Text',
      persist: 'log',
      reply: true,
      properties: [],
      actions: [{ name: 'ack', label: 'Ack' }],
    },
  ],
}

function head(post: string[]): ChannelHead {
  return {
    channel: { anchor: 'TKT-1', type: 'room', state: 'open', last_seq: 0, rev: 0 },
    ephemeral: [],
    _actions: { post, edit_own: false, delete_own: false, remove: false, lifecycle: [] },
  }
}

describe('channel schema store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getSchema.mockReset()
    handlers.clear()
  })

  it('loads once for concurrent callers and filters by entity type', async () => {
    getSchema.mockResolvedValue({ channel_types: [room] })
    const store = useChannelSchemaStore()
    await Promise.all([store.ensureLoaded(), store.ensureLoaded()])
    await store.ensureLoaded()
    expect(getSchema).toHaveBeenCalledTimes(1)
    expect(store.channelTypesFor('ticket')).toEqual([room])
    expect(store.channelTypesFor('feature')).toEqual([])
    expect(store.messageType('room', 'text')?.label).toBe('Text')
  })

  it('gives an empty list when the load fails', async () => {
    getSchema.mockRejectedValue(new Error('down'))
    const store = useChannelSchemaStore()
    await store.ensureLoaded()
    expect(store.loaded).toBe(true)
    expect(store.channelTypes).toEqual([])
  })

  it('reloads on refresh', async () => {
    getSchema.mockResolvedValueOnce({ channel_types: [] })
    let store!: ReturnType<typeof useChannelSchema>
    mount(
      defineComponent({
        setup() {
          store = useChannelSchema()
          return () => h('div')
        },
      })
    )
    await flushPromises()
    expect(store.channelTypes).toEqual([])

    getSchema.mockResolvedValueOnce({ channel_types: [room] })
    handlers.get('refresh')!()
    await flushPromises()
    expect(store.channelTypes).toEqual([room])
  })

  it('lets only the latest load land', async () => {
    let resolveOld!: (s: ChannelSchema) => void
    getSchema.mockReturnValueOnce(new Promise((r) => (resolveOld = r)))
    const store = useChannelSchemaStore()
    void store.ensureLoaded()
    getSchema.mockResolvedValueOnce({ channel_types: [room] })
    await store.reload()
    resolveOld({ channel_types: [] })
    await flushPromises()
    expect(store.channelTypes).toEqual([room])
  })

  it('reloads once when a head or message names an unknown type or action', async () => {
    getSchema.mockResolvedValue({ channel_types: [room] })
    const store = useChannelSchemaStore()
    await store.ensureLoaded()

    const known: Message = { seq: 1, rev: 1, type: 'text', _actions: ['ack', 'edit', 'delete'] }
    store.checkNames('room', head(['text']), [known])
    expect(getSchema).toHaveBeenCalledTimes(1)

    store.checkNames('room', head(['text', 'poll']), [known])
    await flushPromises()
    expect(getSchema).toHaveBeenCalledTimes(2)
    store.checkNames('room', head(['text', 'poll']), [known])
    expect(getSchema).toHaveBeenCalledTimes(2)

    store.checkNames('room', null, [{ ...known, _actions: ['promote'] }])
    await flushPromises()
    expect(getSchema).toHaveBeenCalledTimes(3)
  })
})
