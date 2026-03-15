<script setup lang="ts">
import { computed } from 'vue'
import SlimSelect from 'slim-select/vue'
import 'slim-select/styles'

const props = defineProps<{
  modelValue: string[]
  options: string[]
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const data = computed(() =>
  props.options.map((opt) => ({
    text: opt,
    value: opt,
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
    @update:model-value="handleUpdate"
  />
</template>

<style>
/* Ensure SlimSelect styles integrate well with the form */
.ss-main {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  min-height: 38px;
}

.ss-main:focus-within {
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.ss-content {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.ss-option.ss-highlighted {
  background: #f1f5f9;
}

.ss-option.ss-selected {
  background: #e0e7ff;
  color: #4338ca;
}

.ss-value {
  background: #e0e7ff;
  color: #4338ca;
}

.ss-value-delete {
  color: #6366f1;
}

.ss-value-delete:hover {
  color: #dc2626;
}
</style>
