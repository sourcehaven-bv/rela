<script setup lang="ts">
/**
 * A from/to pair of dates, validated against each other.
 *
 * Its own component rather than a mode on `RlDateField`, because a range is
 * two values with a rule between them: each end bounds the other, the pair
 * carries one label and one error, and a field that switched between one
 * input and two would change its own model type. `RlDateField` stays the
 * thing that picks one date.
 *
 * Both ends use the native picker for the same reasons the single field
 * does, and the model is the same native string format.
 *
 * The ends bound each other through `min` and `max` rather than by
 * validating after the fact, so the browser's own picker refuses an end
 * before the start. A message after the press would be telling the user off
 * for something the control let them do.
 */
import { computed, useId } from 'vue'
import RlFieldShell from './RlFieldShell.vue'
import RlIcon from '../common/RlIcon.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

export interface DateRange {
  start: string
  end: string
}

const props = withDefaults(
  defineProps<{
    modelValue?: DateRange
    label: string
    hint?: string
    error?: string
    required?: boolean
    labelHidden?: boolean
    disabled?: boolean
    withTime?: boolean
    /** Earliest date either end may take. */
    min?: string
    /** Latest date either end may take. */
    max?: string
    /** Names each end for a screen reader; the labels are never drawn. */
    startLabel?: string
    endLabel?: string
    timeZoneLabel?: string
  }>(),
  {
    modelValue: () => ({ start: '', end: '' }),
    required: false,
    labelHidden: false,
    disabled: false,
    withTime: false,
    startLabel: 'From',
    endLabel: 'To',
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: DateRange] }>()

const group = useId()

const hintText = computed(() =>
  props.timeZoneLabel && props.withTime
    ? [props.hint, messages.timeZoneNote({ zone: props.timeZoneLabel })].filter(Boolean).join(' ')
    : props.hint,
)

/*
 * Each end narrows the other's allowed span, on top of whatever the caller
 * set. The start cannot pass the end, and the end cannot precede the start.
 */
const startMax = computed(() => props.modelValue.end || props.max)
const endMin = computed(() => props.modelValue.start || props.min)

function update(part: keyof DateRange, value: string) {
  emit('update:modelValue', { ...props.modelValue, [part]: value })
}
</script>

<template>
  <RlFieldShell
    :label="label"
    :hint="hintText"
    :error="error"
    :required="required"
    :label-hidden="labelHidden"
  >
    <template #default="{ describedBy, invalid, required: isRequired }">
      <!--
        A group, so a reader meets the pair as one control named by the
        field's label and then the two ends inside it, rather than as two
        loose dates that happen to sit together.
      -->
      <div
        class="rl-date-range"
        role="group"
        :aria-describedby="describedBy"
        :aria-label="label"
      >
        <div class="rl-date-range__end">
          <label class="rl-visually-hidden" :for="`${group}-start`">{{ startLabel }}</label>
          <input
            :id="`${group}-start`"
            class="rl-control rl-date-range__input"
            :type="withTime ? 'datetime-local' : 'date'"
            :value="modelValue.start"
            :disabled="disabled"
            :required="isRequired"
            :min="min"
            :max="startMax"
            :aria-invalid="invalid || undefined"
            @input="update('start', ($event.target as HTMLInputElement).value)"
          />
          <RlIcon
            :name="withTime ? 'clock' : 'calendar'"
            :size="16"
            class="rl-date-range__icon"
            aria-hidden="true"
          />
        </div>

        <span class="rl-date-range__separator" aria-hidden="true">–</span>

        <div class="rl-date-range__end">
          <label class="rl-visually-hidden" :for="`${group}-end`">{{ endLabel }}</label>
          <input
            :id="`${group}-end`"
            class="rl-control rl-date-range__input"
            :type="withTime ? 'datetime-local' : 'date'"
            :value="modelValue.end"
            :disabled="disabled"
            :required="isRequired"
            :min="endMin"
            :max="max"
            :aria-invalid="invalid || undefined"
            @input="update('end', ($event.target as HTMLInputElement).value)"
          />
          <RlIcon
            :name="withTime ? 'clock' : 'calendar'"
            :size="16"
            class="rl-date-range__icon"
            aria-hidden="true"
          />
        </div>
      </div>
    </template>
  </RlFieldShell>
</template>

<style scoped>
.rl-date-range {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-date-range__end {
  position: relative;
  display: flex;
  flex: 1;
  min-width: 0;
}

.rl-date-range__input {
  width: 100%;
  padding-right: var(--rl-space-8);
}

.rl-date-range__input::-webkit-calendar-picker-indicator {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.rl-date-range__icon {
  position: absolute;
  top: 50%;
  right: var(--rl-space-3);
  transform: translateY(-50%);
  color: var(--rl-color-text-muted);
  pointer-events: none;
}

.rl-date-range__separator {
  flex: none;
  color: var(--rl-color-text-subtle);
}

/*
 * Stacked once the two ends and a datetime in each no longer fit side by
 * side. The dash is dropped rather than left floating between two rows: the
 * order of the fields already says which end is which.
 */
@media (max-width: 480px) {
  .rl-date-range { flex-direction: column; align-items: stretch; }
  .rl-date-range__separator { display: none; }
}
</style>
