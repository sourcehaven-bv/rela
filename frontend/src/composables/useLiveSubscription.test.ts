import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ApiError } from '@/api/errors'

const putLiveSubscriptions =
  vi.fn<(conn: string, subs: { key: string; path: string }[]) => Promise<void>>()
vi.mock('@/api/live', () => ({
  putLiveSubscriptions: (...a: Parameters<typeof putLiveSubscriptions>) =>
    putLiveSubscriptions(...a),
}))

import {
  registerLive,
  setLiveConnection,
  clearLiveConnection,
  handleLiveChanged,
  setLiveReconnect,
  resetLiveSubscriptions,
  MAX_LIVE_SUBSCRIPTIONS,
} from './useLiveSubscription'
import type { QueryCache } from '@pinia/colada'
import {
  installVisibleRefetch,
  resetVisibleRefetch,
  REFETCH_JITTER_MAX_MS,
} from './visibleRefetch'

/** A query cache with no entries: these tests check only deferred work. */
function useQueryCacheStub(): QueryCache {
  return { getEntries: () => [] } as unknown as QueryCache
}

async function settle() {
  await vi.runAllTimersAsync()
}

function lastCall() {
  const calls = putLiveSubscriptions.mock.calls
  return calls[calls.length - 1]!
}

function sentKeys(): string[] {
  return lastCall()[1].map((s) => s.key)
}

