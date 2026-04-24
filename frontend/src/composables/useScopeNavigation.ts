import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { toApiOperator, parseFilterQueryParams, filterStateToApiParams } from '@/utils/filters'
import type { ListParams } from '@/types'

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
  const entitiesStore = useEntitiesStore()

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
      // Build query params matching what EntityList uses
      const params: ListParams = {
        per_page: 1000, // Fetch all to get accurate position
      }

      // Add sort from query params or list default
      const sort = route.query.sort as string | undefined
      if (sort) {
        params.sort = sort
      } else if (listConfig.default_sort?.length) {
        params.sort = listConfig.default_sort
          .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
          .join(',')
      }

      // Add pre-configured filters from list config
      for (const filter of listConfig.filters || []) {
        if (filter.operator && filter.value) {
          const apiOp = toApiOperator(filter.operator)
          params[`filter[${filter.property}][${apiOp}]`] = filter.value
        }
      }

      // Add user-selected filters from query (bracket format `filter[prop][op]`).
      // We re-serialize via the shared filterStateToApiParams helper so the
      // backend gets identical params to what EntityList sends.
      const userFilters = parseFilterQueryParams(route.query)
      const userParams = filterStateToApiParams(userFilters)
      const paramsRecord = params as Record<string, string | number | undefined>
      for (const [key, value] of Object.entries(userParams)) {
        paramsRecord[key] = value
      }

      const result = await entitiesStore.fetchList(listConfig.entity, params)
      const ids = result.data.map((e) => e.id)
      const currentIndex = ids.indexOf(entityId())

      if (currentIndex === -1) {
        scopeNav.value = null
        return
      }

      scopeNav.value = {
        prevId: currentIndex > 0 ? ids[currentIndex - 1] : null,
        nextId: currentIndex < ids.length - 1 ? ids[currentIndex + 1] : null,
        current: currentIndex + 1,
        total: ids.length,
        label: listConfig.title || fromListId,
      }
    } catch {
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
