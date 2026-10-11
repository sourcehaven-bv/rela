<script setup lang="ts">
import { ref, shallowRef, computed, watch, nextTick, onMounted, onUnmounted, type Component } from 'vue'
import { RouterLink, useRoute, useRouter, type RouteLocationRaw } from 'vue-router'
import { useQuery, useQueryCache } from '@pinia/colada'
import { useSchemaStore, useUIStore } from '@/stores'
import { useListKeyboard } from '@/composables/useListKeyboard'
import { useListSelection } from '@/composables/useListSelection'
import { useListActions } from '@/composables/useListActions'
import { useListReorder } from '@/composables/useListReorder'
import { useUrlFilterSync } from '@/composables/useUrlFilterSync'
import { useWorld } from '@/composables/useWorld'
import { useCreateTarget } from '@/composables/useCreateTarget'
import { useListGrouping, type ListSection } from '@/composables/useListGrouping'
import { listEntities, listAllEntities, getErrorMessage } from '@/api'
import { entityKeys } from '@/queries/entities'
import { beginOptimisticRemove } from '@/queries/optimisticList'
import { filterStateToApiParams } from '@/utils/filters'
import { defaultSortParam, groupedSort, listBaseParams, sortParam } from '@/utils/listParams'
import { editFormRoute, entityDetailHref } from '@/utils/entityRoute'
import { entityRef } from '@/utils/entityRef'
import { worldText } from '@/utils/worldText'
import { safeInternalHref } from '@/utils/openIntent'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { renderMarkdown } from '@/utils/markdown'
import { actionAllowed } from '@/utils/affordancesWarning'
import { getCellValue, formatCellValue } from '@/utils/format'
import { densePropertyRoutingHint, isDenseEmpty } from '@/widgets/viewRouting'
import { defaultRegistry } from '@/widgets/registry'
import type { DenseRoutingHint } from '@/widgets/viewRouting'
import type { Entity, ListMeta, ListParams, ListResponse, FilterState, PageScope } from '@/types'
import { viewHeaderMarkdown, viewFooterMarkdown } from '@/types'
import FilterBar from './FilterBar.vue'
import Pagination from './Pagination.vue'
import SearchBox from './SearchBox.vue'
import AdHocFilterMenu from './AdHocFilterMenu.vue'
import BackButton from '@/components/common/BackButton.vue'
import ExportMenu from '@/components/entity/ExportMenu.vue'
import AddToPileMenu from '@/components/piles/AddToPileMenu.vue'
import PageHeaderContent from '@/components/common/PageHeaderContent'
import InlineCreateFormModal from '@/components/forms/InlineCreateFormModal.vue'
import { useCreateModal } from '@/composables/useCreateModal'
import { usePageTabScope } from '@/composables/usePageTabScope'
import WorldBadge from '@/components/entity/WorldBadge.vue'
import WorldBanner from '@/components/common/WorldBanner.vue'
import { listExportUrl } from '@/api/transforms'
import { useBackTarget } from '@/composables/useBackTarget'
import { useConfirm } from '@/composables/useConfirm'
import { useBulkDelete, useDeleteKey } from '@/composables/useBulkDelete'
import { useDetailPanel } from '@/composables/useDetailPanel'
import EntityDetailPanel from '@/components/entity/EntityDetailPanel.vue'
import RlBulkActionBar from 'rela-components/components/data/RlBulkActionBar.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlBanner from 'rela-components/components/feedback/RlBanner.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import RlKbd from 'rela-components/components/data/RlKbd.vue'
import RlTable from 'rela-components/components/table/RlTable.vue'
import type { SortClickEvent, TableColumn } from 'rela-components/components/table/types'
import { listColumnOf, nameColumn, toTableColumns, toTableNameColumn } from './tableColumns'
import { fromPageQuery } from '@/utils/pageContext'

const props = defineProps<{
  listId: string
  /** Set when the list is a tab of an entity page: rows are those the anchor reaches. */
  pageScope?: PageScope
}>()

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { confirm } = useConfirm()

// Back affordance — renders when ?return_to= or ?from= is present. See TKT-JIEKC.
const backTarget = useBackTarget()

/*
 * The detail panel, addressed by `?selected=<id>`.
 *
 * The URL is the ONLY state: there is no local `selected` ref that the query
 * mirrors. That is what makes the panel survive a reload, a back step and a
 * pasted link without a second code path — the same reason filters and sort
 * already live in the query. A mirrored ref would need reconciling with the
 * route on every external navigation, which is the bug useUrlFilterSync's
 * echo-signature exists to manage; with no local copy there is no echo.
 *
 * The entity TYPE is not in the query. It is `listConfig.entity`, so putting
 * it in the URL would let a link name a row this list cannot contain.
 */
const panel = useDetailPanel()

const selectedEntityId = computed(() => {
  const raw = route.query.selected
  const value = Array.isArray(raw) ? raw[raw.length - 1] : raw
  return typeof value === 'string' && value !== '' ? value : null
})

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
// `isPending` is true while a query key has no resolved data. Same-key SSE
// refetches keep the entry `success` (placeholderData holds the rows, no
// spinner — the liveness win).
//
// A param change (page/filter/sort) swaps to a NEW key whose entry starts
// out pending — but `placeholderData: (prev) => prev` below seeds it from
// the previous entry, and Colada then exposes the state as
// `{status: 'success', data: placeholderData}`. So `isPending` stays false
// and the old rows are held on screen instead of flashing the spinner.
// The block spinner is therefore reached only on a cold first load.
// Pinned by the "pagination keeps previous rows" tests.
const loading = computed(() => listQueryRef.value?.isPending.value ?? true)

// pageState is a stable, test-visible signal of whether this screen has
// finished resolving — the same contract DynamicForm's `form-state-*` marker
// provides, generalized so a screenshot can wait for ANY screen rather than
// only an edit form. Without it a capture has nothing to poll and can only
// hang until its timeout, which is why screenshot{} used to refuse every view
// but the form.
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (loadError.value) return 'error'
  return loading.value ? 'pending' : 'loaded'
})
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

// The table's own Add button, below the rows: the same dialog as New, and
// hidden under the same gate. In a grouped list each section has one, and the
// new row starts with that section's value so it lands where it was added.
const createPrefill = ref<{ properties: Record<string, unknown> }>()
function onTableAdd(section: ListSection | { prefill?: undefined }) {
  createPrefill.value = section.prefill ? { properties: section.prefill } : undefined
  createModal.show()
}
// From `_actions` alone, under every world. The server computes the map for
// the FACE it served (a stand-in published face reports `update: false`
// unless a grant names that face), and every write this list makes goes to
// that row's address (`entityRef`), so the verdict and the write agree by
// construction. An earlier revision ANDed in `!isWorldBound` here, which made
// every list read-only under a configured `default_world` — including lists
// of types that declare no faces at all (atlas worlds issue 2).
//
// canUpdate gates the configured bulk actions (via anySelectedAllowsUpdate);
// canDelete gates row selection, the bar's Delete and the Delete/Backspace
// shortcut.
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
const { selectedIds, toggle: toggleSelection, clear: clearActionSelection, selectAll } = useListSelection()
const hasSelection = computed(() => selectedIds.value.size > 0)

