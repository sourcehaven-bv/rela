import { ref } from 'vue'
import { useRoute, useRouter, type RouteLocationRaw } from 'vue-router'
import { useSchemaStore } from '@/stores'
import { toApiOperator, parseFilterQueryParams, filterStateToApiParams } from '@/utils/filters'
import { getEntityPosition, type ScopeDescriptor, type PositionRef } from '@/api/entities'
import { readFromPage } from '@/utils/pageContext'
import { usePageStore } from '@/stores/pages'
import { defaultSortParam } from '@/utils/listParams'
import { tabIsRelationOrdered } from '@/utils/relationOrder'
import { listPiles } from '@/api/piles'
import { knownPileName, rememberPileNames } from '@/composables/pileNames'
import { useWorld } from '@/composables/useWorld'

export interface ScopeNav {
  // Neighbours carry their type, not just id, so navigation builds the correct
  // /entity/<type>/<id> route even when a (search) scope spans entity types.
  prev: PositionRef | null
  next: PositionRef | null
  current: number
  total: number
  label: string
}

/**
 * Composable for navigating between entities in a list context.
 * Preserves list filters and sorting while moving through items.
 *
 * `servedAddress` is the row on screen (`ID@face`), for a pile scope: a pile
 * holds a face, so the position is looked up by address rather than by the
 * bare `entityId` the list and search scopes walk.
 */
