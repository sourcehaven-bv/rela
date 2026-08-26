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
//
// The fallback is deliberately SILENT (no console.warn): this computed
// runs on every reactive tick, so warning here would spam the console
// for any stale/in-progress date value. The raw-string passthrough is
// the right visible signal that something is off (RR-UD2J).
const displayValue = computed(() => {
  if (!stringValue.value) return ''
  return formatDate(stringValue.value) ?? stringValue.value
})

// `<input type="date">` accepts ONLY `YYYY-MM-DD` and silently renders blank
// for anything else — including the RFC3339 timestamps the API returns for
// `date` properties (`2026-09-15T00:00:00Z`). Binding the raw string therefore
// showed an empty input over a stored value, which reads as "my data is gone".
// Narrow to the date part; pass through anything already in the right shape,
// and leave un-parseable input alone so the user's in-progress typing is never
// swallowed.
const inputValue = computed(() => {
  const raw = stringValue.value
  if (!raw) return ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw
  const match = /^(\d{4}-\d{2}-\d{2})T/.exec(raw)
  return match ? match[1] : raw
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
    :value="inputValue"
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
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

input:disabled {
  background: var(--hover-bg);
  cursor: not-allowed;
}

input.is-error {
  border-color: var(--error-color, #ef4444);
}

input.is-error:focus {
  box-shadow: 0 0 0 2px var(--error-ring);
}
</style>
