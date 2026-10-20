import { describe, it, expect, vi, afterEach } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import type { EphemeralMessage } from '@/types/channels'
import type { Entity } from '@/types'
import { useExpiryClock } from './useExpiryClock'
import { usePendingKeys } from './usePendingKeys'
import { useRefPicker } from './useRefPicker'

// The small composables OpenChannel is built from (TKT-91BK6F), tested
// without mounting it.

function signal(id: string, expiresAt: string): EphemeralMessage {
  return { id, type: 'typing', author: { user: 'ann' }, at: expiresAt, expires_at: expiresAt }
}

afterEach(() => vi.useRealTimers())

describe('useExpiryClock', () => {
  it('moves now on at the soonest expiry, one timer at a time, and stops with its scope', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.setSystemTime(new Date('2026-03-01T10:00:00Z'))
    const signals = ref<EphemeralMessage[]>([
      signal('a', '2026-03-01T10:00:05Z'),
      signal('b', '2026-03-01T10:00:02Z'),
    ])
    const scope = effectScope()
    const now = scope.run(() => useExpiryClock(() => signals.value))!
    const start = now.value

    vi.advanceTimersByTime(2_000)
    await nextTick()
    expect(now.value).toBe(start + 2_000)
    expect(vi.getTimerCount()).toBe(1)

    vi.advanceTimersByTime(3_000)
    await nextTick()
    expect(now.value).toBe(start + 5_000)
    expect(vi.getTimerCount()).toBe(0)

    signals.value = [signal('c', '2026-03-01T10:00:09Z')]
    await nextTick()
    expect(vi.getTimerCount()).toBe(1)
    scope.stop()
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('usePendingKeys', () => {
  it('holds a key while its op runs, refuses a second run and reports a failure', async () => {
    const onError = vi.fn()
    const { pending, run } = usePendingKeys(onError)
    let finish!: (v: string) => void
    const first = run('k', () => new Promise<string>((r) => (finish = r)))
    expect(pending.value.has('k')).toBe(true)
    expect(await run('k', async () => 'again')).toBeUndefined()
    finish('done')
    expect(await first).toBe('done')
    expect(pending.value.has('k')).toBe(false)

    const err = new Error('refused')
    expect(await run('k', () => Promise.reject(err))).toBeUndefined()
    expect(onError).toHaveBeenCalledWith(err)
    expect(pending.value.size).toBe(0)
  })
})

describe('useRefPicker', () => {
  const entity = { id: 'P-1', type: 'person', title: 'Pat', properties: {} } as unknown as Entity

  it('settles with the picked entity, with null on close, and with null when replaced', async () => {
    const picker = useRefPicker()
    const picked = picker.pickRef(['person'])
    expect(picker.open.value).toBe(true)
    expect(picker.types.value).toEqual(['person'])
    picker.onPicked('P-1', entity)
    expect(await picked).toEqual({ id: 'P-1', type: 'person', title: expect.any(String) })
    expect(picker.open.value).toBe(false)

    const closed = picker.pickRef(['person'])
    picker.onClose()
    expect(await closed).toBeNull()

    const replaced = picker.pickRef(['person'])
    const second = picker.pickRef(['team'])
    expect(await replaced).toBeNull()
    expect(picker.types.value).toEqual(['team'])
    picker.onClose()
    expect(await second).toBeNull()
  })
})
