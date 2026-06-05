<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const boolValue = computed(() => props.modelValue === true || props.modelValue === 'true')

function onChange(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).checked)
}
</script>

<template>
  <!-- Display mode uses a real disabled checkbox so screen readers get
       native "checkbox, checked|unchecked, read-only" semantics, and
       rendering is consistent across system fonts (RR-UD2I). -->
  <input
    v-if="mode === 'display'"
    type="checkbox"
    :checked="boolValue"
    disabled
    aria-readonly="true"
    class="display-checkbox"
  />
  <input
    v-else
    :id="id"
    type="checkbox"
    :checked="boolValue"
    :disabled="disabled"
    @change="onChange"
  />
</template>

<style scoped>
/* Visually distinct from an editable checkbox: muted opacity + no
   pointer cursor make read-only state legible without losing the
   native checkbox affordance. */
.display-checkbox {
  opacity: 0.85;
  cursor: default;
}
</style>
