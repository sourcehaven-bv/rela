<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { searchEntities } from '@/api'
import { useSchemaStore } from '@/stores'
import type { Entity, PropertyDef } from '@/types'

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()

// Types
interface ActiveFilter {
  id: string
  type: 'type' | 'property'
  property: string
  value: string
  label: string
}

interface FilterOption {
  category: 'type' | 'property'
  property: string
  label: string
  propertyDef?: PropertyDef
  entityType?: string
}

// Refs
const searchInputRef = ref<HTMLInputElement | null>(null)
const filterMenuRef = ref<HTMLDivElement | null>(null)
const filterSearchRef = ref<HTMLInputElement | null>(null)

// State
const query = ref('')
const results = ref<Entity[]>([])
const loading = ref(false)
const searched = ref(false)
const selectedIndex = ref(-1)
const inResults = ref(false)
const showHelp = ref(false)

// Filter state
const activeFilters = ref<ActiveFilter[]>([])
const showFilterMenu = ref(false)
const filterSearch = ref('')
const selectedFilterOption = ref<FilterOption | null>(null)
const filterValueInput = ref('')
const filterMenuIndex = ref(0)

// Computed
const entityTypes = computed(() => {
  const types: Array<{ value: string; label: string }> = []
  for (const [name, def] of schemaStore.entityTypes) {
    types.push({ value: name, label: def.label || name })
  }
  return types
})

// Get all filterable properties across entity types
const filterOptions = computed((): FilterOption[] => {
  const options: FilterOption[] = []
  const seenProperties = new Set<string>()

  // Add type filter option
  options.push({
    category: 'type',
    property: 'type',
    label: 'Entity Type',
  })

  // Add property filters from all entity types
  for (const [typeName, typeDef] of schemaStore.entityTypes) {
    for (const [propName, propDef] of Object.entries(typeDef.properties)) {
      // Skip if already seen (properties with same name across types)
      const key = propName
      if (seenProperties.has(key)) continue
      seenProperties.add(key)

      options.push({
        category: 'property',
        property: propName,
        label: propName.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase()),
        propertyDef: propDef,
        entityType: typeName,
      })
    }
  }

  return options
})

// Filter the options based on search
const filteredOptions = computed(() => {
  if (!filterSearch.value) return filterOptions.value

  const search = filterSearch.value.toLowerCase()
  return filterOptions.value.filter(opt =>
    opt.label.toLowerCase().includes(search) ||
    opt.property.toLowerCase().includes(search)
  )
})

// Get values for a property (for enum dropdowns)
const selectedPropertyValues = computed((): string[] => {
  if (!selectedFilterOption.value) return []

  if (selectedFilterOption.value.category === 'type') {
    return entityTypes.value.map(t => t.value)
  }

  const propDef = selectedFilterOption.value.propertyDef
  if (propDef?.values?.length) {
    return propDef.values
  }

  // For non-enum properties, check all entity types for possible values
  const allValues = new Set<string>()
  for (const [, typeDef] of schemaStore.entityTypes) {
    const prop = typeDef.properties[selectedFilterOption.value.property]
    if (prop?.values) {
      prop.values.forEach(v => allValues.add(v))
    }
  }
  return Array.from(allValues)
})

// Build full search query including filters
const fullSearchQuery = computed(() => {
  const parts: string[] = []

  // Add text query
  if (query.value.trim()) {
    parts.push(query.value.trim())
  }

  // Add active filters
  for (const filter of activeFilters.value) {
    if (filter.type === 'type') {
      parts.push(`type:${filter.value}`)
    } else {
      parts.push(`prop:${filter.property}=${filter.value}`)
    }
  }

  return parts.join(' ')
})

// Methods
async function search() {
  const searchQuery = fullSearchQuery.value
  if (!searchQuery) {
    results.value = []
    searched.value = false
    return
  }

  loading.value = true
  searched.value = true

  try {
    const response = await searchEntities(searchQuery)
    results.value = response.data
  } catch (err) {
    console.error('Search error:', err)
    results.value = []
  } finally {
    loading.value = false
  }

  // Update URL with query params
  const urlParams: Record<string, string> = {}
  if (query.value.trim()) {
    urlParams.q = query.value
  }
  for (const filter of activeFilters.value) {
    if (filter.type === 'type') {
      urlParams.type = filter.value
    } else {
      urlParams[`filter_${filter.property}`] = filter.value
    }
  }
  router.replace({ query: urlParams })
}

