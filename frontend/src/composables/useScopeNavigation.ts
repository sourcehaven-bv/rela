import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSchemaStore } from '@/stores'
import { toApiOperator, parseFilterQueryParams, filterStateToApiParams } from '@/utils/filters'
import { getEntityPosition, type ScopeDescriptor } from '@/api/entities'

export interface ScopeNav {
  prevId: string | null
  nextId: string | null
  current: number
  total: number
  label: string
}

/**
 * Composable for navigating between entities in a list context.
 * Preserves list filters and sorting while moving through items.
 */
export function useScopeNavigation(entityType: () => string, entityId: () => string) {
  const route = useRoute()
  const router = useRouter()
  const schemaStore = useSchemaStore()

  const scopeNav = ref<ScopeNav | null>(null)

  async function loadScopeNav() {
    const from = route.query.from as string | undefined
    if (!from) {
      scopeNav.value = null
      return
    }

    // Two scope origins share the `?from=` mechanism: the search view
    // (`from=search`) and any configured list (`from=<listId>`). Each builds a
    // ScopeDescriptor the server resolves into a position — see #844 and the
    // backend scope.go. The descriptor is the single extension point: adding a
    // new origin means adding a branch here plus a `source` on the backend.
    const built = from === 'search' ? buildSearchScope() : buildListScope(from)
    if (!built) {
      scopeNav.value = null
      return
    }

    try {
      const pos = await getEntityPosition(entityId(), built.scope)
      scopeNav.value = {
        prevId: pos.prev,
        nextId: pos.next,
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

    // Sort from query params or list default.
    const sortParam = route.query.sort as string | undefined
    let sort = sortParam
    if (!sort && listConfig.default_sort?.length) {
      sort = listConfig.default_sort
        .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
        .join(',')
    }

    // Free-text search applied within the list, if any. Including it here is
    // what lets list scope navigation honor an active ?q= filter — the prior
    // implementation ignored q (known limitation, now resolved).
    const q = route.query.q as string | undefined

    const scope: ScopeDescriptor = {
      source: q ? 'search' : 'list',
      type: listConfig.entity,
    }
    if (Object.keys(filters).length) scope.filters = filters
    if (sort) scope.sort = sort
    if (q) scope.q = q

    return { scope, label: listConfig.title || listId }
  }

  function navigateScope(direction: 'prev' | 'next') {
    if (!scopeNav.value) return

    const targetId = direction === 'prev' ? scopeNav.value.prevId : scopeNav.value.nextId
    if (!targetId) return

    // Preserve all query params for consistent navigation
    router.push({
      path: `/entity/${entityType()}/${targetId}`,
      query: route.query,
    })
  }

  return {
    scopeNav,
    loadScopeNav,
    navigateScope,
  }
}
