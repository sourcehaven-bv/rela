<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { isCancelledFetch } from '@/composables/usePageData'
import { getEntityRelations } from '@/api'
import { entityDisplayTitleWithId } from '@/utils/entityDisplay'
import type { FormFieldOrRelation, Entity, RelationEntry, RelationAffordance } from '@/types'
import InlineCreateFormModal from './InlineCreateFormModal.vue'
import { useInlineCreate } from '@/composables/useInlineCreate'
import WorldBadge from '@/components/entity/WorldBadge.vue'
import { DEFAULT_WORLD, useWorld } from '@/composables/useWorld'
import {
  missingIds,
  mergeResolvedLinks,
  indexKnownEntities,
  resolveSelected,
} from './outOfPageLinks'
import { faceCandidates, mergeFamilyCandidates, offersFace, widenWorlds } from './familyCandidates'
import { addressedEntry } from './relationsPatch'
import { entityRef, refBareId, refFace } from '@/utils/entityRef'

// Per-edge state emitted on the incoming-changed channel after
// TKT-GFQK unified the save path. DynamicForm wraps this into a
// RelationCardState so buildRelationsPatch emits under the inverse
// body key. Pickers don't edit per-edge meta so `updated` is empty.
export interface RelationPickerIncomingState {
  // Snapshot loaded from the server, used as the diff baseline.
  loadedEntries: RelationEntry[]
  // Current desired set after user edits.
  currentEntries: RelationEntry[]
  added: Array<{ targetId: string }>
  removed: string[]
}

const props = defineProps<{
  field: FormFieldOrRelation
  entityType: string
  entityId?: string
  value: string[]
  // TKT-G7N5: per-relation-type affordance verdict from the server.
  // Undefined / all fields undefined = default (everything allowed).
  // `creatable === false` hides every "+ Add" affordance (search,
  // inline-create); `removable === false` hides the per-entity x.
  verdict?: RelationAffordance
}>()

const emit = defineEmits<{
  update: [value: string[]]
  // Companion to `update`: a Map of selected-target ID → entity type.
  // The unified PATCH builder needs `type` for every resource identifier
  // (JSON:API §9). RelationPicker is the only widget that knows the
  // type at pick time. Emitted on every `update` and on initial load.
  'update:types': [types: Map<string, string>]
  // Incoming-direction edits flow through this channel. The payload
  // carries enough state for DynamicForm to build the RelationCardState
  // routed under `-incoming` suffix, which buildRelationsPatch then
  // emits under the inverse body key. (See TKT-GFQK.)
  'incoming-changed': [payload: RelationPickerIncomingState]
}>()

const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()

// The picker offers entities to LINK TO, so under a world it must offer that
// world's faces (BUG-3). This is the prerequisite for the face badge below:
// labelling rows that the query chose without regard to worlds would put a
// truthful-looking badge on a row picked by the wrong rule.
const { world, worldParam } = useWorld()

// State
const loading = ref(false)
const candidates = ref<Entity[]>([])
// Candidates the ambient world does not serve, offered on a face some other
// world serves (see familyCandidates.ts), mapped to that face's label, which
// the row shows. Keyed by offWorldKey: two target types may share an id.
const offWorldFaces = ref<Map<string, string>>(new Map())
const searchQuery = ref('')
const showDropdown = ref(false)
const showCreateModal = ref(false)
const createTargetType = ref('')
// Form id for the open create modal, set alongside createTargetType so the
// modal always renders the form the server resolved for that exact type.
const createFormId = ref('')

// For direction: incoming, the picker manages its own value list. The
// parent's `:value` prop is sourced from `entity.relations`, which the
// backend only populates with outgoing edges, so it's never useful for
// reverse pickers. `incomingOriginal` is the snapshot for diff-on-save.
//
// Load-failure-cannot-wipe guarantee (TKT-GFQK F7b): on incoming
// pickers, edits are emitted ONLY after a successful load. If the
// load fails or hasn't completed, the picker stays inert — no
// `incoming-changed` event fires until the user actually selects or
// removes a peer, AND the snapshot is non-null.
const isIncoming = computed(() => props.field.direction === 'incoming')

