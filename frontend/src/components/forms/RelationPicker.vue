<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { isCancelledFetch } from '@/composables/usePageData'
import { getEntityRelations } from '@/api'
import type { FormFieldOrRelation, Entity, RelationEntry, RelationAffordance } from '@/types'
import InlineCreateModal from './InlineCreateModal.vue'

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

// State
const loading = ref(false)
const candidates = ref<Entity[]>([])
const searchQuery = ref('')
const showDropdown = ref(false)
const showCreateModal = ref(false)
const createTargetType = ref('')

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
const incomingValue = ref<string[]>([])
const incomingOriginal = ref<string[]>([])
// Snapshot of the loaded edges keyed by ID. Used by the new
// emitIncomingDiff to construct a RelationEntry-shaped payload
// without a second GET. Empty until loadIncomingValue succeeds.
const incomingLoadedEntries = ref<RelationEntry[]>([])
const incomingLoaded = ref(false)

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

const label = computed(() => props.field.label || props.field.relation || '')
const help = computed(() => props.field.help || relationType.value?.description || '')

const isMulti = computed(() => {
  // For incoming, cardinality is bounded by `max_incoming` (how many sources
  // may point at us), not `max_outgoing`.
  if (!relationType.value) return true
  const limit = isIncoming.value ? relationType.value.max_incoming : relationType.value.max_outgoing
  return limit !== 1
})

const effectiveValue = computed(() => (isIncoming.value ? incomingValue.value : props.value))

const selectedEntities = computed(() => {
  return candidates.value.filter((c) => effectiveValue.value.includes(c.id))
})

const filteredCandidates = computed(() => {
  if (!searchQuery.value) {
    return candidates.value.filter((c) => !effectiveValue.value.includes(c.id))
  }
  const query = searchQuery.value.toLowerCase()
  return candidates.value.filter(
    (c) =>
      !effectiveValue.value.includes(c.id) &&
      (c.id.toLowerCase().includes(query) ||
        (c._title ?? '').toLowerCase().includes(query))
  )
})

// Methods
async function loadCandidates() {
  loading.value = true
  try {
    const allCandidates: Entity[] = []
    for (const targetType of targetTypes.value) {
      const result = await entitiesStore.fetchList(targetType, { per_page: 100 })
      allCandidates.push(...result.data)
    }
    candidates.value = allCandidates
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load relation candidates:', err)
  } finally {
    loading.value = false
  }
}

async function loadIncomingValue() {
  if (!isIncoming.value || !props.entityId || !props.field.relation) return
  try {
    const edges = await getEntityRelations(
      props.entityType,
      props.entityId,
      props.field.relation,
      'incoming',
    )
    const ids = edges.map((e) => e.id)
    incomingValue.value = ids
    incomingOriginal.value = [...ids]
    incomingLoadedEntries.value = edges
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
    // Newly-added: look up the type from candidates.
    const cand = candidates.value.find((c) => c.id === id)
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
  const out = new Map<string, string>()
  for (const c of candidates.value) {
    if (ids.includes(c.id)) out.set(c.id, c.type)
  }
  return out
}

function selectEntity(entity: Entity) {
  if (isIncoming.value) {
    incomingValue.value = isMulti.value
      ? [...incomingValue.value, entity.id]
      : [entity.id]
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

function formatEntityLabel(entity: Entity): string {
  // _title is the metamodel-aware display title from the API, falling back to id
  // when the entity type has no display property set. Matches EntityDetail.vue.
  if (entity._title && entity._title !== entity.id) {
    return `${entity._title} (${entity.id})`
  }
  return entity.id
}

function openCreateModal(targetType: string) {
  createTargetType.value = targetType
  showCreateModal.value = true
  showDropdown.value = false
}

function handleEntityCreated(entity: Entity) {
  // Add to candidates and select it
  candidates.value.push(entity)
  selectEntity(entity)
}

// Lifecycle
onMounted(async () => {
  await loadCandidates()
  await loadIncomingValue()
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

    <!-- Selected entities -->
    <div v-if="selectedEntities.length" class="selected-entities">
      <div
        v-for="entity in selectedEntities"
        :key="entity.id"
        class="selected-entity"
      >
        <span class="entity-type">{{ entity.type }}</span>
        <span class="entity-label">{{ formatEntityLabel(entity) }}</span>
        <button
          v-if="canRemove"
          type="button"
          class="remove-btn"
          @click="removeEntity(entity.id)"
        >
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
        </div>
        <div v-if="filteredCandidates.length > 10" class="dropdown-more">
          +{{ filteredCandidates.length - 10 }} more...
        </div>
        <!-- Add new buttons -->
        <div v-if="targetTypes.length > 0" class="dropdown-actions">
          <button
            v-for="targetType in targetTypes"
            :key="targetType"
            type="button"
            class="add-new-btn"
            @click.stop="openCreateModal(targetType)"
          >
            + Add new {{ schemaStore.getEntityType(targetType)?.label || targetType }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="loading-indicator">
        Loading...
      </div>
    </div>

    <p v-if="help" class="field-help">{{ help }}</p>

    <!-- Inline Create Modal -->
    <InlineCreateModal
      :show="showCreateModal"
      :entity-type="createTargetType"
      @close="showCreateModal = false"
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
  color: var(--text-color);
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
  background: var(--hover-bg);
  border-radius: 4px;
  font-size: 13px;
}

.selected-entity .entity-type {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 4px;
  border-radius: 2px;
}

.selected-entity .entity-label {
  color: var(--text-color);
}

.remove-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 18px;
  cursor: pointer;
  padding: 0 2px;
  line-height: 1;
}

.remove-btn:hover {
  color: var(--error-color, #ef4444);
}

.search-wrapper {
  position: relative;
}

.search-wrapper input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.search-wrapper input:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
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
  background: var(--hover-bg);
}

.dropdown-item .entity-type {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 4px;
  border-radius: 2px;
}

.dropdown-item .entity-label {
  flex: 1;
  font-size: 14px;
  color: var(--text-color);
}

.dropdown-empty,
.dropdown-more {
  padding: 12px;
  text-align: center;
  color: var(--muted-text);
  font-size: 13px;
}

.dropdown-actions {
  border-top: 1px solid var(--border-color);
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
  background: var(--hover-bg);
  border: 1px dashed var(--border-color);
  border-radius: 4px;
  color: var(--accent-color, #6366f1);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.add-new-btn:hover {
  background: var(--accent-color, #6366f1);
  border-color: var(--accent-color, #6366f1);
  color: white;
}

.loading-indicator {
  padding: 8px 12px;
  color: var(--muted-text);
  font-size: 13px;
}

.field-help {
  font-size: 13px;
  color: var(--muted-text);
  margin: 0;
}
</style>
