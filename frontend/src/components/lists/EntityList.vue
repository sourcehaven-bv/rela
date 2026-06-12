<script setup lang="ts">
import { ref, shallowRef, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useMutation, useQueryCache } from '@pinia/colada'
import { useSchemaStore, useUIStore } from '@/stores'
import { useListKeyboard } from '@/composables/useListKeyboard'
import { useListSelection } from '@/composables/useListSelection'
import { useListActions } from '@/composables/useListActions'
import { useUrlFilterSync } from '@/composables/useUrlFilterSync'
import { listEntities, deleteEntity, getErrorMessage } from '@/api'
import { entityKeys } from '@/queries/entities'
import { beginOptimisticRemove, rollbackOptimistic } from '@/queries/optimisticList'
import { toApiOperator, filterStateToApiParams } from '@/utils/filters'
import { entityDetailHref } from '@/utils/entityRoute'
import { actionAllowed } from '@/utils/affordancesWarning'
import { getCellValue, formatCellValue, isEnumPropertyDef, asArray } from '@/utils/format'
import type { Entity, ListMeta, ListParams, ListResponse, FilterState } from '@/types'
import FilterBar from './FilterBar.vue'
import Pagination from './Pagination.vue'
import SearchBox from './SearchBox.vue'
import AdHocFilterMenu from './AdHocFilterMenu.vue'
import Badge from '@/components/common/Badge.vue'
import BackButton from '@/components/common/BackButton.vue'
import { useBackTarget } from '@/composables/useBackTarget'
import { useConfirm } from '@/composables/useConfirm'

const props = defineProps<{
  listId: string
}>()

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { confirm } = useConfirm()

// Back affordance — renders when ?return_to= or ?from= is present. See TKT-JIEKC.
const backTarget = useBackTarget()

// Responsive: detect mobile for card vs table layout
const mobileQuery = typeof window !== 'undefined' ? window.matchMedia('(max-width: 768px)') : null
const isMobile = ref(mobileQuery?.matches ?? false)
function onMediaChange(e: MediaQueryListEvent) { isMobile.value = e.matches }
onMounted(() => { mobileQuery?.addEventListener('change', onMediaChange) })
onUnmounted(() => { mobileQuery?.removeEventListener('change', onMediaChange) })

// State
//
// Page is the only piece of list state the client owns directly; it feeds
// the query key (input). Everything else below is derived from the query
// result (output). Splitting page-as-input from meta-as-output removes the
// read/write cycle the old single `meta.page` had.
//
// The actual useQuery() call lives lower (after queryParams is defined,
// since its `enabled` is evaluated synchronously during setup) and assigns
// into this holder. The derived computeds here read it lazily, so they can
// be declared up top where useListActions / useListKeyboard need them.
const page = ref(1)
const queryCache = useQueryCache()

type ListQuery = ReturnType<typeof useQuery<ListResponse<Entity>>>
const listQueryRef = shallowRef<ListQuery>()

const entities = computed<Entity[]>(() => listQueryRef.value?.data.value?.data ?? [])
const meta = computed<ListMeta>(
  () =>
    listQueryRef.value?.data.value?.meta ?? {
      total: 0,
      page: page.value,
      per_page: listConfig.value?.page_size || 25,
      has_more: false,
    }
)
// Spinner only on the first load (no data yet); background refetches keep
// the current rows via placeholderData.
const loading = computed(() => listQueryRef.value?.isPending.value ?? true)
const loadError = computed(() => {
  const err = listQueryRef.value?.error.value
  return err ? getErrorMessage(err, 'Failed to load entities') : null
})
const includedEntities = computed<Record<string, Entity>>(
  () => listQueryRef.value?.data.value?.included ?? {}
)
// Collection-scope verb verdicts (e.g. {create: true|false}). Always
// emitted by the data-entry server; absent only for non-data-entry
// callers, in which case affordances render defensively (the server
// still 403s on click). See `_actions` in api-reference.md.
const collectionActions = computed<Record<string, boolean> | undefined>(
  () => listQueryRef.value?.data.value?._actions
)

