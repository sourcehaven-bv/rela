<script setup lang="ts">
import { computed } from 'vue'
import type { FormFieldOrRelation, PropertyDef } from '@/types'
import { defaultRegistry, defaultWidgetFor } from '@/widgets/registry'
import FieldShell from './FieldShell.vue'

const props = defineProps<{
  field: FormFieldOrRelation
  propertyDef?: PropertyDef
  value: unknown
  error?: string
  readonly?: boolean
  // Sparse per-option allow map: only `false` entries appear; absent
  // keys default to allowed. An option is disabled when EITHER this
  // map denies it or the existing transition rules deny it — the two
  // signals are independent and either one is sufficient.
  optionVerdicts?: Record<string, boolean>
}>()

const emit = defineEmits<{
  update: [value: unknown]
}>()

const fieldId = computed(() => `field-${props.field.property}`)
const label = computed(() => props.field.label || props.field.property || '')
const placeholder = computed(() => props.field.placeholder || '')
const help = computed(() => props.field.help || props.propertyDef?.description || '')

// Resolve the widget once from config + property def. The registry
// honours an explicit field.widget then falls back to type defaulting.
const resolvedWidgetName = computed(() =>
  props.field.widget && props.field.widget.trim() !== ''
    ? props.field.widget
    : defaultWidgetFor(props.propertyDef)
)
const widgetComponent = computed(() =>
  defaultRegistry.resolve(props.field.widget, props.propertyDef)
)

const isCheckbox = computed(() => resolvedWidgetName.value === 'checkbox')
// rrule renders its own help inside the builder; preserve the pre-refactor
// behaviour of forwarding help only to that widget.
const isRrule = computed(() => resolvedWidgetName.value === 'rrule')
</script>

<template>
  <FieldShell
    :field-id="fieldId"
    :label="label"
    :required="propertyDef?.required"
    :help="help"
    :error="error"
    :label-position="isCheckbox ? 'after' : 'before'"
  >
    <component
      :is="widgetComponent"
      :id="fieldId"
      :model-value="value"
      :property-def="propertyDef"
      :disabled="readonly"
      :required="propertyDef?.required"
      :error="error"
      :placeholder="placeholder"
      :help="isRrule ? help : undefined"
      :option-verdicts="optionVerdicts"
      :transitions="field.transitions"
      @update:model-value="emit('update', $event)"
    />
  </FieldShell>
</template>
