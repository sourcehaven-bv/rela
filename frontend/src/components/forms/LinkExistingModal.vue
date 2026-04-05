<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { searchEntities, createRelation } from '@/api'
import { useSchemaStore } from '@/stores'
import type { Entity } from '@/types'

const props = defineProps<{
  show: boolean
  relation: string
  linkAs: 'from' | 'to'
  peerId: string
  entityTypes: string[]
  excludeIds?: string[]
}>()

const emit = defineEmits<{
  close: []
  linked: [entity: Entity]
}>()

const schemaStore = useSchemaStore()

// State
const query = ref('')
const results = ref<Entity[]>([])
const searching = ref(false)
const linking = ref(false)
const error = ref<string | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)

// Compute peer entity type for createRelation
const peerType = computed(() => {
  for (const [type, def] of schemaStore.entityTypes) {
    // Check if peerId matches this type's prefix pattern
    if (def.id_prefix && props.peerId.startsWith(def.id_prefix)) {
      return type
    }
  }
  return ''
})

// Filter out already-linked entities
const filteredResults = computed(() => {
  const exclude = new Set(props.excludeIds || [])
  exclude.add(props.peerId) // Can't link to self
  return results.value.filter((e) => !exclude.has(e.id))
})

// Debounced search
let searchTimeout: ReturnType<typeof setTimeout> | null = null

watch(query, (q) => {
  if (searchTimeout) clearTimeout(searchTimeout)
  if (!q.trim()) {
    results.value = []
    return
  }
  searchTimeout = setTimeout(() => doSearch(q), 200)
})

// Reset when modal opens
watch(() => props.show, (show) => {
  if (show) {
    query.value = ''
    results.value = []
    error.value = null
    // Focus search input
    setTimeout(() => searchInput.value?.focus(), 100)
  }
})

async function doSearch(q: string) {
  searching.value = true
  error.value = null
  try {
    // Search for each entity type and merge results
    const allResults: Entity[] = []
    for (const type of props.entityTypes) {
      const response = await searchEntities(q, type)
      allResults.push(...response.data)
    }
    results.value = allResults
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Search failed'
  } finally {
    searching.value = false
  }
}

async function linkEntity(target: Entity) {
  linking.value = true
  error.value = null
  try {
    if (props.linkAs === 'to') {
      // peer --relation--> target
      await createRelation(peerType.value, props.peerId, props.relation, target.id)
    } else {
      // target --relation--> peer
      await createRelation(target.type, target.id, props.relation, props.peerId)
    }
    emit('linked', target)
    emit('close')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to link entity'
  } finally {
    linking.value = false
  }
}

function handleClose() {
  if (!linking.value) {
    emit('close')
  }
}

function entityTitle(entity: Entity): string {
  return (entity.properties.title as string) || (entity.properties.name as string) || entity.id
}

function entityTypeLabel(type: string): string {
  return schemaStore.getEntityType(type)?.label || type
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="modal-overlay" @click.self="handleClose">
      <div class="modal-content">
        <header class="modal-header">
          <h3>Link Existing Entity</h3>
          <button type="button" class="close-btn" @click="handleClose">&times;</button>
        </header>

        <div class="modal-body">
          <div v-if="error" class="error-message">{{ error }}</div>

          <div class="search-field">
            <input
              ref="searchInput"
              v-model="query"
              type="text"
              placeholder="Search entities..."
              :disabled="linking"
            />
            <span v-if="searching" class="search-spinner"/>
          </div>

          <div class="results-list">
            <div v-if="query && !searching && filteredResults.length === 0" class="empty-state">
              No matching entities found
            </div>

            <button
              v-for="entity in filteredResults"
              :key="entity.id"
              class="result-item"
              :disabled="linking"
              @click="linkEntity(entity)"
            >
              <span class="result-id">{{ entity.id }}</span>
              <span class="result-title">{{ entityTitle(entity) }}</span>
              <span class="result-type">{{ entityTypeLabel(entity.type) }}</span>
            </button>
          </div>
        </div>

        <footer class="modal-footer">
          <button type="button" class="btn btn-secondary" :disabled="linking" @click="handleClose">
            Cancel
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: var(--card-bg);
  border-radius: 8px;
  width: 100%;
  max-width: 520px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.2);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  color: var(--text-color);
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--muted-text);
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color);
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid var(--border-color);
}

.search-field {
  position: relative;
}

.search-field input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
  box-sizing: border-box;
}

.search-field input:focus {
  outline: none;
  border-color: var(--accent-color);
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

.results-list {
  max-height: 300px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--input-bg);
  cursor: pointer;
  text-align: left;
  font-size: 14px;
  color: var(--text-color);
  transition: background 0.1s;
}

.result-item:hover:not(:disabled) {
  background: var(--hover-bg, rgba(99, 102, 241, 0.05));
  border-color: var(--accent-color);
}

.result-item:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.result-id {
  font-family: monospace;
  font-size: 12px;
  color: var(--muted-text);
  white-space: nowrap;
}

.result-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-type {
  font-size: 12px;
  color: var(--muted-text);
  white-space: nowrap;
}

.empty-state {
  padding: 20px;
  text-align: center;
  color: var(--muted-text);
  font-size: 14px;
}

.error-message {
  padding: 10px 12px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--error-color, #ef4444);
  border-radius: 6px;
  color: var(--error-color, #ef4444);
  font-size: 14px;
}

.btn {
  padding: 10px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover:not(:disabled) {
  filter: brightness(0.95);
}
</style>
