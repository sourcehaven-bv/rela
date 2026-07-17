<script setup lang="ts">
/**
 * Single-value entity-target selector, shared across the SPA (TKT-DL16XM).
 *
 * Given a pre-resolved candidate list, the user picks exactly one target. The
 * COMMITTED VALUE is the candidate's bare display title (`entityDisplayTitle`),
 * never `Title (ID)` — the data-entry relation filter matches on the backend's
 * `DisplayTitle` (api_v1.go matchRelationFilter), so a `Title (ID)` value would
 * match nothing (RR-X4QWBF). The dropdown MAY show `Title (ID)` for
 * disambiguation, but that is display-only.
 *
 * Two modes:
 * - `select`   → native `<select>` of titles. Right for a small closed set.
 * - `typeahead`→ text search box + filtered dropdown. Right for larger sets.
 *
 * The typeahead has TWO distinct states that must NOT be conflated (RR-NH8B6D):
 * the in-progress `searchQuery` (local, throwaway) and the committed value (the
 * `modelValue` prop, owned by the parent). Typing only filters candidates;
 * the value changes ONLY on a deliberate option click. Clicking away discards
 * the search string without committing it. This mirrors how a native `<select>`
 * behaves and keeps an external `modelValue` change (back/forward nav, SSE)
 * from clobbering mid-type input.
 *
 * This component deliberately carries NONE of RelationPicker's write-path
 * machinery (incoming-changed, verdicts, InlineCreateModal, update:types,
 * multi-select). It is presentational: candidates in, one title out.
 */
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { entityDisplayTitle, entityDisplayTitleWithId } from '@/utils/entityDisplay'
import type { TargetCandidate } from '@/types'

const props = withDefaults(
  defineProps<{
    /** Resolved candidate entities to choose among. */
    candidates: TargetCandidate[]
    /** Committed value: the bare display title of the chosen candidate (or ''). */
    modelValue: string
    /** Widget shape. */
    mode: 'select' | 'typeahead'
    /** Accessible id for the control (label `for=`). */
    controlId?: string
    /** Placeholder for the typeahead search box / empty select option. */
    placeholder?: string
    /** Label for the empty (no-filter) choice in select mode. */
    allLabel?: string
  }>(),
  {
    controlId: undefined,
    placeholder: 'Search…',
    allLabel: 'All',
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

// Candidates sorted by display title, case-insensitive. De-duplicated by the
// bare title so two entities sharing a DisplayTitle collapse to one option
// (the backend matches both — documented title-collision behavior, RR-A51QQ2).
//
// The option VALUE is always the bare title. The LABEL shows the id in
// parentheses to disambiguate — but ONLY when the title is unique. When several
// candidates share a title, the collapsed option matches all of them, so
// pinning one candidate's id in the label would be misleading (it's whichever
// happened to be fetched first). For collided titles we show the bare title
// instead (RR-0TY8MA).
const sortedOptions = computed(() => {
  // First pass: count how many candidates resolve to each title.
  const titleCounts = new Map<string, number>()
  for (const c of props.candidates) {
    const value = entityDisplayTitle(c)
    titleCounts.set(value, (titleCounts.get(value) ?? 0) + 1)
  }
  const seen = new Set<string>()
  const opts: { value: string; label: string }[] = []
  for (const c of props.candidates) {
    const value = entityDisplayTitle(c)
    if (seen.has(value)) continue
    seen.add(value)
    // Unique title → show "Title (ID)"; collided title → bare title.
    const label = titleCounts.get(value) === 1 ? entityDisplayTitleWithId(c) : value
    opts.push({ value, label })
  }
  return opts.sort((a, b) => a.value.localeCompare(b.value, undefined, { sensitivity: 'base' }))
})

// A committed value the current candidate set doesn't cover — a deep-linked or
// externally-set title that isn't among the loaded options (beyond the fetch
// cap, or during the pre-load window before candidates arrive). Without this,
// a native <select> bound to such a value would render blank/"All" even though
// the filter IS applied, so the control would silently misrepresent the active
// filter (RR-5GU270). We surface it as an explicit option so the control always
// reflects the committed value.
const missingSelectedValue = computed(() => {
  if (!props.modelValue) return ''
  return sortedOptions.value.some((o) => o.value === props.modelValue) ? '' : props.modelValue
})

// --- select mode -----------------------------------------------------------

// A local mirror so v-model on the native <select> stays a one-liner while the
// canonical value is still the parent's modelValue.
const selectValue = computed({
  get: () => props.modelValue,
  set: (v: string) => emit('update:modelValue', v),
})

// --- typeahead mode --------------------------------------------------------

// In-progress search string. Component-local; NEVER driven by modelValue, so an
// external modelValue change can't clobber what the user is typing (RR-NH8B6D).
const searchQuery = ref('')
const showDropdown = ref(false)

// The label to show in the closed typeahead: the committed selection, or empty.
const committedLabel = computed(() => props.modelValue)

const filteredOptions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return sortedOptions.value
  return sortedOptions.value.filter((o) => o.label.toLowerCase().includes(q))
})

