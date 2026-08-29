<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, onUnmounted } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import { useScopeNavigation } from '@/composables'
import { useBackTarget } from '@/composables/useBackTarget'
import { isCancelledFetch } from '@/composables/usePageData'
import { fetchView, getCommands, getErrorMessage } from '@/api'
import type { ViewEntity, ViewResponse, ViewSection, ViewSectionField } from '@/api'
import type { Entity } from '@/types'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { useAutoSave } from '@/composables/useAutoSave'
import { toggleCheckboxInSource } from '@/utils/checkboxToggle'
import type { Command } from '@/types'
import { getEditFormId } from '@/types'
import { entityDetailHref } from '@/utils/entityRoute'
import { computeActionAllowed } from '@/utils/affordancesWarning'
import { isInputFocused } from '@/utils/dom'
import { isAnyModalOpen } from '@/composables/modalStack'
import {
  renderMarkdown,
  renderMermaidDiagrams,
  renderPlantUMLDiagrams,
  getCheckboxStats,
  type EntityRefResolver,
} from '@/utils/markdown'
import BackButton from '@/components/common/BackButton.vue'
import Badge from '@/components/common/Badge.vue'
import InaccessibleField from '@/components/common/InaccessibleField.vue'
import PropertyDisplay from '@/components/common/PropertyDisplay.vue'
import type { PropertyItem } from '@/components/common/PropertyDisplay.vue'
import { defaultRegistry } from '@/widgets/registry'
import { viewFieldRoutingHint } from '@/widgets/viewRouting'
import type { WidgetRoutingHint } from '@/widgets/types'
import type { PropertyDef } from '@/types'
import type { Component } from 'vue'
import DocumentsPanel from '@/components/entity/DocumentsPanel.vue'
import CommandModal from '@/components/entity/CommandModal.vue'
import ExportMenu from '@/components/entity/ExportMenu.vue'
import { entityExportUrl } from '@/api/transforms'
import SectionEditForm, { type SectionEditField } from '@/components/forms/SectionEditForm.vue'
import AutoSaveIndicator from '@/components/forms/AutoSaveIndicator.vue'
import {
  buildSectionEditFields as buildSectionEditFieldsPure,
  sectionShouldRouteToInlineEdit as sectionShouldRouteToInlineEditPure,
  applyPropertyToEntry,
  applyPropertyToRow,
  rowShouldRouteToInlineEdit as rowShouldRouteToInlineEditPure,
} from './sectionEditFields'
import type { AutoSaveErrorInfo } from '@/composables/useAutoSave'
import { useConfirm, withConfirmError } from '@/composables/useConfirm'
import { useDelayedPending } from '@/composables/useDelayedPending'
import { beginRouteLoad } from '@/composables/useNavigationPending'
import { PENDING_TIMINGS } from '@/composables/pendingTimings'

const props = withDefaults(
  defineProps<{
    entityType: string
    entityId: string
    /**
     * Hide the header action toolbar (commands, Edit, History, Delete).
     *
     * For embedding this component as a READ surface — the calendar's preview
     * modal — where the host supplies its own actions. Without it the modal
     * shows two Edit buttons and a destructive Delete one click from a
     * calendar chip, which is not what a preview is for.
     *
     * Suppresses the buttons only. Permission checks, keyboard handlers and
     * every other behaviour are untouched, so this cannot widen what a
     * principal may do.
     */
    hideActions?: boolean
  }>(),
  { hideActions: false }
)

const router = useRouter()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { confirm } = useConfirm()

// Scope navigation (prev/next within a list) and back affordance
// (return_to / from precedence). Two parallel concerns: scope-nav walks
// a list; backTarget answers "where do I go back to". Both can be active
// at once.
const { scopeNav, loadScopeNav, navigateScope } = useScopeNavigation(() => props.entityId)
const backTarget = useBackTarget()

// State
const loading = ref(true)
const error = ref<string | null>(null)
const viewData = ref<ViewResponse | null>(null)

// Prev/next within a list re-fetches this component in place. Blanking the
// page to a centred spinner on every step was the worst layout shift in
// the app — the article collapsed to ~140px and sprang back — and on a
// fast connection it read as a flicker, not a loading state.
//
// Two rules apply, in order. (1) Keep previous content: while an entity is
// already on screen, stepping to a neighbour holds it until the next
// resolves, so nothing is shown at all. (2) On a genuine cold load, the
// gate still suppresses anything quicker than the navigation delay.
const showBlockLoader = useDelayedPending(
  () => loading.value && !viewData.value,
  {
    delay: PENDING_TIMINGS.navDelayMs,
    minDuration: PENDING_TIMINGS.navMinDurationMs,
  }
)
const commands = ref<Command[]>([])
const showOverflowMenu = ref(false)

const commandModalRef = ref<InstanceType<typeof CommandModal> | null>(null)

function openHistory() {
  router.push(`/history/${props.entityType}/${props.entityId}`)
}
const contentRef = ref<HTMLElement | null>(null)

// Computed
const typeDef = computed(() => schemaStore.getEntityType(props.entityType))
const editFormId = computed(() => getEditFormId(schemaStore, props.entityType))

const entry = computed(() => viewData.value?.entry || null)
const entryTitle = computed(() => {
  if (!entry.value) return props.entityId
  return entityDisplayTitle(entry.value)
})

// inaccessibleByName maps each inaccessible field's name to its reason
// (e.g. 'git-crypt'), letting consumers render per-property tooltips
// without re-walking the array. Sourced from the view-rendered entry —
// the backend's entityToV1 (api_v1.go) attaches inaccessible[] when the
// underlying entity is locked.
const inaccessibleByName = computed<Map<string, string>>(() => {
  const m = new Map<string, string>()
  for (const f of entry.value?.inaccessible || []) {
    m.set(f.name, f.reason)
  }
  return m
})

const isInaccessible = computed(() => (entry.value?.inaccessible?.length ?? 0) > 0)

// Affordance gates: `_actions` map from the server. `false` → hide;
// anything else → render. See frontend/src/utils/affordancesWarning.ts.
const canUpdate = computeActionAllowed(entry, 'update')
const canDelete = computeActionAllowed(entry, 'delete')

// The entry's content section gets a custom renderer (mermaid + interactive
// checkboxes) instead of the generic section render-path. Other content
// sections — content cards from configured views — use the generic path.
//
// Shared predicate so the section-finding (entryContentSection) and section-
// mutating (handleCheckboxToggle) paths stay in sync — drift between the two
// would silently update the wrong section's content.
function isEntryContentSection(s: { display?: string; hasContent?: boolean; content?: string }) {
  return s.display === 'content' && s.hasContent === true && !!s.content
}
const entryContentSection = computed(() => {
  const sections = viewData.value?.sections
  if (!sections) return null
  return sections.find(isEntryContentSection) || null
})

const checkboxStats = computed(() => {
  const c = entryContentSection.value?.content
  return c ? getCheckboxStats(c) : null
})