// Filter menu methods
function toggleFilterMenu() {
  showFilterMenu.value = !showFilterMenu.value
  if (showFilterMenu.value) {
    filterSearch.value = ''
    filterMenuIndex.value = 0
    selectedFilterOption.value = null
    nextTick(() => filterSearchRef.value?.focus())
  }
}

function closeFilterMenu() {
  showFilterMenu.value = false
  selectedFilterOption.value = null
  filterValueInput.value = ''
  filterSearch.value = ''
}

function selectFilterOption(option: FilterOption) {
  selectedFilterOption.value = option
  filterValueInput.value = ''
  filterMenuIndex.value = 0
}

function applyFilter(value: string) {
  if (!selectedFilterOption.value || !value) return

  const filterId = `${selectedFilterOption.value.property}-${Date.now()}`
  const label = selectedFilterOption.value.category === 'type'
    ? entityTypes.value.find(t => t.value === value)?.label || value
    : value

  activeFilters.value.push({
    id: filterId,
    type: selectedFilterOption.value.category,
    property: selectedFilterOption.value.property,
    value,
    label: `${selectedFilterOption.value.label}: ${label}`,
  })

  closeFilterMenu()
  search()
}

function removeFilter(filterId: string) {
  activeFilters.value = activeFilters.value.filter(f => f.id !== filterId)
  search()
}

function clearAllFilters() {
  activeFilters.value = []
  search()
}

function handleFilterKeydown(e: KeyboardEvent) {
  if (!showFilterMenu.value) return

  const options = selectedFilterOption.value ? selectedPropertyValues.value : filteredOptions.value

  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      filterMenuIndex.value = Math.min(options.length - 1, filterMenuIndex.value + 1)
      break
    case 'ArrowUp':
      e.preventDefault()
      filterMenuIndex.value = Math.max(0, filterMenuIndex.value - 1)
      break
    case 'Enter':
      e.preventDefault()
      if (selectedFilterOption.value) {
        const value = selectedPropertyValues.value[filterMenuIndex.value]
        if (value) applyFilter(value)
      } else {
        const option = filteredOptions.value[filterMenuIndex.value]
        if (option) selectFilterOption(option)
      }
      break
    case 'Escape':
      e.preventDefault()
      if (selectedFilterOption.value) {
        selectedFilterOption.value = null
      } else {
        closeFilterMenu()
      }
      break
    case 'Backspace':
      if (!filterSearch.value && selectedFilterOption.value) {
        selectedFilterOption.value = null
      }
      break
  }
}

// Click outside to close menu
function handleClickOutside(e: MouseEvent) {
  if (filterMenuRef.value && !filterMenuRef.value.contains(e.target as Node)) {
    closeFilterMenu()
  }
}

function getEntityLabel(entity: Entity): string {
  return String(entity.properties.title || entity.id)
}

function getEntityTypeLabel(type: string): string {
  const def = schemaStore.entityTypes.get(type)
  return def?.label || type
}

// Keyboard navigation
function handleKeydown(e: KeyboardEvent) {
  // Don't intercept if in filter menu
  if (showFilterMenu.value) return

  const target = e.target as HTMLElement
  const isInInput = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA'

  // F key to open filter menu (when not in input)
  if (e.key === 'f' && !isInInput && !e.metaKey && !e.ctrlKey) {
    e.preventDefault()
    toggleFilterMenu()
    return
  }

  // If we're in the input and user presses Tab or ArrowDown, enter results mode
  if (document.activeElement === searchInputRef.value) {
    if ((e.key === 'Tab' || e.key === 'ArrowDown') && results.value.length > 0) {
      e.preventDefault()
      inResults.value = true
      selectedIndex.value = 0
      searchInputRef.value?.blur()
      return
    }
  }

  // If not in input (in results mode)
  if (inResults.value && results.value.length > 0) {
    switch (e.key) {
      case 'j':
      case 'ArrowDown':
        e.preventDefault()
        selectedIndex.value = Math.min(results.value.length - 1, selectedIndex.value + 1)
        scrollSelectedIntoView()
        break

      case 'k':
      case 'ArrowUp':
        e.preventDefault()
        selectedIndex.value = Math.max(0, selectedIndex.value - 1)
        scrollSelectedIntoView()
        break

      case 'Enter':
      case 'o':
        if (selectedIndex.value >= 0) {
          e.preventDefault()
          navigateToResult(selectedIndex.value)
        }
        break

      case 'Escape':
      case '/':
        e.preventDefault()
        focusInput()
        break
    }
  }
}

