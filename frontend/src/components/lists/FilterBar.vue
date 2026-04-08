<script setup lang="ts">
import { ref, watch, computed, onBeforeUnmount } from 'vue'
import type {
  ListConfig,
  EntityType,
  FilterControl,
  PropertyDef,
  FilterState,
} from '@/types'

const props = defineProps<{
  config: ListConfig
  entityType?: EntityType
  filters: FilterState
}>()

const emit = defineEmits<{
  filter: [filters: FilterState]
}>()

// Debounce window for text-input filters. Select/multi-select fire immediately
// because they only change on a deliberate click.
const TEXT_DEBOUNCE_MS = 250

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

// Which control keys are text widgets (vs select / multi-select). Text
// widgets debounce and may have in-progress unsent input; the props.filters
// watcher must NOT clobber them. Select widgets fire immediately on change
// so there's no in-progress state to preserve.
const textWidgetKeys = computed(() => {
  const set = new Set<string>()
  for (const filter of resolvedFilters.value) {
    if (filter.widget === 'text') set.add(filter.key)
  }
  return set
})

// Local widget state is just a string per control, but we hold onto each
// property's incoming operator separately so non-default ops (e.g. `<=` from a
// deep-linked URL) survive a user edit. Widgets don't yet expose operator
// selection — that's a future enhancement.
function initializeFilters(existingFilters: FilterState): Record<string, string> {
  const result: Record<string, string> = {}
  for (const control of props.config.filter_controls || []) {
    const key = control.property || control.relation
    if (key) {
      result[key] = existingFilters[key]?.value ?? ''
    }
  }
  return result
}

function captureOperators(existingFilters: FilterState): Record<string, string | undefined> {
  const ops: Record<string, string | undefined> = {}
  for (const control of props.config.filter_controls || []) {
    const key = control.property || control.relation
    if (key) ops[key] = existingFilters[key]?.op
  }
  return ops
}

const localFilters = ref<Record<string, string>>(initializeFilters(props.filters))
const preservedOps = ref<Record<string, string | undefined>>(captureOperators(props.filters))

// Debounce timer for text-input filters. Hoisted above the props.filters
// watcher because that watcher needs to check whether an edit is mid-flight.
let textDebounceTimer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.filters,
  (newFilters) => {
    // When an external change arrives (back/forward nav, programmatic URL
    // update, another tab) while the user is mid-type in a text widget,
    // naively reassigning localFilters would drop their keystrokes. Preserve
    // the text-widget values when a debounce is pending; the pending timer
    // will then emit what the user *actually* typed, not the externally
    // supplied value.
    const rebuilt = initializeFilters(newFilters)
    if (textDebounceTimer !== null) {
      for (const key of textWidgetKeys.value) {
        rebuilt[key] = localFilters.value[key] ?? ''
      }
    }
    localFilters.value = rebuilt
    preservedOps.value = captureOperators(newFilters)
  },
)

function buildState(): FilterState {
  const state: FilterState = {}
  for (const [key, value] of Object.entries(localFilters.value)) {
    if (!value) continue
    const fv: FilterState[string] = { value }
    const op = preservedOps.value[key]
    // Omit op when it's absent or the default '=' form — same convention
    // as buildQueryWithFilters, so the state shape is canonical throughout.
    if (op && op !== '=') fv.op = op
    state[key] = fv
  }
  return state
}

function emitFilters() {
  emit('filter', buildState())
}

function handleTextInput() {
  if (textDebounceTimer) clearTimeout(textDebounceTimer)
  textDebounceTimer = setTimeout(() => {
    textDebounceTimer = null
    emitFilters()
  }, TEXT_DEBOUNCE_MS)
}

function handleFilterChange() {
  // Select widgets fire here — flush any pending text debounce so a select
  // change doesn't get clobbered by a stale text emit.
  if (textDebounceTimer) {
    clearTimeout(textDebounceTimer)
    textDebounceTimer = null
  }
  emitFilters()
}

function handleMultiSelectChange(key: string, event: Event) {
  const select = event.target as HTMLSelectElement
  const selected = Array.from(select.selectedOptions).map((opt) => opt.value)
  localFilters.value[key] = selected.join(',')
  handleFilterChange()
}

function getMultiSelectValues(key: string): string[] {
  const val = localFilters.value[key]
  if (!val) return []
  return val.split(',').filter(Boolean)
}

function clearFilters() {
  if (textDebounceTimer) {
    clearTimeout(textDebounceTimer)
    textDebounceTimer = null
  }
  localFilters.value = {}
  preservedOps.value = {}
  emit('filter', {})
}

function hasActiveFilters(): boolean {
  return Object.values(localFilters.value).some((v) => v)
}

onBeforeUnmount(() => {
  if (textDebounceTimer !== null) {
    clearTimeout(textDebounceTimer)
    textDebounceTimer = null
  }
})
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

        <!-- Text widget (default) — debounced to avoid a fetch per keystroke -->
        <input
          v-else
          :id="`filter-${filter.key}`"
          v-model="localFilters[filter.key]"
          type="text"
          :placeholder="`Filter by ${filter.label}`"
          @input="handleTextInput"
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
