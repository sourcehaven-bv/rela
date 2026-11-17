import { onBeforeUnmount } from 'vue'
import { useQuery } from '@pinia/colada'
import { getPageStatus } from '@/api/pageStatus'
import { useEvents } from '@/composables/useEvents'
import { useWorld } from '@/composables/useWorld'
import { cancelWhenVisible, runWhenVisible } from '@/composables/visibleRefetch'

/** How long a burst of entity changes is left to settle before judging again. */
const EVENT_DEBOUNCE_MS = 1000

/**
 * The done marks of a steps page (TKT-MWZF0UO), keyed by tab id.
 *
 * Fetched only when some step declares `done_when`, so a page without one
 * costs no request. Judged again after an entity change, since a write is
 * what moves a verdict; moving between steps changes nothing. There is no
 * polling.
 *
 * A mark is advisory: it never blocks Next. A failed fetch keeps the last
 * marks rather than clearing them.
 */
export function usePageStatus(opts: {
  page: () => string
  entity: () => string | undefined
  enabled: () => boolean
}) {
  const { on } = useEvents()
  const { worldParam } = useWorld()

  // Keyed on the page, its entity and the world: marks judged for one
  // entity or world must not show for another.
  const query = useQuery({
    key: () => ['page-status', opts.page(), opts.entity() ?? '', worldParam.value ?? ''],
    query: () => getPageStatus(opts.page(), opts.entity(), worldParam.value),
    enabled: opts.enabled,
  })

  const recountKey = Symbol('page-status')
  function recount() {
    runWhenVisible(recountKey, () => {
      if (opts.enabled()) void query.refetch()
    })
  }

  let timer: ReturnType<typeof setTimeout> | null = null
  on('entity:changed', () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(recount, EVENT_DEBOUNCE_MS)
  })
  onBeforeUnmount(() => {
    cancelWhenVisible(recountKey)
    if (timer) clearTimeout(timer)
  })

  return {
    /** Whether step `id` is done. False while loading, on failure, and for a step without a check. */
    done(id: string): boolean {
      return query.data.value?.steps[id]?.done === true
    },
  }
}
