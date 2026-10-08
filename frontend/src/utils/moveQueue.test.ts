import { describe, it, expect, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { createMoveQueue } from './moveQueue'

describe('createMoveQueue', () => {
  it('sends one move at a time and settles once the queue is empty', async () => {
    const settle = vi.fn().mockResolvedValue(undefined)
    const enqueue = createMoveQueue(settle)
    let release!: () => void
    const first = vi.fn(() => new Promise<void>((r) => (release = r)))
    const second = vi.fn().mockResolvedValue(undefined)
    const done = [enqueue(first), enqueue(second)]
    await flushPromises()
    expect(first).toHaveBeenCalled()
    expect(second).not.toHaveBeenCalled()
    release()
    await Promise.all(done)
    expect(second).toHaveBeenCalled()
    expect(settle).toHaveBeenCalledTimes(1)
  })

  it('keeps sending after a failed send or settle', async () => {
    const settle = vi.fn().mockRejectedValue(new Error('refetch aborted'))
    const enqueue = createMoveQueue(settle)
    await enqueue(() => Promise.reject(new Error('nope')))
    const next = vi.fn().mockResolvedValue(undefined)
    await enqueue(next)
    expect(next).toHaveBeenCalled()
    expect(settle).toHaveBeenCalledTimes(2)
  })
})
