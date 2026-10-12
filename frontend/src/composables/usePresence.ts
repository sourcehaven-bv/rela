import { ref, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { getPresence } from '@/api/presence'
import { livePresencePath } from '@/api/live'
import { shouldDropHeldContent } from '@/api/errors'
import { useLiveSubscription } from '@/composables/useLiveSubscription'
import type { PresenceViewer } from '@/types/entity'

/** What usePresence needs to name one face. */
export interface PresenceTarget {
  type: string
  /** `ID` on a type without faces, `ID@face` on one with faces. */
  address: string
  world?: string
}

/**
 * Who else has an entity's face open (TKT-MHXH87).
 *
 * The live subscription on the presence path is what lists this user as a
 * viewer, so it lasts exactly as long as the component. The server sends
 * `live:changed` when the viewers move, and the composable re-reads them.
 *
 * A refused re-read (401/403/404) clears the viewers: holding the old list
 * would keep showing who reads an entity this user may no longer see. Any
 * other failure keeps the list, as a transient error should.
 *
 * `target` null means nothing to show yet (the entry has not loaded).
 */
export function usePresence(target: MaybeRefOrGetter<PresenceTarget | null>) {
  const viewers = ref<PresenceViewer[]>([])
  const more = ref(0)
  let seq = 0

  function clear() {
    viewers.value = []
    more.value = 0
  }

  async function refetch() {
    const t = toValue(target)
    const mine = ++seq
    if (!t) {
      clear()
      return
    }
    try {
      const res = await getPresence(t.type, t.address, t.world)
      if (mine !== seq) return
      viewers.value = res.viewers ?? []
      more.value = res.more ?? 0
    } catch (err) {
      if (mine !== seq) return
      if (shouldDropHeldContent(err)) clear()
    }
  }

  const targetKey = () => {
    const t = toValue(target)
    return t ? `${t.type}/${t.address}/${t.world ?? ''}` : ''
  }
  watch(
    targetKey,
    () => {
      // Another face's viewers must not linger while the new list loads.
      clear()
      void refetch()
    },
    { immediate: true }
  )

  useLiveSubscription(
    'entity-presence',
    () => {
      const t = toValue(target)
      return t ? livePresencePath(t.type, t.address, t.world) : null
    },
    () => void refetch()
  )

  return { viewers, more, refetch }
}
