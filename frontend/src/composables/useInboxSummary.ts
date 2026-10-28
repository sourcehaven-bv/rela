import { computed, onBeforeUnmount, watch, type Ref } from 'vue'
import { useQuery, type QueryCache } from '@pinia/colada'
import { getInboxSummary } from '@/api/channels'
import { liveInboxSummaryPath } from '@/api/live'
import { ApiError, shouldDropHeldContent } from '@/api/errors'
import { problemType } from '@/components/entity/channels/channelErrors'
import { useEvents } from '@/composables/useEvents'
import { registerLive } from '@/composables/useLiveSubscription'
import { cancelWhenVisible, runWhenVisible } from '@/composables/visibleRefetch'
import type { Inbox, InboxSummary } from '@/types/channels'

/** The Pinia Colada key of the inbox summary, shared by the sidebar and the page. */
export const INBOX_SUMMARY_KEY = ['inbox-summary']

/**
 * The live key of the summary. It is fixed, so registering it again replaces
 * the entry in place and the badge keeps its slot among the subscriptions
 * the stream accepts (MAX_LIVE_SUBSCRIPTIONS). The worst case, an entity page
 * with its channels open plus the inbox page and this badge, stays under it.
 */
export const INBOX_SUMMARY_LIVE_KEY = 'inbox-summary'

/**
 * What the query holds. `hidden` says the inbox is not available to this
 * caller (realtime off, or no identity); `summary` is absent then, and also
 * after a refusal that drops held content.
 */
export interface InboxSummaryState {
  hidden: boolean
  summary?: InboxSummary
}

/** A 404 (realtime off or channels not wired) or a 403 `no_principal`. */
function hidesInbox(err: unknown): boolean {
  if (!(err instanceof ApiError)) return false
  return err.status === 404 || (err.status === 403 && problemType(err) === 'no_principal')
}

/**
 * Reads the summary. A refusal is an answer, not a failure: it hides the
 * item or drops the badge, per shouldDropHeldContent. Any other error
 * throws, so the query keeps the last badge (a network error or a 5xx says
 * nothing about what the user may read).
 */
async function readSummary(): Promise<InboxSummaryState> {
  try {
    return { hidden: false, summary: await getInboxSummary() }
  } catch (err) {
    if (hidesInbox(err)) return { hidden: true }
    if (shouldDropHeldContent(err)) return { hidden: false }
    throw err
  }
}

/** The counts of an inbox first page read with default parameters. */
export function summaryOfInbox(inbox: Inbox): InboxSummary {
  return {
    waiting: inbox.count ?? 0,
    waiting_truncated: inbox.truncated ?? false,
    unread_channels: inbox.unread_count ?? 0,
    mentions: inbox.mention_count ?? 0,
    degraded: inbox.degraded,
  }
}

/**
 * Writes the counts of a default inbox page into the summary query, so the
 * badge matches the page without a second request.
 */
export function storeInboxSummary(cache: QueryCache, inbox: Inbox) {
  const state: InboxSummaryState = { hidden: false, summary: summaryOfInbox(inbox) }
  cache.setQueryData(INBOX_SUMMARY_KEY, state)
}

/**
 * The sidebar's inbox summary (TKT-9YRRM5): a Pinia Colada query, live on
 * `/api/v1/_inbox/summary` under one fixed key.
 *
 * `enabled` is true when the channel schema has loaded channel types. The
 * summary is registered live once the first answer arrives, and removed when
 * the inbox turns out hidden or the caller unmounts. Call it from a
 * component that lives as long as the app (the sidebar). A `live:changed`
 * frame, the app's `refresh` event and a return to the tab refetch it.
 */
export function useInboxSummary(enabled: Ref<boolean>) {
  const { on } = useEvents()

  const query = useQuery({
    key: INBOX_SUMMARY_KEY,
    query: readSummary,
    enabled: () => enabled.value,
  })

  const refetchKey = Symbol('inbox-summary')
  function refetch() {
    runWhenVisible(refetchKey, () => {
      if (enabled.value) void query.refetch()
    })
  }

  let unregister: (() => void) | null = null
  function setLive(live: boolean) {
    if (live && !unregister) {
      unregister = registerLive(INBOX_SUMMARY_LIVE_KEY, liveInboxSummaryPath(), refetch)
    } else if (!live && unregister) {
      unregister()
      unregister = null
    }
  }

  watch(
    () => [enabled.value, query.data.value] as const,
    ([live, state]) => setLive(live && state !== undefined && !state.hidden),
    { immediate: true }
  )

  on('refresh', refetch)

  onBeforeUnmount(() => {
    cancelWhenVisible(refetchKey)
    setLive(false)
  })

  const hidden = computed(() => query.data.value?.hidden ?? false)
  const summary = computed(() => (enabled.value ? query.data.value?.summary : undefined))

  return { summary, hidden, refetch }
}