// refResolver wires the server-supplied mentions map into the markdown
// renderer so bare-ID code spans become titled in-app links. Null when
// the response carries no mentions; renderMarkdown then behaves exactly
// as before (no resolver, no rewrite).
const refResolver = computed<EntityRefResolver | undefined>(() => {
  const mentions = viewData.value?.mentions
  if (!mentions) return undefined
  return (id) => {
    const m = mentions[id]
    if (!m) return null
    return {
      type: m.type,
      title: m.title,
      inaccessible: m.inaccessible,
      inaccessibleReason: m.inaccessible_reason,
    }
  }
})

const renderedEntryContent = computed(() =>
  entryContentSection.value
    ? renderMarkdown(entryContentSection.value.content || '', {
        refResolver: refResolver.value,
        interactive: true,
      })
    : ''
)

// Re-renders re-process mermaid diagrams inside the content body. Checkbox
// clicks are handled via delegation on contentRef (see contentClick), which
// doesn't need re-binding on every content swap.
//
// flush: 'post' so the watch fires after the v-html update has landed in the
// DOM — otherwise contentRef.value is the previous (or null) element and
// mermaid diagrams render on stale content. nextTick alone is not enough
// because the watch can fire in the same tick as loading→entry visibility.
watch(
  renderedEntryContent,
  async () => {
    if (contentRef.value) {
      await renderMermaidDiagrams(contentRef.value)
      renderPlantUMLDiagrams(contentRef.value, schemaStore.app?.plantuml_server_url)
    }
  },
  { flush: 'post' }
)

// Content-only useAutoSave instance. EntityDetail does not own a form
// surface, so the property and relations channels are disabled —
// scheduleFieldSave/scheduleRelationsChange would throw if called. The
// content channel debounces at 100ms so rapid checkbox clicks coalesce
// into one PATCH while still feeling instant; the e2e suite tolerates
// up to 2s for the UI poll and 5s for the server poll.
//
// contentRef is a computed mirror of viewData.entry.content; the
// composable only reads its shape (never writes), so a read-only
// computed is sufficient. Property/relations callbacks are no-ops to
// satisfy AutoSaveOptions's required surface; they are unreachable
// because their channels are disabled and mergeServerResponse skips
// them.
const entryContent = computed(() => entry.value?.content ?? '')
const entryProperties = computed<Record<string, unknown>>(() => entry.value?.properties ?? {})
// When the route changes mid-debounce, the watch pins the previous
// entity identity here so the in-flight flush PATCHes the entity the
// user actually clicked, not the one they just navigated to.
const pinEntityForFlush = ref<{ type: string; id: string } | null>(null)
const contentAutoSave = useAutoSave({
  getEntityType: () => pinEntityForFlush.value?.type ?? props.entityType,
  getEntityId: () => pinEntityForFlush.value?.id ?? props.entityId,
  contentDebounceMs: 100,
  formData: entryProperties as unknown as import('vue').Ref<Record<string, unknown>>,
  contentRef: entryContent as unknown as import('vue').Ref<string>,
  inverseToCanonical: new Map(),
  buildRelationsBody: () => null,
  applyServerProperty: () => {},
  applyServerContent: (next) => {
    const view = viewData.value
    if (!view || !view.entry) return
    // If the route changed mid-flush, the previous entity's PATCH may
    // resolve after the new entity's loadView; reject the apply by
    // entity-identity rather than splice stale content into the new
    // view.
    const pinned = pinEntityForFlush.value
    if (pinned && (view.entry.id !== pinned.id || view.entry.type !== pinned.type)) return
    const nextSections = view.sections.map((s) =>
      isEntryContentSection(s) ? { ...s, content: next } : s
    )
    viewData.value = { ...view, entry: { ...view.entry, content: next }, sections: nextSections }
  },
  onError: (msg) => uiStore.error(msg),
  disablePropertyChannel: true,
  disableRelationsChannel: true,
})

function contentClick(event: MouseEvent) {
  const target = event.target as HTMLElement | null
  const checkbox = target?.closest<HTMLInputElement>('input[type="checkbox"][data-cb-idx]')
  if (!checkbox) return
  event.preventDefault()
  const raw = checkbox.dataset.cbIdx
  if (raw === undefined) return
  const idx = parseInt(raw, 10)
  if (Number.isNaN(idx)) return
  handleCheckboxToggle(idx)
}

function handleCheckboxToggle(index: number) {
  const current = entry.value
  const view = viewData.value
  if (!current || !view) return
  let newContent: string
  try {
    newContent = toggleCheckboxInSource(current.content || '', index)
  } catch (err) {
    // The toggler is intentionally narrower than the renderer (see
    // checkboxToggle.ts — `*`, `+`, ordered-list checkboxes parse as
    // task items in marked but are rejected here). Surface the thrown
    // detail so users know which line of the source the click missed.
    const detail = getErrorMessage(err, 'unknown error')
    uiStore.error(`Failed to toggle checkbox: ${detail}`)
    console.error(err)
    return
  }
  // Apply optimistically so a second click within the debounce window
  // toggles the post-first state (the source for the next toggle is
  // viewData.entry.content). Without this, two rapid clicks on the
  // same index would each toggle the unchanged server content and
  // net to zero on the next PATCH.
  const nextSections = view.sections.map((s) =>
    isEntryContentSection(s) ? { ...s, content: newContent } : s
  )
  viewData.value = { ...view, entry: { ...current, content: newContent }, sections: nextSections }
  contentAutoSave.scheduleContentSave(newContent)
}

// Keyboard shortcuts
function handleKeydown(e: KeyboardEvent) {
  if (isInputFocused()) return
  if (isAnyModalOpen()) return
  if (document.querySelector('.shortcuts-overlay')) return

  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    if (!canUpdate.value) {
      uiStore.warning('Edit not permitted for this entity')
      return
    }
    editEntity()
  }
  if ((e.key === 'Delete' || e.key === 'Backspace') && entry.value) {
    e.preventDefault()
    if (!canDelete.value) {
      uiStore.warning('Delete not permitted for this entity')
      return
    }
    void requestDelete()
  }
  if (e.key === 'p' && scopeNav.value?.prev) {
    e.preventDefault()
    navigateScope('prev')
  }
  if (e.key === 'n' && scopeNav.value?.next) {
    e.preventDefault()
    navigateScope('next')
  }
  if (e.key === 'Escape' && backTarget.value) {
    e.preventDefault()
    router.push(backTarget.value.to)
  }
}

function closeOverflow() {
  showOverflowMenu.value = false
}
watch(showOverflowMenu, (open) => {
  if (open) document.addEventListener('click', closeOverflow)
  else document.removeEventListener('click', closeOverflow)
})

// Commands — separate fetch, abortable to survive rapid navigation
// (BUG-6C3V: a stale fetch resolving against an unmounted component).
let commandsAbort: AbortController | null = null

