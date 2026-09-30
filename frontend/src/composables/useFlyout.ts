import { computed, shallowRef } from 'vue'
import type { Entity } from '@/types'

/**
 * The panels a sidebar entry slides out over the page (`open: flyout`).
 *
 * A module-level ref for the same reason as useDetailPanel: the sidebar that
 * opens the flyout and the layer in App.vue that draws it are siblings, and
 * neither is a parent of the other.
 *
 * Not in the URL. A flyout is a quick look at something while staying on the
 * page, so it closes on the next navigation, and a reload or a shared link
 * should not bring it back.
 */

export interface FlyoutList {
  /** The sidebar row that opened it, for the row's pressed state. */
  navId: string
  title: string
  listId: string
  /** The list's own page, for the expand control. */
  href: string
}

const list = shallowRef<FlyoutList | null>(null)
/** The row opened in a second panel beside the list. */
const entity = shallowRef<Entity | null>(null)

export function useFlyout() {
  /** Opens a list, or closes it when that same list is already open. */
  function toggle(next: FlyoutList) {
    if (list.value?.navId === next.navId) {
      close()
      return
    }
    list.value = next
    entity.value = null
  }

  function openEntity(next: Entity) {
    entity.value = next
  }

  function closeEntity() {
    entity.value = null
  }

  function close() {
    list.value = null
    entity.value = null
  }

  return {
    list: computed(() => list.value),
    entity: computed(() => entity.value),
    openNavId: computed(() => list.value?.navId ?? null),
    toggle,
    openEntity,
    closeEntity,
    close,
  }
}

/** Test seam: reset module state between cases. */
export function resetFlyout() {
  list.value = null
  entity.value = null
}