// Affordance gates: `_actions` map from the server. `false` → hide;
// anything else → render. Helper keeps the contract DRY across
// components; see frontend/src/utils/affordancesWarning.ts.
function canCreate(): boolean {
  return actionAllowed({ _actions: collectionActions.value }, 'create')
}
function canDelete(entity: Entity): boolean {
  return actionAllowed(entity, 'delete')
}
function canUpdate(entity: Entity): boolean {
  return actionAllowed(entity, 'update')
}
// Bulk-action visibility: an action shows iff at least one selected
// entity permits the underlying `update` write. (All bulk actions
// today reduce to `update` at the entity level; transition / relation
// verbs are deferred to phase 3.) Returns true when nothing is
// selected (the bar isn't visible anyway) or when no `_actions` data
// is loaded yet (defensive fallback).
function anySelectedAllowsUpdate(): boolean {
  if (selectedIds.value.size === 0) return true
  for (const e of entities.value) {
    if (selectedIds.value.has(e.id) && canUpdate(e)) return true
  }
  return false
}

// Selection and actions
const { selectedIds, toggle: toggleSelection, clear: clearActionSelection, isSelected, selectAll } = useListSelection()
const hasSelection = computed(() => selectedIds.value.size > 0)

const listIdRef = computed(() => props.listId)

const { resolvedActions, processing: actionProcessing, executeAction, triggerAction } = useListActions({
  listId: listIdRef,
  selectedIds,
  entities,
  onClearSelection: () => clearActionSelection(),
  onRequestConfirm: (action, actionId, triggerEl) => {
    void requestActionConfirm(action, actionId, triggerEl)
  },
  // Bulk actions mutate entities server-side; invalidate the list so it
  // refetches the post-action state.
  onComplete: () => {
    void queryCache.invalidateQueries({ key: entityKeys.list(listConfig.value?.entity ?? '') })
  },
})

// Bulk action confirm. We don't pass executeAction as onConfirm because it
// uses Promise.allSettled internally and never throws — partial failures
// surface via uiStore.error and the script-error dialog. Wrapping it in
// onConfirm would silently report success even when 100% of writes failed.
// Instead: confirm-then-fire-and-forget. The action toasts its own results.
async function requestActionConfirm(
  action: import('@/types').ActionConfig,
  actionId: string,
  triggerEl: HTMLElement | null,
) {
  const ok = await confirm({
    title: `${action.label}?`,
    message: `Apply ${action.label} to ${selectedIds.value.size} selected entities?`,
    confirmLabel: action.label,
  })
  if (!ok) return
  void executeAction(actionId, action, triggerEl)
}

// Static (config-pinned) filter properties — used by useUrlFilterSync to
// reject URL filters that would silently override the list's intended scope.
// Computed inline (not via configuredFilters) to avoid a forward reference.
function staticFilterProperties(): Set<string> {
  const list = schemaStore.getList(props.listId)
  const set = new Set<string>()
  for (const f of list?.filters || []) {
    if (f.operator && f.value && f.property) set.add(f.property)
  }
  return set
}

// User-selected filters and free-text search synced bidirectionally with the URL.
const { filters, q: searchQuery, writeToQuery } = useUrlFilterSync({ staticFilterProperties })
const searchBoxRef = ref<InstanceType<typeof SearchBox> | null>(null)
const filterMenuRef = ref<InstanceType<typeof AdHocFilterMenu> | null>(null)

// Sort specs: array of { property, direction } for multi-field sorting
interface SortSpec {
  property: string
  direction: 'asc' | 'desc'
}
const sortSpecs = ref<SortSpec[]>([])

// Computed for keyboard navigation
const itemCount = computed(() => entities.value.length)
const hasPrevPage = computed(() => page.value > 1)
const hasNextPage = computed(() => meta.value.has_more || page.value * meta.value.per_page < meta.value.total)

// Keyboard navigation
const { selectedIndex, clearSelection } = useListKeyboard({
  itemCount,
  hasPrevPage,
  hasNextPage,
  hasSelection,
  onOpen: (index) => {
    const entity = entities.value[index]
    if (entity) navigateToEntity(entity)
  },
  onEdit: (index) => {
    const entity = entities.value[index]
    if (entity && listConfig.value?.edit_form) {
      router.push(`/form/${listConfig.value.edit_form}/${entity.id}`)
    }
  },
  onCreate: () => {
    if (listConfig.value?.create_form) {
      router.push(`/form/${listConfig.value.create_form}`)
    }
  },
  onDelete: (index) => {
    const entity = entities.value[index]
    if (!entity) return
    if (!canDelete(entity)) {
      uiStore.warning('Delete not permitted for this entity')
      return
    }
    void requestDelete(entity)
  },
  onSelect: (index) => {
    const entity = entities.value[index]
    if (entity) toggleSelection(entity.id)
  },
  onClearSelection: () => {
    clearActionSelection()
  },
  onPrevPage: () => {
    if (hasPrevPage.value) {
      handlePageChange(page.value - 1)
    }
  },
  onNextPage: () => {
    if (hasNextPage.value) {
      handlePageChange(page.value + 1)
    }
  },
  onFocusSearch: () => {
    searchBoxRef.value?.focus()
  },
  onOpenFilter: () => {
    filterMenuRef.value?.open()
  },
})

