<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import SlimSelect from 'slim-select/vue'
import 'slim-select/styles'
import { useSchemaStore } from '@/stores'
import {
  getEntityRelations,
  searchEntities,
  getEntity,
} from '@/api'
import type { FormFieldOrRelation, RelationProperty } from '@/types/config'
import type { RelationEntry, Entity } from '@/types/entity'
import type { PropertyDef } from '@/types/schema'
import type { RelationCardState } from './relationsPatch'
import type { RelationAffordance } from '@/types'

// Re-export so existing `import type { RelationCardState } from './RelationCards.vue'`
// callers keep working without a churn rename.
export type { RelationCardState }

const props = defineProps<{
  field: FormFieldOrRelation
  entityType: string
  entityId: string
  // TKT-G7N5: per-relation-type affordance verdict from the server.
  // Undefined / fields all-undefined = default (everything allowed).
  // `creatable === false` hides the + Add button; `removable === false`
  // hides every per-link x; `fields[name].writable === false` disables
  // the inline meta-field input. Per-relation-type uniform.
  verdict?: RelationAffordance
}>()

const emit = defineEmits<{
  'cards-changed': [payload: RelationCardState]
}>()

const router = useRouter()
const schemaStore = useSchemaStore()

// State
const entries = ref<RelationEntry[]>([])
const originalEntries = ref<RelationEntry[]>([])
const entityCache = ref<Map<string, Entity>>(new Map())
const loading = ref(false)
const error = ref<string | null>(null)

// Track which entries have been touched
const addedIds = ref<Set<string>>(new Set())
const removedIds = ref<Set<string>>(new Set())
const updatedIds = ref<Set<string>>(new Set())

// Add relation state
const showAddSearch = ref(false)
const searchQuery = ref('')
const searchResults = ref<Entity[]>([])
const searching = ref(false)
const selectedTarget = ref<Entity | null>(null)
const newMeta = ref<Record<string, unknown>>({})

const isIncoming = computed(() => props.field.direction === 'incoming')

// TKT-G7N5: per-relation-type affordance helpers. Defaults preserve
// today's behavior — buttons render unless explicitly denied.
const canCreate = computed(() => props.verdict?.creatable !== false)
const canRemove = computed(() => props.verdict?.removable !== false)

// Per-meta-field disabled lookup. Returns true when the verdict
// reports writable=false for the named meta field. Absence in the
// fields map = writable default.
function isMetaFieldDisabled(propName: string): boolean {
  return props.verdict?.fields?.[propName]?.writable === false
}

const relationType = computed(() => {
  if (!props.field.relation) return undefined
  return schemaStore.getRelationType(props.field.relation)
})

const relationProperties = computed<Record<string, PropertyDef>>(() => {
  return relationType.value?.properties ?? {}
})

const targetTypes = computed(() => {
  if (!relationType.value) return []
  return isIncoming.value ? relationType.value.from : relationType.value.to
})

const fieldProperties = computed<RelationProperty[]>(() => props.field.properties ?? [])

const canLink = computed(() => {
  if (!selectedTarget.value) return false
  for (const prop of fieldProperties.value) {
    if (prop.required && !newMeta.value[prop.property]) return false
  }
  return true
})

const hasPendingChanges = computed(() => {
  return addedIds.value.size > 0 || removedIds.value.size > 0 || updatedIds.value.size > 0
})

// Load relations on mount
onMounted(() => {
  loadRelations()
})

