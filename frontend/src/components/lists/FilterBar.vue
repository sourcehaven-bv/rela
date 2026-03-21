<script setup lang="ts">
import { ref, watch } from 'vue'
import type { ListConfig, EntityType } from '@/types'

const props = defineProps<{
  config: ListConfig
  entityType?: EntityType
  filters: Record<string, string>
}>()

const emit = defineEmits<{
  filter: [filters: Record<string, string>]
}>()

// Initialize local filters with empty strings for each filter control
// This ensures selects show "All" by default instead of blank
function initializeFilters(existingFilters: Record<string, string>): Record<string, string> {
  const result: Record<string, string> = {}
  for (const control of props.config.filter_controls || []) {
    const key = control.property || control.relation
    if (key) {
      result[key] = existingFilters[key] ?? ''
    }
  }
  return result
}

const localFilters = ref<Record<string, string>>(initializeFilters(props.filters))

watch(
  () => props.filters,
  (newFilters) => {
    localFilters.value = initializeFilters(newFilters)
  }
)

function handleFilterChange() {
  emit('filter', { ...localFilters.value })
}

function clearFilters() {
  localFilters.value = {}
  emit('filter', {})
}

function getFilterOptions(property: string): string[] {
  if (!props.entityType) return []
  const propDef = props.entityType.properties[property]
  return propDef?.values || []
}

function hasActiveFilters(): boolean {
  return Object.values(localFilters.value).some((v) => v)
}
</script>

<template>
  <div class="filter-bar">
    <div class="filters">
      <div
        v-for="filter in config.filter_controls"
        :key="filter.property ?? filter.relation ?? 'unknown'"
        class="filter-item"
      >
        <template v-if="filter.property">
          <label :for="`filter-${filter.property}`">
            {{ filter.label || filter.property }}
          </label>
          <select
            v-if="getFilterOptions(filter.property).length"
            :id="`filter-${filter.property}`"
            v-model="localFilters[filter.property]"
            @change="handleFilterChange"
          >
            <option value="">All</option>
            <option
              v-for="option in getFilterOptions(filter.property)"
              :key="option"
              :value="option"
            >
              {{ option }}
            </option>
          </select>
          <input
            v-else
            :id="`filter-${filter.property}`"
            v-model="localFilters[filter.property]"
            type="text"
            :placeholder="`Filter by ${filter.label || filter.property}`"
            @input="handleFilterChange"
          />
        </template>
      </div>
    </div>
    <button
      v-if="hasActiveFilters()"
      class="clear-filters"
      @click="clearFilters"
    >
      Clear filters
    </button>
  </div>
</template>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.filters {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.filter-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-item label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.filter-item select,
.filter-item input {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 14px;
  min-width: 150px;
  background: var(--input-bg);
  color: var(--text-color);
}

.filter-item select:focus,
.filter-item input:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.clear-filters {
  padding: 6px 12px;
  background: none;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 13px;
  color: var(--muted-text);
  cursor: pointer;
  transition: all 0.15s;
}

.clear-filters:hover {
  background: var(--hover-bg);
  color: var(--text-color);
}
</style>
