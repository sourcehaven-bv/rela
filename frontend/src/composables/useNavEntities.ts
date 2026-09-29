import { computed, onBeforeUnmount, type Ref } from 'vue'
import { useQuery } from '@pinia/colada'
import { listEntities } from '@/api'
import { shouldDropHeldContent } from '@/api/errors'
import { useEvents } from '@/composables/useEvents'
import { useWorld } from '@/composables/useWorld'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { entityDetailHref } from '@/utils/entityRoute'
import type { ListParams, SidebarEntities, SidebarGroup } from '@/types'

/** How long a burst of entity changes is left to settle before refetching. */
const EVENT_DEBOUNCE_MS = 1000

/**
 * The list endpoint's page maximum. There is deliberately no configurable
 * limit; one page is the bound.
 */
const SIDEBAR_ENTITIES_PAGE = 100

/** One entity link under an `entities:` entry. */
export interface NavEntityRow {
  title: string
  href: string
}

/** Resolves an `entities:` entry to its rows, if they have loaded. */
export type NavEntitiesLookup = (entities: SidebarEntities) => NavEntityRow[] | undefined

/** The cache key for one entry: its whole query definition. */
export function navEntitiesKey(entities: SidebarEntities): string {
  return `${entities.type}|${entities.query_scope ?? ''}|${entities.sort ?? ''}`
}

/**
 * The rows behind the sidebar's `entities:` entries (TKT-PEKL8L).
 *
 * The rows come from the ordinary list endpoint, not from the sidebar
 * payload. That endpoint already applies the ACL, the world, faces and the
 * query scope; the sidebar only carries the query definition, so it stays
 * free of per-principal data.
 *
 * All entries are fetched as one query and refetched after an entity change
 * or a config reload. An entry whose fetch fails shows no rows; a refused
 * fetch drops rows already on screen, because they are entity titles the
 * principal may no longer read (see shouldDropHeldContent).
 */
export function useNavEntities(groups: Ref<SidebarGroup[]>) {
  const { worldParam } = useWorld()
  const { on } = useEvents()

  const entries = computed(() => {
    const byKey = new Map<string, SidebarEntities>()
    for (const group of groups.value) {
      for (const item of group.items) {
        if (item.entities) byKey.set(navEntitiesKey(item.entities), item.entities)
      }
    }
    return [...byKey.values()]
  })

  function paramsFor(entities: SidebarEntities): ListParams {
    const p: ListParams = { per_page: SIDEBAR_ENTITIES_PAGE }
    if (entities.query_scope) p.query_scope = entities.query_scope
    if (entities.sort) p.sort = entities.sort
    if (worldParam.value) p.world = worldParam.value
    return p
  }

  const query = useQuery({
    key: () => ['nav-entities', worldParam.value ?? '', ...entries.value.map(navEntitiesKey)],
    query: async ({ signal }) => {
      const settled = await Promise.allSettled(
        entries.value.map((e) => listEntities(e.type, paramsFor(e), signal))
      )
      const rows = new Map<string, NavEntityRow[]>()
      settled.forEach((result, i) => {
        const entities = entries.value[i]
        if (result.status === 'rejected') {
          console.error('Failed to load sidebar entities:', entities, result.reason)
          return
        }
        rows.set(
          navEntitiesKey(entities),
          result.value.data.map((e) => ({
            title: entityDisplayTitle(e),
            href: entityDetailHref({ id: e.id, type: e.type || entities.type }),
          }))
        )
      })
      return rows
    },
    enabled: () => entries.value.length > 0,
  })

  function refetch() {
    if (entries.value.length > 0) void query.refetch()
  }

  let timer: ReturnType<typeof setTimeout> | null = null
  on('entity:changed', () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(refetch, EVENT_DEBOUNCE_MS)
  })
  on('refresh', refetch)
  onBeforeUnmount(() => {
    if (timer) clearTimeout(timer)
  })

  const rowsFor: NavEntitiesLookup = (entities) => {
    if (shouldDropHeldContent(query.error.value)) return undefined
    return query.data.value?.get(navEntitiesKey(entities))
  }

  return { rowsFor }
}
