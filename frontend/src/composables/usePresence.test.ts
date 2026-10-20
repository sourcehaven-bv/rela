import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { ApiError } from '@/api/errors'
import { registerEntityPlurals } from '@/api/entities'
import type { PresenceResponse } from '@/types'
import { usePresence, type PresenceTarget } from './usePresence'

// The live subscription on the presence path is what makes this user a
// viewer; `live:changed` on it re-reads the viewers. A refused re-read clears
// them, because the list says who reads an entity this user may no longer
// see. Any other failure keeps it (TKT-IMBF7U).

const getPresenceMock = vi.fn<(type: string, address: string) => Promise<PresenceResponse>>()
vi.mock('@/api/presence', () => ({
  getPresence: (...a: [string, string]) => getPresenceMock(...a),
}))

const subs: { label: string; path: () => string | null; onChange: () => void }[] = []
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: () => string | null, onChange: () => void) => {
    subs.push({ label, path, onChange })
  },
}))

const bob = { user: 'bob@example.com', tool: 'data-entry', since: '2026-10-12T10:00:00Z' }
const carol = { user: 'carol@example.com', tool: 'mcp', since: '2026-10-12T10:01:00Z' }

function host(target: () => PresenceTarget | null) {
  let api!: ReturnType<typeof usePresence>
  const w = mount(
    defineComponent({
      setup() {
        api = usePresence(target)
        return () => h('div')
      },
    })
  )
  return { w, api }
}

function httpError(status: number) {
  return new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })
}

describe('usePresence', () => {
  beforeEach(() => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    getPresenceMock.mockReset()
    subs.length = 0
  })

  it('fetches on mount and subscribes the presence path', async () => {
    getPresenceMock.mockResolvedValue({ viewers: [bob], more: 2 })
    const { api } = host(() => ({ type: 'ticket', address: 'TKT-1@draft' }))
    await flushPromises()

    expect(getPresenceMock).toHaveBeenCalledWith('ticket', 'TKT-1@draft')
    expect(api.viewers.value).toEqual([bob])
    expect(api.more.value).toBe(2)
    expect(subs[0]!.label).toBe('entity-presence')
    expect(subs[0]!.path()).toBe('/api/v1/tickets/TKT-1@draft/_presence')
  })

  it('subscribes nothing and fetches nothing without a target', async () => {
    host(() => null)
    await flushPromises()
    expect(getPresenceMock).not.toHaveBeenCalled()
    expect(subs[0]!.path()).toBeNull()
  })

  it('refetches on live:changed', async () => {
    getPresenceMock.mockResolvedValue({ viewers: [bob], more: 0 })
    const { api } = host(() => ({ type: 'ticket', address: 'TKT-1' }))
    await flushPromises()

    getPresenceMock.mockResolvedValue({ viewers: [bob, carol], more: 0 })
    subs[0]!.onChange()
    await flushPromises()
    expect(api.viewers.value).toEqual([bob, carol])
  })

  it.each([401, 403, 404])(
    'clears the viewers when the refetch is refused with %i',
    async (status) => {
      getPresenceMock.mockResolvedValue({ viewers: [bob], more: 4 })
      const { api } = host(() => ({ type: 'ticket', address: 'TKT-1' }))
      await flushPromises()

      getPresenceMock.mockRejectedValue(httpError(status))
      subs[0]!.onChange()
      await flushPromises()
      expect(api.viewers.value).toEqual([])
      expect(api.more.value).toBe(0)
    }
  )

  it('keeps the viewers on a transient failure', async () => {
    getPresenceMock.mockResolvedValue({ viewers: [bob], more: 1 })
    const { api } = host(() => ({ type: 'ticket', address: 'TKT-1' }))
    await flushPromises()

    getPresenceMock.mockRejectedValue(httpError(503))
    subs[0]!.onChange()
    await flushPromises()
    expect(api.viewers.value).toEqual([bob])
    expect(api.more.value).toBe(1)
  })

  it('clears the old face at once and ignores its late answer', async () => {
    const address = ref('TKT-1')
    let resolveOld!: (r: PresenceResponse) => void
    getPresenceMock.mockResolvedValueOnce({ viewers: [bob], more: 0 })
    const { api } = host(() => ({ type: 'ticket', address: address.value }))
    await flushPromises()
    expect(api.viewers.value).toEqual([bob])

    getPresenceMock.mockReturnValueOnce(new Promise((r) => (resolveOld = r)))
    subs[0]!.onChange()
    getPresenceMock.mockResolvedValueOnce({ viewers: [carol], more: 0 })
    address.value = 'TKT-2'
    await flushPromises()
    expect(api.viewers.value).toEqual([carol])

    resolveOld({ viewers: [bob], more: 0 })
    await flushPromises()
    expect(api.viewers.value).toEqual([carol])
  })
})
