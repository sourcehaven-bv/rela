<script setup lang="ts">
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  const num = parseInt(raw, 10)
  // Preserve FieldRenderer behaviour: emit the parsed integer, or the
  // raw string when it isn't a valid integer (e.g. mid-typing "-").
  emit('update:modelValue', isNaN(num) ? raw : num)
}
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ stringValue }}</span>
  <input
    v-else
    :id="id"
    type="number"
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
