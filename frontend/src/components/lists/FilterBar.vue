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

const localFilters = ref<Record<string, string>>({ ...props.filters })

watch(
  () => props.filters,
  (newFilters) => {
    localFilters.value = { ...newFilters }
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
        :key="filter.property"
        class="filter-item"
      >
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
  background: #f8fafc;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
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
  color: #64748b;
}

.filter-item select,
.filter-item input {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 14px;
  min-width: 150px;
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
  color: #64748b;
  cursor: pointer;
  transition: all 0.15s;
}

.clear-filters:hover {
  background: white;
  color: var(--text-color);
}
</style>
