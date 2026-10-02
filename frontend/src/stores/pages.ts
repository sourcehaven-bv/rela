import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { SidebarPage } from '@/types'

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

  function set(next: Record<string, SidebarPage> | undefined) {
    pages.value = next ?? {}
    loaded.value = true
  }

  return { pages, loaded, set }
})
