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
      class="rl-control"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
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

/*
 * The input box comes from the library's global `.rl-control`
 * (rl/styles/control.css, imported by styles/rl.css): border, radius, focus
 * ring, and the invalid and disabled visuals. It is declared globally rather
 * than per component precisely so a host rendering a native element can reach
 * it, which is what this widget is.
 *
 * The invalid visual is driven by `aria-invalid` there, so the old error class
 * is gone: the attribute now carries both the appearance and the announcement,
 * where the class did the first and nothing for the second.
 */

.tz-indicator {
  font-size: 12px;
  color: var(--text-muted, #6b7280);
}
</style>
