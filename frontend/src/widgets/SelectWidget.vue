<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import Badge from '@/components/common/Badge.vue'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

const options = computed(() => props.propertyDef?.values || [])

const hasTransitions = computed(
  () => !!props.transitions && Object.keys(props.transitions).length > 0
)

const transitionEntries = computed(() => {
  if (!props.transitions) return []
  return Object.entries(props.transitions).sort((a, b) => a[0].localeCompare(b[0]))
})

// An option is disabled when EITHER the affordance verdict denies it OR
// the active transition rules don't permit moving to it. The two signals
// are independent; either is sufficient.
function isOptionDisabled(opt: string): boolean {
  if (props.optionVerdicts && props.optionVerdicts[opt] === false) {
    return true
  }
  if (!hasTransitions.value || !props.transitions) {
    return false
  }
  const currentVal = stringValue.value
  if (!currentVal || opt === currentVal) {
    return false
  }
  const allowed = props.transitions[currentVal] || []
  return !allowed.includes(opt)
}

function onChange(event: Event) {
  emit('update:modelValue', (event.target as HTMLSelectElement).value)
}
</script>

<template>
  <!-- Pass the field's wire-level binding (propertyName) to Badge for
       style lookup (RR-UD1E). When absent, Badge falls back to a
       cross-property scan of schemaStore.styles. -->
  <Badge
    v-if="mode === 'display' && stringValue"
    :value="stringValue"
    :property="propertyName"
  />
  <span v-else-if="mode === 'display'" class="display-value" />
  <div v-else class="select-widget">
    <select
      :id="id"
      :class="{ 'is-error': !!error }"
      :value="stringValue"
      :disabled="disabled"
      @change="onChange"
    >
      <option value="">Select...</option>
      <option
        v-for="opt in options"
        :key="opt"
        :value="opt"
        :disabled="isOptionDisabled(opt)"
        :class="{ 'disabled-transition': isOptionDisabled(opt) }"
      >
        {{ opt }}{{ isOptionDisabled(opt) ? ' (not allowed)' : '' }}
      </option>
    </select>

    <div v-if="hasTransitions" class="transitions-info">
      <p class="transitions-title">Allowed transitions</p>
      <div v-for="[from, tos] in transitionEntries" :key="from" class="transitions-row">
        <span class="transitions-from">{{ from }}</span>
        <span class="transitions-arrow">&rarr;</span>
        <span class="transitions-to">{{ tos.join(', ') }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.select-widget {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

select {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
  transition: all 0.15s;
}

select:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

select:disabled {
  background: var(--hover-bg);
  cursor: not-allowed;
}

select.is-error {
  border-color: var(--error-color, #ef4444);
}

select.is-error:focus {
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.1);
}

/* Restores the pre-refactor 14px stack: old layout had .form-field
   gap:6px plus .transitions-info margin-top:8px = 14px. The new
   .select-widget wrapper uses gap:8px; the 6px top here makes the
   combined gap 14px. */
.transitions-info {
  margin-top: 6px;
  padding: 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
}

.transitions-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
  margin: 0 0 8px;
}

.transitions-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  padding: 4px 0;
}

.transitions-from {
  font-weight: 500;
  color: var(--text-color);
}

.transitions-arrow {
  color: var(--muted-text);
}

.transitions-to {
  color: var(--muted-text);
}

.disabled-transition {
  color: var(--muted-text);
  font-style: italic;
}
</style>
