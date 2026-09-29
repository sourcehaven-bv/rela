import { computed, onScopeDispose, shallowRef, type Component } from 'vue'

/**
 * The detail panel that sits beside a list, as a module-level slot the
 * routed view fills and the app shell renders.
 *
 * The panel lives in `RlAppShell`'s `#panel` slot, which is in `App.vue` two
 * levels above the routed view that knows what belongs there. A module-level
 * ref rather than a Teleport because the shell's panel element and the view
 * mount in an order Teleport cannot be told to wait for: on a cold load the
 * view resolves its lazy chunk after the shell has rendered, and on an
 * in-app navigation before the next shell render. A ref is order-free.
 *
 * Provide/inject would scope this correctly, but the shell would still need
 * a reactive holder to render from, so it would be this plus a ceremony.
 */

interface PanelContent {
  component: Component
  props: Record<string, unknown>
  /**
   * How the panel sits against the view behind it. A view that needs its
   * full width (a gantt, a calendar) asks for `overlay`; a list shares the
   * row with `inline`. Below 1080px RlAppShell forces `overlay` regardless.
   */
  mode: 'inline' | 'overlay'
}

const content = shallowRef<PanelContent | null>(null)

/** Read side, for the app shell. */
export function useDetailPanelOutlet() {
  return {
    content: computed(() => content.value),
    open: computed(() => content.value !== null),
    mode: computed(() => content.value?.mode ?? 'inline'),
  }
}

/**
 * Write side, for a routed view. Clears on scope dispose, so navigating away
 * from a list with an open panel cannot leave the panel behind on the next
 * screen.
 *
 * Two writers can share a screen: an entity page opens its anchor's details
 * while the list in its tab opens rows. Each clears only what it showed
 * itself, so a list resolving "no row selected" (as it does on every
 * refetch) does not close the page's panel.
 */
export function useDetailPanel() {
  let shown: PanelContent | null = null

  function show(next: PanelContent) {
    shown = next
    content.value = next
  }

  function clear() {
    if (shown !== null && content.value === shown) content.value = null
    shown = null
  }

  onScopeDispose(clear)

  return { show, clear }
}

/** Test seam: reset module state between cases. */
export function resetDetailPanel() {
  content.value = null
}
