import type { EntryKey } from '@pinia/colada'
import type { ListConfig } from '@/types/config'

/**
 * The lists that must refetch when entities of `changed` change, other than
 * lists of `changed` itself: the roll-ups a list reads read the types the
 * server publishes in its `refresh_types` (design decision Q8), so a change
 * to a tag refreshes a project list whose roll-up walks to tags. Keyed by
 * list id, mapped to the list's entity type.
 */
export function listsRefreshedBy(
  lists: Iterable<[string, ListConfig]>,
  changed: string
): Map<string, string> {
  const out = new Map<string, string>()
  for (const [id, list] of lists) {
    if (list.entity !== changed && list.refresh_types?.includes(changed)) {
      out.set(id, list.entity)
    }
  }
  return out
}

/**
 * Whether key is a list query of one of the lists in ids: a key under
 * `entityKeys.list(type)` whose params name the list in `list_id`. Only
 * that list's queries refetch, not every list of its type, so a page, a
 * kanban or a nav count over the same type is left alone.
 */
export function isListQueryOf(key: EntryKey, ids: ReadonlyMap<string, string>): boolean {
  if (key.length < 4 || key[0] !== 'entities' || key[2] !== 'list' || typeof key[3] !== 'string') {
    return false
  }
  const type = key[1]
  const listId = listIdOf(key[3])
  return listId !== undefined && ids.get(listId) === type
}

// The list_id in a canonicalListParams string, which is a JSON array of
// [name, value] pairs, or '' when the query has no params.
function listIdOf(canonical: string): string | undefined {
  if (canonical === '') return undefined
  try {
    const pairs = JSON.parse(canonical) as unknown
    if (!Array.isArray(pairs)) return undefined
    for (const pair of pairs) {
      if (Array.isArray(pair) && pair[0] === 'list_id' && typeof pair[1] === 'string') {
        return pair[1]
      }
    }
  } catch {
    return undefined
  }
  return undefined
}
