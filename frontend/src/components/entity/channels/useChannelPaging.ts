import { nextTick, ref } from 'vue'
import type { useChannel } from '@/composables/useChannel'

/** How many older pages a reply quote's jump reads before it gives up. */
const JUMP_MAX_PAGES = 10

/** The open channel's paging the handlers drive. */
export type PagedChannel = Pick<
  ReturnType<typeof useChannel>,
  'messages' | 'hasOlder' | 'loadOlder' | 'loadNewest'
>

/**
 * The paging handlers of an open channel: load older, load newest, and
 * the jump to a quoted message. A failure goes to onError, which shows it
 * without a form. onHeld runs after the render once a jump has loaded the
 * quoted message, so the log can show, scroll to and focus it.
 */
export function useChannelPaging(
  ch: PagedChannel,
  onError: (err: unknown) => void,
  onHeld: (seq: number) => void
) {
  const loadingOlder = ref(false)

  async function onLoadOlder() {
    if (loadingOlder.value) return
    loadingOlder.value = true
    try {
      await ch.loadOlder()
    } catch (err) {
      onError(err)
    } finally {
      loadingOlder.value = false
    }
  }

  async function onLoadNewest() {
    try {
      await ch.loadNewest()
    } catch (err) {
      onError(err)
    }
  }

  /** Reads older pages until the quoted message is held, within a bound. */
  async function onJump(seq: number) {
    const held = () => ch.messages.value.some((m) => m.seq === seq)
    for (let i = 0; i < JUMP_MAX_PAGES && !held() && ch.hasOlder.value; i++) await onLoadOlder()
    if (!held()) return
    await nextTick()
    onHeld(seq)
  }

  return { loadingOlder, onLoadOlder, onLoadNewest, onJump }
}
