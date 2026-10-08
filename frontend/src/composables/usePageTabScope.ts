import { computed } from 'vue'
import { useQueryCache } from '@pinia/colada'
import { createRelation } from '@/api/entities'
import { entityKeys } from '@/queries/entities'
import { usePageStore } from '@/stores/pages'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { pageScopeParams } from '@/utils/listParams'
import { tabIsRelationOrdered } from '@/utils/relationOrder'
import type { Entity, PageScope, SidebarPage, SidebarPageLink } from '@/types'

/** How a new row is linked to a page's entity: the anchor and the edge to create from its side. */
export interface AnchorLink {
  anchorType: string
  anchor: string
  relation: string
  direction: 'outgoing' | 'incoming'
}

/*
 * The edge is created from the anchor's side with the link's direction, which
 * covers both directions in one call shape. When the link fails the row still
 * exists, so the reader is told which row to link by hand. Reports whether
 * the link was made.
 */
async function linkToAnchor(link: AnchorLink, entity: Entity): Promise<boolean> {
  try {
    await createRelation(
      link.anchorType,
      link.anchor,
      link.relation,
      entity.id,
      undefined,
      link.direction
    )
    return true
  } catch {
    useUIStore().error(
      `${entity.id} was created, but linking it to ${link.anchor} failed. Link it by hand.`
    )
    return false
  }
}

/*
 * The anchor link for one of the page's links. The anchor is the face on
 * screen: a content-scoped link from a faced anchor must name it, and a bare
 * id would be refused as face_required.
 */
function anchorLink(
  page: SidebarPage,
  s: PageScope,
  link: SidebarPageLink | undefined
): AnchorLink | undefined {
  if (!page.entity_type || !link) return undefined
  return {
    anchorType: page.entity_type,
    anchor: s.ref ?? s.entity,
    relation: link.relation,
    direction: link.direction,
  }
}

/**
 * The scope a list or kanban shows when it is a tab of an entity page: the
 * rows the page's entity (the anchor) reaches over the tab's relation.
 *
 * `params` narrows the collection read. The server resolves the relation and
 * direction from config, so the request names only the page, the tab and the
 * anchor.
 *
 * `linkCreated` links a row created in the tab to the anchor, so the new row
 * lands in the tab it was added from.
 *
 * Both are inert when `scope()` is undefined, which is every list and board
 * outside an entity page.
 */
export function usePageTabScope(scope: () => PageScope | undefined) {
  const pageStore = usePageStore()
  const schemaStore = useSchemaStore()

  const params = computed(() => pageScopeParams(scope()))

  // A relation tab shows one type, so it carries exactly one link.
  const binding = computed((): AnchorLink | undefined => {
    const s = scope()
    const page = s && pageStore.pages[s.page]
    const links = page?.tabs.find((t) => t.id === s?.tab)?.links
    return page && s && links?.length === 1 ? anchorLink(page, s, links[0]) : undefined
  })

  async function linkCreated(entity: Entity): Promise<void> {
    const b = binding.value
    if (b) await linkToAnchor(b, entity)
  }

  // Whether the tab's rows are in relation order, which the reader sets by
  // dragging. See tabIsRelationOrdered.
  const relationOrdered = computed(() => {
    const s = scope()
    return !!s && tabIsRelationOrdered(pageStore.pages[s.page], s.tab, schemaStore.relationTypes)
  })

  return { params, linkCreated, relationOrdered }
}

/**
 * The relation a row of `type` created on `tab` of an entity page is linked
 * over. The open tab's link wins. Otherwise the page's single link for the
 * type is used. When the page offers two different relations for the type,
 * none is chosen: picking one would be a guess about the author's intent.
 */
export function pageCreateLink(
  page: SidebarPage,
  tab: string,
  type: string
): SidebarPageLink | undefined {
  const forType = (links: SidebarPageLink[] | undefined) =>
    (links ?? []).filter((l) => l.type === type)
  const onTab = forType(page.tabs.find((t) => t.id === tab)?.links)
  if (onTab.length === 1) return onTab[0]
  if (onTab.length > 1) return undefined
  const distinct = new Map<string, SidebarPageLink>()
  for (const l of page.tabs.flatMap((t) => forType(t.links)))
    distinct.set(`${l.relation}/${l.direction}`, l)
  return distinct.size === 1 ? [...distinct.values()][0] : undefined
}

/**
 * Links a row created outside the page's tabs, from the space's Create menu,
 * to the entity page it was created from.
 *
 * `target` resolves the link when the create starts. The create dialog lives
 * outside the page and survives navigation, so the link is fixed then, not
 * when the row is saved. It is undefined when no entity page is open or the
 * page has no link for the type.
 *
 * `linkCreated` makes the link, then refreshes the type's lists: the row
 * already exists when the link lands, so its own refetch may have missed it.
 */
export function usePageCreateLink() {
  const pageStore = usePageStore()
  const queryCache = useQueryCache()

  function target(type: string): AnchorLink | undefined {
    const s = pageStore.current
    const page = s && pageStore.pages[s.page]
    return page && s ? anchorLink(page, s, pageCreateLink(page, s.tab, type)) : undefined
  }

  async function linkCreated(to: AnchorLink | undefined, entity: Entity): Promise<void> {
    if (!to || !(await linkToAnchor(to, entity))) return
    void queryCache.invalidateQueries({ key: entityKeys.list(entity.type) })
  }

  return { target, linkCreated }
}
