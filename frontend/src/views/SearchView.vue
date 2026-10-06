<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { RouterLink, useRoute, useRouter, type RouteLocationRaw } from 'vue-router'
import { searchEntities } from '@/api'
import { useSchemaStore } from '@/stores'
import { parseFilterQueryParams } from '@/utils/filters'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { isInputFocused } from '@/utils/dom'
import { ownedEntityHref } from '@/utils/entityRoute'
import { useBackTarget } from '@/composables/useBackTarget'
import { useWorld } from '@/composables/useWorld'
import { useDetailPanel } from '@/composables/useDetailPanel'
import EntityDetailPanel from '@/components/entity/EntityDetailPanel.vue'
import BackButton from '@/components/common/BackButton.vue'
import AdHocFilterMenu from '@/components/lists/AdHocFilterMenu.vue'
import type { Entity } from '@/types'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import RlKbd from 'rela-components/components/data/RlKbd.vue'

const route = useRoute()
const router = useRouter()
const { worldParam } = useWorld()
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
const loadError = ref(false)

// pageState mirrors DynamicForm's `form-state-*` contract so a screenshot can
// wait for this screen. A search is "loaded" once a query has actually run:
// an idle search page and a page mid-query look the same otherwise, and a
// capture taken between them would show an empty result list that means
// "nothing typed yet" rather than "nothing found".
// A failed search is `error`, never `loaded`: an empty result list after a
// failed fetch reads exactly like "nothing found", and a screenshot hook that
// waits for `loaded` would photograph the failure as a good page.
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (loadError.value) return 'error'
  return loading.value || !searched.value ? 'pending' : 'loaded'
})
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

