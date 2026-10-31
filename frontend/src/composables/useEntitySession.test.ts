import { afterEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'

import { ApiError } from '@/api/errors'
import { READ_RETRIES, useEntitySession, type EntitySessionOptions } from './useEntitySession'

// Single orders useEntitySession.prop.test.ts reaches rarely or cannot see.

const failure = (status: number) =>
  new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const scopes: ReturnType<typeof effectScope>[] = []
afterEach(() => {
  scopes.splice(0).forEach((s) => s.stop())
  vi.useRealTimers()
})

function session(over: Partial<EntitySessionOptions> = {}, address = ref('A')) {
  const reload = vi.fn(async (_full: boolean) => {})
  const onRefused = vi.fn()
  const scope = effectScope()
  scopes.push(scope)
  const s = scope.run(() => useEntitySession(() => address.value, { reload, onRefused, ...over }))!
  return { s, reload, onRefused, address }
}

describe('useEntitySession', () => {
  it('lands nothing after a refusal, also a newer read waiting in prepare', async () => {
    const { s, onRefused } = session()
    const older = deferred<string>()
    const newer = deferred<string>()
    const prepared = deferred<boolean>()
    const land = vi.fn()
    const first = s.read(() => older.promise, { land })
    const second = s.read(() => newer.promise, { land, prepare: () => prepared.promise })

    newer.resolve('newer')
    await nextTick()
    older.reject(failure(404))
    expect(await first).toMatchObject({ kind: 'refused' })
    expect(onRefused).toHaveBeenCalledTimes(1)

    prepared.resolve(true)
    expect(await second).toEqual({ kind: 'superseded' })
    expect(land).not.toHaveBeenCalled()
  })

  it('does not void a read with a write to another address', async () => {
    const { s } = session()
    const answer = deferred<string>()
    const land = vi.fn()
    const read = s.read(() => answer.promise, { land })
    // The edits of the entity being left drain after the route moved on.
    await s.write(async () => 'saved', 'B')
    answer.resolve('A1')
    expect(await read).toEqual({ kind: 'landed', value: 'A1' })
  })

  it('reads again after a write to its address, and lands the newer answer', async () => {
    const { s } = session()
    const answers = ['A0', 'A1']
    const first = deferred<void>()
    const land = vi.fn()
    let sends = 0
    const read = s.read(
      async () => {
        const v = answers[sends++]!
        if (sends === 1) await first.promise
        return v
      },
      { land }
    )
    await s.write(async () => 'saved')
    first.resolve()
    expect(await read).toEqual({ kind: 'landed', value: 'A1' })
    expect(land).toHaveBeenCalledOnce()
  })

  it('reloads in full when a load runs out of retries', async () => {
    const { s, reload } = session()
    // Every answer arrives after one more write has completed.
    const read = s.read(
      async () => {
        await s.write(async () => 'saved')
        return 'old'
      },
      { land: vi.fn() }
    )
    expect(await read).toEqual({ kind: 'stale' })
    expect(reload).toHaveBeenCalledWith(true)
    expect(READ_RETRIES).toBeGreaterThan(0)
  })

  it('reloads lightly when a quiet refresh meets a held edit', async () => {
    const holds = ref(true)
    const { s, reload } = session({ holdsWrites: () => holds.value })
    expect(await s.read(async () => 'v', { land: vi.fn(), background: true, quiet: true })).toEqual(
      { kind: 'stale' }
    )
    expect(reload).not.toHaveBeenCalled()
    holds.value = false
    await nextTick()
    expect(reload).toHaveBeenCalledWith(false)
  })

  it('tries a failed background reload again after a backoff', async () => {
    vi.useFakeTimers()
    const { s, reload } = session()
    const outcome = await s.read(
      async () => {
        throw failure(503)
      },
      { land: vi.fn(), background: true }
    )
    expect(outcome).toMatchObject({ kind: 'failed', current: true })
    expect(reload).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1000)
    expect(reload).toHaveBeenCalledWith(true)
  })

  it('reports a failing land as a failure rather than throwing', async () => {
    const { s } = session()
    const outcome = await s.read(async () => 'v', {
      land: () => {
        throw new Error('boom')
      },
    })
    expect(outcome).toMatchObject({ kind: 'failed' })
  })
})