async function loadCommands() {
  commandsAbort?.abort()
  commandsAbort = new AbortController()
  const localAbort = commandsAbort
  try {
    commands.value = await getCommands(
      { pageType: 'entity', entityType: props.entityType },
      localAbort.signal
    )
  } catch (err) {
    if (localAbort.signal.aborted) return
    if (isCancelledFetch(err)) return
    console.error('Failed to load commands:', err)
    commands.value = []
  }
}

function runCommand(cmd: Command) {
  commandModalRef.value?.runCommand(cmd)
}

async function loadView() {
  loading.value = true
  error.value = null
  // Report EVERY load to the global activity bar, including the ones where
  // previous content stays on screen.
  //
  // Holding the old entity is what stops the page collapsing, but it also
  // means a prev/next step looks like NOTHING happened until the new data
  // lands — the click reads as ignored. Stale content with no indicator is
  // the failure mode; the bar is what distinguishes "showing you the
  // previous one while the next loads" from "frozen".
  //
  // A bar is the right shape for that precisely because it does not
  // contradict the content: it is peripheral and additive, where a spinner
  // replacing the article would assert there is nothing to see.
  //
  // The route itself settles in ~99ms while this fetch runs ~2s, so without
  // reporting here the bar would never cover the part that actually takes
  // time. The 250ms gate still suppresses it entirely when the fetch is
  // quick.
  const settleRouteLoad = beginRouteLoad()
  try {
    viewData.value = await fetchView(props.entityType, props.entityId)
    if (viewData.value?.entry) {
      // Seed the autosave baseline so the first toggle's no-op
      // suppression can compare against server state without waiting
      // for the response of a sentinel PATCH.
      contentAutoSave.recordServerSnapshot(viewData.value.entry)
    }
    await Promise.all([loadCommands(), loadScopeNav()])
  } catch (err) {
    if (isCancelledFetch(err)) return
    error.value = getErrorMessage(err, 'Failed to load entity')
    console.error('Failed to load entity view:', err)
  } finally {
    loading.value = false
    settleRouteLoad?.()
  }
}

// Actions
function editEntity() {
  // The backend refuses to write through inaccessible (git-crypt encrypted)
  // entities; bail out client-side so the Edit shortcut is also a no-op.
  if (isInaccessible.value) return
  if (!editFormId.value) {
    uiStore.error('No edit form configured for this entity type')
    return
  }
  router.push({ name: 'form-edit', params: { id: editFormId.value, entityId: props.entityId } })
}

async function requestDelete() {
  if (!entry.value) return
  const ok = await confirm({
    title: 'Delete Entity?',
    message: `Are you sure you want to delete '${props.entityId}'? This action cannot be undone.`,
    confirmLabel: 'Delete',
    danger: true,
    onConfirm: withConfirmError(
      async () => {
        // Use the entity API directly — entitiesStore.remove is the
        // canonical CRUD path; keep using it.
        const { useEntitiesStore } = await import('@/stores')
        const entitiesStore = useEntitiesStore()
        await entitiesStore.remove(props.entityType, props.entityId)
      },
      'Failed to delete entity',
      uiStore
    ),
  })
  if (!ok) return
  uiStore.success('Entity deleted successfully')
  router.push(backTargetAfterDelete())
}

function backTargetAfterDelete(): string {
  if (backTarget.value) return backTarget.value.to
  const listId = schemaStore.findListIdForEntityType(props.entityType)
  if (listId) return `/list/${listId}`
  return '/'
}

// Section navigation helpers
//
// entityTarget is the single source of truth for a section entry's destination,
// bound to the RouterLinks below so a cmd/middle-clicked tab lands exactly where
// a plain click does.
// Nil: returns undefined when the entry has no resolvable route (empty type),
// and the template renders plain text instead of an anchor.
function entityTarget(entity: { id: string; type: string }, cellLink?: string): string | undefined {
  return entityDetailHref(entity, { cellLink }) || undefined
}

function navigateToEntity(entity: { id: string; type: string }, cellLink?: string) {
  const path = entityDetailHref(entity, { cellLink })
  if (!path) return
  router.push(path)
}

function navigateToEdit(formId: string, entityId: string) {
  router.push({ name: 'form-edit', params: { id: formId, entityId } })
}

// Look up a schema PropertyDef for an entity type's property. Returns
// undefined when the entity type or property isn't in the schema. Used
// to pre-resolve defs at section level (RR-UD1H) instead of in every
// cell render.
function getPropertyDef(entityType: string, propertyName: string): PropertyDef | undefined {
  const et = schemaStore.getEntityType(entityType)
  return et?.properties?.[propertyName]
}

// entryDisplayValue: the entry-section analogue of `rowDisplayValue` below —
// prefer the live `entry.properties` over the display-stringified
// `fields[i].values` server mirror, which `handlePropertyApplied` never
// updates (it rewrites `entry.properties` only).
//
// Before TKT-HOIX1 the entry display path was reachable only when the ACL
// denied EVERY field in the section, so nothing could edit those values and
// the mirror could not go stale. With `render` defaulting to display, a
// display-rendered section is now the common case and can sit alongside a
// `render: input` section editing the same property — at which point the
// display section would show last-loadView's string forever. Same bug class
// as RR-FC1C, same fix.
function entryDisplayValue(field: ViewSectionField): unknown {
  const props = entry.value?.properties
  if (field.property && props && field.property in props) {
    return props[field.property]
  }
  return field.values ?? []
}

function mapFieldsToProperties(fields: ViewSectionField[] | undefined): PropertyItem[] {
  if (!fields) return []
  // Pre-resolve PropertyDef for the entry entity once (RR-UD1H). For
  // entry-level properties the entity type is fixed for the section.
  // When the property isn't in the schema we leave propertyDef undefined
  // and let PropertyDisplay fall back to a WidgetRoutingHint
  // (RR-UD2B) -- no more synthesised PropertyDef lies.
  const entryType = entry.value?.type
  return fields.map((field) => {
    // PropertyDisplay's `name` is used as a vue list key; favor the raw
    // property name when available and fall back to a slugged label so
    // older shapes still render.
    const name = field.property ?? field.label.toLowerCase().replace(/\s+/g, '_')
    const def = entryType && field.property ? getPropertyDef(entryType, field.property) : undefined
    return {
      name,
      label: field.label,
      value: entryDisplayValue(field),
      propType: field.propType,
      propertyDef: def,
      span: field.span,
      inaccessible: field.inaccessible ?? false,
      inaccessibleReason: field.property ? inaccessibleByName.value.get(field.property) : undefined,
      attachments: field.property ? entry.value?._attachments?.[field.property] : undefined,
      max: def?.max,
    }
  })
}

// FieldRow bundles per-field data for cards/list rendering. Computed
// once per (entity, field) instead of recomputed inline on every
// reactive tick (RR-UD2A).
interface FieldRow {
  field: ViewSectionField
  widget: Component
  hint: WidgetRoutingHint
}

