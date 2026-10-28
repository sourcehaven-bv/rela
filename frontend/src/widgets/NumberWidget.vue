<script setup lang="ts">
// The input box comes from the library's global `.rl-control`
// (rl/styles/control.css, imported by styles/rl.css): border, radius, focus
// ring, and the invalid and disabled visuals. It is declared globally rather
// than per component precisely so a host rendering a native element can reach
// it, which is what this widget is -- so this widget carries no styles of its
// own.
//
// The invalid visual is driven by `aria-invalid` there, so the old error class
// is gone: the attribute now carries both the appearance and the announcement,
// where the class did the first and nothing for the second.
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { useSchemaStore } from '@/stores/schema'
import { formatDisplayNumber } from '@/utils/format'

const props = defineProps<WidgetProps>()
const schemaStore = useSchemaStore()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

// Display mode honours the property's `display:` format, such as a currency
// (TKT-SVLQQ6). A display surface may pass only the property name, so the
// def falls back to the schema. The input stays a plain number.
const displayValue = computed(() => {
  const def =
    props.propertyDef ?? schemaStore.displayDefForProperty(props.propertyName, props.entityType)
  return formatDisplayNumber(props.modelValue, def) ?? stringValue.value
})

// An integer's declared range (TKT-BRAM1A) bounds the input's stepper. Read
// from the def, not the widget's `max` prop, which is a file's attachment
// cap. An out-of-range value is a warning, not an error (DEC-HWZHA): the
// server saves it and warns. Native min/max would instead make the browser
// refuse to submit the form, so they are set only while the value is empty
// or in range, where they cannot fail.
const bounds = computed(() => {
  const none = { min: undefined, max: undefined }
  const def = props.propertyDef
  if (def?.type !== 'integer') return none
  const raw = props.modelValue
  const n = typeof raw === 'number' ? raw : raw == null || raw === '' ? undefined : Number(raw)
  if (
    n !== undefined &&
    (Number.isNaN(n) || (def.min != null && n < def.min) || (def.max != null && n > def.max))
  ) {
    return none
  }
  return { min: def.min, max: def.max }
})

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  const num = parseInt(raw, 10)
  // Preserve FieldRenderer behaviour: emit the parsed integer, or the
  // raw string when it isn't a valid integer (e.g. mid-typing "-").
  emit('update:modelValue', isNaN(num) ? raw : num)
}
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ displayValue }}</span>
  <input
    v-else
    :id="id"
    type="number"
    class="rl-control"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
    :value="stringValue"
    :min="bounds.min"
    :max="bounds.max"
    :placeholder="placeholder"
    :disabled="disabled"
    @input="onInput"
  />
</template>
