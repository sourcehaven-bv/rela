import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { ref } from 'vue'

const listVersions = vi.fn()
vi.mock('@/api/history', () => ({ listVersions: (...a: unknown[]) => listVersions(...a) }))

import { useHasHistory } from './useHasHistory'

const timeline = (n: number) => ({
  versions: Array.from({ length: n }, (_, i) => ({ version: i + 1 })),
  face: '',
  worldFaceAbsent: false,
})

describe('useHasHistory', () => {
  beforeEach(() => {
    listVersions.mockReset()
  })

  function setup(over: { enabled?: boolean; address?: string | null } = {}) {
    const enabled = ref(over.enabled ?? true)
    const address = ref<string | null>(over.address === undefined ? 'ACC-1' : over.address)
    const world = ref<string | undefined>(undefined)
    const has = useHasHistory({
      enabled: () => enabled.value,
      entityType: () => 'account',
      address: () => address.value,
      world: () => world.value,
    })
    return { has, enabled, address, world }
  }

  it('is true when the face has a version', async () => {
    listVersions.mockResolvedValue(timeline(2))
    const { has } = setup()
    await flushPromises()
    expect(has.value).toBe(true)
    expect(listVersions).toHaveBeenCalledWith('account', 'ACC-1', undefined)
  })

  it('is false for an empty timeline', async () => {
    listVersions.mockResolvedValue(timeline(0))
    const { has } = setup()
    await flushPromises()
    expect(has.value).toBe(false)
  })

  it('is false on a failure', async () => {
    listVersions.mockImplementation(() => Promise.reject(new Error('501')))
    const { has } = setup()
    await flushPromises()
    expect(has.value).toBe(false)
  })

  it('reads nothing where History is not offered or the entry has not loaded', async () => {
    setup({ enabled: false })
    setup({ address: null })
    await flushPromises()
    expect(listVersions).not.toHaveBeenCalled()
  })

  it('drops an answer for an address no longer on screen', async () => {
    let resolveFirst: (v: unknown) => void = () => {}
    listVersions
      .mockReturnValueOnce(new Promise((r) => (resolveFirst = r)))
      .mockResolvedValueOnce(timeline(0))
    const { has, address } = setup()
    address.value = 'ACC-2'
    await flushPromises()
    resolveFirst(timeline(3))
    await flushPromises()
    expect(has.value).toBe(false)
  })
})
