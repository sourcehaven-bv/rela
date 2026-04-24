<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { isCancelledFetch } from '@/composables/usePageData'
import { getEntityRelations } from '@/api'
import type { FormFieldOrRelation, Entity } from '@/types'
import InlineCreateModal from './InlineCreateModal.vue'

// Reverse-relation diff payload. Mirrors RelationCardState (without the
// `updated` channel — pickers don't edit relation properties) so DynamicForm
// can route incoming-picker changes through its existing direction-aware
// save reconciler.
export interface RelationPickerIncomingState {
  added: Array<{ targetId: string }>
  removed: string[]
}

const props = defineProps<{
  field: FormFieldOrRelation
  entityType: string
  entityId?: string
  value: string[]
}>()

const emit = defineEmits<{
  update: [value: string[]]
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
const isIncoming = computed(() => props.field.direction === 'incoming')
const incomingValue = ref<string[]>([])
const incomingOriginal = ref<string[]>([])

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
        String(c.properties.title || '').toLowerCase().includes(query))
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
  } catch (err) {
    if (isCancelledFetch(err)) return
    console.error('Failed to load incoming relations:', err)
  }
}

function emitIncomingDiff() {
  const original = new Set(incomingOriginal.value)
  const current = new Set(incomingValue.value)
  const added = incomingValue.value
    .filter((id) => !original.has(id))
    .map((id) => ({ targetId: id }))
  const removed = incomingOriginal.value.filter((id) => !current.has(id))
  emit('incoming-changed', { added, removed })
}

function selectEntity(entity: Entity) {
  if (isIncoming.value) {
    incomingValue.value = isMulti.value
      ? [...incomingValue.value, entity.id]
      : [entity.id]
    emitIncomingDiff()
  } else if (isMulti.value) {
    emit('update', [...props.value, entity.id])
  } else {
    emit('update', [entity.id])
  }
  searchQuery.value = ''
  showDropdown.value = false
}

function removeEntity(entityId: string) {
  if (isIncoming.value) {
    incomingValue.value = incomingValue.value.filter((id) => id !== entityId)
    emitIncomingDiff()
  } else {
    emit('update', props.value.filter((id) => id !== entityId))
  }
}

function getEntityLabel(entity: Entity): string {
  return String(entity.properties.title || entity.id)
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
        <span class="entity-label">{{ getEntityLabel(entity) }}</span>
        <button type="button" class="remove-btn" @click="removeEntity(entity.id)">
          &times;
        </button>
      </div>
    </div>

    <!-- Search input -->
    <div class="search-wrapper">
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
          <span class="entity-id">{{ entity.id }}</span>
          <span class="entity-label">{{ getEntityLabel(entity) }}</span>
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

.dropdown-item .entity-id {
  font-family: monospace;
  font-size: 12px;
  color: var(--muted-text);
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
