import { computed, onScopeDispose, shallowRef, type Slot } from 'vue'
import type { ViewTab } from 'rela-components/components/layout/RlViewTabs.vue'

/**
 * The page header band that spans the content and the detail panel, as a
 * module-level slot the routed view fills and the app shell renders.
 *
 * Same shape, and the same reason, as useDetailPanel: the header lives in
 * `RlAppShell`'s `#header` slot in `App.vue`, two levels above the routed
 * view that knows the title, the actions and the tools. See that file for
 * why this is a module-level ref rather than a Teleport or a provide/inject.
 *
 * Hoisting it is what makes the header span BOTH panes. Left in the view, it
 * would sit inside `#main` and stop at the panel's left edge, so opening a
 * panel would visually cut the page title in half.
 *
 * It carries SLOTS rather than a component and props, unlike useDetailPanel.
 * A header's tools are the view's own live controls — the search box and
 * filter menu that its keyboard shortcuts hold template refs to — and a slot
 * renders in the scope that declared it, so those refs keep resolving from
 * the view. Passing them as props would mean marshalling refs across the
 * boundary and re-binding every event by hand.
 */

interface HeaderContent {
  title: string
  /** Right-aligned action cluster: create, export, whatever the view offers. */
  actions?: Slot
  /** The lower bar: search, filter, sort. */
  tools?: Slot
}

/**
 * The page a tabbed screen shows (TKT-ITQ0HL): its title and its tab bar.
 *
 * A separate channel from HeaderContent because two components fill the
 * header on such a screen. The page owns the title and the tabs; the view
 * mounted in the active tab still owns the actions and tools. With one
 * channel the last writer would erase the other's half.
 */
interface PageFrame {
  title: string
  /** Beside the title: an entity page's `badge:` property, drawn by its widget. */
  badge?: Slot
  /** The trigger of an entity page's menu, beside the title. */
  menu?: Slot
  /** The tabs to show. The shell hides the bar when there is one or none. */
  tabs: ViewTab[]
  active: string
  select: (id: string) => void
}

const content = shallowRef<HeaderContent | null>(null)
const frame = shallowRef<PageFrame | null>(null)

/** Read side, for the app shell. The page's title wins over the view's. */
export function usePageHeaderOutlet() {
  return {
    content: computed(() => content.value),
    frame: computed(() => frame.value),
    title: computed(() => frame.value?.title ?? content.value?.title ?? ''),
    badge: computed(() => frame.value?.badge),
    menu: computed(() => frame.value?.menu),
    present: computed(() => content.value !== null || frame.value !== null),
  }
}

/**
 * Write side, for a tabbed page. Clears on scope dispose, like
 * usePageHeader.
 */
export function usePageFrame() {
  function show(next: PageFrame) {
    frame.value = next
  }

  function clear() {
    frame.value = null
  }

  onScopeDispose(clear)

  return { show, clear }
}

/**
 * Write side, for a routed view. Clears on scope dispose, so navigating from
 * a view that sets a header to one that does not cannot leave the previous
 * title stranded above the new screen.
 */
export function usePageHeader() {
  function show(next: HeaderContent) {
    content.value = next
  }

  function clear() {
    content.value = null
  }

  onScopeDispose(clear)

  return { show, clear }
}

/** Test seam: reset module state between cases. */
export function resetPageHeader() {
  content.value = null
  frame.value = null
}
