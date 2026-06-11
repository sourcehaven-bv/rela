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

// Long-array fallback threshold (RR-UD2C): above this count, display
// mode renders a comma-joined string instead of a row of Badges to
// avoid breaking card layouts. 5 matches the pre-refactor "joined
// text is readable" rule of thumb; bump (or make configurable via a
// view option) if a real use case wants more.
const LONG_ARRAY_THRESHOLD = 5

function onUpdate(value: string[]) {
  emit('update:modelValue', value)
}
</script>

<template>
  <!-- Widget owns its multiplicity (RR-UD1G): in display mode it renders
       a row of badges itself, so callers don't loop over field.values. -->
  <span v-if="mode === 'display'" class="badge-row">
    <!-- Empty arrays render an em-dash placeholder (RR-UD2C) so a
         "no value" field is visually distinguishable from a loading
         state or a missing field. -->
    <span v-if="arrayValue.length === 0" class="empty-placeholder">—</span>
    <!-- Long arrays fall back to a comma-joined string so cards don't
         grow vertically to fit 50+ Badges. -->
    <span v-else-if="arrayValue.length > LONG_ARRAY_THRESHOLD" class="long-fallback">
      {{ arrayValue.join(', ') }}
    </span>
    <template v-else>
      <Badge v-for="v in arrayValue" :key="v" :value="v" :property="propertyName" />
    </template>
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

.empty-placeholder {
  color: var(--muted-text);
}

.long-fallback {
  color: var(--text-color);
}
</style>