// fieldRowsFor returns the precomputed FieldRow array for one entity's
// fields. Cards/list templates iterate this instead of calling helper
// functions inline per cell.
function fieldRowsFor(ent: { fields?: ViewSectionField[] }): FieldRow[] {
  return (ent.fields ?? []).map((field) => {
    const hint = viewFieldRoutingHint(field)
    return {
      field,
      hint,
      widget: defaultRegistry.resolveFromHint(hint),
    }
  })
}

// Memoize per (section reference, entry reference) so the SectionEditForm's
// `watch(() => props.fields)` only fires when the underlying section or
// entry identity changes — not on every reactive tick (RR-FB2D NEW-4).
const sectionEditFieldsCache = new WeakMap<
  ViewSection,
  { entry: Entity; fields: SectionEditField[] }
>()
function memoBuildSectionEditFields(section: ViewSection, ent: Entity): SectionEditField[] {
  const cached = sectionEditFieldsCache.get(section)
  if (cached && cached.entry === ent) return cached.fields
  const fields = buildSectionEditFieldsPure(section.fields, ent, getPropertyDef)
  sectionEditFieldsCache.set(section, { entry: ent, fields })
  return fields
}

function sectionShouldRouteToInlineEdit(section: ViewSection, ent: Entity): boolean {
  return sectionShouldRouteToInlineEditPure(section.fields, ent, getPropertyDef)
}

// The properties inline-edit section renders its OWN heading row (so the
// heading and the auto-save indicator are flex siblings on one line —
// TKT-U62DVR). For that case the generic <h2> above is suppressed to avoid
// a duplicate heading. Every other section keeps the shared <h2>.
//
// Requires a truthy `section.heading`: SectionEditForm only renders its
// header row when `heading` is non-empty (v-if="heading"), so if we
// suppressed the generic <h2> for a headingless properties section we'd
// end up with NO heading at all and a bare indicator. Gating on the same
// truthiness keeps the two guards in agreement (RR-32ARO9).
function sectionRendersOwnHeading(section: ViewSection): boolean {
  const ent = entry.value
  return (
    !!section.heading &&
    section.display === 'properties' &&
    ent != null &&
    sectionShouldRouteToInlineEdit(section, ent)
  )
}

// One-shot dedupe for the 401/403 → loadView path. Cleared on each
// loadView call to allow the next 4xx to refetch again.
let pendingRefetch = false

function handlePropertyApplied(
  prop: string,
  value: unknown,
  applyOwner: { type: string; id: string }
) {
  const view = viewData.value
  const nextEntry = applyPropertyToEntry(view?.entry ?? null, prop, value, applyOwner)
  if (!nextEntry || !view) return
  viewData.value = { ...view, entry: nextEntry }
}

function handleSectionEditError(msg: string, info?: AutoSaveErrorInfo) {
  uiStore.error(msg)
  if ((info?.status === 401 || info?.status === 403) && !pendingRefetch) {
    pendingRefetch = true
    void loadView().finally(() => {
      pendingRefetch = false
    })
  }
}

function handleVerdictFlip(_prop: string, label: string) {
  uiStore.warning(`Permission changed — your unsaved edit to '${label}' was discarded`)
}

// ─── Per-row inline edit on cards/list sections (TKT-IHC7C) ──────────────

// Above this row count, a cards/list section falls back to display mode
// for ALL rows. SectionEditForm instances are cheap individually but
// quadratic in aggregate (each holds its own debounce timers and watch);
// 100 rows is the soft cap (RR-FC1D). Above this the user is more
// likely browsing than editing.
const INLINE_EDIT_ROW_CAP = 100

// Thin SFC adapter — the cap-behaviour logic lives in the pure module
// so it's unit-testable without mounting EntityDetail.
function rowShouldRouteToInlineEdit(ent: ViewEntity, rowCount: number): boolean {
  return rowShouldRouteToInlineEditPure(ent, rowCount, INLINE_EDIT_ROW_CAP, getPropertyDef)
}

// Memoize per-row field shape, keyed on the row reference. Spread-clone
// of the row in handleRowPropertyApplied produces a new reference for
// THAT row only — surrounding rows retain their references and their
// cached entry. Cache invalidation is naturally GC-driven via the
// WeakMap (RR-FC1E NEW-3).
const rowEditFieldsCache = new WeakMap<ViewEntity, SectionEditField[]>()
function memoBuildRowEditFields(ent: ViewEntity): SectionEditField[] {
  const cached = rowEditFieldsCache.get(ent)
  if (cached) return cached
  const fields = buildSectionEditFieldsPure(ent.fields, ent, getPropertyDef)
  rowEditFieldsCache.set(ent, fields)
  return fields
}

// rowDisplayValue: per-cell display-mode read prefers `_props` over the
// display-stringified `fields[i].values`. This eliminates the
// stale-string-mirror bug where a verdict flips writable→display and
// the cell shows last-loadView's display string instead of the
// post-edit value (RR-FC1C).
function rowDisplayValue(ent: ViewEntity, field: ViewSectionField): unknown {
  if (field.property && ent._props && field.property in ent._props) {
    return ent._props[field.property]
  }
  return field.values ?? []
}

// Owner → (sectionIdx, rowIdx) index, rebuilt per viewData change so
// `handleRowPropertyApplied` is O(1) instead of O(sections × rows)
// (RR-FC1E NEW-3). The rebuild is itself O(rows) but only fires when
// `viewData.value` reference changes — including after every confirmed
// row PATCH, which is acceptable.
const rowIndex = ref(new Map<string, { sectionIdx: number; rowIdx: number }>())
watch(
  viewData,
  (vd) => {
    const next = new Map<string, { sectionIdx: number; rowIdx: number }>()
    if (vd?.sections) {
      vd.sections.forEach((section, sectionIdx) => {
        section.entities?.forEach((ent, rowIdx) => {
          next.set(`${ent.type}/${ent.id}`, { sectionIdx, rowIdx })
        })
      })
    }
    rowIndex.value = next
  },
  { immediate: true }
)

function handleRowPropertyApplied(
  prop: string,
  value: unknown,
  applyOwner: { type: string; id: string }
) {
  const view = viewData.value
  if (!view) return
  const key = `${applyOwner.type}/${applyOwner.id}`
  const loc = rowIndex.value.get(key)
  if (!loc) return // row deleted / repositioned beyond identity reach
  const section = view.sections[loc.sectionIdx]
  const currentRow = section?.entities?.[loc.rowIdx]
  if (!currentRow) return
  // applyPropertyToRow rejects the apply if the row at this location
  // no longer matches the owner identity (defensive against an index
  // rebuilt mid-flight against a different layout).
  const nextRow = applyPropertyToRow(currentRow, prop, value, applyOwner)
  if (!nextRow) return
  const nextSections = view.sections.map((s, i) => {
    if (i !== loc.sectionIdx) return s
    return {
      ...s,
      entities: s.entities?.map((e, j) => (j === loc.rowIdx ? nextRow : e)),
    }
  })
  viewData.value = { ...view, sections: nextSections }
}

function shouldUseBadge(value: string, propType?: string): boolean {
  return !!propType && !!value
}

