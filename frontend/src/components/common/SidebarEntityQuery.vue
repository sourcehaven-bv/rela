<script setup lang="ts">
/**
 * SidebarEntityQuery runs the list query behind an `entities:` navigation
 * entry (TKT-PEKL8L) and hands the rows to its slot. It renders no markup of
 * its own: the links are drawn by Sidebar.vue, so they share its scoped
 * `.nav-item` styles instead of copying them.
 *
 * The rows come from the ordinary list endpoint, not from the sidebar
 * payload. That endpoint already applies the ACL, the world, faces and the
 * query scope; the sidebar only carries the query definition, so it stays
 * free of per-principal data.
 *
 * Refresh needs no code here. The query is keyed under
 * `entityKeys.listParams(type, …)`, and useEvents invalidates
 * `entityKeys.type(type)` on `entity:changed` and the root key on `refresh`,
 * so a write to this type refetches it. Two entries (or an open list) with
 * the same params share one cache entry and one request.
 */
import { computed, watch } from 'vue'
import { useQuery } from '@pinia/colada'
import type { RouteLocationRaw } from 'vue-router'
import { listEntities } from '@/api'
import { shouldDropHeldContent } from '@/api/errors'
import { entityKeys } from '@/queries/entities'
import { useWorld } from '@/composables/useWorld'
import { entityRef } from '@/utils/entityRef'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { entityDetailHref } from '@/utils/entityRoute'
import type { ListParams, SidebarEntities } from '@/types'

/**
 * The list endpoint's page maximum. There is deliberately no configurable
 * limit; one page is the bound, and the rest is reported as `overflow`.
 */
const SIDEBAR_ENTITIES_PAGE = 100

export interface SidebarEntityRow {
  /** Face-aware address, stable across worlds; used as the v-for key. */
  key: string
  title: string
  path: string
  to: RouteLocationRaw
}

const props = defineProps<{ entities: SidebarEntities }>()

const emit = defineEmits<{
  /**
   * Fired when a fetch settles: true when there is something to draw, rows
   * or a failure notice. Not fired while the first fetch is pending.
   */
  (e: 'shown', shown: boolean): void
}>()

const { worldParam } = useWorld()

const params = computed<ListParams>(() => {
  const p: ListParams = { per_page: SIDEBAR_ENTITIES_PAGE }
  if (props.entities.query_scope) p.query_scope = props.entities.query_scope
  if (props.entities.sort) p.sort = props.entities.sort
  if (worldParam.value) p.world = worldParam.value
  return p
})

const query = useQuery({
  key: () => entityKeys.listParams(props.entities.type, params.value),
  query: ({ signal }) => listEntities(props.entities.type, params.value, signal),
  placeholderData: (prev) => prev,
})

/**
 * True when the last refetch was refused (401/403/404). Colada keeps the last
 * successful data across an error, so without this a revoked session would
 * leave entity titles in the sidebar on every route (frontend/CLAUDE.md,
 * "Held content is read-side ACL's blind spot").
 */
const denied = computed(
  () => query.status.value === 'error' && shouldDropHeldContent(query.error.value)
)

const rows = computed<SidebarEntityRow[]>(() => {
  if (denied.value) return []
  const data = query.data.value?.data ?? []
  return data.map((e) => {
    const path = entityDetailHref({ id: e.id, type: e.type || props.entities.type })
    // The world rides along so the link opens the face this row showed, the
    // same rule EntityList's row links follow (TKT-6NCSSC).
    const to: RouteLocationRaw = worldParam.value
      ? { path, query: { world: worldParam.value } }
      : { path }
    return { key: entityRef(e), title: entityDisplayTitle(e), path, to }
  })
})

/** Matching rows beyond the one page served. */
const overflow = computed(() => {
  if (denied.value) return 0
  const total = query.data.value?.meta?.total ?? 0
  return Math.max(0, total - rows.value.length)
})

/**
 * True when there is nothing trustworthy to show after a failed fetch. A
 * failure must never read as "nothing matched", so the slot gets an explicit
 * flag instead of an empty row list. A refusal drops held rows; any other
 * failure keeps the rows from the last success, since it is transient.
 */
const failed = computed(() => query.status.value === 'error' && (denied.value || !query.data.value))

watch(
  () => query.status.value,
  (status) => {
    if (status === 'error') {
      console.error('Failed to load sidebar entities:', props.entities, query.error.value)
    }
  },
  { immediate: true }
)
watch(
  () => [query.status.value, rows.value.length, failed.value] as const,
  ([status, n, fail]) => {
    if (status !== 'pending') emit('shown', n > 0 || fail)
  },
  { immediate: true }
)
</script>

<template>
  <slot :rows="rows" :overflow="overflow" :failed="failed" />
</template>
