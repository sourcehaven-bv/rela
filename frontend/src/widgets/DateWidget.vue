<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { formatDate } from '@/utils/format'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

// Display-mode rendering reuses the existing utils/format.ts helper so
// dates render consistently with how PropertyDisplay formats them today
// (RR-UD1A). Falls back to the raw string for un-parseable values.
const displayValue = computed(() => {
  if (!stringValue.value) return ''
  return formatDate(stringValue.value) ?? stringValue.value
})

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ displayValue }}</span>
  <input
    v-else
    :id="id"
    type="date"
    :class="{ 'is-error': !!error }"
    :value="stringValue"
    :placeholder="placeholder"
    :disabled="disabled"
    @input="onInput"
  />
</template>

<style scoped>
input {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
  transition: all 0.15s;
}

input:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

input:disabled {
  background: var(--hover-bg);
  cursor: not-allowed;
}

input.is-error {
  border-color: var(--error-color, #ef4444);
}

input.is-error:focus {
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.1);
}
</style>
