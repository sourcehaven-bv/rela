<script setup lang="ts">
import { computed } from 'vue'
import TagSelect from '@/components/ui/TagSelect.vue'
import Badge from '@/components/common/Badge.vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
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
  <!-- Widget owns its multiplicity (RR-UD1G): in display mode it renders
       a row of badges itself, so callers don't loop over field.values. -->
  <span v-if="mode === 'display'" class="badge-row">
    <Badge v-for="v in arrayValue" :key="v" :value="v" :property="propertyName" />
  </span>
  <TagSelect
    v-else
    :model-value="arrayValue"
    :options="options"
    :placeholder="placeholder || 'Select...'"
    :disabled="disabled"
    :option-verdicts="optionVerdicts"
    @update:model-value="onUpdate"
  />
</template>

<style scoped>
.badge-row {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
