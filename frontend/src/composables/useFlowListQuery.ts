import { computed, type ComputedRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/**
 * The filters and page of a flow list view, kept in the query string so a
 * link or a reload keeps them (TKT-ZAXRXND).
 */
export function useFlowListQuery(perPage: number) {
  const route = useRoute()
  const router = useRouter()

  function param(name: string): ComputedRef<string> {
    return computed(() => {
      const v = route.query[name]
      return typeof v === 'string' ? v : ''
    })
  }

  const rawPage = param('page')
  const page = computed(() => Math.max(1, Number.parseInt(rawPage.value, 10) || 1))

  /** Merges `patch` into the query; an empty or undefined value removes the key. */
  function setQuery(patch: Record<string, string | undefined>) {
    const next: Record<string, string> = {}
    for (const [k, v] of Object.entries({ ...route.query, ...patch })) {
      if (typeof v === 'string' && v !== '') next[k] = v
    }
    void router.replace({ query: next })
  }

  /** Sets a filter and goes back to the first page. */
  function setFilter(name: string, value: string) {
    setQuery({ [name]: value, page: undefined })
  }

  function setPage(n: number) {
    setQuery({ page: n > 1 ? String(n) : undefined })
  }

  function pageCount(total: number | undefined): number {
    return Math.max(1, Math.ceil((total ?? 0) / perPage))
  }

  return { param, page, setFilter, setPage, pageCount }
}
