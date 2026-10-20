import { shallowRef } from 'vue'

/**
 * The operations in flight on an open channel, by the pending keys of
 * rela-components' channelFields (messagePendingKey, lifecyclePendingKey).
 * The library shows a button's pending label while its key is held. The
 * set is replaced, never mutated, so the library's prop sees each change.
 */
export function usePendingKeys(onError: (err: unknown) => void) {
  const pending = shallowRef<ReadonlySet<string>>(new Set())

  function set(key: string, on: boolean) {
    const next = new Set(pending.value)
    if (on) next.add(key)
    else next.delete(key)
    pending.value = next
  }

  /**
   * Runs op under key, unless key is already in flight. A failure goes to
   * onError and answers undefined.
   */
  async function run<T>(key: string, op: () => Promise<T>): Promise<T | undefined> {
    if (pending.value.has(key)) return undefined
    set(key, true)
    try {
      return await op()
    } catch (err) {
      onError(err)
      return undefined
    } finally {
      set(key, false)
    }
  }

  return { pending, set, run }
}
