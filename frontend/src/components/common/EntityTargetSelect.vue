<script setup lang="ts">
/**
 * Single-value entity-target selector, shared across the SPA (TKT-DL16XM).
 *
 * Given a pre-resolved candidate list, the user picks exactly one target. The
 * COMMITTED VALUE is the candidate's id: the data-entry relation filter matches
 * on the target id (TKT-ZKPA1E), so two targets that share a title stay apart
 * and a rename does not change the filter. The control shows titles. A legacy
 * title value that exactly one candidate has is replaced by that candidate's
 * id. Any other value that is no candidate's id (an ambiguous title, or a
 * target beyond the fetch cap) is shown as it is.
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
 * machinery (incoming-changed, verdicts, inline create, update:types,
 * multi-select). It is presentational: candidates in, one id out.
 */
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { entityDisplayTitle, entityDisplayTitleWithId } from '@/utils/entityDisplay'
import type { TargetCandidate } from '@/types'

const { candidates, mode, controlId, placeholder, allLabel } = defineProps<{
  /** Resolved candidate entities to choose among. */
  candidates: TargetCandidate[]
  /** Widget shape. */
  mode: 'select' | 'typeahead'
  /** Accessible id for the control (label `for=`). */
  controlId?: string
  /** Placeholder for the typeahead search box / empty select option. */
  placeholder?: string
  /** Label for the empty (no-filter) choice in select mode. */
  allLabel?: string
}>()

/** Committed value: the id of the chosen candidate (or ''). */
const model = defineModel<string>({ required: true })

const { t } = useI18n()
const placeholderText = computed(() => placeholder ?? t('shell.targetSelect.search'))
const allText = computed(() => allLabel ?? t('shell.targetSelect.all'))

// One option per candidate, sorted by display title case-insensitively and
// then by id. The value is the id; the label is "Title (ID)", so candidates
// that share a title can be told apart.
const sortedOptions = computed(() => {
  const opts = candidates.map((c) => ({
    value: c.id,
    title: entityDisplayTitle(c),
    label: entityDisplayTitleWithId(c),
  }))
  return opts.sort(
    (a, b) =>
      a.title.localeCompare(b.title, undefined, { sensitivity: 'base' }) ||
      a.value.localeCompare(b.value)
  )
})

// A committed value the current candidate set doesn't cover — a deep-linked or
// externally-set title that isn't among the loaded options (beyond the fetch
// cap, or during the pre-load window before candidates arrive). Without this,
// a native <select> bound to such a value would render blank/"All" even though
// the filter IS applied, so the control would silently misrepresent the active
// filter (RR-5GU270). We surface it as an explicit option so the control always
// reflects the committed value.
const missingSelectedValue = computed(() => {
  if (!model.value) return ''
  return sortedOptions.value.some((o) => o.value === model.value) ? '' : model.value
})

// A legacy deep link carries a title instead of an id. When exactly one
// candidate has that title, commit its id, so the URL moves to the id form and
// the control shows one option for it. Otherwise the value stays as it is.
const legacyTitleId = computed(() => {
  if (!missingSelectedValue.value) return ''
  const hits = sortedOptions.value.filter((o) => o.title === model.value)
  return hits.length === 1 ? hits[0].value : ''
})

watch(
  legacyTitleId,
  (id) => {
    if (id) model.value = id
  },
  { immediate: true }
)

// --- typeahead mode --------------------------------------------------------

// In-progress search string. Component-local; NEVER driven by modelValue, so an
// external modelValue change can't clobber what the user is typing (RR-NH8B6D).
const searchQuery = ref('')
const showDropdown = ref(false)

// The label to show in the closed typeahead: the committed candidate's title,
// the raw value when no candidate has that id, or empty.
const committedLabel = computed(() => {
  if (!model.value) return ''
  return sortedOptions.value.find((o) => o.value === model.value)?.title ?? model.value
})

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
  model.value = value
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
    <!-- select mode: native <select>; option values are ids -->
    <select v-if="mode === 'select'" :id="controlId" v-model="model">
      <option value="">{{ allText }}</option>
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
          :placeholder="committedLabel || placeholderText"
          :class="{ 'has-selection': !!committedLabel }"
          @focus="openTypeahead"
          @input="showDropdown = true"
        />
        <button
          v-if="committedLabel"
          type="button"
          class="clear-selection"
          :aria-label="t('shell.targetSelect.clear', { label: committedLabel })"
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
          {{ allText }}
        </div>
        <div
          v-for="opt in filteredOptions"
          :key="opt.value"
          class="dropdown-item"
          role="option"
          :aria-selected="opt.value === model"
          @click="commit(opt.value)"
        >
          {{ opt.label }}
        </div>
        <div v-if="filteredOptions.length === 0" class="dropdown-empty">
          {{ t('shell.targetSelect.noMatches') }}
        </div>
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
  border: 1px solid var(--rl-color-border);
  border-radius: 4px;
  font-size: 14px;
  min-width: 150px;
  background: var(--rl-color-bg-raised);
  color: var(--rl-color-text);
}

.typeahead-input {
  position: relative;
  display: flex;
  align-items: center;
}

.typeahead-input input {
  padding: 6px 26px 6px 10px;
  border: 1px solid var(--rl-color-border);
  border-radius: 4px;
  font-size: 14px;
  min-width: 150px;
  width: 100%;
  background: var(--rl-color-bg-raised);
  color: var(--rl-color-text);
}

.typeahead-input input.has-selection::placeholder {
  color: var(--rl-color-text);
  opacity: 1;
}

.typeahead-input input:focus,
.entity-target-select select:focus {
  outline: none;
  border-color: var(--rl-color-accent);
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

.clear-selection {
  position: absolute;
  right: 6px;
  background: none;
  border: none;
  color: var(--rl-color-text-muted);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 0 2px;
}

.clear-selection:hover {
  color: var(--rl-color-danger, #ef4444);
}

.dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
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
  color: var(--rl-color-text);
  transition: background 0.15s;
}

.dropdown-item:hover,
.dropdown-item[aria-selected='true'] {
  background: var(--rl-color-bg-hover);
}

.dropdown-item.all-option {
  color: var(--rl-color-text-muted);
  border-bottom: 1px solid var(--rl-color-border);
}

.dropdown-empty {
  padding: 12px;
  text-align: center;
  color: var(--rl-color-text-muted);
  font-size: 13px;
}
</style>
