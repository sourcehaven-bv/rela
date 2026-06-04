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
    const fromListId = route.query.from as string | undefined
    if (!fromListId) {
      scopeNav.value = null
      return
    }

    const listConfig = schemaStore.getList(fromListId)
    if (!listConfig) {
      scopeNav.value = null
      return
    }

    try {
      // Build the scope descriptor matching what EntityList renders. The
      // server runs the same filter/sort pipeline and returns the position
      // directly — no fetch-all, so it is correct past the pagination cap
      // that previously truncated this set to 25 (#844).
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

      // Free-text search from the originating list, if any. Including it here
      // is what lets scope navigation honor an active ?q= search — the prior
      // implementation ignored q (known limitation, now resolved).
      const q = route.query.q as string | undefined

      const scope: ScopeDescriptor = {
        source: q ? 'search' : 'list',
        type: listConfig.entity,
      }
      if (Object.keys(filters).length) scope.filters = filters
      if (sort) scope.sort = sort
      if (q) scope.q = q

      const pos = await getEntityPosition(entityId(), scope)

      scopeNav.value = {
        prevId: pos.prev,
        nextId: pos.next,
        current: pos.current,
        total: pos.total,
        label: listConfig.title || fromListId,
      }
    } catch {
      // 404 (entity not in scope) or any error → no scope nav, same as before.
      scopeNav.value = null
    }
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
