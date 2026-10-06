import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { PageScope, SidebarPage } from '@/types'

/**
 * The pages of `pages:`, each with the tabs this principal may see
 * (TKT-ITQ0HL).
 *
 * Fed by the `_sidebar` response, like the space store: which tabs are
 * visible depends on who asks, and `_config` serves every tab to everyone.
 * `loaded` tells a page that has not seen the sidebar yet apart from a page
 * id the config does not declare.
 */
export const usePageStore = defineStore('pages', () => {
  const pages = ref<Record<string, SidebarPage>>({})
  const loaded = ref(false)

  /**
   * The entity page tab on screen, set by PageView while it shows an anchor.
   * The space's Create menu sits outside the page and reads it to link a new
   * row to the anchor. PageView clears it on unmount, which holds while
   * RouterView has no KeepAlive or leave Transition around it; with either, a
   * page off screen could still be published here.
   */
  const current = ref<PageScope>()

  function set(next: Record<string, SidebarPage> | undefined) {
    pages.value = next ?? {}
    loaded.value = true
  }

  return { pages, loaded, current, set }
})
