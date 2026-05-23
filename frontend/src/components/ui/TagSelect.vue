<script setup lang="ts">
import { computed } from 'vue'
import SlimSelect from 'slim-select/vue'
import 'slim-select/styles'

const props = defineProps<{
  modelValue: string[]
  options: string[]
  placeholder?: string
  // disabled mirrors the standard HTML attribute — SlimSelect honors
  // it by suppressing user input on the wrapped select. Used by form
  // affordance plumbing (TKT-G7N5) to render a read-only multi-select.
  disabled?: boolean
  // optionVerdicts: per-option allow map. Sparse — only `false`
  // entries appear; absent keys default to allowed. Matches the
  // scalar-select option-filter shape (FieldRenderer). When provided,
  // values currently in modelValue that are denied are still
  // displayed (so the user can see + remove them) but are flagged
  // disabled in the dropdown so they can't be re-added.
  optionVerdicts?: Record<string, boolean>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const data = computed(() =>
  props.options.map((opt) => ({
    text: opt,
    value: opt,
    // Server-side affordance verdict denies this option → render
    // disabled in the dropdown so the user can't pick it.
    disabled: props.optionVerdicts?.[opt] === false,
  }))
)

const settings = computed(() => ({
  placeholderText: props.placeholder || 'Select...',
  closeOnSelect: false,
  showSearch: true,
  searchPlaceholder: 'Search...',
  allowDeselect: true,
}))

function handleUpdate(value: string[]) {
  emit('update:modelValue', value)
}
</script>

<template>
  <SlimSelect
    :model-value="modelValue"
    :data="data"
    :settings="settings"
    :multiple="true"
    :disabled="disabled"
    @update:model-value="handleUpdate"
  />
</template>

<style>
/* Ensure SlimSelect styles integrate well with the form */
.ss-main {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  min-height: 38px;
  background: var(--input-bg);
  color: var(--text-color);
}

.ss-main:focus-within {
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.ss-content {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  background: var(--card-bg);
}

.ss-option {
  color: var(--text-color);
}

.ss-option.ss-highlighted {
  background: var(--hover-bg);
}

.ss-option.ss-selected {
  background: color-mix(in srgb, var(--accent-color) 20%, transparent);
  color: var(--accent-color);
}

.ss-value {
  background: color-mix(in srgb, var(--accent-color) 20%, transparent);
  color: var(--accent-color);
}

.ss-value-delete {
  color: var(--accent-color);
}

.ss-value-delete:hover {
  color: var(--error-color, #dc2626);
}

.ss-search input {
  background: var(--input-bg);
  color: var(--text-color);
}
</style>
