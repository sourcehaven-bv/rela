import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { ApiError } from '@/api/errors'
import { registerEntityPlurals } from '@/api/entities'
import type { ChannelAnchor } from '@/api/channels'
import type { ChannelList, ChannelState } from '@/types/channels'
import { useEntityChannels, type UseEntityChannelsOptions } from './useEntityChannels'

// The entity's channel list is held across live refetches. A refused
// refetch clears it, because it lists channels on an entity this user may
// no longer read; any other failure keeps it (TKT-91BK6F).

const listMock = vi.fn<(a: ChannelAnchor, state?: ChannelState) => Promise<ChannelList>>()
vi.mock('@/api/channels', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/channels')>()),
  listChannels: (a: ChannelAnchor, state?: ChannelState) => listMock(a, state),
}))

const subs: { label: string; path: () => string | null; onChange: () => void }[] = []
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: () => string | null, onChange: () => void) => {
    subs.push({ label, path, onChange })
  },
}))

const roomType = {
  type: 'room',
  cardinality: 'one' as const,
  create: false,
  channels: [{ anchor: 'TKT-1', type: 'room', state: 'open' as const, last_seq: 1, rev: 1 }],
}
const list: ChannelList = { anchor: 'TKT-1', types: [roomType] }

function host(anchor: () => ChannelAnchor | null, options?: UseEntityChannelsOptions) {
  let api!: ReturnType<typeof useEntityChannels>
  mount(
    defineComponent({
      setup() {
        api = useEntityChannels(anchor, options)
        return () => h('div')
      },
    })
  )
  return api
}

function httpError(status: number) {
  return new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })
}

describe('useEntityChannels', () => {
  beforeEach(() => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    listMock.mockReset()
    subs.length = 0
  })

  it('drops an answer that arrives after the component is gone', async () => {
    let resolve!: (l: ChannelList) => void
    listMock.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    let api!: ReturnType<typeof useEntityChannels>
    const w = mount(
      defineComponent({
        setup() {
          api = useEntityChannels(() => ({ type: 'ticket', address: 'TKT-1' }))
          return () => h('div')
        },
      })
    )
    w.unmount()
    resolve(list)
    await flushPromises()
    expect(api.types.value).toEqual([])
  })

  it('lists and subscribes the channel list path', async () => {
    listMock.mockResolvedValue(list)
    const ch = host(() => ({ type: 'ticket', address: 'TKT-1@draft', world: 'main' }), {
      state: 'open',
    })
    await flushPromises()
    expect(listMock).toHaveBeenCalledWith(
      { type: 'ticket', address: 'TKT-1@draft', world: 'main' },
      'open'
    )
    expect(ch.types.value).toEqual([roomType])
    expect(subs[0]!.label).toBe('entity-channels')
    expect(subs[0]!.path()).toBe('/api/v1/tickets/TKT-1@draft/_channels?state=open&world=main')
  })

  it.each([401, 403, 404])(
    'clears the list when the refetch is refused with %i',
    async (status) => {
      listMock.mockResolvedValue(list)
      const ch = host(() => ({ type: 'ticket', address: 'TKT-1' }))
      await flushPromises()

      listMock.mockRejectedValue(httpError(status))
      subs[0]!.onChange()
      await flushPromises()
      expect(ch.types.value).toEqual([])
      expect(ch.error.value).toBeInstanceOf(ApiError)
    }
  )

  it('keeps the list on a transient failure', async () => {
    listMock.mockResolvedValue(list)
    const ch = host(() => ({ type: 'ticket', address: 'TKT-1' }))
    await flushPromises()

    listMock.mockRejectedValue(httpError(503))
    subs[0]!.onChange()
    await flushPromises()
    expect(ch.types.value).toEqual([roomType])
  })

  it('drops the subscription while not live and refetches when live again', async () => {
    listMock.mockResolvedValue(list)
    const live = ref(true)
    host(() => ({ type: 'ticket', address: 'TKT-1' }), { live })
    await flushPromises()

    live.value = false
    await flushPromises()
    expect(subs[0]!.path()).toBeNull()
    expect(listMock).toHaveBeenCalledTimes(1)

    live.value = true
    await flushPromises()
    expect(subs[0]!.path()).toBe('/api/v1/tickets/TKT-1/_channels')
    expect(listMock).toHaveBeenCalledTimes(2)
  })

  it('ignores the late answer of a previous entity', async () => {
    const address = ref('TKT-1')
    let resolveOld!: (l: ChannelList) => void
    listMock.mockReturnValueOnce(new Promise((r) => (resolveOld = r)))
    const ch = host(() => ({ type: 'ticket', address: address.value }))
    await flushPromises()

    listMock.mockResolvedValueOnce({ anchor: 'TKT-2', types: [] })
    address.value = 'TKT-2'
    await flushPromises()
    resolveOld(list)
    await flushPromises()
    expect(ch.types.value).toEqual([])
  })
})
