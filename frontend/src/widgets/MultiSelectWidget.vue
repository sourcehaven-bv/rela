<script setup lang="ts">
import { computed } from 'vue'
import TagSelect from '@/components/ui/TagSelect.vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const options = computed(() => props.propertyDef?.values || [])

const arrayValue = computed(() => {
  if (Array.isArray(props.modelValue)) return props.modelValue.map(String)
  if (props.modelValue) return [String(props.modelValue)]
  return []
})

function onUpdate(value: string[]) {
  emit('update:modelValue', value)
}
</script>

<template>
  <TagSelect
    :model-value="arrayValue"
    :options="options"
    :placeholder="placeholder || 'Select...'"
    :disabled="disabled"
    :option-verdicts="optionVerdicts"
    @update:model-value="onUpdate"
  />
</template>