function scrollSelectedIntoView() {
  nextTick(() => {
    const items = document.querySelectorAll('.result-item')
    const selected = items[selectedIndex.value]
    if (selected) {
      selected.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  })
}

function focusInput() {
  inResults.value = false
  selectedIndex.value = -1
  nextTick(() => {
    searchInputRef.value?.focus()
    searchInputRef.value?.select()
  })
}

function navigateToResult(index: number) {
  const entity = results.value[index]
  if (entity) {
    router.push(`/entity/${entity.type}/${entity.id}`)
  }
}

// Clear selection when results change
watch(results, () => {
  selectedIndex.value = -1
  inResults.value = false
})

// Auto-focus on mount
onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  document.addEventListener('click', handleClickOutside)
  nextTick(() => {
    searchInputRef.value?.focus()
  })
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('click', handleClickOutside)
})

// Initialize from URL params
watch(
  () => route.query,
  (newQuery) => {
    // Restore text query
    if (newQuery.q && typeof newQuery.q === 'string') {
      query.value = newQuery.q
    }

    // Restore filters from URL
    const restoredFilters: ActiveFilter[] = []

    // Type filter
    if (newQuery.type && typeof newQuery.type === 'string') {
      const typeLabel = entityTypes.value.find(t => t.value === newQuery.type)?.label || newQuery.type
      restoredFilters.push({
        id: `type-${Date.now()}`,
        type: 'type',
        property: 'type',
        value: newQuery.type,
        label: `Entity Type: ${typeLabel}`,
      })
    }

    // Property filters (filter_<prop>=value)
    for (const [key, value] of Object.entries(newQuery)) {
      if (key.startsWith('filter_') && typeof value === 'string') {
        const propName = key.replace('filter_', '')
        const propLabel = propName.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())
        restoredFilters.push({
          id: `${propName}-${Date.now()}`,
          type: 'property',
          property: propName,
          value,
          label: `${propLabel}: ${value}`,
        })
      }
    }

    if (restoredFilters.length > 0) {
      activeFilters.value = restoredFilters
    }

    // Trigger search if we have query or filters
    if (query.value || activeFilters.value.length > 0) {
      search()
    }
  },
  { immediate: true }
)
</script>

