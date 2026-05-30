<script setup lang="ts">
import { computed } from 'vue'
import RruleBuilder from '@/components/forms/RruleBuilder.vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const stringValue = computed(() => {
  if (props.modelValue === null || props.modelValue === undefined) return ''
  return String(props.modelValue)
})
</script>

<template>
  <RruleBuilder
    :model-value="stringValue"
    :help="help"
    :readonly="disabled"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>
