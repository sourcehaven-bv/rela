import { onBeforeUnmount, watch, type Ref } from 'vue'
import { useRoute } from 'vue-router'
import { useQuery } from '@pinia/colada'
import { getNavStatus } from '@/api'
import { useEvents } from '@/composables/useEvents'
import type { NavItemStatus } from 'rela-components/types'
import { useSpaceStore } from '@/stores/space'

/** How long a burst of entity changes is left to settle before recounting. */
const EVENT_DEBOUNCE_MS = 1000

/**
 * The status markers for the sidebar, keyed by each entry's `status_key`.
 *
 * Fetched only when some entry declares `status:` rules, so a config without
 * any costs no request. Recounted on navigation and after an entity change:
 * a write is what moves a count, and the live-update feed already reports
 * every one. There is no polling.
 *
 * A failed fetch keeps the last markers rather than clearing them. A marker
 * is advisory, and the list it points at is still the truth.
 */
export function useNavStatus(enabled: Ref<boolean>) {
  const route = useRoute()
  const { on } = useEvents()
  const space = useSpaceStore()

  // Keyed on the space: a count belongs to the entries of one space, and
  // markers from the space just left must not show under the new one.
  const query = useQuery({
    key: () => ['nav-status', space.current ?? ''],
    query: () => getNavStatus(space.current),
    enabled: () => enabled.value,
  })

  function recount() {
    if (enabled.value) void query.refetch()
  }

  watch(() => route.path, recount)

  let timer: ReturnType<typeof setTimeout> | null = null
  on('entity:changed', () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(recount, EVENT_DEBOUNCE_MS)
  })
  onBeforeUnmount(() => {
    if (timer) clearTimeout(timer)
  })

  return {
    statusFor(key: string | undefined): NavItemStatus | undefined {
      return key ? query.data.value?.items[key] : undefined
    },
  }
}
