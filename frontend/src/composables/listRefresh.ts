import type { ListConfig } from '@/types/config'

/**
 * The entity types of the lists that must refetch when entities of
 * `changed` change, other than `changed` itself: a list's roll-ups read the
 * types the server publishes in `refresh_types` (design decision Q8), so a
 * change to a tag refreshes a project list whose roll-up walks to tags.
 * Sorted, without duplicates.
 */
export function listTypesRefreshedBy(lists: Iterable<ListConfig>, changed: string): string[] {
  const out = new Set<string>()
  for (const list of lists) {
    if (list.entity !== changed && list.refresh_types?.includes(changed)) {
      out.add(list.entity)
    }
  }
  return [...out].sort()
}
