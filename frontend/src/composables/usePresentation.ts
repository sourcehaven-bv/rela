import { computed, onScopeDispose, shallowRef, watchEffect } from 'vue'

/**
 * Present mode of a steps page (TKT-35BY3PE): the shell hides the sidebar and
 * the next-action card and enlarges the content, for a screen the whole room
 * reads.
 *
 * A module-level flag, like usePageHeader, because the shell that changes is
 * in App.vue and the page that knows it is presenting is a routed view below
 * it. The page publishes a getter rather than setting the flag once, so the
 * flag follows the route: browser back out of `?present=1` restores the shell.
 */
const presenting = shallowRef(false)

/** Read side, for the app shell. */
export function usePresentationOutlet() {
  return { presenting: computed(() => presenting.value) }
}

/**
 * Write side, for a page. `source` is read reactively; the flag clears when
 * the calling scope is disposed, so leaving the page restores the shell.
 */
export function usePresentation(source: () => boolean) {
  watchEffect(() => {
    presenting.value = source()
  })
  onScopeDispose(() => {
    presenting.value = false
  })
}

/** Test seam: reset module state between cases. */
export function resetPresentation() {
  presenting.value = false
}
