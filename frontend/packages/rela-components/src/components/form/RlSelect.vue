<script setup lang="ts">
/**
 * Single-choice dropdown.
 *
 * Deliberately the native `select`. It is already keyboard accessible, it
 * opens as the platform picker on a phone, and it needs no focus management,
 * none of which a custom listbox gets for free.
 */
import RlFieldShell from './RlFieldShell.vue'
import RlIcon from '../common/RlIcon.vue'
import type { SelectOption } from './types'

withDefaults(
  defineProps<{
    modelValue?: string
    label: string
    options: SelectOption[]
    hint?: string
    error?: string
    required?: boolean
    labelHidden?: boolean
    disabled?: boolean
    size?: 'sm' | 'md' | 'lg'
    /** Shown as a disabled first option while nothing is chosen. */
    placeholder?: string
  }>(),
  { modelValue: '', required: false, labelHidden: false, disabled: false, size: 'md' },
)

defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <RlFieldShell
    v-slot="control"
    :label="label"
    :hint="hint"
    :error="error"
    :required="required"
    :label-hidden="labelHidden"
  >
    <div class="rl-select">
      <select
        :id="control.id"
        class="rl-control rl-select__input"
        :class="size !== 'md' && `rl-control--${size}`"
        :value="modelValue"
        :disabled="disabled"
        :required="control.required"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @change="$emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
      >
        <option v-if="placeholder" value="" disabled>{{ placeholder }}</option>
        <option
          v-for="option in options"
          :key="option.value"
          :value="option.value"
          :disabled="option.disabled"
        >
          {{ option.label }}
        </option>
      </select>
      <!-- The native arrow differs per platform, so it is hidden and redrawn. -->
      <RlIcon name="chevron-down" :size="16" class="rl-select__arrow" aria-hidden="true" />
    </div>
  </RlFieldShell>
</template>

<style scoped>
.rl-select {
  position: relative;
  display: flex;
}

.rl-select__input {
  appearance: none;
  /* Room for the arrow, so a long option label never runs underneath it. */
  padding-right: var(--rl-space-8);
  cursor: pointer;
}

.rl-select__arrow {
  position: absolute;
  top: 50%;
  right: var(--rl-space-3);
  transform: translateY(-50%);
  color: var(--rl-color-text-muted);
  pointer-events: none;
}
</style>