// TKT-G7N5 affordance helpers. Defaults preserve today's behavior —
// affordances render unless explicitly denied.
const canCreate = computed(() => props.verdict?.creatable !== false)
const canRemove = computed(() => props.verdict?.removable !== false)
// Incoming rows are keyed by address (addressedEntry): an incoming
// content-scoped edge belongs to one face of its source, so `POL-1@draft` and
// `POL-1@published` are two rows.
const incomingValue = ref<string[]>([])
const incomingOriginal = ref<string[]>([])
// Snapshot of the loaded edges keyed by address. Used by the new
// emitIncomingDiff to construct a RelationEntry-shaped payload
// without a second GET. Empty until loadIncomingValue succeeds.
const incomingLoadedEntries = ref<RelationEntry[]>([])
const incomingLoaded = ref(false)

// Entities that are linked but absent from `candidates` (BUG-LSCDJK).
//
// `candidates` is a CHOICE list — the first `per_page: 100` per target type —
// so it answers "what could I pick", not "what did I pick". Past 100 entities
// an existing link falls outside it and has no resolvable type, which makes
// reshapeLegacyToModern return null and drops the whole relations payload.
//
// The edges carry the type (`RelationEntry.type`) — why the `cards` widget was
// never affected. Resolving from that source feeds both the emitted type map
// and `selectedEntities`.
const resolvedLinks = ref<Entity[]>([])

// Computed
const relationType = computed(() => {
  if (!props.field.relation) return undefined
  return schemaStore.getRelationType(props.field.relation)
})

const targetTypes = computed(() => {
  if (!relationType.value) return []
  // Incoming pickers select sources that link AT us, so candidates come
  // from the relation's `from:` set instead of `to:`.
  return isIncoming.value ? relationType.value.from : relationType.value.to
})

// Label resolution (DEC-6C1NAA): an authored form label wins, then the
// metamodel's own label for the relation type, then the raw relation id.
// The metamodel label is server-authored and language-neutral, so consulting
// it is not a derivation from an identifier — and the cleanup migration
// strips a form label that duplicates it, so the SPA MUST read it here or
// that label is lost.
// An incoming picker shows edges pointing AT us, so the inverse label is the
// correct one ("blocked by", not "blocks"). Mirrors the server-side resolution
// in internal/dataentry/export.go relationDisplayLabel.
const label = computed(
  () =>
    props.field.label ||
    (isIncoming.value ? relationType.value?.inverse?.label : undefined) ||
    relationType.value?.label ||
    props.field.relation ||
    ''
)
const help = computed(() => props.field.help || relationType.value?.description || '')

const isMulti = computed(() => {
  // For incoming, cardinality is bounded by `max_incoming` (how many sources
  // may point at us), not `max_outgoing`.
  if (!relationType.value) return true
  const limit = isIncoming.value ? relationType.value.max_incoming : relationType.value.max_outgoing
  return limit !== 1
})

const effectiveValue = computed(() => (isIncoming.value ? incomingValue.value : props.value))

// Whether a source picks a face: an incoming content-scoped edge from a type
// with faces. Its candidates are one row per face (faceCandidates).
function picksFace(sourceType: string): boolean {
  return isIncoming.value && relationType.value?.scope === 'content' && hasFaces(sourceType)
}

// The key a candidate is selected under: its address when it picks a face,
// else its id.
function candidateKey(e: Entity): string {
  return picksFace(e.type) ? entityRef(e) : e.id
}

const candidatesByKey = computed(() => new Map(candidates.value.map((c) => [candidateKey(c), c])))

// One chip of an incoming picker. `face` names the source face the edge
// belongs to ('' for an identity edge); `editable` is false when the server
// reports the principal may not remove it.
interface IncomingChip {
  key: string
  entity: Entity
  face: string
  editable: boolean
}