// Computed
const listConfig = computed(() => schemaStore.getList(props.listId))
const entityType = computed(() => {
  if (!listConfig.value) return undefined
  return schemaStore.getEntityType(listConfig.value.entity)
})

// Pre-configured filters from list config
const configuredFilters = computed(() => {
  return listConfig.value?.filters?.filter(f => f.operator && f.value) || []
})

// Check if any columns reference relations (need to include related entities)
const hasRelationColumns = computed(() => {
  return listConfig.value?.columns?.some(col => col.relation) || false
})

const hasActions = computed(() => resolvedActions.value.length > 0)

// Build query params. Reads `page` (input state), never `meta` (query
// output) — otherwise the query key would depend on its own result.
const queryParams = computed((): ListParams => {
  const params: ListParams = {
    page: page.value,
    per_page: listConfig.value?.page_size || 25,
  }

  // Add pre-configured filters from list config
  for (const filter of configuredFilters.value) {
    const apiOp = toApiOperator(filter.operator)
    const key = `filter[${filter.property}][${apiOp}]`
    const filterValue = filter.value
    // Append to existing filter or create new
    const existing = (params as Record<string, string | number | undefined>)[key]
    if (existing) {
      (params as Record<string, string | number | undefined>)[key] = `${existing},${filterValue}`
    } else {
      (params as Record<string, string | number | undefined>)[key] = filterValue as string
    }
  }

  // Add user-selected filters via the shared serializer so EntityList and
  // useScopeNavigation stay in lockstep on the wire format.
  const userParams = filterStateToApiParams(filters.value)
  const paramsRecord = params as Record<string, string | number | undefined>
  for (const [key, value] of Object.entries(userParams)) {
    paramsRecord[key] = value
  }

  // Free-text search: backend intersects ?q= results with the typed list.
  if (searchQuery.value) {
    paramsRecord.q = searchQuery.value
  }

  // Add sorting - supports multi-field sorting
  if (sortSpecs.value.length > 0) {
    params.sort = sortSpecs.value
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  } else if (listConfig.value?.default_sort?.length) {
    const defaultSort = listConfig.value.default_sort
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
    params.sort = defaultSort
  }

  // Include related entities for relation columns
  if (hasRelationColumns.value) {
    params.include = '*'
  }

  return params
})

// The list query (FEAT-XY2D1L). Declared here — after queryParams /
// listConfig — because Pinia Colada evaluates `enabled` synchronously
// during setup; the derived computeds up in the State block read it
// through `listQueryRef`. Keyed on the canonicalized params, so
// page/filter/sort/search changes swap cache entries automatically — the
// reactive key coalesces synchronous trigger bursts and drops stale
// responses, replacing the old generation counter + scheduleFetch
// microtask. useEvents' SSE invalidation on ['entities', <type>] marks it
// stale and background-refetches while mounted, so lists go live (they
// never reacted to SSE before). placeholderData keeps the previous rows
// visible during a param change instead of flashing the spinner.
const listQuery = useQuery({
  key: () => entityKeys.listParams(listConfig.value?.entity ?? '', queryParams.value),
  query: () => {
    const config = listConfig.value
    if (!config) throw new Error(`unknown list: ${props.listId}`)
    return listEntities(config.entity, queryParams.value)
  },
  enabled: () => !!listConfig.value,
  placeholderData: (prev) => prev,
})
listQueryRef.value = listQuery

function handleSort(field: string, event: MouseEvent) {
  const existingIndex = sortSpecs.value.findIndex((s) => s.property === field)

  if (event.shiftKey) {
    // Multi-sort: add/toggle field while keeping others
    if (existingIndex >= 0) {
      // Toggle direction if already in list
      const spec = sortSpecs.value[existingIndex]
      if (spec.direction === 'asc') {
        spec.direction = 'desc'
      } else {
        // Remove from sort if clicking desc again
        sortSpecs.value.splice(existingIndex, 1)
      }
    } else {
      // Add new sort field
      sortSpecs.value.push({ property: field, direction: 'asc' })
    }
  } else {
    // Single-sort: replace all with just this field
    if (existingIndex >= 0 && sortSpecs.value.length === 1) {
      // Toggle direction if already the only sort
      sortSpecs.value[0].direction = sortSpecs.value[0].direction === 'asc' ? 'desc' : 'asc'
    } else {
      // Replace all sorts with just this field
      sortSpecs.value = [{ property: field, direction: 'asc' }]
    }
  }

  // Sort changed → reset to page 1; the query reacts to both inputs.
  page.value = 1
}

