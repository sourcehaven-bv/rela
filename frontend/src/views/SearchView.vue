<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { searchEntities } from '@/api'
import { useSchemaStore } from '@/stores'
import type { Entity } from '@/types'

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()

// State
const query = ref('')
const typeFilter = ref('')
const results = ref<Entity[]>([])
const loading = ref(false)
const searched = ref(false)

// Computed
const entityTypes = computed(() => {
  const types: Array<{ value: string; label: string }> = []
  for (const [name, def] of schemaStore.entityTypes) {
    types.push({ value: name, label: def.label || name })
  }
  return types
})

// Methods
async function search() {
  if (!query.value.trim()) {
    results.value = []
    searched.value = false
    return
  }

  loading.value = true
  searched.value = true

  try {
    const response = await searchEntities(query.value, typeFilter.value || undefined)
    results.value = response.data
  } catch (err) {
    console.error('Search error:', err)
    results.value = []
  } finally {
    loading.value = false
  }

  // Update URL with query params
  router.replace({
    query: {
      q: query.value,
      ...(typeFilter.value ? { type: typeFilter.value } : {}),
    },
  })
}

function getEntityLabel(entity: Entity): string {
  return String(entity.properties.title || entity.id)
}

function getEntityTypeLabel(type: string): string {
  const def = schemaStore.entityTypes.get(type)
  return def?.label || type
}

// Initialize from URL params
watch(
  () => route.query,
  (newQuery) => {
    if (newQuery.q && typeof newQuery.q === 'string') {
      query.value = newQuery.q
      typeFilter.value = (newQuery.type as string) || ''
      search()
    }
  },
  { immediate: true }
)
</script>

<template>
  <div class="search-view">
    <h1>Search</h1>

    <div class="search-form">
      <div class="search-input-row">
        <input
          v-model="query"
          type="text"
          placeholder="Search entities..."
          class="search-input"
          @keyup.enter="search"
        />
        <select v-model="typeFilter" class="type-filter">
          <option value="">All types</option>
          <option v-for="type in entityTypes" :key="type.value" :value="type.value">
            {{ type.label }}
          </option>
        </select>
        <button class="btn btn-primary" @click="search" :disabled="loading">
          {{ loading ? 'Searching...' : 'Search' }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>Searching...</span>
    </div>

    <div v-else-if="searched && results.length === 0" class="empty-state">
      <p>No results found for "{{ query }}"</p>
    </div>

    <div v-else-if="results.length > 0" class="search-results">
      <p class="results-count">{{ results.length }} result{{ results.length !== 1 ? 's' : '' }} found</p>

      <div class="results-list">
        <router-link
          v-for="entity in results"
          :key="entity.id"
          :to="`/entity/${entity.type}/${entity.id}`"
          class="result-item"
        >
          <span class="result-type">{{ getEntityTypeLabel(entity.type) }}</span>
          <span class="result-id">{{ entity.id }}</span>
          <span class="result-title">{{ getEntityLabel(entity) }}</span>
        </router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-view {
  max-width: 800px;
}

h1 {
  margin-bottom: 24px;
}

.search-form {
  margin-bottom: 24px;
}

.search-input-row {
  display: flex;
  gap: 12px;
}

.search-input {
  flex: 1;
  padding: 10px 14px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 15px;
}

.search-input:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.type-filter {
  padding: 10px 14px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 14px;
  min-width: 140px;
}

.btn {
  padding: 10px 20px;
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

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #4f46e5;
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px;
  color: #64748b;
}

.spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.empty-state {
  padding: 48px 24px;
  text-align: center;
  color: #64748b;
  background: #f8fafc;
  border-radius: 8px;
}

.results-count {
  margin-bottom: 16px;
  color: #64748b;
  font-size: 14px;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  text-decoration: none;
  color: inherit;
  transition: all 0.15s;
}

.result-item:hover {
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.result-type {
  font-size: 11px;
  text-transform: uppercase;
  color: #64748b;
  background: #f1f5f9;
  padding: 4px 8px;
  border-radius: 4px;
  font-weight: 500;
}

.result-id {
  font-family: monospace;
  font-size: 13px;
  color: #64748b;
}

.result-title {
  flex: 1;
  font-size: 15px;
  color: #1e293b;
}
</style>
