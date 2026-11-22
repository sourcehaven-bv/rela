/**
 * The reader's sort, kept in the URL as `?sort=a,-b` (a ascending, b
 * descending), the form the list endpoint takes. A sorted board or chart is
 * then a link, and survives a refresh, as a search in `?q=` does.
 *
 * `allowed` names the properties the view can sort on. A key outside it is
 * left out of the sort rather than sent: a link written for another board, or
 * for a config that has since changed, must not turn into a refused read.
 *
 * The view owns its own state. `set` writes it and the URL together, and a
 * route change that did not come from `set` (back, forward, a pasted link)
 * reads the URL again.
 */
import { computed, ref, watch, type WritableComputedRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { sortParam } from '@/utils/listParams'
import type { SortSpec } from '@/types'

/** Parse `a,-b` into specs. Anything that is not a string reads as no sort. */
export function parseSortQuery(raw: unknown): SortSpec[] {
  const value = Array.isArray(raw) ? raw[raw.length - 1] : raw
  if (typeof value !== 'string' || value === '') return []
  const specs: SortSpec[] = []
  for (const part of value.split(',')) {
    const desc = part.startsWith('-')
    const property = desc ? part.slice(1) : part
    if (property && !specs.some((s) => s.property === property)) {
      specs.push({ property, direction: desc ? 'desc' : 'asc' })
    }
  }
  return specs
}

export function useUrlSort(allowed: () => Set<string> | undefined): WritableComputedRef<SortSpec[]> {
  const route = useRoute()
  const router = useRouter()
  const raw = ref<SortSpec[]>(parseSortQuery(route.query.sort))

  watch(
    () => route.query.sort,
    (value) => {
      const parsed = parseSortQuery(value)
      if (sortParam(parsed) !== sortParam(raw.value)) raw.value = parsed
    },
  )

  return computed<SortSpec[]>({
    get: () => {
      const names = allowed()
      return names ? raw.value.filter((s) => names.has(s.property)) : raw.value
    },
    set: (next) => {
      raw.value = next
      const query = { ...route.query }
      const param = sortParam(next)
      if (param) query.sort = param
      else delete query.sort
      void router.replace({ query })
    },
  })
}
