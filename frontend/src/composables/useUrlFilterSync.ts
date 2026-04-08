/**
 * Bidirectional sync between Vue Router query params and a FilterState ref.
 *
 * - On setup, seeds the filter state from the current URL synchronously (so the
 *   first list fetch already includes URL-supplied filters — no second fetch).
 * - `writeToQuery` is the only way callers should mutate the state; it updates
 *   the URL via `router.replace` (no history entry per keystroke) and records a
 *   signature so the route watcher can ignore the echo.
 * - The route watcher reacts to external navigation (back/forward, deep links)
 *   and re-reads from the query when the change isn't our own write. The
 *   signature comparison is self-healing: even if a write fails or the user's
 *   change collides with a static filter, the next external nav resets it.
 * - Static filters (from `data-entry.yaml` `filters:`) take precedence over URL
 *   filters; collisions log a warning and the URL filter is dropped (silent
 *   zero-result traps are worse than a console warning).
 */
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  parseFilterQueryParams,
  buildQueryWithFilters,
  stringifyFilterQuery,
} from '@/utils/filters'
import type { FilterState } from '@/types/filters'

export interface UseUrlFilterSyncOptions {
  /**
   * Properties already pinned by static `filters:` config. URL filters for
   * these properties are ignored (with a console warning) so users can't
   * silently override the list's intended scope.
   */
  staticFilterProperties: () => Set<string>
}

export function useUrlFilterSync(opts: UseUrlFilterSyncOptions) {
  const route = useRoute()
  const router = useRouter()
  const filters = ref<FilterState>({})

  // Signature of the most recent query we wrote ourselves. Used to ignore the
  // immediate route watcher echo from our own router.replace call.
  let lastWrittenSig = ''

  function readFromQuery() {
    const fromUrl = parseFilterQueryParams(route.query)
    const blocked = opts.staticFilterProperties()
    for (const prop of Object.keys(fromUrl)) {
      if (blocked.has(prop)) {
        // The collision check locks the WHOLE property, not a specific
        // operator — a static `filter[date][gte]` in data-entry.yaml blocks
        // any URL filter on `date`, including `[lte]`. If you need range
        // filters combined with static scope, define both bounds in
        // data-entry.yaml rather than via URL.
        console.warn(
          `useUrlFilterSync: URL filter for "${prop}" ignored ` +
            `(property is locked by a static \`filters:\` entry in data-entry.yaml; ` +
            `static filters lock the whole property, not individual operators)`,
        )
        delete fromUrl[prop]
      }
    }
    filters.value = fromUrl
  }

  // Seed synchronously so the caller's first fetch sees URL filters.
  readFromQuery()

  function writeToQuery(newFilters: FilterState) {
    filters.value = newFilters
    const newQuery = buildQueryWithFilters(route.query, newFilters)
    lastWrittenSig = stringifyFilterQuery(newQuery)
    router.replace({ query: newQuery })
  }

  // React to external navigation (back/forward, deep links). Self-writes are
  // detected via signature comparison and skipped.
  watch(
    () => route.query,
    (q) => {
      if (stringifyFilterQuery(q) === lastWrittenSig) return
      readFromQuery()
    },
  )

  return { filters, writeToQuery, readFromQuery }
}