// The selected rows on this page that the principal may delete. A selected
// row without a delete grant is left out of the delete and stays selected.
const deletableSelection = computed(() =>
  entities.value.filter((e) => selectedIds.value.has(e.id) && canDelete(e))
)
const anyRowDeletable = computed(() => entities.value.some(canDelete))

// Bulk delete, with no confirm: the server keeps a deleted entity for a grace
// period, and the toast that reports the delete offers Undo.
const { deleting, deleteMany } = useBulkDelete({
  noun: () => entityNoun.value,
  // Rows leave at once; the refetch afterwards brings back any that failed.
  onStart: (ids) => {
    for (const id of ids) beginOptimisticRemove(queryCache, listKey.value, id)
  },
  // Rows that were not deleted stay selected: those whose delete failed, and
  // those the principal could not delete. The entities watcher below clears
  // the selection whenever the rows change, so this waits for it to run.
  onSettled: async ({ deleted }) => {
    await nextTick()
    const gone = new Set(deleted)
    selectedIds.value = new Set([...selectionBeforeDelete].filter((id) => !gone.has(id)))
  },
  // Every param variant: a deleted entity may appear under other filters.
  refresh: () => queryCache.invalidateQueries({ key: entityKeys.list(listConfig.value?.entity ?? '') }),
})
let selectionBeforeDelete = new Set<string>()

function deleteSelected() {
  const rows = deletableSelection.value
  if (rows.length === 0) return
  selectionBeforeDelete = new Set(selectedIds.value)
  // Addressed to the ROW: on a bare face this deletes the entity, on a
  // non-bare face it removes that face only (the server's rule for `ID@face`).
  void deleteMany(rows.map((e) => ({ id: e.id, type: e.type, ref: entityRef(e) })))
}

useDeleteKey({
  enabled: () => deletableSelection.value.length > 0,
  onDelete: deleteSelected,
})

// What the bulk bar counts, in the schema's own words ("3 taken selected").
const entityNoun = computed(() => {
  const type = listConfig.value?.entity ?? ''
  const def = schemaStore.getEntityType(type)
  const singular = def?.label || type || 'item'
  return { singular, plural: def?.label_plural || def?.plural || undefined }
})

const listIdRef = computed(() => props.listId)

const { resolvedActions, processing: actionProcessing, executeAction, triggerAction } = useListActions({
  listId: listIdRef,
  selectedIds,
  entities,
  // A bulk `set` writes the ROW on screen. Selection is keyed by entity id,
  // so the address is looked up per row at execution time.
  addressOf: (entityId) => {
    const row = entities.value.find((e) => e.id === entityId)
    return row ? entityRef(row) : entityId
  },
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
    message:
      typeof action.confirm === 'string'
        ? action.confirm
        : `Apply ${action.label} to ${selectedIds.value.size} selected entities?`,
    confirmLabel: action.label,
  })
  if (!ok) return
  void executeAction(actionId, action, triggerEl)
}

// Static (config-pinned) filter properties — used by useUrlFilterSync to
// reject URL filters that would silently override the list's intended scope.
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

// The selected world (`?world=`). A list is one of only two surfaces the API
// can serve under a non-default world — see worldCapablePath in
// internal/dataentry/world.go.
const { world, isWorldBound, worldParam } = useWorld()

// The world's projection note, in the operator's words (`messages.projection`)
// and only on a list of a type that declares faces: a type without faces has
// one state in every world, so nothing on its list is filtered by the world
// and the note would be false. Nothing declared, nothing rendered — the app
// has no sentence of its own for this (TKT-5SZG2L).
const listTypeHasFaces = computed(() => {
  const def = schemaStore.getEntityType(listConfig.value?.entity ?? '')
  return Object.keys(def?.faces ?? {}).length > 0
})
const projectionNote = computed<string>(() => {
  if (!listTypeHasFaces.value) return ''
  const info = world.value ? schemaStore.worlds.get(world.value) : undefined
  return worldText(info?.messages?.projection, { world: world.value })
})

// The create button's destination — see useCreateTarget for why a create
// button carries a world of its own. Whether it SHOWS is `_actions.create`.
const { target: createFormTarget, targetWorld: createWorld } = useCreateTarget(
  computed(() => listConfig.value?.create_form),
  computed(() => listConfig.value?.create_world),
  worldParam,
  computed(() => fromPageQuery(route)),
)

// New opens the create form in a dialog over the list; the new entity then
// opens in the detail panel, as if its row had been clicked.
function refreshAfterCreate() {
  void queryCache.invalidateQueries({ key: entityKeys.list(listConfig.value?.entity ?? '') })
}
// In an entity-page tab a new row is linked to the anchor first, so the
// refresh already shows it in the tab.
const tabScope = usePageTabScope(() => props.pageScope)
// The server withholds the relation order from a reader who may not see the
// order value. The tab then reads in its configured default_sort, as any
// other list does, rather than in store order. Set from the first read that
// asked for the relation order and did not get it.
const orderWithheld = ref(false)
watch(() => props.pageScope, () => (orderWithheld.value = false))
const relationOrdered = computed(() => tabScope.relationOrdered.value && !orderWithheld.value)
const createModal = useCreateModal(async (entity) => {
  await tabScope.linkCreated(entity)
  refreshAfterCreate()
  openPanel(entity)
}, async (entity) => {
  await tabScope.linkCreated(entity)
  refreshAfterCreate()
})
// A section's prefill belongs to the one dialog it opened. New and the `n`
// shortcut open the dialog without one.
watch(createModal.open, (open) => {
  if (!open) createPrefill.value = undefined
})

// The operator's announcement for the world on screen, or '' to announce
// nothing. Config, not data — `/_schema`.worlds is served identically to every
// principal, so this discloses nothing per-caller. Same computed as
// EntityDetail's, deliberately spelled the same way in both.
const worldBanner = computed<string>(
  () => (world.value ? schemaStore.worlds.get(world.value)?.banner : '') || '',
)
const searchBoxRef = ref<InstanceType<typeof SearchBox> | null>(null)
const filterMenuRef = ref<InstanceType<typeof AdHocFilterMenu> | null>(null)

// Sort specs: array of { property, direction } for multi-field sorting
interface SortSpec {
  property: string
  direction: 'asc' | 'desc'
}
const sortSpecs = ref<SortSpec[]>([])

// The rows in the order the table shows them, for the keyboard cursor. A
// grouped list shows them section by section with closed sections skipped,
// which is not necessarily the order they arrived in.
const visibleRows = computed<Entity[]>(() =>
  grouping.grouped.value
    ? grouping.sections.value.filter((s) => !s.collapsed).flatMap((s) => s.items)
    : entities.value,
)