export function useScopeNavigation(entityId: () => string, servedAddress?: () => string | null) {
  const route = useRoute()
  const router = useRouter()
  const schemaStore = useSchemaStore()
  const pageStore = usePageStore()
  // The page's world: the position must be computed over the same set the
  // page (list, search results, pile panel) showed.
  const { worldParam } = useWorld()

  const scopeNav = ref<ScopeNav | null>(null)

  async function loadScopeNav() {
    const from = route.query.from as string | undefined
    if (!from) {
      scopeNav.value = null
      return
    }

    // Three scope origins share the `?from=` mechanism: the search view
    // (`from=search`), a pile (`from=pile&pile=<id>`) and any configured list
    // (`from=<listId>`). Each builds a ScopeDescriptor the server resolves
    // into a position — see #844 and the backend scope.go. The descriptor is
    // the single extension point: adding a new origin means adding a branch
    // here plus a `source` on the backend.
    const built =
      from === 'search' ? buildSearchScope() : from === 'pile' ? await buildPileScope() : buildListScope(from)
    if (!built) {
      scopeNav.value = null
      return
    }

    const id = built.scope.source === 'pile' ? (servedAddress?.() ?? entityId()) : entityId()

    try {
      const pos = await getEntityPosition(id, built.scope, worldParam.value)
      scopeNav.value = {
        prev: pos.prev,
        next: pos.next,
        current: pos.current,
        total: pos.total,
        label: built.label,
      }
    } catch {
      // 404 (entity not in scope) or any error → no scope nav, same as before.
      scopeNav.value = null
    }
  }

  // buildSearchScope mirrors what SearchView showed: a free-text query (q) over
  // possibly-mixed entity types, optionally narrowed to a single type chip. The
  // server resolves position within that relevance-ordered set, so prev/next
  // can cross entity types.
  function buildSearchScope(): { scope: ScopeDescriptor; label: string } | null {
    const q = route.query.q as string | undefined
    if (!q) return null
    const scope: ScopeDescriptor = { source: 'search', q }
    const type = route.query.type as string | undefined
    if (type) scope.type = type
    return { scope, label: `Search: ${q}` }
  }

  // buildPileScope walks one of the user's piles. The descriptor carries the
  // pile id and nothing else; the server refuses any other field. The label
  // is the pile's name, from the sidebar's listing when it has loaded, else
  // from one fetch of the list.
  async function buildPileScope(): Promise<{ scope: ScopeDescriptor; label: string } | null> {
    const pile = route.query.pile
    if (typeof pile !== 'string' || !pile) return null
    let name = knownPileName(pile)
    if (name === undefined) {
      try {
        rememberPileNames((await listPiles(worldParam.value)).piles)
        name = knownPileName(pile)
      } catch {
        // The position request decides whether the scope exists; a missing
        // name only costs the label.
      }
    }
    return { scope: { source: 'pile', pile }, label: name ?? 'Pile' }
  }

  // buildListScope reconstructs the scope EntityList rendered: list-config
  // filters + user filters + sort (+ any active q), so the navigator observes
  // the same ordered set as the list. Returns null when the list config is
  // missing.
  function buildListScope(listId: string): { scope: ScopeDescriptor; label: string } | null {
    const listConfig = schemaStore.getList(listId)
    if (!listConfig) return null

    const filters: Record<string, string> = {}

    // Pre-configured filters from list config.
    for (const filter of listConfig.filters || []) {
      if (filter.operator && filter.value) {
        const apiOp = toApiOperator(filter.operator)
        filters[`filter[${filter.property}][${apiOp}]`] = filter.value
      }
    }

    // User-selected filters from query (bracket format `filter[prop][op]`).
    // We re-serialize via the shared filterStateToApiParams helper so the
    // backend gets identical params to what EntityList sends.
    const userFilters = parseFilterQueryParams(route.query)
    for (const [key, value] of Object.entries(filterStateToApiParams(userFilters))) {
      filters[key] = value
    }

    // Sort from query params or list default. A tab in relation order has no
    // default: the server walks the relation order when the scope has no sort.
    const tab = readFromPage(route.query)
    const relationOrdered =
      !!tab?.entity && tabIsRelationOrdered(pageStore.pages[tab.page], tab.tab, schemaStore.relationTypes)
    const sort = (route.query.sort as string | undefined) || defaultSortParam(listConfig, relationOrdered)

    // Free-text search applied within the list, if any. Including it here is
    // what lets list scope navigation honor an active ?q= filter — the prior
    // implementation ignored q (known limitation, now resolved).
    const q = route.query.q as string | undefined

    // A list opened from an entity-page tab showed only the anchor's rows.
    // The list pipeline applies `q` as well, so such a scope stays a list
    // scope: the search pipeline cannot narrow to the tab.
    const scope: ScopeDescriptor = {
      source: q && !tab?.entity ? 'search' : 'list',
      type: listConfig.entity,
    }
    if (tab?.entity) {
      scope.scope_page = tab.page
      scope.scope_tab = tab.tab
      scope.anchor = tab.entity
    }
    if (Object.keys(filters).length) scope.filters = filters
    if (sort) scope.sort = sort
    if (q) scope.q = q
    // The list's configured scope, for the same reason the filters above are
    // carried: the descriptor rebuilds the list's query server-side, so a
    // field it omits is a field the position silently drops.
    if (listConfig.query_scope) scope.query_scope = listConfig.query_scope

    return { scope, label: listConfig.title || listId }
  }

  // scopeTarget is the single source of truth for where prev/next goes — bound
  // to the nav links' `to` AND used by navigateScope, so a cmd-clicked tab and
  // the keyboard shortcut land on the identical URL.
  //
  // Use the target's OWN type, not the current entity's — a search scope can
  // span types, so the next/prev entity may be a different type. Preserve all
  // query params so the scope (from=, q=, …) survives the hop.
  //
  // Nil: returns undefined when there is no scope or no neighbour in that
  // direction; callers render a disabled affordance rather than a dead link.
  function scopeTarget(direction: 'prev' | 'next'): RouteLocationRaw | undefined {
    if (!scopeNav.value) return undefined

    const target = direction === 'prev' ? scopeNav.value.prev : scopeNav.value.next
    if (!target) return undefined

    // A pile scope holds faces, so its neighbours are addressed by their
    // address (`ID@face`); the other scopes walk entities by id.
    const isPile = route.query.from === 'pile'
    return {
      path: `/entity/${target.type}/${isPile ? (target.address ?? target.id) : target.id}`,
      query: route.query,
    }
  }

  function navigateScope(direction: 'prev' | 'next') {
    const target = scopeTarget(direction)
    if (!target) return
    router.push(target)
  }

  return {
    scopeNav,
    loadScopeNav,
    scopeTarget,
    navigateScope,
  }
}