const incomingChips = computed<IncomingChip[]>(() => {
  const loaded = new Map(incomingLoadedEntries.value.map((e) => [e.id, e]))
  return incomingValue.value.map((key) => {
    const entry = loaded.get(key)
    const entity =
      candidatesByKey.value.get(key) ??
      knownById.value.get(refBareId(key)) ??
      ({ id: refBareId(key), type: entry?.type ?? '', properties: {} } as Entity)
    return { key, entity, face: entry?.face ?? refFace(key), editable: entry?.editable !== false }
  })
})

// The chips grouped by face, in first-seen order. A widget with no faced row
// has one group with face ''.
const incomingGroups = computed(() => {
  const groups = new Map<string, IncomingChip[]>()
  for (const chip of incomingChips.value) {
    const g = groups.get(chip.face) ?? []
    g.push(chip)
    groups.set(chip.face, g)
  }
  return [...groups.entries()].map(([face, chips]) => ({ face, chips }))
})

const knownById = computed(() => indexKnownEntities(resolvedLinks.value, candidates.value))

const selectedEntities = computed(() => resolveSelected(effectiveValue.value, knownById.value))

// Note the asymmetry this leaves (pre-existing, widened by BUG-LSCDJK): the
// dropdown offers `candidates` only, so an out-of-page link the user can now
// SEE and remove cannot be re-added from here. Making search hit the search
// endpoint instead of the cached page is the real fix, and is its own change.
const filteredCandidates = computed(() => {
  const offered = candidates.value.filter(
    (c) => !effectiveValue.value.includes(candidateKey(c)) && (!picksFace(c.type) || offersFace(c))
  )
  if (!searchQuery.value) return offered
  const query = searchQuery.value.toLowerCase()
  return offered.filter(
    (c) => c.id.toLowerCase().includes(query) || (c._title ?? '').toLowerCase().includes(query)
  )
})

// Methods
// Bumped per loadCandidates run, so a slower earlier run (a mount racing the
// reload after an inline create) cannot overwrite a newer result.
let candidatesGeneration = 0

async function loadCandidates() {
  const generation = ++candidatesGeneration
  loading.value = true
  try {
    // Target types load in parallel; Promise.all keeps their order.
    const perType = await Promise.all(targetTypes.value.map(loadTypeCandidates))
    if (generation !== candidatesGeneration) return
    const hints = new Map<string, string>()
    for (const { offWorld } of perType) offWorld.forEach((face, key) => hints.set(key, face))
    candidates.value = perType.flatMap((t) => t.rows)
    offWorldFaces.value = hints
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load relation candidates:', err)
  } finally {
    if (generation === candidatesGeneration) loading.value = false
  }
}