function scrollToSection(sectionId: string) {
  const el = document.getElementById(sectionId)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// Lifecycle
onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  loadView()
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  commandsAbort?.abort()
  // Flush any pending toggle PATCH so navigating away mid-debounce
  // doesn't silently drop the user's click. commitImmediately returns
  // a promise; we don't await it because Vue's onBeforeUnmount is
  // synchronous — the FIFO chain will run to completion regardless.
  void contentAutoSave.commitImmediately()
})

onUnmounted(() => document.removeEventListener('click', closeOverflow))

// Watch for route changes
watch(
  () => [props.entityType, props.entityId],
  (_next, prev) => {
    // Flush pending toggles for the previous entity before loading the
    // next one. The composable reads entity identity from
    // getEntityType/getEntityId at fire time, which would resolve to
    // the new entity by the time the FIFO chain runs — so we capture
    // the previous identity for the duration of the flush via a
    // one-shot override before triggering commit.
    const [prevType, prevId] = prev
    const fireWith = { type: prevType, id: prevId }
    pinEntityForFlush.value = fireWith
    void contentAutoSave.commitImmediately().finally(() => {
      if (pinEntityForFlush.value === fireWith) pinEntityForFlush.value = null
    })
    loadView()
  }
)
</script>

<template>
  <div class="entity-detail">
    <!-- Deliberately empty while loading below the threshold: no spinner,
         no reserved block, no layout spring. The ActivityBar carries the
         navigation case; this only paints for a slow cold load. -->
    <div v-if="showBlockLoader" class="loading-state">
      <div class="spinner" />
      <span>Loading…</span>
    </div>
    <div v-else-if="loading && !entry" class="entity-detail-placeholder" />

    <div v-else-if="error" class="error-state">
      <h2>Error</h2>
      <p>{{ error }}</p>
      <button class="btn btn-primary" @click="loadView">Retry</button>
    </div>

    <template v-else-if="entry">
      <!-- Back affordance + optional scope (prev/next) navigation. The bar
           renders when either a back target exists (?return_to or ?from)
           or the user is in a list-scoped context (?from drives scopeNav).
           Both can be present simultaneously. -->
      <div v-if="backTarget || scopeNav" class="scope-nav mobile-topbar">
        <BackButton v-if="backTarget" :target="backTarget" />
        <template v-if="scopeNav">
          <button v-if="scopeNav.prev" class="scope-nav-btn" @click="navigateScope('prev')">
            ← Prev <kbd>P</kbd>
          </button>
          <span v-else class="scope-nav-btn disabled">← Prev</span>
          <span class="scope-nav-progress">[{{ scopeNav.current }}/{{ scopeNav.total }}]</span>
          <span class="scope-nav-label">{{ scopeNav.label }}</span>
          <button v-if="scopeNav.next" class="scope-nav-btn" @click="navigateScope('next')">
            Next → <kbd>N</kbd>
          </button>
          <span v-else class="scope-nav-btn disabled">Next →</span>
        </template>
      </div>

      <header class="detail-header">
        <div class="header-info">
          <span class="entity-type-badge">{{ typeDef?.label || entityType }}</span>
          <h1 class="text-wrap-anywhere">{{ entryTitle }}</h1>
        </div>
        <!-- Desktop actions -->
        <div v-if="!props.hideActions" class="header-actions desktop-actions">
          <button
            v-for="cmd in commands"
            :key="cmd.id"
            class="btn btn-command"
            @click="runCommand(cmd)"
          >
            {{ cmd.label }}
          </button>
          <button
            v-if="editFormId && !isInaccessible && canUpdate"
            class="btn btn-secondary"
            @click="editEntity"
          >
            Edit <kbd>E</kbd>
          </button>
          <button class="btn btn-secondary" @click="openHistory">History</button>
          <ExportMenu :url-for="(t: string) => entityExportUrl(entityType, entityId, t)" />
          <button v-if="canDelete" class="btn btn-danger" @click="requestDelete">
            Delete <kbd>Del</kbd>
          </button>
        </div>

        <!-- Mobile actions: Edit primary, delete icon, overflow menu for commands -->
        <div v-if="!props.hideActions" class="header-actions mobile-actions">
          <button
            v-if="editFormId && !isInaccessible && canUpdate"
            class="btn btn-secondary"
            @click="editEntity"
          >
            Edit
          </button>
          <button
            v-if="canDelete"
            class="btn btn-danger mobile-delete-btn"
            aria-label="Delete"
            @click="requestDelete"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <polyline points="3 6 5 6 21 6" />
              <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2" />
            </svg>
          </button>
          <div v-if="commands.length" class="overflow-menu-wrapper">
            <button
              class="btn btn-secondary mobile-overflow-btn"
              aria-label="More actions"
              @click.stop="showOverflowMenu = !showOverflowMenu"
            >
              ⋯
            </button>
            <div v-if="showOverflowMenu" class="overflow-menu" @click="showOverflowMenu = false">
              <button
                v-for="cmd in commands"
                :key="cmd.id"
                class="overflow-menu-item"
                @click="runCommand(cmd)"
              >
                {{ cmd.label }}
              </button>
            </div>
          </div>
        </div>
      </header>

      <!-- Inaccessible (git-crypt encrypted) banner. Sits above the
           sections so it dominates the visual hierarchy when the entity
           cannot be read with the current key. -->
      <aside v-if="isInaccessible" class="inaccessible-banner">
        <span class="inaccessible-banner-icon" aria-hidden="true">🔒</span>
        <div>
          <strong>This entity is git-crypt encrypted.</strong>
          <p>
            The file is stored as ciphertext and cannot be read with the current configuration. Run
            <code>git-crypt unlock</code>
            in the project root to decrypt it, then reload this page.
            <a
              href="https://github.com/AGWA/git-crypt#readme"
              target="_blank"
              rel="noopener noreferrer"
              >Learn more about git-crypt.</a
            >
          </p>
        </div>
      </aside>

      <!-- Jump bar (only when there's enough to jump between) -->
      <nav v-if="viewData && viewData.sections.length > 1" class="jump-bar">
        <button
          v-for="section in viewData.sections.filter((s) => s.heading)"
          :key="section.sectionId"
          class="jump-link"
          @click="scrollToSection(section.sectionId)"
        >
          {{ section.heading }}
        </button>
      </nav>

      <!-- Sections -->
      <div v-if="viewData" class="sections">
        <section
          v-for="section in viewData.sections"
          :id="section.sectionId"
          :key="section.sectionId"
          class="view-section"
        >
          <h2
            v-if="
              !sectionRendersOwnHeading(section) &&
              (section.heading || (section === entryContentSection && checkboxStats))
            "
            class="section-heading"
          >
            {{ section.heading }}
            <span v-if="section === entryContentSection && checkboxStats" class="cb-stats"
              >({{ checkboxStats.checked }}/{{ checkboxStats.total }})</span
            >
          </h2>

          <div v-if="section.isEmpty" class="section-empty">
            {{ section.emptyMessage || 'No items' }}
          </div>

          <!--
            Properties section routing (TKT-IHC7B):
            - At least one writable field → SectionEditForm (inline edit).
            - All non-writable                → PropertyDisplay (read-only).
            The `:key` on `${entry.type}/${entry.id}` forces SectionEditForm
            remount on entity-id change so its lifecycle handles route
            navigation cleanly (RR-FB1D + RR-FB2A).
          -->
          <SectionEditForm
            v-else-if="
              section.display === 'properties' &&
              entry &&
              sectionShouldRouteToInlineEdit(section, entry)
            "
            :key="`${entry.type}/${entry.id}`"
            :heading="section.heading"
            :entity-type="entry.type"
            :entity-id="entry.id"
            :initial-values="entry.properties"
            :attachments="entry._attachments"
            :fields="memoBuildSectionEditFields(section, entry)"
            :on-property-applied="handlePropertyApplied"
            :on-error="handleSectionEditError"
            :on-verdict-flip="handleVerdictFlip"
            :on-attachment-changed="loadView"
          />
          <PropertyDisplay
            v-else-if="section.display === 'properties'"
            :properties="mapFieldsToProperties(section.fields)"
          />

          <!-- Entry content with mermaid + interactive checkboxes.
               Function ref instead of string ref because this template lives
               inside a v-for: Vue would otherwise collect template-refs of
               the same name into an array per iteration. -->
          <div
            v-else-if="section === entryContentSection"
            :ref="
              (el) => {
                contentRef = el as HTMLElement | null
              }
            "
            class="content-body md-body"
            @click="contentClick"
            v-html="renderedEntryContent"
          />

          <!-- Other content sections (e.g. content cards from a configured view). -->
          <div
            v-else-if="section.display === 'content' && section.hasContent"
            class="content-block"
          >
            <div
              class="markdown-content md-body"
              v-html="renderMarkdown(section.content || '', refResolver)"
            />
          </div>

          <div
            v-else-if="section.display === 'content' && section.entities?.length"
            class="content-cards"
          >
            <article
              v-for="ent in section.entities"
              :key="ent.id"
              :data-entity-id="ent.id"
              class="content-card"
            >
              <header class="card-header">
                <component
                  :is="entityTarget(ent) ? RouterLink : 'span'"
                  class="card-header-link"
                  v-bind="entityTarget(ent) ? { to: entityTarget(ent) } : {}"
                >
                  <span class="entity-type">{{ ent.type }}</span>
                  <span class="entity-title">{{ ent.title }}</span>
                  <span class="entity-id">{{ ent.id }}</span>
                </component>
              </header>
              <div
                v-if="ent.hasContent"
                class="markdown-content md-body"
                v-html="renderMarkdown(ent.content || '', refResolver)"
              />
            </article>
          </div>

          <div v-else-if="section.display === 'cards'" class="cards-grid">
            <article
              v-for="ent in section.entities"
              :key="ent.id"
              :data-entity-id="ent.id"
              class="entity-card"
            >
              <!--
                Navigation handler moved from <article> to .card-header
                (TKT-IHC7C / RR-FC1B): clicks on inline-edit widgets
                inside the card must not bubble to the row-level click
                because that would navigate away mid-edit. The header
                still navigates; cells stay editable.
              -->
              <header class="card-header">
                <component
                  :is="entityTarget(ent) ? RouterLink : 'span'"
                  class="card-header-link"
                  v-bind="entityTarget(ent) ? { to: entityTarget(ent) } : {}"
                >
                  <span class="entity-type">{{ ent.type }}</span>
                  <span class="entity-title">{{ ent.title }}</span>
                  <span class="entity-id">{{ ent.id }}</span>
                </component>
                <button
                  v-if="ent.editFormId"
                  class="edit-btn"
                  title="Edit"
                  @click.stop="navigateToEdit(ent.editFormId, ent.id)"
                >
                  &times;
                </button>
              </header>
              <SectionEditForm
                v-if="rowShouldRouteToInlineEdit(ent, section.entities?.length ?? 0)"
                :key="`${ent.type}/${ent.id}`"
                :entity-type="ent.type"
                :entity-id="ent.id"
                :initial-values="ent._props ?? {}"
                :fields="memoBuildRowEditFields(ent)"
                :on-property-applied="handleRowPropertyApplied"
                :on-error="handleSectionEditError"
                :on-verdict-flip="handleVerdictFlip"
              >
                <!-- Indicator rendered inline in the card (TKT-IHC7C). NOT
                     teleported: the former <Teleport> target span lived in
                     the same render subtree as the Teleport itself, so Vue
                     could leave the teleported child orphaned with a null
                     instance that crashed on route-driven unmount (#997). -->
                <template #indicator="{ status, error }">
                  <span class="card-indicator-slot">
                    <AutoSaveIndicator :status="status" :error="error" />
                  </span>
                </template>
              </SectionEditForm>
              <div v-else-if="ent.fields?.length" class="card-fields">
                <div v-for="row in fieldRowsFor(ent)" :key="row.field.label" class="card-field">
                  <span class="field-label">{{ row.field.label }}:</span>
                  <!-- The wire-level inaccessibleReason map is keyed on
                       the entry's properties, not the per-entity card
                       row's. We don't have a per-card reason map today
                       (see RR-UD2E follow-up), so InaccessibleField
                       falls back to the generic tooltip. -->
                  <InaccessibleField v-if="row.field.inaccessible" />
                  <component
                    :is="row.widget"
                    v-else
                    :model-value="rowDisplayValue(ent, row.field)"
                    :mode="'display'"
                    :property-name="row.hint.propertyName"
                    class="field-value"
                  />
                </div>
              </div>
            </article>
          </div>

          <ul v-else-if="section.display === 'list'" class="entity-list">
            <li
              v-for="ent in section.entities"
              :key="ent.id"
              :data-entity-id="ent.id"
              class="list-item"
            >
              <component
                :is="entityTarget(ent) ? RouterLink : 'span'"
                class="list-link"
                v-bind="entityTarget(ent) ? { to: entityTarget(ent) } : {}"
              >
                <span class="entity-type">{{ ent.type }}</span>
                <span class="entity-title">{{ ent.title }}</span>
                <span class="entity-id">{{ ent.id }}</span>
              </component>
              <SectionEditForm
                v-if="rowShouldRouteToInlineEdit(ent, section.entities?.length ?? 0)"
                :key="`${ent.type}/${ent.id}`"
                :entity-type="ent.type"
                :entity-id="ent.id"
                :initial-values="ent._props ?? {}"
                :fields="memoBuildRowEditFields(ent)"
                :on-property-applied="handleRowPropertyApplied"
                :on-error="handleSectionEditError"
                :on-verdict-flip="handleVerdictFlip"
              >
                <!-- Indicator rendered inline in the row (TKT-IHC7C). NOT
                     teleported: the former <Teleport> target span lived in
                     the same render subtree as the Teleport itself, so Vue
                     could leave the teleported child orphaned with a null
                     instance that crashed on route-driven unmount (#997). -->
                <template #indicator="{ status, error }">
                  <span class="list-indicator-slot">
                    <AutoSaveIndicator :status="status" :error="error" />
                  </span>
                </template>
              </SectionEditForm>
              <span v-else-if="ent.fields?.length" class="list-fields">
                <template v-for="row in fieldRowsFor(ent)" :key="row.field.label">
                  <!-- Per-card inaccessibility reason isn't on the wire
                       today (see RR-UD2E follow-up); InaccessibleField
                       falls back to the generic tooltip. -->
                  <InaccessibleField v-if="row.field.inaccessible" />
                  <component
                    :is="row.widget"
                    v-else
                    :model-value="rowDisplayValue(ent, row.field)"
                    :mode="'display'"
                    :property-name="row.hint.propertyName"
                  />
                </template>
              </span>
            </li>
          </ul>

          <div v-else-if="section.display === 'table'" class="table-wrapper">
            <template v-if="section.isGrouped && section.groups?.length">
              <div v-for="group in section.groups" :key="group.groupName" class="table-group">
                <h3 class="group-heading">{{ group.groupName }}</h3>
                <table class="data-table">
                  <thead>
                    <tr>
                      <th v-for="col in section.columns" :key="col.property || col.relation">
                        {{ col.label || col.property || col.relation }}
                      </th>
                      <th class="actions-col" />
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in group.rows" :key="row.entityId">
                      <td v-for="(cell, idx) in row.cells" :key="idx">
                        <a
                          v-if="cell.link"
                          :href="cell.link"
                          @click.prevent="
                            navigateToEntity(
                              {
                                id: cell.entityId || row.entityId,
                                type: cell.entityType || row.entityType,
                              },
                              cell.link
                            )
                          "
                        >
                          <template v-for="(val, vidx) in cell.values" :key="vidx">
                            <Badge
                              v-if="shouldUseBadge(val, cell.propType)"
                              :value="val"
                              :property="cell.propType"
                            />
                            <span v-else>{{ val }}</span>
                            <span v-if="vidx < cell.values.length - 1">, </span>
                          </template>
                        </a>
                        <template v-else>
                          <template v-for="(val, vidx) in cell.values" :key="vidx">
                            <Badge
                              v-if="shouldUseBadge(val, cell.propType)"
                              :value="val"
                              :property="cell.propType"
                            />
                            <span v-else>{{ val }}</span>
                            <span v-if="vidx < cell.values.length - 1">, </span>
                          </template>
                        </template>
                      </td>
                      <td class="actions-cell">
                        <button
                          v-if="row.editFormId"
                          class="icon-btn"
                          title="Edit"
                          @click="navigateToEdit(row.editFormId, row.entityId)"
                        >
                          &#9998;
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>

            <table v-else class="data-table">
              <thead>
                <tr>
                  <th v-for="col in section.columns" :key="col.property || col.relation">
                    {{ col.label || col.property || col.relation }}
                  </th>
                  <th class="actions-col" />
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in section.rows" :key="row.entityId">
                  <td v-for="(cell, idx) in row.cells" :key="idx">
                    <a
                      v-if="cell.link"
                      :href="cell.link"
                      @click.prevent="
                        navigateToEntity(
                          {
                            id: cell.entityId || row.entityId,
                            type: cell.entityType || row.entityType,
                          },
                          cell.link
                        )
                      "
                    >
                      <template v-for="(val, vidx) in cell.values" :key="vidx">
                        <Badge
                          v-if="shouldUseBadge(val, cell.propType)"
                          :value="val"
                          :property="cell.propType"
                        />
                        <span v-else>{{ val }}</span>
                        <span v-if="vidx < cell.values.length - 1">, </span>
                      </template>
                    </a>
                    <template v-else>
                      <template v-for="(val, vidx) in cell.values" :key="vidx">
                        <Badge
                          v-if="shouldUseBadge(val, cell.propType)"
                          :value="val"
                          :property="cell.propType"
                        />
                        <span v-else>{{ val }}</span>
                        <span v-if="vidx < cell.values.length - 1">, </span>
                      </template>
                    </template>
                  </td>
                  <td class="actions-cell">
                    <button
                      v-if="row.editFormId"
                      class="icon-btn"
                      title="Edit"
                      @click="navigateToEdit(row.editFormId, row.entityId)"
                    >
                      &#9998;
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <!-- External documents (renders only when configured for this type).
             Nested here (not a sibling of .sections) so it picks up the
             container's flex `gap` for free instead of duplicating that
             spacing via its own margin. -->
        <DocumentsPanel :entity-type="entityType" :entity-id="entityId" />
      </div>

      <CommandModal ref="commandModalRef" :entity-id="entityId" />
    </template>

    <div v-else class="error-state">
      <h2>Entity not found</h2>
      <p>{{ entityType }} "{{ entityId }}" could not be found.</p>
      <RouterLink :to="backTargetAfterDelete()" class="btn btn-secondary">
        Back to list
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.entity-detail {
  max-width: 1200px;
  padding: 0 0 24px;
}

.inaccessible-banner {
  display: flex;
  align-items: flex-start;
  gap: var(--space-md);
  margin-bottom: 24px;
  padding: 12px 16px;
  border: 1px solid var(--color-border, #ccc);
  border-left: 4px solid var(--color-warning, #d9970e);
  border-radius: var(--radius-sm);
  background: var(--color-surface, #fafafa);
}

.inaccessible-banner-icon {
  font-size: 20px;
  line-height: 1;
}

.inaccessible-banner p {
  margin: 4px 0 0;
  font-size: var(--font-size-base);
  line-height: 1.5;
}

.inaccessible-banner code {
  padding: 1px 4px;
  background: var(--color-code-bg, #eee);
  border-radius: 3px;
  font-family: var(--font-mono, monospace);
}

/* Uses global .loading-state, .error-state, .spinner from App.vue */

/* Header */
.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-lg);
  margin-bottom: 24px;
}

.header-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.entity-type-badge {
  display: inline-block;
  font-size: var(--font-size-xs);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.header-info h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: var(--text-color);
}

.header-actions {
  display: flex;
  gap: var(--space-sm);
}

.header-actions kbd {
  padding: 2px 5px;
  font-size: 10px;
  background: var(--border-color);
  border-radius: 3px;
  font-family: monospace;
  margin-left: 4px;
}

.btn-command {
  background: var(--accent-color, #3b82f6);
  color: white;
}

.btn-command:hover:not(:disabled) {
  filter: brightness(1.1);
}

/* Mobile-responsive header. .desktop-actions and .mobile-actions are
 * toggled by media queries in App.vue's global styles. */
.mobile-actions {
  display: none;
}

@media (max-width: 720px) {
  .desktop-actions {
    display: none;
  }
  .mobile-actions {
    display: flex;
    align-items: center;
  }
}

.mobile-delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 6px 10px;
}

.overflow-menu-wrapper {
  position: relative;
}

.mobile-overflow-btn {
  font-size: var(--font-size-lg);
  line-height: 1;
  padding: 6px 12px;
}

.overflow-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 4px);
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  min-width: 160px;
  z-index: 50;
}

.overflow-menu-item {
  display: block;
  width: 100%;
  padding: 8px 12px;
  background: none;
  border: none;
  text-align: left;
  font-size: var(--font-size-base);
  color: var(--text-color);
  cursor: pointer;
}

.overflow-menu-item:hover {
  background: var(--hover-bg);
}

/* Scope Navigation Bar
 * .scope-nav-btn styles live in src/styles/back-button.css — see TKT-JIEKC. */
.scope-nav {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  margin-bottom: 20px;
}

.scope-nav-progress {
  font-size: var(--font-size-dense);
  font-weight: 600;
  color: var(--text-color);
  font-family: monospace;
}

.scope-nav-label {
  flex: 1;
  font-size: var(--font-size-dense);
  color: var(--muted-text);
}

/* Jump bar */
.jump-bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  padding: 12px 0;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 24px;
}

.jump-link {
  padding: 6px 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-dense);
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.15s;
}

.jump-link:hover {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: white;
}

/* Sections */
.sections {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xl);
}