<template>
  <div class="search-view">
    <header class="search-header">
      <h1>Search</h1>
      <button
        type="button"
        class="help-btn"
        :class="{ active: showHelp }"
        title="Show search syntax help"
        @click="showHelp = !showHelp"
      >
        ?
      </button>
    </header>

    <!-- Search syntax help panel -->
    <div v-if="showHelp" class="help-panel">
      <h3>Search Syntax</h3>
      <div class="help-content">
        <div class="help-section">
          <h4>Text Search</h4>
          <p>Simply type keywords to search across all entity titles and content.</p>
          <code class="example">bug login</code>
        </div>

        <div class="help-section">
          <h4>Filter by Type</h4>
          <p>Use <code>type:</code> to filter by entity type.</p>
          <code class="example">type:ticket bug</code>
        </div>

        <div class="help-section">
          <h4>Filter by Property</h4>
          <p>Use <code>prop:property=value</code> to filter by property values.</p>
          <code class="example">prop:status=open</code>
          <code class="example">prop:priority=high bug</code>
        </div>

        <div class="help-section">
          <h4>Combine Filters</h4>
          <p>Combine multiple filters and text search together.</p>
          <code class="example">type:ticket prop:status=open prop:priority=high</code>
        </div>

        <div class="help-section">
          <h4>Keyboard Shortcuts</h4>
          <ul class="shortcut-list">
            <li><kbd>F</kbd> Open filter menu</li>
            <li><kbd>Tab</kbd> / <kbd>&darr;</kbd> Enter results navigation</li>
            <li><kbd>j</kbd> / <kbd>k</kbd> Navigate results</li>
            <li><kbd>Enter</kbd> / <kbd>o</kbd> Open selected result</li>
            <li><kbd>/</kbd> Focus search input</li>
          </ul>
        </div>
      </div>
    </div>

    <div class="search-form">
      <div class="search-input-row">
        <input
          ref="searchInputRef"
          v-model="query"
          type="text"
          placeholder="Search entities..."
          class="search-input"
          @keyup.enter="search"
          @focus="inResults = false"
        />

        <!-- Filter button with dropdown -->
        <div ref="filterMenuRef" class="filter-dropdown">
          <button
            class="btn btn-secondary filter-btn"
            type="button"
            @click.stop="toggleFilterMenu"
          >
            + Filter <kbd>F</kbd>
          </button>

          <div v-if="showFilterMenu" class="filter-menu" @click.stop @keydown="handleFilterKeydown">
            <!-- Property/Type selection -->
            <template v-if="!selectedFilterOption">
              <input
                ref="filterSearchRef"
                v-model="filterSearch"
                type="text"
                placeholder="Search properties..."
                class="filter-search"
                @keydown="handleFilterKeydown"
              />
              <div class="filter-options">
                <div
                  v-for="(option, index) in filteredOptions"
                  :key="option.property"
                  class="filter-option"
                  :class="{ highlighted: index === filterMenuIndex }"
                  @click="selectFilterOption(option)"
                  @mouseenter="filterMenuIndex = index"
                >
                  <span class="option-category">{{ option.category }}</span>
                  <span class="option-label">{{ option.label }}</span>
                </div>
                <div v-if="filteredOptions.length === 0" class="filter-empty">
                  No matching properties
                </div>
              </div>
            </template>

            <!-- Value selection -->
            <template v-else>
              <div class="filter-header">
                <button class="back-btn" type="button" @click="selectedFilterOption = null">
                  &larr;
                </button>
                <span>{{ selectedFilterOption.label }}</span>
              </div>

              <!-- Enum/predefined values -->
              <div v-if="selectedPropertyValues.length > 0" class="filter-options">
                <div
                  v-for="(value, index) in selectedPropertyValues"
                  :key="value"
                  class="filter-option"
                  :class="{ highlighted: index === filterMenuIndex }"
                  @click="applyFilter(value)"
                  @mouseenter="filterMenuIndex = index"
                >
                  {{ value }}
                </div>
              </div>

              <!-- Free text input for non-enum -->
              <div v-else class="filter-text-input">
                <input
                  v-model="filterValueInput"
                  type="text"
                  placeholder="Enter value..."
                  class="filter-search"
                  @keydown.enter="applyFilter(filterValueInput)"
                />
                <button
                  class="btn btn-primary btn-sm"
                  :disabled="!filterValueInput"
                  type="button"
                  @click="applyFilter(filterValueInput)"
                >
                  Apply
                </button>
              </div>
            </template>
          </div>
        </div>

        <button class="btn btn-primary" :disabled="loading" @click="search">
          {{ loading ? 'Searching...' : 'Search' }}
        </button>
      </div>

      <!-- Active filters chips -->
      <div v-if="activeFilters.length > 0" class="active-filters">
        <span class="filters-label">Filters:</span>
        <div
          v-for="filter in activeFilters"
          :key="filter.id"
          class="filter-chip"
        >
          <span>{{ filter.label }}</span>
          <button class="chip-remove" type="button" @click="removeFilter(filter.id)">&times;</button>
        </div>
        <button class="clear-filters" type="button" @click="clearAllFilters">
          Clear all
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner"/>
      <span>Searching...</span>
    </div>

    <div v-else-if="searched && results.length === 0" class="empty-state">
      <p>No results found for "{{ query }}"</p>
    </div>

    <div v-else-if="results.length > 0" class="search-results">
      <p class="results-count">{{ results.length }} result{{ results.length !== 1 ? 's' : '' }} found</p>

      <div class="results-list">
        <div
          v-for="(entity, index) in results"
          :key="entity.id"
          class="result-item"
          :class="{ selected: index === selectedIndex }"
          @click="navigateToResult(index)"
        >
          <span class="result-type">{{ getEntityTypeLabel(entity.type) }}</span>
          <span class="result-id">{{ entity.id }}</span>
          <span class="result-title">{{ getEntityLabel(entity) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-view {
  max-width: 800px;
}

.search-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.search-header h1 {
  margin: 0;
}

.help-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: var(--bg-color, #f8fafc);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 50%;
  font-size: 14px;
  font-weight: 600;
  color: var(--muted-text);
  cursor: pointer;
  transition: all 0.15s;
}

.help-btn:hover,
.help-btn.active {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: white;
}

.help-panel {
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 24px;
}

.help-panel h3 {
  margin: 0 0 16px;
  font-size: 16px;
  color: var(--text-color);
}

.help-content {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
}

.help-section {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 14px;
}

.help-section h4 {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.help-section p {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--muted-text);
  line-height: 1.4;
}

.help-section code {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
  background: var(--hover-bg);
  padding: 2px 6px;
  border-radius: 4px;
}

.help-section code.example {
  display: block;
  padding: 8px 10px;
  margin-top: 6px;
  background: var(--sidebar-bg);
  color: var(--sidebar-text);
  border-radius: 4px;
}

.help-section code.example + code.example {
  margin-top: 4px;
}

.shortcut-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.shortcut-list li {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--muted-text);
  padding: 4px 0;
}

.shortcut-list kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 22px;
  padding: 0 6px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-family: inherit;
  font-size: 11px;
  font-weight: 500;
  color: var(--text-color);
  box-shadow: 0 1px 0 var(--border-color);
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
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 15px;
  background: var(--input-bg);
  color: var(--text-color);
}

