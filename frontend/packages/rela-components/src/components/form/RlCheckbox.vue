<script setup lang="ts">
/**
 * Checkbox with its label.
 *
 * The native input stays in the DOM and keeps focus and keyboard behaviour;
 * it is only made invisible, with the visible box drawn beside it. Styling
 * the real control with `appearance: none` loses the indeterminate glyph and
 * the forced-colours rendering, so it is not used.
 */
import { computed } from 'vue'
import RlFieldShell from './RlFieldShell.vue'
import RlIcon from '../common/RlIcon.vue'

const props = withDefaults(
  defineProps<{
    modelValue?: boolean
    label: string
    hint?: string
    error?: string
    required?: boolean
    disabled?: boolean
    /** A parent whose children are only partly checked. */
    indeterminate?: boolean
    /**
     * Keeps the label as the accessible name but takes it off screen, for a
     * box whose meaning the surrounding layout already carries: a row's
     * select box under a column of them, for one.
     */
    labelHidden?: boolean
  }>(),
  {
    modelValue: false,
    required: false,
    disabled: false,
    indeterminate: false,
    labelHidden: false,
  },
)

defineEmits<{ 'update:modelValue': [value: boolean] }>()

const glyph = computed(() => (props.indeterminate ? 'minus' : 'check'))
</script>

<template>
  <RlFieldShell
    v-slot="control"
    label-mode="wrap"
    :label="label"
    :label-hidden="labelHidden"
    :hint="hint"
    :error="error"
    :required="required"
  >
    <span class="rl-checkbox">
      <input
        :id="control.id"
        class="rl-checkbox__input"
        type="checkbox"
        :checked="modelValue"
        :disabled="disabled"
        :required="control.required"
        :indeterminate="indeterminate"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @change="$emit('update:modelValue', ($event.target as HTMLInputElement).checked)"
      />
      <span
        class="rl-checkbox__box"
        :class="{ 'rl-checkbox__box--on': modelValue || indeterminate }"
        aria-hidden="true"
      >
        <RlIcon v-if="modelValue || indeterminate" :name="glyph" :size="14" />
      </span>
    </span>
  </RlFieldShell>
</template>

<style scoped>
.rl-checkbox {
  position: relative;
  display: inline-flex;
  flex: none;
}

/*
 * Kept in the layout at the size of the drawn box rather than removed, so the
 * hit area and the ring position match what the user sees.
 */
.rl-checkbox__input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  opacity: 0;
  cursor: pointer;
}

.rl-checkbox__box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg);
  color: var(--rl-color-text-inverse);
  transition: background-color var(--rl-duration-fast) var(--rl-ease), border-color var(--rl-duration-fast) var(--rl-ease);
}

.rl-checkbox__box--on {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-accent);
}

.rl-checkbox__input:focus-visible + .rl-checkbox__box {
  box-shadow:
    0 0 0 var(--rl-focus-ring-gap) var(--rl-color-bg),
    0 0 0 calc(var(--rl-focus-ring-gap) + var(--rl-focus-ring-width)) var(--rl-color-focus);
}

.rl-checkbox__input:disabled + .rl-checkbox__box {
  background: var(--rl-color-bg-hover);
  border-color: var(--rl-color-border);
  color: var(--rl-color-text-subtle);
  cursor: not-allowed;
}

@media (pointer: coarse) {
  /* The drawn box stays 18px; the invisible input grows to a full target. */
  .rl-checkbox__input {
    inset: calc((var(--rl-tap-target) - 18px) / -2);
    width: var(--rl-tap-target);
    height: var(--rl-tap-target);
  }
}
</style>