// The candidates of one target type, and the face label of each row the
// ambient world does not serve, keyed by offWorldKey.
async function loadTypeCandidates(
  targetType: string
): Promise<{ rows: Entity[]; offWorld: Map<string, string> }> {
  // fetchAllList, not fetchList: `candidates` is not just what the dropdown
  // offers, it is also the ONLY source for `buildOutgoingTypes`, which must
  // name the type of every ALREADY-LINKED target — including ones the user
  // will never scroll to. One page left anything past the first 100
  // unresolvable, so `reshapeLegacyToModern` returned null and the form's
  // entire relations autosave aborted with "unknown types" (BUG-HOB9BR).
  const ambient = entitiesStore.fetchAllList(targetType, {
    ...(worldParam.value ? { world: worldParam.value } : {}),
  })
  // A faced target is widened to every face the reader may read: the
  // relation's head is the entity, so a face the ambient world excludes
  // is still a valid target (DEC-NPZICR, BUG-FYEEVX). A faceless type
  // resolves to its one row in every world, so it needs no second query.
  const worlds = hasFaces(targetType) ? widenWorlds(schemaStore.worlds, ambientWorld()) : []
  const widened = Promise.allSettled(
    worlds.map((w) => entitiesStore.fetchAllList(targetType, { world: w }))
  )
  // The ambient list is the core of the picker, and its failure fails the load.
  // A widened list is an extra: one that fails is logged and left out, so the
  // rows the ambient world serves are still offered.
  const result = await ambient
  const others: Entity[][] = []
  const perFace = picksFace(targetType)
  for (const [i, settled] of (await widened).entries()) {
    if (settled.status === 'fulfilled') {
      others.push(settled.value.data)
      warnIfTruncated(targetType, settled.value.meta.has_more)
    } else if (!isCancelledFetch(settled.reason)) {
      console.error(
        `RelationPicker: candidates for "${targetType}" in world "${worlds[i]}" failed:`,
        settled.reason
      )
    }
  }
  warnIfTruncated(targetType, result.meta.has_more)
  if (perFace) {
    // Every row names its face: the face is what the user picks.
    const rows = faceCandidates([result.data, ...others])
    const labels = new Map<string, string>()
    for (const e of rows) {
      labels.set(offWorldKey(e), schemaStore.faceLabel(e.type, refFace(entityRef(e))))
    }
    return { rows, offWorld: labels }
  }
  const merged = mergeFamilyCandidates(result.data, others)
  const offWorld = new Map<string, string>()
  for (const e of merged.rows) {
    if (merged.offWorld.has(e.id)) {
      offWorld.set(offWorldKey(e), schemaStore.faceLabel(e.type, refFace(entityRef(e))))
    }
  }
  return { rows: merged.rows, offWorld }
}

// has_more on a MERGED all-pages response means `listAllEntities` hit its
// 50-page cap, so the set is knowingly incomplete and BUG-HOB9BR is live
// again past that boundary. Worth a warning specifically because the
// user-facing message for that failure ("reload the form and try again")
// is advice that cannot work — a reload refetches the same 50 pages.
// Same reasoning as KanbanView's truncation banner.
function warnIfTruncated(targetType: string, hasMore: boolean | undefined) {
  if (!hasMore) return
  console.warn(
    `RelationPicker: candidate list for "${targetType}" is truncated at the ` +
      `page cap; relation saves may fail for targets beyond it (BUG-HOB9BR).`
  )
}

// Resolve the type of every linked entity the candidate page omits
// (BUG-LSCDJK); see `outOfPageLinks.ts` for why the candidate page cannot
// answer this and the edges can.
//
// Best-effort by design: on failure we keep whatever the candidate page
// resolved. Turning a transient lookup error into a blocked save would
// reproduce the very symptom this fixes, and there is no wipe risk — an
// unresolved id yields no type, so reshapeLegacyToModern refuses to emit a
// malformed identifier rather than writing a wrong one.
//
// The failure is logged, not surfaced: the user sees a missing chip, then a
// reload-toast at save time, with nothing connecting the two. A deliberate
// deferral — a toast per transient lookup error is its own noise problem, and
// the refusal path keeps the data correct meanwhile.
async function resolveOutOfPageLinks() {
  if (isIncoming.value || !props.field.relation || !props.entityId) return
  // `effectiveValue`, not `props.value`: identical for an outgoing picker, but
  // reading what `selectedEntities` renders keeps the two from drifting if the
  // incoming guard above is ever relaxed.
  const missing = missingIds(effectiveValue.value, knownById.value)
  if (missing.length === 0) return
  try {
    const edges = await getEntityRelations(props.entityType, props.entityId, props.field.relation)
    resolvedLinks.value = mergeResolvedLinks(resolvedLinks.value, edges, new Set(missing))
  } catch (err) {
    if (isCancelledFetch(err)) return
    console.error('Failed to resolve linked entities outside the candidate page:', err)
  }
}

