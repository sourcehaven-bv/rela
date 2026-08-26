<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { useUIStore } from '@/stores'
import { useStringValue } from './useStringValue'
import {
  formatDatetime,
  utcISOToLocalInput,
  localInputToUtcISO,
} from '@/utils/format'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const uiStore = useUIStore()

// The zone used to interpret input and render display — the user's chosen
// display timezone, or the browser zone when unset.
const timezone = computed(() => uiStore.effectiveTimezone)

const stringValue = useStringValue(() => props.modelValue)

// Display-mode rendering: format the stored UTC instant in the effective
// zone. Falls back to the raw string for un-parseable values (silent, like
// DateWidget — this computed runs every reactive tick).
const displayValue = computed(() => {
  if (!stringValue.value) return ''
  return formatDatetime(stringValue.value, timezone.value) ?? stringValue.value
})

// The value bound to <input type="datetime-local">: the stored UTC instant
// expressed as local wall-clock in the effective zone.
const inputValue = computed(() => utcISOToLocalInput(stringValue.value, timezone.value))

// Non-destructive: we emit ONLY in response to real user input on THIS field,
// converting the local wall-clock back to a canonical UTC instant. We never
// emit on mount, so simply viewing an entity (or editing an unrelated field)
// never rewrites a datetime the user didn't touch — avoiding spurious git
// churn on values with a different stored offset (RR-N1Z9BF).
function onInput(event: Event) {
  const local = (event.target as HTMLInputElement).value
  // Cleared input -> empty value (property becomes unset); otherwise convert.
  emit('update:modelValue', local === '' ? '' : localInputToUtcISO(local, timezone.value))
}
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ displayValue }}</span>
  <div v-else class="datetime-widget">
    <input
      :id="id"
      type="datetime-local"
      :class="{ 'is-error': !!error }"
      :value="inputValue"
      :placeholder="placeholder"
      :disabled="disabled"
      @input="onInput"
    />
    <span class="tz-indicator">Times shown in {{ timezone }}</span>
  </div>
</template>

<style scoped>
.datetime-widget {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

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

.tz-indicator {
  font-size: 12px;
  color: var(--text-muted, #6b7280);
}
</style>
