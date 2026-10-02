import { computed } from 'vue'
import { createRelation } from '@/api/entities'
import { usePageStore } from '@/stores/pages'
import { useUIStore } from '@/stores/ui'
import { pageScopeParams } from '@/utils/listParams'
import type { Entity, PageScope } from '@/types'

/**
 * The scope a list or kanban shows when it is a tab of an entity page: the
 * rows the page's entity (the anchor) reaches over the tab's relation.
 *
 * `params` narrows the collection read. The server resolves the relation and
 * direction from config, so the request names only the page, the tab and the
 * anchor.
 *
 * `linkCreated` links a row created in the tab to the anchor, so the new row
 * lands in the tab it was added from. The edge is created from the anchor's
 * side with the tab's direction, which covers both directions in one call
 * shape. When the link fails the row still exists, so the reader is told
 * which row to link by hand.
 *
 * Both are inert when `scope()` is undefined, which is every list and board
 * outside an entity page.
 */
export function usePageTabScope(scope: () => PageScope | undefined) {
  const pageStore = usePageStore()
  const uiStore = useUIStore()

  const params = computed(() => pageScopeParams(scope()))

  const binding = computed(() => {
    const s = scope()
    if (!s) return undefined
    const page = pageStore.pages[s.page]
    const tab = page?.tabs.find((t) => t.id === s.tab)
    if (!page?.entity_type || !tab?.relation) return undefined
    return { anchorType: page.entity_type, anchor: s.entity, relation: tab.relation, direction: tab.direction }
  })

  async function linkCreated(entity: Entity): Promise<void> {
    const b = binding.value
    if (!b) return
    try {
      await createRelation(b.anchorType, b.anchor, b.relation, entity.id, undefined, b.direction)
    } catch {
      uiStore.error(`${entity.id} was created, but linking it to ${b.anchor} failed. Link it by hand.`)
    }
  }

  return { params, linkCreated }
}