// Computed for keyboard navigation. A grouped list is one page.
const itemCount = computed(() => visibleRows.value.length)
const hasPrevPage = computed(() => !grouping.grouped.value && page.value > 1)
const hasNextPage = computed(
  () => !grouping.grouped.value && (meta.value.has_more || page.value * meta.value.per_page < meta.value.total),
)

// Keyboard navigation
const { selectedIndex, clearSelection } = useListKeyboard({
  itemCount,
  hasPrevPage,
  hasNextPage,
  hasSelection,
  onOpen: (index) => {
    const entity = visibleRows.value[index]
    if (entity) navigateToEntity(entity)
  },
  onEdit: (index) => {
    // The form opens on the row's ADDRESS, face included, so an edit from a
    // world-bound list edits the face the row showed and not its bare id, in
    // the list's world so the form loads the relations that world serves.
    const entity = visibleRows.value[index]
    if (entity && listConfig.value?.edit_form) {
      router.push(editFormRoute(listConfig.value.edit_form, entityRef(entity), worldParam.value))
    }
  },
  onCreate: () => {
    if (createFormTarget.value) createModal.show()
  },
  onSelect: (index) => {
    const entity = visibleRows.value[index]
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

// `group_by:` splits the rows into sections; see useListGrouping.
const groupBy = computed(() => listConfig.value?.group_by)
const grouping = useListGrouping({
  listId: () => props.listId,
  groupBy: () => groupBy.value,
  entityType: () => entityType.value,
  response: () => listQueryRef.value?.data.value,
  filterParams: () => queryParams.value as Record<string, unknown>,
})

// listExportUrlFor builds the export URL for the current list view + chosen
// transform, forwarding the active filter[...] and q params from the URL so the
// export matches what the user is looking at (the backend re-applies the same
// ACL + filter pipeline).
function listExportUrlFor(transform: string): string {
  const cfg = listConfig.value
  if (!cfg) return '#'
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(route.query)) {
    if (!key.startsWith('filter[') && key !== 'q') continue
    if (typeof value === 'string') params.set(key, value)
  }
  // Export is pinned to the view it was invoked from, so it carries the same
  // scope the rows on screen were selected by. Without this an export of a
  // scoped list silently widens to the type's default — a file that looks
  // complete and is not.
  if (cfg.query_scope) params.set('query_scope', cfg.query_scope)
  for (const [key, value] of Object.entries(tabScope.params.value)) {
    if (value) params.set(key, value)
  }
  return listExportUrl(cfg.entity, props.listId, transform, params)
}

// Admin-authored info regions (markdown from data-entry.yaml, sanitized by
// renderMarkdown). `header` is canonical; `description` (previously unused) is a
// fallback for the top slot, used only when `header` is unset. No refResolver
// here — the /_config endpoint carries no mentions map, so bare-ID code spans
// stay inert; standard [text](/entity/ID) links work.
// Lists opt into the legacy `description` alias; kanban deliberately does not.
const headerHtml = computed(() =>
  renderMarkdown(viewHeaderMarkdown(listConfig.value, { allowDescriptionAlias: true }))
)
const footerHtml = computed(() => renderMarkdown(viewFooterMarkdown(listConfig.value)))

// Check if any columns reference relations (need to include related entities)
const hasRelationColumns = computed(() => {
  return listConfig.value?.columns?.some(col => col.relation) || false
})

const hasActions = computed(() => resolvedActions.value.length > 0)
// Rows are selectable when there is something to do with a selection: a
// configured bulk action, a row the principal may delete, or a pile to add
// the rows to (TKT-K3RJLH).
const selectable = computed(() => hasActions.value || anyRowDeletable.value || schemaStore.pilesAvailable)

// What "Add to pile" adds: each selected row's ADDRESS, so a pile holds the
// face the list shows, as the bulk delete does.
function selectedAddresses(): string[] {
  return entities.value.filter((e) => selectedIds.value.has(e.id)).map((e) => entityRef(e))
}

// Build query params. Reads `page` (input state), never `meta` (query
// output) — otherwise the query key would depend on its own result.
const queryParams = computed((): ListParams => {
  // The config's share of the read (list_id, static filters, default sort,
  // scope) comes from the helper the sidebar flyout uses too.
  const params: ListParams = listConfig.value
    ? listBaseParams(props.listId, listConfig.value, { relationOrdered: relationOrdered.value })
    : { per_page: 25, list_id: props.listId }
  // A grouped list is read whole, so it has no page to name.
  if (!groupBy.value) params.page = page.value

  // Add user-selected filters via the shared serializer so EntityList and
  // useScopeNavigation stay in lockstep on the wire format.
  const userParams = filterStateToApiParams(filters.value)
  const paramsRecord = params as Record<string, string | number | undefined>
  for (const [key, value] of Object.entries(userParams)) {
    paramsRecord[key] = value
  }

  // Free-text search: backend intersects ?q= results with the typed list.
  //
  // Sent under a world too. This was suppressed while the index held only
  // DEFAULT-face documents, because a hit under `?world=published` would then
  // have been a draft leaking onto a published surface. The index is keyed per
  // face now and a world IS the search scope (TKT-9KZGJO): the backend matches
  // each entity's prime in the requested world, so the hits are that world's
  // and an entity with no face there cannot appear.
  //
  // `world` is already on `params` above, so nothing extra is needed here —
  // but do not reintroduce a client-side suppression: the scoping is the
  // server's to enforce, and a UI that quietly drops `q` would look like a
  // search returning nothing rather than one that was never run.
  if (searchQuery.value) {
    paramsRecord.q = searchQuery.value
  }

  // The reader's sort replaces the configured default the base carries.
  // Multi-field: the header's shift-click appends. A grouped list keeps its
  // group property in front, so the reader's sort orders rows within a group.
  if (sortSpecs.value.length > 0) params.sort = sortParam(groupedSort(groupBy.value, sortSpecs.value))

  // Include related entities for relation columns — under a world too.
  //
  // This used to be suppressed under a world, because neighbor resolution went
  // through the ungated, default-world reader and a published row would have
  // arrived wrapped in DRAFT neighbors. Two facts in that reasoning are now
  // false: TKT-WRLDAPI item 4 made neighbor resolution WORLD-SCOPED (an
  // included peer is that world's face; a neighbor with no face in the world
  // is absent), and the backend no longer "omits relations entirely on
  // world-bound reads" — it resolves them per neighbour.
  //
  // So suppressing it now costs relation columns their content for no reason.
  // See the same orphan in stores/entities.ts for the general shape.
  if (hasRelationColumns.value) {
    params.include = '*'
  }

  if (worldParam.value) {
    params.world = worldParam.value
  }

  Object.assign(params, tabScope.params.value)

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
//
// A grouped list reads every row up to `max_rows` instead of one page, since a
// page boundary would cut a section in half. Same paged loop the board uses.
const listKey = computed(() => {
  const type = listConfig.value?.entity ?? ''
  return groupBy.value
    ? entityKeys.listAll(type, queryParams.value, groupBy.value.max_rows)
    : entityKeys.listParams(type, queryParams.value)
})
const listQuery = useQuery({
  key: () => listKey.value,
  query: ({ signal }) => {
    const config = listConfig.value
    if (!config) throw new Error(`unknown list: ${props.listId}`)
    if (config.group_by) {
      return listAllEntities(config.entity, queryParams.value, signal, { maxRows: config.group_by.max_rows })
    }
    return listEntities(config.entity, queryParams.value)
  },
  enabled: () => !!listConfig.value,
  placeholderData: (prev) => prev,
})
listQueryRef.value = listQuery
watch(
  () => listQuery.data.value,
  (res) => {
    if (!res || listQuery.isPlaceholderData.value || res.meta.relation_order) return
    if (relationOrdered.value && !queryParams.value.sort) orderWithheld.value = true
  },
)

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

/*
 * A column for the property the list is grouped by repeats its section's
 * heading on every row, so it is left out. Not for date buckets: a bucket
 * spans days, and the column still says which one.
 */
const visibleListColumns = computed(() => {
  const columns = listConfig.value?.columns ?? []
  const grouping = groupBy.value
  if (!grouping || grouping.buckets) return columns
  return columns.filter((column) => column.property !== grouping.property)
})

/**
 * The list's columns in the library's shape, minus the title column which the
 * table renders itself through the `name` slot.
 *
 * Annotated rather than inferred on purpose: a `cell-<key>` slot that matches
 * no column renders EMPTY and Vue reports nothing, so the compiler seeing
 * these as `TableColumn[]` is what turns a future key rename into an error
 * instead of a blank column. See tableColumns.ts.
 */
const tableColumns = computed<TableColumn[]>(() =>
  toTableColumns(visibleListColumns.value).map((column) => ({
    ...column,
    /*
     * A stacked row drops an empty cell so a bare label is never left beside
     * a blank value. The row cannot see inside a slot, so emptiness has to be
     * declared here — and it is rela's own definition: a LOCKED cell is not
     * empty, because the 🔒 is information rather than absence.
     */
    isEmpty: (item: unknown) => {
      const listColumn = listColumnOf(column)
      if (!listColumn) return true
      const entity = item as Entity
      if (isCellInaccessible(entity, listColumn)) return false
      return getFormattedCellValue(entity, listColumn) === ''
    },
    /*
     * Which columns survive `compact: 'compress'`, the one-line layout the
     * list takes beside an open detail panel.
     *
     * An enum badge earns the space and a date does not: the badge is the
     * state you scan the list for, while the rest of the row is in the panel
     * already. A face column is that kind of state too. Derived from the widget hint rather than from config, because
     * `ListColumn` has no way to say it — an operator-facing `primary:` is
     * the extensibility pass, not this one.
     */
    primary: !!listColumnOf(column)?.face || isEnumColumn(column),
  })),
)

/** Whether a column renders as an enum badge, per the widget routing. */
function isEnumColumn(column: TableColumn): boolean {
  const listColumn = listColumnOf(column)
  if (!listColumn?.property) return false
  const hint = columnWidgets.value.get(listColumn.property)?.hint
  return hint?.kind === 'enum' || hint?.kind === 'enum-list'
}

/**
 * The column whose value fills the table's name cell.
 *
 * `link` names the title column when the list config declares one; otherwise
 * it is the first column, the same rule the old card layout and the world
 * badge both used. The badge rides this cell through the `meta` slot, so
 * "the title", "the thing you click" and "the thing the badge sits beside"
 * stay one cell — separating them would put the badge next to an unrelated
 * value.
 */
const titleColumn = computed(
  () => linkColumn.value ?? nameColumn(listConfig.value?.columns ?? []),
)

/**
 * The title column as a `TableColumn`, which gives the name header a sort
 * control. Without it the table renders a plain label and the title — the
 * column users most often sort by — cannot be sorted at all.
 */
const tableNameColumn = computed(() => toTableNameColumn(titleColumn.value))

/**
 * The rows, as the sections the library's table wants.
 *
 * A flat list is one section whose title is never shown. A grouped list has
 * one per group, from useListGrouping. `title` on each item satisfies
 * `CollectionItem`; the cell slots render the real contents, so this value is
 * only a fallback for the table's own name column, which the `name` slot
 * replaces anyway.
 */
function toTableItem(entity: Entity) {
  return { ...entity, title: entityDisplayTitle(entity) }
}
const tableSections = computed(() =>
  grouping.grouped.value
    ? grouping.sections.value.map((section) => ({ ...section, items: section.items.map(toTableItem) }))
    : [{ id: 'entities', title: '', items: entities.value.map(toTableItem) }],
)

// Dragging rows on a tab shown in relation order. Only while the reader
// sees that order: their own sort, or a grouping, puts the rows elsewhere,
// and a row dropped there would jump back on the next read.
const reorder = useListReorder({
  order: () => meta.value.relation_order,
  active: () => relationOrdered.value && sortSpecs.value.length === 0 && !grouping.grouped.value,
  rows: () => entities.value,
  key: () => listKey.value,
  type: () => listConfig.value?.entity ?? '',
})
const reorderableRow = computed(() => (reorder.reorderable.value ? () => true : undefined))

function onSectionCollapse(section: { id: string }, collapsed: boolean) {
  grouping.setCollapsed(section.id, collapsed)
}

/** rela's sort specs in the library's shape, primary key first. */
const tableSort = computed(() =>
  sortSpecs.value.map((spec) => ({ key: `prop:${spec.property}`, dir: spec.direction })),
)

/**
 * Bridges the table's sort gesture onto rela's existing sort cycle.
 *
 * The header reports WHAT was pressed and leaves the meaning to us, so the
 * asc → desc → removed cycle and shift-to-append stay rela's, which matters
 * because an operator's `default_sort` can seed a multi-key sort the user
 * then edits. `handleSort` already implements that cycle against a
 * MouseEvent, so this reconstructs only the one bit it reads.
 *
 * Reaching the table's own button also fixes a real defect: rela's sort
 * control was a bare `<th>` with a click handler, so it had no keyboard route
 * at all and announced no sort state.
 */
function onSortClick(column: TableColumn, event: SortClickEvent) {
  const property = listColumnOf(column)?.property
  if (!property) return
  handleSort(property, { shiftKey: event.additive } as MouseEvent)
}

/**
 * Per-row attributes. Carries `data-entity-id`, which e2e uses to address a
 * row: a git-crypt entity with every column locked renders no link, so the
 * href is not a usable key for it.
 *
 */
function rowAttrs(item: { id: string }) {
  return { 'data-entity-id': item.id }
}

/** A table row carries the entity id, so selection maps straight across. */
function onTableToggle(item: { id: string }) {
  toggleSelection(item.id)
}

function onTableToggleAll(_section: unknown, checked: boolean) {
  if (checked) selectAll(entities.value.map((e) => e.id))
  else clearActionSelection()
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

// The column whose cell carries the row's link, if the list config declares
// one. Hoisted out of entityTarget so the template can share the same
// answer: the badge below rides the row's TITLE, and "the title" and "the
// thing you click" must be the same cell or the badge would sit beside an
// unrelated value.
const linkColumn = computed(() => listConfig.value?.columns?.find((col) => col.link))

// entityTarget is the SINGLE source of truth for a row's destination — used both
// by the row's plain-click push AND by the title cell's RouterLink `to`.
// Building the href separately would silently drop the query below: the tab
// would open on an unscoped detail page where prev/next navigation is dead and
// the back target is wrong, while still rendering a valid-looking page.
// Nil: returns undefined when no safe internal path resolves (empty entity
// type, or a cellLink that is not a same-origin path); the row then renders no
// link and the click is inert.
function entityTarget(entity: Entity): RouteLocationRaw | undefined {
  // Build query params to preserve navigation context.
  // Filters are already in `route.query` via useUrlFilterSync — we just
  // forward all `filter[*]` entries unchanged so the bracket format is the
  // single source of truth (no legacy `filter_*` underscore form).
  const query: Record<string, string | string[]> = {
    from: props.listId,
    scope: `list:${props.listId}`,
    // Inside a page tab, Back on the entity returns to the tab.
    ...fromPageQuery(route),
  }

  // Include sort if active
  if (sortSpecs.value.length > 0) {
    query.sort = sortSpecs.value
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  } else {
    const sort = defaultSortParam(listConfig.value, relationOrdered.value)
    if (sort) query.sort = sort
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

  // The world rides along too (TKT-6NCSSC). A row followed from a world-bound
  // list must resolve in that world; without this the reader lands on the
  // DEFAULT face of an entity whose row they just saw carrying a fallback
  // badge. Same rule the detail page's neighbour links and historyTarget follow.
  if (worldParam.value) {
    query.world = worldParam.value
  }

  // Check for column-level link first (use first column with link)
  const columnLink = linkColumn.value?.link
    ? resolveLinkTarget(linkColumn.value.link, entity.type, entity.id)
    : ''

  // entityDetailHref returns columnLink when set, otherwise the
  // entity-route path.
  const path = entityDetailHref(
    { id: entity.id, type: entity.type },
    { cellLink: columnLink },
  )
  if (!path || !safeInternalHref(path)) return undefined
  return { path, query }
}

// Row targets, computed ONCE per entity rather than per template reference
// (RR-SYFX1B -- the same reason columnWidgets below is per-column, not
// per-cell). entityTarget walks route.query, maps sortSpecs and scans columns,
// and the template reads it twice per row (the v-else-if and the :to), so at 25
// rows that was 50 traversals per render, re-running on every reactive tick
// including hover-driven selectedIndex changes.
// navigateToEntity backs the keyboard Enter on the cursor row. The row's
// MOUSE navigation is the RouterLink in the table's `name` slot, which the
// library stretches over the whole row — so a pointer click, cmd-click and
// middle-click are all the anchor's own default action, and none of them
// reach this. Nil: does nothing when no safe internal path resolves.
function navigateToEntity(entity: Entity) {
  const target = rowTargets.value.get(entity.id) ?? entityTarget(entity)
  if (!target) return
  router.push(target)
}

/*
 * Open the panel on a row. A `replace`, not a `push`: stepping through rows
 * is browsing one list, so it should not bury the screen the reader arrived
 * from under one history entry per row they glanced at. Back returns to
 * wherever they came from, which is what they mean by it.
 */
function openPanel(entity: Entity) {
  if (entity.id === selectedEntityId.value) return
  void router.replace({ query: { ...route.query, selected: entity.id } })
}

function closePanel() {
  const query = { ...route.query }
  delete query.selected
  void router.replace({ query })
}

/*
 * The canonical item view. A panel previews a row of this list; the entity's
 * own page is the address you share when you mean the entity rather than
 * this view of it. Pushed, because leaving the list IS a place change, and
 * it carries the row's full navigation context (scope, sort, filters) the
 * same way a row click does.
 */
function expandPanel() {
  const entity = selectedEntity.value
  if (!entity) return
  const target = rowTargets.value.get(entity.id) ?? entityTarget(entity)
  if (target) router.push(target)
}

/*
 * A plain click on a row opens the panel instead of following the link.
 *
 * Only a plain one. Cmd/ctrl-click, shift-click and middle-click are how a
 * reader asks for a new tab or window, and the destination they expect is
 * the entity's own page, not this list with a query param. Those stay the
 * anchor's own default action, untouched — which is also why the row keeps
 * being a real RouterLink with a real href: it is what gives the row a
 * hover URL preview and a working "copy link address".
 *
 * `button !== 0` never actually arrives here (a middle click fires `auxclick`,
 * not `click`) but is checked anyway, because the cost of being wrong about
 * that is hijacking a new-tab gesture.
 */
function onRowClick(entity: Entity, event: MouseEvent) {
  if (event.defaultPrevented) return
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  if (event.button !== 0) return
  // Capture phase: RouterLink's own click handler is on the anchor and would
  // otherwise still run and navigate. preventDefault alone does not stop it,
  // because it checks its own guard conditions rather than defaultPrevented.
  event.preventDefault()
  event.stopPropagation()
  openPanel(entity)
}

/*
 * The row named by `?selected=`, or null when the param names a row this
 * page does not hold. Both cases are reachable from a shared link: a stale
 * id, or a valid row on another page of the same list.
 */
const selectedEntity = computed(() => {
  const id = selectedEntityId.value
  if (!id) return null
  return entities.value.find((e) => e.id === id) ?? null
})

/*
 * Feed the shell's panel outlet. Driven by the URL rather than by the click
 * handler, so a deep link, a back step and a click all arrive the same way.
 *
 * Waits for the row to be present before opening: the id alone would be
 * enough to render EntityDetail, but a `?selected=` that no row on this page
 * matches is more likely a stale link than a request to show a hidden row,
 * and opening on it would report an error the reader cannot act on from
 * here. Silence leaves the list usable and the param harmless.
 */
watch(
  [selectedEntity, () => listConfig.value?.entity],
  ([entity, entityType]) => {
    if (!entity || !entityType) {
      panel.clear()
      return
    }
    panel.show({
      component: EntityDetailPanel,
      props: {
        entityType: entity.type || entityType,
        entityId: entityRef(entity),
        onClose: closePanel,
        onExpand: expandPanel,
      },
      mode: 'inline',
    })
  },
  { immediate: true },
)

const rowTargets = computed(() => {
  const byId = new Map<string, RouteLocationRaw | undefined>()
  for (const entity of entities.value) {
    byId.set(entity.id, entityTarget(entity))
  }
  return byId
})

// Widget resolution for property cells, keyed by column property name and
// computed ONCE per column rather than per cell (RR-UD2A -- the same reason
// PropertyDisplay precomputes `rows`). resolve()/resolveFromHint() walk a Map
// and can console.warn; doing that per cell would be one lookup and one
// potential warning per row per render.
//
// Relation columns are absent from this map on purpose: they have no
// PropertyDef and there is no relation widget, so they stay on the string
// path in getFormattedCellValue.
const columnWidgets = computed(() => {
  const byProperty = new Map<string, { component: Component; hint: DenseRoutingHint }>()
  const type = entityType.value
  if (!type) return byProperty
  for (const column of listConfig.value?.columns ?? []) {
    if (!column.property || byProperty.has(column.property)) continue
    const hint = densePropertyRoutingHint(type.properties[column.property], column.property)
    byProperty.set(column.property, { component: defaultRegistry.resolveFromHint(hint), hint })
  }
  return byProperty
})

// The widget for a cell, or undefined when the cell should NOT render one:
// a relation column (no PropertyDef, no relation widget) or an empty value
// (widgets may render a "no value" placeholder that cells must not show --
// see isDenseEmpty). Both fall through to the plain string span.
function cellWidget(
  entity: Entity,
  column: { property?: string; relation?: string; direction?: 'outgoing' | 'incoming'; face?: boolean }
) {
  if (!column.property) return undefined
  const entry = columnWidgets.value.get(column.property)
  if (!entry) return undefined
  return isDenseEmpty(getCellValue(entity, column)) ? undefined : entry
}

interface ResolvedCell {
  component: Component
  propertyName: string
  modelValue: unknown
}

// One resolved cell: the widget to render plus the value already shaped the
// way that widget wants. Returns undefined when the cell must fall back to the
// plain string span (relation column, or an empty value -- see cellWidget).
//
// Memoized per (entity, column). The template reads it four times per cell
// (v-else-if, :is, and two bindings); without the cache each read redoes the
// Map lookup and re-formats the value.
//
// Safe against staleness because entity objects are copy-on-write: both
// optimistic paths in queries/optimisticList.ts rebuild the changed entity
// via `data.map(e => e.id === id ? update(e) : e)`, and a refetch parses
// fresh objects. A mutated cell therefore always arrives as a NEW identity
// and misses the cache. If an entity ever starts being mutated in place,
// this cache goes stale silently -- keyed on identity, it cannot detect it.
//
// WeakMap so a dropped row's entry is collected with the row. The inner key
// is the column object, stable across renders because listConfig.columns is
// the same array.
const cellCache = new WeakMap<Entity, Map<object, ResolvedCell | undefined>>()

function resolveCell(
  entity: Entity,
  column: { property?: string; relation?: string; direction?: 'outgoing' | 'incoming'; face?: boolean }
): ResolvedCell | undefined {
  let perEntity = cellCache.get(entity)
  if (!perEntity) {
    perEntity = new Map()
    cellCache.set(entity, perEntity)
  }
  if (perEntity.has(column)) return perEntity.get(column)

  const entry = cellWidget(entity, column)
  const resolved: ResolvedCell | undefined = entry
    ? {
        component: entry.component,
        propertyName: entry.hint.propertyName,
        // Passthrough widgets render String(value); everything else owns its
        // display formatting and wants the stored value.
        modelValue: entry.hint.preformatted
          ? getFormattedCellValue(entity, column)
          : getCellValue(entity, column),
      }
    : undefined
  perEntity.set(column, resolved)
  return resolved
}

// isCellInaccessible reports whether the cell's underlying property is
// listed in the entity's inaccessible array (e.g. git-crypt encrypted).
// Such cells render a lock indicator instead of the value.
function isCellInaccessible(entity: Entity, column: { property?: string }): boolean {
  if (!entity.inaccessible || entity.inaccessible.length === 0) return false
  if (!column.property) return false
  return entity.inaccessible.some((f) => f.name === column.property)
}

// relationCellKey returns the wire key in entity.relations that holds a
// relation column's target IDs. Outgoing columns read the relation type
// directly; incoming columns read the relation's inverse key — the declared
// inverse name if the metamodel has one, else the `<relation>_inverse`
// fallback. This mirrors the backend's inverseRelationKey (see MECHANISM.md);
// the two must stay in lockstep.
function relationCellKey(relation: string, direction?: 'outgoing' | 'incoming'): string {
  if (direction === 'incoming') {
    return schemaStore.getInverseName(relation) ?? `${relation}_inverse`
  }
  return relation
}

// The label of the face this row was served in. The server sends it as
// `_world.face` on each row under a world; under the default world there is
// none, and the cell stays empty.
function faceLabel(entity: Entity): string {
  const face = entity._world?.face
  if (!face) return ''
  return schemaStore.getEntityType(entity.type)?.faces?.[face]?.label || face
}

function getFormattedCellValue(
  entity: Entity,
  column: { property?: string; relation?: string; direction?: 'outgoing' | 'incoming'; face?: boolean },
): string {
  // For relation columns, resolve IDs to titles using included entities.
  // Outgoing edges are serialized under the relation type; incoming edges
  // under the relation's INVERSE key (matching the backend serializer, see
  // MECHANISM.md). The included map carries both target and source entities
  // because the list is fetched with ?include=*.
  if (column.face) return faceLabel(entity)
  if (column.relation) {
    const key = relationCellKey(column.relation, column.direction)
    const relationIds = entity.relations?.[key] || []
    const titles = relationIds.map((id) => {
      const included = includedEntities.value[id]
      return included ? entityDisplayTitle(included) : id
    })
    return titles.join(', ')
  }

  const value = getCellValue(entity, column)
  // Pass the user's effective display zone so datetime cells honor the
  // Settings display-timezone preference, matching the form widget (RR-K3WEW2).
  return formatCellValue(value, column.property, entityType.value, uiStore.effectiveTimezone)
}

// Reconcile input `page` with the server's returned page. If the backend
// clamps an out-of-range page (e.g. requested 5, only 3 exist), adopt the
// server's value so the query key, keyboard nav, and Pagination controls
// all agree on one page-of-truth.
watch(
  () => listQuery.data.value?.meta.page,
  (serverPage) => {
    if (serverPage !== undefined && serverPage !== page.value) {
      page.value = serverPage
    }
  }
)

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
  <div v-if="listConfig" class="entity-list" :data-testid="`page-state-${pageState}`">
    <!--
      Painted by the app shell, above BOTH this list and the detail panel
      beside it, so opening a panel no longer cuts the page title in half.
      Declared here so the refs and handlers still resolve from this
      component; see PageHeaderContent.
    -->
    <PageHeaderContent :title="listConfig.title || listConfig.entity">
      <template #actions>
        <BackButton v-if="backTarget" :target="backTarget" />
        <ExportMenu :url-for="listExportUrlFor" />
        <RlButton
          v-if="createFormTarget && canCreate()"
          :as="RouterLink"
          :to="createFormTarget"
          variant="primary"
          icon="plus"
          @click.capture="createModal.onClick"
        >
          New
          <template #trailing><RlKbd keys="N" /></template>
        </RlButton>
      </template>

      <template #tools>
        <!--
          Rendered under a world too. It was OMITTED while search could not be
          world-scoped, because a box that returned default-world hits on a
          published page would have surfaced drafts. Search is scoped to the
          world now, so the affordance is real: it searches the same faces this
          list shows, and an entity absent from the world is absent from its
          search. The banner below says so.
        -->
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
      </template>
    </PageHeaderContent>

    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="headerHtml" class="view-info view-info--top" v-html="headerHtml"/>

    <!--
      Gated on `!loadError`: the banner ASSERTS "you are looking at world X",
      and it must not make that claim over an error state. An unknown world is
      a 400 (unknown_world), so a banner rendered beside the error read
      "Showing the nonexistent world" above a message saying no such world is
      declared — the page contradicting itself, and in the direction that
      reads as a designed property rather than a mistake.
    -->
    <!--
      Both halves are operator config: the ANNOUNCEMENT (`banner:` on the
      world) and the NOTE (`messages.projection`, rendered only on a list of a
      faced type). Neither declared: no banner at all.
    -->
    <WorldBanner v-if="isWorldBound && !loadError && (worldBanner || projectionNote)" :label="worldBanner">
      {{ projectionNote }}
    </WorldBanner>

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
      <!--
        A grouped list loads up to its row cap and no further, so past the cap
        the sections are a prefix of the list. Said once, above them, rather
        than left for the reader to infer from counts that look complete.
      -->
      <RlBanner
        v-if="grouping.truncated.value && !loading"
        tone="warning"
        data-testid="group-truncated"
      >
        Showing the first {{ entities.length }} of {{ grouping.total.value }} rows. Filter the list to see the rest.
      </RlBanner>
      <RlStatusRegion v-if="loading">Loading...</RlStatusRegion>

      <div v-else-if="loadError" class="empty-state load-error">
        <p>{{ loadError }}</p>
        <RlButton variant="secondary" @click="listQuery.refetch()">Retry</RlButton>
      </div>

      <!--
        The two cases need different words and different offers, which is the
        distinction RlEmptyState is built around: a filter matched nothing and
        wants widening, or nothing exists yet and wants creating.
      -->
      <RlEmptyState
        v-else-if="entities.length === 0"
        class="empty-state"
        :icon="searchQuery ? 'search' : 'inbox'"
        :title="
          searchQuery
            ? `No matches for \u201c${searchQuery}\u201d.`
            : `No ${listConfig.entity}s found.`
        "
      >
        <template #actions>
          <RlButton v-if="searchQuery" variant="secondary" @click="handleSearchUpdate('')">
            Clear search
          </RlButton>
          <RlButton
            v-else-if="createFormTarget && canCreate()"
            :as="RouterLink"
            :to="createFormTarget"
            variant="secondary"
            @click.capture="createModal.onClick"
          >
            Create one
          </RlButton>
        </template>
      </RlEmptyState>

      <template v-else>
      <!--
        One table for every width: RlTable reshapes its own rows once they no
        longer fit, which is what rela's separate `v-if="isMobile"` card
        branch used to do by hand. It measures ITSELF, not the window, so an
        open detail panel narrows it without the viewport changing.

        `compress` rather than the default `stack`: this list is a master list
        beside that panel, so a row is an index entry into fields the panel is
        already showing. Stacking them would repeat the panel at three times
        the height. Only columns marked `primary` survive — see tableColumns.
      -->
      <RlTable
        :sections="tableSections"
        :columns="tableColumns"
        :sort="tableSort"
        :selected-ids="selectable ? selectedIds : undefined"
        :selected-id="selectedEntityId ?? undefined"
        :cursor-id="visibleRows[selectedIndex]?.id"
        :row-attrs="rowAttrs"
        :name-column="tableNameColumn"
        :show-section-header="grouping.grouped.value"
        :show-add="!!createFormTarget && canCreate()"
        compact="compress"
        :reorderable="reorderableRow"
        @reorder="reorder.onReorder"
        @sort-click="onSortClick"
        @toggle="onTableToggle"
        @toggle-all="onTableToggleAll"
        @add="onTableAdd"
        @collapse="onSectionCollapse"
      >
        <!--
          The row's primary control. A plain link with no class of its own:
          the row wraps this in a `display: contents` span and stretches ONE
          overlay over whatever it gets, so rela keeps cmd/middle-click and a
          hover URL preview without the `.row-link::after` it used to own.
          Two stretched overlays would fight, which is why this slot exists.
        -->
        <template #name="{ item }">
          <span v-if="titleColumn && isCellInaccessible(item as Entity, titleColumn)" class="inaccessible-cell" title="inaccessible">🔒</span>
          <RouterLink
            v-else-if="rowTargets.get(item.id)"
            :to="rowTargets.get(item.id)!"
            @click.capture="onRowClick(item as Entity, $event)"
          >
            <component
              :is="resolveCell(item as Entity, titleColumn!)!.component"
              v-if="titleColumn && resolveCell(item as Entity, titleColumn)"
              :model-value="resolveCell(item as Entity, titleColumn)!.modelValue"
              :mode="'display'"
              :property-name="resolveCell(item as Entity, titleColumn)!.propertyName"
              :entity-type="listConfig.entity"
            />
            <template v-else>{{ titleColumn ? getFormattedCellValue(item as Entity, titleColumn) : item.title }}</template>
          </RouterLink>
          <span v-else>{{ titleColumn ? getFormattedCellValue(item as Entity, titleColumn) : item.title }}</span>
        </template>

        <!--
          Per-ROW face provenance, beside the title. Each row resolves through
          the world independently, so one list can mix a first-choice hit and
          a stand-in, and the two are otherwise byte-identical — which is the
          whole reason the badge exists. WorldBadge is a no-op for a
          first-choice hit and under the default world, so a typical list
          shows none at all.
        -->
        <template #meta="{ item }">
          <WorldBadge :world="(item as Entity)._world" :entity-type="(item as Entity).type" />
        </template>

        <!--
          Every cell goes through the generic slot: a rela cell is a property
          widget, a relation chip, or a lock for a git-crypt-inaccessible
          value, never the plain string `column.field` would cover.
        -->
        <template #cell="{ item, column }">
          <span
            v-if="listColumnOf(column!) && isCellInaccessible(item as Entity, listColumnOf(column!)!)"
            class="inaccessible-cell"
            title="inaccessible"
          >🔒</span>
          <span v-else class="list-cell">
            <component
              :is="resolveCell(item as Entity, listColumnOf(column!)!)!.component"
              v-if="listColumnOf(column!) && resolveCell(item as Entity, listColumnOf(column!)!)"
              :model-value="resolveCell(item as Entity, listColumnOf(column!)!)!.modelValue"
              :mode="'display'"
              :property-name="resolveCell(item as Entity, listColumnOf(column!)!)!.propertyName"
              :entity-type="listConfig.entity"
            />
            <template v-else>{{ listColumnOf(column!) ? getFormattedCellValue(item as Entity, listColumnOf(column!)!) : '' }}</template>
          </span>
        </template>

      </RlTable>
      </template>

      <!--
        The selection's actions float over the bottom of the list rather than
        replacing the column headers. A zero-height sticky anchor holds the
        bar at the bottom of the visible pane while the list scrolls beneath.
      -->
      <div v-if="selectable" class="entity-list__bulk-anchor">
        <RlBulkActionBar
          :count="selectedIds.size"
          :label="entityNoun.singular"
          :plural-label="entityNoun.plural"
          @clear="clearActionSelection"
        >
          <!--
            `confirm` is the only destructive signal an action config carries,
            so it is what picks the danger tone.
          -->
          <RlButton
            v-for="{ id, config } in resolvedActions"
            v-show="anySelectedAllowsUpdate()"
            :key="id"
            variant="secondary"
            size="sm"
            :tone="config.confirm ? 'danger' : 'default'"
            :disabled="actionProcessing"
            data-testid="bulk-action"
            @click="(e: MouseEvent) => triggerAction(id, config, e)"
          >
            <RlKbd v-if="config.key" :keys="config.key" />
            {{ config.label }}
          </RlButton>
          <!--
            No confirm: the delete can be undone from the toast it raises.
            Hidden when no selected row may be deleted.
          -->
          <RlButton
            v-if="deletableSelection.length > 0"
            variant="secondary"
            size="sm"
            tone="danger"
            :disabled="deleting"
            data-testid="bulk-delete"
            @click="deleteSelected"
          >
            Delete
          </RlButton>
          <AddToPileMenu
            :addresses="selectedAddresses"
            variant="primary"
            placement="top"
            @added="clearActionSelection"
          />
        </RlBulkActionBar>
      </div>

      <Pagination
        v-if="!grouping.grouped.value && meta.total > meta.per_page"
        :meta="meta"
        @page-change="handlePageChange"
      />
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="footerHtml" class="view-info view-info--bottom" v-html="footerHtml"/>

    <!--
      No "Create & add another" when a section's Add opened the dialog: the
      form clears itself for the next record and the section's value would go
      with it, so the second row would land outside the section.
    -->
    <InlineCreateFormModal
      v-if="createModal.open.value && listConfig.create_form"
      :show="true"
      :form-id="listConfig.create_form"
      :entity-type="listConfig.entity"
      :world="createWorld || undefined"
      :prefill="createPrefill"
      :add-another="!createPrefill"
      @close="createModal.close"
      @created="createModal.created"
      @created-another="createModal.createdAnother"
    />
  </div>

  <RlStatusRegion v-else tone="error">
    The list "{{ listId }}" does not exist in the configuration.
  </RlStatusRegion>
</template>

<style scoped>
.entity-list__bulk-anchor {
  position: sticky;
  bottom: 0;
  height: 0;
  z-index: var(--rl-z-sticky-raised);
}

/*
 * One line per cell, as the library's own text cells are: a long value
 * ellipsizes instead of wrapping in its fixed-width column and making the
 * row as tall as the text. The detail view shows the whole value.
 */
.list-cell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-cell :deep(*) {
  white-space: nowrap;
}

.inaccessible-cell {
  color: var(--color-text-muted, #888);
  font-style: italic;
  cursor: help;
}

/* Info-region styles (.view-info) live in styles/view-info.css — shared with
   KanbanView so both views render admin-authored markdown identically. */

/*
 * No width cap. A master list is read by scanning down one column and across
 * to the state beside it, so the columns want the whole pane; a 1200px cap
 * left the table short of its own container and the rows re-wrapped long
 * before the pane ran out of room.
 */


.filter-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  background: var(--rl-color-bg-hover);
  border: 1px solid var(--rl-color-border);
  border-radius: 16px;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
}

.filter-chip.removable {
  gap: var(--rl-space-1);
  padding-right: 4px;
  background: color-mix(in srgb, var(--rl-color-accent) 15%, transparent);
  border-color: color-mix(in srgb, var(--rl-color-accent) 30%, transparent);
  color: var(--rl-color-accent);
}

.chip-remove {
  background: none;
  border: none;
  cursor: pointer;
  font-size: var(--rl-font-size-md);
  line-height: 1;
  padding: 0 4px;
  color: inherit;
  opacity: 0.7;
}

.chip-remove:hover {
  opacity: 1;
}

.adhoc-filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
  margin-bottom: 12px;
}

/*
 * Layout only. The table paints its own surface, so a card colour here showed
 * through as a lighter frame around a darker table.
 *
 * It also carries the full bleed: the table reaches the pane's edges while the
 * title, filter chips and banners above it keep the page gutter. A bordered
 * table inset from its own container reads as a card floating in the page
 * rather than as the page's content, and the header band above it is already
 * full-bleed — so the inset showed up as the table's rules failing to line up
 * with the header's underline.
 *
 * No `overflow: hidden` here. It was clipping the corners of a rounded card
 * this stopped being, and it would now crop the bleed back off.
 */
.list-content {
  margin-inline: calc(-1 * var(--rl-page-gutter-left)) calc(-1 * var(--rl-page-gutter-right));
}

/*
 * ...but only the table wants it. The filter bar, the loading and empty
 * states and the pager are prose-width chrome and stay lined up with the
 * title above them, so they take the gutter back on their own terms. Same
 * shape as RlDetailPanel's `.rl-detail-panel-bleed`, inverted: there the body
 * pads and one child escapes, here the row bleeds and the others opt back in.
 */
.list-content > :not(.rl-table) {
  padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: var(--rl-space-4);
  color: var(--rl-color-text-muted);
}

/* Stretched link (the Bootstrap `.stretched-link` pattern). A <tr> may not be
   or contain an <a> at the row level, so the first cell's link is expanded over
   the whole row: cmd/ctrl-click, middle-click, right-click "Open in new tab"
   and the hover URL preview all work anywhere on the row, with exactly ONE link
   per row in the accessibility tree.
   Known trade-off (accepted, TKT-3CSZRG): the overlay sits above the row's text,
   so text selection within a row is not possible. */
/* Nested controls must stay above the overlay or they become unclickable. */
.select-column,
.select-cell {
  width: 32px;
  text-align: center;
}

.select-cell input[type="checkbox"],
.select-column input[type="checkbox"] {
  cursor: pointer;
  accent-color: var(--rl-color-accent, #6366f1);
}

.actions-column {
  width: 40px;
}

.actions-cell {
  width: 40px;
  white-space: nowrap;
}

.table-scroll-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
</style>