// Re-resolve when the linked set changes after mount. The picker is keyed on
// DynamicForm's `saveGeneration`, never incremented, so a post-mount reload (a
// committed transition, an attachment change) reassigns `relations.value`
// WITHOUT remounting this component — resolving only at mount would leave such
// an id typeless. Free when nothing is missing: the resolve returns before any
// request once every id is known.
watch(
  () => effectiveValue.value.join('\u0000'),
  async () => {
    await resolveOutOfPageLinks()
    // Re-emit: the parent's map was built from what was known at the last
    // emit, so a type resolved just now would otherwise never reach it,
    // leaving the save to abort on an id this component has already resolved.
    if (!isIncoming.value && props.value.length > 0) {
      emit('update:types', buildOutgoingTypes(props.value))
    }
  }
)

async function loadIncomingValue() {
  if (!isIncoming.value || !props.field.relation) return
  // Create mode: the entity doesn't exist yet, so there are no existing
  // incoming edges to load. Establish an empty baseline and mark the
  // picker loaded so selections emit as pure additions. Without this,
  // `incomingLoaded` stays false and emitIncomingDiff() no-ops every
  // pick — the load-failure-cannot-wipe guard (TKT-GFQK) wrongly
  // suppressing additions on a brand-new entity (BUG-10IPBP).
  if (!props.entityId) {
    incomingValue.value = []
    incomingOriginal.value = []
    incomingLoadedEntries.value = []
    incomingLoaded.value = true
    return
  }
  try {
    const edges = await getEntityRelations(
      props.entityType,
      props.entityId,
      props.field.relation,
      'incoming'
    )
    const rows = edges.map(addressedEntry)
    const ids = rows.map((e) => e.id)
    incomingValue.value = ids
    incomingOriginal.value = [...ids]
    incomingLoadedEntries.value = rows
    incomingLoaded.value = true
  } catch (err) {
    if (isCancelledFetch(err)) return
    console.error('Failed to load incoming relations:', err)
    // Stay inert (incomingLoaded=false) so any user-triggered emit
    // is a no-op until load succeeds. Prevents wipe-on-load-failure.
  }
}

// emitIncomingDiff sends the current desired peer set to DynamicForm
// (TKT-GFQK). The payload includes the loaded snapshot AND the
// current entries so DynamicForm can build a RelationCardState whose
// `entries` field reflects the post-edit set (driving the inverse-
// keyed body in buildRelationsPatch).
//
// Emits ONLY if the load succeeded, so a failed load can't manifest
// as a save-time `data: []` wipe (load-failure-cannot-wipe).
function emitIncomingDiff() {
  if (!incomingLoaded.value) return
  const original = new Set(incomingOriginal.value)
  const current = new Set(incomingValue.value)
  const added = incomingValue.value
    .filter((id) => !original.has(id))
    .map((id) => ({ targetId: id }))
  const removed = incomingOriginal.value.filter((id) => !current.has(id))

  // Build currentEntries from loaded snapshot + any newly-added peer
  // (whose type comes from candidates). Removed entries are excluded.
  const loadedById = new Map(incomingLoadedEntries.value.map((e) => [e.id, e]))
  const currentEntries: RelationEntry[] = []
  for (const id of incomingValue.value) {
    const fromLoaded = loadedById.get(id)
    if (fromLoaded) {
      currentEntries.push(fromLoaded)
      continue
    }
    // Newly-added: look up the type from candidates. The key already names
    // the face a content-scoped edge from a faced peer hangs on.
    const cand = candidatesByKey.value.get(id)
    if (cand) {
      currentEntries.push({ id, type: cand.type, direction: 'incoming' })
    }
  }
  emit('incoming-changed', {
    loadedEntries: incomingLoadedEntries.value,
    currentEntries,
    added,
    removed,
  })
}

// Build a Map<id, type> from candidates for the current outgoing
// selection. Used to feed DynamicForm's pickerTypes so the unified
// PATCH builder can populate `type` per resource identifier without
// guessing via `to[0]` or `id_prefix`.
function buildOutgoingTypes(ids: string[]): Map<string, string> {
  const known = knownById.value
  const out = new Map<string, string>()
  for (const id of ids) {
    const entity = known.get(id)
    if (entity) out.set(id, entity.type)
  }
  return out
}

