<script setup lang="ts">
/**
 * Single-line text input. Also covers the native text-like types, so an email
 * or a URL field gets the right keyboard on a phone without a new component.
 */
import RlFieldShell from './RlFieldShell.vue'

withDefaults(
  defineProps<{
    modelValue?: string
    label: string
    hint?: string
    error?: string
    required?: boolean
    labelHidden?: boolean
    disabled?: boolean
    readonly?: boolean
    placeholder?: string
    size?: 'sm' | 'md' | 'lg'
    /** Native type. Anything that renders as a one-line text box. */
    type?: 'text' | 'email' | 'url' | 'tel' | 'password' | 'search'
    /** Passed straight through; `off` for anything sensitive. */
    autocomplete?: string
  }>(),
  {
    modelValue: '',
    required: false,
    labelHidden: false,
    disabled: false,
    readonly: false,
    size: 'md',
    type: 'text',
  },
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
    <input
      :id="control.id"
      class="rl-control"
      :class="size !== 'md' && `rl-control--${size}`"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :required="control.required"
      :autocomplete="autocomplete"
      :aria-describedby="control.describedBy"
      :aria-invalid="control.invalid || undefined"
      @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
  </RlFieldShell>
</template>
