import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { QueryCache } from '@pinia/colada'
import {
  REFETCH_JITTER_MAX_MS,
  cancelWhenVisible,
  installVisibleRefetch,
  invalidateWhenVisible,
  resetVisibleRefetch,
  runWhenVisible,
} from './visibleRefetch'

// TASK-550RD: a hidden tab must not refetch, and a tab that becomes visible
// refetches after a random delay so tabs waking together spread out.

let visibility: DocumentVisibilityState = 'visible'

function setVisibility(v: DocumentVisibilityState) {
  visibility = v
  document.dispatchEvent(new Event('visibilitychange'))
}

interface FakeEntry {
  active: boolean
  options: { enabled: boolean | (() => boolean) } | null
}

function fakeCache(entries: FakeEntry[]) {
  const refresh = vi.fn().mockResolvedValue(undefined)
  const invalidateQueries = vi.fn().mockResolvedValue(undefined)
  const cache = {
    getEntries: () => entries,
    refresh,
    invalidateQueries,
  } as unknown as QueryCache
  return { cache, refresh, invalidateQueries }
}

describe('visibleRefetch', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    visibility = 'visible'
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
  })

  afterEach(() => {
    resetVisibleRefetch()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('refetches active queries now when visible, marks them stale when hidden', () => {
    const { cache, invalidateQueries } = fakeCache([])
    void invalidateWhenVisible(cache, { key: ['entities'] })
    expect(invalidateQueries).toHaveBeenLastCalledWith({ key: ['entities'] }, true)

    visibility = 'hidden'
    void invalidateWhenVisible(cache, { key: ['entities'] })
    expect(invalidateQueries).toHaveBeenLastCalledWith({ key: ['entities'] }, false)
  })

  it('refreshes active, enabled queries after a jittered delay once visible', () => {
    const active = { active: true, options: { enabled: true } }
    const inactive = { active: false, options: { enabled: true } }
    const disabled = { active: true, options: { enabled: () => false } }
    const { cache, refresh } = fakeCache([active, inactive, disabled])
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    installVisibleRefetch(cache)

    setVisibility('hidden')
    vi.advanceTimersByTime(REFETCH_JITTER_MAX_MS)
    expect(refresh).not.toHaveBeenCalled()

    setVisibility('visible')
    vi.advanceTimersByTime(REFETCH_JITTER_MAX_MS / 2 - 1)
    expect(refresh).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(refresh).toHaveBeenCalledWith(active)
  })

  it('does not refetch on reconnect while hidden', () => {
    const { cache, refresh } = fakeCache([{ active: true, options: { enabled: true } }])
    installVisibleRefetch(cache)
    visibility = 'hidden'
    window.dispatchEvent(new Event('online'))
    vi.advanceTimersByTime(REFETCH_JITTER_MAX_MS)
    expect(refresh).not.toHaveBeenCalled()

    visibility = 'visible'
    window.dispatchEvent(new Event('online'))
    vi.advanceTimersByTime(REFETCH_JITTER_MAX_MS)
    expect(refresh).toHaveBeenCalledTimes(1)
  })

  it('defers keyed work while hidden and runs the latest once when shown', () => {
    const { cache } = fakeCache([])
    installVisibleRefetch(cache)
    const first = vi.fn()
    const second = vi.fn()

    visibility = 'hidden'
    runWhenVisible('k', first)
    runWhenVisible('k', second)
    expect(first).not.toHaveBeenCalled()

    setVisibility('visible')
    vi.advanceTimersByTime(REFETCH_JITTER_MAX_MS)
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)

    const now = vi.fn()
    runWhenVisible('k', now)
    expect(now).toHaveBeenCalledTimes(1)
  })

  it('drops work cancelled before the tab is shown', () => {
    const { cache } = fakeCache([])
    installVisibleRefetch(cache)
    const kept = vi.fn()
    const dropped = vi.fn()
    setVisibility('hidden')
    runWhenVisible('kept', kept)
    runWhenVisible('dropped', dropped)
    cancelWhenVisible('dropped')

    setVisibility('visible')
    vi.advanceTimersByTime(REFETCH_JITTER_MAX_MS)
    expect(kept).toHaveBeenCalledTimes(1)
    expect(dropped).not.toHaveBeenCalled()
  })
})
