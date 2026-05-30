<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = computed(() => {
  if (props.modelValue === null || props.modelValue === undefined) return ''
  return String(props.modelValue)
})

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
}
</script>

<template>
  <textarea
    :id="id"
    :value="stringValue"
    :placeholder="placeholder"
    :disabled="disabled"
    rows="4"
    @input="onInput"
  />
</template>