function openTypeahead() {
  // Seed the search box empty (not with the committed value) so the full list
  // shows on open and typing filters from scratch.
  searchQuery.value = ''
  showDropdown.value = true
}

function commit(value: string) {
  emit('update:modelValue', value)
  searchQuery.value = ''
  showDropdown.value = false
}

function clearSelection() {
  commit('')
}

// Close the dropdown when focus/click leaves the widget. Discards the search
// string WITHOUT committing (a partial search is not a selection).
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.entity-target-select')) {
    showDropdown.value = false
    searchQuery.value = ''
  }
}

watch(showDropdown, (open) => {
  if (open) {
    document.addEventListener('click', handleClickOutside)
  } else {
    document.removeEventListener('click', handleClickOutside)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div class="entity-target-select">
    <!-- select mode: native <select> of titles -->
    <select v-if="mode === 'select'" :id="controlId" v-model="selectValue">
      <option value="">{{ allLabel }}</option>
      <!-- Active committed value not covered by the loaded candidates (deep
           link beyond the fetch cap, or pre-load). Keeps the control honest
           about what's actually filtered (RR-5GU270). -->
      <option v-if="missingSelectedValue" :value="missingSelectedValue">
        {{ missingSelectedValue }}
      </option>
      <option v-for="opt in sortedOptions" :key="opt.value" :value="opt.value">
        {{ opt.label }}
      </option>
    </select>

    <!-- typeahead mode: search box + filtered dropdown -->
    <div v-else class="typeahead">
      <div class="typeahead-input">
        <input
          :id="controlId"
          v-model="searchQuery"
          type="text"
          role="combobox"
          :aria-expanded="showDropdown"
          aria-haspopup="listbox"
          aria-autocomplete="list"
          :placeholder="committedLabel || placeholder"
          :class="{ 'has-selection': !!committedLabel }"
          @focus="openTypeahead"
          @input="showDropdown = true"
        />
        <button
          v-if="committedLabel"
          type="button"
          class="clear-selection"
          :aria-label="`Clear ${committedLabel}`"
          @click="clearSelection"
        >
          &times;
        </button>
      </div>

      <div v-if="showDropdown" class="dropdown" role="listbox">
        <div
          class="dropdown-item all-option"
          role="option"
          :aria-selected="!committedLabel"
          @click="commit('')"
        >
          {{ allLabel }}
        </div>
        <div
          v-for="opt in filteredOptions"
          :key="opt.value"
          class="dropdown-item"
          role="option"
          :aria-selected="opt.value === committedLabel"
          @click="commit(opt.value)"
        >
          {{ opt.label }}
        </div>
        <div v-if="filteredOptions.length === 0" class="dropdown-empty">No matching entities</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.entity-target-select {
  position: relative;
}

.entity-target-select select {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 14px;
  min-width: 150px;
  background: var(--input-bg);
  color: var(--text-color);
}

.typeahead-input {
  position: relative;
  display: flex;
  align-items: center;
}

.typeahead-input input {
  padding: 6px 26px 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 14px;
  min-width: 150px;
  width: 100%;
  background: var(--input-bg);
  color: var(--text-color);
}

.typeahead-input input.has-selection::placeholder {
  color: var(--text-color);
  opacity: 1;
}

.typeahead-input input:focus,
.entity-target-select select:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.clear-selection {
  position: absolute;
  right: 6px;
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 0 2px;
}

.clear-selection:hover {
  color: var(--error-color, #ef4444);
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
  padding: 8px 12px;
  cursor: pointer;
  font-size: 14px;
  color: var(--text-color);
  transition: background 0.15s;
}

.dropdown-item:hover,
.dropdown-item[aria-selected='true'] {
  background: var(--hover-bg);
}

.dropdown-item.all-option {
  color: var(--muted-text);
  border-bottom: 1px solid var(--border-color);
}

.dropdown-empty {
  padding: 12px;
  text-align: center;
  color: var(--muted-text);
  font-size: 13px;
}
</style>
