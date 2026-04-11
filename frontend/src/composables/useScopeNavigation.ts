import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { toApiOperator, parseFilterQueryParams, filterStateToApiParams } from '@/utils/filters'
import type { ListParams } from '@/types'

export interface ScopeNav {
  backUrl: string
  prevId: string | null
  nextId: string | null
  prevType?: string
  nextType?: string
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

    // Search results scope: read stored result set from sessionStorage
    if (fromListId === 'search') {
      loadSearchScope()
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
        backUrl: `/list/${fromListId}`,
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

  function loadSearchScope() {
    try {
      const raw = sessionStorage.getItem('search-scope')
      if (!raw) { scopeNav.value = null; return }
      const data = JSON.parse(raw) as {
        ids: { type: string; id: string }[]
        backUrl: string
        label: string
      }
      const currentIndex = data.ids.findIndex(e => e.id === entityId() && e.type === entityType())
      if (currentIndex === -1) { scopeNav.value = null; return }

      const prev = currentIndex > 0 ? data.ids[currentIndex - 1] : null
      const next = currentIndex < data.ids.length - 1 ? data.ids[currentIndex + 1] : null
      scopeNav.value = {
        backUrl: data.backUrl,
        prevId: prev?.id ?? null,
        nextId: next?.id ?? null,
        prevType: prev?.type,
        nextType: next?.type,
        current: currentIndex + 1,
        total: data.ids.length,
        label: data.label,
      }
    } catch {
      scopeNav.value = null
    }
  }

  function navigateScope(direction: 'prev' | 'next') {
    if (!scopeNav.value) return

    const targetId = direction === 'prev' ? scopeNav.value.prevId : scopeNav.value.nextId
    if (!targetId) return

    // Use type from scope if available (search results can mix entity types)
    const targetType = direction === 'prev'
      ? scopeNav.value.prevType ?? entityType()
      : scopeNav.value.nextType ?? entityType()

    // Preserve all query params for consistent navigation
    router.push({
      path: `/entity/${targetType}/${targetId}`,
      query: route.query,
    })
  }

  function goBack() {
    if (scopeNav.value) {
      router.push(scopeNav.value.backUrl)
    }
  }

  return {
    scopeNav,
    loadScopeNav,
    navigateScope,
    goBack,
  }
}