describe('live subscription manager', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.useFakeTimers()
    resetVisibleRefetch()
    resetLiveSubscriptions()
    putLiveSubscriptions.mockReset()
    putLiveSubscriptions.mockResolvedValue()
  })

  it('waits for a connection id before sending', async () => {
    registerLive('a', '/api/v1/tickets', () => {})
    await settle()
    expect(putLiveSubscriptions).not.toHaveBeenCalled()

    setLiveConnection('c1')
    await settle()
    expect(putLiveSubscriptions).toHaveBeenCalledWith('c1', [{ key: 'a', path: '/api/v1/tickets' }])
  })

  it('sends registrations made together in one PUT', async () => {
    setLiveConnection('c1')
    registerLive('a', '/api/v1/tickets', () => {})
    registerLive('b', '/api/v1/features', () => {})
    await settle()
    expect(putLiveSubscriptions).toHaveBeenCalledTimes(1)
    expect(sentKeys()).toEqual(['a', 'b'])
  })

  it('calls onChange for added keys only after a 204', async () => {
    setLiveConnection('c1')
    const a = vi.fn()
    const b = vi.fn()
    registerLive('a', '/api/v1/tickets', a)
    await settle()
    expect(a).toHaveBeenCalledTimes(1)

    registerLive('b', '/api/v1/features', b)
    await settle()
    expect(a).toHaveBeenCalledTimes(1) // already accepted: not called again
    expect(b).toHaveBeenCalledTimes(1)
  })

  it('does not call onChange when the PUT fails', async () => {
    putLiveSubscriptions.mockRejectedValue(
      new ApiError('bad', { kind: 'http', status: 400, original: null })
    )
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    setLiveConnection('c1')
    const a = vi.fn()
    registerLive('a', '/api/v1/tickets', a)
    await settle()
    expect(a).not.toHaveBeenCalled()
  })

  it('routes live:changed to the key', async () => {
    const a = vi.fn()
    const b = vi.fn()
    registerLive('a', '/api/v1/tickets', a)
    registerLive('b', '/api/v1/features', b)
    handleLiveChanged('b')
    handleLiveChanged('unknown')
    expect(a).not.toHaveBeenCalled()
    expect(b).toHaveBeenCalledTimes(1)
  })

  it('unregistering sends the smaller set', async () => {
    setLiveConnection('c1')
    const off = registerLive('a', '/api/v1/tickets', () => {})
    registerLive('b', '/api/v1/features', () => {})
    await settle()
    off()
    await settle()
    expect(sentKeys()).toEqual(['b'])
  })

  it('an old unregister does not remove a newer registration of the key', async () => {
    const first = vi.fn()
    const second = vi.fn()
    const off = registerLive('a', '/api/v1/tickets', first)
    registerLive('a', '/api/v1/tickets?page=2', second)
    off()
    handleLiveChanged('a')
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
  })

  it('a new connection re-sends and re-adds every key', async () => {
    setLiveConnection('c1')
    const a = vi.fn()
    registerLive('a', '/api/v1/tickets', a)
    await settle()
    clearLiveConnection()
    setLiveConnection('c2')
    await settle()
    expect(lastCall()[0]).toBe('c2')
    expect(a).toHaveBeenCalledTimes(2)
  })

  it('on 404 waits for the next ready', async () => {
    putLiveSubscriptions.mockRejectedValueOnce(
      new ApiError('gone', { kind: 'http', status: 404, original: null })
    )
    setLiveConnection('c1')
    registerLive('a', '/api/v1/tickets', () => {})
    await settle()
    registerLive('b', '/api/v1/features', () => {})
    await settle()
    expect(putLiveSubscriptions).toHaveBeenCalledTimes(1)

    setLiveConnection('c2')
    await settle()
    expect(putLiveSubscriptions).toHaveBeenCalledTimes(2)
    expect(sentKeys()).toEqual(['a', 'b'])
  })

  it('on 409 reconnects', async () => {
    const reconnect = vi.fn()
    setLiveReconnect(reconnect)
    putLiveSubscriptions.mockRejectedValueOnce(
      new ApiError('stale', { kind: 'http', status: 409, original: null })
    )
    setLiveConnection('c1')
    registerLive('a', '/api/v1/tickets', () => {})
    await settle()
    expect(reconnect).toHaveBeenCalledTimes(1)
  })

  it('backs off between reconnects while the PUT keeps getting 409', async () => {
    // No jitter, so the exponential steps are exact.
    vi.spyOn(Math, 'random').mockReturnValue(0)
    const reconnect = vi.fn()
    setLiveReconnect(reconnect)
    putLiveSubscriptions.mockRejectedValue(
      new ApiError('stale', { kind: 'http', status: 409, original: null })
    )
    registerLive('a', '/api/v1/tickets', () => {})
    setLiveConnection('c1')
    await vi.advanceTimersByTimeAsync(50 + 1000)
    expect(reconnect).toHaveBeenCalledTimes(1)

    setLiveConnection('c2')
    await vi.advanceTimersByTimeAsync(50 + 1999)
    expect(reconnect).toHaveBeenCalledTimes(1) // second wait is 2s
    await vi.advanceTimersByTimeAsync(1)
    expect(reconnect).toHaveBeenCalledTimes(2)
  })

  it('adds up to the jitter maximum to the 409 backoff', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    const reconnect = vi.fn()
    setLiveReconnect(reconnect)
    putLiveSubscriptions.mockRejectedValue(
      new ApiError('stale', { kind: 'http', status: 409, original: null })
    )
    registerLive('a', '/api/v1/tickets', () => {})
    setLiveConnection('c1')
    await vi.advanceTimersByTimeAsync(50 + 1000 + REFETCH_JITTER_MAX_MS / 2 - 1)
    expect(reconnect).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(reconnect).toHaveBeenCalledTimes(1)
  })

  it('defers the baseline refetch of a hidden tab until it is shown', async () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    const onChange = vi.fn()
    registerLive('a', '/api/v1/tickets', onChange)
    setLiveConnection('c1')
    await settle()
    expect(onChange).not.toHaveBeenCalled()
    setLiveConnection('c2')
    await settle()
    visibility.mockReturnValue('visible')
    installVisibleRefetch(useQueryCacheStub())
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(REFETCH_JITTER_MAX_MS)
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('sends at most the server limit', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    setLiveConnection('c1')
    for (let i = 0; i <= MAX_LIVE_SUBSCRIPTIONS; i++)
      registerLive(`k${i}`, '/api/v1/tickets', () => {})
    await settle()
    expect(sentKeys()).toHaveLength(MAX_LIVE_SUBSCRIPTIONS)
  })
})
