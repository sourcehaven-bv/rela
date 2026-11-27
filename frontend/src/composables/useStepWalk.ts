import { computed, onBeforeUnmount } from 'vue'
import { useQuery } from '@pinia/colada'
import { getEntityPosition, listEntities, type ScopeDescriptor } from '@/api/entities'
import { shouldDropHeldContent } from '@/api/errors'
import { useEvents } from '@/composables/useEvents'
import { useWorld } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores'
import { usePageStore } from '@/stores/pages'
import { listBaseParams, pageScopeParams } from '@/utils/listParams'
import { tabIsRelationOrdered } from '@/utils/relationOrder'
import type { ListParams, PageScope, SidebarPageTab, StepWalkState } from '@/types'

/** How long a burst of entity changes is left to settle before reading again. */
const EVENT_DEBOUNCE_MS = 500

/**
 * The scope descriptor of a list read: the same list, filters, sort and
 * narrowing, so the position walks exactly the rows the list shows. Built
 * from the one list-read builder, so the two cannot drift.
 */
function walkScope(entityType: string, params: ListParams): ScopeDescriptor {
  const scope: ScopeDescriptor = { source: 'list', type: entityType }
  const filters: Record<string, string> = {}
  for (const [key, value] of Object.entries(params)) {
    if (key.startsWith('filter[') && typeof value === 'string') filters[key] = value
  }
  if (Object.keys(filters).length) scope.filters = filters
  if (params.sort) scope.sort = params.sort
  if (params.query_scope) scope.query_scope = params.query_scope
  if (params.list_id) scope.list_id = params.list_id
  if (params.scope_page && params.scope_tab && params.anchor) {
    scope.scope_page = params.scope_page
    scope.scope_tab = params.scope_tab
    scope.anchor = params.anchor
  }
  return scope
}

/**
 * One step of a steps page that walks the rows of its list one at a time
 * (TKT-M07C97Y).
 *
 * The URL is the truth: `item()` is the row on screen, read from `?item=`.
 * Without one, the walk reads the list's first row, which the page puts in
 * the URL. With one, it asks `/_position` where the row sits among the
 * list's rows. Both reads go through the ACL-gated list pipeline, so a
 * hidden row is never shown, linked or counted.
 */
export function useStepWalk(opts: {
  page: () => string
  tab: () => SidebarPageTab | undefined
  /** The narrowing of an entity page's tab to its entity, if any. */
  scope: () => PageScope | undefined
  item: () => string | undefined
  enabled: () => boolean
}) {
  const schemaStore = useSchemaStore()
  const pageStore = usePageStore()
  const { worldParam } = useWorld()
  const { on } = useEvents()

  const listId = computed(() => opts.tab()?.target)
  const listConfig = computed(() => (listId.value ? schemaStore.getList(listId.value) : undefined))
  const walking = computed(() => opts.enabled() && !!opts.tab()?.walk && !!listConfig.value)

  // The list read the tab's list would make, narrowed to the page's entity.
  const params = computed((): ListParams | undefined => {
    const cfg = listConfig.value
    const tab = opts.tab()
    if (!cfg || !listId.value || !tab) return undefined
    const scope = opts.scope()
    const relationOrdered =
      !!scope &&
      tabIsRelationOrdered(pageStore.pages[opts.page()], tab.id, schemaStore.relationTypes)
    return { ...listBaseParams(listId.value, cfg, { relationOrdered }), ...pageScopeParams(scope) }
  })
  const entityType = computed(() => listConfig.value?.entity ?? '')

  const firstQuery = useQuery({
    key: () => [
      'step-walk-first',
      entityType.value,
      JSON.stringify(params.value),
      worldParam.value ?? '',
    ],
    query: async () => {
      const world = worldParam.value
      const res = await listEntities(entityType.value, {
        ...params.value,
        per_page: 1,
        ...(world ? { world } : {}),
      })
      return res.data[0]?.id ?? null
    },
    enabled: () => walking.value && !opts.item(),
  })

  const positionQuery = useQuery({
    key: () => [
      'step-walk-position',
      entityType.value,
      JSON.stringify(params.value),
      opts.item() ?? '',
      worldParam.value ?? '',
    ],
    query: () =>
      getEntityPosition(
        opts.item() as string,
        walkScope(entityType.value, params.value ?? {}),
        worldParam.value
      ),
    enabled: () => walking.value && !!opts.item(),
  })

  const state = computed((): StepWalkState => {
    if (!walking.value) return 'off'
    if (opts.item()) {
      // The position answers 404 for a row outside the list. Any other
      // failure keeps a position already held rather than blanking the step.
      const err = positionQuery.error.value
      if (err && shouldDropHeldContent(err)) return 'missing'
      if (positionQuery.data.value) return 'ready'
      return err ? 'error' : 'loading'
    }
    if (firstQuery.data.value === null) return 'empty'
    return firstQuery.error.value ? 'error' : 'loading'
  })

  // A write can add, remove or move a row, so the walk reads again.
  let timer: ReturnType<typeof setTimeout> | null = null
  on('entity:changed', () => {
    if (!walking.value) return
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => {
      if (opts.item()) void positionQuery.refetch()
      else void firstQuery.refetch()
    }, EVENT_DEBOUNCE_MS)
  })
  onBeforeUnmount(() => {
    if (timer) clearTimeout(timer)
  })

  return {
    state,
    entityType,
    /** The list's first row, read only while no `?item=` is set. */
    first: computed(() =>
      walking.value && !opts.item() ? (firstQuery.data.value ?? undefined) : undefined
    ),
    position: computed(() => (state.value === 'ready' ? positionQuery.data.value : undefined)),
  }
}
