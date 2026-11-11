import { onBeforeUnmount, watch, type Ref } from 'vue'
import type { InboxSummary } from '@/types/channels'

/**
 * The count the tab title carries: unread mentions and replies plus what
 * waits on the user. Unread channels are left out; they are not addressed
 * to the user.
 */
export function titleCount(summary: InboxSummary | undefined): number {
  if (!summary) return 0
  return summary.mentions + summary.waiting
}

/** The title with a `(N) ` prefix, or the title alone for zero. */
export function badgedTitle(base: string, count: number): string {
  return count > 0 ? `(${count}) ${base}` : base
}

/**
 * Puts the inbox count in front of the tab title (TKT-48S88RB). The title
 * the page had when this is called is the base; nothing else in the SPA
 * sets the title. A hidden tab's count moves only in the tab that notifies,
 * since the summary refetches only in a visible tab (TASK-550RD).
 */
export function useTabTitleBadge(summary: Ref<InboxSummary | undefined>) {
  const base = document.title
  watch(
    () => titleCount(summary.value),
    (n) => {
      document.title = badgedTitle(base, n)
    },
    { immediate: true }
  )
  onBeforeUnmount(() => {
    document.title = base
  })
}
