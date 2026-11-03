import { nextTick, watch, type Ref } from 'vue'

/** The part of RlChannel a message link drives. */
export interface LinkTarget {
  jumpTo(seq: number): void
}

/**
 * A message link's reveal on an open channel (TKT-O6RC8Z). It waits until
 * seq is set and the log is mounted, then one tick more so the log's scroll
 * to the bottom on mount has run. OpenChannel mounts the log only once it
 * holds the channel's head, so a mounted log means the first page is held.
 * A call whose seq or log changed during its tick stops: the change runs
 * the watcher again, and that call reveals the new seq. So a change is
 * never dropped and a message is never jumped to twice. The call that
 * goes on calls jumpTo once and onDone. jumpTo reveals a held message, or asks for
 * older pages with `jump`, which the channel's paging answers.
 */
export function useMessageLink(
  seq: () => number | undefined,
  view: Readonly<Ref<LinkTarget | null>>,
  onDone: () => void
) {
  watch(
    [seq, view],
    async ([s, v]) => {
      if (s === undefined || !v) return
      await nextTick()
      if (seq() !== s || view.value !== v) return
      v.jumpTo(s)
      onDone()
    },
    { immediate: true }
  )
}
