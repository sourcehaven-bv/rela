<script setup lang="ts">
import { computed } from 'vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import NavIcon from '@/components/common/NavIcon.vue'
import { NO_ICON, iconNames, isKnownIcon } from '@/utils/icons'

/**
 * Chooses an icon from the fixed set the server accepts, so a typo cannot
 * reach a save. Empty means the icon the kind of entry derives; `none`
 * means no icon. A stored name outside the set stays selectable, so opening
 * a screen never changes it.
 */
const props = withDefaults(
  defineProps<{
    modelValue: string
    label?: string
    /** What empty means, such as "The list icon". */
    defaultLabel?: string
  }>(),
  { label: 'Icon', defaultLabel: 'The default icon' }
)
const emit = defineEmits<{ 'update:modelValue': [value: string | undefined] }>()

const options = computed(() => {
  const names = iconNames()
  const unknown =
    props.modelValue && props.modelValue !== NO_ICON && !isKnownIcon(props.modelValue)
      ? [{ value: props.modelValue, label: `${props.modelValue} (unknown)` }]
      : []
  return [
    { value: '', label: props.defaultLabel },
    { value: NO_ICON, label: 'No icon' },
    ...unknown,
    ...names.map((n) => ({ value: n, label: n })),
  ]
})
</script>

<template>
  <div class="icon-picker">
    <RlSelect
      :model-value="modelValue"
      :label="label"
      size="sm"
      :options="options"
      @update:model-value="emit('update:modelValue', $event || undefined)"
    />
    <span class="icon-picker__glyph" aria-hidden="true">
      <NavIcon v-if="modelValue" :name="modelValue" />
    </span>
  </div>
</template>

<style scoped>
.icon-picker {
  display: flex;
  align-items: flex-end;
  gap: var(--rl-space-2);
}

.icon-picker > :first-child {
  flex: 1;
}

.icon-picker__glyph {
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
}
</style>
