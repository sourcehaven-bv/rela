<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import type { FormFieldOrRelation, Entity } from '@/types'

const props = defineProps<{
  field: FormFieldOrRelation
  entityType: string
  value: string[]
}>()

const emit = defineEmits<{
  update: [value: string[]]
}>()

const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()

// State
const loading = ref(false)
const candidates = ref<Entity[]>([])
const searchQuery = ref('')
const showDropdown = ref(false)

// Computed
const relationType = computed(() => {
  if (!props.field.relation) return undefined
  return schemaStore.getRelationType(props.field.relation)
})

const targetTypes = computed(() => {
  if (!relationType.value) return []
  return relationType.value.to
})

const label = computed(() => props.field.label || props.field.relation || '')
const help = computed(() => props.field.help || relationType.value?.description || '')

const isMulti = computed(() => {
  // Check cardinality constraints
  if (!relationType.value) return true
  return relationType.value.max_outgoing !== 1
})

const selectedEntities = computed(() => {
  return candidates.value.filter((c) => props.value.includes(c.id))
})

const filteredCandidates = computed(() => {
  if (!searchQuery.value) {
    return candidates.value.filter((c) => !props.value.includes(c.id))
  }
  const query = searchQuery.value.toLowerCase()
  return candidates.value.filter(
    (c) =>
      !props.value.includes(c.id) &&
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
    console.error('Failed to load relation candidates:', err)
  } finally {
    loading.value = false
  }
}

function selectEntity(entity: Entity) {
  if (isMulti.value) {
    emit('update', [...props.value, entity.id])
  } else {
    emit('update', [entity.id])
  }
  searchQuery.value = ''
  showDropdown.value = false
}

function removeEntity(entityId: string) {
  emit('update', props.value.filter((id) => id !== entityId))
}

function getEntityLabel(entity: Entity): string {
  return String(entity.properties.title || entity.id)
}

// Lifecycle
onMounted(() => {
  loadCandidates()
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
      </div>

      <div v-if="loading" class="loading-indicator">
        Loading...
      </div>
    </div>

    <p v-if="help" class="field-help">{{ help }}</p>
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
