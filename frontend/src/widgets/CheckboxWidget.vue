<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const boolValue = computed(() => props.modelValue === true || props.modelValue === 'true')

function onChange(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).checked)
}
</script>

<template>
  <input :id="id" type="checkbox" :checked="boolValue" :disabled="disabled" @change="onChange" />
</template>
