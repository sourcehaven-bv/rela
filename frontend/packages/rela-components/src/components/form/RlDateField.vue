<script setup lang="ts">
/**
 * Date, or date and time, picker.
 *
 * Uses the native picker rather than a hand-built calendar: it is localised,
 * keyboard accessible and familiar on every platform. The model is the native
 * string format, `YYYY-MM-DD` or `YYYY-MM-DDTHH:mm`.
 *
 * `withTime` changes the type rather than adding a second component, because
 * a datetime differs from a date only in what it stores.
 */
import { computed } from 'vue'
import RlFieldShell from './RlFieldShell.vue'
import RlIcon from '../common/RlIcon.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label: string
    hint?: string
    error?: string
    required?: boolean
    labelHidden?: boolean
    disabled?: boolean
    withTime?: boolean
    min?: string
    max?: string
    /**
     * Named so the stored instant is unambiguous. A datetime without a stated
     * zone is read differently by every reader, so it is shown, not implied.
     */
    timeZoneLabel?: string
  }>(),
  { modelValue: '', required: false, labelHidden: false, disabled: false, withTime: false },
)

defineEmits<{ 'update:modelValue': [value: string] }>()

const hintText = computed(() =>
  props.timeZoneLabel && props.withTime
    ? [props.hint, messages.timeZoneNote({ zone: props.timeZoneLabel })].filter(Boolean).join(' ')
    : props.hint,
)
</script>

<template>
  <RlFieldShell
    v-slot="control"
    :label="label"
    :hint="hintText"
    :error="error"
    :required="required"
    :label-hidden="labelHidden"
  >
    <div class="rl-date">
      <input
        :id="control.id"
        class="rl-control rl-date__input"
        :type="withTime ? 'datetime-local' : 'date'"
        :value="modelValue"
        :disabled="disabled"
        :required="control.required"
        :min="min"
        :max="max"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      />
      <RlIcon
        :name="withTime ? 'clock' : 'calendar'"
        :size="16"
        class="rl-date__icon"
        aria-hidden="true"
      />
    </div>
  </RlFieldShell>
</template>

<style scoped>
.rl-date {
  position: relative;
  display: flex;
}

/*
 * Chrome's own indicator is hidden and replaced so the field matches the
 * select. The input keeps its click-to-open behaviour, so the drawn icon
 * needs no handler of its own.
 */
.rl-date__input { padding-right: var(--rl-space-8); }
.rl-date__input::-webkit-calendar-picker-indicator {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.rl-date__icon {
  position: absolute;
  top: 50%;
  right: var(--rl-space-3);
  transform: translateY(-50%);
  color: var(--rl-color-text-muted);
  pointer-events: none;
}
</style>