// Helper to get sort index and direction for a field
function getSortInfo(field: string): { index: number; direction: 'asc' | 'desc' | null } {
  const idx = sortSpecs.value.findIndex((s) => s.property === field)
  if (idx < 0) return { index: -1, direction: null }
  return { index: idx, direction: sortSpecs.value[idx].direction }
}

function handleFilter(newFilters: FilterState) {
  // The filters watcher reacts to this and triggers loadEntities.
  writeToQuery(newFilters)
}

function handleSearchUpdate(value: string) {
  // Watcher on searchQuery resets page and re-fetches.
  writeToQuery(filters.value, value)
}

// Properties hidden from the AdHocFilterMenu — already covered by the
// FilterBar's static widgets (filter_controls), already pinned by the list's
// `filters:` config, or already active as an ad-hoc chip. Including all three
// in one set so the menu never offers a duplicate.
const lockedAdHocProperties = computed(() => {
  const set = new Set<string>(staticFilterProperties())
  for (const fc of listConfig.value?.filter_controls || []) {
    if (fc.property) set.add(fc.property)
    if (fc.relation) set.add(fc.relation)
  }
  for (const prop of Object.keys(filters.value)) set.add(prop)
  return set
})

function handleAdHocApply(property: string, value: string) {
  writeToQuery({ ...filters.value, [property]: { value } })
}

function removeAdHocFilter(property: string) {
  const next = { ...filters.value }
  delete next[property]
  writeToQuery(next)
}

// Filters added via the ad-hoc menu are rendered as chips. We treat any
// active filter that isn't covered by FilterBar (filter_controls) and isn't
// a static-pinned config filter as ad-hoc.
const adHocFilterChips = computed(() => {
  const filterControlKeys = new Set<string>()
  for (const fc of listConfig.value?.filter_controls || []) {
    if (fc.property) filterControlKeys.add(fc.property)
    if (fc.relation) filterControlKeys.add(fc.relation)
  }
  const pinned = staticFilterProperties()
  return Object.entries(filters.value)
    .filter(([prop]) => !filterControlKeys.has(prop) && !pinned.has(prop))
    .map(([property, fv]) => ({ property, value: fv.value }))
})

function handlePageChange(newPage: number) {
  page.value = newPage
}

// Resolve a link configuration value to a path (mirrors backend resolveLinkTarget)
function resolveLinkTarget(link: string, entityType: string, entityId: string): string {
  if (!link) return ''
  if (link === 'detail') return `/entity/${entityType}/${entityId}`
  if (link.startsWith('document/')) {
    const docName = link.slice('document/'.length)
    return `/document/${docName}/${entityId}`
  }
  return ''
}

function navigateToEntity(entity: Entity) {
  // Build query params to preserve navigation context.
  // Filters are already in `route.query` via useUrlFilterSync — we just
  // forward all `filter[*]` entries unchanged so the bracket format is the
  // single source of truth (no legacy `filter_*` underscore form).
  const query: Record<string, string | string[]> = {
    from: props.listId,
    scope: `list:${props.listId}`,
  }

  // Include sort if active
  if (sortSpecs.value.length > 0) {
    query.sort = sortSpecs.value
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  } else if (listConfig.value?.default_sort?.length) {
    query.sort = listConfig.value.default_sort
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  }

  // Forward bracket-format filter params from the current URL. Narrow the
  // LocationQueryValue type explicitly (it's string | null | (string|null)[]).
  for (const [key, value] of Object.entries(route.query)) {
    if (!key.startsWith('filter[')) continue
    if (value === null) continue
    if (Array.isArray(value)) {
      const filtered = value.filter((v): v is string => v !== null)
      if (filtered.length > 0) query[key] = filtered
    } else {
      query[key] = value
    }
  }

  // Forward search query so the back-button to a searched list keeps the
  // search state. Scope navigation (prev/next within a search result set)
  // is a known v1 limitation — useScopeNavigation does not yet honor q.
  if (searchQuery.value) {
    query.q = searchQuery.value
  }

  // Check for column-level link first (use first column with link)
  const columnWithLink = listConfig.value?.columns?.find((col) => col.link)
  const columnLink = columnWithLink?.link
    ? resolveLinkTarget(columnWithLink.link, entity.type, entity.id)
    : ''

  // entityDetailHref returns columnLink when set, otherwise the
  // entity-route path. Centralised so right-click / middle-click open
  // through a real <a href> on the row markup elsewhere.
  const path = entityDetailHref(
    { id: entity.id, type: entity.type },
    { cellLink: columnLink },
  )
  if (!path) return
  router.push({ path, query })
}