function buildFilterLabel(property: string, value: string): string {
  if (property === 'type') {
    const t = entityTypes.value.find((t) => t.value === value)
    return `Entity Type: ${t?.label || value}`
  }
  // DEC-6C1NAA: the property name is shown raw, not title-cased.
  return `${property}: ${value}`
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
  loadError.value = false

  try {
    const response = await searchEntities(searchQuery, undefined, undefined, worldParam.value)
    results.value = response.data
  } catch (err) {
    console.error('Search error:', err)
    results.value = []
    loadError.value = true
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
  return entityDisplayTitle(entity)
}

function getEntityTypeLabel(type: string): string {
  const def = schemaStore.entityTypes.get(type)
  return def?.label || type
}

// Keyboard navigation
function handleKeydown(e: KeyboardEvent) {
  const isInInput = isInputFocused()

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

// resultTarget is the SINGLE source of truth for where a result goes — bound to
// the RouterLink's `to` AND used by the keyboard handler's push, so a
// cmd-clicked tab and an Enter press land on the identical URL. Building the
// href separately from the push is how the scope below silently goes missing.
//
// Pass the search scope so the detail page can show prev/next across the exact
// result set the user saw (#844 / scope.go). `from=search` selects the search
// origin in useScopeNavigation; `q` is the *full* query string (including any
// type:/prop: chips), so the backend's executeQuery reproduces the identical
// ordered, possibly-mixed-type result.
//
// An owned hit (TKT-QO14GB) goes to its owner's page, anchored at the hit. It
// carries no search scope: the owner is not one of the results, so prev/next
// across them would start from a page outside the set.
function resultTarget(entity: Entity): RouteLocationRaw {
  if (entity._owner) return ownedEntityHref(entity)
  const scopeQuery: Record<string, string> = { from: 'search' }
  const full = fullSearchQuery.value
  if (full) scopeQuery.q = full
  return { path: `/entity/${entity.type}/${entity.id}`, query: scopeQuery }
}

function navigateToResult(index: number) {
  const entity = results.value[index]
  if (!entity) return
  router.push(resultTarget(entity))
}

/*
 * A plain click opens the result in the detail panel beside the results, the
 * way a list row does, so the reader can scan several hits without losing the
 * result set. Modified clicks stay the link's own (new tab, new window), and
 * Enter still opens the entity's own page.
 *
 * The open result lives in `?selected=`, so a back step and a shared link
 * restore it. A `replace`: browsing hits is not a place change.
 */
const panel = useDetailPanel()

const selectedResultId = computed(() => {
  const raw = route.query.selected
  const value = Array.isArray(raw) ? raw[raw.length - 1] : raw
  return typeof value === 'string' && value !== '' ? value : null
})

function openResult(entity: Entity, event: MouseEvent) {
  if (event.defaultPrevented) return
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  if (event.button !== 0) return
  // Capture phase, so RouterLink's own handler does not navigate.
  event.preventDefault()
  event.stopPropagation()
  if (entity.id === selectedResultId.value) return
  void router.replace({ query: { ...route.query, selected: entity.id } })
}

function closeResult() {
  const next = { ...route.query }
  delete next.selected
  void router.replace({ query: next })
}

// Waits for the hit to be in the results, like EntityList: a `?selected=` no
// result matches is a stale link, not a request to show that entity.
const selectedResult = computed(() => {
  const id = selectedResultId.value
  if (!id) return null
  return results.value.find((e) => e.id === id) ?? null
})

watch(
  selectedResult,
  (entity) => {
    if (!entity) {
      panel.clear()
      return
    }
    panel.show({
      component: EntityDetailPanel,
      props: {
        entityType: entity.type,
        entityId: entity.id,
        onClose: closeResult,
        onExpand: () => router.push(resultTarget(entity)),
      },
      mode: 'inline',
    })
  },
  { immediate: true },
)

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

// Initialize from URL params. `selected` is left out of the key: opening a
// result in the panel changes the URL, and must not re-run the search.
const searchParamsKey = computed(() => {
  const rest = { ...route.query }
  delete rest.selected
  return JSON.stringify(rest)
})

watch(
  searchParamsKey,
  () => {
    const newQuery = route.query
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
  <div class="search-view" :data-testid="`page-state-${pageState}`">
    <header class="search-header mobile-topbar mobile-topbar--with-menu">
      <div class="header-left">
        <BackButton v-if="backTarget" :target="backTarget" />
        <h1>Search</h1>
      </div>
      <RlIconButton
        icon="help"
        label="Search syntax help"
        :pressed="showHelp"
        @click="showHelp = !showHelp"
      />
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
          <p>Click <strong>+ Filter</strong> or press <RlKbd keys="F" /> to filter by entity type or property values.</p>
        </div>

        <div class="help-section">
          <h4>Combine Search &amp; Filters</h4>
          <p>Use text search together with multiple filters for precise results.</p>
        </div>

        <div class="help-section shortcuts-section">
          <h4>Keyboard Shortcuts</h4>
          <ul class="shortcut-list">
            <li><RlKbd keys="F" /> Open filter menu</li>
            <li><RlKbd keys="Tab or &darr;" separator="or" /> Enter results</li>
            <li><RlKbd keys="j or k" separator="or" /> Navigate results</li>
            <li><RlKbd keys="Enter or o" separator="or" /> Open selected</li>
            <li><RlKbd keys="/" /> Focus search input</li>
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

        <RlButton
          variant="primary"
          :loading="loading"
          pending-label="Searching…"
          @click="search"
        >
          Search
        </RlButton>
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

    <!-- No block spinner here on purpose. The Search BUTTON already owns
         this operation's pending state (one indicator per user act), and
         previous results stay on screen while the next query runs rather
         than being replaced by a spinner — the keep-previous-content rule.
         A re-search therefore never blanks the list. -->
    <RlEmptyState
      v-if="searched && !loading && results.length === 0"
      class="empty-state"
      icon="search"
      :title="`No results found for \u201c${query}\u201d`"
    />

    <!-- Before the first query: say what search covers, so the page is not blank. -->
    <RlEmptyState
      v-else-if="!searched && !loading"
      class="search-intro"
      icon="search"
      title="Search entities"
      description="Find entities by title or content, and narrow the results with filters."
    />

    <section v-else-if="results.length > 0" class="search-results" aria-labelledby="search-results-heading">
      <p id="search-results-heading" class="results-count">{{ results.length }} result{{ results.length !== 1 ? 's' : '' }} found</p>

      <ul class="results-list">
        <li v-for="(entity, index) in results" :key="entity.id" class="result-row">
          <RouterLink
            class="result-item"
            :class="{ selected: index === selectedIndex, open: entity.id === selectedResultId }"
            :to="resultTarget(entity)"
            :aria-current="entity.id === selectedResultId ? 'true' : undefined"
            @click.capture="openResult(entity, $event)"
          >
            <span class="result-type">{{ getEntityTypeLabel(entity.type) }}</span>
            <span class="result-id">{{ entity.id }}</span>
            <span class="result-title"
              >{{ getEntityLabel(entity)
              }}<span v-if="entity._owner" class="result-owner">in {{ entity._owner.title }}</span></span
            >
          </RouterLink>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.search-view {
  max-width: 800px;
}

.search-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 24px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.search-header h1 {
  margin: 0;
}


.help-panel {
  background: var(--rl-color-bg-hover);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 24px;
}

.help-panel h3 {
  margin: 0 0 16px;
  font-size: 16px;
  color: var(--rl-color-text);
}

.help-content {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
}

.help-section {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 6px;
  padding: 14px;
}

.help-section h4 {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--rl-color-text);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.help-section p {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--rl-color-text-muted);
  line-height: 1.4;
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
  color: var(--rl-color-text-muted);
  padding: 4px 0;
}

.search-form {
  margin-bottom: 24px;
}

.search-input-row {
  display: flex;
  gap: 12px;
  align-items: stretch;
}

/* Match the input beside it. `:deep` because the library's button styles are
 * scoped, so a bare `.rl-button` here would match nothing. */
.search-input-row :deep(.rl-button) {
  height: 42px;
  padding-top: 0;
  padding-bottom: 0;
}

.search-input {
  flex: 1;
  height: 42px;
  padding: 0 14px;
  border: 1px solid var(--rl-color-border);
  border-radius: 6px;
  font-size: 15px;
  background: var(--rl-color-bg-raised);
  color: var(--rl-color-text);
}

.search-input:focus {
  outline: none;
  border-color: var(--rl-color-accent, #6366f1);
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
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
  color: var(--rl-color-text-muted);
  font-weight: 500;
}

.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px 4px 10px;
  background: color-mix(in srgb, var(--rl-color-accent) 15%, transparent);
  border: 1px solid color-mix(in srgb, var(--rl-color-accent) 30%, transparent);
  border-radius: 16px;
  font-size: 13px;
  color: var(--rl-color-accent);
}

.chip-remove {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0 2px;
  color: var(--rl-color-accent);
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
  color: var(--rl-color-text-muted);
  padding: 4px 8px;
}

.clear-filters:hover {
  color: var(--rl-color-danger);
  text-decoration: underline;
}

/* RlEmptyState centres its own icon, heading and text; the panel it sits in
   is this view's. */
.empty-state,
.search-intro {
  padding: 48px 24px;
  background: var(--rl-color-bg-hover);
  border-radius: 8px;
}

.results-count {
  margin-bottom: 16px;
  color: var(--rl-color-text-muted);
  font-size: 14px;
}

/* Now a <ul> of <li> results. Reset UA list styling so the flex column
   renders exactly as it did as a div. */
.results-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

/* The <li> carries only the list semantics; the link inside it is the card. The
   row is a plain block so the <ul>'s flex column still measures it (display:
   contents would drop the list role in several screen readers), and the link is
   made to fill it. */
.result-row {
  display: block;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  text-decoration: none;
  color: inherit;
  transition: all 0.15s;
  cursor: pointer;
}

.result-item:hover {
  border-color: var(--rl-color-accent);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.result-item.selected {
  background: color-mix(in srgb, var(--rl-color-accent) 15%, transparent);
  border-color: var(--rl-color-accent);
  outline: 2px solid var(--rl-color-accent);
  outline-offset: -2px;
}

/* The result showing in the detail panel. */
.result-item.open {
  background: color-mix(in srgb, var(--rl-color-accent) 10%, transparent);
  border-color: var(--rl-color-accent);
}

.result-item.selected:hover {
  background: color-mix(in srgb, var(--rl-color-accent) 25%, transparent);
}

.result-type {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--rl-color-text-muted);
  background: var(--rl-color-bg-hover);
  padding: 4px 8px;
  border-radius: 4px;
  font-weight: 500;
}

.result-id {
  font-family: monospace;
  font-size: 13px;
  color: var(--rl-color-text-muted);
}

.result-title {
  flex: 1;
  font-size: 15px;
  color: var(--rl-color-text);
}

.result-owner {
  margin-left: 6px;
  font-size: 13px;
  color: var(--rl-color-text-muted);
}

@media (max-width: 768px) {
  .search-input-row {
    flex-wrap: wrap;
  }

  /* .search-header uses .mobile-topbar.mobile-topbar--with-menu from
     mobile-bars.css. */
  .search-header h1 {
    font-size: 18px;
  }
}

@media (max-width: 480px) {
  /* The shell's main pane drops to 12px horizontal padding at this breakpoint;
     the sticky header's full-bleed negative margin must match or it
     pokes past the screen edge and triggers horizontal scroll. */
  .search-header {
    margin-left: -12px;
    margin-right: -12px;
  }
}
</style>
