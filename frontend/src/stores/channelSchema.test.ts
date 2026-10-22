import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import type { ChannelSchema, ChannelTypeSchema, ChannelHead, Message } from '@/types/channels'
import { RETRY_BASE_MS, useChannelSchema, useChannelSchemaStore } from './channelSchema'
import { useSchemaStore } from './schema'

// The descriptor loads lazily, apart from the schema store's boot load, and
// reloads on `refresh` (a realtime.yaml reload sends one). A failed load
// gives an empty list, which hides the channel surfaces (TKT-91BK6F R5).

const getSchema = vi.fn<() => Promise<ChannelSchema>>()
vi.mock('@/api/channels', () => ({ getChannelSchema: () => getSchema() }))

const getConfig = vi.fn()
vi.mock('@/api/schema', () => ({ getConfig: () => getConfig(), getSchema: vi.fn() }))

const handlers = new Map<string, () => void>()
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({ on: (type: string, fn: () => void) => handlers.set(type, fn) }),
}))

const room: ChannelTypeSchema = {
  name: 'room',
  label: 'Room',
  attach_to: ['ticket'],
  anchor: 'family',
  cardinality: 'one',
  remove_reason_required: false,
  typing: false,
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
    typing_now: [],
    _actions: {
      post,
      edit_own: false,
      delete_own: false,
      remove: false,
      lifecycle: [],
      typing: false,
    },
  }
}

describe('channel schema store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getSchema.mockReset()
    getConfig.mockReset()
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

  it('gives an empty list when the first load fails, and retries after a delay', async () => {
    vi.useFakeTimers()
    try {
      getSchema.mockRejectedValueOnce(new Error('down'))
      const store = useChannelSchemaStore()
      await store.ensureLoaded()
      expect(store.loaded).toBe(false)
      expect(store.channelTypes).toEqual([])

      getSchema.mockResolvedValue({ channel_types: [room] })
      await store.ensureLoaded()
      expect(getSchema).toHaveBeenCalledTimes(1)

      vi.advanceTimersByTime(RETRY_BASE_MS)
      await store.ensureLoaded()
      expect(getSchema).toHaveBeenCalledTimes(2)
      expect(store.loaded).toBe(true)
      expect(store.channelTypes).toEqual([room])
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps the held descriptor when a reload fails', async () => {
    getSchema.mockResolvedValueOnce({ channel_types: [room] })
    const store = useChannelSchemaStore()
    await store.ensureLoaded()
    getSchema.mockRejectedValueOnce(new Error('502'))
    await store.reload()
    expect(store.loaded).toBe(true)
    expect(store.channelTypes).toEqual([room])
  })

  it('reloads the descriptor and the presentation on refresh', async () => {
    getSchema.mockResolvedValueOnce({ channel_types: [] })
    getConfig.mockResolvedValue({
      channels: { room: { messages: { text: { fields: [{ property: 'body', label: 'Say' }] } } } },
    })
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
    expect(
      useSchemaStore().channelPresentations.get('room')?.messages?.text?.fields?.[0]?.label
    ).toBe('Say')

    // A failed /_config read keeps the presentation held.
    getSchema.mockResolvedValueOnce({ channel_types: [room] })
    getConfig.mockRejectedValueOnce(new Error('502'))
    handlers.get('refresh')!()
    await flushPromises()
    expect(useSchemaStore().channelPresentations.has('room')).toBe(true)
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