function isEnumColumn(column: { property?: string }): boolean {
  if (!column.property || !entityType.value) return false
  return isEnumPropertyDef(entityType.value.properties[column.property])
}

// isCellInaccessible reports whether the cell's underlying property is
// listed in the entity's inaccessible array (e.g. git-crypt encrypted).
// Such cells render a lock indicator instead of the value.
function isCellInaccessible(entity: Entity, column: { property?: string }): boolean {
  if (!entity.inaccessible || entity.inaccessible.length === 0) return false
  if (!column.property) return false
  return entity.inaccessible.some((f) => f.name === column.property)
}

function getFormattedCellValue(entity: Entity, column: { property?: string; relation?: string }): string {
  // For relation columns, resolve IDs to titles using included entities
  if (column.relation) {
    const relationIds = entity.relations?.[column.relation] || []
    const titles = relationIds.map((id) => {
      const included = includedEntities.value[id]
      return included?._title || included?.properties?.title || id
    })
    return titles.join(', ')
  }

  const value = getCellValue(entity, column)
  return formatCellValue(value, column.property, entityType.value)
}

function handleDelete(entity: Entity, event: Event) {
  event.stopPropagation()
  void requestDelete(entity)
}

// Delete as an optimistic mutation: the row vanishes immediately, rolls
// back on failure, and the list prefix is invalidated on settle so every
// param variant (other pages/filters where the entity might appear)
// refetches and reconciles with the server.
const { mutate: deleteEntityMutation } = useMutation({
  mutation: ({ entity }: { entity: Entity }) => deleteEntity(entity.type, entity.id),
  onMutate({ entity }: { entity: Entity }) {
    return beginOptimisticRemove(
      queryCache,
      entityKeys.listParams(entity.type, queryParams.value),
      entity.id
    )
  },
  onError(err, _vars, context) {
    rollbackOptimistic(queryCache, context)
    uiStore.error(getErrorMessage(err, 'Failed to delete entity'))
  },
  onSuccess(_data, { entity }) {
    uiStore.success(`Deleted ${entity.id}`)
  },
  async onSettled(_data, _err, { entity }) {
    // Invalidate the whole list (all param variants), not just the
    // visible page — the deleted entity may appear under other filters.
    await queryCache.invalidateQueries({ key: entityKeys.list(entity.type) })
  },
})

async function requestDelete(entity: Entity) {
  const ok = await confirm({
    title: 'Delete Entity?',
    message: `Are you sure you want to delete '${entity.id}'? This action cannot be undone.`,
    confirmLabel: 'Delete',
    danger: true,
  })
  if (!ok) return
  deleteEntityMutation({ entity })
}

// Reset paging/sort/selection on list switch. The query reacts to the
// input changes — no manual fetch trigger needed.
watch(() => props.listId, () => {
  sortSpecs.value = []
  page.value = 1
  clearSelection()
  clearActionSelection()
})

// Clear selection when the visible entities change (incl. background
// SSE-driven refetches).
watch(entities, () => {
  clearSelection()
  clearActionSelection()
})

// Reset to page 1 when filters or free-text search change (covers user
// edits via writeToQuery and back/forward nav the URL sync picks up). The
// query re-keys off page + filters + search together, so a simultaneous
// filter+search edit still produces one fetch.
watch(filters, () => {
  page.value = 1
}, { deep: true })

watch(searchQuery, () => {
  page.value = 1
})
</script>

