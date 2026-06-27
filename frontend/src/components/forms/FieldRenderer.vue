<script setup lang="ts">
import { computed } from 'vue'
import type { FormFieldOrRelation, PropertyDef, AttachmentInfo } from '@/types'
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
  // File-widget context: the owning entity identity + current attachment
  // LIST + the property's `max`, forwarded so the file widget can
  // upload/preview/remove. Only the file-property edit path supplies these.
  entityType?: string
  entityId?: string
  attachments?: AttachmentInfo[]
  max?: number
}>()

const emit = defineEmits<{
  update: [value: unknown]
  'attachment-changed': []
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
      :mode="'edit'"
      :property-def="propertyDef"
      :property-name="field.property"
      :disabled="readonly"
      :required="propertyDef?.required"
      :error="error"
      :placeholder="placeholder"
      :help="help"
      :option-verdicts="optionVerdicts"
      :transitions="field.transitions"
      :attachments="attachments"
      :max="max"
      :entity-type="entityType"
      :entity-id="entityId"
      @update:model-value="emit('update', $event)"
      @attachment-changed="emit('attachment-changed')"
    />
  </FieldShell>
</template>
