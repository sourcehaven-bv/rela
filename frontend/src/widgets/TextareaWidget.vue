<script setup lang="ts">
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
}
</script>

<template>
  <textarea
    :id="id"
    :class="{ 'is-error': !!error }"
    :value="stringValue"
    :placeholder="placeholder"
    :disabled="disabled"
    rows="4"
    @input="onInput"
  />
</template>

<style scoped>
textarea {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
  transition: all 0.15s;
}

textarea:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

textarea:disabled {
  background: var(--hover-bg);
  cursor: not-allowed;
}

textarea.is-error {
  border-color: var(--error-color, #ef4444);
}

textarea.is-error:focus {
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.1);
}
</style>
