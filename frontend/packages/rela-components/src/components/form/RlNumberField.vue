<script setup lang="ts">
/**
 * Numeric input.
 *
 * The model is a number or `undefined` rather than a string, so a consumer
 * never has to parse it. An empty box is `undefined`, which is distinct from
 * a deliberate 0.
 */
import RlFieldShell from './RlFieldShell.vue'

withDefaults(
  defineProps<{
    modelValue?: number
    label: string
    hint?: string
    error?: string
    required?: boolean
    labelHidden?: boolean
    disabled?: boolean
    readonly?: boolean
    placeholder?: string
    min?: number
    max?: number
    step?: number
    /** Shown after the box, such as `hours` or `%`. */
    suffix?: string
  }>(),
  { required: false, labelHidden: false, disabled: false, readonly: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: number | undefined] }>()

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  emit('update:modelValue', raw === '' ? undefined : Number(raw))
}
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
    <div class="rl-number">
      <input
        :id="control.id"
        class="rl-control"
        type="number"
        inputmode="decimal"
        :value="modelValue ?? ''"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        :required="control.required"
        :min="min"
        :max="max"
        :step="step"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @input="onInput"
      />
      <!-- Decorative: the unit belongs in the label for a screen reader. -->
      <span v-if="suffix" class="rl-number__suffix" aria-hidden="true">{{ suffix }}</span>
    </div>
  </RlFieldShell>
</template>

<style scoped>
.rl-number {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

/* The box takes the space; the unit keeps only what its text needs. */
.rl-number .rl-control { flex: 1; min-width: 0; }

.rl-number__suffix {
  flex: none;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
}
</style>
