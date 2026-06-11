<script setup lang="ts">
import { computed } from 'vue'
import RruleBuilder from '@/components/forms/RruleBuilder.vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { formatValue } from '@/utils/format'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

// Display-mode reuses utils/format.ts (RR-UD1B); falls back to the raw
// string for un-parseable values.
const displayValue = computed(() => {
  if (!stringValue.value) return ''
  return formatValue(stringValue.value, 'rrule')
})
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ displayValue }}</span>
  <RruleBuilder
    v-else
    :model-value="stringValue"
    :help="help"
    :readonly="disabled"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>
