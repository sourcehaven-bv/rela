import { ref, watch, type Ref } from 'vue'
import { listVersions } from '@/api/history'

export interface HasHistorySource {
  /** Whether History is offered at all: the capability and the operator's choice. */
  enabled: () => boolean
  entityType: () => string
  /** The address on screen (`POL-1@draft`), or null until the entry loads. */
  address: () => string | null
  world: () => string | undefined
}

/**
 * Whether the face on screen has any recorded version (BUG-9WCZYOX), so the
 * History button is shown only when it leads somewhere. One timeline read per
 * address, made only where History is offered; nothing is read otherwise.
 *
 * False until the answer arrives, and on any failure: a hidden button is the
 * quiet answer, and the history page keeps its own error handling. An answer
 * for an address that is no longer on screen is dropped. A version the sweep
 * records after the page loaded shows the button on the next visit.
 */
export function useHasHistory(src: HasHistorySource): Ref<boolean> {
  const has = ref(false)
  let asked = 0
  watch(
    () => [src.enabled(), src.entityType(), src.address(), src.world()] as const,
    ([enabled, type, address, world]) => {
      const mine = ++asked
      has.value = false
      if (!enabled || !address) return
      listVersions(type, address, world)
        .then((timeline) => {
          if (mine === asked) has.value = timeline.versions.length > 0
        })
        .catch(() => {
          if (mine === asked) has.value = false
        })
    },
    { immediate: true }
  )
  return has
}
