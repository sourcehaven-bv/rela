<script setup lang="ts">
/**
 * Multi-line text input.
 *
 * `autoGrow` makes the box follow its content instead of showing a scrollbar,
 * which suits a comment or a description where the length is unpredictable.
 * The height is reset before it is measured, otherwise the box can only ever
 * grow and never shrink back when text is deleted.
 */
import { nextTick, ref, watch } from 'vue'
import RlFieldShell from './RlFieldShell.vue'

const props = withDefaults(
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
    rows?: number
    autoGrow?: boolean
    /** Shows a live character count and caps input at this length. */
    maxlength?: number
  }>(),
  {
    modelValue: '',
    required: false,
    labelHidden: false,
    disabled: false,
    readonly: false,
    rows: 3,
    autoGrow: false,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const field = ref<HTMLTextAreaElement | null>(null)

function resize() {
  const el = field.value
  if (!el || !props.autoGrow) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
  resize()
}

// Also resize when the value is changed from outside, such as a form reset.
watch(() => props.modelValue, () => nextTick(resize))
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
    <textarea
      :id="control.id"
      ref="field"
      class="rl-control rl-control--multiline"
      :class="{ 'rl-textarea--auto': autoGrow }"
      :value="modelValue"
      :rows="rows"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :required="control.required"
      :maxlength="maxlength"
      :aria-describedby="control.describedBy"
      :aria-invalid="control.invalid || undefined"
      @input="onInput"
    />
    <!--
      The count is a status rather than an alert: it changes on every
      keystroke, and announcing each change would drown out the typing.
    -->
    <span v-if="maxlength" class="rl-textarea__count" role="status">
      {{ modelValue.length }} / {{ maxlength }}
    </span>
  </RlFieldShell>
</template>

<style scoped>
.rl-textarea--auto {
  resize: none;
  overflow: hidden;
}

.rl-textarea__count {
  align-self: flex-end;
  margin-top: var(--rl-space-1);
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}
</style>