.search-input:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

/* Filter dropdown */
.filter-dropdown {
  position: relative;
}

.filter-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.filter-menu {
  position: absolute;
  top: 100%;
  left: 0;
  margin-top: 4px;
  min-width: 280px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  z-index: 100;
  overflow: hidden;
}

.filter-search {
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
  outline: none;
  background: var(--input-bg);
  color: var(--text-color);
}

.filter-search:focus {
  background: var(--hover-bg);
}

.filter-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color);
  background: var(--hover-bg);
  font-weight: 500;
  font-size: 14px;
}

.back-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 16px;
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-color);
}

.back-btn:hover {
  background: var(--border-color);
}

.filter-options {
  max-height: 240px;
  overflow-y: auto;
}

.filter-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.1s;
}

.filter-option:hover,
.filter-option.highlighted {
  background: var(--hover-bg);
}

.option-category {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 6px;
  border-radius: 3px;
  font-weight: 500;
}

.option-label {
  flex: 1;
}

.filter-empty {
  padding: 16px;
  text-align: center;
  color: var(--muted-text);
  font-size: 14px;
}

.filter-text-input {
  display: flex;
  gap: 8px;
  padding: 12px;
}

.filter-text-input .filter-search {
  flex: 1;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

/* Active filters chips */
.active-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.filters-label {
  font-size: 13px;
  color: var(--muted-text);
  font-weight: 500;
}

.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px 4px 10px;
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent-color) 30%, transparent);
  border-radius: 16px;
  font-size: 13px;
  color: var(--accent-color);
}

.chip-remove {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0 2px;
  color: var(--accent-color);
  opacity: 0.7;
}

.chip-remove:hover {
  opacity: 1;
}

.clear-filters {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 13px;
  color: var(--muted-text);
  padding: 4px 8px;
}

.clear-filters:hover {
  color: var(--error-color);
  text-decoration: underline;
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover {
  background: var(--hover-bg);
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
  filter: brightness(0.9);
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px;
  color: var(--muted-text);
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
  color: var(--muted-text);
  background: var(--hover-bg);
  border-radius: 8px;
}

.results-count {
  margin-bottom: 16px;
  color: var(--muted-text);
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
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  text-decoration: none;
  color: inherit;
  transition: all 0.15s;
  cursor: pointer;
}

.result-item:hover {
  border-color: var(--accent-color);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.result-item.selected {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  border-color: var(--accent-color);
  outline: 2px solid var(--accent-color);
  outline-offset: -2px;
}

.result-item.selected:hover {
  background: color-mix(in srgb, var(--accent-color) 25%, transparent);
}

.result-type {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--hover-bg);
  padding: 4px 8px;
  border-radius: 4px;
  font-weight: 500;
}

.result-id {
  font-family: monospace;
  font-size: 13px;
  color: var(--muted-text);
}

.result-title {
  flex: 1;
  font-size: 15px;
  color: var(--text-color);
}
</style>
