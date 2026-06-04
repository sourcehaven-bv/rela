<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { searchEntities } from '@/api'
import { useSchemaStore } from '@/stores'
import { parseFilterQueryParams } from '@/utils/filters'
import { useBackTarget } from '@/composables/useBackTarget'
import BackButton from '@/components/common/BackButton.vue'
import AdHocFilterMenu from '@/components/lists/AdHocFilterMenu.vue'
import type { Entity } from '@/types'

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()
const backTarget = useBackTarget()

interface ActiveFilter {
  id: string
  type: 'type' | 'property'
  property: string
  value: string
  label: string
}

const searchInputRef = ref<HTMLInputElement | null>(null)
const filterMenuRef = ref<InstanceType<typeof AdHocFilterMenu> | null>(null)

const query = ref('')
const results = ref<Entity[]>([])
const loading = ref(false)
const searched = ref(false)
const selectedIndex = ref(-1)
const inResults = ref(false)
const showHelp = ref(false)

const activeFilters = ref<ActiveFilter[]>([])

// Lock only the `type` chip (single-valued by design — only one Entity Type
// makes sense). Properties stay unlocked so the user can OR-combine multiple
// values on the same property the way the pre-extraction code allowed —
// e.g. `prop:status=open prop:status=in_progress`. Each chip then becomes
// an additional `prop:` clause in `fullSearchQuery`.
const lockedFilterProperties = computed(() => {
  const set = new Set<string>()
  if (activeFilters.value.some((f) => f.property === 'type')) set.add('type')
  return set
})

const entityTypes = computed(() => {
  const types: Array<{ value: string; label: string }> = []
  for (const [name, def] of schemaStore.entityTypes) {
    types.push({ value: name, label: def.label || name })
  }
  return types
})

function titleCase(str: string): string {
  return str.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
}

function buildFilterLabel(property: string, value: string): string {
  if (property === 'type') {
    const t = entityTypes.value.find((t) => t.value === value)
    return `Entity Type: ${t?.label || value}`
  }
  return `${titleCase(property)}: ${value}`
}

function handleAdHocApply(property: string, value: string) {
  activeFilters.value.push({
    id: `${property}-${Date.now()}`,
    type: property === 'type' ? 'type' : 'property',
    property,
    value,
    label: buildFilterLabel(property, value),
  })
  search()
}

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
  // Sync the URL first so removing the last filter chip (or clearing the
  // text query) clears stale params, even when the early-return below skips
  // the API call.
  syncUrlFromState()

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
}

function syncUrlFromState() {
  const urlParams: Record<string, string> = {}
  if (query.value.trim()) {
    urlParams.q = query.value
  }
  for (const filter of activeFilters.value) {
    if (filter.type === 'type') {
      urlParams.type = filter.value
    } else {
      urlParams[`filter[${filter.property}]`] = filter.value
    }
  }
  router.replace({ query: urlParams })
}

function removeFilter(filterId: string) {
  activeFilters.value = activeFilters.value.filter(f => f.id !== filterId)
  search()
}

function clearAllFilters() {
  activeFilters.value = []
  search()
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
  const target = e.target as HTMLElement
  const isInInput = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA'

  // F key to open filter menu (when not in an input). The menu owns its own
  // keydown handling once open, so we don't need to early-return here.
  if (e.key === 'f' && !isInInput && !e.metaKey && !e.ctrlKey) {
    e.preventDefault()
    filterMenuRef.value?.open()
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
  if (!entity) return
  // Pass the search scope so the detail page can show prev/next across the
  // exact result set the user saw (#844 / scope.go). `from=search` selects the
  // search origin in useScopeNavigation; `q` is the *full* query string
  // (including any type:/prop: chips), so the backend's executeQuery
  // reproduces the identical ordered, possibly-mixed-type result.
  const scopeQuery: Record<string, string> = { from: 'search' }
  const full = fullSearchQuery.value
  if (full) scopeQuery.q = full
  router.push({ path: `/entity/${entity.type}/${entity.id}`, query: scopeQuery })
}

// Clear selection when results change
watch(results, () => {
  selectedIndex.value = -1
  inResults.value = false
})

// Auto-focus on mount
onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  nextTick(() => {
    searchInputRef.value?.focus()
  })
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
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
      restoredFilters.push({
        id: `type-${Date.now()}`,
        type: 'type',
        property: 'type',
        value: newQuery.type,
        label: buildFilterLabel('type', newQuery.type),
      })
    }

    // Property filters: parse bracket-format `filter[prop]=value`. SearchView
    // only emits the equality form (no operator suffix), so we restore those
    // and ignore any operator-suffixed entries deep-linked from elsewhere.
    const restoredFromBrackets = parseFilterQueryParams(newQuery)
    for (const [propName, fv] of Object.entries(restoredFromBrackets)) {
      if (fv.op && fv.op !== '=') continue
      restoredFilters.push({
        id: `${propName}-${Date.now()}`,
        type: 'property',
        property: propName,
        value: fv.value,
        label: buildFilterLabel(propName, fv.value),
      })
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
      <BackButton v-if="backTarget" :target="backTarget" />
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
      <h3>How to Search</h3>
      <div class="help-content">
        <div class="help-section">
          <h4>Text Search</h4>
          <p>Type keywords in the search box to find matching entities by title or content.</p>
        </div>

        <div class="help-section">
          <h4>Add Filters</h4>
          <p>Click <strong>+ Filter</strong> or press <kbd>F</kbd> to filter by entity type or property values.</p>
        </div>

        <div class="help-section">
          <h4>Combine Search &amp; Filters</h4>
          <p>Use text search together with multiple filters for precise results.</p>
        </div>

        <div class="help-section shortcuts-section">
          <h4>Keyboard Shortcuts</h4>
          <ul class="shortcut-list">
            <li><kbd>F</kbd> Open filter menu</li>
            <li><kbd>Tab</kbd> / <kbd>&darr;</kbd> Enter results</li>
            <li><kbd>j</kbd> / <kbd>k</kbd> Navigate results</li>
            <li><kbd>Enter</kbd> / <kbd>o</kbd> Open selected</li>
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

        <AdHocFilterMenu
          ref="filterMenuRef"
          mode="search"
          :locked-properties="lockedFilterProperties"
          @apply="handleAdHocApply"
        />

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

.help-section kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 5px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-family: inherit;
  font-size: 11px;
  font-weight: 500;
  color: var(--text-color);
  box-shadow: 0 1px 0 var(--border-color);
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
  align-items: stretch;
}

.search-input-row .btn {
  height: 42px;
  padding-top: 0;
  padding-bottom: 0;
}

.search-input {
  flex: 1;
  height: 42px;
  padding: 0 14px;
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

/* Filter dropdown styles live on AdHocFilterMenu (scoped). */

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

/* Uses global .btn, .btn-secondary, .btn-primary, .loading-state, .spinner from App.vue */

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

@media (max-width: 768px) {
  .search-input-row {
    flex-wrap: wrap;
  }
}
</style>