function selectEntity(entity: Entity) {
  if (isIncoming.value) {
    const key = candidateKey(entity)
    incomingValue.value = isMulti.value ? [...incomingValue.value, key] : [key]
    emitIncomingDiff()
  } else {
    const next = isMulti.value ? [...props.value, entity.id] : [entity.id]
    emit('update', next)
    emit('update:types', buildOutgoingTypes(next))
  }
  searchQuery.value = ''
  showDropdown.value = false
}

function removeEntity(entityId: string) {
  if (isIncoming.value) {
    incomingValue.value = incomingValue.value.filter((id) => id !== entityId)
    emitIncomingDiff()
  } else {
    const next = props.value.filter((id) => id !== entityId)
    emit('update', next)
    emit('update:types', buildOutgoingTypes(next))
  }
}

/**
 * Whether a row's face is worth naming.
 *
 * The badge exists to flag a SURPRISE, so it renders only when the row is not
 * the world's prime — `via: 'chain'` with `chain_position > 0` (a stand-in from
 * later in the chain) or a `fallback-default`. In the default world every row
 * is the default face, so a badge on every row is noise that trains the reader
 * to ignore it.
 *
 * WorldBadge now enforces this rule itself, so it is the single source of both
 * the vocabulary and the decision. The check is repeated here only to skip
 * MOUNTING the component for an ordinary row — a pure saving in a list that
 * can render hundreds of candidates, not a second opinion. If the two ever
 * disagree, WorldBadge wins: it renders nothing.
 */
function showsFaceBadge(entity: Entity): boolean {
  // An off-world row's `_world` describes the world that served it, not the
  // ambient one, so its stand-in wording would mislead. It names its face
  // instead (offWorldFace).
  if (offWorldFaces.value.has(offWorldKey(entity))) return false
  const w = entity._world
  if (!w) return false
  if (w.via === 'fallback-default') return true
  return w.via === 'chain' && (w.chain_position ?? 0) > 0
}

function hasFaces(entityType: string): boolean {
  return Object.keys(schemaStore.getEntityType(entityType)?.faces ?? {}).length > 0
}

// The world the candidate query reads in: the URL's, else the operator's
// default. '' is the default world.
function ambientWorld(): string {
  return world.value || schemaStore.defaultWorld || DEFAULT_WORLD
}

function offWorldKey(entity: Entity): string {
  return `${entity.type}/${entityRef(entity)}`
}

// The face label for a candidate the ambient world does not serve, so the
// reader can tell a draft-only target from one the world shows. '' otherwise.
function offWorldFace(entity: Entity): string {
  return offWorldFaces.value.get(offWorldKey(entity)) ?? ''
}

function formatEntityLabel(entity: Entity): string {
  // _title is the metamodel-aware display title from the API, falling back
  // to id when the entity type has no display property set.
  return entityDisplayTitleWithId(entity)
}

// Inline-create targets for this relation's candidate types. A type appears
// only when the principal may create it AND a create form resolves — the
// server decides both, so there is no permission arithmetic here. Empty inside
// an already-nested form (the depth cap).
const inlineCreateTargets = useInlineCreate(targetTypes)

function openCreateModal(target: { entityType: string; formId: string }) {
  createTargetType.value = target.entityType
  createFormId.value = target.formId
  showCreateModal.value = true
  showDropdown.value = false
}

// Clear the form id too, so the modal component unmounts rather than lingering
// hidden with its stale focus/modal-stack state.
function closeCreateModal() {
  showCreateModal.value = false
  createFormId.value = ''
}

function handleEntityCreated(entity: Entity) {
  closeCreateModal()
  // Push into candidates before selecting. `loadCandidates` now fetches every
  // page, but this entity did not EXIST when it ran, so it is in none of them.
  // Without the push it is unresolvable for both display and — via
  // `buildOutgoingTypes` — its type, which is the BUG-HOB9BR failure again.
  // Do not drop this on the grounds that candidates are complete: they are
  // complete as of the fetch, and this entity postdates it.
  candidates.value.push(entity)
  selectEntity(entity)
}

