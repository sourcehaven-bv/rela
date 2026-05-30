<script setup lang="ts">
import RruleBuilder from '@/components/forms/RruleBuilder.vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)
</script>

<template>
  <RruleBuilder
    :model-value="stringValue"
    :help="help"
    :readonly="disabled"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>
