<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { ListConfig, EntityType, FilterControl, PropertyDef } from '@/types'

const props = defineProps<{
  config: ListConfig
  entityType?: EntityType
  filters: Record<string, string>
}>()

const emit = defineEmits<{
  filter: [filters: Record<string, string>]
}>()

// Resolved filter control with computed widget type and options
interface ResolvedFilter {
  key: string
  label: string
  widget: 'select' | 'multi-select' | 'text'
  options: string[]
  isRelation: boolean
}

const resolvedFilters = computed((): ResolvedFilter[] => {
  if (!props.config.filter_controls) return []
  return props.config.filter_controls.map((fc) => resolveFilter(fc))
})

function resolveFilter(fc: FilterControl): ResolvedFilter {
  const key = fc.relation || fc.property || ''
  const label = fc.label || titleCase(key)

  if (fc.relation) {
    // Relation filters: use text input for now (could be enhanced to select with targets)
    return { key, label, widget: 'text', options: [], isRelation: true }
  }

  // Property filter
  const propDef = props.entityType?.properties[fc.property || '']
  if (!propDef) {
    return { key, label, widget: 'text', options: [], isRelation: false }
  }

  const options = propDef.values || []
  const widget = resolveWidgetType(propDef, options)

  return { key, label, widget, options, isRelation: false }
}

function resolveWidgetType(propDef: PropertyDef, options: string[]): 'select' | 'multi-select' | 'text' {
  // Multi-select for list properties with enum values
  if (propDef.list && options.length > 0) {
    return 'multi-select'
  }
  // Select for properties with defined values (enums)
  if (options.length > 0) {
    return 'select'
  }
  // Text for everything else
  return 'text'
}

function titleCase(str: string): string {
  return str
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

// Initialize local filters with empty strings for each filter control
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

function handleMultiSelectChange(key: string, event: Event) {
  const select = event.target as HTMLSelectElement
  const selected = Array.from(select.selectedOptions).map((opt) => opt.value)
  localFilters.value[key] = selected.join(',')
  emit('filter', { ...localFilters.value })
}

function getMultiSelectValues(key: string): string[] {
  const val = localFilters.value[key]
  if (!val) return []
  return val.split(',').filter(Boolean)
}

function clearFilters() {
  localFilters.value = {}
  emit('filter', {})
}

function hasActiveFilters(): boolean {
  return Object.values(localFilters.value).some((v) => v)
}
</script>

<template>
  <div class="filter-bar">
    <div class="filters">
      <div
        v-for="filter in resolvedFilters"
        :key="filter.key"
        class="filter-item"
      >
        <label :for="`filter-${filter.key}`">
          {{ filter.label }}
        </label>

        <!-- Select widget -->
        <select
          v-if="filter.widget === 'select'"
          :id="`filter-${filter.key}`"
          v-model="localFilters[filter.key]"
          @change="handleFilterChange"
        >
          <option value="">All</option>
          <option
            v-for="option in filter.options"
            :key="option"
            :value="option"
          >
            {{ option }}
          </option>
        </select>

        <!-- Multi-select widget -->
        <select
          v-else-if="filter.widget === 'multi-select'"
          :id="`filter-${filter.key}`"
          multiple
          :class="{ 'has-selection': getMultiSelectValues(filter.key).length > 0 }"
          @change="(e) => handleMultiSelectChange(filter.key, e)"
        >
          <option
            v-for="option in filter.options"
            :key="option"
            :value="option"
            :selected="getMultiSelectValues(filter.key).includes(option)"
          >
            {{ option }}
          </option>
        </select>

        <!-- Text widget (default) -->
        <input
          v-else
          :id="`filter-${filter.key}`"
          v-model="localFilters[filter.key]"
          type="text"
          :placeholder="`Filter by ${filter.label}`"
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

/* Multi-select specific styles */
.filter-item select[multiple] {
  min-height: 80px;
  max-height: 120px;
}

.filter-item select[multiple].has-selection {
  border-color: var(--accent-color);
}
</style>