// Lifecycle
onMounted(async () => {
  await loadCandidates()
  // Fill in the links the candidate page missed before emitting, so the first
  // emit is already complete (BUG-LSCDJK). It needs only `loadCandidates` (to
  // know what is missing), so it runs alongside the incoming load rather than
  // adding a third serial hop to a form that may mount several pickers.
  await Promise.all([loadIncomingValue(), resolveOutOfPageLinks()])
  // Surface types for any pre-existing outgoing selection so the
  // submit-time PATCH builder knows the type even when the user
  // didn't touch this widget.
  if (!isIncoming.value && props.value.length > 0) {
    emit('update:types', buildOutgoingTypes(props.value))
  }
})

// Close dropdown when clicking outside
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.relation-picker')) {
    showDropdown.value = false
  }
}

watch(showDropdown, (show) => {
  if (show) {
    document.addEventListener('click', handleClickOutside)
  } else {
    document.removeEventListener('click', handleClickOutside)
  }
})

// Clean up event listener on unmount
onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div class="form-field relation-picker">
    <label>
      {{ label }}
    </label>

    <!-- Incoming: grouped per source face; a row the principal may not
         remove is locked. -->
    <template v-if="isIncoming">
      <div v-for="group in incomingGroups" :key="group.face" class="selected-group">
        <span v-if="group.face" class="group-face">{{
          schemaStore.faceLabel(group.chips[0].entity.type, group.face)
        }}</span>
        <div class="selected-entities">
          <div v-for="chip in group.chips" :key="chip.key" class="selected-entity">
            <span class="entity-type">{{ chip.entity.type }}</span>
            <span class="entity-label">{{ formatEntityLabel(chip.entity) }}</span>
            <span
              v-if="!chip.editable"
              class="lock"
              title="You cannot change this face's relations"
              aria-label="read-only"
              >🔒</span
            >
            <button
              v-else-if="canRemove"
              type="button"
              class="remove-btn"
              @click="removeEntity(chip.key)"
            >
              &times;
            </button>
          </div>
        </div>
      </div>
    </template>

    <!-- Selected entities -->
    <div v-else-if="selectedEntities.length" class="selected-entities">
      <div v-for="entity in selectedEntities" :key="entity.id" class="selected-entity">
        <span class="entity-type">{{ entity.type }}</span>
        <span class="entity-label">{{ formatEntityLabel(entity) }}</span>
        <!--
          TRAILING, after the title: the type chip leads and the face badge
          follows, matching how the rest of the app orders the two.
        -->
        <WorldBadge
          v-if="showsFaceBadge(entity)"
          :world="entity._world"
          :entity-type="entity.type"
        />
        <span v-if="offWorldFace(entity)" class="face-hint">{{ offWorldFace(entity) }}</span>
        <button v-if="canRemove" type="button" class="remove-btn" @click="removeEntity(entity.id)">
          &times;
        </button>
      </div>
    </div>

    <!-- Search input (TKT-G7N5: hidden when relation is not creatable) -->
    <div v-if="canCreate" class="search-wrapper">
      <input
        v-model="searchQuery"
        type="text"
        role="combobox"
        :aria-expanded="showDropdown"
        aria-haspopup="listbox"
        aria-autocomplete="list"
        :placeholder="`Search ${targetTypes.join(', ')}...`"
        @focus="showDropdown = true"
        @input="showDropdown = true"
      />

      <!-- Dropdown -->
      <div v-if="showDropdown && !loading" class="dropdown" role="listbox">
        <div v-if="filteredCandidates.length === 0" class="dropdown-empty">
          No matching entities found
        </div>
        <div
          v-for="entity in filteredCandidates.slice(0, 10)"
          v-else
          :key="entity.id"
          class="dropdown-item"
          role="option"
          @click="selectEntity(entity)"
        >
          <span class="entity-type">{{ entity.type }}</span>
          <span class="entity-label">{{ formatEntityLabel(entity) }}</span>
          <WorldBadge
            v-if="showsFaceBadge(entity)"
            :world="entity._world"
            :entity-type="entity.type"
          />
          <span v-if="offWorldFace(entity)" class="face-hint">{{ offWorldFace(entity) }}</span>
        </div>
        <div v-if="filteredCandidates.length > 10" class="dropdown-more">
          +{{ filteredCandidates.length - 10 }} more...
        </div>
        <!-- Add new buttons -->
        <div v-if="inlineCreateTargets.length > 0" class="dropdown-actions">
          <button
            v-for="target in inlineCreateTargets"
            :key="target.entityType"
            type="button"
            class="add-new-btn"
            @click.stop="openCreateModal(target)"
          >
            + New {{ target.label }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="loading-indicator">Loading...</div>
    </div>

    <p v-if="help" class="field-help">{{ help }}</p>

    <!-- Inline Create Modal -->
    <InlineCreateFormModal
      v-if="createFormId"
      :show="showCreateModal"
      :form-id="createFormId"
      :entity-type="createTargetType"
      @close="closeCreateModal"
      @created="handleEntityCreated"
    />
  </div>
</template>

<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field label {
  font-size: 14px;
  font-weight: 500;
  color: var(--rl-color-text);
}

.selected-entities {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.selected-entity {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px 4px 10px;
  background: var(--rl-color-bg-hover);
  border-radius: 4px;
  font-size: 13px;
}

.selected-entity .entity-type {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--rl-color-text-muted);
  background: var(--rl-color-border);
  padding: 2px 4px;
  border-radius: 2px;
}

.selected-entity .entity-label {
  color: var(--rl-color-text);
}

.remove-btn {
  background: none;
  border: none;
  color: var(--rl-color-text-muted);
  font-size: 18px;
  cursor: pointer;
  padding: 0 2px;
  line-height: 1;
}

.remove-btn:hover {
  color: var(--rl-color-danger, #ef4444);
}

.selected-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-face {
  font-size: 12px;
  color: var(--rl-color-text-muted);
}

.lock {
  font-size: 12px;
}

.face-hint {
  margin-left: 0.35rem;
  font-size: 0.72rem;
  color: var(--muted-text);
}

.search-wrapper {
  position: relative;
}

.search-wrapper input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--rl-color-border);
  border-radius: 6px;
  font-size: 14px;
  background: var(--rl-color-bg-raised);
  color: var(--rl-color-text);
}

