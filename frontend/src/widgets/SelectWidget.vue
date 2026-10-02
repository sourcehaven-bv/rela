<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { useSchemaStore } from '@/stores/schema'
import Badge from '@/components/common/Badge.vue'

const props = defineProps<WidgetProps>()

const schemaStore = useSchemaStore()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

// Defensive coercion for display mode (RR-UD2F): if a caller hands an
// array to SelectWidget instead of MultiSelectWidget (e.g. TKT-HOIX1's
// explicit widget override on a list-typed field), useStringValue would
// stringify to "a,b" -- one Badge with a literal comma-joined value.
// Take the first element and warn so the misconfiguration is visible
// rather than rendering garbage.
const safeStringValue = computed(() => {
  const v = props.modelValue
  if (Array.isArray(v)) {
    if (v.length > 1) {
      console.warn(
        '[SelectWidget] received multi-element array in display mode; rendering first only -- consider widget: multi-select'
      )
    }
    return v.length > 0 ? String(v[0]) : ''
  }
  return stringValue.value
})

const options = computed(() => props.propertyDef?.values || [])

// Display labels keyed by value (value stays the submitted identity). Shared
// resolver so all enum pickers agree; see the store. Edit-mode `<option>` text
// shows the label, falling back to the raw value when unlabeled.
const optionLabels = computed(() =>
  schemaStore.resolveOptionLabels(props.propertyDef, props.propertyName, props.entityType)
)
function optionLabel(opt: string): string {
  return optionLabels.value[opt] ?? opt
}

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
       deterministic style lookup (RR-UD1E + RR-UD2D). Display mode uses
       safeStringValue, which guards against array input (RR-UD2F). -->
  <Badge
    v-if="mode === 'display' && safeStringValue"
    :value="safeStringValue"
    :property="propertyName"
    :entity-type="entityType"
  />
  <span v-else-if="mode === 'display'" class="display-value" />
  <div v-else class="select-widget">
    <select
      :id="id"
      class="rl-control"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
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
        {{ optionLabel(opt) }}{{ isOptionDisabled(opt) ? ' (not allowed)' : '' }}
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

/* Restores the pre-refactor 14px stack: old layout had .form-field
   gap:6px plus .transitions-info margin-top:8px = 14px. The new
   .select-widget wrapper uses gap:8px; the 6px top here makes the
   combined gap 14px. */
.transitions-info {
  margin-top: 6px;
  padding: 12px;
  background: var(--rl-color-bg-hover);
  border: 1px solid var(--rl-color-border);
  border-radius: 6px;
}

.transitions-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--rl-color-text-muted);
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
  color: var(--rl-color-text);
}

.transitions-arrow {
  color: var(--rl-color-text-muted);
}

.transitions-to {
  color: var(--rl-color-text-muted);
}

.disabled-transition {
  color: var(--rl-color-text-muted);
  font-style: italic;
}
</style>