.view-section {
  scroll-margin-top: 20px;
}

/* KEEP IN SYNC with SectionEditForm.vue `.section-edit-form-header
   .section-heading` (RR-ZE29PY): the properties inline-edit section renders
   its heading inside that component, which can't reach these scoped styles. */
.section-heading {
  font-size: var(--font-size-lg);
  font-weight: 600;
  margin: 0 0 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-color);
}

.cb-stats {
  font-size: var(--font-size-base);
  font-weight: 500;
  color: var(--muted-text);
  margin-left: 8px;
}

.section-empty {
  padding: 24px;
  text-align: center;
  color: var(--muted-text);
  background: var(--hover-bg);
  border-radius: var(--radius-md);
  font-style: italic;
}

/* Rendered-markdown element styling (headings, lists, code, links, tables,
   blockquotes, line-height, …) is shared across all markdown surfaces via the
   `.md-body` class on the container — see styles/markdown-content.css. Both
   `.content-body` and `.markdown-content` carry that class, so neither
   redefines those properties here (an earlier `.markdown-content` line-height
   override silently beat the shared value and reintroduced drift). */

/* Generic content (collected entities, configured-view content sections). */
.content-block {
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
}

.content-cards {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.content-card {
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
}

.content-card .card-header {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-bottom: 12px;
  cursor: pointer;
}

.content-card .card-header:hover .entity-title {
  color: var(--accent-color);
}

/* Cards grid (relation sections etc.) */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: var(--space-lg);
}

