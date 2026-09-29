<script setup lang="ts">
/**
 * Small group of mutually exclusive choices, such as Month / Week.
 *
 * Distinct from RlViewTabs: tabs switch between panels of content and use the
 * tab role, while this changes a setting that reshapes the view already on
 * screen. It is a radio group, which is what it behaves like.
 *
 * Arrow keys move between options and the roving tabindex keeps the group a
 * single Tab stop, as the radio group pattern requires.
 */
import { ref } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'

export interface SegmentOption {
  value: string
  label: string
  icon?: IconName
  disabled?: boolean
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: SegmentOption[]
    label: string
    size?: 'sm' | 'md'
    /** Shows only icons, with the label as the accessible name. */
    iconOnly?: boolean
  }>(),
  { size: 'md', iconOnly: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const root = ref<HTMLElement | null>(null)

function move(direction: 1 | -1) {
  const usable = props.options.filter((option) => !option.disabled)
  const current = usable.findIndex((option) => option.value === props.modelValue)
  const next = usable[(current + direction + usable.length) % usable.length]
  if (!next) return

  emit('update:modelValue', next.value)
  // Focus follows selection, which is the expected behaviour for a radio group.
  root.value?.querySelector<HTMLElement>(`[data-value="${next.value}"]`)?.focus()
}
</script>

<template>
  <div
    ref="root"
    class="rl-segmented"
    :class="`rl-segmented--${size}`"
    role="radiogroup"
    :aria-label="label"
    @keydown.left.prevent="move(-1)"
    @keydown.up.prevent="move(-1)"
    @keydown.right.prevent="move(1)"
    @keydown.down.prevent="move(1)"
  >
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      class="rl-segmented__option"
      :class="{ 'rl-segmented__option--active': option.value === modelValue }"
      :data-value="option.value"
      :aria-checked="option.value === modelValue"
      :aria-label="iconOnly ? option.label : undefined"
      :disabled="option.disabled"
      :tabindex="option.value === modelValue ? 0 : -1"
      @click="emit('update:modelValue', option.value)"
    >
      <RlIcon v-if="option.icon" :name="option.icon" :size="size === 'sm' ? 14 : 15" aria-hidden="true" />
      <span v-if="!iconOnly">{{ option.label }}</span>
    </button>
  </div>
</template>

<style scoped>
.rl-segmented {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-hover);
}

.rl-segmented__option {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text-muted);
  font-family: inherit;
  font-weight: var(--rl-font-weight-medium);
  cursor: pointer;
  transition: background-color var(--rl-duration-fast) var(--rl-ease), color var(--rl-duration-fast) var(--rl-ease);
}

.rl-segmented--sm .rl-segmented__option {
  height: 24px;
  padding: 0 var(--rl-space-2);
  font-size: var(--rl-font-size-xs);
}
.rl-segmented--md .rl-segmented__option {
  height: 28px;
  padding: 0 var(--rl-space-3);
  font-size: var(--rl-font-size-sm);
}

.rl-segmented__option:hover:not(:disabled):not(.rl-segmented__option--active) {
  color: var(--rl-color-text);
}

/*
 * The active option is raised off the track, not just recoloured. It takes
 * the chip token rather than the page background because it sits on the
 * track: the two have to be told apart from each other, and in the dark theme
 * the page is darker than the track, so the page colour sank the chip instead
 * of lifting it.
 */
.rl-segmented__option--active {
  background: var(--rl-color-chip-raised);
  color: var(--rl-color-text);
  box-shadow: var(--rl-shadow-sm);
}

.rl-segmented__option:disabled { opacity: 0.5; cursor: not-allowed; }

.rl-segmented__option:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: 1px;
}

@media (pointer: coarse) {
  .rl-segmented--sm .rl-segmented__option,
  .rl-segmented--md .rl-segmented__option { min-height: 32px; }
}
</style>
