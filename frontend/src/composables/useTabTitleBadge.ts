import { onBeforeUnmount, watch, type Ref } from 'vue'
import type { InboxSummary } from '@/types/channels'
import { setTitleBadgeCount } from '@/composables/useDocumentTitle'

/**
 * The count the tab title carries: unread mentions and replies plus what
 * waits on the user. Unread channels are left out; they are not addressed
 * to the user.
 */
export function titleCount(summary: InboxSummary | undefined): number {
  if (!summary) return 0
  return summary.mentions + summary.waiting
}

/**
 * Puts the inbox count in front of the tab title (TKT-48S88RB). The count
 * goes to useDocumentTitle, which writes the title. A hidden tab's count
 * moves only in the tab that notifies, since the summary refetches only in
 * a visible tab (TASK-550RD).
 */
export function useTabTitleBadge(summary: Ref<InboxSummary | undefined>) {
  watch(() => titleCount(summary.value), setTitleBadgeCount, { immediate: true })
  onBeforeUnmount(() => setTitleBadgeCount(0))
}