<template>
  <div v-if="listConfig" class="entity-list">
    <header class="list-header">
      <div class="header-left">
        <BackButton v-if="backTarget" :target="backTarget" />
        <h1>{{ listConfig.title || listConfig.entity }}</h1>
      </div>
      <router-link
        v-if="listConfig.create_form && canCreate()"
        :to="`/form/${listConfig.create_form}`"
        class="btn btn-primary"
      >
        + New <kbd>N</kbd>
      </router-link>
    </header>

    <div v-if="configuredFilters.length" class="configured-filters">
      <span
        v-for="filter in configuredFilters"
        :key="`${filter.property}-${filter.operator}-${filter.value}`"
        class="filter-chip"
      >
        {{ filter.property }} {{ filter.operator }} {{ filter.value }}
      </span>
    </div>

    <div class="search-row">
      <SearchBox
        ref="searchBoxRef"
        :model-value="searchQuery"
        :placeholder="`Search ${listConfig.entity}s...`"
        @update:model-value="handleSearchUpdate"
      />
      <AdHocFilterMenu
        ref="filterMenuRef"
        mode="list"
        :entity-type="entityType"
        :locked-properties="lockedAdHocProperties"
        @apply="handleAdHocApply"
      />
    </div>

    <div v-if="adHocFilterChips.length" class="adhoc-filter-chips">
      <span
        v-for="chip in adHocFilterChips"
        :key="chip.property"
        class="filter-chip removable"
      >
        {{ chip.property }}: {{ chip.value }}
        <button
          type="button"
          class="chip-remove"
          :title="`Remove ${chip.property} filter`"
          @click="removeAdHocFilter(chip.property)"
        >
          &times;
        </button>
      </span>
    </div>

    <div class="list-content">
      <FilterBar
        v-if="listConfig.filter_controls?.length"
        :config="listConfig"
        :entity-type="entityType"
        :filters="filters"
        @filter="handleFilter"
      />
      <div v-if="loading" class="loading-state">
        <div class="spinner"/>
        <span>Loading...</span>
      </div>

      <div v-else-if="loadError" class="empty-state load-error">
        <p>{{ loadError }}</p>
        <button type="button" class="btn btn-secondary" @click="listQuery.refetch()">
          Retry
        </button>
      </div>

      <div v-else-if="entities.length === 0" class="empty-state">
        <p v-if="searchQuery">No matches for &ldquo;{{ searchQuery }}&rdquo;.</p>
        <p v-else>No {{ listConfig.entity }}s found.</p>
        <button
          v-if="searchQuery"
          type="button"
          class="btn btn-secondary"
          @click="handleSearchUpdate('')"
        >
          Clear search
        </button>
        <router-link
          v-else-if="listConfig.create_form"
          :to="`/form/${listConfig.create_form}`"
          class="btn btn-secondary"
        >
          Create one
        </router-link>
      </div>

      <template v-else>
      <!-- Mobile card layout -->
      <div v-if="isMobile" class="mobile-card-list">
        <div
          v-for="(entity, index) in entities"
          :key="'card-' + entity.id"
          class="mobile-card"
          :class="{ selected: index === selectedIndex, 'action-selected': isSelected(entity.id) }"
          @click="navigateToEntity(entity)"
        >
          <div class="mobile-card-header">
            <span class="mobile-card-title">
              {{ getFormattedCellValue(entity, listConfig.columns[0]) }}
            </span>
            <button
              v-if="canDelete(entity)"
              class="delete-btn"
              title="Delete"
              @click="handleDelete(entity, $event)"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="3 6 5 6 21 6"/>
                <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
              </svg>
            </button>
          </div>
          <div v-if="listConfig.columns.length > 1" class="mobile-card-fields">
            <div
              v-for="column in listConfig.columns.slice(1)"
              :key="column.property || column.relation"
              class="mobile-card-field"
            >
              <span class="mobile-card-label">{{ column.label || column.property || column.relation }}</span>
              <span
                v-if="isCellInaccessible(entity, column)"
                class="inaccessible-cell"
                title="inaccessible"
              >🔒</span>
              <div
                v-else-if="isEnumColumn(column) && asArray(getCellValue(entity, column)).length > 0"
                class="badge-row"
              >
                <Badge
                  v-for="badgeValue in asArray(getCellValue(entity, column))"
                  :key="badgeValue"
                  :value="badgeValue"
                  :property="column.property"
                  :entity-type="entityType"
                />
              </div>
              <span v-else class="mobile-card-value">{{ getFormattedCellValue(entity, column) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Desktop table layout -->
      <div v-else class="table-scroll-wrapper">
      <table class="entity-table">
        <thead>
          <tr v-if="hasSelection" class="action-header-row">
            <th class="select-column">
              <input
                type="checkbox"
                :checked="selectedIds.size === entities.length"
                :indeterminate="selectedIds.size > 0 && selectedIds.size < entities.length"
                @change="selectedIds.size === entities.length ? clearActionSelection() : selectAll(entities.map(e => e.id))"
              />
            </th>
            <th :colspan="listConfig.columns.length + 1" class="action-header-cell">
              <span class="action-header-count">{{ selectedIds.size }} selected</span>
              <button
                v-for="{ id, config } in resolvedActions"
                v-show="anySelectedAllowsUpdate()"
                :key="id"
                class="action-header-btn"
                :disabled="actionProcessing"
                @click="(e) => triggerAction(id, config, e)"
              >
                <kbd>{{ config.key }}</kbd>
                {{ config.label }}
              </button>
            </th>
          </tr>
          <tr v-else>
            <th v-if="hasActions" class="select-column">
              <input
                type="checkbox"
                :checked="false"
                @change="selectAll(entities.map(e => e.id))"
              />
            </th>
            <th
              v-for="column in listConfig.columns"
              :key="column.property || column.relation"
              :class="{
                sortable: column.sortable !== false && column.property,
                sorted: getSortInfo(column.property || '').index >= 0,
                'sorted-desc': getSortInfo(column.property || '').direction === 'desc',
              }"
              @click="column.sortable !== false && column.property && handleSort(column.property, $event)"
            >
              {{ column.label || column.property || column.relation }}
              <span v-if="getSortInfo(column.property || '').index >= 0" class="sort-indicator">
                <span v-if="sortSpecs.length > 1" class="sort-order">{{ getSortInfo(column.property || '').index + 1 }}</span>
                {{ getSortInfo(column.property || '').direction === 'desc' ? '▼' : '▲' }}
              </span>
            </th>
            <th class="actions-column"/>
          </tr>
        </thead>
        <TransitionGroup tag="tbody" name="row">
          <tr
            v-for="(entity, index) in entities"
            :key="entity.id"
            class="entity-row"
            :data-entity-id="entity.id"
            :class="{ selected: index === selectedIndex, 'action-selected': isSelected(entity.id) }"
            @click="navigateToEntity(entity)"
          >
            <td v-if="hasActions" class="select-cell" @click.stop>
              <input
                type="checkbox"
                :checked="isSelected(entity.id)"
                @change="toggleSelection(entity.id)"
              />
            </td>
            <td
              v-for="column in listConfig.columns"
              :key="column.property || column.relation"
            >
              <span
                v-if="isCellInaccessible(entity, column)"
                class="inaccessible-cell"
                title="inaccessible"
              >🔒</span>
              <div
                v-else-if="isEnumColumn(column) && asArray(getCellValue(entity, column)).length > 0"
                class="badge-row"
              >
                <Badge
                  v-for="badgeValue in asArray(getCellValue(entity, column))"
                  :key="badgeValue"
                  :value="badgeValue"
                  :property="column.property"
                  :entity-type="entityType"
                />
              </div>
              <span v-else>
                {{ getFormattedCellValue(entity, column) }}
              </span>
            </td>
            <td class="actions-cell">
              <button
                v-if="canDelete(entity)"
                class="delete-btn"
                title="Delete"
                @click="handleDelete(entity, $event)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="3 6 5 6 21 6"/>
                  <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
                </svg>
              </button>
            </td>
          </tr>
        </TransitionGroup>
      </table>
      </div>
      </template>

      <Pagination
        v-if="meta.total > meta.per_page"
        :meta="meta"
        @page-change="handlePageChange"
      />
    </div>
  </div>

  <div v-else class="error-state">
    <h2>List not found</h2>
    <p>The list "{{ listId }}" does not exist in the configuration.</p>
  </div>
</template>

<style scoped>
.inaccessible-cell {
  color: var(--color-text-muted, #888);
  font-style: italic;
  cursor: help;
}

.entity-list {
  max-width: 1200px;
}

.list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.list-header h1 {
  margin: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover {
  filter: brightness(0.9);
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover {
  background: var(--hover-bg);
}

.configured-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  margin-bottom: 12px;
}

.filter-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  font-size: 12px;
  color: var(--text-color);
}

.filter-chip.removable {
  gap: 6px;
  padding-right: 4px;
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  border-color: color-mix(in srgb, var(--accent-color) 30%, transparent);
  color: var(--accent-color);
}

.chip-remove {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  padding: 0 4px;
  color: inherit;
  opacity: 0.7;
}

.chip-remove:hover {
  opacity: 1;
}

.search-row {
  display: flex;
  align-items: stretch;
  gap: 8px;
  margin-bottom: 12px;
}

.adhoc-filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.list-content {
  background: var(--card-bg);
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: var(--muted-text);
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.entity-table {
  width: 100%;
  border-collapse: collapse;
}

.entity-table thead {
  position: sticky;
  top: 0;
  z-index: 5;
}

.entity-table th {
  text-align: left;
  padding: 12px 16px;
  background: var(--hover-bg);
  border-bottom: 1px solid var(--border-color);
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.action-header-row th {
  background: var(--hover-bg);
}

.action-header-cell {
  text-transform: none;
  letter-spacing: normal;
}

.action-header-count {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-color);
  margin-right: 0.75rem;
}

.action-header-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  margin-right: 0.5rem;
  vertical-align: middle;
  padding: 0.2rem 0.6rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--card-bg);
  color: var(--text-color);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.action-header-btn:hover:not(:disabled) {
  background: var(--hover-bg);
}

.action-header-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.action-header-btn kbd {
  display: inline-block;
  padding: 0.05rem 0.3rem;
  border: 1px solid var(--border-color);
  border-radius: 3px;
  background: var(--hover-bg);
  font-family: monospace;
  font-size: 11px;
  line-height: 1;
}

.entity-table th.sortable {
  cursor: pointer;
  user-select: none;
}

.entity-table th.sortable:hover {
  filter: brightness(0.95);
}

.entity-table th.sorted {
  color: var(--accent-color);
}

.sort-indicator {
  margin-left: 4px;
  font-size: 10px;
}

.sort-order {
  font-size: 9px;
  background: var(--accent-color);
  color: white;
  padding: 1px 4px;
  border-radius: 8px;
  margin-right: 2px;
}

.entity-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
}

.entity-row {
  cursor: pointer;
  transition: background 0.15s;
}

.entity-row:hover {
  background: var(--hover-bg);
}

.entity-row.selected {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  outline: 2px solid var(--accent-color);
  outline-offset: -2px;
}

.entity-row.selected:hover {
  background: color-mix(in srgb, var(--accent-color) 25%, transparent);
}

.entity-row.action-selected {
  background: color-mix(in srgb, var(--accent-color) 10%, transparent);
}

.entity-row.action-selected:hover {
  background: color-mix(in srgb, var(--accent-color) 20%, transparent);
}

.row-leave-active {
  transition: opacity 0.35s ease, background-color 0.35s ease;
  overflow: hidden;
}

.row-leave-active td {
  transition: padding 0.35s ease, line-height 0.35s ease, font-size 0.35s ease, border-color 0.35s ease;
  overflow: hidden;
}

.row-leave-from {
  opacity: 1;
}

.row-leave-to {
  opacity: 0;
  background-color: color-mix(in srgb, var(--accent-color) 20%, transparent);
}

.row-leave-to td {
  padding-top: 0;
  padding-bottom: 0;
  line-height: 0;
  font-size: 0;
  border-color: transparent;
}

.select-column,
.select-cell {
  width: 32px;
  text-align: center;
}

.select-cell input[type="checkbox"],
.select-column input[type="checkbox"] {
  cursor: pointer;
  accent-color: var(--accent-color, #6366f1);
}

.error-state {
  padding: 48px;
  text-align: center;
  color: var(--muted-text);
}

.error-state h2 {
  color: var(--error-color, #ef4444);
}

.actions-column {
  width: 40px;
}

.actions-cell {
  width: 40px;
  white-space: nowrap;
}

.delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: var(--muted-text);
  cursor: pointer;
  transition: all 0.15s;
}

.delete-btn:hover {
  background: color-mix(in srgb, var(--error-color) 15%, transparent);
  color: var(--error-color);
}

.table-scroll-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.mobile-card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.mobile-card + .mobile-card {
  margin-top: 8px;
}

.mobile-card:hover {
  border-color: var(--accent-color, #6366f1);
}

.mobile-card.selected {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  outline: 2px solid var(--accent-color);
  outline-offset: -2px;
}

.mobile-card.action-selected {
  background: color-mix(in srgb, var(--accent-color) 10%, transparent);
}

.mobile-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
}

.mobile-card-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-color);
  flex: 1;
  min-width: 0;
  word-break: break-word;
}

.mobile-card-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border-color);
}

.mobile-card-field {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
}

.mobile-card-label {
  color: var(--muted-text);
}

.mobile-card-label::after {
  content: ':';
}

.mobile-card-value {
  color: var(--text-color);
}

@media (max-width: 768px) {
  .list-header {
    position: sticky;
    top: 0;
    z-index: 10;
    background: var(--bg-color);
    padding: 8px 0;
    margin-bottom: 12px;
  }

  .list-header h1 {
    font-size: 18px;
  }

  .list-content {
    background: none;
    box-shadow: none;
    border-radius: 0;
    overflow: visible;
  }

  .mobile-card .delete-btn {
    width: 44px;
    height: 44px;
  }
}
</style>
