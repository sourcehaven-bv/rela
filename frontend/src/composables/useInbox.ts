import { onScopeDispose, shallowRef } from 'vue'
import { useQueryCache } from '@pinia/colada'
import { getInbox } from '@/api/channels'
import { liveInboxPath } from '@/api/live'
import { shouldDropHeldContent } from '@/api/errors'
import { useLiveSubscription } from '@/composables/useLiveSubscription'
import { storeInboxSummary } from '@/composables/useInboxSummary'
import type { Inbox } from '@/types/channels'

/**
 * The inbox page's data (TKT-9YRRM5): the first page of every kind, live on
 * `/api/v1/_inbox` while the page is mounted.
 *
 * A refused refetch (401/403/404) clears what is held: holding it would keep
 * showing rows this user may no longer read. Any other failure keeps it.
 * Each answer also becomes the sidebar's summary, so the badge matches the
 * page without a second request.
 */
export function useInbox() {
  const queryCache = useQueryCache()
  const inbox = shallowRef<Inbox | null>(null)
  const error = shallowRef<unknown>(null)
  const loading = shallowRef(true)
  let seq = 0

  async function refetch() {
    const mine = ++seq
    try {
      const got = await getInbox()
      if (mine !== seq) return
      inbox.value = got
      error.value = null
      storeInboxSummary(queryCache, got)
    } catch (err) {
      if (mine !== seq) return
      error.value = err
      if (shouldDropHeldContent(err)) inbox.value = null
    } finally {
      if (mine === seq) loading.value = false
    }
  }

  void refetch()

  // An answer that arrives after the page is gone must not land.
  onScopeDispose(() => {
    seq++
  })

  useLiveSubscription('inbox', liveInboxPath(), () => void refetch())

  return { inbox, error, loading, refetch }
}
