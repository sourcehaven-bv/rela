import { onScopeDispose, ref, watch, type Ref } from 'vue'
import type { EphemeralMessage } from '@/types/channels'
import { nextExpiry } from './channelView'

/**
 * A clock for ephemeral signals (TKT-91BK6F R17): `now` moves on when the
 * soonest of `signals` expires, so a view filtered on `now` drops it then.
 * One timer at a time, re-armed when the signals or `now` change, and
 * cleared with the calling scope.
 */
export function useExpiryClock(
  signals: () => readonly EphemeralMessage[] | undefined
): Readonly<Ref<number>> {
  const now = ref(Date.now())
  let timer: ReturnType<typeof setTimeout> | undefined

  function stop() {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }

  watch(
    [signals, now],
    ([list]) => {
      stop()
      const at = nextExpiry(list ?? [], now.value)
      if (at !== undefined) timer = setTimeout(() => (now.value = Date.now()), at - now.value)
    },
    { immediate: true }
  )
  onScopeDispose(stop)
  return now
}