.entity-card {
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: border-color 0.15s;
}

.entity-card:hover {
  border-color: var(--accent-color);
}

.card-header {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-bottom: 12px;
}

.entity-type {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 4px;
  border-radius: 2px;
}

.entity-title {
  font-weight: 500;
  color: var(--text-color);
  flex: 1;
}

.entity-id {
  font-size: var(--font-size-xs);
  font-family: monospace;
  color: var(--muted-text);
}

.edit-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 16px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
}

.edit-btn:hover {
  background: var(--hover-bg);
  color: var(--text-color);
}

.card-fields {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
}

.card-field {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  font-size: var(--font-size-dense);
}

.field-label {
  color: var(--muted-text);
}

.field-value {
  color: var(--text-color);
}

/* Entity list */
.entity-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: 8px 12px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
}

.list-link {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  cursor: pointer;
  flex: 1;
  /* Was an <a> with no href (so cmd-click did nothing); now a real RouterLink.
     Keep the inherited text colour and no underline. */
  color: inherit;
  text-decoration: none;
}

/* The link inside a card header carries the header's own flex layout, so the
   header still reads as one row. It takes the free space, leaving the
   edit-button (which may NOT live inside an <a>) a sibling beside it. */
.card-header-link {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex: 1;
  min-width: 0;
  cursor: pointer;
  color: inherit;
  text-decoration: none;
}

