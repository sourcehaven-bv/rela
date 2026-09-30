import { onBeforeUnmount, watch, type Ref } from 'vue'
import { useRoute } from 'vue-router'
import { useQuery } from '@pinia/colada'
import { getNavItems, type NavItemList } from '@/api'
import { shouldDropHeldContent } from '@/api/errors'
import { useEvents } from '@/composables/useEvents'
import { useSpaceStore } from '@/stores/space'

/** How long a burst of entity changes is left to settle before refetching. */
const EVENT_DEBOUNCE_MS = 1000

/**
 * The entries of the sidebar's generated groups (`items_from:`), keyed by
 * each group's `items_key`.
 *
 * Fetched only when some group declares `items_from:`. Refetched on
 * navigation and after an entity change, like the status markers. Navigation
 * matters here too: a link or unlink sends no live update, so a new owner's
 * initial shows once the user moves on.
 *
 * A failed fetch keeps the last entries, EXCEPT when the server refused it.
 * The entries are entity titles, so a refused refetch means the principal may
 * no longer read what is on screen, and it is dropped (see
 * shouldDropHeldContent).
 */
export function useNavItems(enabled: Ref<boolean>) {
  const route = useRoute()
  const { on } = useEvents()
  const space = useSpaceStore()

  // Keyed on the space: entries belong to the groups of one space.
  const query = useQuery({
    key: () => ['nav-items', space.current ?? ''],
    query: () => getNavItems(space.current),
    enabled: () => enabled.value,
  })

  function refetch() {
    if (enabled.value) void query.refetch()
  }

  watch(() => route.path, refetch)

  let timer: ReturnType<typeof setTimeout> | null = null
  on('entity:changed', () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(refetch, EVENT_DEBOUNCE_MS)
  })
  onBeforeUnmount(() => {
    if (timer) clearTimeout(timer)
  })

  return {
    itemsFor(key: string): NavItemList | undefined {
      if (shouldDropHeldContent(query.error.value)) return undefined
      return query.data.value?.items[key]
    },
  }
}