.search-wrapper input:focus {
  outline: none;
  border-color: var(--rl-color-accent);
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

.dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  margin-top: 4px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 100;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.dropdown-item:hover {
  background: var(--rl-color-bg-hover);
}

.dropdown-item .entity-type {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--rl-color-text-muted);
  background: var(--rl-color-border);
  padding: 2px 4px;
  border-radius: 2px;
}

.dropdown-item .entity-label {
  flex: 1;
  font-size: 14px;
  color: var(--rl-color-text);
}

.dropdown-empty,
.dropdown-more {
  padding: 12px;
  text-align: center;
  color: var(--rl-color-text-muted);
  font-size: 13px;
}

.dropdown-actions {
  border-top: 1px solid var(--rl-color-border);
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.add-new-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px 12px;
  background: var(--rl-color-bg-hover);
  border: 1px dashed var(--rl-color-border);
  border-radius: 4px;
  color: var(--rl-color-accent, #6366f1);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.add-new-btn:hover {
  background: var(--rl-color-accent, #6366f1);
  border-color: var(--rl-color-accent, #6366f1);
  color: white;
}

.loading-indicator {
  padding: 8px 12px;
  color: var(--rl-color-text-muted);
  font-size: 13px;
}

.field-help {
  font-size: 13px;
  color: var(--rl-color-text-muted);
  margin: 0;
}
</style>