.list-link:hover .entity-title {
  color: var(--accent-color);
}

.list-fields {
  display: flex;
  gap: var(--space-xs);
}

/* AutoSaveIndicator slot for inline-edit rows (TKT-IHC7C). Rendered inline
   in the row's SectionEditForm rather than teleported (#997). Pinned to the
   top-right of the form so it reads as inline-right of the row content,
   matching the pre-teleport placement. */
.list-indicator-slot,
.card-indicator-slot {
  position: absolute;
  top: -28px;
  right: 0;
  display: inline-flex;
  align-items: center;
}

/* Table */
.table-wrapper {
  overflow-x: auto;
}

.table-group {
  margin-bottom: 24px;
}

.group-heading {
  font-size: var(--font-size-base);
  font-weight: 600;
  color: var(--muted-text);
  margin: 0 0 8px;
  padding: 4px 0;
  border-bottom: 1px solid var(--border-color);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--font-size-base);
}

.data-table th,
.data-table td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-color);
}

.data-table th {
  font-weight: 500;
  color: var(--muted-text);
  background: var(--hover-bg);
}

.data-table td {
  color: var(--text-color);
}

.data-table tbody tr:hover {
  background: var(--hover-bg);
}

.data-table a {
  color: var(--accent-color);
  text-decoration: none;
}

.data-table a:hover {
  text-decoration: underline;
}

.actions-col {
  width: 60px;
}

.actions-cell {
  text-align: center;
}

.icon-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  cursor: pointer;
  padding: 4px 8px;
  font-size: var(--font-size-base);
  border-radius: var(--radius-sm);
}

.icon-btn:hover {
  background: var(--hover-bg);
  color: var(--text-color);
}

@media (max-width: 768px) {
  /* .scope-nav uses .mobile-topbar from mobile-bars.css for the sticky
     chrome + safe-area math. Override only the layout: dense (no gap,
     no wrap) so prev/progress/next sit flush across the bar. */
  .scope-nav {
    gap: 0;
    flex-wrap: nowrap;
    justify-content: space-between;
    /* The bar uses 12px horizontal padding here instead of the default
       16 — the prev/next buttons hug the edges. */
    padding-left: 12px;
  }

  .desktop-actions {
    display: none;
  }

  /* Inline mobile actions (Edit / delete / overflow) — comfortable touch
     targets and spacing. Toggled visible via the .mobile-actions rules
     above. */
  .mobile-actions {
    display: flex;
    gap: var(--space-sm);
    align-items: center;
  }

  .mobile-actions .btn {
    min-height: 44px;
  }

  .mobile-actions .btn-secondary {
    flex: 1;
  }

  .header-info h1 {
    font-size: var(--font-size-xl);
  }

  .detail-section {
    background: none;
    border: none;
    box-shadow: none;
    border-radius: 0;
    padding: 0;
    margin-bottom: 20px;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 16px;
  }
}

</style>