async function loadRelations() {
  if (!props.field.relation) return
  loading.value = true
  error.value = null
  try {
    const direction = props.field.direction === 'incoming' ? 'incoming' : undefined
    const loaded = await getEntityRelations(props.entityType, props.entityId, props.field.relation, direction)
    entries.value = loaded
    // Deep copy for diffing
    originalEntries.value = JSON.parse(JSON.stringify(loaded))
    // Reset change tracking
    addedIds.value.clear()
    removedIds.value.clear()
    updatedIds.value.clear()
    // Fetch entity details in parallel
    const uncached = loaded.filter((entry) => !entityCache.value.has(entry.id))
    const results = await Promise.allSettled(
      uncached.map((entry) =>
        getEntity('', entry.id, { fields: 'id,type,properties.title' }).then((entity) => ({
          id: entry.id,
          entity,
        }))
      )
    )
    for (const result of results) {
      if (result.status === 'fulfilled') {
        entityCache.value.set(result.value.id, result.value.entity)
      }
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load relations'
  } finally {
    loading.value = false
  }
}

function getEntityTitle(id: string): string {
  const entity = entityCache.value.get(id)
  if (!entity) return id
  return String(entity.properties.title || entity._title || id)
}

function navigateToEntity(id: string) {
  const entity = entityCache.value.get(id)
  if (entity) {
    router.push(`/entity/${entity.type}/${id}`)
  }
}

function emitUpdate() {
  const added: RelationCardState['added'] = []
  const removed: string[] = [...removedIds.value]
  const updated: RelationCardState['updated'] = []

  for (const entry of entries.value) {
    if (addedIds.value.has(entry.id)) {
      added.push({ targetId: entry.id, meta: entry.meta ? { ...entry.meta } : undefined })
    } else if (updatedIds.value.has(entry.id)) {
      updated.push({ targetId: entry.id, meta: entry.meta ? { ...entry.meta } : {} })
    }
  }

  emit('cards-changed', {
    entries: entries.value,
    added,
    removed,
    updated,
  })
}

// Property editing - mutate in-place to preserve component instances (SlimSelect etc.)
function updateProperty(entryId: string, property: string, value: unknown) {
  const entry = entries.value.find((e) => e.id === entryId)
  if (!entry) return
  if (!entry.meta) entry.meta = {}
  entry.meta[property] = value

  // Mark as updated (unless it was newly added - those are already tracked)
  if (!addedIds.value.has(entryId)) {
    updatedIds.value.add(entryId)
  }
  emitUpdate()
}

function getPropertyDef(propName: string): PropertyDef | undefined {
  return relationProperties.value[propName]
}

function getEnumValues(propName: string): string[] {
  const def = getPropertyDef(propName)
  if (!def) return []
  if (def.values?.length) return def.values
  const customType = schemaStore.customTypes.get(def.type)
  if (customType?.values?.length) return customType.values
  return []
}

function getInputType(propName: string): string {
  const def = getPropertyDef(propName)
  if (!def) return 'text'
  if (def.type === 'date') return 'date'
  if (def.type === 'integer') return 'number'
  if (def.type === 'boolean') return 'checkbox'
  return 'text'
}


// Memoized SlimSelect data per property name — stable references prevent
// the deep watcher on :data from calling setData and resetting selection.
const slimDataCache = new Map<string, { text: string; value: string; placeholder?: boolean }[]>()

function slimSelectData(propName: string) {
  const cached = slimDataCache.get(propName)
  if (cached) return cached
  const values = getEnumValues(propName)
  const data = [
    { text: 'Select...', value: '', placeholder: true as const },
    ...values.map((v) => ({ text: v, value: v })),
  ]
  slimDataCache.set(propName, data)
  return data
}

const slimSettings = { showSearch: false, allowDeselect: true }

function handleSlimUpdate(entryId: string, property: string, value: string | string[]) {
  const strVal = Array.isArray(value) ? value[0] || '' : value
  updateProperty(entryId, property, strVal)
}

function isEnum(propName: string): boolean {
  return getEnumValues(propName).length > 0
}


function isBoolean(propName: string): boolean {
  const def = getPropertyDef(propName)
  return def?.type === 'boolean'
}

// Search for add
let searchTimeout: ReturnType<typeof setTimeout> | null = null

watch(searchQuery, (q) => {
  if (searchTimeout) clearTimeout(searchTimeout)
  if (!q.trim()) {
    searchResults.value = []
    return
  }
  searchTimeout = setTimeout(() => doSearch(q), 200)
})

async function doSearch(q: string) {
  searching.value = true
  try {
    const responses = await Promise.all(targetTypes.value.map((type) => searchEntities(q, type)))
    const allResults = responses.flatMap((r) => r.data)
    // Exclude already-linked entities (including pending adds)
    const linkedIds = new Set(entries.value.map((e) => e.id))
    linkedIds.add(props.entityId)
    searchResults.value = allResults.filter((e) => !linkedIds.has(e.id))
  } catch {
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

function selectTarget(entity: Entity) {
  selectedTarget.value = entity
  searchQuery.value = ''
  searchResults.value = []
  // Initialize meta with empty values
  newMeta.value = {}
  for (const prop of fieldProperties.value) {
    newMeta.value[prop.property] = ''
  }
}

function addRelation() {
  if (!selectedTarget.value || !props.field.relation) return
  error.value = null

  const targetId = selectedTarget.value.id
  const meta = fieldProperties.value.length > 0 ? { ...newMeta.value } : undefined

  // Add to local entries
  const newEntry: RelationEntry = {
    id: targetId,
    type: selectedTarget.value.type,
    direction: isIncoming.value ? 'incoming' : 'outgoing',
    meta,
  }
  entries.value.push(newEntry)
  addedIds.value.add(targetId)

  // Cache the entity
  entityCache.value.set(targetId, selectedTarget.value)

  // Reset add form
  selectedTarget.value = null
  newMeta.value = {}
  showAddSearch.value = false

  emitUpdate()
}

function removeRelation(targetId: string) {
  if (!props.field.relation) return
  error.value = null

  // Remove from entries
  entries.value = entries.value.filter((e) => e.id !== targetId)

  if (addedIds.value.has(targetId)) {
    // Was a pending add - just remove from added set
    addedIds.value.delete(targetId)
  } else {
    // Was an existing relation - track as removed
    removedIds.value.add(targetId)
    updatedIds.value.delete(targetId)
  }

  emitUpdate()
}

function cancelAdd() {
  showAddSearch.value = false
  selectedTarget.value = null
  searchQuery.value = ''
  searchResults.value = []
  newMeta.value = {}
}

function formatLabel(name: string): string {
  return name
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

function entryStatus(id: string): 'added' | 'updated' | null {
  if (addedIds.value.has(id)) return 'added'
  if (updatedIds.value.has(id)) return 'updated'
  return null
}
</script>

<template>
  <div class="relation-cards">
    <label class="section-label">
      {{ field.label || field.relation }}
      <span v-if="hasPendingChanges" class="pending-badge">unsaved</span>
    </label>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="loading" class="loading-indicator">Loading relations...</div>

    <!-- Relation cards -->
    <div v-else-if="entries.length" class="cards-list">
      <div
        v-for="entry in entries"
        :key="entry.id"
        class="relation-card"
        :class="{
          'card-added': entryStatus(entry.id) === 'added',
          'card-updated': entryStatus(entry.id) === 'updated',
        }"
      >
        <div class="card-header">
          <div class="card-identity">
            <span class="entity-id" @click="navigateToEntity(entry.id)">
              {{ entry.id }}
            </span>
            <span class="entity-title" @click="navigateToEntity(entry.id)">
              {{ getEntityTitle(entry.id) }}
            </span>
          </div>
          <button
            v-if="canRemove"
            type="button"
            class="remove-btn"
            title="Remove relation"
            @click="removeRelation(entry.id)"
          >
            &times;
          </button>
        </div>

        <!-- Property fields - always editable, no click-to-edit -->
        <div v-if="fieldProperties.length" class="card-properties">
          <div v-for="prop in fieldProperties" :key="prop.property" class="card-property">
            <span class="prop-label">{{ prop.label || formatLabel(prop.property) }}</span>

            <!-- Enum select (SlimSelect) -->
            <SlimSelect
              v-if="isEnum(prop.property)"
              :key="`${entry.id}-${prop.property}`"
              :model-value="String(entry.meta?.[prop.property] || '')"
              :data="slimSelectData(prop.property)"
              :settings="slimSettings"
              :disabled="isMetaFieldDisabled(prop.property)"
              @update:model-value="handleSlimUpdate(entry.id, prop.property, $event)"
            />

            <!-- Boolean checkbox -->
            <input
              v-else-if="isBoolean(prop.property)"
              :checked="!!entry.meta?.[prop.property]"
              type="checkbox"
              class="inline-edit-checkbox"
              :disabled="isMetaFieldDisabled(prop.property)"
              @change="updateProperty(entry.id, prop.property, ($event.target as HTMLInputElement).checked)"
            />

            <!-- Date / text / number input -->
            <input
              v-else
              :value="entry.meta?.[prop.property] ?? ''"
              :type="getInputType(prop.property)"
              class="inline-edit"
              :disabled="isMetaFieldDisabled(prop.property)"
              @input="updateProperty(entry.id, prop.property, ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="!loading" class="empty-state">
      No {{ field.label || field.relation }} relations yet.
    </div>

    <!-- Add relation -->
    <div v-if="showAddSearch" class="add-section">
      <div v-if="!selectedTarget" class="search-wrapper">
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="`Search ${targetTypes.join(', ')}...`"
          class="search-input"
        />
        <div v-if="searching" class="search-spinner" />

        <div v-if="searchResults.length" class="search-results">
          <div
            v-for="entity in searchResults.slice(0, 10)"
            :key="entity.id"
            class="search-result"
            @click="selectTarget(entity)"
          >
            <span class="result-id">{{ entity.id }}</span>
            <span class="result-title">{{ entity.properties.title || entity.id }}</span>
            <span class="result-type">{{ entity.type }}</span>
          </div>
        </div>

        <div v-else-if="searchQuery && !searching" class="search-empty">
          No matching entities found
        </div>
      </div>

      <!-- Property form for new relation -->
      <div v-if="selectedTarget" class="new-relation-form">
        <div class="selected-target">
          Linking to: <strong>{{ selectedTarget.id }}</strong>
          {{ selectedTarget.properties.title ? `- ${selectedTarget.properties.title}` : '' }}
        </div>

        <div v-if="fieldProperties.length" class="new-meta-fields">
          <div v-for="prop in fieldProperties" :key="prop.property" class="form-field">
            <label>
              {{ prop.label || formatLabel(prop.property) }}
              <span v-if="prop.required" class="required">*</span>
            </label>

            <SlimSelect
              v-if="isEnum(prop.property)"
              :key="`new-${prop.property}`"
              :model-value="String(newMeta[prop.property] || '')"
              :data="slimSelectData(prop.property)"
              :settings="slimSettings"
              @update:model-value="(v: string | string[]) => { newMeta[prop.property] = Array.isArray(v) ? v[0] || '' : v }"
            />

            <input
              v-else-if="isBoolean(prop.property)"
              v-model="newMeta[prop.property]"
              type="checkbox"
            />

            <input
              v-else
              v-model="newMeta[prop.property]"
              :type="getInputType(prop.property)"
              :required="prop.required"
            />
          </div>
        </div>

        <div class="new-relation-actions">
          <button type="button" class="btn btn-secondary" @click="cancelAdd">Cancel</button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!canLink"
            @click="addRelation"
          >
            Link
          </button>
        </div>
      </div>

      <button v-if="!selectedTarget" type="button" class="btn btn-secondary cancel-search" @click="cancelAdd">
        Cancel
      </button>
    </div>

    <button v-if="!showAddSearch && canCreate" type="button" class="add-btn" @click="showAddSearch = true">
      + Add {{ field.label || field.relation }}
    </button>
  </div>
</template>

<style scoped>
.relation-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
  display: flex;
  align-items: center;
  gap: 8px;
}

.pending-badge {
  font-size: 11px;
  font-weight: 500;
  color: var(--warning-color, #f59e0b);
  background: rgba(245, 158, 11, 0.1);
  padding: 1px 6px;
  border-radius: 3px;
}

.cards-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.relation-card {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 12px;
  background: var(--card-bg);
  transition: border-color 0.15s, background 0.15s;
}

.relation-card.card-added {
  border-color: var(--success-color, #22c55e);
  background: rgba(34, 197, 94, 0.03);
}

.relation-card.card-updated {
  border-color: var(--warning-color, #f59e0b);
  background: rgba(245, 158, 11, 0.03);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.card-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  min-width: 0;
}

.entity-id {
  font-family: monospace;
  font-size: 12px;
  color: var(--accent-color, #6366f1);
  white-space: nowrap;
}

.entity-id:hover {
  text-decoration: underline;
}

.entity-title {
  font-size: 14px;
  color: var(--text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.entity-title:hover {
  text-decoration: underline;
}

.remove-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 20px;
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
  flex-shrink: 0;
}

.remove-btn:hover {
  color: var(--error-color, #ef4444);
}

.card-properties {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-color);
  align-items: center;
}

.card-property {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.prop-label {
  color: var(--muted-text);
  white-space: nowrap;
}

.inline-edit,
.inline-select {
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 13px;
  background: var(--input-bg);
  color: var(--text-color);
  min-width: 100px;
  transition: border-color 0.15s;
}

.inline-select {
  cursor: pointer;
  padding-right: 24px;
}

.inline-edit:focus,
.inline-select:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.inline-edit-checkbox {
  appearance: none;
  -webkit-appearance: none;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color, #4a4a5a);
  border-radius: 4px;
  background: var(--input-bg, #1e1e28);
  cursor: pointer;
  position: relative;
  transition: all 0.15s;
  flex-shrink: 0;
}

.inline-edit-checkbox:checked {
  background: var(--accent-color, #6366f1);
  border-color: var(--accent-color, #6366f1);
}

.inline-edit-checkbox:checked::after {
  content: '';
  position: absolute;
  left: 5px;
  top: 1px;
  width: 5px;
  height: 10px;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.inline-edit-checkbox:hover {
  border-color: var(--accent-color, #6366f1);
}

.inline-edit-checkbox:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.3);
}

.empty-state {
  padding: 16px;
  text-align: center;
  color: var(--muted-text);
  font-size: 14px;
  border: 1px dashed var(--border-color);
  border-radius: 6px;
}

/* Add section */
.add-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.search-wrapper {
  position: relative;
}

.search-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
  box-sizing: border-box;
}

.search-input:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.search-spinner {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: translateY(-50%) rotate(360deg); }
}

.search-results {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  margin-top: 4px;
  max-height: 240px;
  overflow-y: auto;
  z-index: 100;
}

.search-result {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.search-result:hover {
  background: var(--hover-bg, rgba(99, 102, 241, 0.05));
}

.result-id {
  font-family: monospace;
  font-size: 12px;
  color: var(--muted-text);
  white-space: nowrap;
}

.result-title {
  flex: 1;
  font-size: 14px;
  color: var(--text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-type {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 6px;
  border-radius: 3px;
  white-space: nowrap;
}

.search-empty {
  padding: 12px;
  text-align: center;
  color: var(--muted-text);
  font-size: 13px;
}

.cancel-search {
  align-self: flex-start;
}

/* New relation form */
.new-relation-form {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 12px;
  background: var(--card-bg);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.selected-target {
  font-size: 14px;
  color: var(--text-color);
}

.selected-target strong {
  font-family: monospace;
  color: var(--accent-color, #6366f1);
}

.new-meta-fields {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-field label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-color);
}

.form-field input[type="text"],
.form-field input[type="number"],
.form-field input[type="date"],
.form-field select {
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.form-field input:focus,
.form-field select:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.required {
  color: var(--error-color, #ef4444);
  margin-left: 2px;
}

.new-relation-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* Buttons */
.add-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px 12px;
  background: var(--hover-bg);
  border: 1px dashed var(--border-color);
  border-radius: 6px;
  color: var(--accent-color, #6366f1);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.add-btn:hover {
  background: var(--accent-color, #6366f1);
  border-color: var(--accent-color, #6366f1);
  color: white;
}

.btn {
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  filter: brightness(1.1);
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover:not(:disabled) {
  filter: brightness(0.95);
}

.loading-indicator {
  padding: 12px;
  text-align: center;
  color: var(--muted-text);
  font-size: 13px;
}

.error-message {
  padding: 10px 12px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--error-color, #ef4444);
  border-radius: 6px;
  color: var(--error-color, #ef4444);
  font-size: 14px;
}

@media (max-width: 768px) {
  .card-properties {
    flex-direction: column;
    align-items: stretch;
  }

  .card-property {
    flex-direction: column;
    align-items: stretch;
    gap: 4px;
  }

  .prop-label {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.3px;
  }

  .inline-edit,
  .inline-select {
    width: 100%;
    min-width: 0;
  }
}
</style>

<style>
/* SlimSelect dark mode overrides (unscoped — .ss-content is portaled to body) */
.relation-cards .ss-main {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 4px;
  min-height: 30px;
  font-size: 13px;
  background: var(--input-bg);
  color: var(--text-color);
}

.relation-cards .ss-main:focus-within {
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

/* .ss-content is portaled to <body>, so we need !important */
.ss-content {
  border: 1px solid var(--border-color, #e2e8f0) !important;
  border-radius: 6px !important;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1) !important;
  background: var(--card-bg) !important;
}

.ss-content .ss-option {
  color: var(--text-color);
}

.ss-content .ss-option.ss-highlighted {
  background: var(--hover-bg);
}

.ss-content .ss-option.ss-selected {
  background: color-mix(in srgb, var(--accent-color) 20%, transparent);
  color: var(--accent-color);
}

.ss-content .ss-search input {
  background: var(--input-bg);
  color: var(--text-color);
}
</style>
